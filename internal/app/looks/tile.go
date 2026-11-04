package looks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
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

// Плитка макета альбома: /media/look/<16 hex>?v=1&i=&px=&r=&bg=0|1[&f=0|1][&ph=N] (131 design §8), по
// образцу /media/tpl: i — 0 обложка, 1…MaxLookSteps шаг; px — ширина в точках; r — ширина/высота
// картинки (места по LookBoxes для r); bg=1 — только фон шага; f=1 — только лицевая сторона обложки
// (плитка карточки; r — отношение самой стороны), только при i = 0. ph=N (1…2⁵³−1, 163) — места
// заняты свободными фото ленты, ответ JPEG, px до lookPhotoPx; число — только сброс кэша WebView,
// есть ли фото, решает Go; 0 и без ph — серые места, PNG. Ключ — только из индекса библиотеки.
const lookMaxStep = model.MaxLookSteps

// Пределы запроса — те же, что у /media/tpl (tplMinPx… в корне App берёт их отсюда).
const (
	MinPx = 64
	MaxPx = 1024
	MinR  = 0.2
	MaxR  = 5
)

const (
	lookPhotoPx  = 2048 // кадр с фото: большая сторона не больше (превью 2048 — предел копий кэша)
	lookPhotoBig = 1024 // кадр шире — места берут превью 2048, уже — миниатюры (163-22)
	lookJPEGQ    = 85
	lookMaxPhLen = 16 // цифр в ph: 2⁵³−1 — 16 знаков
)

// lookQuery — разбор запроса плитки: v, i, px, r, bg, f, ph; каждый один раз, i, px, r обязательны.
type lookQuery struct {
	px    int     // ширина в точках
	r     float64 // ширина/высота
	i     int
	bg    bool
	front bool   // f=1: только лицевая сторона обложки
	ph    uint64 // 0 — без фото
	haveI bool   // i был в запросе
}

// parseFlag — "0" или "1"; остальное — не флаг.
func parseFlag(s string) (v, ok bool) {
	return s == "1", s == "0" || s == "1"
}

// set — один параметр запроса: false — имя неизвестно или значение не по белому списку.
func (q *lookQuery) set(k, v string) bool {
	var ok bool
	switch k {
	case "v":
		_, err := strconv.ParseUint(v, 10, 16)
		return err == nil
	case "i":
		n, err := strconv.Atoi(v)
		q.i, q.haveI = n, err == nil
		return err == nil && n >= 0 && n <= lookMaxStep
	case "px":
		n, err := strconv.Atoi(v)
		q.px = n
		return err == nil && n >= MinPx && n <= lookPhotoPx
	case "r":
		f, err := strconv.ParseFloat(v, 64)
		q.r = f
		return err == nil && f >= MinR && f <= MaxR
	case "bg":
		q.bg, ok = parseFlag(v)
	case "f":
		q.front, ok = parseFlag(v)
	case "ph":
		n, err := strconv.ParseUint(v, 10, 53)
		q.ph = n
		return err == nil && len(v) <= lookMaxPhLen
	}
	return ok
}

func parseLookQuery(raw string) (lookQuery, bool) {
	vals, err := url.ParseQuery(raw)
	if err != nil {
		return lookQuery{}, false
	}
	var q lookQuery
	for k, v := range vals {
		if len(v) != 1 || !q.set(k, v[0]) {
			return lookQuery{}, false
		}
	}
	// Шире MaxPx — только кадр с фото.
	return q, q.haveI && q.px > 0 && q.r > 0 && (!q.front || q.i == 0) && (q.px <= MaxPx || q.ph > 0)
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
		tpl = model.LookTpl(tpl, r)
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

// keys — ключи библиотеки всех шаблонов плитки.
func (l *Service) keys(tpls []model.SpreadTpl) []string {
	var keys []string
	for _, t := range tpls {
		keys = append(keys, l.h.Keys(t)...)
	}
	return keys
}

// draw — картинка плитки: один шаблон как drawTpl, две стороны обложки — склеены в одну картинку
// шириной px (каждая в половину ширины, отношение стороны r/2). photos — фото на места каждого
// шаблона (nil — серые места); jpg — JPEG вместо PNG (кадр с фото).
func (l *Service) draw(tpls []model.SpreadTpl, px int, r float64, libs map[string]model.PhotoInfo, photos [][]model.PhotoInfo, jpg bool) ([]byte, error) {
	with := func(k int) []model.PhotoInfo {
		if k < len(photos) {
			return photos[k]
		}
		return nil
	}
	if len(tpls) == 1 {
		img, err := l.h.Image(tpls[0], px, r, libs, with(0))
		if err != nil {
			return nil, err
		}
		return l.encode(img, jpg)
	}
	halfPx, halfR := px/2, max(MinR, r/2)
	imgs := make([]image.Image, len(tpls))
	w, h := 0, 0
	for i, t := range tpls {
		hpx := halfPx
		if i == len(tpls)-1 {
			hpx = px - halfPx*i // ширина в сумме ровно px
		}
		img, err := l.h.Image(t, hpx, halfR, libs, with(i))
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
	return l.encode(sheet, jpg)
}

// encode — PNG плитки или JPEG кадра с фото (лист непрозрачный).
func (l *Service) encode(img image.Image, jpg bool) ([]byte, error) {
	if !jpg {
		return l.h.PNG(img)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: lookJPEGQ}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// lookPhotosKey — часть ключа кэша по фото на местах: ID и путь каждой копии (путь копии строится
// от пути, размера и времени оригинала — замена файла сама меняет ключ). Короткий отпечаток, а не
// список: на лист бывают десятки мест.
func lookPhotosKey(photos [][]model.PhotoInfo) string {
	h := sha256.New()
	for _, side := range photos {
		for _, p := range side {
			h.Write([]byte(p.ID + "\x00" + p.Path + "\x00"))
		}
		h.Write([]byte{1})
	}
	return hex.EncodeToString(h.Sum(nil)[:12])
}

// errLookRetry — рисование с фото не вышло, отдан лист без фото: в кэш он не кладётся.
var errLookRetry = errors.New("лист макета нарисован без фото")

// lookCacheKey — ключ кэша ответа: запрос, найденные свои файлы набора (появился файл — плитка
// рисуется заново) и фото на местах phKey.
func lookCacheKey(id string, q lookQuery, libs map[string]model.PhotoInfo, phKey string) string {
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
	return decor.LookPath + id + "|" + strconv.Itoa(q.i) + "|" + strconv.Itoa(q.px) + "|" +
		strconv.FormatFloat(q.r, 'g', -1, 64) + "|" + bg + "|" + front + "|" + strings.Join(found, ",") + phKey
}

// Serve — /media/look/*: false — запрос не к плиткам макетов. Ключ — только из индекса
// библиотеки, параметры — белый список; мусор и паника — 404 (ответ пишется после рисования).
func (l *Service) Serve(w http.ResponseWriter, r *http.Request) bool {
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
	s := l.loaded()
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
	jpg := q.ph > 0
	if jpg { // кадр с фото: большая сторона ≤ lookPhotoPx (px уже не больше, высота — px / r)
		q.px = min(q.px, max(MinPx, int(lookPhotoPx*q.r)))
	}
	var photos [][]model.PhotoInfo
	phKey := ""
	if jpg {
		if photos, err = l.tileFiles(r.Context(), t, tpls, q.i, l.candidates(), q.px > lookPhotoBig); err != nil {
			l.h.Cancelled() // запрос снят: ответа и записи в кэш нет
			return true
		}
		phKey = "|ph|" + lookPhotosKey(photos)
	}
	libs := l.h.LibsFor(s, l.keys(tpls), true)
	key := lookCacheKey(id, q, libs, phKey)
	var plain []byte // лист без фото после сбоя рисования с ними
	data, err := s.CachedPNG(r.Context(), key, func() ([]byte, error) {
		d, err := l.draw(tpls, q.px, q.r, libs, photos, jpg)
		if err == nil || !hasPhotos(photos) {
			return d, err
		}
		// Копию могла убрать чистка кэша между выдачей и рисованием: один повтор без фото (163-19).
		if plain, err = l.draw(tpls, q.px, q.r, libs, nil, jpg); err != nil {
			return nil, err
		}
		return nil, errLookRetry
	})
	if errors.Is(err, errLookRetry) {
		data, err = plain, nil
	}
	if err != nil {
		http.NotFound(w, r)
		return true
	}
	if jpg {
		w.Header().Set("Content-Type", "image/jpeg")
	} else {
		w.Header().Set("Content-Type", "image/png")
	}
	w.Header().Set("Cache-Control", "no-cache") // как у /media/tpl: свой фон может появиться позже
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	return true
}

// hasPhotos — на места хотя бы одной плитки поставлено фото.
func hasPhotos(photos [][]model.PhotoInfo) bool {
	for _, side := range photos {
		if len(side) > 0 {
			return true
		}
	}
	return false
}
