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

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Установка и удаление наборов (design §4.3, §4.5). Разбор архива — пакет pack (App переносит
// манифест сюда копией: decor не импортирует pack — depguard); здесь — содержимое (картинка,
// шаблон), запись под ключами и сборка мусора.

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

var (
	// ErrPackEmpty — ни один предмет набора не прошёл проверку.
	ErrPackEmpty = errors.New("в наборе нечего ставить")
	errPackOlder = errors.New("уже стоит версия набора новее")
	errPackSame  = errors.New("этот набор уже стоит")
	// ErrLibFull — своё и наборы вместе дошли до MaxLibraryLen.
	ErrLibFull = fmt.Errorf("библиотека заполнена (%d ГБ)", MaxLibraryLen>>30)
)

// PackManifest — проверенный манифест набора (pack.Manifest без пропущенных предметов).
type PackManifest struct {
	ID, Title, About, Author, License, Source string
	Version                                   int
	Items                                     []PackItem
}

// PackItem — предмет манифеста: имя записи (для расширения и тоста), вид, тема, имя, размер.
type PackItem struct {
	File, Kind, Theme, Name string
	Size                    int64
}

// PackSource — предмет i манифеста потоком со сверкой размера и sha256 (pack.Archive.Extract).
// Ошибка — архив битый или подменён: набор не ставится целиком.
type PackSource interface {
	Extract(i int, w io.Writer) error
}

// Staged — набор, распакованный во временные файлы и проверенный, до записи в библиотеку;
// держит временные файлы до InstallPack или Close.
type Staged struct {
	Ask     string // "" — ставить; AskSame, AskUpdate, AskOlder — набор с этим id уже стоит
	From    int    // версия стоящего
	Skipped []Skip

	m      PackManifest
	origin string
	zip    string
	items  []stagedItem
}

type stagedItem struct {
	kind, key, theme, name, ext string
	tmp                         string // картинка: временный файл в папке вида
	tpl                         []byte // шаблон: канонический JSON (ключ — TplKey)
	libs                        []libRef
	w, h                        int
	bytes                       int64
}

// libRef — ссылка шаблона на картинку библиотеки: фон (TplBg.Lib) или стикер (TplItem.Lib).
type libRef struct{ kind, key string }

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
		Origin: st.origin, Installed: installed, Zip: st.zip, Items: make([]packEntry, 0, len(st.items))}
	if r.Title == "" {
		r.Title = st.m.ID
	}
	for _, it := range st.items {
		r.Items = append(r.Items, packEntry{Key: it.key, Kind: it.kind, Theme: it.theme, Name: it.name, W: it.w, H: it.h, Bytes: it.bytes})
	}
	return r
}

// StagePack — проверка содержимого набора во временные файлы (без записи в библиотеку).
// origin — OriginFile | OriginNet, zipSum — sha256 архива. Набор с тем же id уже стоит — Ask
// без распаковки (AskUpdate распаковывается: его ставит ConfirmPack). Битый архив — ошибка,
// временные удалены; плохой предмет — Skipped; не осталось ни одного — ErrPackEmpty (Staged
// с причинами всё равно возвращается).
func (s *Store) StagePack(m PackManifest, src PackSource, origin, zipSum string) (*Staged, error) {
	if !validPackID(m.ID) {
		return nil, errors.New("недопустимый id набора")
	}
	if origin != OriginNet {
		origin = OriginFile
	}
	st := &Staged{m: m, origin: origin, zip: zipSum, Skipped: []Skip{}}
	s.mu.RLock()
	ro, old, used := s.readOnly, s.packOf(m.ID), s.libBytes()
	s.mu.RUnlock()
	if ro {
		return nil, ErrReadOnly
	}
	if old != nil {
		st.From = old.Version
		switch {
		case m.Version == old.Version:
			st.Ask = AskSame
			return st, nil
		case m.Version < old.Version:
			st.Ask = AskOlder
			return st, nil
		}
		st.Ask = AskUpdate
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
		case KindBg, KindSticker:
			si, why, err = s.stageImage(i, it, src)
		case KindSpread:
			si, why, err = stageTpl(i, src)
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
	t, err := model.ParseSpreadTpl(buf.Bytes())
	if errors.Is(err, model.ErrTplNewer) || err == nil && !tplKnown(t) {
		return si, "шаблон из новой версии программы", nil
	}
	if err != nil {
		return si, "шаблон разворота не читается", nil
	}
	data, err := json.Marshal(t)
	if err != nil {
		return si, "шаблон разворота не читается", nil
	}
	si.key, si.ext, si.tpl, si.libs, si.bytes = model.TplKey(t), "json", data, tplLibs(t), int64(len(data))
	return si, "", nil
}

// capBuffer — буфер шаблона не больше MaxTplFile + 1: предел и без доверия к источнику.
type capBuffer struct{ bytes.Buffer }

func (b *capBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > model.MaxTplFile+1 {
		return 0, errors.New("шаблон разворота больше предела")
	}
	return b.Buffer.Write(p)
}

// tplKnown — все узоры и рисунки шаблона знакомы этой версии (зажим незнакомый ключ оставляет).
func tplKnown(t model.SpreadTpl) bool {
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
	return true
}

// tplLibs — картинки библиотеки, на которые ссылается шаблон.
func tplLibs(t model.SpreadTpl) []libRef {
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
	return out
}

// checkTplLibs — ссылка шаблона только на картинку этого набора нужного вида или уже стоящую
// в библиотеке (design §4.2): ссылка на чужое — шаблон пропускается.
func (s *Store) checkTplLibs(st *Staged) {
	have := map[libRef]bool{}
	for _, it := range st.items {
		if it.tpl == nil {
			have[libRef{it.kind, it.key}] = true
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	st.items = slices.DeleteFunc(st.items, func(it stagedItem) bool {
		for _, ref := range it.libs {
			if _, ok := s.files[ref.kind][ref.key]; !ok && !have[ref] {
				st.Skipped = append(st.Skipped, Skip{Name: it.name, Why: "шаблон ссылается на фон или стикер не из набора"})
				return true
			}
		}
		return false
	})
}

// libBytes — байты библиотеки (свои + наборы, ключ считается один раз); под mu. Насыщается
// за пределом: реестры правят руками.
func (s *Store) libBytes() int64 {
	seen := map[string]bool{}
	var total int64
	add := func(kind, key string, n int64) {
		if !seen[kind+key] {
			seen[kind+key] = true
			total += min(max(n, 0), MaxLibraryLen+1-total)
		}
	}
	for _, e := range s.entries {
		add(e.Kind, e.Key, e.Bytes)
	}
	for _, r := range s.packs {
		for _, e := range r.Items {
			add(e.Kind, e.Key, e.Bytes)
		}
	}
	return total
}

// packStep — крючок теста атомарности: сбой на предмете i (в работе — никогда).
var packStep = func(int) error { return nil }

// InstallPack записывает проверенный набор (design §4.3): файлы под именами от ключей (файл с
// тем же ключом есть — байты те же, временный удаляется), последним — реестр Наборы/<id>.json
// (точка фиксации). Сбой до неё — новые файлы удаляются, библиотека прежняя. Обновление —
// ключи прежней версии, которых нет в новой, уходят сборкой мусора; keep — нужны открытому
// альбому. Квота сверяется заново (ErrLibFull). Временные файлы st после успеха пусты; Close
// всё равно нужен.
func (s *Store) InstallPack(st *Staged, keep func(key string) bool) (PackInfo, error) {
	if st == nil || len(st.items) == 0 {
		return PackInfo{}, ErrPackEmpty
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	s.mu.RLock()
	ro, old, used := s.readOnly, s.packOf(st.m.ID), s.libBytes()
	var need int64 // только новые файлы: тот же ключ на диске placeItems не пишет
	for _, it := range st.items {
		if _, ok := s.files[it.kind][it.key]; !ok {
			need += it.bytes
		}
	}
	s.mu.RUnlock()
	if ro {
		return PackInfo{}, ErrReadOnly
	}
	// Повторная сверка квоты: между StagePack и «Заменить?» могли пройти минуты, а своё и другие
	// наборы — встать за это время.
	if used+need > MaxLibraryLen {
		return PackInfo{}, ErrLibFull
	}
	if old != nil && old.Version == st.m.Version {
		return PackInfo{}, errPackSame
	}
	if old != nil && old.Version > st.m.Version {
		return PackInfo{}, errPackOlder
	}
	reg := st.reg(nowMs())
	if old != nil {
		reg.Installed = old.Installed // обновление остаётся на своём месте в каталоге
	}
	placed, created, err := s.placeItems(st)
	if err == nil {
		err = s.writePackReg(reg)
	}
	if err != nil {
		for _, p := range created {
			_ = os.Remove(p)
		}
		return PackInfo{}, errors.New("набор не записался: " + paths.NoPath(err).Error())
	}
	s.mu.Lock()
	for i, it := range st.items {
		s.indexFile(it.kind, it.key, placed[i])
	}
	if old != nil {
		s.packs[slices.Index(s.packs, old)] = reg
	} else {
		s.packs = append(s.packs, reg)
	}
	s.mu.Unlock()
	if old != nil {
		have := map[string]bool{}
		for _, e := range reg.Items {
			have[e.Kind+e.Key] = true
		}
		gone := slices.DeleteFunc(slices.Clone(old.Items), func(e packEntry) bool { return have[e.Kind+e.Key] })
		s.collect(gone, keep, nil, false)
	}
	return reg.info(), nil
}

// placeItems — файлы предметов на места; created — новые (откат), placed[i] — путь предмета i.
func (s *Store) placeItems(st *Staged) (placed, created []string, err error) {
	placed = make([]string, len(st.items))
	for i := range st.items {
		it := &st.items[i]
		if err := packStep(i); err != nil {
			return nil, created, err
		}
		path := s.packPath(it.kind, it.key, it.ext)
		if !library.Under(path, s.root) { // имя от ключа, но проверка дешевле ошибки
			return nil, created, errors.New("недопустимый путь в библиотеке")
		}
		placed[i] = path
		if regularFile(path) {
			if it.tmp != "" {
				_ = os.Remove(it.tmp)
				it.tmp = ""
			}
			continue
		}
		if it.tpl != nil {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, created, err
			}
			err = library.WriteFileAtomic(path, it.tpl)
		} else {
			err = os.Rename(it.tmp, path)
		}
		if err != nil {
			return nil, created, err
		}
		it.tmp = ""
		created = append(created, path)
	}
	return placed, created, nil
}

// writePackReg — реестр набора атомарно.
func (s *Store) writePackReg(reg *packReg) error {
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Join(s.root, packsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return library.WriteFileAtomic(filepath.Join(dir, reg.ID+".json"), data)
}
