// Package collect — «Собрать проект в папку» (069): план сбора (что копировать и куда), копирование
// с отменой и ходом, копия проекта с путями в собранной папке и её запись только относительными
// путями. Пишет только в папку сбора и только под своими именами: ничего не перезаписывает и не
// удаляет, кроме своих временных *.sacopy.tmp. Методы App — internal/app/collect.go.
//
// Карта: plan.go (Make, раскладка по папкам, имена), fonts.go (AddFonts — свои шрифты), copy.go
// (Copy, CheckSpace, ErrDiskFull), album.go (Album, Save — собранный альбом), size.go (SizeText).
package collect

import (
	"cmp"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Служебные папки сбора (decisions №11): имена по-русски, как весь интерфейс.
const (
	DirPhotos = "Фото"
	DirOther  = "Прочие" // внутри DirPhotos: фото не из папок альбома
	// DirLoose — отдельные фото (097, decisions 097-5): рядом с DirPhotos, а не внутри — иначе
	// папка альбома «Фото» накрыла бы их и добавления пропали бы.
	DirLoose  = "Фото отдельно"
	DirOwn    = "Свои фоны и стикеры"
	DirGuides = "Разметка"
)

// Виды элементов плана.
const (
	KindPhoto = "photo" // фото альбома (Refs не пусты) или из ленты в режиме «Все» (Refs пусты)
	KindOwn   = "own"   // своё из библиотеки (Lib): в собранном альбоме — обычный файл
	KindGuide = "guide" // разметка типографии; Refs — RefCover и/или RefSpreads
	KindXMP   = "xmp"   // сайдкар фото
)

// Ссылки разметки в Item.Refs.
const (
	RefCover   = "cover"
	RefSpreads = "spreads"
)

// mtimeSlack — допуск времени «уже на месте»: FAT32 и exFAT флешек хранят время с шагом 2 с.
const mtimeSlack = 2 * time.Second

// maxDup — сколько вариантов « (N)» пробовать на одно имя.
const maxDup = 999

// Item — файл плана.
type Item struct {
	Kind    string
	Src     string // абсолютный путь источника
	Rel     string // путь в папке сбора, "/"-разделители, только наши имена
	Size    int64
	ModTime time.Time
	Skip    bool     // уже на месте: тот же размер и время ±2 с
	Refs    []string // ID записей Photos (photo, own) или RefCover/RefSpreads (guide)
}

// Plan — план сбора.
type Plan struct {
	Items         []Item
	Missing       []string // ID фото, которых нет: в собранном альбоме остаются со старым путём
	MissingGuides int      // разметок, которых нет (остаются со старым путём)
	SideBusy      int      // сайдкаров, чьё имя в папке занято чужим файлом: не копируются
	Bytes         int64    // к копированию (без Skip)
	// Свои шрифты (082, AddFonts): гарнитуры, чьи файлы запрещено переносить, и сколько стоящих в
	// альбоме гарнитур нет на этом компьютере.
	FontsRestricted []string
	MissingFonts    int
	// LooseLost — ключи (key) отдельных фото не из альбома, которых нет на диске (режим «Все»): в
	// собранном альбоме остаются со старым путём, как пропавшие фото альбома.
	LooseLost []string
}

// Info — сводка для окна (069, контракт CollectInfo). Scanning и FolderName заполняет App: лента
// ещё читается, имя папки — от имени файла альбома.
type Info struct {
	Photos        int    `json:"photos"` // фото альбома
	Extra         int    `json:"extra"`  // фото из папок альбома вне альбома (режим «Все»)
	Loose         int    `json:"loose"`  // отдельные фото (097) вне альбома (режим «Все»): не входят в Extra
	Own           int    `json:"own"`
	Guides        int    `json:"guides"`
	Missing       int    `json:"missing"`
	MissingGuides int    `json:"missingGuides"`
	Bytes         int64  `json:"bytes"`
	Scanning      bool   `json:"scanning"`
	FolderName    string `json:"folderName"` // имя папки, которую создаст сбор (заполняет App)
	// Fonts — файлов своих шрифтов к копированию; FontsRestricted — названия гарнитур, которые
	// лицензия не даёт переносить; MissingFonts — гарнитур альбома нет на этом компьютере (082).
	Fonts           int      `json:"fonts"`
	FontsRestricted []string `json:"fontsRestricted"`
	MissingFonts    int      `json:"missingFonts"`
}

// Info — сводка плана; сайдкары считаются только в байтах.
func (pl Plan) Info() Info {
	in := Info{Missing: len(pl.Missing), MissingGuides: pl.MissingGuides, Bytes: pl.Bytes,
		FontsRestricted: append([]string{}, pl.FontsRestricted...), MissingFonts: pl.MissingFonts}
	for _, it := range pl.Items {
		switch {
		case it.Kind == KindPhoto && len(it.Refs) > 0:
			in.Photos++
		case it.Kind == KindPhoto && strings.HasPrefix(it.Rel, DirLoose+"/"):
			in.Loose++
		case it.Kind == KindPhoto:
			in.Extra++
		case it.Kind == KindOwn:
			in.Own++
		case it.Kind == KindGuide:
			in.Guides++
		case it.Kind == KindFont:
			in.Fonts++
		}
	}
	return in
}

// reachable — отвечает ли том источника; подменяется в тестах.
var reachable = osx.Reachable

// Make — план сбора проекта p в папку dir ("" — без сверки с папкой назначения: сводка окна,
// ничего не пропускается). extra — пути фото ленты (режим «Все», не nil): берутся лежащие внутри
// папок альбома и все отдельные фото (097). stat — Stat источника (os.Stat; подменяется в тестах).
func Make(p *model.Project, extra []string, dir string, stat func(string) (fs.FileInfo, error)) (Plan, error) {
	var pl Plan
	roots := makeRoots(p.Folders)
	srcs := sources(p, extra, roots, stat, &pl)
	loose := looseSet(p)
	for i := range srcs {
		if loose[key(srcs[i].path)] && srcs[i].kind == KindPhoto {
			srcs[i].dest = DirLoose
		} else {
			srcs[i].dest = destDir(srcs[i], roots)
		}
		srcs[i].lowDest = nameKey(srcs[i].dest)
	}
	// Нижний регистр — заранее: в сравнении сортировки ToLower занимал почти всё время плана.
	slices.SortFunc(srcs, func(a, b src) int {
		return cmp.Or(strings.Compare(a.lowDest, b.lowDest), strings.Compare(a.low, b.low), strings.Compare(a.path, b.path))
	})
	for start := 0; start < len(srcs); {
		end := start + 1
		for end < len(srcs) && srcs[end].lowDest == srcs[start].lowDest {
			end++
		}
		if err := nameDir(&pl, srcs[start:end], dir, stat); err != nil {
			return Plan{}, err
		}
		start = end
	}
	for _, it := range pl.Items {
		if !it.Skip {
			pl.Bytes += it.Size
		}
	}
	return pl, nil
}

// src — источник до выбора имени.
type src struct {
	kind    string
	path    string
	low     string // path в нижнем регистре
	refs    []string
	st      fs.FileInfo
	dest    string // каталог назначения от папки сбора, "/"
	lowDest string
}

// root — папка альбома, ставшая папкой внутри «Фото»; low — путь в нижнем регистре с
// разделителем на конце (префикс для «внутри», как library.Under).
type root struct{ path, name, low string }

func lowDir(path string) string {
	low := lower(filepath.Clean(path))
	if !strings.HasSuffix(low, string(filepath.Separator)) {
		low += string(filepath.Separator)
	}
	return low
}

// makeRoots — корни: папки альбома от коротких к длинным; вложенная в уже взятую — её часть, не
// корень. Имя — имя папки через FolderName, повтор без учёта регистра (и «Прочие») — « (2)»;
// имена — в порядке папок альбома (первая добавленная — без номера), не длины.
func makeRoots(folders []string) []root {
	list := make([]string, len(folders))
	for i, f := range folders {
		list[i] = filepath.Clean(f)
	}
	byLen := slices.Clone(list)
	slices.SortStableFunc(byLen, func(a, b string) int { return len(a) - len(b) })
	var top []string
	for _, f := range byLen {
		// Отдельный корень внутри другого (подпапка на «.» или через ссылку, 093-4) — часть корня:
		// FolderName срезает точку, копия ссылкой не будет — сканер собранного альбома его прочтёт.
		if !slices.ContainsFunc(top, func(r string) bool { return paths.SamePath(r, f) || library.Under(f, r) }) {
			top = append(top, f)
		}
	}
	used := map[string]bool{nameKey(DirOther): true}
	var out []root
	for _, f := range list {
		if i := slices.Index(top, f); i >= 0 {
			top[i] = "" // повтор той же папки в списке — один корень
			out = append(out, root{f, dupName(paths.FolderName(filepath.Base(f), "Папка"), used), lowDir(f)})
		}
	}
	return out
}

// dupName — name или «name (N)», не занятое в used (регистр не важен); занимает его.
func dupName(name string, used map[string]bool) string {
	cand := name
	for n := 2; used[nameKey(cand)]; n++ {
		cand = fmt.Sprintf("%s (%d)", name, n)
	}
	used[nameKey(cand)] = true
	return cand
}

// sources — файлы проекта: записи Photos (один файл — один источник со всеми ID), фото ленты
// внутри корней (режим «Все»), разметки. Нет файла — в Missing / MissingGuides.
func sources(p *model.Project, extra []string, roots []root, stat func(string) (fs.FileInfo, error), pl *Plan) []src {
	var out []src
	at := map[string]int{} // ключ пути → индекс в out
	add := func(kind, path, ref string) bool {
		k := key(path)
		if i, ok := at[k]; ok {
			if ref != "" {
				out[i].refs = append(out[i].refs, ref)
			}
			if kind == KindPhoto && out[i].kind == KindOwn {
				out[i].kind = KindPhoto // и фото альбома, и своё — лежит с фото
			}
			return true
		}
		st, ok := regular(kind, path, stat)
		if !ok {
			return false
		}
		s := src{kind: kind, path: filepath.Clean(path), st: st}
		s.low = lower(s.path)
		if ref != "" {
			s.refs = []string{ref}
		}
		at[k] = len(out)
		out = append(out, s)
		return true
	}
	ownOnly := ownOnlyRefs(p)
	for _, id := range slices.Sorted(maps.Keys(p.Photos)) {
		ref := p.Photos[id]
		if ref.Path == "" || !add(kindOf(ref, ownOnly[id]), ref.Path, id) {
			pl.Missing = append(pl.Missing, id)
		}
	}
	for _, e := range extraIn(extra, roots) {
		if _, in := at[key(e)]; !in {
			add(KindPhoto, e, "") // пропало из ленты за время сбора — просто не берём
		}
	}
	for _, f := range looseAll(p, extra) {
		if _, in := at[key(f)]; !in && !add(KindPhoto, f, "") {
			pl.LooseLost = append(pl.LooseLost, key(f))
		}
	}
	guide := func(path, ref string) {
		if path != "" && !add(KindGuide, path, ref) {
			pl.MissingGuides++
		}
	}
	if p.Cover != nil {
		guide(p.Cover.Guide, RefCover)
	}
	guide(p.SpreadGuide, RefSpreads)
	return out
}

// kindOf — вид источника записи: своё из библиотеки или запись только рамочек и своих стикеров — KindOwn.
func kindOf(ref model.PhotoRef, ownOnly bool) string {
	if ref.Lib != "" || ownOnly {
		return KindOwn
	}
	return KindPhoto
}

// extraIn — фото ленты (режим «Все») внутри корней альбома; пустые пути отброшены.
func extraIn(extra []string, roots []root) []string {
	var out []string
	for _, e := range extra {
		le := lower(filepath.Clean(e))
		if e != "" && slices.ContainsFunc(roots, func(r root) bool { return strings.HasPrefix(le, r.low) }) {
			out = append(out, e)
		}
	}
	return out
}

// looseAll — отдельные фото для режима «Все» (extra != nil): все, не только прочитанные лентой;
// вне режима — ничего.
func looseAll(p *model.Project, extra []string) []string {
	if extra == nil {
		return nil
	}
	return p.LooseFiles()
}

// looseSet — ключи (key) путей отдельных фото проекта.
func looseSet(p *model.Project) map[string]bool {
	out := map[string]bool{}
	for _, f := range p.LooseFiles() {
		out[key(f)] = true
	}
	return out
}

// ownOnlyRefs — записи, на которые ссылаются только рамочки мест и свои стикеры (079-27): в
// собранном альбоме lib снят (retarget), и при повторном сборе такая запись иначе ушла бы как
// фото в «Фото/Прочие» — в ленту. Фото фона не в счёт: фоном бывает и обычный кадр ленты.
func ownOnlyRefs(p *model.Project) map[string]bool {
	own, other := map[string]bool{}, map[string]bool{}
	for _, s := range p.Sheets() {
		for _, id := range s.Photos {
			other[id] = true
		}
		for _, b := range s.Backdrops() {
			other[b.Photo] = true
		}
		for _, st := range s.Stickers {
			if st.Photo != "" {
				own[st.Photo] = true
			}
		}
		for _, id := range s.BorderPhotos() {
			own[id] = true
		}
	}
	for id := range other {
		delete(own, id)
	}
	return own
}

// fileExt — расширение годится для вида: копируем только то, что программа сама берёт (фото —
// JPEG и PNG, своё из библиотеки — ещё SVG, как decor.ImportSticker; разметка — GuideExt; шрифт —
// TTF и OTF).
// Подменённый .salbum иначе вынес бы в папку сбора чужие файлы, .lnk, .url, desktop.ini.
func fileExt(kind, path string) bool {
	switch kind {
	case KindGuide:
		return salbum.GuideExt(path)
	case KindFont:
		ext := strings.ToLower(filepath.Ext(path))
		return ext == ".ttf" || ext == ".otf"
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png":
		return true
	case ".svg":
		return kind == KindOwn
	}
	return false
}

// regular — источник вида kind — обычный файл годного расширения на отвечающем томе.
func regular(kind, path string, stat func(string) (fs.FileInfo, error)) (fs.FileInfo, bool) {
	if !fileExt(kind, path) || paths.CheckLocalPath(path) != nil || !reachable(path) {
		return nil, false
	}
	st, err := stat(path)
	if err != nil || !st.Mode().IsRegular() {
		return nil, false
	}
	return st, true
}

// destDir — каталог назначения источника (от папки сбора, "/").
func destDir(s src, roots []root) string {
	switch s.kind {
	case KindOwn:
		return DirOwn
	case KindGuide:
		return DirGuides
	}
	for _, r := range roots {
		if !strings.HasPrefix(s.low, r.low) {
			continue
		}
		rel, _ := filepath.Rel(r.path, filepath.Dir(s.path)) // путь внутри корня: Rel не ошибается
		parts := []string{DirPhotos, r.name}
		if rel != "." {
			for _, el := range strings.Split(filepath.ToSlash(rel), "/") {
				parts = append(parts, paths.FolderName(el, "Папка"))
			}
		}
		return strings.Join(parts, "/")
	}
	return DirPhotos + "/" + DirOther
}

// nameDir — имена источников одного каталога назначения: сначала файлы, затем их сайдкары
// (под итоговым именем фото). Один ReadDir на каталог.
func nameDir(pl *Plan, group []src, dir string, stat func(string) (fs.FileInfo, error)) error {
	dest := group[0].dest
	nm := names{used: map[string]bool{}, onDisk: map[string]fs.DirEntry{}}
	if dir != "" {
		if ents, err := os.ReadDir(filepath.Join(dir, filepath.FromSlash(dest))); err == nil {
			for _, e := range ents {
				nm.onDisk[nameKey(e.Name())] = e
			}
		}
	}
	var sides []sideSrc
	seenSide := map[string]bool{}
	for _, s := range group {
		fallback := "Файл"
		if s.kind == KindPhoto {
			fallback = "Фото"
		}
		base := paths.FileName(filepath.Base(s.path), fallback)
		name, skip, ok := nm.pick(base, s.st)
		if !ok {
			return fmt.Errorf("в папке «%s» слишком много файлов с именем «%s»", dest, base)
		}
		pl.Items = append(pl.Items, Item{Kind: s.kind, Src: s.path, Rel: dest + "/" + name, Size: s.st.Size(),
			ModTime: s.st.ModTime(), Skip: skip, Refs: s.refs})
		if s.kind != KindPhoto {
			continue
		}
		if sp, long, ok := sidecar(s.path, stat); ok && !seenSide[key(sp.path)] {
			seenSide[key(sp.path)] = true
			sp.name = strings.TrimSuffix(name, filepath.Ext(name)) + ".xmp"
			if long {
				sp.name = name + ".xmp"
			}
			sides = append(sides, sp)
		}
	}
	for _, sd := range sides {
		ok, skip := nm.free(sd.name, sd.st)
		if !ok {
			pl.SideBusy++
			continue
		}
		nm.used[nameKey(sd.name)] = true
		pl.Items = append(pl.Items, Item{Kind: KindXMP, Src: sd.path, Rel: dest + "/" + sd.name, Size: sd.st.Size(),
			ModTime: sd.st.ModTime(), Skip: skip})
	}
	return nil
}

// names — имена одного каталога назначения: занятые планом (used) и лежащие на диске (onDisk);
// ключи — в нижнем регистре.
type names struct {
	used   map[string]bool
	onDisk map[string]fs.DirEntry
}

// free — имя свободно (ok) или на диске под ним уже тот же файл (ok и skip).
func (nm names) free(name string, st fs.FileInfo) (ok, skip bool) {
	if nm.used[nameKey(name)] {
		return false, false
	}
	e, there := nm.onDisk[nameKey(name)]
	if !there {
		return true, false
	}
	return same(e, st), same(e, st)
}

// pick — первое годное из «имя.ext», «имя (2).ext» … «имя (maxDup).ext»; занимает его.
func (nm names) pick(base string, st fs.FileInfo) (name string, skip, ok bool) {
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for n := 1; n <= maxDup; n++ {
		cand := base
		if n > 1 {
			cand = fmt.Sprintf("%s (%d)%s", stem, n, ext)
		}
		if ok, skip := nm.free(cand, st); ok {
			nm.used[nameKey(cand)] = true
			return cand, skip, true
		}
	}
	return "", false, false
}

// sideSrc — найденный сайдкар источника.
type sideSrc struct {
	path, name string
	st         fs.FileInfo
}

// sidecar — сайдкар фото, как его берёт сканер ленты (library sidecarFor): «имя.jpg.xmp», затем
// «имя.xmp»; long — первый вид.
func sidecar(photo string, stat func(string) (fs.FileInfo, error)) (s sideSrc, long, ok bool) {
	for i, cand := range []string{photo + ".xmp", strings.TrimSuffix(photo, filepath.Ext(photo)) + ".xmp"} {
		if st, err := stat(cand); err == nil && st.Mode().IsRegular() {
			return sideSrc{path: cand, st: st}, i == 0, true
		}
	}
	return sideSrc{}, false, false
}

// same — на диске обычный файл того же размера и времени изменения (±2 с).
func same(e fs.DirEntry, st fs.FileInfo) bool {
	if !e.Type().IsRegular() {
		return false
	}
	fi, err := e.Info()
	if err != nil || fi.Size() != st.Size() {
		return false
	}
	d := fi.ModTime().Sub(st.ModTime())
	return d <= mtimeSlack && d >= -mtimeSlack
}

// key — ключ пути: один файл — один ключ (Windows без учёта регистра, как paths.SamePath).
func key(p string) string {
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

func lower(s string) string { return strings.ToLower(s) }

// nameKey — ключ имени в папке сбора: без регистра и с «й», «ё» в NFC — macOS (APFS, HFS+)
// не различает NFC и NFD, Windows и macOS — регистр; разные ключи одного имени дали бы
// коллизию только при копировании.
func nameKey(s string) string { return lower(model.ComposeNames(s)) }

// relLocal — Rel плана годится как путь внутри папки сбора (страж от ошибки плана).
func relLocal(rel string) bool {
	return rel != "" && path.Clean(rel) == rel && filepath.IsLocal(filepath.FromSlash(rel))
}
