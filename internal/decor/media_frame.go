package decor

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // заголовок JPEG для DecodeConfig (рамочка — PNG, но подмена расширением не должна падать иначе)
	_ "image/png"
	"math"
	"net/http"
	"os"
	"sync"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
)

// ServeFramePic — накладка и маска рамочки-картинки по запросу `w,h,s,c,o,m,px,v` (079 design
// §5, 079-22): 9-срез картинки path под место w×h мм. Маршрут /media/f/<id> и путь — дело
// вызывающего (Editor.BorderPath, CheckLocalPath, пропавшее); сюда приходит уже проверенный
// путь, и пакет его не выбирает. Разбор и зажим — gen.ParseFramePic: мусор, лишнее и px вне
// 16…4096 — 404; паника — guard (404, слот освобождён). Источник — копия ≤ 2048 px по длинной
// стороне в двух последних слотах, готовые PNG — в LRU по пути, размеру и времени файла.

const (
	framePicSide   = 2048       // длинная сторона копии-источника для экрана
	framePicArea   = 20_000_000 // px картинки после масштаба k: больше — уменьшается (память слота)
	framePicMaxSrc = 64_000_000 // исходник больше — не рисуется на экране (печать — тот же предел)
	framePicSlots  = 2          // копий-источников в памяти (design §4: декод ≤ 2)
	framePicCache  = mediaCacheMax / 2
)

// picStamp — файл на момент запроса: заменённый под тем же путём ответ не из кэша.
type picStamp struct {
	path string
	size int64
	mod  int64
}

// picSource — оригинал уменьшен до framePicSide; w, h — размер оригинала (от него k).
type picSource struct {
	stamp picStamp
	img   *image.NRGBA
	w, h  int
}

const picBadMax = 256 // запомненных отказов; больше — забываются все (файлы редко меняются)

var (
	picMu    sync.Mutex
	picSrcs  []*picSource // старые первыми
	picBad   = map[picStamp]struct{}{}
	picMedia = newMediaCache(framePicCache)

	// Заглушки тестов: чтение заголовка и полный декод.
	readPicConfig = func(path string) (image.Config, error) {
		f, err := imgfile.OpenRegular(path)
		if err != nil {
			return image.Config{}, err
		}
		defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
		cfg, _, err := image.DecodeConfig(f)
		return cfg, err
	}
	openOriented = imgfile.OpenOriented
)

// picRejected — отказ по этому файлу уже был: повторный запрос не читает файл.
func picRejected(st picStamp) bool {
	picMu.Lock()
	defer picMu.Unlock()
	_, bad := picBad[st]
	return bad
}

func picReject(st picStamp) {
	picMu.Lock()
	defer picMu.Unlock()
	if len(picBad) >= picBadMax {
		clear(picBad)
	}
	picBad[st] = struct{}{}
}

// picCheckHeader — размеры по заголовку до декода и бюджета памяти (ориентация EXIF на предел не
// влияет: он симметричен по сторонам). Раздутый заголовок или мусор — отказ, запоминается по stamp.
// Сторона больше MaxFrameSide не отказ (079-40): копия-источник всё равно ≤ framePicSide, а такой
// файл приходит от картинки и в «Фонах» без копии в «Рамочках» и из собранного альбома.
func picCheckHeader(st picStamp) error {
	picMu.Lock()
	for _, p := range picSrcs {
		if p.stamp == st { // уже прошла проверку и лежит в слоте
			picMu.Unlock()
			return nil
		}
	}
	picMu.Unlock()
	cfg, err := readPicConfig(st.path)
	if err != nil {
		if _, perr := os.Stat(st.path); perr == nil { // файл есть, но не картинка — отказ постоянный
			picReject(st)
		}
		return err
	}
	if cfg.Width < 1 || cfg.Height < 1 || int64(cfg.Width)*int64(cfg.Height) > framePicMaxSrc {
		picReject(st)
		return errors.New("картинка слишком велика")
	}
	return nil
}

// loadPicSource — копия-источник: из слотов или декодом файла (imgfile: пределы и бюджет памяти).
func loadPicSource(st picStamp) (*picSource, error) {
	picMu.Lock()
	for _, p := range picSrcs {
		if p.stamp == st {
			picMu.Unlock()
			return p, nil
		}
	}
	picMu.Unlock()
	img, release, err := openOriented(st.path)
	if err != nil {
		return nil, err
	}
	defer release()
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if int64(w)*int64(h) > framePicMaxSrc {
		return nil, errors.New("картинка слишком велика")
	}
	src := &picSource{stamp: st, w: w, h: h}
	switch long := max(w, h); {
	case long > framePicSide:
		k := float64(framePicSide) / float64(long)
		src.img = imaging.Resize(img, max(1, int(math.Round(float64(w)*k))), max(1, int(math.Round(float64(h)*k))), imaging.Lanczos)
	default:
		src.img = imaging.Clone(img)
	}
	picMu.Lock()
	picSrcs = append(picSrcs, src)
	if len(picSrcs) > framePicSlots {
		picSrcs = picSrcs[len(picSrcs)-framePicSlots:]
	}
	picMu.Unlock()
	return src, nil
}

// ServeFramePic — см. описание выше. GET и HEAD; остальное — 404.
func ServeFramePic(w http.ResponseWriter, r *http.Request, path string) {
	defer guard(w, r)
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	req, ok := gen.ParseFramePic(r.URL.RawQuery)
	if !ok {
		http.NotFound(w, r)
		return
	}
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	st := picStamp{path: path, size: fi.Size(), mod: fi.ModTime().UnixNano()}
	if picRejected(st) {
		http.NotFound(w, r)
		return
	}
	key := fmt.Sprintf("%s|%d|%d|%s", st.path, st.size, st.mod, r.URL.RawQuery)
	data, hit := picMedia.get(key)
	if !hit {
		if err := picCheckHeader(st); err != nil { // до слота и бюджета декода
			http.NotFound(w, r)
			return
		}
		select {
		case genSem <- struct{}{}:
		case <-r.Context().Done(): // окно ушло со страницы, пока ждали слот
			return
		}
		if data, err = drawFramePic(st, req); err != nil {
			http.NotFound(w, r)
			return
		}
		picMedia.put(key, data)
	}
	w.Header().Set("Content-Type", "image/png")
	// Файл под путём может смениться (перепривязка, замена): ответ проверяется по времени файла.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "", fi.ModTime(), bytes.NewReader(data))
}

// drawFramePic рисует ответ в занятом слоте genSem; слот освобождается и при панике (её ловит
// guard).
func drawFramePic(st picStamp, req gen.FramePicRequest) ([]byte, error) {
	defer func() { <-genSem }()
	src, err := loadPicSource(st)
	if err != nil {
		return nil, err
	}
	var img image.Image
	if req.Mask {
		img = picMask(src, req)
	} else {
		img = picOverlay(src, req)
	}
	return encodePNG(img)
}

// picScale — px на мм места и мм на px оригинала: k = min(W/iw, H/ih) × размер (design §2).
func picScale(src *picSource, req gen.FramePicRequest) (pxmm, k float64) {
	pw, _ := req.Size()
	k = min(req.W/float64(src.w), req.H/float64(src.h)) * req.Scale
	return float64(pw) / req.W, k
}

// picOverlay — картинка в масштабе k (Lanczos), затем 9-срез на всё место: углы 1:1, стороны и
// середина тянутся (gen.Nine). Копия больше framePicArea уменьшается, а толщины рамки на месте —
// по масштабу k (gen.NineTo): иначе между рамкой и окном маски (k·срез) осталось бы кольцо.
func picOverlay(src *picSource, req gen.FramePicRequest) *image.RGBA {
	pw, ph := req.Size()
	pxmm, k := picScale(src, req)
	tw, th := float64(src.w)*k*pxmm, float64(src.h)*k*pxmm
	c := req.Cut
	var dsl [4]int
	capped := tw*th > framePicArea // размер 2 при месте в 4096 px
	if capped {
		dsl = gen.SlicePx(c, tw, th)
		f := math.Sqrt(framePicArea / (tw * th))
		tw, th = tw*f, th*f
	}
	w, h := max(1, int(math.Round(tw))), max(1, int(math.Round(th)))
	var scaled image.Image = src.img
	if b := src.img.Bounds(); b.Dx() != w || b.Dy() != h {
		scaled = imaging.Resize(src.img, w, h, imaging.Lanczos)
	}
	sl := gen.SlicePx(c, float64(w), float64(h))
	if !capped {
		dsl = sl
	}
	dst := image.NewRGBA(image.Rect(0, 0, pw, ph))
	gen.NineTo(dst, dst.Bounds(), scaled, sl, dsl)
	return dst
}

// picMask — маска окна: прозрачная область картинки (центральная часть среза) растянута на окно
// места — место минус отступы k·срез, как их считает модель (BorderIns, предел 0,7 стороны).
func picMask(src *picSource, req gen.FramePicRequest) *image.NRGBA {
	pw, ph := req.Size()
	pxmm, k := picScale(src, req)
	iw, ih, c := float64(src.w), float64(src.h), req.Cut
	ins := gen.ClampInsets([4]float64{k * c[0] * iw, k * c[1] * ih, k * c[2] * iw, k * c[3] * ih}, req.W, req.H)
	win := image.Rect(int(math.Round(ins[0]*pxmm)), int(math.Round(ins[1]*pxmm)),
		pw-int(math.Round(ins[2]*pxmm)), ph-int(math.Round(ins[3]*pxmm)))
	return gen.StretchWindow(gen.WindowMask(src.img, c), c, image.Pt(pw, ph), win)
}
