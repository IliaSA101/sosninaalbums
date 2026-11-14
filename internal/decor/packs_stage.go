package decor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/looktpl"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Проверка содержимого набора (design §4.2): распаковка во временные файлы, пределы, шаблоны и
// ссылки на картинки — до записи в библиотеку (её делает packs_install.go).

// Пределы содержимого набора (design §4.2): набор готовят заранее — больше не уменьшаем.
const (
	packMaxBgPixels = 100_000_000
	packMaxSide     = 4096 // сторона стикера
)

// Ответ на повтор набора (Staged.Ask).
const (
	AskSame   = "same"   // эта версия уже стоит
	AskUpdate = "update" // стоит старее — вопрос «Заменить?»
	AskOlder  = "older"  // стоит новее — отказ
)

// Staged — набор, распакованный во временные файлы и проверенный, до записи в библиотеку;
// держит временные файлы до InstallPack или Close.
type Staged struct {
	Ask     string // "" — ставить; AskSame, AskUpdate, AskOlder — набор с этим id уже стоит
	From    int    // версия стоящего
	Skipped []Skip

	m       PackManifest
	decls   []themeDecl // объявления манифеста после правила decor (themes_rules.go)
	origin  string
	zip     string
	zipSz   int64 // размер zip из каталога (только из сети), 0 — не известен
	items   []stagedItem
	refresh bool // та же версия из сети, реестр писала программа без тем (tv = 0): перекачка (170-19)
}

type stagedItem struct {
	kind, key, theme, name, ext string
	topic                       string  // тема набора (170): id из decls или встроенная
	tmp                         string  // картинка: временный файл в папке вида
	win                         winInfo // рамочка: окно, найденное при проверке
	tpl                         []byte  // шаблон: канонический JSON (ключ — TplKey); макет и типография — байты как пришли
	item                        string  // макет: путь записи в манифесте (поиск после обновления набора)
	src                         string  // sha256[:16] записи манифеста (packEntry.Src)
	libs                        []libRef
	w, h                        int
	bytes                       int64
}

// libRef — ссылка шаблона на картинку библиотеки: фон (TplBg.Lib), стикер (TplItem.Lib) или
// рамочка места (TplBorder.Lib, вид frame); kind "" — ключ из сырых байт макета (поле новее
// программы), вид неизвестен.
type libRef struct{ kind, key string }

// SetZipSize запоминает размер zip из каталога: набор из сети показывает его как PackInfo.Bytes (137-31).
func (st *Staged) SetZipSize(n int64) { st.zipSz = max(n, 0) }

// Close удаляет временные файлы, не ушедшие в библиотеку.
func (st *Staged) Close() {
	if st == nil {
		return
	}
	for i := range st.items {
		if st.items[i].tmp != "" {
			_ = os.Remove(st.items[i].tmp)
			st.items[i].tmp = ""
		}
	}
}

// Info — строка набора для вопроса: проверенные предметы, а до распаковки (повтор) — манифест.
func (st *Staged) Info() PackInfo {
	r := st.reg(0)
	if len(st.items) == 0 {
		for _, it := range st.m.Items {
			r.Items = append(r.Items, packEntry{Kind: it.Kind, Bytes: it.Size})
		}
	}
	p := r.info()
	p.Removable = false
	return p
}

// reg — реестр набора из проверенных предметов.
func (st *Staged) reg(installed int64) *packReg {
	r := &packReg{V: packRegV, ID: st.m.ID, Version: st.m.Version, Title: cleanName(st.m.Title), About: cleanText(st.m.About, 300),
		Author: cleanName(st.m.Author), License: cleanText(st.m.License, 40), Source: cleanText(st.m.Source, 200),
		Origin: st.origin, MinApp: st.m.MinApp, Installed: installed, Zip: st.zip, ZipSize: st.zipSz, Items: make([]packEntry, 0, len(st.items)),
		Themes: st.decls, TV: 1}
	if r.Title == "" {
		r.Title = st.m.ID
	}
	for _, it := range st.items {
		r.Items = append(r.Items, packEntry{Key: it.key, Kind: it.kind, Theme: it.theme, Name: it.name, W: it.w, H: it.h, Bytes: it.bytes, Item: it.item, Src: it.src, Topic: it.topic})
	}
	return r
}

// StagePack — проверка содержимого набора во временные файлы (без записи в библиотеку).
// origin — OriginFile | OriginNet | OriginDemo, zipSum — sha256 архива. Набор с тем же id уже стоит — Ask
// без распаковки (AskUpdate распаковывается: его ставит ConfirmPack; тот же набор из сети и реестр
// без tv — тоже AskUpdate, перекачка 170-9). Битый архив — ошибка,
// временные удалены; плохой предмет — Skipped; не осталось ни одного — ErrPackEmpty (Staged
// с причинами всё равно возвращается).
func (s *Store) StagePack(m PackManifest, src PackSource, origin, zipSum string) (*Staged, error) {
	if !validPackID(m.ID) {
		return nil, ErrBadPackID
	}
	if origin != OriginNet && origin != OriginDemo {
		origin = OriginFile
	}
	st := &Staged{m: m, decls: checkThemeDecls(m.Themes), origin: origin, zip: zipSum, Skipped: []Skip{}}
	s.mu.RLock()
	ro, old, used := s.readOnly, s.packOf(m.ID), s.libBytes()
	oldBytes := s.packDiskBytes(old) // nil — 0
	s.mu.RUnlock()
	if ro {
		return nil, ErrReadOnly
	}
	if old != nil {
		st.From = old.Version
		switch {
		case m.Version == old.Version && origin == OriginNet && old.Origin == OriginNet && old.TV == 0:
			// Решение о перекачке принял план (libpacks.netPlan, 170-19): здесь только условие по реестру,
			// без счёта принятых тем — иначе отказ при нуле принятых давал бы петлю «Обновить».
			st.refresh = true
		case m.Version == old.Version:
			st.Ask = AskSame
			return st, nil
		case m.Version < old.Version:
			st.Ask = AskOlder
			return st, nil
		}
		st.Ask = AskUpdate
		used -= oldBytes // стоящая версия уходит: как предпроверка netInstall; точная сверка — в InstallPack
	}
	var need int64
	for _, it := range m.Items {
		need += max(it.Size, 0)
	}
	if used+need > MaxLibraryLen { // до распаковки: по размерам манифеста
		return nil, ErrLibFull
	}
	if err := s.stageItems(st, src); err != nil {
		st.Close()
		return nil, err
	}
	s.checkTplLibs(st)
	if len(st.items) == 0 {
		return st, ErrPackEmpty
	}
	return st, nil
}

// stageItems — предметы по одному; ошибка — только битый архив или диск.
func (s *Store) stageItems(st *Staged, src PackSource) error {
	seen := map[string]bool{}
	for i, it := range st.m.Items {
		name, label := cleanName(it.Name), cleanName(it.Name)
		if name == "" { // плитка — «Без названия», тост — по имени файла
			name, label = noName, cleanName(it.File)
		}
		var (
			si  stagedItem
			why string
			err error
		)
		switch it.Kind {
		case KindBg, KindSticker, KindFrame:
			si, why, err = s.stageImage(i, it, src)
		case KindSpread:
			si, why, err = stageTpl(i, src)
		case KindPrint:
			si, why, err = s.stagePrint(i, src, st)
		case KindAlbum:
			si, why, err = stageLook(i, it, src)
		default:
			why = "незнакомый вид — нужна новая версия программы"
		}
		if err != nil {
			return err
		}
		if why == "" && seen[it.Kind+si.key] {
			why = "повтор в наборе"
		}
		if why != "" {
			if si.tmp != "" {
				_ = os.Remove(si.tmp)
			}
			st.Skipped = append(st.Skipped, Skip{Name: label, Why: why})
			continue
		}
		seen[it.Kind+si.key] = true
		si.kind, si.theme, si.name = it.Kind, packTheme(it.Theme), name
		si.topic = st.topicOf(it.Topic)
		if si.src == "" && it.Kind != KindSpread && it.Kind != KindPrint { // ключ картинки и макета — sha байт
			si.src = strings.TrimPrefix(si.key, libPrefix)
		}
		st.items = append(st.items, si)
	}
	return nil
}

// stageImage — картинка набора во временный файл папки вида, sha256 по дороге (ключ), затем
// проверки «Загрузить своё» (checkCopy) и пределы набора. why — причина пропуска.
func (s *Store) stageImage(i int, it PackItem, src PackSource) (si stagedItem, why string, err error) {
	dir := filepath.Join(s.root, kindDirs[it.Kind])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return si, "", paths.NoPath(err)
	}
	f, err := os.CreateTemp(dir, tmpPrefix+"*.tmp")
	if err != nil {
		return si, "", paths.NoPath(err)
	}
	si.tmp = f.Name()
	h := sha256.New()
	err = src.Extract(i, io.MultiWriter(f, h))
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(si.tmp)
		return stagedItem{}, "", paths.NoPath(err)
	}
	sum := h.Sum(nil)
	si.key = libPrefix + hex.EncodeToString(sum[:8])
	info, ext, err := checkCopy(si.tmp)
	if err != nil {
		return si, err.Error(), nil
	}
	if fileExt(it.File) != ext {
		return si, "содержимое не совпадает с расширением файла", nil
	}
	if it.Kind == KindBg && int64(info.W)*int64(info.H) > packMaxBgPixels {
		return si, fmt.Sprintf("фон больше %d Мп", packMaxBgPixels/1_000_000), nil
	}
	if it.Kind == KindSticker && max(info.W, info.H) > packMaxSide {
		return si, fmt.Sprintf("стикер больше %d px по стороне", packMaxSide), nil
	}
	if it.Kind == KindFrame {
		if ext != "png" {
			return si, errNeedPNG.Error(), nil
		}
		if max(info.W, info.H) > packMaxSide {
			return si, fmt.Sprintf("рамочка больше %d px по стороне", packMaxSide), nil
		}
		win, err := readWindow(si.tmp) // окно проверяется при установке, как при «Загрузить своё…»
		if err != nil {
			return si, err.Error(), nil
		}
		si.win = win
	}
	si.ext, si.w, si.h, si.bytes = ext, info.W, info.H, info.Size
	return si, "", nil
}

// fileExt — расширение имени записи как у файла библиотеки (jpeg → jpg).
func fileExt(name string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	if ext == "jpeg" {
		return "jpg"
	}
	return ext
}

// stageTpl — шаблон разворота в память: ParseSpreadTpl (зажим), генераторы известны этой
// версии, канонический JSON и ключ TplKey (тот же шаблон не дублируется).
func stageTpl(i int, src PackSource) (si stagedItem, why string, err error) {
	var buf capBuffer
	if err := src.Extract(i, &buf); err != nil {
		return si, "", err
	}
	t, err := looktpl.ParseSpreadTpl(buf.Bytes())
	if errors.Is(err, looktpl.ErrTplNewer) || err == nil && !tplKnown(t) {
		return si, "шаблон из новой версии программы", nil
	}
	if err != nil {
		return si, "шаблон разворота не читается", nil
	}
	data, err := json.Marshal(t)
	if err != nil {
		return si, "шаблон разворота не читается", nil
	}
	si.key, si.ext, si.tpl, si.libs, si.bytes = looktpl.TplKey(t), "json", data, tplLibs(t), int64(len(data))
	sum := sha256.Sum256(buf.Bytes()) // байты записи: Extract уже сверил их с sha256 манифеста
	si.src = hex.EncodeToString(sum[:8])
	return si, "", nil
}

// capBuffer — буфер шаблона не больше MaxTplFile + 1: предел и без доверия к источнику.
type capBuffer struct{ bytes.Buffer }

func (b *capBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > looktpl.MaxTplFile+1 {
		return 0, errors.New("шаблон разворота больше предела")
	}
	return b.Buffer.Write(p)
}

// tplKnown — все узоры, рисунки и встроенные рамочки шаблона знакомы этой версии (зажим
// незнакомый ключ оставляет).
func tplKnown(t looktpl.SpreadTpl) bool {
	bgs := slices.Clone(t.Page)
	if t.Whole != nil {
		bgs = append(bgs, *t.Whole)
	}
	for _, b := range bgs {
		if b.Pat != nil && !gen.Known(b.Pat.Kind) {
			return false
		}
	}
	for _, it := range t.Stickers {
		if it.Art != nil && !gen.Known(it.Art.Kind) {
			return false
		}
	}
	for _, b := range t.Borders {
		if b.Kind != "" && !gen.FrameKnown(b.Kind) {
			return false
		}
	}
	return true
}

// tplLibs — картинки библиотеки, на которые ссылается шаблон.
func tplLibs(t looktpl.SpreadTpl) []libRef {
	var out []libRef
	bgs := slices.Clone(t.Page)
	if t.Whole != nil {
		bgs = append(bgs, *t.Whole)
	}
	for _, b := range bgs {
		if b.Lib != "" {
			out = append(out, libRef{KindBg, b.Lib})
		}
	}
	for _, it := range t.Stickers {
		if it.Lib != "" {
			out = append(out, libRef{KindSticker, it.Lib})
		}
	}
	for _, b := range t.Borders {
		if b.Lib != "" {
			out = append(out, libRef{KindFrame, b.Lib})
		}
	}
	return out
}

// checkTplLibs — ссылка шаблона (и макета альбома — checkLookLibs 131 тем же правилом) только на
// картинку этого набора нужного вида или уже стоящую в библиотеке (design §4.2): ссылка на чужое —
// шаблон или макет пропускается.
func (s *Store) checkTplLibs(st *Staged) {
	have := map[libRef]bool{}
	for _, it := range st.items {
		if it.tpl == nil {
			have[libRef{it.kind, it.key}] = true
			have[libRef{"", it.key}] = true // ключ сырых байт макета: вид неизвестен — любой
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	st.items = slices.DeleteFunc(st.items, func(it stagedItem) bool {
		for _, ref := range it.libs {
			if !have[ref] && !s.haveFile(ref) {
				why := "шаблон ссылается на фон, стикер или рамочку не из набора"
				if it.kind == KindAlbum {
					why = "макет ссылается на картинку не из набора"
				}
				st.Skipped = append(st.Skipped, Skip{Name: it.name, Why: why})
				return true
			}
		}
		return false
	})
}

// haveFile — файл ключа ref есть в библиотеке; вид "" — любого вида. Под mu.
func (s *Store) haveFile(ref libRef) bool {
	if ref.kind != "" {
		_, ok := s.files[ref.kind][ref.key]
		return ok
	}
	for _, m := range s.files {
		if _, ok := m[ref.key]; ok {
			return true
		}
	}
	return false
}
