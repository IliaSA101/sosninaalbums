package geom

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Подгон кадра по лицу (спека 130, design §4): по рамке лица и линии глаз ставится масштаб
// и сдвиг так, чтобы лицо было нормой доли окна места, а глаза лежали на линии нормы по
// центру. Чистая геометрия: детектор и модель сюда не заходят, лицо приходит долями кадра.

// Пороги невязки (design §4.4, решение 0.27-130-14): размер мимо нормы больше чем на 12 %
// или сдвиг больше 1 мм. Флаг по невязке, а не по упору: S = 4,03 при лице 99 % нормы флага не даёт.
const (
	FaceSizeTol  = 0.12
	FaceShiftTol = 1.0 // мм
)

// Pt — точка в долях кадра (0..1), x вправо, y вниз.
type Pt struct{ X, Y float64 }

// FaceNorm — норма лица в долях высоты окна места: Size — высота рамки лица, Eye — линия
// глаз от верха окна. По X лицо стоит по центру окна.
type FaceNorm struct{ Size, Eye float64 }

// DefaultFaceNorm — начальная норма с двух фото демо-альбома Марины (решение 0.27-130-2).
var DefaultFaceNorm = FaceNorm{Size: 0.35, Eye: 0.32}

// FaceMiss — невязка подгона: Size — доля размера (факт / норма − 1, минус — лицо мельче нормы),
// DX и DY — на сколько мм точка лица (центр рамки по X, линия глаз по Y) ушла от цели.
// Bad — вход негодный (не число, пустая рамка), остальные поля тогда нули.
type FaceMiss struct {
	Size, DX, DY float64
	Bad          bool
}

// Small — лицо мельче нормы больше чем на 12 % (в рост: упёрлись в MaxScale).
func (m FaceMiss) Small() bool { return !m.Bad && m.Size < -FaceSizeTol }

// Big — лицо крупнее нормы больше чем на 12 % (S уже 1, дальше уменьшать некуда).
func (m FaceMiss) Big() bool { return !m.Bad && m.Size > FaceSizeTol }

// Edge — точка лица мимо цели больше чем на 1 мм по любой оси (лицо у края фото).
func (m FaceMiss) Edge() bool {
	return !m.Bad && (math.Abs(m.DX) > FaceShiftTol || math.Abs(m.DY) > FaceShiftTol)
}

// FaceFlag — одна причина «не по норме» на место.
type FaceFlag int

// Флаги в порядке приоритета: размер важнее положения.
const (
	FaceOK FaceFlag = iota
	FaceSmall
	FaceBig
	FaceEdge
)

// Flag — единственная причина мимо нормы: мелкое, крупное, у края; FaceOK — в норме.
// Bad флага не даёт: «негодный вход» вызывающий отсекает сам.
func (m FaceMiss) Flag() FaceFlag {
	switch {
	case m.Small():
		return FaceSmall
	case m.Big():
		return FaceBig
	case m.Edge():
		return FaceEdge
	}
	return FaceOK
}

// FaceCrop — кадр места по лицу. win и o — окно рамочки места (Project.BorderWindow) и его
// форма, aspect — RawAspect фото, cur — кадр до подгона (из него берётся только поворот R),
// flipH/flipV — зеркало фото. box (рамка) и eye (середина глаз) — в долях кадра после
// поворота четвертями cur.R, как детектор видел картинку; наклон (R минус четверти) не
// трогается. Лицо ставится: высота рамки — n.Size·win.H, центр рамки — по центру окна,
// линия глаз — на n.Eye·win.H от верха окна. S зажат в [1, MaxScale]; если сдвига не хватает,
// линия глаз (Y экрана) остаётся в полосе faceYBand от лучшей достижимой, внутри неё центр
// (X экрана) ближе к цели (solveShift, 0.27-130-72). Кадр округлён до 0.001 как
// model.ClampFrame, невязка — по округлённому. Не число (в том числе в cur) или пустая рамка — cur как есть и Bad.
func FaceCrop(win layout.Rect, o Outline, aspect float64, cur Crop, flipH, flipV bool,
	box layout.Rect, eye Pt, n FaceNorm) (Crop, FaceMiss) {
	fs := faceSpot{win: win, o: o, aspect: aspect, flipH: flipH, flipV: flipV, box: box, eye: eye}
	if !fs.valid() || !validNorm(n) || !finite(cur.R) {
		return cur, FaceMiss{Bad: true}
	}
	// Высота лица линейна по S: меряем при S = 1 и растягиваем до нормы.
	h1, _, _ := fs.probe(Crop{S: 1, R: cur.R})
	s := faceRound(ClampF(n.Size*win.H/h1, 1, MaxScale))
	c := Crop{S: s, R: cur.R}
	// При фиксированном S точка лица аффинна по (X, Y): три пробы дают матрицу.
	_, x0, y0 := fs.probe(c)
	_, xX, yX := fs.probe(Crop{S: s, X: 1, R: cur.R})
	_, xY, yY := fs.probe(Crop{S: s, Y: 1, R: cur.R})
	tx, ty := win.X+win.W/2, win.Y+n.Eye*win.H
	c.X, c.Y = solveShift(y0, yX-y0, yY-y0, x0, xX-x0, xY-x0, ty, tx)
	c.X, c.Y = faceRound(c.X), faceRound(c.Y)
	return c, fs.miss(c, n)
}

// FaceMissAt — невязка кадра cur как он есть (без подгона): по ней «правленый» кадр —
// лицо мимо нормы листа дальше порогов. Аргументы — как у FaceCrop.
func FaceMissAt(win layout.Rect, o Outline, aspect float64, cur Crop, flipH, flipV bool,
	box layout.Rect, eye Pt, n FaceNorm) FaceMiss {
	fs := faceSpot{win: win, o: o, aspect: aspect, flipH: flipH, flipV: flipV, box: box, eye: eye}
	if !fs.valid() || !validNorm(n) || !finite(cur.S, cur.X, cur.Y, cur.R) {
		return FaceMiss{Bad: true}
	}
	return fs.miss(cur, n)
}

// FaceShare — доли лица в окне при кадре cur: Size — высота рамки, Eye — линия глаз от верха
// окна. Норма «как это лицо» (NormFrom) берётся отсюда с места-образца. ok = false — вход негодный.
func FaceShare(win layout.Rect, o Outline, aspect float64, cur Crop, flipH, flipV bool,
	box layout.Rect, eye Pt) (n FaceNorm, ok bool) {
	fs := faceSpot{win: win, o: o, aspect: aspect, flipH: flipH, flipV: flipV, box: box, eye: eye}
	if !fs.valid() || !finite(cur.S, cur.X, cur.Y, cur.R) {
		return FaceNorm{}, false
	}
	h, _, y := fs.probe(cur)
	return FaceNorm{Size: h / win.H, Eye: (y - win.Y) / win.H}, true
}

// faceSpot — лицо в месте: всё, кроме кадра, чтобы считать точку лица на листе при любом кадре.
type faceSpot struct {
	win          layout.Rect
	o            Outline
	aspect       float64
	flipH, flipV bool
	box          layout.Rect
	eye          Pt
}

func validNorm(n FaceNorm) bool {
	return finite(n.Size, n.Eye) && n.Size > 0
}

// valid — окно, пропорции и рамка лица годятся для расчёта.
func (f faceSpot) valid() bool {
	if !finite(f.win.X, f.win.Y, f.win.W, f.win.H, f.aspect, f.box.X, f.box.Y, f.box.W, f.box.H, f.eye.X, f.eye.Y) {
		return false
	}
	return f.win.W > 0 && f.win.H > 0 && f.aspect > 0 && f.box.H > 0
}

// probe — высота лица на листе и точка лица (центр рамки по X, линия глаз по Y), мм, при кадре c.
func (f faceSpot) probe(c Crop) (h, x, y float64) {
	img := PlaceImageIn(f.win, f.aspect, c, f.o)
	q := quarterOfR(c.R)
	// Точка в кадре после четвертей → в осях файла (обратный поворот) → зеркало в осях файла.
	pa, pb := unturn(q, f.box.X+f.box.W/2, f.eye.Y)
	if f.flipH {
		pa = 1 - pa
	}
	if f.flipV {
		pb = 1 - pb
	}
	cx, cy := img.X+img.W/2, img.Y+img.H/2
	sn, cs := sinCosDeg(c.R)
	dx, dy := rotXY(sn, cs, img.X+pa*img.W-cx, img.Y+pb*img.H-cy)
	// Вертикаль листа при нечётных четвертях идёт вдоль ширины файла.
	side := img.H
	if q%2 == 1 {
		side = img.W
	}
	return f.box.H * side, cx + dx, cy + dy
}

// miss — невязка кадра c против нормы n.
func (f faceSpot) miss(c Crop, n FaceNorm) FaceMiss {
	h, x, y := f.probe(c)
	return FaceMiss{
		Size: h/(n.Size*f.win.H) - 1,
		DX:   x - (f.win.X + f.win.W/2),
		DY:   y - (f.win.Y + n.Eye*f.win.H),
	}
}

// quarterOfR — четверти по часовой (0…3) поворота r: ближайшая к r/90, наклон ±44,9 ничьей не даёт.
func quarterOfR(r float64) int {
	q := int(math.Round(math.Mod(r, 360) / 90))
	return ((q % 4) + 4) % 4
}

// unturn — точка (u, v) кадра, повёрнутого на q четвертей по часовой, в долях исходного кадра.
// Одна четверть по часовой: (x, y) → (1−y, x), обратная: (u, v) → (v, 1−u).
func unturn(q int, u, v float64) (float64, float64) {
	for range q {
		u, v = v, 1-u
	}
	return u, v
}

// faceRound — до 0.001, как model.ClampFrame; минус ноль не оставляет.
func faceRound(v float64) float64 {
	v = math.Round(v*1000) / 1000
	if v == 0 {
		return 0
	}
	return v
}
