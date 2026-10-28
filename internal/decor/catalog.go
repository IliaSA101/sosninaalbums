package decor

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Каталог окна «Фоны» / «Стикеры» / «Рамочки» (design §5.2): встроенные пресеты + свои − скрытые, темы
// с числами, недавние.

// Темы (решение №92). recent и all — не поле записи, а выборки.
const (
	ThemeRecent = "recent"
	ThemeAll    = "all"
	ThemeMine   = "mine"
	// ThemeBrand — фирменные стикеры программы (спека 116): тема только встроенных, свои и наборы
	// в неё не кладутся (isTheme).
	ThemeBrand = "brand"
)

var themeList = []struct{ id, name string }{
	{ThemeRecent, "Недавние"}, {ThemeAll, "Все"}, {"basic", "Универсальные"}, {"wedding", "Свадьба"},
	{"kids", "Дети"}, {"kindergarten", "Детский сад"}, {"school", "Школа"}, {"grad", "Выпускной"},
	{"holiday", "Праздник"}, {ThemeBrand, "Фирменные"}, {ThemeMine, "Мои"},
}

// isTheme — тема, в которой может лежать элемент (не выборка).
func isTheme(id string) bool {
	if id == ThemeRecent || id == ThemeAll || id == ThemeBrand || id == ThemeNew {
		return false
	}
	return slices.ContainsFunc(themeList, func(t struct{ id, name string }) bool { return t.id == id })
}

// MaxRecent — недавних в каталоге (решение №98).
const MaxRecent = 12

// Item — плитка каталога.
type Item struct {
	ID     string  `json:"id"` // sa:<пресет> | my:<16 hex>
	Theme  string  `json:"theme"`
	Name   string  `json:"name"`
	URL    string  `json:"url"` // встроенный — /media/gen без px; свой — /media/lib/t/<hex>?v=1; UI дописывает &px=
	Own    bool    `json:"own"`
	Aspect float64 `json:"aspect"` // ширина/высота картинки; узор — 0 (заполняет область)
	W      int     `json:"w"`
	H      int     `json:"h"`
	Color  string  `json:"color,omitempty"` // цвет страниц пресета фона: подложка плитки
	Dark   bool    `json:"dark,omitempty"`  // светлый стикер (116): плитке — тёмная шашечка, на светлой он не виден
	// Pages, Places — у разворота (067): 1 — страница, 2 — разворот; мест в шаблоне.
	Pages  int    `json:"pages,omitempty"`
	Places int    `json:"places,omitempty"`
	Pack   string `json:"pack,omitempty"` // название набора (067): подсказка «Набор «…»»
	// Steps, From, Vig, Cover — у макета альбома (131): разворотов и мест при n по умолчанию (с
	// обложкой), есть ли шаг-виньетка и оформление обложки.
	Steps int  `json:"steps,omitempty"`
	From  int  `json:"from,omitempty"`
	Vig   bool `json:"vig,omitempty"`
	Cover bool `json:"cover,omitempty"`
	// PackID — чей предмет (133): BuiltinPack | id набора; своё — "". Peek — крупная картинка без px
	// (/media/lib/p/…) у своего и набора; у встроенного и шаблона разворота — "": UI берёт URL с px.
	PackID string `json:"packId,omitempty"`
	Peek   string `json:"peek,omitempty"`
	// Form, Style — у стикера набора и встроенного (133, browse.go): "corner" | "divider" | "" (стикер);
	// стиль — у обычного стикера, например «Значки». Свои, фоны и рамочки — без них.
	Form  string `json:"form,omitempty"`
	Style string `json:"style,omitempty"`
}

// Theme — строка списка тем слева.
type Theme struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Catalog — содержимое окна одного вида.
type Catalog struct {
	Kind     string   `json:"kind"`
	Themes   []Theme  `json:"themes"`
	Items    []Item   `json:"items"`
	Recent   []string `json:"recent"` // ID плиток, новые первыми
	New      []string `json:"new"`    // ID плиток «Новые» в порядке каталога (133); пусто — []
	ReadOnly bool     `json:"readOnly"`
	// DemoOnly — плитки вида есть и все из примера темы (142): UI даёт указатель на наборы библиотеки.
	DemoOnly bool `json:"demoOnly"`
}

// Catalog — плитки вида kind. recent — недавние из prefs (новые первыми); w×h — мм листа:
// плитка узора рисуется в его пропорции и масштабе.
func (s *Store) Catalog(kind string, recent []string, w, h float64) (Catalog, error) {
	if _, ok := kindDirs[kind]; !ok {
		return Catalog{}, ErrKind
	}
	w, h = tileSize(w, h)
	cat := Catalog{Kind: kind, Items: []Item{}, Recent: []string{}, New: []string{}}
	for _, p := range presetsOf(kind) {
		if gen.Known(p.spec.Kind) { // текстура без файла в сборке — нет плитки (решение №148)
			cat.Items = append(cat.Items, p.item(w, h))
		}
	}
	cat.Items = append(cat.Items, spreadBuiltins(kind)...)
	cat.Items = append(cat.Items, frameBuiltins(kind)...)
	for i := range cat.Items {
		cat.Items[i].PackID = BuiltinPack
		markForm(&cat.Items[i], kind)
	}
	s.mu.RLock()
	cat.ReadOnly = s.readOnly
	demoID := s.demo.id
	// Копии записей: ✎ и × меняют их под mu, пока каталог собирается.
	own := make([]entry, 0, len(s.entries))
	for _, e := range s.entries {
		if _, ok := s.files[kind][e.Key]; ok && e.Kind == kind && !e.Hidden {
			own = append(own, *e)
		}
	}
	packs := s.packItems(kind)
	var fresh map[string]bool
	// «Новые» — не у окна макетов (131, 0.28-133-67); часы — один вызов на каталог и только при наборах.
	if len(s.packs) > 0 && kind != KindAlbum {
		fresh = s.freshPacks(nowMs())
	}
	s.mu.RUnlock()
	fromPacks, mine := s.libItems(kind, packs, own)
	// Порядок 133 §3: наборы (новый выше), встроенные, свои.
	cat.Items = slices.Concat(fromPacks, cat.Items, mine)
	cat.New = newIDs(cat.Items, fresh)
	cat.DemoOnly = demoID != "" && len(cat.Items) > 0 && !slices.ContainsFunc(cat.Items, func(it Item) bool { return it.PackID != demoID })
	has := make(map[string]bool, len(cat.Items))
	count := map[string]int{ThemeAll: len(cat.Items)}
	for _, it := range cat.Items {
		has[it.ID] = true
		count[it.Theme]++
	}
	for _, id := range recent {
		if has[id] && !slices.Contains(cat.Recent, id) && len(cat.Recent) < MaxRecent {
			cat.Recent = append(cat.Recent, id)
		}
	}
	count[ThemeRecent] = len(cat.Recent)
	if len(cat.New) > 0 { // «Новые» — первой строкой и только при предметах
		cat.Themes = append(cat.Themes, Theme{ID: ThemeNew, Name: "Новые", Count: len(cat.New)})
	}
	for _, t := range themeList {
		// Пустые темы скрыты; «Все» и «Мои» (подсказка «Загрузите свои…») есть всегда.
		if count[t.id] > 0 || t.id == ThemeAll || t.id == ThemeMine {
			cat.Themes = append(cat.Themes, Theme{ID: t.id, Name: t.name, Count: count[t.id]})
		}
	}
	return cat, nil
}

// libItems — плитки наборов (design §4.4; тот же ключ у своего — плитка своего: у неё ✎ и ×)
// и отдельно свои новыми первыми, как «Мои шаблоны». Шаблон разворота, файл которого не
// разбирается, — без плитки.
func (s *Store) libItems(kind string, packs []Item, own []entry) (fromPacks, mine []Item) {
	if kind == KindSpread {
		keys := make([]string, 0, len(packs)+len(own))
		for _, it := range packs {
			keys = append(keys, it.ID)
		}
		for _, e := range own {
			keys = append(keys, e.Key)
		}
		s.warmTpl(keys)
	}
	fromPacks = make([]Item, 0, len(packs))
	mine = make([]Item, 0, len(own))
	for _, it := range packs {
		if slices.ContainsFunc(own, func(e entry) bool { return e.Key == it.ID }) {
			continue
		}
		if !s.meta(kind, &it) {
			continue
		}
		fromPacks = append(fromPacks, it)
	}
	// Ничья по Added (два шаблона в одну мс): новее тот, что сохранён позже. own идёт в порядке
	// хранения, поэтому разворот перед устойчивой сортировкой — порядок не случаен.
	slices.Reverse(own)
	slices.SortStableFunc(own, func(a, b entry) int { return cmp.Compare(b.Added, a.Added) })
	for i := range own {
		it := own[i].item()
		if !s.meta(kind, &it) {
			continue
		}
		mine = append(mine, it)
	}
	return fromPacks, mine
}

// meta — сведения плитки шаблона или макета из файла; false — файл не разбирается, плитки нет.
// Картинкам сведения не нужны.
func (s *Store) meta(kind string, it *Item) bool {
	switch kind {
	case KindSpread:
		return s.tplMeta(it)
	case KindAlbum:
		return s.lookMeta(it)
	}
	return true
}

// tileSize — мм плитки узора: лист, а без него (или с мусором) — квадрат 300 мм.
func tileSize(w, h float64) (float64, float64) {
	const maxMM = 2500 // предел области gen
	if !(w > 0 && h > 0 && w <= maxMM && h <= maxMM) {
		return 300, 300
	}
	return w, h
}

// item — плитка своего: превью через кэш миниатюр по ключу (/media/lib/t).
func (e *entry) item() Item {
	it := Item{ID: e.Key, Theme: e.Theme, Name: e.Name, URL: LibURL(e.Key, false), Own: true, W: e.W, H: e.H}
	switch e.Kind { // плитку шаблона и макета рисует Go (/media/tpl, /media/look), не кэш миниатюр
	case KindSpread:
		it.URL = TplURL(e.Key)
	case KindAlbum:
		it.URL = LookURL(e.Key)
	case KindBg, KindSticker, KindFrame: // peek — только у картинок: у JSON макета /media/lib/p дал бы 404
		it.Peek = LibURL(e.Key, true)
	}
	if e.W > 0 && e.H > 0 {
		it.Aspect = float64(e.W) / float64(e.H)
	}
	return it
}

// LibURL — адрес плитки (t) или превью (p) своего; v=1 — чтобы UI мог дописать &px=.
func LibURL(key string, preview bool) string {
	t := "t"
	if preview {
		t = "p"
	}
	return LibPath + t + "/" + strings.TrimPrefix(key, libPrefix) + "?v=1"
}

// Background — что ставит щелчок по плитке фона (ApplyBackground в App): встроенный — узор
// и цвет страниц пресета; свой — фото фона по ключу (путь — из индекса, не от интерфейса).
type Background struct {
	Pattern *model.Pattern  // встроенный; nil — свой
	Color   string          // цвет страниц пресета
	Photo   model.PhotoInfo // свой: ID = LibPhotoID(Lib), Path, Lib, W, H, Size
}

// Background — разбор ID плитки фона.
func (s *Store) Background(id string) (Background, error) {
	if p, ok := findPreset(bgPresets, id); ok {
		if !gen.Known(p.spec.Kind) {
			return Background{}, ErrNotFound
		}
		sp := gen.Clamp(p.spec)
		// Preset — отметка плитки в альбоме (решение №201): имя в карточке и выбранная в галерее.
		return Background{Pattern: &model.Pattern{Kind: sp.Kind, Inks: sp.Inks, Scale: sp.Scale, Seed: sp.Seed, Preset: p.id},
			Color: p.color}, nil
	}
	if !model.ValidLibKey(id) {
		return Background{}, ErrNotFound
	}
	s.mu.RLock()
	e := s.entryOf(KindBg, id)
	path, ok := s.files[KindBg][id]
	ok = ok && (e != nil || s.inPack(KindBg, id))
	s.mu.RUnlock()
	if !ok {
		return Background{}, ErrNotFound
	}
	// Размеры — из файла с поворотом EXIF, как у записи при открытии альбома.
	info, err := library.ReadInfo(path)
	if err != nil {
		return Background{}, ErrNotFound
	}
	return Background{Photo: model.PhotoInfo{ID: model.LibPhotoID(id), Path: path, Lib: id, W: info.W, H: info.H, Size: info.Size}}, nil
}

// cleanName — имя для показа: без управляющих и невидимых знаков, пробелы схлопнуты,
// ≤ 60 знаков; пусто — "".
func cleanName(s string) string { return cleanText(s, 60) }

// cleanText — cleanName с другим пределом (описание, лицензия, источник набора).
func cleanText(s string, maxRunes int) string {
	var b strings.Builder
	n, space := 0, false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = b.Len() > 0
			continue
		}
		if r == utf8.RuneError || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		if space {
			if n+1 >= maxRunes {
				break
			}
			b.WriteByte(' ')
			n++
			space = false
		}
		if n == maxRunes {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// noName — имя, если от файла ничего не осталось.
const noName = "Без названия"
