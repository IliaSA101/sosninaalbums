package geom

import (
	"encoding/json"
	"flag"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

var update = flag.Bool("update", false, "перезаписать testdata/parity/frames.json")

// frameCase — общий вектор кадра для Go и frontend (geom/frame.ts, frame.test.ts).
type frameCase struct {
	Cell   layout.Rect `json:"cell"`
	Aspect float64     `json:"aspect"`
	Frame  Crop        `json:"frame"`
	Px     int         `json:"px"` // ширина фото в пикселях (PhotoRef.W); 0 — неизвестна
	Img    layout.Rect `json:"img"`
	DPI    int         `json:"dpi"`
}

func frameCases() []frameCase {
	sq := layout.Rect{X: 10, Y: 20, W: 100, H: 100}
	return []frameCase{
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 1}, Px: 6000},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 2, X: -1, Y: -1}, Px: 6000},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 2, X: 1, Y: 1}, Px: 6000},
		{Cell: layout.Rect{W: 80, H: 120}, Aspect: 1.5, Frame: Crop{S: 1, X: 0.5}, Px: 6000},
		{Cell: layout.Rect{X: 300, Y: 12, W: 150, H: 100}, Aspect: 1.5, Frame: Crop{S: 1, X: 1, Y: -1}, Px: 6000},
		{Cell: layout.Rect{X: 312, Y: 12, W: 100, H: 100}, Aspect: 4000.0 / 6000, Frame: Crop{S: 1.35, X: -0.4, Y: 0.25}, Px: 4000},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 4, X: 0.123, Y: -0.5}, Px: 0},
		// Поворот фото в рамке (010): четверти, наклон, ход по осям фото.
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 1, R: 90}, Px: 6000},
		{Cell: layout.Rect{X: 10, Y: 20, W: 80, H: 120}, Aspect: 1.5, Frame: Crop{S: 1, R: -90, X: 0.5}, Px: 6000},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 1, R: 180, X: 1, Y: -1}, Px: 6000},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 1, R: 30}, Px: 6000},
		{Cell: layout.Rect{X: 300, Y: 12, W: 150, H: 100}, Aspect: 1.5, Frame: Crop{S: 1.5, X: 0.5, Y: -0.75, R: -12.3}, Px: 6000},
		{Cell: layout.Rect{X: 312, Y: 12, W: 100, H: 100}, Aspect: 4000.0 / 6000, Frame: Crop{S: 2.25, X: -1, Y: 1, R: 44.9}, Px: 4000},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 3, X: 0.3, Y: 0.6, R: 135.1}, Px: 0},
		{Cell: sq, Aspect: 1.5, Frame: Crop{S: 1, X: 1, Y: 1, R: -135.1}, Px: 6000},
	}
}

func near(a, b layout.Rect) bool {
	return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9 && math.Abs(a.W-b.W) < 1e-9 && math.Abs(a.H-b.H) < 1e-9
}

func TestFramesParity(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "parity", "frames.json")
	if *update {
		cs := frameCases()
		for i := range cs {
			cs[i].Img = PlaceImage(cs[i].Cell, cs[i].Aspect, cs[i].Frame)
			cs[i].DPI = FrameDPI(cs[i].Px, cs[i].Img)
		}
		data, _ := json.MarshalIndent(cs, "", "  ")
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cs []frameCase
	if err := json.Unmarshal(data, &cs); err != nil || len(cs) != len(frameCases()) {
		t.Fatalf("векторы: %v, %d", err, len(cs))
	}
	for i, c := range cs {
		img := PlaceImage(c.Cell, c.Aspect, c.Frame)
		if !near(img, c.Img) || FrameDPI(c.Px, img) != c.DPI {
			t.Errorf("вектор %d: %+v dpi %d, ждали %+v dpi %d", i, img, FrameDPI(c.Px, img), c.Img, c.DPI)
		}
		// Фото всегда закрывает ячейку целиком.
		if !coversCell(img, c.Cell, c.Frame.R) {
			t.Errorf("вектор %d: ячейка открыта", i)
		}
	}
	// Ручная сверка: 3:2 в квадрате 100 мм, S=1 — 150×100 по центру; 6000 px на 150 мм ≈ 1016 dpi.
	if !near(cs[0].Img, layout.Rect{X: -15, Y: 20, W: 150, H: 100}) || cs[0].DPI != 1016 {
		t.Fatalf("вектор 0: %+v %d", cs[0].Img, cs[0].DPI)
	}
}

// coversCell — фото img, повёрнутое на r вокруг своего центра, закрывает ячейку cell:
// все её углы, переведённые в оси фото, лежат внутри img.
func coversCell(img, cell layout.Rect, r float64) bool {
	cx, cy := img.X+img.W/2, img.Y+img.H/2
	const eps = 1e-7
	for _, c := range [4][2]float64{{0, 0}, {cell.W, 0}, {0, cell.H}, {cell.W, cell.H}} {
		u, v := Rotate(-r, cell.X+c[0]-cx, cell.Y+c[1]-cy)
		if math.Abs(u) > img.W/2+eps || math.Abs(v) > img.H/2+eps {
			return false
		}
	}
	return true
}

// Закрытие точное при любых θ, S, X, Y; ход сдвига до краёв.
func TestPlaceImageRotCovers(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for range 3000 {
		cell := layout.Rect{X: rng.Float64() * 300, Y: rng.Float64() * 300, W: 10 + rng.Float64()*200, H: 10 + rng.Float64()*200}
		f := Crop{S: 1 + rng.Float64()*3, X: rng.Float64()*2 - 1, Y: rng.Float64()*2 - 1, R: NormDeg(rng.Float64()*360 - 180)}
		switch rng.Intn(4) { // края хода и четверти — самые острые случаи
		case 0:
			f.X, f.Y = math.Round(f.X), math.Round(f.Y)
		case 1:
			f.R = float64(90 * (rng.Intn(4) - 1))
		}
		a := 0.3 + rng.Float64()*3
		if img := PlaceImage(cell, a, f); !coversCell(img, cell, f.R) {
			t.Fatalf("ячейка открыта: cell %+v, a %.3f, кадр %+v, Img %+v", cell, a, f, img)
		}
		// Вписанное фото (contain): углы фото в рамке, пропорции фото.
		if c := ContainImage(cell, a, f.R); !inside(c, f.R, cell) || math.Abs(c.W/c.H-a) > 1e-9 {
			t.Fatalf("contain %+v вне рамки %+v (a %.3f, R %v)", c, cell, a, f.R)
		}
	}
}

// inside — img, повёрнутое на r вокруг своего центра, лежит в рамке cell.
func inside(img layout.Rect, r float64, cell layout.Rect) bool {
	cx, cy := img.X+img.W/2, img.Y+img.H/2
	for _, uv := range [4][2]float64{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
		dx, dy := Rotate(r, uv[0]*img.W/2, uv[1]*img.H/2)
		x, y := cx+dx, cy+dy
		if x < cell.X-1e-9 || x > cell.X+cell.W+1e-9 || y < cell.Y-1e-9 || y > cell.Y+cell.H+1e-9 {
			return false
		}
	}
	return true
}

// R = 0 — прежняя формула дословно (до последнего бита).
func TestPlaceImageRotZeroSame(t *testing.T) {
	old := func(cell layout.Rect, aspect float64, f Crop) layout.Rect {
		w, h := cell.W, cell.W/aspect
		if h < cell.H {
			h, w = cell.H, cell.H*aspect
		}
		w, h = w*f.S, h*f.S
		fx, fy := w-cell.W, h-cell.H
		return layout.Rect{X: cell.X - fx*(1+f.X)/2, Y: cell.Y - fy*(1+f.Y)/2, W: w, H: h}
	}
	rng := rand.New(rand.NewSource(3))
	for range 500 {
		cell := layout.Rect{X: rng.Float64() * 600, Y: rng.Float64() * 300, W: 10 + rng.Float64()*280, H: 10 + rng.Float64()*280}
		f := Crop{S: 1 + rng.Float64()*3, X: rng.Float64()*2 - 1, Y: rng.Float64()*2 - 1}
		a := 0.3 + rng.Float64()*3
		want := old(cell, a, f)
		if got := PlaceImage(cell, a, f); got != want {
			t.Fatalf("%+v != %+v", got, want)
		}
		if r := placeImageRot(cell, a, f); !near6(r, want) {
			t.Fatalf("ветка с поворотом при 0 — %+v, ждали %+v", r, want)
		}
	}
}

func near6(a, b layout.Rect) bool {
	return math.Abs(a.X-b.X) < 1e-6 && math.Abs(a.Y-b.Y) < 1e-6 && math.Abs(a.W-b.W) < 1e-6 && math.Abs(a.H-b.H) < 1e-6
}

// contain с поворотом: повёрнутое фото целиком внутри ячейки и упирается хотя бы в одну сторону.
func TestContainImageRot(t *testing.T) {
	cell := layout.Rect{X: 10, Y: 20, W: 120, H: 80}
	for _, r := range []float64{0, 90, -90, 180, 30, -44.9, 135.1} {
		img := ContainImage(cell, 1.5, r)
		sn, cs := sinCosDeg(r)
		c, s := math.Abs(cs), math.Abs(sn)
		bw, bh := img.W*c+img.H*s, img.W*s+img.H*c
		if bw > cell.W+1e-9 || bh > cell.H+1e-9 || math.Abs(bw-cell.W) > 1e-9 && math.Abs(bh-cell.H) > 1e-9 {
			t.Errorf("R=%v: описанный %.3f×%.3f в ячейке %v×%v", r, bw, bh, cell.W, cell.H)
		}
		if math.Abs(img.X+img.W/2-(cell.X+cell.W/2)) > 1e-9 || math.Abs(img.Y+img.H/2-(cell.Y+cell.H/2)) > 1e-9 {
			t.Errorf("R=%v: не по центру: %+v", r, img)
		}
	}
	if ContainImage(cell, 1.5, 0) != (layout.Rect{X: 10, Y: 20, W: 120, H: 80}) {
		t.Fatal("R = 0: прежняя формула")
	}
}
