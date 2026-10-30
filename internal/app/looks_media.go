package app

import (
	"bytes"
	"image"
	"image/draw"
	"log"
	"net/http"
	"net/url"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
)

// Плитка макета альбома: /media/look/<16 hex>?v=1&i=&px=&r=&bg=0|1[&f=0|1] (131 design §8), по образцу
// /media/tpl: i — 0 обложка, 1…MaxLookSteps шаг; px — ширина в точках; r — ширина/высота картинки
// (места по LookBoxes для r); bg=1 — только фон шага; f=1 — только лицевая сторона обложки (плитка
// карточки; r — отношение самой стороны), только при i = 0. Ключ — только из индекса библиотеки.
const lookMaxStep = model.MaxLookSteps

// lookQuery — разбор запроса плитки: v, i, px, r, bg, f; каждый один раз, i, px, r обязательны.
type lookQuery struct {
	tplQuery
	i     int
	bg    bool
	front bool // f=1: только лицевая сторона обложки
}

// parseFlag — "0" или "1"; остальное — не флаг.
func parseFlag(s string) (v, ok bool) {
	return s == "1", s == "0" || s == "1"
}

func parseLookQuery(raw string) (lookQuery, bool) {
	vals, err := url.ParseQuery(raw)
	if err != nil {
		return lookQuery{}, false
	}
	var q lookQuery
	haveI, ok := false, false
	for k, v := range vals {
		if len(v) != 1 {
			return lookQuery{}, false
		}
		switch k {
		case "v":
			if _, err := strconv.ParseUint(v[0], 10, 16); err != nil {
				return lookQuery{}, false
			}
		case "i":
			n, err := strconv.Atoi(v[0])
			if err != nil || n < 0 || n > lookMaxStep {
				return lookQuery{}, false
			}
			q.i, haveI = n, true
		case "px":
			n, err := strconv.Atoi(v[0])
			if err != nil || n < tplMinPx || n > tplMaxPx {
				return lookQuery{}, false
			}
			q.px = n
		case "r":
			f, err := strconv.ParseFloat(v[0], 64)
			if err != nil || !(f >= tplMinR && f <= tplMaxR) {
				return lookQuery{}, false
			}
			q.r = f
		case "bg":
			if q.bg, ok = parseFlag(v[0]); !ok {
				return lookQuery{}, false
			}
		case "f":
			if q.front, ok = parseFlag(v[0]); !ok {
				return lookQuery{}, false
			}
		default:
			return lookQuery{}, false
		}
	}
	return q, haveI && q.px > 0 && q.r > 0 && (!q.front || q.i == 0)
}

// lookTile — шаблоны плитки шага i макета t на листе с отношением сторон r: один, у обложки без
// front — два (задняя и лицевая стороны рядом слева направо). bg — только фон, front — только
// лицевая сторона обложки (r — её отношение). false — шага или обложки нет.
func lookTile(t model.AlbumTpl, i int, bg, front bool, r float64) ([]model.SpreadTpl, bool) {
	if i == 0 {
		return lookCoverTiles(t.Cover, bg, front, r)
	}
	if i < 1 || i > len(t.Steps) {
		return nil, false
	}
	st := t.Steps[i-1]
	tpl := st.Tpl
	switch {
	case bg:
		tpl.Places, tpl.Borders, tpl.Stickers, tpl.Texts, tpl.Drafts = nil, nil, nil, nil, nil
	case st.Role == model.RoleVignette:
		tpl.Places, tpl.Borders = nil, nil // места листа 031 строит виньетка, не шаг
	default:
		tpl.Places = model.LookBoxes(tpl, r)
	}
	return []model.SpreadTpl{tpl}, true
}

// lookCoverTiles — обложка: стороны строит model.CoverTpl.SideTpl, как library-verify (фон по
// CoverPageBg, места, рамочки, заготовки, стикеры и надписи стороны). Зона стороны — вся её половина
// листа (корешок и поля зависят от печати альбома, плитке неизвестны). bg без front — фон листа
// целиком (узор и фото на весь лист, как их ставит альбом).
func lookCoverTiles(c *model.CoverTpl, bg, front bool, r float64) ([]model.SpreadTpl, bool) {
	if c == nil {
		return nil, false
	}
	if bg && !front {
		return []model.SpreadTpl{c.SheetBgTpl()}, true
	}
	zone, sides := r/2, []int{0, 1}
	if front {
		zone, sides = r, []int{1}
	}
	out := make([]model.SpreadTpl, 0, len(sides))
	for _, k := range sides {
		tpl := c.SideTpl(k, zone)
		if bg {
			tpl.Places, tpl.Borders, tpl.Stickers, tpl.Texts, tpl.Drafts = nil, nil, nil, nil, nil
		}
		out = append(out, tpl)
	}
	return out, true
}

// lookKeys — ключи библиотеки всех шаблонов плитки.
func lookKeys(tpls []model.SpreadTpl) []string {
	var keys []string
	for _, t := range tpls {
		keys = append(keys, tplKeys(t)...)
	}
	return keys
}

// drawLook — PNG плитки: один шаблон как drawTpl, две стороны обложки — склеены в одну картинку
// шириной q.px (каждая в половину ширины, отношение стороны r/2).
func drawLook(tpls []model.SpreadTpl, q tplQuery, libs map[string]model.PhotoInfo) ([]byte, error) {
	if len(tpls) == 1 {
		return drawTpl(tpls[0], q, libs)
	}
	half := tplQuery{px: q.px / 2, r: max(tplMinR, q.r/2)}
	imgs := make([]image.Image, len(tpls))
	w, h := 0, 0
	for i, t := range tpls {
		hq := half
		if i == len(tpls)-1 {
			hq.px = q.px - half.px*i // ширина в сумме ровно q.px
		}
		img, err := tplImage(t, hq, libs)
		if err != nil {
			return nil, err
		}
		imgs[i] = img
		w += img.Bounds().Dx()
		h = max(h, img.Bounds().Dy())
	}
	sheet := image.NewNRGBA(image.Rect(0, 0, w, h))
	x := 0
	for _, img := range imgs {
		b := img.Bounds()
		draw.Draw(sheet, image.Rect(x, 0, x+b.Dx(), b.Dy()), img, b.Min, draw.Src)
		x += b.Dx()
	}
	return encodeTile(sheet)
}

// serveLook — /media/look/*: false — запрос не к плиткам макетов. Ключ — только из индекса
// библиотеки, параметры — белый список; мусор и паника — 404 (ответ пишется после рисования).
func (a *App) serveLook(w http.ResponseWriter, r *http.Request) bool {
	seg, ok := strings.CutPrefix(r.URL.Path, decor.LookPath)
	if !ok {
		return false
	}
	defer func() {
		if v := recover(); v != nil {
			log.Printf("ПАНИКА в media look: %v\n%s", v, debug.Stack())
			http.NotFound(w, r)
		}
	}()
	s := a.decorLib.Load()
	id := "my:" + seg
	q, okQ := parseLookQuery(r.URL.RawQuery)
	if s == nil || !okQ || !model.ValidLibKey(id) || r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return true
	}
	t, err := s.Look(id)
	if err != nil {
		http.NotFound(w, r)
		return true
	}
	tpls, ok := lookTile(t, q.i, q.bg, q.front, q.r)
	if !ok {
		http.NotFound(w, r)
		return true
	}
	libs := a.libsFor(s, lookKeys(tpls), true)
	found := make([]string, 0, len(libs))
	for k := range libs {
		found = append(found, k)
	}
	slices.Sort(found)
	bg, front := "0", "0"
	if q.bg {
		bg = "1"
	}
	if q.front {
		front = "1"
	}
	// Ключ кэша — и по найденным своим: файл набора появился — плитка рисуется заново.
	key := decor.LookPath + id + "|" + strconv.Itoa(q.i) + "|" + strconv.Itoa(q.px) + "|" +
		strconv.FormatFloat(q.r, 'g', -1, 64) + "|" + bg + "|" + front + "|" + strings.Join(found, ",")
	data, err := s.CachedPNG(r.Context(), key, func() ([]byte, error) { return drawLook(tpls, q.tplQuery, libs) })
	if err != nil {
		http.NotFound(w, r)
		return true
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache") // как у /media/tpl: свой фон может появиться позже
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	return true
}
