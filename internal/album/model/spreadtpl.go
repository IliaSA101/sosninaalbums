package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// Готовые развороты библиотеки (спека 067, design §1): места без фото и оформление области.
// Геометрия — от области, а не в мм: шаблон встаёт на любой формат. Формат .salbum не
// меняется — шаблон пишет обычные поля проекта, альбом его не помнит (решение 5).

const (
	// TplV — версия файла шаблона; больше — файл новее программы (пропуск).
	TplV = 1
	// MaxTplFile — байтов в файле шаблона.
	MaxTplFile = 256 << 10
	// Пределы шаблона: доля области от края (якорь), размер предмета в долях, мм области.
	maxTplShift = 2.0
	minTplSize  = 0.01
	maxTplSize  = 4.0
	minTplRatio = 0.2
	maxTplRatio = 5.0
	minTplMM    = 10.0
	maxTplMM    = 2500.0
)

var (
	// ErrTplEmpty — в шаблоне нет ни мест, ни оформления (или он не прошёл зажим).
	ErrTplEmpty = errors.New("в шаблоне разворота нечего ставить")
	// ErrTplNewer — шаблон из новой версии программы.
	ErrTplNewer = errors.New("шаблон разворота из новой версии программы")
	errTplBig   = errors.New("файл шаблона разворота слишком большой")
)

// SpreadTpl — готовый разворот или страница (067): места без фото и оформление области.
type SpreadTpl struct {
	V     int     `json:"v"`     // TplV
	Pages int     `json:"pages"` // 1 — страница, 2 — разворот
	Ratio float64 `json:"ratio"` // W/H области при сохранении, 0,2…5 (превью, справка)
	// W, H — мм области при сохранении (0 — неизвестно): по ним кегль надписей × S'/S.
	W      float64 `json:"w,omitempty"`
	H      float64 `json:"h,omitempty"`
	Places []Box   `json:"places,omitempty"` // доли области, порядок = порядок фото (как Template.Boxes)
	Whole  *TplBg  `json:"whole,omitempty"`  // Pages = 2: узор и фото фона на весь разворот (цвета нет)
	Page   []TplBg `json:"page"`             // len = Pages: левая, правая
	// Stickers — в порядке слоёв: сначала лежавшие под местами (Under), затем остальные.
	Stickers []TplItem `json:"stickers,omitempty"`
	Texts    []TplText `json:"texts,omitempty"`
	// Borders — рамочки мест (131, V = 1): 0.27 их не видит — места без рамочек. Drafts — заготовки
	// (надписи с Draft): 0.27 их не видит и не печатает. Texts + Drafts ≤ MaxTexts.
	Borders []TplBorder `json:"borders,omitempty"`
	Drafts  []TplText   `json:"drafts,omitempty"`
}

// TplBg — фон страницы (или всего разворота): цвет, узор, фото фона — только своё из
// библиотеки по ключу (решение 6: кадр из съёмки в шаблон не идёт), рамка — по области.
type TplBg struct {
	Color string   `json:"color,omitempty"` // #rrggbb
	Pat   *Pattern `json:"pat,omitempty"`
	Lib   string   `json:"lib,omitempty"` // my:<16 hex>
	Fade  float64  `json:"fade,omitempty"`
}

// Anchor — привязка центра предмета к области: AX, AY −1/0/+1 — от левого (верхнего) края,
// центра, правого (нижнего); DX, DY — сдвиг центра от этой точки в долях S = min(W, H)
// области. Угловой стикер так остаётся в углу на любом формате (решение 3).
type Anchor struct {
	AX int8    `json:"ax"`
	AY int8    `json:"ay"`
	DX float64 `json:"dx"`
	DY float64 `json:"dy"`
}

// TplItem — стикер шаблона: рисунок Art или своё из библиотеки Lib (ровно одно). W, H — доли
// S; у растяжимого рисунка (StretchArt) — доли ширины и высоты области, и сдвиг якоря тоже:
// лента у нижнего края на любом формате остаётся у края во всю ширину.
type TplItem struct {
	Anchor
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Rot   float64 `json:"rot,omitempty"`
	Fade  float64 `json:"fade,omitempty"`
	FlipH bool    `json:"flipH,omitempty"`
	FlipV bool    `json:"flipV,omitempty"`
	Under bool    `json:"under,omitempty"` // лежал под всеми местами области — так и встанет
	Art   *Art    `json:"art,omitempty"`
	Lib   string  `json:"lib,omitempty"`
}

// TplText — надпись шаблона: W — доля S, кегль при постановке × S'/S.
type TplText struct {
	Anchor
	W     float64   `json:"w"`
	Rot   float64   `json:"rot,omitempty"`
	Auto  bool      `json:"auto,omitempty"`
	Text  string    `json:"text"`
	Style TextStyle `json:"style"`
}

// StretchArt — рисунок растягивается по осям области (gen.Meta.Stretch: плашка, волна-лента):
// в шаблоне его W, H — доли ширины и высоты области, а не S.
func StretchArt(a *Art) bool {
	if a == nil {
		return false
	}
	m, ok := gen.Lookup(a.Kind)
	return ok && m.Stretch
}

// ParseSpreadTpl — шаблон из файла (библиотека, набор): не больше MaxTplFile, неизвестные поля
// пропускаются (файл новой версии того же V читается), V новее — ErrTplNewer, затем зажим;
// пусто — ErrTplEmpty.
func ParseSpreadTpl(data []byte) (SpreadTpl, error) {
	if len(data) > MaxTplFile {
		return SpreadTpl{}, errTplBig
	}
	var t SpreadTpl
	if err := json.Unmarshal(data, &t); err != nil {
		return SpreadTpl{}, err
	}
	if t.V > TplV {
		return SpreadTpl{}, ErrTplNewer
	}
	c, ok := ClampSpreadTpl(t)
	if !ok {
		return SpreadTpl{}, ErrTplEmpty
	}
	return c, nil
}

// TplKey — ключ шаблона в библиотеке: my: + 8 байт sha256 канонического JSON (json.Marshal
// после ClampSpreadTpl): тот же шаблон не дублируется. Не маршалится — "".
func TplKey(t SpreadTpl) string {
	b, err := json.Marshal(t)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return libKeyPrefix + hex.EncodeToString(sum[:8])
}

// ClampSpreadTpl — шаблон к допустимому; возвращает копию. Отказ (false): V не TplV, Pages не
// 1|2, len(Page) ≠ Pages, после зажима пусто. Остальное зажимается: Ratio 0,2…5 (иначе 1), W
// и H 10…2500 мм (иначе оба 0), места — ClampBoxes, цвета, узоры, ключи; у предмета якорь
// −1…1, сдвиг ±2, размер 0,01…4, NaN — предмет отброшен; стикер с обоими или без Art/Lib —
// отброшен; стикеров ≤ MaxStickers, надписей и заготовок вместе ≤ MaxTexts; рамочки — по
// clampTplBorders. Фото и узор всего разворота исключают постраничные (как слоты Spread).
// Доли округляются до 1e-6 (короткий ключ).
func ClampSpreadTpl(t SpreadTpl) (SpreadTpl, bool) {
	if t.V != TplV || t.Pages != 1 && t.Pages != 2 || len(t.Page) != t.Pages {
		return SpreadTpl{}, false
	}
	c := SpreadTpl{V: TplV, Pages: t.Pages, Ratio: 1}
	if r := Finite(t.Ratio, 0); r >= minTplRatio && r <= maxTplRatio {
		c.Ratio = round6(r)
	}
	if w, h := Finite(t.W, 0), Finite(t.H, 0); w >= minTplMM && w <= maxTplMM && h >= minTplMM && h <= maxTplMM {
		c.W, c.H = geom.Round2(w), geom.Round2(h)
	}
	places, at := tplPlaces(t.Places)
	for _, b := range ClampBoxes(places) {
		b.X, b.Y, b.W, b.H, b.Round = round6(b.X), round6(b.Y), round6(b.W), round6(b.H), round6(b.Round)
		c.Places = append(c.Places, b)
	}
	c.Whole, c.Page = clampTplBgs(t)
	for _, it := range t.Stickers {
		if len(c.Stickers) == MaxStickers {
			break
		}
		if ci, ok := clampTplItem(it); ok {
			c.Stickers = append(c.Stickers, ci)
		}
	}
	// Под местами — первыми: порядок в срезе и есть порядок слоёв (design §2, decorOrder).
	slices.SortStableFunc(c.Stickers, func(a, b TplItem) int { return boolRank(b.Under) - boolRank(a.Under) })
	c.Texts = clampTplTexts(t.Texts, MaxTexts)
	c.Drafts = clampTplTexts(t.Drafts, MaxTexts-len(c.Texts))
	c.Borders = clampTplBorders(t.Borders, at, len(c.Places))
	return c, !c.empty()
}

// clampTplBgs — фоны шаблона: весь разворот (только у Pages = 2, без цвета — он постраничный)
// и страницы; узор и фото всего разворота исключают постраничные (как слоты Spread).
func clampTplBgs(t SpreadTpl) (*TplBg, []TplBg) {
	var whole *TplBg
	if t.Pages == 2 && t.Whole != nil {
		if w := clampTplBg(*t.Whole); w.Pat != nil || w.Lib != "" {
			w.Color = ""
			whole = &w
		}
	}
	pages := make([]TplBg, len(t.Page))
	for i, pg := range t.Page {
		pages[i] = clampTplBg(pg)
		if whole != nil && whole.Pat != nil {
			pages[i].Pat = nil
		}
		if whole != nil && whole.Lib != "" {
			pages[i].Lib, pages[i].Fade = "", 0
		}
	}
	return whole, pages
}

// empty — ни мест, ни цвета, узора и фона, ни стикеров, ни надписей и заготовок.
func (t SpreadTpl) empty() bool {
	if len(t.Places)+len(t.Stickers)+len(t.Texts)+len(t.Drafts) > 0 || t.Whole != nil {
		return false
	}
	for _, pg := range t.Page {
		if pg != (TplBg{}) {
			return false
		}
	}
	return true
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}

// round6 — до 1e-6 (доли области: 2500 мм × 1e-6 — тысячные мм); минус ноль не оставляет.
func round6(v float64) float64 {
	v = math.Round(v*1e6) / 1e6
	if v == 0 {
		return 0
	}
	return v
}

// clampTplBg — фон шаблона: цвет #rrggbb, ClampPattern, ключ библиотеки; прозрачность — только
// при фото, 0…1 шаг 0,01. Pat — копия.
func clampTplBg(b TplBg) TplBg {
	c := TplBg{Color: BgColor(b.Color), Pat: ClampPattern(b.Pat)}
	if ValidLibKey(b.Lib) {
		c.Lib = b.Lib
		c.Fade = geom.Round2(geom.ClampF(Finite(b.Fade, 0), 0, 1))
	}
	return c
}

// clampAnchor — якорь −1…1, сдвиг ±maxTplShift; NaN — false.
func clampAnchor(a Anchor) (Anchor, bool) {
	if !finite(a.DX) || !finite(a.DY) {
		return Anchor{}, false
	}
	return Anchor{
		AX: max(-1, min(1, a.AX)), AY: max(-1, min(1, a.AY)),
		DX: round6(geom.ClampF(a.DX, -maxTplShift, maxTplShift)),
		DY: round6(geom.ClampF(a.DY, -maxTplShift, maxTplShift)),
	}, true
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// tplSize — размер предмета в долях: NaN — false, иначе minTplSize…maxTplSize.
func tplSize(v float64) (float64, bool) {
	if !finite(v) {
		return 0, false
	}
	return round6(geom.ClampF(v, minTplSize, maxTplSize)), true
}

func clampTplItem(it TplItem) (TplItem, bool) {
	a, ok := clampAnchor(it.Anchor)
	w, okW := tplSize(it.W)
	h, okH := tplSize(it.H)
	if !ok || !okW || !okH || (it.Art == nil) == (it.Lib == "") || !finite(it.Rot) {
		return TplItem{}, false
	}
	c := TplItem{Anchor: a, W: w, H: h, Rot: geom.NormDeg(it.Rot), FlipH: it.FlipH, FlipV: it.FlipV, Under: it.Under,
		Fade: geom.Round2(geom.ClampF(Finite(it.Fade, 0), 0, 1))}
	if it.Art != nil {
		if c.Art = ClampArt(it.Art); c.Art == nil {
			return TplItem{}, false
		}
	} else if c.Lib = it.Lib; !ValidLibKey(c.Lib) {
		return TplItem{}, false
	}
	return c, true
}

func clampTplText(tx TplText) (TplText, bool) {
	a, ok := clampAnchor(tx.Anchor)
	w, okW := tplSize(tx.W)
	s := CleanText(tx.Text, MaxTextRunes)
	if !ok || !okW || !finite(tx.Rot) || strings.TrimSpace(s) == "" {
		return TplText{}, false
	}
	return TplText{Anchor: a, W: w, Rot: geom.NormDeg(tx.Rot), Auto: tx.Auto, Text: s, Style: ClampStyle(tx.Style)}, true
}
