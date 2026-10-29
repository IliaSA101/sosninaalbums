package agents

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Развороты: list_spreads и get_spread. Размеры — мм, координаты — от левого верхнего угла разворота.

type pagingIn struct {
	Offset int `json:"offset,omitempty" jsonschema:"сколько записей пропустить (с 0)"`
	Limit  int `json:"limit,omitempty" jsonschema:"сколько записей вернуть"`
}

type spreadItem struct {
	N        int       `json:"n" jsonschema:"номер разворота с 1"`
	ID       string    `json:"id"`
	Name     string    `json:"name,omitempty"`
	Kind     string    `json:"kind" jsonschema:"spread или vignette"`
	Layout   string    `json:"layout" jsonschema:"ключ раскладки; grid — свободные места"`
	Photos   int       `json:"photos"`
	Places   int       `json:"places"`
	Empty    int       `json:"empty" jsonschema:"пустых мест"`
	Texts    int       `json:"texts"`
	Stickers int       `json:"stickers"`
	Bg       [2]string `json:"bg" jsonschema:"цвет левой и правой страницы"`
	LowDPI   int       `json:"lowDPI" jsonschema:"фото с малым разрешением для печати"`
	LookStep int       `json:"lookStep" jsonschema:"шаг макета, которым поставлен разворот; 0 — не макетом"`
}

type spreadsOut struct {
	meta
	Total  int          `json:"total"`
	Offset int          `json:"offset"`
	Items  []spreadItem `json:"items"`
}

// spreadKind — вид листа для агента.
func spreadKind(sv model.SpreadView) string {
	if sv.Kind == model.KindVignette {
		return "vignette"
	}
	return "spread"
}

func spreadItemOf(c *call, n int, sv model.SpreadView) spreadItem {
	empty := 0
	for _, p := range sv.Places {
		if p.Photo == "" {
			empty++
		}
	}
	places := len(sv.Places)
	if !sv.Grid {
		places = len(sv.Rects)
	}
	layout := cleanKey(sv.Layout)
	if sv.Grid {
		layout = "grid"
	}
	return spreadItem{N: n, ID: cleanID(sv.ID), Name: c.text(sv.Name, maxFile), Kind: spreadKind(sv), Layout: layout,
		Photos: len(sv.Photos), Places: places, Empty: empty, Texts: len(sv.Texts), Stickers: len(sv.Stickers),
		Bg: [2]string{model.BgColor(sv.Bg[0]), model.BgColor(sv.Bg[1])}, LowDPI: sv.LowDPI, LookStep: sv.LookStep}
}

// listSpreads — страница разворотов (без обложки: её отдаёт get_spread «cover»).
func (c *call) listSpreads(in pagingIn) (spreadsOut, error) {
	v, ed, err := c.view()
	if err != nil {
		return spreadsOut{}, err
	}
	lim := defLimit(in.Limit, 50)
	out := spreadsOut{meta: c.metaOf(v, ed), Total: len(v.Spreads), Offset: in.Offset, Items: []spreadItem{}}
	for i := in.Offset; i < len(v.Spreads) && len(out.Items) < lim; i++ {
		out.Items = append(out.Items, spreadItemOf(c, i+1, v.Spreads[i]))
	}
	return out, nil
}

// defLimit — limit из входа или значение по умолчанию (схема уже проверила пределы).
func defLimit(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// spreadRef — разворот во входе: номер (число или строка), «cover» или id.
type spreadRef string

func (r *spreadRef) UnmarshalJSON(b []byte) error {
	var n json.Number
	if err := json.Unmarshal(b, &n); err == nil {
		*r = spreadRef(n.String())
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*r = spreadRef(s)
	return nil
}

// spreadProp — схема поля spread: строка или целое.
func spreadProp(props map[string]*jsonschema.Schema, desc string) {
	props["spread"] = &jsonschema.Schema{Types: []string{"string", "integer"}, Description: desc, MaxLength: ptr(maxID)}
}

func ptr[T any](v T) *T { return &v }

// findSpread ищет разворот: «cover» — обложка, число — номер с 1, иначе id. n 0 — обложка.
func findSpread(v model.View, ref string) (sv model.SpreadView, n int, ok bool) {
	ref = strings.TrimSpace(ref)
	if strings.EqualFold(ref, "cover") {
		if v.Cover == nil {
			return sv, 0, false
		}
		return v.Cover.Sheet, 0, true
	}
	if k, err := strconv.Atoi(ref); err == nil {
		if k >= 1 && k <= len(v.Spreads) {
			return v.Spreads[k-1], k, true
		}
		return sv, 0, false
	}
	for i, s := range v.Spreads {
		if s.ID == ref || cleanID(s.ID) == ref {
			return s, i + 1, true
		}
	}
	return sv, 0, false
}

type frameOut struct {
	Photo string  `json:"photo"`
	File  string  `json:"file,omitempty"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Rot   float64 `json:"rot"`
	DPI   int     `json:"dpi" jsonschema:"0 — размер фото неизвестен"`
	Gray  bool    `json:"gray"`
	FlipH bool    `json:"flipH"`
	FlipV bool    `json:"flipV"`
}

type placeOut struct {
	ID    string  `json:"id"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Rot   float64 `json:"rot"`
	Photo string  `json:"photo,omitempty"`
	Shape string  `json:"shape"`
	Role  string  `json:"role"`
}

type textOut struct {
	ID    string  `json:"id"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Text  string  `json:"text,omitempty"`
	Font  string  `json:"font"`
	Size  float64 `json:"size" jsonschema:"pt"`
	Draft bool    `json:"draft" jsonschema:"заготовка: в печать не идёт"`
}

type captionOut struct {
	Photo string `json:"photo"`
	Text  string `json:"text,omitempty"`
}

type stickerOut struct {
	ID    string  `json:"id"`
	Decor string  `json:"decor" jsonschema:"id плитки каталога; пусто — процедурный"`
	Name  string  `json:"name"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
}

type backgroundOut struct {
	Scope string `json:"scope" jsonschema:"left, right, spread, spine или album"`
	Kind  string `json:"kind" jsonschema:"color, photo или pattern"`
	Decor string `json:"decor,omitempty"`
	Name  string `json:"name,omitempty"`
	Color string `json:"color,omitempty"`
	Photo string `json:"photo,omitempty"`
}

type spreadOut struct {
	meta
	N           int             `json:"n" jsonschema:"номер разворота с 1; 0 — обложка"`
	Cover       bool            `json:"cover"`
	ID          string          `json:"id"`
	Name        string          `json:"name,omitempty"`
	Kind        string          `json:"kind"`
	W           float64         `json:"w"`
	H           float64         `json:"h"`
	Mid         float64         `json:"mid" jsonschema:"сгиб, мм от левого края; 0 — нет"`
	Frames      []frameOut      `json:"frames"`
	Places      []placeOut      `json:"places"`
	Texts       []textOut       `json:"texts"`
	Captions    []captionOut    `json:"captions"`
	Stickers    []stickerOut    `json:"stickers"`
	Backgrounds []backgroundOut `json:"backgrounds"`
}

func (c *call) getSpread(ref string) (spreadOut, error) {
	v, ed, err := c.view()
	if err != nil {
		return spreadOut{}, err
	}
	sv, n, ok := findSpread(v, ref)
	if !ok {
		return spreadOut{}, errNoSpread
	}
	byID := photoMap(c.s.h.Photos())
	out := spreadOut{meta: c.metaOf(v, ed), N: n, Cover: n == 0, ID: cleanID(sv.ID), Name: c.text(sv.Name, maxFile),
		Kind: spreadKind(sv), W: sv.W, H: sv.H, Mid: sv.Mid,
		Frames: []frameOut{}, Places: []placeOut{}, Texts: []textOut{}, Captions: []captionOut{},
		Stickers: []stickerOut{}, Backgrounds: []backgroundOut{}}
	if n == 0 {
		out.Kind = "cover"
	}
	for i, id := range sv.Photos {
		f := frameOut{Photo: cleanID(id), File: c.text(photoName(byID, id), maxFile)}
		if i < len(sv.Rects) {
			r := sv.Rects[i]
			f.X, f.Y, f.W, f.H = r.X, r.Y, r.W, r.H
		}
		if i < len(sv.Frames) {
			fv := sv.Frames[i]
			f.Rot, f.DPI, f.Gray, f.FlipH, f.FlipV = fv.CellRot, fv.DPI, fv.Gray, fv.FlipH, fv.FlipV
		}
		out.Frames = append(out.Frames, f)
	}
	for _, p := range sv.Places {
		out.Places = append(out.Places, placeOut{ID: cleanID(p.ID), X: p.X, Y: p.Y, W: p.W, H: p.H, Rot: p.Rot,
			Photo: cleanID(p.Photo), Shape: cleanID(p.Shape), Role: cleanID(p.Role)})
	}
	for _, t := range sv.Texts {
		out.Texts = append(out.Texts, textOut{ID: cleanID(t.ID), X: t.X, Y: t.Y, W: t.W, H: t.H, Text: c.shown(t.Text, t.Src),
			Font: cleanID(t.Style.Font), Size: t.Style.Size, Draft: t.Draft})
	}
	for _, cp := range sv.Captions {
		out.Captions = append(out.Captions, captionOut{Photo: cleanID(cp.Photo), Text: c.shown(cp.Text, cp.Src)})
	}
	for _, s := range sv.Stickers {
		out.Stickers = append(out.Stickers, c.stickerOf(s))
	}
	out.Backgrounds = c.backgroundsOf(sv, byID)
	return out, nil
}

// shown — текст блока или подписи для агента (уровень ≥ 2). Если в исходном тексте есть поля имени
// ({Фамилия}, {Имя}…), раскрытый вид содержит имя из таблицы имён, а она наружу не идёт ни на
// каком уровне: отдаётся шаблон.
func (c *call) shown(text, src string) string {
	if src != "" && len(model.NameFieldsOf(src)) > 0 {
		text = src
	}
	return c.text(text, maxText)
}

func (c *call) stickerOf(s model.StickerView) stickerOut {
	o := stickerOut{ID: cleanID(s.ID), Decor: cleanID(s.Lib), X: s.X, Y: s.Y, W: s.W, H: s.H}
	if s.Lib != "" {
		o.Name = c.libName("sticker", s.Lib)
	} else {
		o.Name = cleanLine(s.Name, maxFile)
	}
	return o
}

func (c *call) backgroundsOf(sv model.SpreadView, byID map[string]library.Photo) []backgroundOut {
	out := []backgroundOut{}
	for i, scope := range [2]string{"left", "right"} {
		if model.BgColor(sv.Bg[i]) != "" {
			out = append(out, backgroundOut{Scope: scope, Kind: "color", Color: model.BgColor(sv.Bg[i])})
		}
	}
	for _, b := range sv.Backdrops {
		o := backgroundOut{Scope: cleanID(b.Scope), Kind: "photo", Photo: cleanID(b.Photo), Decor: cleanID(b.Lib)}
		if b.Album {
			o.Scope = "album"
		}
		if b.Lib != "" {
			o.Name = c.libName("bg", b.Lib)
		} else {
			o.Name = c.text(photoName(byID, b.Photo), maxFile)
		}
		out = append(out, o)
	}
	for _, p := range sv.Patterns {
		o := backgroundOut{Scope: cleanID(p.Scope), Kind: "pattern", Decor: cleanID(p.Preset), Name: cleanLine(p.Name, maxFile)}
		if p.Album {
			o.Scope = "album"
		}
		out = append(out, o)
	}
	return out
}

func photoMap(ps []library.Photo) map[string]library.Photo {
	m := make(map[string]library.Photo, len(ps))
	for _, p := range ps {
		m[p.ID] = p
	}
	return m
}
