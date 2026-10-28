package decor

import (
	"cmp"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Наборы библиотеки (спека 067, design §4.3–§4.5): файлы набора лежат в общих папках видов по
// ключу (решение 8) — искатель 048 находит их и в 0.15; принадлежность — реестр Наборы/<id>.json,
// по файлу на набор: library.json 0.15 переписывает своей структурой, поле «набор» там потерялось бы.

const (
	packsDir   = "Наборы"
	packRegMax = 1 << 20 // 1000 предметов — сотни КБ
	packRegV   = 1
	// BuiltinPack — строка встроенного в окне «Наборы»: не по грамматике id, с набором не совпадёт.
	BuiltinPack = "sa:builtin"
)

// Откуда набор (PackInfo.Origin).
const (
	OriginBuiltin = "builtin"
	OriginFile    = "file"
	OriginNet     = "net"
)

// packReg — реестр одного набора; Items — в порядке манифеста.
type packReg struct {
	V         int    `json:"v"`
	ID        string `json:"id"`
	Version   int    `json:"version"`
	Title     string `json:"title"`
	About     string `json:"about"`
	Author    string `json:"author"`
	License   string `json:"license"`
	Source    string `json:"source"`
	Origin    string `json:"origin"`
	MinApp    string `json:"minApp,omitempty"` // x.y.z; нужен слою типографий (понизили программу — набор не действует)
	Installed int64  `json:"installed"`        // unix мс; порядок наборов в каталоге
	// Base — граница первой партии (133 §4): пишет RemovePack оставшимся наборам, чтобы удаление самого
	// раннего не сдвинуло её. 0.27 поле не читает и при обновлении набора теряет — граница тогда по Installed.
	Base  int64       `json:"base,omitempty"`
	Zip   string      `json:"zip"` // sha256 архива
	Items []packEntry `json:"items"`
}

type packEntry struct {
	Key   string `json:"key"`
	Kind  string `json:"kind"`
	Theme string `json:"theme"`
	Name  string `json:"name"`
	W     int    `json:"w"`
	H     int    `json:"h"`
	Bytes int64  `json:"bytes"`
	Item  string `json:"item,omitempty"` // макет (131): путь записи в манифесте — поиск после обновления набора
}

// PackInfo — строка окна «Наборы библиотеки».
type PackInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	About     string `json:"about"`
	Author    string `json:"author"`
	License   string `json:"license"`
	Source    string `json:"source"` // текст, не ссылка (решение 14)
	Origin    string `json:"origin"` // builtin | file | net
	Version   int    `json:"version"`
	Bg        int    `json:"bg"`
	Sticker   int    `json:"sticker"`
	Spread    int    `json:"spread"`
	Frame     int    `json:"frame"`
	Print     int    `json:"print"` // 123: профили типографий (.saprint)
	Bytes     int64  `json:"bytes"`
	Removable bool   `json:"removable"`
}

// validPackID — ^[a-z0-9][a-z0-9-]{2,39}$ (как pack.ValidID: decor не импортирует pack), не
// устройство Windows: из id строится имя реестра Наборы/<id>.json, а «con.json» — устройство.
func validPackID(id string) bool {
	if len(id) < 3 || len(id) > 40 || id[0] == '-' || paths.Device(id) {
		return false
	}
	for i := range len(id) {
		if c := id[i]; (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

// loadPacks читает Наборы/*.json; битый реестр — пропуск и строка в log.txt (файлы набора
// находит Find — альбомы не ломаются). Остатки записи и скачивания (*.tmp) убираются.
func (s *Store) loadPacks() {
	dir := filepath.Join(s.root, packsDir)
	list, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, de := range list {
		name := de.Name()
		if strings.HasSuffix(name, ".tmp") {
			_ = os.Remove(filepath.Join(dir, name))
			continue
		}
		id, ok := strings.CutSuffix(name, ".json")
		if !ok || !validPackID(id) || !de.Type().IsRegular() {
			continue
		}
		reg, err := readPackReg(filepath.Join(dir, name), id)
		if err != nil {
			log.Printf("библиотека: реестр набора %s не читается (%v) — пропущен", id, paths.NoPath(err))
			continue
		}
		s.packs = append(s.packs, reg)
	}
	slices.SortStableFunc(s.packs, func(a, b *packReg) int {
		return cmp.Or(cmp.Compare(a.Installed, b.Installed), cmp.Compare(a.ID, b.ID))
	})
}

// readPackReg — реестр набора с зажимом: правят руками, а размеры идут в квоту библиотеки.
func readPackReg(path, id string) (*packReg, error) {
	data, err := readCapped(path, packRegMax)
	if err != nil {
		return nil, err
	}
	var reg packReg
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, errors.New("не JSON")
	}
	if reg.V < 1 || reg.ID != id {
		return nil, errors.New("нет версии или чужой id")
	}
	reg.Version = max(reg.Version, 1)
	if !validVer(reg.MinApp) {
		reg.MinApp = ""
	}
	if reg.Title = cleanName(reg.Title); reg.Title == "" {
		reg.Title = id
	}
	reg.About, reg.Author = cleanText(reg.About, 300), cleanName(reg.Author)
	reg.License, reg.Source = cleanText(reg.License, 40), cleanText(reg.Source, 200)
	if !dateOK(reg.Base) {
		reg.Base = 0 // мусор и сброшенные часы — не граница
	}
	if reg.Origin != OriginNet {
		reg.Origin = OriginFile
	}
	items, seen := reg.Items, map[string]bool{}
	reg.Items = make([]packEntry, 0, len(items))
	for _, e := range items {
		if !model.ValidLibKey(e.Key) || kindDirs[e.Kind] == "" || seen[e.Kind+e.Key] {
			continue
		}
		seen[e.Kind+e.Key] = true
		e.Theme = packTheme(e.Theme)
		if e.Name = cleanName(e.Name); e.Name == "" {
			e.Name = noName
		}
		e.W, e.H = min(max(e.W, 0), imgfile.MaxPixels), min(max(e.H, 0), imgfile.MaxPixels)
		e.Bytes = min(max(e.Bytes, 0), MaxBgBytes)
		if e.Kind != KindAlbum || !ValidItemPath(e.Item) {
			e.Item = "" // путь только у макета и по грамматике манифеста: в ФС он не идёт
		}
		reg.Items = append(reg.Items, e)
	}
	return &reg, nil
}

// packTheme — тема предмета набора: из списка 048, «Мои» — только у своих.
func packTheme(th string) string {
	if !isTheme(th) || th == ThemeMine {
		return "basic"
	}
	return th
}

// packOf — набор с id; под mu.
func (s *Store) packOf(id string) *packReg {
	for _, r := range s.packs {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// inPack — ключ вида kind есть в каком-нибудь наборе; под mu.
func (s *Store) inPack(kind, key string) bool {
	for _, r := range s.packs {
		for _, e := range r.Items {
			if e.Kind == kind && e.Key == key {
				return true
			}
		}
	}
	return false
}

// heldBy — ключ нужен ещё кому-то, кроме набора except: своему (и скрытому — решение 0.12 №80)
// или другому набору; под mu.
func (s *Store) heldBy(key string, except *packReg) bool {
	for _, e := range s.entries {
		if e.Key == key {
			return true
		}
	}
	for _, r := range s.packs {
		if r == except {
			continue
		}
		for _, e := range r.Items {
			if e.Key == key {
				return true
			}
		}
	}
	return false
}

// packItems — плитки наборов вида kind (под mu): наборы от нового к старому (ничья по installed —
// по id), внутри — порядок манифеста (133 §3). Ключ в двух наборах — одна плитка у старшего
// (по installed), порядок вывода на владение не влияет; файла нет — нет плитки.
func (s *Store) packItems(kind string) []Item {
	type block struct {
		reg   *packReg
		items []Item
	}
	var blocks []block
	seen := map[string]bool{}
	for _, r := range s.packs { // s.packs — по installed возр.: первый набор берёт ключ
		b := block{reg: r}
		for _, e := range r.Items {
			if e.Kind != kind || seen[e.Key] {
				continue
			}
			if _, ok := s.files[kind][e.Key]; !ok {
				continue
			}
			seen[e.Key] = true
			b.items = append(b.items, e.item(r))
		}
		blocks = append(blocks, b)
	}
	slices.SortStableFunc(blocks, func(a, b block) int {
		return cmp.Or(cmp.Compare(b.reg.Installed, a.reg.Installed), cmp.Compare(a.reg.ID, b.reg.ID))
	})
	var out []Item
	for _, b := range blocks {
		out = append(out, b.items...)
	}
	return out
}

// item — плитка предмета набора: превью — как у своего, по ключу (/media/lib); шаблон и макет рисует
// Go (/media/tpl, /media/look), страницы, места и карточку — Catalog по файлу.
func (e packEntry) item(r *packReg) Item {
	it := Item{ID: e.Key, Theme: e.Theme, Name: e.Name, URL: LibURL(e.Key, false), W: e.W, H: e.H, Pack: r.Title, PackID: r.ID}
	switch e.Kind {
	case KindSpread:
		it.URL = TplURL(e.Key)
	case KindAlbum:
		it.URL = LookURL(e.Key)
	case KindBg, KindSticker, KindFrame: // peek — только у картинок (JSON макета не картинка)
		it.Peek = LibURL(e.Key, true)
	}
	if e.W > 0 && e.H > 0 {
		it.Aspect = float64(e.W) / float64(e.H)
	}
	markForm(&it, e.Kind)
	return it
}

// Packs — строки окна «Наборы»: встроенные (не удаляются), затем наборы по installed.
func (s *Store) Packs() []PackInfo {
	out := []PackInfo{{ID: BuiltinPack, Title: "Встроенные", Author: "Sosnina Albums", Origin: OriginBuiltin,
		Version: 1, Bg: knownPresets(bgPresets), Sticker: knownPresets(stickerPresets), Spread: len(spreadBuiltins(KindSpread)), Frame: knownFrames()}}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, r := range s.packs {
		out = append(out, r.info())
	}
	return out
}

func knownPresets(list []preset) int {
	n := 0
	for _, p := range list {
		if gen.Known(p.spec.Kind) {
			n++
		}
	}
	return n
}

func (r *packReg) info() PackInfo {
	p := PackInfo{ID: r.ID, Title: r.Title, About: r.About, Author: r.Author, License: r.License, Source: r.Source,
		Origin: r.Origin, Version: r.Version, Removable: true}
	for _, e := range r.Items {
		switch e.Kind {
		case KindBg:
			p.Bg++
		case KindSticker:
			p.Sticker++
		case KindSpread:
			p.Spread++
		case KindFrame:
			p.Frame++
		case KindPrint:
			p.Print++
		}
		p.Bytes += e.Bytes
	}
	return p
}

// packPath — файл предмета по ключу в папке вида (имя только от ключа).
func (s *Store) packPath(kind, key, ext string) string {
	return filepath.Join(s.root, kindDirs[kind], strings.TrimPrefix(key, libPrefix)+"."+ext)
}

// indexFile — файл в индекс ключ → путь; под mu на запись (или до публикации Store).
func (s *Store) indexFile(kind, key, path string) {
	if s.files[kind] == nil {
		s.files[kind] = map[string]string{}
	}
	s.files[kind][key] = path
}

func regularFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// Used — байты библиотеки (свои + наборы): App сверяет квоту по размеру из каталога до
// скачивания (design §4.2), а не после 300 МБ трафика.
func (s *Store) Used() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.libBytes()
}

// DownloadPath — временный файл скачивания набора id: Наборы/.dl-<id>.tmp (остаток оборванного
// убирает loadPacks при запуске). Имя только из id по грамматике.
func (s *Store) DownloadPath(id string) (string, error) {
	if !validPackID(id) {
		return "", errors.New("недопустимый id набора")
	}
	dir := filepath.Join(s.root, packsDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", errors.New("папка наборов недоступна")
	}
	return filepath.Join(dir, ".dl-"+id+".tmp"), nil
}
