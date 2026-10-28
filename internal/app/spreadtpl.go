package app

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"log"
	"math"
	"net/http"
	"net/url"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/disintegration/imaging"
)

// Готовые развороты (спека 067, design §3, §6): окно «Развороты» — вид spread библиотеки
// (DecorCatalog, RenameDecor, HideDecor — общие 048), поставить, вопрос до правки, сохранить
// свой и плитки /media/tpl. Шаблон берётся только по ID плитки из decor.Store.

// TplResult — итог ApplySpreadTpl: вид, сколько фото ушло в ленту и сколько ключей библиотеки
// шаблона нет на этом ПК (их слои пропущены).
type TplResult struct {
	View    model.View `json:"view"`
	Left    int        `json:"left"`
	Missing int        `json:"missing"`
}

// spreadTpl — шаблон плитки itemID и библиотека.
func (a *App) spreadTpl(itemID string) (*decor.Store, model.SpreadTpl, error) {
	s, err := a.decorStore()
	if err != nil {
		return nil, model.SpreadTpl{}, err
	}
	t, err := s.SpreadTpl(itemID)
	return s, t, err
}

// tplLibs — свои фоны и стикеры шаблона, чьи файлы есть на этом ПК (ключ → сведения о файле).
// thumbs — для плитки: путь — миниатюра кэша, а не исходник (рисовать 200 МБ фона ради плитки
// незачем); без кэша миниатюр своих на плитке нет.
func (a *App) tplLibs(s *decor.Store, t model.SpreadTpl, thumbs bool) map[string]model.PhotoInfo {
	return a.libsFor(s, tplKeys(t), thumbs)
}

// libsFor — сведения о файлах библиотеки по ключам keys; ключа или файла нет — пропуск.
func (a *App) libsFor(s *decor.Store, keys []string, thumbs bool) map[string]model.PhotoInfo {
	libs := map[string]model.PhotoInfo{}
	for _, key := range keys {
		if _, have := libs[key]; have {
			continue
		}
		info, ok := s.LibInfo(key)
		if !ok {
			continue
		}
		path := info.Path
		if thumbs {
			if a.cache == nil {
				continue
			}
			f, err := a.cache.File(info, false)
			if err != nil {
				continue
			}
			path = f
		}
		libs[key] = model.PhotoInfo{Path: path, W: info.W, H: info.H, Size: info.Size}
	}
	return libs
}

// tplKeys — ключи библиотеки, на которые ссылается шаблон (фоны, свои стикеры и рамочки).
func tplKeys(t model.SpreadTpl) []string {
	var keys []string
	if t.Whole != nil && t.Whole.Lib != "" {
		keys = append(keys, t.Whole.Lib)
	}
	for _, pg := range t.Page {
		if pg.Lib != "" {
			keys = append(keys, pg.Lib)
		}
	}
	for _, st := range t.Stickers {
		if st.Lib != "" {
			keys = append(keys, st.Lib)
		}
	}
	for _, b := range t.Borders {
		if b.Lib != "" {
			keys = append(keys, b.Lib)
		}
	}
	return keys
}

// PlanSpreadTpl — вопрос до ApplySpreadTpl (как PlanCopyDecor): сколько фото уйдёт в ленту,
// сколько стикеров и надписей заменится, сколько «Авто» станет сеткой.
func (a *App) PlanSpreadTpl(spreadID, scope, itemID string) (plan model.GridPlan, err error) {
	defer guardErr("PlanSpreadTpl", &err)
	s, t, err := a.spreadTpl(itemID)
	if err != nil {
		return plan, err
	}
	return a.editor().PlanSpreadTpl(spreadID, scope, t, a.tplLibs(s, t, false))
}

// ApplySpreadTpl — щелчок по плитке разворота: места и оформление области scope ("spread" |
// "left" | "right") — один шаг отмены; повтор — холостая правка без шага. Плитка — в недавние.
func (a *App) ApplySpreadTpl(spreadID, scope, itemID string) (r TplResult, err error) {
	defer a.guardEdit("ApplySpreadTpl", &r.View, &err)
	ed := a.editor()
	s, t, err := a.spreadTpl(itemID)
	if err != nil {
		return TplResult{View: ed.View()}, err
	}
	v, plan, missing, err := ed.ApplySpreadTpl(spreadID, scope, t, a.tplLibs(s, t, false))
	if r.View, err = a.changedOrErr(v, err); err != nil {
		return r, err
	}
	r.Left, r.Missing = plan.Left, missing
	prefs.RememberDecor(a.cfgDir, decor.KindSpread, itemID)
	return r, nil
}

// SaveSpreadTpl — «Сохранить разворот…»: снимок области (places — с местами, texts — с
// надписями; фото фона из съёмки не входит, решение 6) в свои шаблоны темы theme под именем
// name. Альбом не меняется.
func (a *App) SaveSpreadTpl(spreadID, scope, theme, name string, places, texts bool) (it DecorItem, err error) {
	defer guardErr("SaveSpreadTpl", &err)
	s, err := a.decorStore()
	if err != nil {
		return it, err
	}
	t, err := a.editor().SpreadTplOf(spreadID, scope, places, texts)
	if err != nil {
		return it, err
	}
	got, err := s.SaveSpreadTpl(theme, name, t)
	if err != nil {
		return it, err
	}
	return DecorItem(got), nil
}

// Плитка шаблона: /media/tpl/<id>?v=&px=&r= — px — ширина в точках, r — ширина/высота области.
const (
	tplMinPx  = 64
	tplMaxPx  = 1024
	tplMinR   = 0.2
	tplMaxR   = 5
	tplSideMM = 200 // меньшая сторона области превью: страница 50…1000 мм при любом r
)

// tplQuery — разбор запроса плитки: только v, px, r, каждый один раз; px и r обязательны.
type tplQuery struct {
	px int
	r  float64
}

func parseTplQuery(raw string) (tplQuery, bool) {
	vals, err := url.ParseQuery(raw)
	if err != nil {
		return tplQuery{}, false
	}
	var q tplQuery
	for k, v := range vals {
		if len(v) != 1 {
			return tplQuery{}, false
		}
		switch k {
		case "v":
			if _, err := strconv.ParseUint(v[0], 10, 16); err != nil {
				return tplQuery{}, false
			}
		case "px":
			n, err := strconv.Atoi(v[0])
			if err != nil || n < tplMinPx || n > tplMaxPx {
				return tplQuery{}, false
			}
			q.px = n
		case "r":
			f, err := strconv.ParseFloat(v[0], 64)
			if err != nil || !(f >= tplMinR && f <= tplMaxR) {
				return tplQuery{}, false
			}
			q.r = f
		default:
			return tplQuery{}, false
		}
	}
	return q, q.px > 0 && q.r > 0
}

// serveTpl — /media/tpl/*: false — запрос не к плиткам шаблонов. ID — только встроенный или
// из индекса библиотеки; мусор и паника — 404.
func (a *App) serveTpl(w http.ResponseWriter, r *http.Request) bool {
	seg, ok := strings.CutPrefix(r.URL.Path, decor.TplPath)
	if !ok {
		return false
	}
	// Ответ пишется только после рисования: паника до него — чистый 404.
	defer func() {
		if v := recover(); v != nil {
			log.Printf("ПАНИКА в media tpl: %v\n%s", v, debug.Stack())
			http.NotFound(w, r)
		}
	}()
	s := a.decorLib.Load()
	id, okID := decor.TplID(seg)
	q, okQ := parseTplQuery(r.URL.RawQuery)
	if s == nil || !okID || !okQ || r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return true
	}
	t, err := s.SpreadTpl(id)
	if err != nil {
		http.NotFound(w, r)
		return true
	}
	libs := a.tplLibs(s, t, true)
	found := make([]string, 0, len(libs))
	for k := range libs {
		found = append(found, k)
	}
	slices.Sort(found)
	// Ключ кэша — и по найденным своим: файл набора появился — плитка рисуется заново.
	key := decor.TplPath + id + "?px=" + strconv.Itoa(q.px) + "&r=" + strconv.FormatFloat(q.r, 'g', -1, 64) + "|" + strings.Join(found, ",")
	data, err := s.CachedPNG(r.Context(), key, func() ([]byte, error) { return drawTpl(t, q, libs) })
	if err != nil {
		http.NotFound(w, r)
		return true
	}
	w.Header().Set("Content-Type", "image/png")
	// no-cache: URL плитки постоянный, а свой фон или стикер шаблона может появиться позже
	// (набор, загрузка) — плитка рисуется заново; повтор из серверного кэша дешёвый.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	return true
}

// errTplDraw — шаблон не встал на временный лист плитки.
var errTplDraw = errors.New("плитка шаблона не рисуется")

// drawTpl — PNG плитки: шаблон ставится на временный лист тем же путём, что в альбом
// (album.PreviewTpl), и рисуется как экспорт (export.TplScene); у страницы — левая половина.
func drawTpl(t model.SpreadTpl, q tplQuery, libs map[string]model.PhotoInfo) ([]byte, error) {
	img, err := tplImage(t, q, libs)
	if err != nil {
		return nil, err
	}
	return encodeTile(img)
}

// encodeTile — PNG плитки (быстрое сжатие: плитка живёт в кэше недолго).
func encodeTile(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// tplImage — картинка плитки шириной q.px (drawTpl без кодирования: плитка обложки склеивает две).
func tplImage(t model.SpreadTpl, q tplQuery, libs map[string]model.PhotoInfo) (image.Image, error) {
	wmm, hmm := tplSideMM*q.r, float64(tplSideMM)
	if q.r < 1 {
		wmm, hmm = tplSideMM, tplSideMM/q.r
	}
	p, v, err := album.PreviewTpl(t, wmm, hmm, libs)
	if err != nil {
		return nil, err
	}
	sc, ok := export.TplScene(p, v)
	if !ok {
		return nil, errTplDraw
	}
	sv := v.Spreads[0]
	area := sv.W // ширина области плитки: разворот целиком или левая страница
	if t.Pages == 1 {
		area = sv.W / 2
	}
	dpi := int(math.Ceil(float64(q.px) * 25.4 / area))
	img, err := render.Render(sc, render.Options{PageW: sv.W / 2, PageH: sv.H, DPI: dpi})
	if err != nil {
		return nil, err
	}
	var out image.Image = img
	if t.Pages == 1 {
		out = img.SubImage(image.Rect(0, 0, img.Bounds().Dx()/2, img.Bounds().Dy()))
	}
	// Точек на дюйм — целое с запасом вверх: плитка шире px, ужимается ровно до px.
	if out.Bounds().Dx() != q.px {
		out = imaging.Resize(out, q.px, 0, imaging.Linear)
	}
	return out, nil
}
