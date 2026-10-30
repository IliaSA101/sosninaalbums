package design_test

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// Смена формата страницы и стикеры (0.27-129-107): стоявший целиком на листе остаётся на нём,
// прижатый к краю — прижатым; выходивший раньше — как был.

var rsFormats = [][2]float64{{400, 300}, {300, 200}, {200, 300}, {200, 200}}

var rsN int

// rsSticker — своё фото w×h px стикером листа sid (пропорция w/h); у каждого стикера свой ключ.
func rsSticker(t *testing.T, e *album.Editor, sid string, w, h int) string {
	t.Helper()
	rsN++
	rsKey := fmt.Sprintf("my:%016x", rsN)
	src := model.StickerSrc{Photo: model.PhotoInfo{ID: model.LibPhotoID(rsKey), Path: edtest.Abs("rs.png"), Lib: rsKey, W: w, H: h}}
	_, id, err := e.AddSticker(sid, 0, src)
	if err != nil || id == "" {
		t.Fatalf("AddSticker: %q, %v", id, err)
	}
	return id
}

func rsBox(t *testing.T, e *album.Editor, id string) *model.Sticker {
	t.Helper()
	s, i := edtest.P(e).FindSticker(id)
	if s == nil {
		t.Fatalf("стикера %s нет", id)
	}
	return s.Stickers[i].Clone()
}

func rsSetPage(e *album.Editor, w, h float64) {
	st := e.View().Settings
	st.PageW, st.PageH = w, h
	e.SetSettings(st)
}

// rsGaps — зазоры описанного прямоугольника до краёв листа 2·pw × ph: слева, справа, сверху, снизу.
func rsGaps(st *model.Sticker, pw, ph float64) (l, r, tp, b float64) {
	a := st.Rect().AABB()
	return a.X, 2*pw - a.X - a.W, a.Y, ph - a.Y - a.H
}

// rsPin — к каким краям листа стикер прижат (зазор ≤ 0,05 мм).
type rsPin struct{ l, r, tp, b bool }

// rsCorner — стикер на листе 600×300: ax, ay — 0 (к левому/верхнему краю), 1 (к правому/нижнему),
// 2 (отступ 20 мм); bw — ширина рамки, высота — по пропорции w:h px.
type rsCorner struct {
	w, h   int
	ax, ay int
	bw     float64
}

// rsPlace ставит стикер c на лист sid под углом rot так, чтобы описанный прямоугольник касался краёв.
func rsPlace(t *testing.T, e *album.Editor, sid string, c rsCorner, rot float64) (string, rsPin) {
	t.Helper()
	id := rsSticker(t, e, sid, c.w, c.h)
	bh := c.bw * float64(c.h) / float64(c.w)
	e.SetStickerBox(sid, id, 0, 0, c.bw, bh, rot, "")
	a := rsBox(t, e, id).Rect().AABB()
	at := func(mode int, a0, size, side float64) float64 {
		switch mode {
		case 0:
			return -a0
		case 1:
			return side - size - a0
		}
		return 20 - a0
	}
	e.SetStickerBox(sid, id, at(c.ax, a.X, a.W, 600), at(c.ay, a.Y, a.H, 300), c.bw, bh, rot, "")
	l, r, tp, b := rsGaps(rsBox(t, e, id), 300, 300)
	if min(l, r, tp, b) < -0.02 {
		t.Fatalf("исходный стикер не внутри: %v %v %v %v", l, r, tp, b)
	}
	return id, rsPin{l <= 0.05, r <= 0.05, tp <= 0.05, b <= 0.05}
}

// rsCheck — после смены формата на f: внутри листа, прижатые края — прижаты.
func rsCheck(t *testing.T, e *album.Editor, id string, was rsPin, f [2]float64, rot float64) {
	t.Helper()
	l, r, tp, b := rsGaps(rsBox(t, e, id), f[0], f[1])
	if min(l, r, tp, b) < -0.02 {
		t.Errorf("rot %v, %v: стикер %s за листом: зазоры %.2f %.2f %.2f %.2f", rot, f, id, l, r, tp, b)
	}
	for _, c := range []struct {
		name   string
		pinned bool
		gap    float64
	}{{"слева", was.l, l}, {"справа", was.r, r}, {"сверху", was.tp, tp}, {"снизу", was.b, b}} {
		if c.pinned && c.gap > 0.05 {
			t.Errorf("rot %v, %v: стикер %s был прижат %s, зазор %.2f", rot, f, id, c.name, c.gap)
		}
	}
}

func TestStickerRescaleKeepsInsideAndPinned(t *testing.T) {
	// Углы (квадратный, широкий, высокий) и один свободный с отступом.
	corners := []rsCorner{{100, 100, 0, 0, 60}, {100, 100, 1, 0, 60}, {200, 100, 0, 1, 90}, {100, 200, 1, 1, 60}, {100, 100, 2, 2, 40}}
	for _, rot := range []float64{0, 30} {
		for _, f := range rsFormats {
			e, s1, _ := edtest.NewTwo(t) // лист 600×300
			ids := map[string]rsPin{}
			for _, c := range corners {
				id, pin := rsPlace(t, e, s1, c, rot)
				ids[id] = pin
			}
			rsSetPage(e, f[0], f[1])
			for id, was := range ids {
				rsCheck(t, e, id, was, f, rot)
			}
			// Круг: Save→Load ничего не двигает; отмена возвращает прежний формат одним шагом.
			p := edtest.SaveLoad(t, e)
			for id := range ids {
				if s, i := p.FindSticker(id); s == nil || *s.Stickers[i] != *rsBox(t, e, id) {
					t.Errorf("rot %v, %v: после Save→Load стикер %s другой", rot, f, id)
				}
			}
			e.Undo()
			if st := e.View().Settings; st.PageW != 300 || st.PageH != 300 {
				t.Fatalf("отмена: формат %vx%v", st.PageW, st.PageH)
			}
		}
	}
}

// Растяжимая фигура (пропорция свободна) на проекте-листе 031 (лист = страница, без корешка), во всю
// высоту — прижата сверху и снизу; две смены формата подряд оставляют её на листе, края прижатыми.
func TestStickerRescaleStretchOnSheet(t *testing.T) {
	for _, rot := range []float64{0, 30} {
		e := edtest.Open(t, model.NewSheet(200, 100), "")
		sid := edtest.P(e).Spreads[0].ID
		_, id, err := e.AddSticker(sid, 0, model.StickerSrc{Art: &model.Art{Kind: "plate"}})
		if err != nil || id == "" {
			t.Fatalf("AddSticker: %q, %v", id, err)
		}
		// Высота рамки подобрана так, чтобы описанный прямоугольник был ровно во всю высоту листа.
		rad, w := rot*math.Pi/180, 120.0
		h := (100 - w*math.Sin(rad)) / math.Cos(rad)
		e.SetStickerBox(sid, id, 0, 0, w, h, rot, "")
		a := rsBox(t, e, id).Rect().AABB()
		e.SetStickerBox(sid, id, 30-a.X, -a.Y, w, h, rot, "")
		if st := rsBox(t, e, id); !(st.W == w && st.H > 0) || math.Abs(st.Rect().AABB().H-100) > 0.05 {
			t.Fatalf("rot %v: исходный стикер не встал во всю высоту: %+v", rot, st)
		}
		for _, f := range [][2]float64{{300, 150}, {120, 200}} {
			rsSetPage(e, f[0], f[1])
			b := rsBox(t, e, id).Rect().AABB()
			l, r, tp, bt := b.X, f[0]-b.X-b.W, b.Y, f[1]-b.Y-b.H
			// Повёрнутая растяжимая при нехватке ширины сжимается целиком — прижатие обоих краёв не обещано.
			pinned := rot != 0 || tp <= 0.05 && bt <= 0.05
			if min(l, r, tp, bt) < -0.02 || !pinned {
				t.Errorf("rot %v, %v: зазоры %.2f %.2f %.2f %.2f, ждали прижатие сверху и снизу", rot, f, l, r, tp, bt)
			}
		}
		// Круг: Save→Load ничего не двигает; две смены формата отменяются двумя шагами.
		p := edtest.SaveLoad(t, e)
		if s, i := p.FindSticker(id); s == nil || !reflect.DeepEqual(s.Stickers[i], rsBox(t, e, id)) {
			t.Errorf("rot %v: после Save→Load стикер другой", rot)
		}
		e.Undo()
		e.Undo()
		if st := e.View().Settings; st.PageW != 200 || st.PageH != 100 {
			t.Errorf("rot %v: после двух отмен формат %vx%v", rot, st.PageW, st.PageH)
		}
	}
}

// Стикер больше листа после смены формата сжимается до листа с пропорцией, несколько разом.
func TestStickerRescaleShrinksToSheet(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t) // лист 600×300
	a := rsSticker(t, e, s1, 100, 100)
	b := rsSticker(t, e, s1, 200, 100)
	e.SetStickerBox(s1, a, 160, 10, 280, 280, 0, "")
	e.SetStickerBox(s1, b, 10, 5, 580, 290, 0, "")
	rsSetPage(e, 300, 100) // лист 600×100
	for _, id := range []string{a, b} {
		st := rsBox(t, e, id)
		l, r, tp, bt := rsGaps(st, 300, 100)
		if min(l, r, tp, bt) < -0.02 || math.Abs(st.W/st.H-map[string]float64{a: 1, b: 2}[id]) > 0.02 {
			t.Errorf("%s: %+v, зазоры %.2f %.2f %.2f %.2f", id, st, l, r, tp, bt)
		}
	}
}

// Вынесенный пользователем за лист стикер при смене формата не трогается (кроме общего пересчёта).
func TestStickerRescaleLeavesOutsideAlone(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := rsSticker(t, e, s1, 100, 100)
	e.SetStickerBox(s1, id, -30, 250, 100, 100, 0, "")
	before := rsBox(t, e, id)
	rsSetPage(e, 400, 300)
	st := rsBox(t, e, id)
	// Как раньше: центр ×kx/×ky, W ×kx, H по пропорции от W (центр на листе не обязателен).
	wantW := geom.Round2(before.W * 400 / 300)
	if st.W != wantW || st.H != wantW {
		t.Fatalf("размер: %+v, ждали %v", st, wantW)
	}
	cx, cy := st.X+st.W/2, st.Y+st.H/2
	if math.Abs(cx-20.0*400/300) > 0.02 || math.Abs(cy-300) > 0.02 {
		t.Fatalf("вынесенный стикер сдвинут: центр (%v, %v), %+v", cx, cy, st)
	}
}

// Холостая смена формата — ни шага, ни rev; стикеры у края стоят.
func TestStickerRescaleNoopSameFormat(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := rsSticker(t, e, s1, 100, 100)
	e.SetStickerBox(s1, id, 0, 0, 60, 60, 0, "")
	before := rsBox(t, e, id)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	st := e.View().Settings
	e.SetSettings(st)
	edtest.Same(t, e, rev, undo, "тот же формат")
	if *rsBox(t, e, id) != *before {
		t.Fatal("стикер сдвинулся")
	}
}
