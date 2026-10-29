package agents

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Проекции для агента (design §3): наружу идёт только то, что выбрано здесь, а не json-теги model —
// новое поле модели само не попадёт. Поля с ² (только уровень ≥ 2) — omitempty: на уровне 1
// их нет в ответе.

// Пределы строк из данных альбома, знаков.
const (
	maxFile  = 120  // имена файлов, папок, альбома, названия
	maxText  = 2000 // текст на листе, подпись
	maxDesc  = 300  // описание набора
	maxLabel = 64   // метка XMP
	maxID    = 64   // id из файла альбома
)

// levelOf — уровень доступа из config.json; папки нет — минимальный.
func levelOf(dir string) int {
	if dir == "" {
		return prefs.AgentsMinLevel
	}
	return prefs.AgentsOf(dir).Level
}

// names — уровень ≥ 2: имена файлов, названия и тексты.
func (c *call) names() bool { return c.level >= 2 }

// text — строка из данных на этом уровне: на уровне 1 пусто (omitempty уберёт поле).
func (c *call) text(s string, limit int) string {
	if !c.names() {
		return ""
	}
	if limit >= maxText {
		return cleanText(s, limit)
	}
	return cleanLine(s, limit)
}

// keyRune — знак грамматики ключа раскладки: буквы, цифры, скобки, «|», «:», «,», «-».
func keyRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("()[]|:,-", r)
}

// cleanLine — однострочное имя: как cleanText, но переносы строк (бывают в именах файлов на Linux)
// становятся пробелами.
func cleanLine(s string, limit int) string {
	return strings.NewReplacer("\n", " ").Replace(cleanText(s, limit))
}

// cleanKey — ключ раскладки из файла альбома: только знаки грамматики ключа (layout/keyparse.go),
// не длиннее layout.MaxKeyLen; иное — пусто.
func cleanKey(s string) string {
	if len(s) > layout.MaxKeyLen {
		return ""
	}
	for _, r := range s {
		if !keyRune(r) {
			return ""
		}
	}
	return s
}

// cleanText — строка из данных альбома, файла или набора для агента: без знаков категорий Unicode
// Cc и Cf, кроме \n (управляющие, невидимые теги U+E0000…, bidi U+202A…, нулевой ширины), с
// обрезкой до limit знаков и пометкой «…». Битый UTF-8 пропускается.
func cleanText(s string, limit int) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		switch {
		case r == ' ' || r == ' ':
			r = '\n'
		case r == '\n':
		case r == '�' || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || hiddenRune(r):
			continue
		}
		if n == limit {
			b.WriteString("…")
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// hiddenRune — невидимые знаки вне Cc и Cf, которыми прячут нагрузку: селекторы вариантов
// (U+FE00…, U+E0100…), личное использование (Co) и заполнители хангыля.
func hiddenRune(r rune) bool {
	return unicode.Is(unicode.Variation_Selector, r) || unicode.Is(unicode.Co, r) || r == '\u3164' || r == '\u115F' || r == '\u1160'
}

// cleanID — id из файла альбома: он может быть любой строкой, наружу идут только безопасные знаки.
func cleanID(s string) string {
	b := make([]rune, 0, min(len(s), maxID))
	for _, r := range s {
		if len(b) == maxID {
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == ':', r == '.', r == '-':
			b = append(b, r)
		default:
			b = append(b, '_')
		}
	}
	return string(b)
}

// albumRef — какой это альбом и в каком состоянии: сменился key — открыли другой, агент
// перечитывает; сменился rev — альбом правили.
type albumRef struct {
	Key string `json:"key" jsonschema:"меняется при открытии другого альбома"`
	Rev int    `json:"rev" jsonschema:"растёт с каждой правкой альбома"`
}

// meta — шапка каждого ответа.
type meta struct {
	Album albumRef `json:"album"`
	Level int      `json:"level" jsonschema:"1 — устройство альбома, 2 — ещё имена и тексты, 3 — ещё картинки"`
}

// keySalt — соль процесса: ключ альбома не повторяется между запусками и не раскрывает номер Editor.
var keySalt = func() (b [8]byte) {
	if _, err := rand.Read(b[:]); err != nil { // нет случайных чисел — ключ хуже, но служба жива
		binary.LittleEndian.PutUint64(b[:], 1)
	}
	return b
}()

// albumKey — ключ альбома по номеру Editor; 0 — альбома нет.
func albumKey(ed uint64) string {
	if ed == 0 {
		return ""
	}
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], ed)
	sum := sha256.Sum256(append(keySalt[:], n[:]...))
	return "e" + hex.EncodeToString(sum[:6])
}

// view — вид открытого альбома; альбома нет (edID 0) — errNoAlbum.
func (c *call) view() (model.View, uint64, error) {
	v, ed := c.s.h.View()
	if ed == 0 {
		return model.View{}, 0, errNoAlbum
	}
	return v, ed, nil
}

// meta — шапка ответа; альбома нет — пустой key.
func (c *call) meta() meta {
	v, ed := c.s.h.View()
	return meta{Album: albumRef{Key: albumKey(ed), Rev: v.Rev}, Level: c.level}
}

// metaOf — шапка по уже снятому виду (ответ целиком об одном состоянии).
func (c *call) metaOf(v model.View, ed uint64) meta {
	return meta{Album: albumRef{Key: albumKey(ed), Rev: v.Rev}, Level: c.level}
}

// usedIDs — ID файлов фото, стоящих на разворотах и обложке (без фонов и стикеров).
func usedIDs(v model.View) map[string]bool {
	used := map[string]bool{}
	add := func(sv model.SpreadView) {
		for _, id := range sv.Photos {
			used[model.BaseID(id)] = true
		}
	}
	for _, sv := range v.Spreads {
		add(sv)
	}
	if v.Cover != nil {
		add(v.Cover.Sheet)
	}
	return used
}

// folderIndex — фото по папкам: путь фото → номер папки альбома с 1 (самая глубокая подходящая), 0 — ни в одной.
type folderIndex struct{ dirs []string }

func newFolderIndex(folders []string) folderIndex {
	d := make([]string, len(folders))
	for i, f := range folders {
		d[i] = paths.Fold(filepath.Clean(f)) + string(filepath.Separator)
	}
	return folderIndex{d}
}

func (x folderIndex) of(path string) int {
	p, best, n := paths.Fold(filepath.Clean(path)), 0, 0
	for i, d := range x.dirs {
		if strings.HasPrefix(p, d) && len(d) > best {
			best, n = len(d), i+1
		}
	}
	return n
}

// programOut — сведения о программе в сводке.
type programOut struct {
	Version string `json:"version"`
	Level   int    `json:"level"`
}

type pageOut struct {
	W      float64 `json:"w" jsonschema:"ширина страницы, мм"`
	H      float64 `json:"h" jsonschema:"высота страницы, мм"`
	DPI    int     `json:"dpi"`
	Margin float64 `json:"margin"`
	Gap    float64 `json:"gap"`
	Safe   float64 `json:"safe"`
	Bleed  float64 `json:"bleed"`
	Crop   int     `json:"crop"`
	Fold   string  `json:"fold" jsonschema:"фото через корешок: off, wide (только широкие), any"`
}

type coverOut struct {
	Preset string  `json:"preset"`
	Layout string  `json:"layout"`
	Spine  float64 `json:"spine" jsonschema:"корешок, мм"`
	N      int     `json:"n" jsonschema:"разворотов"`
	Over   bool    `json:"over" jsonschema:"разворотов больше, чем в таблице пресета"`
}

type countsOut struct {
	Spreads      int `json:"spreads"`
	PhotosUsed   int `json:"photosUsed"`
	PhotosTotal  int `json:"photosTotal"`
	Unused       int `json:"unused"`
	Missing      int `json:"missing"`
	Unreadable   int `json:"unreadable"`
	EmptyPlaces  int `json:"emptyPlaces"`
	LowDPI       int `json:"lowDPI"`
	Drafts       int `json:"drafts"`
	Bins         int `json:"bins"`
	FontsMissing int `json:"fontsMissing"`
}

type pageLimitOut struct {
	Who   string `json:"who,omitempty"`
	Count int    `json:"count"`
	Min   int    `json:"min"`
	Max   int    `json:"max"`
	Over  bool   `json:"over" jsonschema:"число разворотов вне предела типографии"`
}

type lookOut struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Found bool   `json:"found"`
}

type folderOut struct {
	N      int    `json:"n"`
	Name   string `json:"name,omitempty"`
	Photos int    `json:"photos"`
}

type albumOut struct {
	meta
	Program      programOut    `json:"program"`
	Name         string        `json:"name" jsonschema:"на уровне 1 всегда «Альбом»"`
	Saved        bool          `json:"saved" jsonschema:"у альбома есть файл"`
	Dirty        bool          `json:"dirty" jsonschema:"есть несохранённые правки"`
	Product      string        `json:"product" jsonschema:"album или sheet (отдельные листы)"`
	Page         pageOut       `json:"page"`
	Cover        *coverOut     `json:"cover"`
	Counts       countsOut     `json:"counts"`
	PageLimit    *pageLimitOut `json:"pageLimit"`
	Look         *lookOut      `json:"look"`
	Folders      []folderOut   `json:"folders"`
	Unfilled     []string      `json:"unfilled" jsonschema:"пустые поля шаблонов в печатаемом тексте"`
	Unnamed      int           `json:"unnamed" jsonschema:"фото с подписью-шаблоном без имени"`
	FontsMissing []string      `json:"fontsMissing,omitempty"`
}

// coverPreset — изделие обложки; id чужого профиля типографии на уровне 1 скрыт так же, как в
// list_print_profiles (в нём бывает имя своего профиля).
func (c *call) coverPreset(p string) string {
	if !c.names() {
		for id, n := range c.hiddenPrints() {
			if p == id || strings.HasPrefix(p, id+"-") {
				return ownPrintID(n)
			}
		}
	}
	return cleanID(p)
}

func foldMode(s model.Settings) string {
	switch {
	case !s.Layflat:
		return "off"
	case s.FoldAny:
		return "any"
	}
	return "wide"
}

// unnamedAlbum — имя альбома без файла: так же его зовёт интерфейс (collect.ts, folderStub) и
// так отвечает уровень 1; на уровнях 2–3 больше нет второго слова для безымянного (0.31-tails-3).
const unnamedAlbum = "Альбом"

// albumName — имя альбома по файлу: без папки и расширения; файла нет — «Альбом».
func albumName(path string) string {
	if path == "" {
		return unnamedAlbum
	}
	b := filepath.Base(path)
	return strings.TrimSuffix(b, filepath.Ext(b))
}

func (c *call) getAlbum() (albumOut, error) {
	v, ed, err := c.view()
	if err != nil {
		return albumOut{}, err
	}
	photos := c.s.h.Photos()
	missing, unread := c.s.h.Status()
	used := usedIDs(v)
	back := map[string]bool{}
	for _, id := range v.BackdropUsed {
		back[model.BaseID(id)] = true
	}
	unused := 0
	for _, p := range photos {
		if !used[p.ID] && !back[p.ID] {
			unused++
		}
	}
	idx := newFolderIndex(v.Folders)
	perFolder := make([]int, len(v.Folders)+1)
	for _, p := range photos {
		perFolder[idx.of(p.Path)]++
	}
	s := v.Settings
	pg := s.Page() // ширина страницы — через Geo (TestPageWReaders), не PageW разворота
	out := albumOut{
		meta: c.metaOf(v, ed), Program: programOut{Version: c.s.h.Version(), Level: c.level},
		Name: unnamedAlbum, Saved: v.Path != "", Dirty: v.Dirty, Product: "album",
		Page: pageOut{W: pg.W, H: pg.H, DPI: s.DPI, Margin: s.Margin, Gap: s.Gap, Safe: s.Safe, Bleed: s.Bleed, Crop: s.Crop, Fold: foldMode(s)},
		Counts: countsOut{Spreads: len(v.Spreads), PhotosUsed: len(used), PhotosTotal: len(photos), Unused: unused,
			Missing: len(missing), Unreadable: len(unread), EmptyPlaces: v.EmptyPlaces, LowDPI: v.LowDPI, Drafts: v.Drafts, Bins: len(v.Bins)},
		Folders:  make([]folderOut, len(v.Folders)),
		Unfilled: make([]string, 0, len(v.Unfilled)),
		Unnamed:  v.Unnamed,
	}
	if s.Sheet {
		out.Product = "sheet"
	}
	if c.names() {
		out.Name = cleanLine(albumName(v.Path), maxFile)
	}
	for _, u := range v.Unfilled {
		out.Unfilled = append(out.Unfilled, cleanLine(u, maxFile))
	}
	for i, f := range v.Folders {
		out.Folders[i] = folderOut{N: i + 1, Name: c.text(filepath.Base(f), maxFile), Photos: perFolder[i+1]}
	}
	if cv := v.Cover; cv != nil {
		out.Cover = &coverOut{Preset: c.coverPreset(cv.Preset), Layout: cleanID(cv.Layout), Spine: cv.Spine, N: cv.N, Over: cv.Over}
	}
	if pl := v.PageLimit; pl != nil {
		out.PageLimit = &pageLimitOut{Who: c.text(pl.Who, maxFile), Count: pl.Count, Min: pl.Min, Max: pl.Max, Over: pl.Warn != ""}
	}
	if lk := v.Look; lk != nil {
		out.Look = &lookOut{ID: cleanID(lk.Key), Name: "Макет альбома", Found: lk.Found}
		if c.names() {
			out.Look.Name = cleanLine(lk.Name, maxFile)
		}
	}
	if len(v.AlbumFonts) > 0 {
		miss := c.s.h.FontsMissing(v.AlbumFonts)
		out.Counts.FontsMissing = len(miss)
		for _, m := range miss {
			if t := c.text(m, maxFile); t != "" {
				out.FontsMissing = append(out.FontsMissing, t)
			}
		}
	}
	return out, nil
}

// photoName — имя файла фото по ID для уровня ≥ 2; нет фото — "".
func photoName(byID map[string]library.Photo, id string) string {
	if p, ok := byID[model.BaseID(id)]; ok {
		return p.Name
	}
	return ""
}
