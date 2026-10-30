package agents

import (
	"errors"
	"math"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// get_contact_sheet (151, design §1.1): лист-сетка миниатюр с номерами. Служба выбирает фото (выборка
// list_photos или названные ids), помечает пропавшие и нечитаемые, а лист рисует фасад (Host.Sheet);
// его JPEG служба перекодирует сама, как любую картинку (image).

// Пределы листа.
const (
	maxSheetIDs  = 64   // фото в ids: 8 × 8
	sheetDefGrid = 6    // клеток в стороне по умолчанию
	sheetMinGrid = 3    // ниже — клетка больше миниатюры 400 px
	sheetMaxGrid = 8    // выше — клетка мала для глаз
	sheetDefSide = 1056 // ≈ 1,1 Мп: клиенты уменьшают картинки крупнее (0.34-151-1)
	sheetMinSide = 768
	sheetMaxSide = 1568
)

type sheetIn struct {
	Filter string   `json:"filter,omitempty" jsonschema:"какие фото: all, used (на разворотах), unused, marked (есть оценка, флаг или цвет); с ids не годится"`
	Folder int      `json:"folder,omitempty" jsonschema:"номер папки из get_album.folders; с ids не годится"`
	Bin    string   `json:"bin,omitempty" jsonschema:"id подборки; с ids не годится"`
	Order  string   `json:"order,omitempty" jsonschema:"shot — по времени съёмки, name — по имени (с уровня 2); с ids не годится"`
	Offset int      `json:"offset,omitempty" jsonschema:"сколько фото пропустить: next из прошлого ответа (с 0); с ids не годится"`
	IDs    []string `json:"ids,omitempty" jsonschema:"до 64 id фото: лист из них в этом порядке (сравнить кандидатов); вместо фильтров"`
	Grid   int      `json:"grid,omitempty" jsonschema:"клеток в стороне листа (по умолчанию 6; с ids — по числу фото)"`
	Size   int      `json:"size,omitempty" jsonschema:"сторона листа, px (по умолчанию 1056)"`
}

func sheetProps(p map[string]*jsonschema.Schema) {
	p["filter"].Enum = enum("all", "used", "unused", "marked") // missing и unreadable — лист из одних заглушек
	p["order"].Enum = enum("shot", "name")
	p["folder"].Minimum = ptr(1.0)
	p["bin"].MaxLength = ptr(maxID)
	p["offset"].Minimum, p["offset"].Maximum = ptr(0.0), ptr(1e6)
	p["ids"].MinItems, p["ids"].MaxItems = ptr(1), ptr(maxSheetIDs)
	p["ids"].Items = &jsonschema.Schema{Type: "string", MinLength: ptr(1), MaxLength: ptr(maxID)}
	p["grid"].Minimum, p["grid"].Maximum = ptr(float64(sheetMinGrid)), ptr(float64(sheetMaxGrid))
	p["size"].Minimum, p["size"].Maximum = ptr(float64(sheetMinSide)), ptr(float64(sheetMaxSide))
}

// sheetCellOut — строка таблицы «номер → фото». Имён файлов нет: лист уходит облаку (У43).
type sheetCellOut struct {
	N      int    `json:"n" jsonschema:"номер на листе"`
	ID     string `json:"id" jsonschema:"id фото: правки только по нему"`
	Used   bool   `json:"used,omitempty" jsonschema:"стоит на развороте или обложке"`
	Flag   int    `json:"flag,omitempty" jsonschema:"1 отобрано, −1 отклонено"`
	Rating int    `json:"rating,omitempty" jsonschema:"0–5"`
	Status string `json:"status,omitempty" jsonschema:"missing или unreadable; пусто — файл в порядке"`
}

type sheetOut struct {
	meta
	Total   int            `json:"total" jsonschema:"фото в выборке"`
	From    int            `json:"from" jsonschema:"номер первой клетки"`
	To      int            `json:"to" jsonschema:"номер последней клетки"`
	Next    int            `json:"next" jsonschema:"offset следующего листа; 0 — это последний"`
	Cells   []sheetCellOut `json:"cells"`
	Pending []int          `json:"pending,omitempty" jsonschema:"номера клеток, чьи миниатюры ещё готовятся: повторите вызов"`
	Broken  []int          `json:"broken,omitempty" jsonschema:"номера клеток, чей файл пропал или не читается"`
	W       int            `json:"w" jsonschema:"ширина картинки, px"`
	H       int            `json:"h" jsonschema:"высота картинки, px"`
	img     []byte
}

func (o sheetOut) jpegBytes() []byte { return o.img }

func registerSheet(e *toolEnv) {
	reg(e, toolSpec{name: "get_contact_sheet", perm: prefs.ClassRead, title: "Лист фото", minLevel: 3, slow: true,
		desc: "Лист-сетка из миниатюр фото с номерами 1…N (JPEG без метаданных и без имён файлов): так выбирают кадры из сотен, не вызывая " +
			"get_photo_preview на каждое. Выборка как у list_photos (filter: all, used, unused, marked; folder, bin, order; offset — следующий лист, " +
			"next в ответе) или до 64 названных фото в ids (сравнить кандидатов; с ids фильтры и offset не годятся). В ответе таблица cells: n → id " +
			"с отметками фото; правки — только по id, номера — для разговора с человеком. Номера считаются по выборке сейчас: листайте до правок " +
			"(после отметок и сборки unused сдвигается), сравнивать — через ids. Клетка мала для глаз и резкости — кадр обложки проверьте " +
			"get_photo_preview. pending — миниатюра ещё готовится (повторите вызов), broken — файла нет или он не читается. " +
			"Только при включённом «+ картинки»." + dataNote},
		sheetProps, func(c *call, in sheetIn) (sheetOut, error) { return c.getContactSheet(in) })
}

// sheetPick — что кладём на лист: фото в порядке клеток и номер первой клетки.
type sheetPick struct {
	list  []library.Photo
	first int // номер первой клетки с 1
	total int // фото в выборке
	grid  int
}

// pick выбирает фото листа: названные ids или страница выборки list_photos.
func (pc *photoCtx) pick(in sheetIn) (sheetPick, error) {
	if len(in.IDs) == 0 {
		g := defInt(in.Grid, sheetDefGrid)
		list := pc.selected(photosIn{Filter: in.Filter, Folder: in.Folder, Bin: in.Bin, Order: in.Order})
		if in.Offset >= len(list) {
			return sheetPick{}, errBadPage
		}
		return sheetPick{list: list[in.Offset:min(in.Offset+g*g, len(list))], first: in.Offset + 1, total: len(list), grid: g}, nil
	}
	if in.Filter != "" || in.Folder != 0 || in.Bin != "" || in.Order != "" || in.Offset != 0 {
		return sheetPick{}, badArgs("с ids нельзя filter, folder, bin, order и offset: лист собирается из названных фото")
	}
	seen := map[string]bool{}
	var list []library.Photo
	for _, raw := range in.IDs {
		id := model.BaseID(strings.TrimSpace(raw))
		if seen[id] {
			continue
		}
		seen[id] = true
		p, ok := pc.byID[id]
		switch {
		case ok:
		case pc.missing[id]: // пропавшее вне ленты: клетка «×»
			p = library.Photo{ID: id}
		default:
			return sheetPick{}, errNoPhoto
		}
		list = append(list, p)
	}
	g := in.Grid
	if need := int(math.Ceil(math.Sqrt(float64(len(list))))); g == 0 {
		g = min(max(need, sheetMinGrid), sheetMaxGrid)
	} else if g*g < len(list) {
		return sheetPick{}, errBadPage
	}
	return sheetPick{list: list, first: 1, total: len(list), grid: g}, nil
}

func (c *call) getContactSheet(in sheetIn) (sheetOut, error) {
	pc, err := c.photoCtx()
	if err != nil {
		return sheetOut{}, err
	}
	pk, err := pc.pick(in)
	if err != nil {
		return sheetOut{}, err
	}
	select { // лист рисуется в одном месте с рендером разворота: оба тяжёлые
	case c.env.render <- struct{}{}:
		defer func() { <-c.env.render }()
	default:
		return sheetOut{}, errCallBusy
	}
	ids := make([]string, len(pk.list))
	for i, p := range pk.list {
		if pc.status(p.ID) == "ok" {
			ids[i] = p.ID
		}
	}
	side := defInt(in.Size, sheetDefSide)
	var states []string
	b, w, h, err := c.image(func(w, h int) bool { return max(w, h) <= side }, func() (jpg []byte, err error) {
		jpg, states, err = c.s.h.Sheet(c.ctx, SheetReq{IDs: ids, First: pk.first, Grid: pk.grid, Side: side})
		return jpg, err
	})
	if err != nil {
		return sheetOut{}, err
	}
	if len(states) != len(ids) {
		return sheetOut{}, errors.New("agents: Host.Sheet вернул не по клетке на фото")
	}
	out := sheetOut{meta: c.metaOf(pc.v, pc.ed), Total: pk.total, From: pk.first, To: pk.first + len(ids) - 1,
		Cells: make([]sheetCellOut, len(ids)), W: w, H: h, img: b}
	if len(in.IDs) == 0 && in.Offset+len(ids) < pk.total {
		out.Next = in.Offset + len(ids)
	}
	for i, p := range pk.list {
		n := pk.first + i
		rating, flag, _ := pc.mark(p)
		cell := sheetCellOut{N: n, ID: cleanID(p.ID), Used: pc.used[p.ID], Flag: min(max(flag, -1), 1), Rating: min(max(rating, 0), 5)}
		if st := pc.status(p.ID); st != "ok" {
			cell.Status = st
		}
		out.Cells[i] = cell
		switch {
		case states[i] == CellPending && ids[i] != "":
			out.Pending = append(out.Pending, n)
		case ids[i] == "" || states[i] != CellOK: // неизвестное состояние от Host — как «×», не как готовое
			out.Broken = append(out.Broken, n)
		}
	}
	return out, nil
}
