package agents

import (
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Библиотека: оформление, наборы, профили типографий, «Последние». Свои предметы названы человеком
// или взяты из имени файла, поэтому на уровне 1 вместо имени — «Свой фон 3» (номер по порядку в
// каталоге), а подробности — с уровня 2 (design §3 «Своё — тоже имена»).

// ownPrefix — как называть своё на уровне 1, по виду каталога.
var ownPrefix = map[string]string{
	"bg": "Свой фон", "sticker": "Свой стикер", "frame": "Своя рамка",
	"spread": "Свой разворот", "look": "Мой макет", "template": "Мой шаблон",
}

const builtinPack = "sa:builtin"

// decorCat — каталог вида на один вызов: готовые для агента имена и признак «своё».
type decorCat struct {
	items []DecorItemLite
	label []string // имя для агента: на уровне 1 — «Свой фон N» у своего
	mask  []bool   // имя взято у человека (своё или из набора-файла)
	byID  map[string]int
	raw   DecorCatalogLite
}

// trustedOrigin — откуда набор, чьим названиям можно верить: встроенный, из подписанного каталога
// (сеть) и пример. Всё прочее (файл, неизвестное) — от человека.
func trustedOrigin(o string) bool { return o == "builtin" || o == "net" || o == "demo" }

// trustedPacks — id стоящих наборов из доверенного источника; ok false — наборы не прочитались
// (тогда своими считаются все, кроме встроенных).
func (c *call) trustedPacks() (set map[string]bool, ok bool) {
	p, err := c.s.h.Packs()
	if err != nil {
		return nil, false
	}
	set = map[string]bool{}
	for _, in := range p.Installed {
		if trustedOrigin(in.Origin) {
			set[in.ID] = true
		}
	}
	return set, true
}

// catalog читает каталог вида и готовит имена; ошибка Host идёт наверх.
func (c *call) catalog(kind string) (*decorCat, error) {
	if dc, ok := c.cats[kind]; ok {
		return dc, nil
	}
	raw, err := c.s.h.Decor(kind)
	if err != nil {
		return nil, err
	}
	trusted, known := c.trustedPacks()
	dc := &decorCat{items: raw.Items, byID: map[string]int{}, raw: raw}
	n := 0
	for i, it := range raw.Items {
		own := it.Own || kind == "template" // «Мои шаблоны» называет человек, что бы ни пришло в Own
		if it.PackID != "" && it.PackID != builtinPack {
			own = own || !known || !trusted[it.PackID]
		}
		name := cleanLine(it.Name, maxFile)
		if own {
			n++
			if !c.names() {
				name = ownPrefix[kind] + " " + strconv.Itoa(n)
			}
		}
		dc.label, dc.mask = append(dc.label, name), append(dc.mask, own)
		dc.byID[it.ID] = i
	}
	if c.cats == nil {
		c.cats = map[string]*decorCat{}
	}
	c.cats[kind] = dc
	return dc, nil
}

// libName — имя предмета каталога kind по id (ключ своего из альбома); нет в каталоге — на уровне 1
// общее «Свой фон», на уровне ≥ 2 пусто.
func (c *call) libName(kind, id string) string {
	if dc, err := c.catalog(kind); err == nil {
		if i, ok := dc.byID[id]; ok {
			return dc.label[i]
		}
	}
	if c.names() {
		return ""
	}
	return ownPrefix[kind]
}

type decorIn struct {
	Kind    string `json:"kind" jsonschema:"вид: bg (фоны), sticker, frame (рамочки), spread (развороты), look (макеты альбома), template (мои шаблоны)"`
	Theme   string `json:"theme,omitempty" jsonschema:"id темы из themes"`
	Query   string `json:"query,omitempty" jsonschema:"часть названия"`
	InAlbum bool   `json:"inAlbum,omitempty" jsonschema:"только то, что стоит в открытом альбоме (bg, sticker, frame, look)"`
	Offset  int    `json:"offset,omitempty" jsonschema:"сколько записей пропустить (с 0)"`
	Limit   int    `json:"limit,omitempty" jsonschema:"сколько записей вернуть"`
}

func decorProps(props map[string]*jsonschema.Schema) {
	props["kind"].Enum = enum("bg", "sticker", "frame", "spread", "look", "template")
	props["theme"].MaxLength = ptr(maxID)
	props["query"].MaxLength = ptr(100)
	pageProps(props, 100, 200)
}

type themeOut struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type decorOut struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Theme  string `json:"theme"`
	Pack   string `json:"pack,omitempty"`
	Own    bool   `json:"own"`
	New    bool   `json:"new"`
	W      int    `json:"w"`
	H      int    `json:"h"`
	Pages  int    `json:"pages"`
	Places int    `json:"places"`
	Steps  int    `json:"steps"`
	Vig    bool   `json:"vig"`
	Cover  bool   `json:"cover"`
}

type decorListOut struct {
	meta
	Total  int        `json:"total"`
	Offset int        `json:"offset"`
	Themes []themeOut `json:"themes"`
	Items  []decorOut `json:"items"`
}

// inAlbumIDs — что из каталога kind стоит в альбоме.
func inAlbumIDs(v model.View, kind string) (map[string]bool, error) {
	set := map[string]bool{}
	sheets := append([]model.SpreadView{}, v.Spreads...)
	if v.Cover != nil {
		sheets = append(sheets, v.Cover.Sheet)
	}
	var ids []string
	switch kind {
	case "bg":
		ids = bgIDs(v, sheets)
	case "sticker":
		for _, sv := range sheets {
			for _, s := range sv.Stickers {
				ids = append(ids, s.Lib)
			}
		}
	case "frame":
		for _, sv := range sheets {
			for _, p := range sv.Places {
				if p.Border != nil {
					ids = append(ids, p.Border.Lib, p.Border.Preset)
				}
			}
		}
	case "look":
		if v.Look != nil {
			ids = append(ids, v.Look.Key)
		}
	default:
		return nil, errBadKind
	}
	for _, id := range ids {
		if id != "" {
			set[id] = true
		}
	}
	return set, nil
}

// bgIDs — плитки фонов альбома: свои фото фона, узоры, фон альбома.
func bgIDs(v model.View, sheets []model.SpreadView) (ids []string) {
	for _, sv := range sheets {
		for _, b := range sv.Backdrops {
			ids = append(ids, b.Lib)
		}
		for _, p := range sv.Patterns {
			ids = append(ids, p.Preset)
		}
	}
	if a := v.AlbumBg; a != nil {
		ids = append(ids, a.Lib, a.Preset)
	}
	return ids
}

func (c *call) listDecor(in decorIn) (decorListOut, error) {
	dc, err := c.catalog(in.Kind)
	if err != nil {
		return decorListOut{}, err
	}
	var in2 map[string]bool
	var v model.View
	var ed uint64
	if in.InAlbum {
		if v, ed, err = c.view(); err != nil {
			return decorListOut{}, err
		}
		if in2, err = inAlbumIDs(v, in.Kind); err != nil {
			return decorListOut{}, err
		}
	}
	isNew := map[string]bool{}
	for _, id := range dc.raw.New {
		isNew[id] = true
	}
	q := strings.ToLower(strings.TrimSpace(in.Query))
	out := decorListOut{Offset: in.Offset, Themes: []themeOut{}, Items: []decorOut{}}
	if in.InAlbum {
		out.meta = c.metaOf(v, ed)
	} else {
		out.meta = c.meta()
	}
	for _, t := range dc.raw.Themes {
		if t.ID != themeGet { // образцы нескачанного агенту не показываем
			out.Themes = append(out.Themes, themeOut{ID: cleanID(t.ID), Name: cleanLine(t.Name, maxFile), Count: t.Count})
		}
	}
	lim := defLimit(in.Limit, 100)
	for i, it := range dc.items {
		switch {
		case in.Theme != "" && it.Theme != in.Theme,
			q != "" && !strings.Contains(strings.ToLower(dc.label[i]), q), // по имени, как его видит агент
			in.InAlbum && !in2[it.ID]:
			continue
		}
		out.Total++
		if out.Total <= in.Offset || len(out.Items) >= lim {
			continue
		}
		pack := ""
		if c.names() || !dc.mask[i] { // название набора из файла — от человека
			pack = cleanLine(it.Pack, maxFile)
		}
		out.Items = append(out.Items, decorOut{ID: cleanID(it.ID), Name: dc.label[i], Theme: cleanID(it.Theme), Pack: pack,
			Own: it.Own, New: isNew[it.ID], W: it.W, H: it.H, Pages: it.Pages, Places: it.Places, Steps: it.Steps, Vig: it.Vig, Cover: it.Cover})
	}
	return out, nil
}

// themeGet — тема «Можно скачать» (144).
const themeGet = "get"
