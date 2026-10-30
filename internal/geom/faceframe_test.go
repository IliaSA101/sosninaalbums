package geom

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Подгон кадра по лицу (спека 130, design §4, решения 0.27-130-4, 11, 14).

// photoPt — точка кадра после q четвертей (u, v) в осях файла. Формулы выписаны по четвертям,
// а не циклом, как в коде: независимая проверка поворота и зеркала.
func photoPt(q int, u, v float64) (a, b float64) {
	switch q {
	case 1:
		return v, 1 - u
	case 2:
		return 1 - u, 1 - v
	case 3:
		return 1 - v, u
	}
	return u, v
}

// facePos — где на листе стоит точка лица (центр рамки по X, глаза по Y) при кадре c:
// точка файла → зеркало → Img → поворот на R вокруг центра Img.
func facePos(win layout.Rect, o Outline, aspect float64, c Crop, flipH, flipV bool, box layout.Rect, eye Pt) (x, y float64) {
	q := int(math.Round(c.R/90)) % 4
	if q < 0 {
		q += 4
	}
	a, b := photoPt(q, box.X+box.W/2, eye.Y)
	if flipH {
		a = 1 - a
	}
	if flipV {
		b = 1 - b
	}
	img := PlaceImageIn(win, aspect, c, o)
	cx, cy := img.X+img.W/2, img.Y+img.H/2
	dx, dy := Rotate(c.R, img.X+a*img.W-cx, img.Y+b*img.H-cy)
	return cx + dx, cy + dy
}

// faceHeight — высота рамки лица на листе: вертикаль листа при нечётных четвертях идёт по ширине файла.
func faceHeight(win layout.Rect, o Outline, aspect float64, c Crop, box layout.Rect) float64 {
	img := PlaceImageIn(win, aspect, c, o)
	if q := int(math.Round(c.R / 90)); q%2 != 0 {
		return box.H * img.W
	}
	return box.H * img.H
}

// faceCase — один вход подгона для таблицы.
type faceCase struct {
	name   string
	win    layout.Rect
	o      Outline
	aspect float64
	r      float64
	fh, fv bool
}

func TestFaceCropTable(t *testing.T) {
	wins := map[string]layout.Rect{
		"место":   {X: 10, Y: 7, W: 50, H: 65},
		"рамочка": {X: 14, Y: 11, W: 41.5, H: 57.3}, // окно меньше места на толщину рамочки
		"широкое": {X: 0, Y: 0, W: 90, H: 60},
	}
	outlines := map[string]Outline{"прямоугольник": {}, "овал": {Oval: true}, "скругление": {Radius: 6}}
	n := 0
	for _, aspect := range []float64{1.5, 2.0 / 3, 1} {
		for wn, win := range wins {
			for on, o := range outlines {
				for _, r := range []float64{0, 90, 180, 270, 7.5, 97.5, -12.3, 187} {
					for _, fl := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
						checkFaceCase(t, faceCase{fmt.Sprint(aspect, " ", wn, " ", on, " ", r, " ", fl), win, o, aspect, r, fl[0], fl[1]})
						n++
					}
				}
			}
		}
	}
	if n != 3*3*3*8*4 {
		t.Fatalf("случаев %d", n)
	}
}

func checkFaceCase(t *testing.T, c faceCase) {
	t.Helper()
	norm := DefaultFaceNorm
	box := layout.Rect{X: 0.42, Y: 0.28, W: 0.13, H: 0.2}
	eye := Pt{X: 0.48, Y: 0.37}
	// прежний S/X/Y подгон не использует
	got, miss := FaceCrop(c.win, c.o, c.aspect, Crop{S: 1.3, X: 0.2, Y: -0.4, R: c.r}, c.fh, c.fv, box, eye, norm)
	if miss.Bad {
		t.Fatalf("%s: Bad", c.name)
	}
	if got.R != c.r {
		t.Errorf("%s: R %v, ждали %v (поворот не меняется)", c.name, got.R, c.r)
	}
	if got.S < 1 || got.S > MaxScale || math.Abs(got.X) > 1 || math.Abs(got.Y) > 1 {
		t.Errorf("%s: кадр вне пределов %+v", c.name, got)
	}
	x, y := facePos(c.win, c.o, c.aspect, got, c.fh, c.fv, box, eye)
	tx, ty := c.win.X+0.5*c.win.W, c.win.Y+norm.Eye*c.win.H
	tilted := c.r != math.Round(c.r/90)*90
	if tilted {
		// Точка и линия глаз вместе достижимы (решение системы лежит в квадрате) — наклон не
		// отнимает точность ни по одной оси; иначе полоса faceYBand тратится не больше своей ширины.
		fs := faceSpot{win: c.win, o: c.o, aspect: c.aspect, flipH: c.fh, flipV: c.fv, box: box, eye: eye}
		tx, ty := c.win.X+c.win.W/2, c.win.Y+norm.Eye*c.win.H
		limit := unreachableY(fs, got, ty) + faceYBand + 0.1
		if bothReachable(fs, got, tx, ty) {
			limit = 0.1
			if math.Abs(miss.DX) > 0.1 {
				t.Errorf("%s: центр достижим, а DX = %.3f мм", c.name, miss.DX)
			}
		}
		if math.Abs(miss.DY) > limit {
			t.Errorf("%s: DY = %.3f мм, допуск %.2f (DX = %.3f)", c.name, miss.DY, limit, miss.DX)
		}
	}
	if miss.Flag() != FaceOK || tilted {
		// Наклон ограничивает ход по обеим осям листа сразу: невязка (до 1 мм) должна
		// честно называть то, что не достигнуто. Точность 0,1 мм — без наклона.
		if math.Abs(x-tx-miss.DX) > 0.1 || math.Abs(y-ty-miss.DY) > 0.1 {
			t.Errorf("%s: невязка %+v не сходится с положением (%.2f, %.2f)", c.name, miss, x, y)
		}
		return
	}
	if math.Abs(x-tx) > 0.1 || math.Abs(y-ty) > 0.1 {
		t.Errorf("%s: глаза в (%.2f, %.2f), цель (%.2f, %.2f)", c.name, x, y, tx, ty)
	}
	if h := faceHeight(c.win, c.o, c.aspect, got, box); math.Abs(h/c.win.H-norm.Size) > 0.12*norm.Size {
		t.Errorf("%s: рамка %.3f высоты окна, норма %v", c.name, h/c.win.H, norm.Size)
	}
}

// Лицо по центру кадра и достижимого размера: подгон не должен давать флаг ни в одном из поворотов.
func TestFaceCropCenteredNoFlag(t *testing.T) {
	win := layout.Rect{W: 50, H: 65}
	box := layout.Rect{X: 0.4, Y: 0.3, W: 0.2, H: 0.25}
	eye := Pt{X: 0.5, Y: 0.4}
	for _, r := range []float64{0, 90, 180, 270, 7.5, 97.5} {
		_, m := FaceCrop(win, Outline{}, 1.5, Crop{S: 1, R: r}, false, false, box, eye, DefaultFaceNorm)
		if m.Flag() != FaceOK {
			t.Errorf("R=%v: флаг %v, невязка %+v", r, m.Flag(), m)
		}
	}
}

// Марина: лицо 0,144 фото 2:3 в месте 47,5 × 63,3 → S = 0,35·63,3 / (0,144·47,5·1,5) = 2,16.
func TestFaceCropMarina(t *testing.T) {
	win := layout.Rect{W: 47.5, H: 63.3}
	box := layout.Rect{X: 0.4, Y: 0.2, W: 0.1, H: 0.144}
	eye := Pt{X: 0.45, Y: 0.25}
	got, m := FaceCrop(win, Outline{}, 2.0/3, Crop{S: 1}, false, false, box, eye, DefaultFaceNorm)
	if math.Abs(got.S-2.16) > 0.01 {
		t.Errorf("S = %v, ждали 2,16 ± 0,01", got.S)
	}
	if m.Flag() != FaceOK {
		t.Errorf("невязка %+v", m)
	}
}

func TestFaceMissFlags(t *testing.T) {
	for _, c := range []struct {
		name string
		m    FaceMiss
		want FaceFlag
	}{
		{"в норме", FaceMiss{Size: 0.05, DX: 0.5, DY: -0.9}, FaceOK},
		{"мелкое ровно 12 %", FaceMiss{Size: -0.12}, FaceOK},
		{"мелкое", FaceMiss{Size: -0.121}, FaceSmall},
		{"крупное ровно 12 %", FaceMiss{Size: 0.12}, FaceOK},
		{"крупное", FaceMiss{Size: 0.13}, FaceBig},
		{"у края по X", FaceMiss{DX: 1.01}, FaceEdge},
		{"у края по Y", FaceMiss{DY: -1.01}, FaceEdge},
		{"ровно 1 мм", FaceMiss{DX: 1, DY: -1}, FaceOK},
		{"размер важнее положения", FaceMiss{Size: -0.3, DX: 5}, FaceSmall},
		{"негодный вход флага не даёт", FaceMiss{Size: -0.5, DX: 9, Bad: true}, FaceOK},
	} {
		if got := c.m.Flag(); got != c.want {
			t.Errorf("%s: флаг %v, ждали %v", c.name, got, c.want)
		}
	}
}

// Флаги ставятся по невязке, а не по упору: S = 4,03 при лице 99 % нормы флага не даёт,
// а лицо в рост при упоре — «мелкое», глаза при этом на линии.
func TestFaceCropResidual(t *testing.T) {
	win := layout.Rect{X: 3, Y: 4, W: 50, H: 65}
	eye := func(b layout.Rect) Pt { return Pt{X: b.X + b.W/2, Y: b.Y + 0.4*b.H} }
	norm := DefaultFaceNorm

	// Нужен S = 4,03: упор, невязка размера 0,7 % — флага нет.
	h1 := 0.35 * win.H / 4.03 / 65 // высота рамки в долях кадра при S = 1 (фото 2:3, высота Img = 65)
	box := layout.Rect{X: 0.4, Y: 0.4, W: h1, H: h1}
	got, m := FaceCrop(win, Outline{}, 50.0/65, Crop{S: 1}, false, false, box, eye(box), norm)
	if got.S != MaxScale {
		t.Fatalf("S = %v, ждали упор %v", got.S, MaxScale)
	}
	if m.Flag() != FaceOK || math.Abs(m.Size+0.0074) > 0.001 {
		t.Errorf("лицо 99 %% нормы: %+v флаг %v", m, m.Flag())
	}

	// Лицо вдвое мельче достижимого: упор, глаза на линии, флаг «мелкое».
	box = layout.Rect{X: 0.45, Y: 0.4, W: 0.03, H: 0.03}
	got, m = FaceCrop(win, Outline{}, 50.0/65, Crop{S: 1}, false, false, box, eye(box), norm)
	if got.S != MaxScale || m.Flag() != FaceSmall || m.Size > -0.5 {
		t.Errorf("лицо в рост: S %v, %+v, флаг %v", got.S, m, m.Flag())
	}
	if math.Abs(m.DY) > 0.1 {
		t.Errorf("при упоре глаза мимо линии на %.2f мм", m.DY)
	}

	// Лицо во весь кадр: S = 1, флаг «крупное».
	box = layout.Rect{X: 0.1, Y: 0.05, W: 0.8, H: 0.9}
	got, m = FaceCrop(win, Outline{}, 50.0/65, Crop{S: 1.7}, false, false, box, eye(box), norm)
	if got.S != 1 || m.Flag() != FaceBig {
		t.Errorf("лицо крупное: S %v, %+v, флаг %v", got.S, m, m.Flag())
	}

	// Лицо у левого края: сдвига не хватает — «у края», линия глаз держится.
	box = layout.Rect{X: 0.0, Y: 0.4, W: 0.12, H: 0.18}
	got, m = FaceCrop(win, Outline{}, 50.0/65, Crop{S: 1}, false, false, box, eye(box), norm)
	if m.Flag() != FaceEdge || m.DX > -1 || math.Abs(m.DY) > 0.1 {
		t.Errorf("у края: %+v флаг %v, кадр %+v", m, m.Flag(), got)
	}
	if got.X != -1 && got.X != 1 {
		t.Errorf("у края сдвиг должен упереться в ±1: %+v", got)
	}
}

// Упор с наклоном: оси фото повёрнуты, поэтому X и Y кадра влияют на обе координаты листа.
// Линия глаз (Y листа) остаётся в полосе faceYBand от лучшей достижимой, центр (X листа) — по остатку хода.
func TestFaceCropTiltedLimit(t *testing.T) {
	win := layout.Rect{X: 5, Y: 5, W: 50, H: 65}
	box := layout.Rect{X: 0.62, Y: 0.30, W: 0.03, H: 0.035} // мелкое лицо справа вверху → S = 4
	eye := Pt{X: 0.635, Y: 0.31}
	for _, r := range []float64{20, -20, 33, 110, 200} {
		got, m := FaceCrop(win, Outline{}, 1.5, Crop{S: 1, R: r}, false, false, box, eye, DefaultFaceNorm)
		if got.S != MaxScale {
			t.Fatalf("R=%v: S = %v, ждали упор", r, got.S)
		}
		if math.Abs(m.DY) > 0.1 {
			t.Errorf("R=%v: глаза мимо линии на %.2f мм (%+v)", r, m.DY, got)
		}
		if m.Flag() != FaceSmall {
			t.Errorf("R=%v: флаг %v, ждали «мелкое»: %+v", r, m.Flag(), m)
		}
		x, y := facePos(win, Outline{}, 1.5, got, false, false, box, eye)
		if math.Abs(y-(win.Y+DefaultFaceNorm.Eye*win.H)) > 0.1 {
			t.Errorf("R=%v: независимый расчёт: глаза на y = %.2f", r, y)
		}
		if math.Abs(x-(win.X+win.W/2)-m.DX) > 0.1 {
			t.Errorf("R=%v: DX %.2f не сходится с положением %.2f", r, m.DX, x-(win.X+win.W/2))
		}
	}
}

// Лицо далеко от центра при наклоне: центр (X листа) недостижим, линия глаз остаётся в полосе
// faceYBand от лучшей достижимой (раньше держалась строго, ценой скачка по X, 0.27-130-72).
func TestFaceCropTiltedFarFace(t *testing.T) {
	win := layout.Rect{X: 5, Y: 5, W: 50, H: 65}
	ty := win.Y + DefaultFaceNorm.Eye*win.H
	for _, c := range [][2]float64{{0.95, 0.5}, {0.97, 0.05}, {0.03, 0.9}, {0.5, 0.02}} {
		box := layout.Rect{X: c[0] - 0.015, Y: c[1] - 0.0175, W: 0.03, H: 0.035}
		eye := Pt{X: c[0], Y: box.Y + 0.4*box.H}
		for _, r := range []float64{20, 110} {
			got, m := FaceCrop(win, Outline{}, 1.5, Crop{S: 1, R: r}, false, false, box, eye, DefaultFaceNorm)
			fs := faceSpot{win: win, aspect: 1.5, box: box, eye: eye}
			g := unreachableY(fs, got, ty)
			if got.S != MaxScale || math.Abs(m.DY) > g+2*FaceShiftTol+0.1 {
				t.Errorf("лицо %v, R=%v: кадр %+v, невязка %+v: ждали упор и линию глаз в полосе %.2f мм от достижимой (%.2f)", c, r, got, m, faceYBand, g)
			}
			x, y := facePos(win, Outline{}, 1.5, got, false, false, box, eye)
			if math.Abs(y-ty-m.DY) > 0.1 || math.Abs(x-(win.X+win.W/2)-m.DX) > 0.1 {
				t.Errorf("лицо %v, R=%v: независимый расчёт (%.2f, %.2f), невязка %+v", c, r, x, y, m)
			}
		}
	}
}

// Лицо низко в кадре и по центру по X: линия глаз недостижима, X листа свободен. Малый наклон
// не должен уводить лицо вбок (раньше при R = 1° центр уезжал на ±25 мм ради долей мм по Y),
// а результат — быть непрерывным по R (0.27-130-72).
func TestFaceCropLowFaceTiltContinuous(t *testing.T) {
	win := layout.Rect{X: 5, Y: 5, W: 50, H: 65}
	// низ кадра; ход кадра по X ≈ 12 мм (S ≈ 1,5) и ≈ 37 мм (S ≈ 2,5)
	for _, box := range []layout.Rect{{X: 0.4, Y: 0.7, W: 0.2, H: 0.2}, {X: 0.44, Y: 0.8, W: 0.12, H: 0.12}} {
		eye := Pt{X: 0.5, Y: box.Y + 0.4*box.H}
		for _, fl := range [][2]bool{{false, false}, {true, false}} {
			var prev Crop
			var prevM FaceMiss
			for i, r := range []float64{0, 1, 3, -1, -3} {
				got, m := FaceCrop(win, Outline{}, 2.0/3, Crop{S: 1, R: r}, fl[0], fl[1], box, eye, DefaultFaceNorm)
				if math.Abs(m.DX) >= 1 {
					t.Errorf("лицо %+v, флип %v, R=%v: DX = %.2f мм, ждали < 1: кадр %+v", box, fl, r, m.DX, got)
				}
				if i == 0 {
					fs := faceSpot{win: win, aspect: 2.0 / 3, flipH: fl[0], flipV: fl[1], box: box, eye: eye}
					if unreachableY(fs, got, win.Y+DefaultFaceNorm.Eye*win.H) < 5 {
						t.Fatalf("лицо %+v: линия глаз достижима — проверка потеряла смысл: %+v", box, m)
					}
					prev, prevM = got, m
					continue
				}
				// R = ±1° от R = 0: центр и линия глаз близки, кадр сдвинулся мало
				if r == 1 || r == -1 {
					if math.Abs(m.DX-prevM.DX) > 1 || math.Abs(m.DY-prevM.DY) > 2*FaceShiftTol+0.1 || math.Abs(got.X-prev.X) > 0.1 {
						t.Errorf("лицо %+v, флип %v, R=%v: скачок против R=0: %+v %+v → %+v %+v", box, fl, r, prev, prevM, got, m)
					}
				}
			}
		}
	}
}

// Полоса линии глаз не шире 2 порогов невязки: шире — глаза «гуляют» ради центра, что видно на печати.
func TestFaceYBandBounded(t *testing.T) {
	if faceYBand <= 0 || faceYBand > 2*FaceShiftTol || faceXTol <= 0 || faceXTol > FaceShiftTol {
		t.Errorf("полоса %v мм, допуск центра %v мм", faceYBand, faceXTol)
	}
}

// Точность решения: на случайных входах (наклон, форма, зеркало) линия глаз стоит в полосе
// faceYBand от ближайшей достижимой, центр по X не хуже, чем у любой точки полосы (перебор по
// сетке квадрата X, Y) или в допуске faceXTol; цель достижима целиком — линия глаз точна.
func TestFaceCropSolveRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(130))
	outs := []Outline{{}, {Oval: true}, {Radius: 5}}
	for i := range 400 {
		fs := faceSpot{
			win:    layout.Rect{X: rng.Float64() * 20, Y: rng.Float64() * 20, W: 30 + rng.Float64()*70, H: 30 + rng.Float64()*70},
			aspect: 0.5 + rng.Float64()*1.5,
			o:      outs[rng.Intn(len(outs))],
			flipH:  rng.Intn(2) == 0,
			flipV:  rng.Intn(2) == 0,
		}
		r := float64(rng.Intn(4)) * 90
		if rng.Intn(3) > 0 {
			r += (rng.Float64()*2 - 1) * 44
		}
		bh := 0.02 + rng.Float64()*0.45
		fs.box = layout.Rect{X: 0.05 + rng.Float64()*0.6, Y: 0.05 + rng.Float64()*0.4, W: bh * 0.7, H: bh}
		fs.eye = Pt{X: fs.box.X + fs.box.W/2, Y: fs.box.Y + 0.38*fs.box.H}
		got, m := FaceCrop(fs.win, fs.o, fs.aspect, Crop{S: 1, R: r}, fs.flipH, fs.flipV, fs.box, fs.eye, DefaultFaceNorm)
		tx, ty := fs.win.X+fs.win.W/2, fs.win.Y+DefaultFaceNorm.Eye*fs.win.H

		g := unreachableY(fs, got, ty)
		if gotDY := math.Abs(m.DY); gotDY > g+2*FaceShiftTol+0.2 {
			t.Fatalf("#%d: |DY| = %.3f, достижимо %.3f, полоса %.2f (кадр %+v, R=%v)", i, gotDY, g, faceYBand, got, r)
		}
		bestDX := bestInBand(fs, got, ty, tx)
		if math.Abs(m.DX) > max(bestDX, faceXTol)+0.2 {
			t.Fatalf("#%d: |DX| = %.3f, в полосе достижимо %.3f (кадр %+v, R=%v)", i, math.Abs(m.DX), bestDX, got, r)
		}
		if bothReachable(fs, got, tx, ty) && math.Abs(m.DY) > 0.2 {
			t.Fatalf("#%d: цель достижима целиком, а DY = %.3f (кадр %+v, R=%v)", i, m.DY, got, r)
		}
	}
}

// bothReachable — при кадре c (S и R) в квадрате X, Y есть точка, где центр и линия глаз
// лежат на цели вместе: решение системы по трём пробам лежит в [−1, 1]².
func bothReachable(fs faceSpot, c Crop, tx, ty float64) bool {
	probe := func(x, y float64) (px, py float64) {
		_, px, py = fs.probe(Crop{S: c.S, X: x, Y: y, R: c.R})
		return
	}
	x0, y0 := probe(0, 0)
	xa, ya := probe(1, 0)
	xb, yb := probe(0, 1)
	e, a, f, b := xa-x0, ya-y0, xb-x0, yb-y0
	det := e*b - f*a
	if math.Abs(det) < 1e-9 {
		return false
	}
	X := ((tx-x0)*b - f*(ty-y0)) / det
	Y := (e*(ty-y0) - a*(tx-x0)) / det
	return math.Abs(X) <= 1 && math.Abs(Y) <= 1
}

// unreachableY — на сколько мм линия глаз ty лежит вне диапазона Y листа при кадре c
// (аффинная функция — крайности в углах квадрата X, Y); 0 — достижима.
func unreachableY(fs faceSpot, c Crop, ty float64) float64 {
	ymin, ymax := math.Inf(1), math.Inf(-1)
	for _, cx := range [...]float64{-1, 1} {
		for _, cy := range [...]float64{-1, 1} {
			_, _, y := fs.probe(Crop{S: c.S, X: cx, Y: cy, R: c.R})
			ymin, ymax = min(ymin, y), max(ymax, y)
		}
	}
	return math.Max(0, math.Max(ymin-ty, ty-ymax))
}

// bestInBand — по сетке квадрата X, Y при кадре c: наименьшее |x − tx| среди точек, чья линия
// глаз в полосе faceYBand от ближайшей достижимой к ty. Сетка грубая: решение может быть лучше.
func bestInBand(fs faceSpot, c Crop, ty, tx float64) float64 {
	const n = 300
	at := func(x, y float64) (px, py float64) {
		_, px, py = fs.probe(Crop{S: c.S, X: x, Y: y, R: c.R})
		return
	}
	// ближайшая достижимая линия глаз
	yk := ty
	if g := unreachableY(fs, c, ty); g > 0 {
		ymin := math.Inf(1)
		for _, cx := range [...]float64{-1, 1} {
			for _, cy := range [...]float64{-1, 1} {
				_, y := at(cx, cy)
				ymin = min(ymin, y)
			}
		}
		if ymin > ty {
			yk = ymin
		} else {
			yk = ty - g // выше всех точек: ближайшая — верхняя
		}
	}
	x00, y00 := at(0, 0)
	x10, y10 := at(1, 0)
	x01, y01 := at(0, 1)
	bestDX := math.Inf(1)
	for i := 0; i <= n; i++ {
		for j := 0; j <= n; j++ {
			X, Y := -1+2*float64(i)/n, -1+2*float64(j)/n
			px := x00 + (x10-x00)*X + (x01-x00)*Y
			py := y00 + (y10-y00)*X + (y01-y00)*Y
			if math.Abs(py-yk) > faceYBand {
				continue
			}
			bestDX = min(bestDX, math.Abs(px-tx))
		}
	}
	return bestDX // +Inf, если полоса уже шага сетки: сетка её не видит, проверка пуста
}

// Подгон не зависит от прежнего S/X/Y; повтор по результату тот же; невязка совпадает с измеренной отдельно.
func TestFaceCropIdempotent(t *testing.T) {
	win := layout.Rect{X: 2, Y: 3, W: 47.5, H: 63.3}
	box := layout.Rect{X: 0.35, Y: 0.25, W: 0.12, H: 0.16}
	eye := Pt{X: 0.41, Y: 0.31}
	for _, r := range []float64{0, 90, 7.5} {
		a, ma := FaceCrop(win, Outline{}, 1.5, Crop{S: 3, X: 0.9, Y: -0.9, R: r}, true, false, box, eye, DefaultFaceNorm)
		b, mb := FaceCrop(win, Outline{}, 1.5, a, true, false, box, eye, DefaultFaceNorm)
		if a != b || ma != mb {
			t.Errorf("R=%v: повтор %+v / %+v, было %+v / %+v", r, b, mb, a, ma)
		}
		if m := FaceMissAt(win, Outline{}, 1.5, a, true, false, box, eye, DefaultFaceNorm); m != ma {
			t.Errorf("R=%v: FaceMissAt %+v, FaceCrop %+v", r, m, ma)
		}
	}
}

// FaceShare — доли лица при кадре: подогнанное под норму возвращает норму (основа «как это лицо»).
func TestFaceShare(t *testing.T) {
	win := layout.Rect{X: 2, Y: 3, W: 47.5, H: 63.3}
	box := layout.Rect{X: 0.35, Y: 0.25, W: 0.12, H: 0.16}
	eye := Pt{X: 0.41, Y: 0.31}
	n := FaceNorm{Size: 0.4, Eye: 0.3}
	for _, r := range []float64{0, 270, -9} {
		got, m := FaceCrop(win, Outline{}, 1.5, Crop{S: 1, R: r}, false, true, box, eye, n)
		if m.Flag() != FaceOK {
			t.Fatalf("R=%v: %+v", r, m)
		}
		s, ok := FaceShare(win, Outline{}, 1.5, got, false, true, box, eye)
		if !ok || math.Abs(s.Size-n.Size) > 0.005 || math.Abs(s.Eye-n.Eye) > 0.002 {
			t.Errorf("R=%v: доли %+v, норма %+v", r, s, n)
		}
	}
}

// Негодный вход — кадр как есть и Bad, без паники и без NaN в кадре.
func TestFaceCropBadInput(t *testing.T) {
	win := layout.Rect{W: 50, H: 65}
	box := layout.Rect{X: 0.4, Y: 0.3, W: 0.1, H: 0.15}
	eye := Pt{X: 0.45, Y: 0.35}
	nan, inf := math.NaN(), math.Inf(1)
	cur := Crop{S: 1.5, X: 0.1, R: 90}
	for name, c := range map[string]struct {
		win    layout.Rect
		aspect float64
		box    layout.Rect
		eye    Pt
		n      FaceNorm
	}{
		"пустая рамка":    {win, 1.5, layout.Rect{}, eye, DefaultFaceNorm},
		"нулевая высота":  {win, 1.5, layout.Rect{X: 0.4, W: 0.1}, eye, DefaultFaceNorm},
		"отрицательная H": {win, 1.5, layout.Rect{X: 0.4, W: 0.1, H: -0.1}, eye, DefaultFaceNorm},
		"NaN в рамке":     {win, 1.5, layout.Rect{X: nan, Y: 0.3, W: 0.1, H: 0.1}, eye, DefaultFaceNorm},
		"Inf в глазах":    {win, 1.5, box, Pt{X: inf, Y: 0.3}, DefaultFaceNorm},
		"пустое окно":     {layout.Rect{}, 1.5, box, eye, DefaultFaceNorm},
		"NaN в окне":      {layout.Rect{W: nan, H: 10}, 1.5, box, eye, DefaultFaceNorm},
		"пропорции 0":     {win, 0, box, eye, DefaultFaceNorm},
		"пропорции NaN":   {win, nan, box, eye, DefaultFaceNorm},
		"норма 0":         {win, 1.5, box, eye, FaceNorm{}},
		"норма NaN":       {win, 1.5, box, eye, FaceNorm{Size: nan, Eye: 0.3}},
	} {
		got, m := FaceCrop(c.win, Outline{}, c.aspect, cur, false, false, c.box, c.eye, c.n)
		if !m.Bad || got != cur || m.Flag() != FaceOK {
			t.Errorf("%s: %+v, %+v", name, got, m)
		}
		if mm := FaceMissAt(c.win, Outline{}, c.aspect, cur, false, false, c.box, c.eye, c.n); !mm.Bad {
			t.Errorf("%s: FaceMissAt не Bad", name)
		}
		if _, ok := FaceShare(c.win, Outline{}, c.aspect, cur, false, false, c.box, c.eye); ok && c.n == DefaultFaceNorm {
			t.Errorf("%s: FaceShare ok", name)
		}
	}
}

// Кадр, мимо которого модель не пропустит: S, X, Y кратны 0.001, минус нуля нет — округление
// такое же, как model.ClampFrame.round3 (geom модель не импортирует, поэтому формула здесь).
func TestFaceCropRound3(t *testing.T) {
	for _, c := range []struct{ in, want float64 }{
		{0.0004, 0}, {-0.0004, 0}, {0.0005, 0.001}, {-0.0005, -0.001},
		{1.2344, 1.234}, {2.9996, 3}, {-1, -1}, {0, 0}, {math.Copysign(0, -1), 0},
	} {
		got := faceRound(c.in)
		if got != c.want || math.Signbit(got) != math.Signbit(c.want) {
			t.Errorf("faceRound(%v) = %v, ждали %v", c.in, got, c.want)
		}
		if ref := math.Round(c.in*1000) / 1000; ref != got && ref != 0 {
			t.Errorf("faceRound(%v) = %v, а round3 = %v", c.in, got, ref)
		}
	}
	rng := rand.New(rand.NewSource(7))
	for i := range 300 {
		win := layout.Rect{X: rng.Float64() * 10, Y: rng.Float64() * 10, W: 30 + rng.Float64()*60, H: 30 + rng.Float64()*60}
		bh := 0.03 + rng.Float64()*0.4
		box := layout.Rect{X: 0.1 + rng.Float64()*0.5, Y: 0.1 + rng.Float64()*0.4, W: bh * 0.7, H: bh}
		r := float64(rng.Intn(4))*90 + (rng.Float64()*2-1)*10*float64(rng.Intn(2))
		got, _ := FaceCrop(win, Outline{}, 0.6+rng.Float64(), Crop{S: 1, R: r}, rng.Intn(2) == 0, false, box, Pt{X: box.X + box.W/2, Y: box.Y + 0.4*box.H}, DefaultFaceNorm)
		for _, v := range []float64{got.S, got.X, got.Y} {
			if math.Abs(v*1000-math.Round(v*1000)) > 1e-6 || (v == 0 && math.Signbit(v)) {
				t.Fatalf("#%d: кадр %+v не кратен 0.001 или с минус нулём", i, got)
			}
		}
	}
}

// Кадр cur с NaN/Inf (R, а у FaceMissAt и FaceShare — и S, X, Y) — негодный вход, не паника и не NaN в ответе.
func TestFaceCropBadCur(t *testing.T) {
	win := layout.Rect{W: 50, H: 65}
	box := layout.Rect{X: 0.4, Y: 0.3, W: 0.1, H: 0.15}
	eye := Pt{X: 0.45, Y: 0.35}
	nan, inf := math.NaN(), math.Inf(-1)
	for _, cur := range []Crop{{S: 1, R: nan}, {S: 1, R: inf}, {S: nan, R: 0}, {S: 1, X: inf}, {S: 1, Y: nan}} {
		if cur.S == 1 && (cur.R == 0 || math.IsNaN(cur.R) || math.IsInf(cur.R, 0)) && cur.X == 0 && cur.Y == 0 {
			got, m := FaceCrop(win, Outline{}, 1.5, cur, false, false, box, eye, DefaultFaceNorm)
			if !m.Bad || math.IsNaN(got.R) != math.IsNaN(cur.R) || got.S != cur.S {
				t.Errorf("FaceCrop, cur %+v: %+v %+v", cur, got, m)
			}
		}
		if m := FaceMissAt(win, Outline{}, 1.5, cur, false, false, box, eye, DefaultFaceNorm); !m.Bad {
			t.Errorf("FaceMissAt, cur %+v: не Bad", cur)
		}
		if _, ok := FaceShare(win, Outline{}, 1.5, cur, false, false, box, eye); ok {
			t.Errorf("FaceShare, cur %+v: ok", cur)
		}
	}
}

// Края без хода: фото ровно закрывает место (S = 1, нет свободного хода) — координата 0, без деления на ноль.
func TestFaceCropNoTravel(t *testing.T) {
	win := layout.Rect{W: 60, H: 40}
	box := layout.Rect{X: 0.3, Y: 0.3, W: 0.4, H: 0.4}
	eye := Pt{X: 0.5, Y: 0.45}
	// Лицо крупное: S = 1; фото пропорций окна — хода нет ни по X, ни по Y.
	got, m := FaceCrop(win, Outline{}, 1.5, Crop{}, false, false, box, eye, DefaultFaceNorm)
	if got != (Crop{S: 1}) || m.Bad {
		t.Errorf("кадр %+v, %+v", got, m)
	}
	if !m.Big() {
		t.Errorf("ждали «крупное»: %+v", m)
	}
}
