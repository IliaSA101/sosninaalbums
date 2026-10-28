package colorpick

// Пипетка окна выбора цвета (спека 109 §4): цвет точки листа берётся из пробного кадра, который
// рисует тот же render, что и экспорт, но по превью фото и в ≈ 1600 px по большей стороне. Кадры
// живут в небольшом кэше по (редактор, лист, rev): щелчки подряд по одному листу рисуют его один раз.

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"slices"
	"sync"
	"sync/atomic"
	"weak"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

const (
	pickSidePx = 1600 // большая сторона пробного кадра, px
	// pickMinDPI — предел снизу, чтобы длинный лист не получил кадр в 0 px. Единица, а не 10: при 10
	// лист длиннее ~4 м раздувал бы кадр сверх pickSidePx, а предел должен побеждать.
	pickMinDPI   = 1
	pickKeep     = 3 // кадров в кэше: 3 × 1600 × 800 × 4 Б ≈ 15 МБ у разворота 2:1, у квадратного листа вдвое больше
	pickParallel = 2 // рендеров кадров одновременно: каждый — десятки МБ и все ядра
)

var ErrOutside = errors.New("точка вне листа")

// pickKey — чей кадр: редактор слабой ссылкой (после «Новый» и «Открыть» прежний альбом со стеком
// отмены кэшем не держится), лист и rev вида.
type pickKey struct {
	ed  weak.Pointer[album.Editor]
	id  string
	rev int
}

// pickCall — идущий рендер ключа; ждущие читают результат после done.
type pickCall struct {
	done chan struct{}
	img  *image.RGBA
	err  error
}

// pickCache — до pickKeep кадров, самый старый вытесняется; inflight сводит параллельные запросы
// одного ключа в один рендер.
type pickCache struct {
	mu       sync.Mutex
	keys     []pickKey // от старых к свежим
	imgs     map[pickKey]*image.RGBA
	inflight map[pickKey]*pickCall
	semOnce  sync.Once
	sem      chan struct{} // слоты рендера: не больше pickParallel сразу
	renders  atomic.Int64  // сколько кадров нарисовано (тесты)
	skipped  atomic.Int64  // сколько картинок убрано из сцены (журнал)
}

// get отдаёт кадр ключа: из кэша, у идущего рендера или своим build. Отмена ctx снимает только
// ожидание чужого рендера.
func (c *pickCache) get(ctx context.Context, k pickKey, build func() (*image.RGBA, error)) (*image.RGBA, error) {
	c.mu.Lock()
	if img, ok := c.imgs[k]; ok {
		c.mu.Unlock()
		return img, nil
	}
	if call, ok := c.inflight[k]; ok {
		c.mu.Unlock()
		select {
		case <-call.done:
			return call.img, call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	call := &pickCall{done: make(chan struct{}), err: errors.New("кадр не построен")}
	if c.inflight == nil {
		c.inflight = map[pickKey]*pickCall{}
	}
	c.inflight[k] = call
	c.mu.Unlock()

	c.semOnce.Do(func() { c.sem = make(chan struct{}, pickParallel) })
	defer func() { // и при панике: ждущие не зависают
		c.mu.Lock()
		delete(c.inflight, k)
		if call.err == nil {
			c.put(k, call.img)
		}
		c.mu.Unlock()
		close(call.done)
	}()
	// Щелчки по разным листам или правки подряд не должны запускать рендеры лавиной: ждём слот,
	// отмена ctx (закрытие программы) снимает ожидание.
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		call.err = ctx.Err()
		return nil, call.err
	}
	call.img, call.err = build()
	return call.img, call.err
}

// pickDPI — разрешение кадра: большая сторона листа укладывается в pickSidePx (а не ширина: у
// высокого листа кадр по ширине раздулся бы по высоте), но не меньше pickMinDPI.
func pickDPI(w, h float64) int {
	return max(int(pickSidePx*25.4/max(w, h)), pickMinDPI)
}

// put кладёт кадр и вытесняет лишние; кадры ушедших редакторов уходят первыми. Под c.mu.
func (c *pickCache) put(k pickKey, img *image.RGBA) {
	if c.imgs == nil {
		c.imgs = map[pickKey]*image.RGBA{}
	}
	c.keys = slices.DeleteFunc(c.keys, func(o pickKey) bool {
		dead := o == k || o.ed.Value() == nil
		if dead {
			delete(c.imgs, o)
		}
		return dead
	})
	c.keys = append(c.keys, k)
	c.imgs[k] = img
	for len(c.keys) > pickKeep {
		delete(c.imgs, c.keys[0])
		c.keys = c.keys[1:]
	}
}

// PickAt — цвет точки (x, y) мм листа spreadID на готовом развороте: "#rrggbb", среднее по
// 3×3 px пробного кадра (фон, узоры, фото, стикеры и тексты — как в экспорте). Альбом не меняется.
// Ошибки: «лист не найден», «точка вне листа».
func (pk *Picker) PickAt(spreadID string, x, y float64) (string, error) {
	if math.IsNaN(x) || math.IsInf(x, 0) || math.IsNaN(y) || math.IsInf(y, 0) {
		return "", ErrOutside
	}
	ed := pk.h.Editor()
	ctx := pk.h.Ctx()
	p, v := ed.Snapshot()
	sv, ok := viewSheet(v, spreadID)
	if !ok {
		return "", ErrNoSheet
	}
	if x < 0 || y < 0 || x >= sv.W || y >= sv.H {
		return "", ErrOutside
	}
	k := pickKey{ed: weak.Make(ed), id: spreadID, rev: v.Rev}
	img, err := pk.pick.get(ctx, k, func() (*image.RGBA, error) {
		scene := export.SheetScene(p, sv)
		if v.Cover != nil && v.Cover.Sheet.ID == spreadID {
			if cs, ok := export.CoverScene(p, v); ok {
				scene = cs // с плашкой корешка и заготовками (131)
			}
		}
		pk.pickPreviews(ctx, p.Photos, &scene)
		dpi := pickDPI(sv.W, sv.H)
		pk.pick.renders.Add(1)
		return render.RenderCtx(ctx, scene, render.Options{PageW: sv.W / 2, PageH: sv.H, DPI: dpi})
	})
	if err != nil {
		return "", err
	}
	return avgHex(img, x/sv.W, y/sv.H), nil
}

// avgHex — среднее по 3×3 px вокруг точки (доли fx, fy листа); у края окно обрезается.
func avgHex(img *image.RGBA, fx, fy float64) string {
	b := img.Bounds()
	cx := min(max(b.Min.X+int(fx*float64(b.Dx())), b.Min.X), b.Max.X-1)
	cy := min(max(b.Min.Y+int(fy*float64(b.Dy())), b.Min.Y), b.Max.Y-1)
	var r, g, bl, n int
	for yy := max(cy-1, b.Min.Y); yy <= min(cy+1, b.Max.Y-1); yy++ {
		for xx := max(cx-1, b.Min.X); xx <= min(cx+1, b.Max.X-1); xx++ {
			c := img.RGBAAt(xx, yy)
			r, g, bl, n = r+int(c.R), g+int(c.G), bl+int(c.B), n+1
		}
	}
	return fmt.Sprintf("#%02x%02x%02x", (r+n/2)/n, (g+n/2)/n, (bl+n/2)/n)
}
