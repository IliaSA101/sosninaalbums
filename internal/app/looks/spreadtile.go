package looks

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
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/disintegration/imaging"
)

// TplLibs — свои фоны и стикеры шаблона, чьи файлы есть на этом ПК (ключ → сведения о файле).
// thumbs — для плитки: путь — миниатюра кэша, а не исходник (рисовать 200 МБ фона ради плитки
// незачем); без кэша миниатюр своих на плитке нет.
func (l *Service) TplLibs(s *decor.Store, t model.SpreadTpl, thumbs bool) map[string]model.PhotoInfo {
	return l.libsFor(s, tplKeys(t), thumbs)
}

// libsFor — сведения о файлах библиотеки по ключам keys; ключа или файла нет — пропуск.
func (l *Service) libsFor(s *decor.Store, keys []string, thumbs bool) map[string]model.PhotoInfo {
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
			if l.h.Cache() == nil {
				continue
			}
			f, err := l.h.Cache().File(info, false)
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

// Плитка шаблона: /media/tpl/<id>?v=&px=&r= — px — ширина в точках, r — ширина/высота области.
const (
	tplMinPx  = MinPx // пределы общие с /media/look: один источник — MinPx…MaxR в tile.go
	tplMaxPx  = MaxPx
	tplMinR   = MinR
	tplMaxR   = MaxR
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

// ServeTpl — /media/tpl/*: false — запрос не к плиткам шаблонов. ID — только встроенный или
// из индекса библиотеки; мусор и паника — 404.
func (l *Service) ServeTpl(w http.ResponseWriter, r *http.Request) bool {
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
	s, _ := l.h.Store()
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
	libs := l.TplLibs(s, t, true)
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
	return tplImageWith(t, q, libs, nil)
}

// tplImageWith — tplImage с фото на местах (макеты, 163): photos садятся в пустые места листа.
func tplImageWith(t model.SpreadTpl, q tplQuery, libs map[string]model.PhotoInfo, photos []model.PhotoInfo) (image.Image, error) {
	wmm, hmm := tplSideMM*q.r, float64(tplSideMM)
	if q.r < 1 {
		wmm, hmm = tplSideMM, tplSideMM/q.r
	}
	p, v, err := album.PreviewTplWith(t, wmm, hmm, libs, photos)
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
