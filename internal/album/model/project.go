package model

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// FormatVersion — версия формата файла. 2: относительные пути, кадр, подписи,
// тексты и фон (спека 006); 3: места сетки `grid` (спека 007); 4: зеркало фото
// `flipH/flipV` и фото на фоне `bgPhoto*` (спека 009); 5: прозрачность фото фона
// `fade` (спека 017) и свои отметки `marks` (спека 004, тот же выпуск 0.7.0);
// 6: до 64 текстов на развороте (спека 026) — 0.9.1 файл не откроет и не обрежет до 20;
// в v6 до выпуска 0.10 легли и поля подписей 030 (`vars`, `person`, `pad`, `caps`, все omitempty)
// обложка 032 (`cover`, omitempty) и виньетка 031 (`sheet`, `kind`, `vig`, `role`, omitempty);
// 7: форма своего шаблона виньетки `vig.own` (спека 038, 0.11) — 0.10 файл не откроет и не
// затрёт форму автосохранением;
// 8 (общий на 0.12, новые поля 0.12 до выпуска ложатся сюда же, без нового номера):
// наклон фото фона `frame.r` у bgPhoto* (спека 045) — 0.11 его обнулил бы и затёр;
// форма и скругление мест `shape`/`round` и формы виньетки `vig` (спека 044);
// безопасные линии обложки `cover.spec.safe`, `spineSafe` (спека 047, omitempty);
// узоры фона `pat/patL/patR`, стикеры `stickers` (и их ID в `order`) и ключ своего из
// библиотеки `photos.*.lib` (спека 048, omitempty);
// 9 (общий на 0.13): новые значения `Spread.Layout` — ключи форм 051 (`Q[…]`, `G(…)`,
// `L/A(…)`, `m<i>`, `S/U/D(…)`); полей нет, но 0.12 показала бы мозаику и правкой затёрла ключ.
// Старые читаются с миграцией.
// Поля 008 (`name`, `rot`, `auto`) вошли в v3 до выпуска v3 — без смены версии (design 008 §1).
const FormatVersion = 9

type Settings struct {
	PageW   float64 `json:"pageW"`   // мм, одна страница; у листа (Sheet) — ширина листа
	PageH   float64 `json:"pageH"`   // мм
	Margin  float64 `json:"margin"`  // мм
	Gap     float64 `json:"gap"`     // мм
	Layflat bool    `json:"layflat"` // фото может идти через корешок
	DPI     int     `json:"dpi"`
	Bg      string  `json:"bg"` // цвет страниц альбома #rrggbb; "" — белый
	// Safe — безопасная зона от края, мм; 0 — без зоны. Без omitempty: 0 допустим,
	// а отсутствующее в файле поле Load заполняет DefaultSafe (028).
	Safe float64 `json:"safe"`
	// Sheet — проект из отдельных листов без корешка (виньетка 031): лист — одна страница
	// PageW×PageH (Geo), layflat всегда. Задаётся при создании и не меняется (SetSettings).
	Sheet bool `json:"sheet,omitempty"`
}

func DefaultSettings() Settings {
	return Settings{PageW: 300, PageH: 300, Margin: 12, Gap: 3, Layflat: true, DPI: 300, Safe: DefaultSafe}
}

type Spread struct {
	ID     string   `json:"id"`
	Photos []string `json:"photos"` // ID фото в порядке ленты; раскладка с перестановкой его не меняет
	// Layout — ключ выбранной раскладки (см. internal/layout); "" — лучшая в порядке ленты.
	Layout string `json:"layout,omitempty"`
	// Variant — номер варианта из файлов до ключей; Load переводит его в Layout.
	Variant int          `json:"variant,omitempty"`
	Texts   []*TextBlock `json:"texts,omitempty"` // порядок слоёв текстов (последний сверху); общий — Order
	BgL     string       `json:"bgL,omitempty"`   // цвет левой страницы; "" — цвет альбома
	BgR     string       `json:"bgR,omitempty"`
	// Grid — места режима «Сетка» (grid.go); пусто — «Авто». Порядок = порядок мест и слоёв.
	// В сетке Photos = фото непустых мест по порядку, Layout хранится для «Вернуть авто».
	Grid []*Place `json:"grid,omitempty"`
	// Order — общий порядок слоёв сетки (order.go): ID мест и текстов снизу вверх;
	// nil — места по Grid, затем тексты. Grid и Texts всегда в его порядке.
	Order []string `json:"order,omitempty"`
	// Name — название в программе (лента, плитки); в экспорт не идёт.
	Name string `json:"name,omitempty"`
	// BgPhoto — фото на фоне всего разворота; BgPhotoL/R — на странице. Взаимоисключающие
	// (backdrop.go); фото лежит в Project.Photos, но «в альбоме» не считается.
	BgPhoto  *Backdrop `json:"bgPhoto,omitempty"`
	BgPhotoL *Backdrop `json:"bgPhotoL,omitempty"`
	BgPhotoR *Backdrop `json:"bgPhotoR,omitempty"`
	// Pat — узор на фоне всего листа; PatL/R — страницы (048, decor.go). Взаимоисключающие,
	// как BgPhoto*; узор лежит под фото фона той же области.
	Pat  *Pattern `json:"pat,omitempty"`
	PatL *Pattern `json:"patL,omitempty"`
	PatR *Pattern `json:"patR,omitempty"`
	// Stickers — стикеры листа (048, decor.go); порядок — относительный порядок слоёв, как Texts.
	Stickers []*Sticker `json:"stickers,omitempty"`
	// Caps — общий стиль подписей листа (030, fields.go); nil — у каждой подписи свой.
	Caps *SheetCaps `json:"caps,omitempty"`
	// Kind — вид разворота (031): "" — обычный, KindVignette — виньетка; Vig — её параметры
	// (sheet.go). Места виньетки — та же сетка с ролями (Place.Role).
	Kind string    `json:"kind,omitempty"`
	Vig  *Vignette `json:"vig,omitempty"`
	// Vars — поля шапки листа (031 dialog §4, fields.go); nil — свои пусты (VarsOf).
	Vars *Vars `json:"vars,omitempty"`
}

// TextStyle — оформление текста; тип живёт в internal/text, проект его хранит.
type TextStyle = text.Style

// Frame — кадр фото в ячейке. Хранится в PhotoRef, поэтому едет вместе с фото.
type Frame struct {
	S float64 `json:"s"` // масштаб 1..4: 1 — минимальный, при котором фото закрывает ячейку
	X float64 `json:"x"` // −1..1 доля свободного хода по ширине: −1 видна левая часть, 0 центр
	Y float64 `json:"y"` // −1..1 по высоте: −1 видна верхняя часть
	// R — поворот фото вокруг центра в рамке, градусы по часовой, (−180, 180]:
	// R = 90·четверти + наклон (±44,9), см. splitR. Рамка стоит, фото её закрывает.
	R float64 `json:"r,omitempty"`
}

// Caption — подпись к фото.
type Caption struct {
	Text  string    `json:"text"`
	Style TextStyle `json:"style"`
	Over  bool      `json:"over,omitempty"` // на фото у нижнего края, а не под ним
	// Pad — отступ от фото, мм (030); nil — DefaultPad, как в прежних файлах.
	Pad *float64 `json:"pad,omitempty"`
}

// TextBlock — свободный текст на развороте.
type TextBlock struct {
	ID string  `json:"id"` // 12 hex, NewID
	X  float64 `json:"x"`  // мм от левого верхнего угла разворота: угол рамки до поворота
	Y  float64 `json:"y"`
	W  float64 `json:"w"` // ширина переноса; высота — по тексту; при Auto — производная (FitTexts)
	// Rot — градусы по часовой вокруг (X, Y), (−180, 180]; 0 — без поворота.
	Rot float64 `json:"rot,omitempty"`
	// Auto — ширина по тексту; false — закреплена (так у прежних файлов).
	Auto  bool      `json:"auto,omitempty"`
	Text  string    `json:"text"`
	Style TextStyle `json:"style"`
}

// PhotoRef — что проект помнит о фото, даже если папку не открыли.
type PhotoRef struct {
	Path string `json:"path"` // в памяти всегда абсолютный; относительным бывает только в файле
	W    int    `json:"w"`
	H    int    `json:"h"`
	Size int64  `json:"size,omitempty"` // байты файла: сверка путей при открытии; 0 — неизвестен
	// Alt — запасной путь из файла, когда фото не нашлось ни там, ни там: пишется
	// обратно как был, иначе автосохранение на чужом компьютере стёрло бы его.
	Alt     string   `json:"alt,omitempty"`
	Frame   *Frame   `json:"frame,omitempty"`   // nil — S=1, X=Y=0
	Caption *Caption `json:"caption,omitempty"` // nil — без подписи
	// FlipH/FlipV — зеркало картинки внутри её прямоугольника; едут с фото, как Frame.
	FlipH bool `json:"flipH,omitempty"`
	FlipV bool `json:"flipV,omitempty"`
	// Person — имя на фото (030): своё или из вставки; едет с фото. nil — из имени файла.
	Person *Person `json:"person,omitempty"`
	// Lib — ключ своего элемента библиотеки my:<16 hex> (048): в файле вместо пути, путь в
	// памяти — от искателя библиотеки; ID записи — LibPhotoID(Lib).
	Lib string `json:"lib,omitempty"`
}

// Project — альбом в памяти. Пути абсолютные; относительными они становятся
// только в файле (file.go). JSON-теги нужны для снимков отмены и сравнения правок.
type Project struct {
	Version int `json:"version"`
	// Product — тип изделия (ProductAlbum); после создания не меняется.
	Product  string              `json:"product"`
	Folders  []string            `json:"folders"`
	Settings Settings            `json:"settings"`
	Spreads  []*Spread           `json:"spreads"`
	Photos   map[string]PhotoRef `json:"photos"` // только фото, стоящие в альбоме

	// FolderAlt — запасной путь папки, которой не нашлось ни там, ни там (как PhotoRef.Alt):
	// encode пишет его обратно как был. Ключ — путь из Folders.
	FolderAlt map[string]string `json:"folderAlt,omitempty"`
	// Cover — обложка (032, cover.go): лист вне Spreads, в нумерацию не входит; nil — нет.
	Cover *Cover `json:"cover,omitempty"`
	// from — версия файла, из которого прочитан проект (0 — новый): по ней
	// Editor решает, нужен ли бэкап перед первой записью.
	from int
	// loaded — свои отметки из файла (ID → отметка): NewEditor копирует их себе (JSON их не несёт).
	loaded map[string]Mark
}

// Доступы к from и loaded: полем их не экспортировать — попали бы в JSON снимков отмены
// и в проверку холостой правки (023).

// From — версия файла, из которого прочитан проект (0 — новый).
func (p *Project) From() int { return p.from }

// SetFrom — версия исходного файла (Load, Restore).
func (p *Project) SetFrom(v int) { p.from = v }

// Loaded — свои отметки, прочитанные из файла.
func (p *Project) Loaded() map[string]Mark { return p.loaded }

// SetLoaded — отметки из файла (Load кладёт, NewEditor копирует).
func (p *Project) SetLoaded(m map[string]Mark) { p.loaded = m }

func New() *Project {
	return &Project{Version: FormatVersion, Product: ProductAlbum, Settings: DefaultSettings(), Photos: map[string]PhotoRef{}}
}

// Params — геометрия для layout. Раскладка считает лист шириной 2·PageW, поэтому листу
// (Sheet) отдаётся половина ширины и признак Sheet: без корешка и постраничных вариантов.
func (p *Project) Params() layout.Params {
	s := p.Settings
	if s.Sheet {
		return layout.Params{PageW: s.PageW / 2, PageH: s.PageH, Margin: s.Margin, Gap: s.Gap, Layflat: true, Sheet: true}
	}
	return layout.Params{PageW: s.PageW, PageH: s.PageH, Margin: s.Margin, Gap: s.Gap, Layflat: s.Layflat}
}

// Aspects — действующие пропорции фото в ячейке: при нечётной четверти поворота
// фото лежит боком, ячейке нужно 1/a. Раскладка и «Под фото» берут их.
func (p *Project) Aspects(ids []string) []float64 {
	a := make([]float64, len(ids))
	for i, id := range ids {
		a[i] = p.aspectOf(id, false)
	}
	return a
}

// RawAspect — пропорции файла, без поворота: по ним считается геометрия самого фото
// (placeImage, фон), иначе повёрнутое на 90° фото растянулось бы.
func (p *Project) RawAspect(id string) float64 { return p.aspectOf(id, true) }

func (p *Project) aspectOf(id string, raw bool) float64 {
	r, ok := p.Photos[id]
	if !ok || r.W <= 0 || r.H <= 0 {
		return 1
	}
	a := float64(r.W) / float64(r.H)
	if turn, _ := SplitR(r.FrameOrDefault().R); !raw && turn%2 == 1 {
		return 1 / a
	}
	return a
}

// Layout — действующая раскладка разворота: выбранная, если строится, иначе лучшая
// в порядке ленты. best — ряд «в порядке ленты» (не пуст, если в развороте есть фото).
func (p *Project) Layout(s *Spread) (r layout.Result, best []layout.Result, ok bool) {
	asp, prm := p.Aspects(s.Photos), p.Params()
	best = layout.Cached(asp, prm, RowLen(len(asp)))
	if s.Layout != "" {
		if r, ok := layout.Build(s.Layout, asp, prm); ok {
			return r, best, true
		}
	}
	if len(best) == 0 {
		return layout.Result{}, best, false
	}
	return best[0], best, true
}

// SetLayout ставит раскладку; лучшую в порядке ленты храним как "", чтобы выбор
// уже стоящей раскладки был холостой правкой.
func (p *Project) SetLayout(s *Spread, key string) {
	if best := layout.Cached(p.Aspects(s.Photos), p.Params(), RowLen(len(s.Photos))); len(best) > 0 && best[0].Key == key {
		key = ""
	}
	s.Layout = key
}

// KeepLayouts — после правки у разворотов, где поменялись фото или геометрия,
// раскладка остаётся прежней (то же дерево ячеек), если строится для нового
// состава; иначе и при другом числе фото — лучшая. Смена действующих пропорций
// (поворот фото на 90°) — тоже правка состава. Нетронутые развороты не трогаем,
// поэтому холостая правка остаётся холостой. Цвет фона и dpi раскладку не меняют —
// по ним развороты не перепроверяются. Сетки (до или после правки) пропускаются:
// ключ у них не действует, его проверит unfreeze.
func (p *Project) KeepLayouts(before *Project) {
	was := make(map[string]*Spread, len(before.Spreads))
	for _, s := range before.Spreads {
		was[s.ID] = s
	}
	settings := before.Params() != p.Params()
	var todo, warm []*Spread
	for _, s := range p.Spreads {
		old := was[s.ID]
		if old == nil || len(s.Grid) > 0 || len(old.Grid) > 0 {
			continue
		}
		// Фото те же и действующие пропорции те же — трогать нечего.
		if !settings && slices.Equal(old.Photos, s.Photos) && slices.Equal(before.Aspects(old.Photos), p.Aspects(s.Photos)) {
			continue
		}
		todo = append(todo, s)
		if len(s.Photos) > 0 {
			warm = append(warm, s)
		}
	}
	// Смена настроек и поворот многих фото — холодный перебор у каждого разворота:
	// параллельно заранее, цикл ниже и следующий View попадут в кэш.
	p.WarmLayouts(warm)
	for _, s := range todo {
		old := was[s.ID]
		if len(old.Photos) != len(s.Photos) {
			s.Layout = ""
			continue
		}
		cur, _, ok := before.Layout(old)
		if !ok {
			s.Layout = ""
			continue
		}
		if _, ok := layout.Build(cur.Key, p.Aspects(s.Photos), p.Params()); ok {
			p.SetLayout(s, cur.Key)
		} else {
			s.Layout = ""
		}
	}
}

// Clone — глубокая копия через JSON. Ошибка здесь значит, что в проект попало
// недопустимое значение (NaN): паника, а не пустой проект — иначе откат в try
// подменил бы альбом пустым. guard в App превращает её в запись в логе.
func (p *Project) Clone() *Project { return Unmarshal(Marshal(p)) }

// Marshal — JSON проекта для снимков и проверки холостой правки.
func Marshal(p *Project) []byte {
	data, err := json.Marshal(p)
	if err != nil {
		panic(fmt.Sprintf("model: снимок проекта не собрался: %v", err))
	}
	return data
}

// Unmarshal — проект из снимка Marshal. Снимок построен самой программой, поэтому плохие
// данные — ошибка кода: паника (как у Clone), а не пустой проект. Файлы читает Load.
func Unmarshal(data []byte) *Project {
	c := New()
	if err := json.Unmarshal(data, c); err != nil {
		panic(fmt.Sprintf("model: снимок проекта не читается: %v", err))
	}
	return c
}

func NewID() string {
	var b [6]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (p *Project) Find(id string) (int, *Spread) {
	for i, s := range p.Spreads {
		if s.ID == id {
			return i, s
		}
	}
	return -1, nil
}

// Detach убирает фото из всех листов (с обложкой): одно фото стоит в альбоме один раз.
// Место сетки при этом пустеет, но остаётся (design 007 §0.3: места правит тот,
// кто убирает фото, — syncGrid не знает, куда фото ушло).
func (p *Project) Detach(ids []string) {
	gone := make(map[string]bool, len(ids))
	for _, id := range ids {
		gone[id] = true
	}
	for _, s := range p.Sheets() {
		s.Photos = slices.DeleteFunc(s.Photos, func(id string) bool { return gone[id] })
		for _, pl := range s.Grid {
			if gone[pl.Photo] {
				pl.Photo = ""
			}
		}
	}
}

// Insert ставит фото в разворот на позицию index (-1 — в конец).
func (s *Spread) Insert(ids []string, index int) {
	if index < 0 || index > len(s.Photos) {
		index = len(s.Photos)
	}
	s.Photos = slices.Insert(s.Photos, index, ids...)
}

// Normalize приводит проект после правки в порядок, сверяясь с состоянием до неё:
// развороты, в которых фото были, а стали пустыми, удаляются — если терять на них
// нечего (empty): разворот-вступление с текстом или фоном живёт без фото, сетка
// с пустыми местами — законная заготовка. Созданный пустым (NewSpread без фото)
// живёт: его Photos не меняются.
func (p *Project) Normalize(before *Project) {
	was := make(map[string]*Spread, len(before.Spreads))
	for _, s := range before.Spreads {
		was[s.ID] = s
	}
	p.Spreads = slices.DeleteFunc(p.Spreads, func(s *Spread) bool {
		old := was[s.ID]
		if old == nil || slices.Equal(old.Photos, s.Photos) {
			return false
		}
		return s.Empty()
	})
}

// Empty — разворот можно удалить молча: терять на нём нечего. Им пользуются
// Normalize и dedupe. Новое поле разворота с данными пользователя обязано
// попасть сюда, иначе Load или правка его выбросит.
func (s *Spread) Empty() bool {
	return len(s.Photos) == 0 && len(s.Texts) == 0 && len(s.Grid) == 0 &&
		s.BgL == "" && s.BgR == "" && s.Name == "" &&
		s.BgPhoto == nil && s.BgPhotoL == nil && s.BgPhotoR == nil && s.Caps == nil &&
		s.Pat == nil && s.PatL == nil && s.PatR == nil && len(s.Stickers) == 0 &&
		s.Kind == "" && s.Vig == nil && s.Vars == nil
}

// Placed — фото стоит в каком-нибудь листе (месте обложки тоже).
func (p *Project) Placed(id string) bool {
	for _, s := range p.Sheets() {
		if slices.Contains(s.Photos, id) {
			return true
		}
	}
	return false
}

// PruneRefs оставляет в Photos только используемые фото: стоящие в местах, на фонах и в
// стикерах (048). Фото, оставшееся только на фоне или в стикере, теряет свойства места (кадр,
// подпись, зеркало): они не переживают уход из мест. Имя (Person) — свойство фото, а не места:
// остаётся.
func (p *Project) PruneRefs() {
	used := map[string]bool{}
	for _, s := range p.Sheets() {
		for _, id := range s.Photos {
			used[id] = true
		}
	}
	onBg := p.OnDecor()
	for id, r := range p.Photos {
		switch {
		case used[id]:
		case !onBg[id]:
			delete(p.Photos, id)
		case r.Frame != nil || r.Caption != nil || r.FlipH || r.FlipV:
			r.Frame, r.Caption, r.FlipH, r.FlipV = nil, nil, false, false
			p.Photos[id] = r
		}
	}
}

// Remember записывает сведения о фото (и ключ своего из библиотеки, 048). Кадр, подпись
// и зеркало остаются: перенос (MovePhoto идёт через AddPhotos) и повторная автосборка их
// не стирают.
func Remember(p *Project, photos []PhotoInfo) []string {
	ids := make([]string, len(photos))
	for i, ph := range photos {
		ids[i] = ph.ID
		r := p.Photos[ph.ID]
		if r.Path != ph.Path {
			r.Alt, r.Size = "", 0 // другой файл: запасной путь и длина прежнего не годятся
		}
		r.Path, r.W, r.H = ph.Path, ph.W, ph.H
		if ph.Lib != "" { // ID своего — от ключа: сведения из Refs без Lib ключ не снимают
			r.Lib = ph.Lib
		}
		if ph.Size > 0 {
			r.Size = ph.Size
		}
		p.Photos[ph.ID] = r
	}
	return ids
}

// VariantLimit — длина ряда раскладок (и круга листания) до 16 фото.
const VariantLimit = 24

// RowLen — длина первого ряда «Авто» для n фото: до 16 — VariantLimit, с layout.ManyFrom —
// первая порция (layout.ManyBatch), следующие досчитываются по требованию (AutoRow).
func RowLen(n int) int {
	if n >= layout.ManyFrom {
		return layout.ManyBatch
	}
	return VariantLimit
}

// AutoRow — ряд «Авто» в порядке ленты (variety.Row: лучшая, формы и ряд layout вперемешку,
// 051), где стоит key, и его место в ряду (−1 — нет). До 16 фото — весь ряд VariantLimit. С
// ManyFrom — наименьший префикс из порций по ManyBatch (до ManyCap), где key есть; нет нигде —
// первая порция. more — ряд можно досчитать: он полный и короче ManyCap.
func AutoRow(asp []float64, prm layout.Params, key string) (row []layout.Result, i int, more bool) {
	if len(asp) < layout.ManyFrom {
		row = variety.RowResults(asp, prm, VariantLimit)
		return row, IndexOf(row, key), false
	}
	for n := layout.ManyBatch; n <= layout.ManyCap; n += layout.ManyBatch {
		row = variety.RowResults(asp, prm, n)
		if i = IndexOf(row, key); i >= 0 {
			return row, i, len(row) == n && n < layout.ManyCap
		}
		if len(row) < n {
			break // дальше ряд не растёт
		}
	}
	row = variety.RowResults(asp, prm, layout.ManyBatch)
	return row, -1, len(row) == layout.ManyBatch
}

// MoreRow — ряд «Авто» на n ≥ ManyFrom фото, досчитанный ещё на порцию после have вариантов
// (не длиннее ManyCap). До 16 фото — весь ряд.
func MoreRow(asp []float64, prm layout.Params, have int) (row []layout.Result, more bool) {
	if len(asp) < layout.ManyFrom {
		return variety.RowResults(asp, prm, VariantLimit), false
	}
	n := min((max(have, 0)/layout.ManyBatch+1)*layout.ManyBatch, layout.ManyCap)
	row = variety.RowResults(asp, prm, n)
	return row, len(row) == n && n < layout.ManyCap
}

// ExtraRow — «Другие варианты» разворота (041): в подборке третьим рядом, в листании и счётчике
// «i / N» — сразу после ряда «Авто» (решение 0.11 №36). Ключи строит layout: SetLayout их примет.
// С ManyFrom фото — пусто: ряд «Авто» там растёт порциями («+»), а страница поменьше делает
// фото мельче порога воздуха (k_air ≤ 16, design 041 §5).
func ExtraRow(asp []float64, prm layout.Params) []layout.Result {
	if len(asp) >= layout.ManyFrom {
		return nil
	}
	return variety.Extra(asp, prm, RowLen(len(asp)))
}

func IndexOf(rs []layout.Result, key string) int {
	for i, r := range rs {
		if r.Key == key {
			return i
		}
	}
	return -1
}

// ProductAlbum — тип изделия «Альбом»: единственный. Лист виньетки (031) — тоже album,
// режим листа задаёт Settings.Sheet. Новый тип изделия (календарь) — только с новой
// FormatVersion (design 010 §1): прежняя программа читает чужое как album и при сохранении
// перезаписала бы его.
const ProductAlbum = "album"

// NormProduct приводит тип изделия к известному: пусто (файл до 0.5.0) — album молча,
// чужое — album и строка в лог.
func NormProduct(s string) string {
	switch s {
	case ProductAlbum:
		return s
	case "":
		return ProductAlbum
	}
	log.Printf("неизвестный тип изделия %s, открыт как %s", paths.LogQ(s), ProductAlbum)
	return ProductAlbum
}
