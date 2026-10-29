package agents

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Фото ленты: list_photos и get_photo. Пути не идут никогда: id, имя² и номер папки.

type photosIn struct {
	Filter string `json:"filter,omitempty" jsonschema:"какие фото: all, used (на разворотах), unused, missing (файла нет), unreadable, marked (есть оценка, флаг или цвет — своя или из XMP файла)"`
	Folder int    `json:"folder,omitempty" jsonschema:"номер папки из get_album.folders"`
	Bin    string `json:"bin,omitempty" jsonschema:"id подборки"`
	Order  string `json:"order,omitempty" jsonschema:"shot — по времени съёмки, name — по имени (с уровня 2)"`
	Offset int    `json:"offset,omitempty" jsonschema:"сколько записей пропустить (с 0)"`
	Limit  int    `json:"limit,omitempty" jsonschema:"сколько записей вернуть"`
}

func photosProps(props map[string]*jsonschema.Schema) {
	props["filter"].Enum = enum("all", "used", "unused", "missing", "unreadable", "marked")
	props["order"].Enum = enum("shot", "name")
	props["folder"].Minimum = ptr(1.0)
	props["bin"].MaxLength = ptr(maxID)
	pageProps(props, 100, 200)
}

// enum — перечисление для схемы.
func enum(vs ...string) []any {
	out := make([]any, len(vs))
	for i, v := range vs {
		out[i] = v
	}
	return out
}

// pageProps — пределы offset и limit в схеме входа.
func pageProps(props map[string]*jsonschema.Schema, def, top int) {
	props["offset"].Minimum = ptr(0.0)
	props["offset"].Maximum = ptr(1e6)
	props["limit"].Minimum = ptr(1.0)
	props["limit"].Maximum = ptr(float64(top))
	props["limit"].Description += " (по умолчанию " + strconv.Itoa(def) + ", не больше " + strconv.Itoa(top) + ")"
}

type photoItem struct {
	ID     string `json:"id"`
	File   string `json:"file,omitempty"`
	Folder int    `json:"folder" jsonschema:"номер папки с 1; 0 — не в папках альбома"`
	W      int    `json:"w"`
	H      int    `json:"h"`
	Shot   string `json:"shot,omitempty" jsonschema:"время съёмки по часам камеры, без пояса"`
	Bytes  int64  `json:"bytes"`
	Used   bool   `json:"used" jsonschema:"стоит на развороте или обложке"`
	Rating int    `json:"rating" jsonschema:"0–5"`
	Flag   int    `json:"flag" jsonschema:"1 отобрано, −1 отклонено, 0 нет"`
	Label  string `json:"label,omitempty"`
	Status string `json:"status" jsonschema:"ok, missing или unreadable"`
	Color  string `json:"color" jsonschema:"класс цветового профиля: srgb, adobe, p3, icc, other; пусто — профиля нет"`
}

type photosOut struct {
	meta
	Total  int         `json:"total"`
	Offset int         `json:"offset"`
	Items  []photoItem `json:"items"`
}

// photoCtx — снимок данных для фото: ленты, отметки, состояния файлов, вид.
type photoCtx struct {
	c       *call
	v       model.View
	ed      uint64
	byID    map[string]library.Photo
	photos  []library.Photo
	marks   map[string]model.MarkView
	missing map[string]bool
	unread  map[string]bool
	used    map[string]bool
	folders folderIndex
}

func (c *call) photoCtx() (*photoCtx, error) {
	v, ed, err := c.view()
	if err != nil {
		return nil, err
	}
	pc := &photoCtx{c: c, v: v, ed: ed, photos: c.s.h.Photos(), marks: c.s.h.Marks(), used: usedIDs(v),
		missing: map[string]bool{}, folders: newFolderIndex(v.Folders)}
	pc.byID = photoMap(pc.photos)
	miss, unread := c.s.h.Status()
	for _, id := range miss {
		pc.missing[model.BaseID(id)] = true
	}
	pc.unread = map[string]bool{}
	for id := range unread {
		pc.unread[model.BaseID(id)] = true
	}
	return pc, nil
}

func (pc *photoCtx) status(id string) string {
	switch {
	case pc.missing[id]:
		return "missing"
	case pc.unread[id]:
		return "unreadable"
	}
	return "ok"
}

// mark — действующие отметки фото: свои поверх XMP из файла.
func (pc *photoCtx) mark(p library.Photo) (rating, flag int, label string) {
	if x := p.Xmp; x != nil {
		rating, flag, label = x.Rating, x.Flag, x.Label
		if label == "other" {
			label = x.LabelText
		}
	}
	m := pc.marks[p.ID]
	if m.Rating != nil {
		rating = *m.Rating
	}
	if m.Flag != nil {
		flag = *m.Flag
	}
	if m.Label != nil {
		label = *m.Label
	}
	return
}

func (pc *photoCtx) item(p library.Photo) photoItem {
	rating, flag, label := pc.mark(p)
	it := photoItem{ID: cleanID(p.ID), File: pc.c.text(p.Name, maxFile), Folder: pc.folders.of(p.Path), W: p.W, H: p.H,
		Bytes: p.Size, Used: pc.used[p.ID], Rating: min(max(rating, 0), 5), Flag: min(max(flag, -1), 1),
		Label: pc.c.text(label, maxLabel), Status: pc.status(p.ID), Color: cleanID(string(p.Color))}
	if p.Shot > 0 {
		it.Shot = time.Unix(p.Shot, 0).UTC().Format("2006-01-02T15:04:05")
	}
	return it
}

// marked — у фото есть своя отметка (оценка, флаг, цвет).
func (pc *photoCtx) marked(p library.Photo) bool {
	r, f, l := pc.mark(p)
	return r != 0 || f != 0 || l != ""
}

func (pc *photoCtx) keep(p library.Photo, in photosIn, back map[string]bool) bool {
	if in.Folder > 0 && pc.folders.of(p.Path) != in.Folder {
		return false
	}
	if in.Bin != "" {
		ok := false
		for _, b := range pc.marks[p.ID].Bins {
			ok = ok || b == in.Bin
		}
		if !ok {
			return false
		}
	}
	switch in.Filter {
	case "used":
		return pc.used[p.ID]
	case "unused":
		return !pc.used[p.ID] && !back[p.ID]
	case "missing":
		return pc.missing[p.ID]
	case "unreadable":
		return pc.unread[p.ID]
	case "marked":
		return pc.marked(p)
	}
	return true
}

func (c *call) listPhotos(in photosIn) (photosOut, error) {
	pc, err := c.photoCtx()
	if err != nil {
		return photosOut{}, err
	}
	back := map[string]bool{}
	for _, id := range pc.v.BackdropUsed {
		back[model.BaseID(id)] = true
	}
	list := make([]library.Photo, 0, len(pc.photos))
	for _, p := range pc.photos {
		if pc.keep(p, in, back) {
			list = append(list, p)
		}
	}
	if in.Filter == "missing" { // пропавшие, которых нет в ленте: только id
		for id := range pc.missing {
			if _, ok := pc.byID[id]; !ok && in.Folder == 0 && in.Bin == "" {
				list = append(list, library.Photo{ID: id})
			}
		}
	}
	sortPhotos(list, in.Order == "name" && c.names())
	out := photosOut{meta: c.metaOf(pc.v, pc.ed), Total: len(list), Offset: in.Offset, Items: []photoItem{}}
	lim := defLimit(in.Limit, 100)
	for i := in.Offset; i < len(list) && len(out.Items) < lim; i++ {
		out.Items = append(out.Items, pc.item(list[i]))
	}
	return out, nil
}

// sortPhotos — порядок по имени или по времени съёмки (без времени — в конце); при равенстве по id.
func sortPhotos(l []library.Photo, byName bool) {
	sort.SliceStable(l, func(i, j int) bool {
		a, b := l[i], l[j]
		if byName {
			if x, y := strings.ToLower(a.Name), strings.ToLower(b.Name); x != y {
				return x < y
			}
		} else if a.Shot != b.Shot {
			return a.Shot != 0 && (b.Shot == 0 || a.Shot < b.Shot)
		}
		return a.ID < b.ID
	})
}

type useOut struct {
	N      int    `json:"n" jsonschema:"номер разворота с 1; 0 — обложка"`
	Spread string `json:"spread"`
}

type binOut struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type photoOut struct {
	meta
	photoItem
	Uses    []useOut `json:"uses"`
	Profile string   `json:"profile,omitempty"`
	Bins    []binOut `json:"bins"`
}

type idIn struct {
	ID string `json:"id" jsonschema:"id фото из list_photos"`
}

func (c *call) getPhoto(id string) (photoOut, error) {
	pc, err := c.photoCtx()
	if err != nil {
		return photoOut{}, err
	}
	id = model.BaseID(strings.TrimSpace(id))
	p, ok := pc.byID[id]
	if !ok {
		if !pc.missing[id] {
			return photoOut{}, errNoPhoto
		}
		p = library.Photo{ID: id}
	}
	out := photoOut{meta: c.metaOf(pc.v, pc.ed), photoItem: pc.item(p), Uses: []useOut{}, Bins: []binOut{},
		Profile: c.text(p.Profile, maxFile)}
	scan := func(n int, sv model.SpreadView) {
		for _, pid := range sv.Photos {
			if model.BaseID(pid) == id {
				out.Uses = append(out.Uses, useOut{N: n, Spread: cleanID(sv.ID)})
				return
			}
		}
	}
	if pc.v.Cover != nil {
		scan(0, pc.v.Cover.Sheet)
	}
	for i, sv := range pc.v.Spreads {
		scan(i+1, sv)
	}
	names := map[string]string{}
	for _, b := range pc.v.Bins {
		names[b.ID] = b.Name
	}
	for _, b := range pc.marks[id].Bins {
		out.Bins = append(out.Bins, binOut{ID: cleanID(b), Name: c.text(names[b], maxFile)})
	}
	return out, nil
}
