package render

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/disintegration/imaging"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
	"golang.org/x/image/vector"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Повёрнутое на холсте (design-rotation §3). Путь 1 (¼ оборота) — prepare в render.go.
// Путь 2 — наклон фото при прямой рамке, закрывающей фото: Transform прямо в ячейку
// холста, клип — сама ячейка. Путь 3 — поворот места или край фото внутри рамки
// (contain с наклоном): Transform во временный RGBA полосами и DrawMask со сглаженной
// маской «рамка ∩ фото». Options.DstMask не берём: он уводит x/image/draw на медленный путь.

type pt struct{ x, y float64 }

// sincosDeg — синус и косинус угла в градусах; у кратных 90° — точные 0 и ±1, иначе
// ¼ оборота дала бы хвост 1e-16 и ячейка сдвинулась бы на пиксель.
func sincosDeg(deg float64) (s, c float64) {
	if q, ok := quarterOf(deg); ok {
		return [4]float64{0, 1, 0, -1}[q], [4]float64{1, 0, -1, 0}[q]
	}
	return math.Sincos(deg * math.Pi / 180)
}

// quarterOf — число четвертей по часовой 0…3, если угол кратен 90°.
func quarterOf(deg float64) (int, bool) {
	r := deg / 90
	n := math.Round(r)
	if math.IsNaN(r) || math.IsInf(r, 0) || math.Abs(r-n) > 1e-9 {
		return 0, false
	}
	return int(math.Mod(math.Mod(n, 4)+4, 4)), true
}

func center(r layout.Rect) (float64, float64) { return r.X + r.W/2, r.Y + r.H/2 }

// hasPivot — у фото задана рамка места, вокруг центра которой поворот (079).
func hasPivot(ph Photo) bool { return ph.Pivot.W > 0 && ph.Pivot.H > 0 }

// pivotOf — центр поворота места: рамка места (Pivot) или центр Cell.
func pivotOf(ph Photo) (float64, float64) {
	if hasPivot(ph) {
		return center(ph.Pivot)
	}
	return center(ph.Cell)
}

// turnedCell — Cell после поворота на Rot вокруг центра поворота: прямоугольник тех же размеров,
// центр которого сдвинут; фигуры строятся вокруг его центра. Без Pivot — сам Cell, дословно.
func turnedCell(ph Photo) layout.Rect {
	if !hasPivot(ph) {
		return ph.Cell
	}
	px, py := center(ph.Pivot)
	cx, cy := center(ph.Cell)
	s, c := sincosDeg(ph.Rot)
	dx, dy := cx-px, cy-py
	return centered(px+dx*c-dy*s, py+dx*s+dy*c, ph.Cell.W, ph.Cell.H)
}

func centered(cx, cy, w, h float64) layout.Rect {
	return layout.Rect{X: cx - w/2, Y: cy - h/2, W: w, H: h}
}

// fold переносит поворот места, кратный 90°, в поворот фото: ячейка снова прямая (при
// нечётной четверти стороны меняются), центр фото поворачивается вокруг центра ячейки (или
// рамки места, Pivot: окно рамочки со смещённым центром). Так место под 90° идёт путём 1
// (Lanczos, без маски), а под 90° с наклоном — путём 2.
func fold(ph Photo) Photo {
	q, ok := quarterOf(ph.Rot)
	if !ok {
		return ph
	}
	if q == 0 {
		ph.Rot = 0 // Img не пересчитываем: иной порядок сложений сдвинул бы px() прежних альбомов
		ph.Pivot = layout.Rect{}
		return ph
	}
	cx, cy := pivotOf(ph)
	ix, iy := center(ph.Img)
	s, c := sincosDeg(ph.Rot)
	dx, dy := ix-cx, iy-cy
	if hasPivot(ph) {
		turned := turnedCell(ph)
		if q%2 == 1 {
			turned = centered(turned.X+turned.W/2, turned.Y+turned.H/2, turned.H, turned.W)
		}
		ph.Cell = turned
	} else if q%2 == 1 {
		ph.Cell = layout.Rect{X: cx - ph.Cell.H/2, Y: cy - ph.Cell.W/2, W: ph.Cell.H, H: ph.Cell.W}
	}
	ph.Pivot = layout.Rect{}
	ph.Img.X, ph.Img.Y = cx+dx*c-dy*s-ph.Img.W/2, cy+dx*s+dy*c-ph.Img.H/2
	ph.ImgRot += ph.Rot
	ph.Rot = 0
	return ph
}

// rotGeom — точка фото (u, v), мм от его левого верхнего угла после зеркала, → пиксели
// холста: P = k·(A·(u, v) + t), A — поворот на Rot + ImgRot. Это M из design-rotation §3.
type rotGeom struct {
	s, c, tx, ty, k float64
}

func newGeom(ph Photo, k float64) rotGeom {
	sr, cr := sincosDeg(ph.Rot)
	s, c := sincosDeg(ph.Rot + ph.ImgRot)
	cx, cy := pivotOf(ph)
	ix, iy := center(ph.Img)
	dx, dy := ix-cx, iy-cy
	ox, oy := cx+dx*cr-dy*sr, cy+dx*sr+dy*cr // центр фото после поворота места
	hw, hh := ph.Img.W/2, ph.Img.H/2
	return rotGeom{s: s, c: c, tx: ox - (hw*c - hh*s), ty: oy - (hw*s + hh*c), k: k}
}

func (g rotGeom) fwd(u, v float64) pt {
	return pt{g.k * (g.c*u - g.s*v + g.tx), g.k * (g.s*u + g.c*v + g.ty)}
}

func (g rotGeom) inv(p pt) (u, v float64) {
	x, y := p.x/g.k-g.tx, p.y/g.k-g.ty
	return g.c*x + g.s*y, -g.s*x + g.c*y
}

// framePoly — рамка (ячейка, повёрнутая на rot вокруг центра) в пикселях холста.
func framePoly(cell layout.Rect, rot, k float64) []pt {
	s, c := sincosDeg(rot)
	cx, cy := center(cell)
	hw, hh := cell.W/2, cell.H/2
	out := make([]pt, 4)
	for i, d := range [4][2]float64{{-hw, -hh}, {hw, -hh}, {hw, hh}, {-hw, hh}} {
		out[i] = pt{k * (cx + d[0]*c - d[1]*s), k * (cy + d[0]*s + d[1]*c)}
	}
	return out
}

func rectPoly(r image.Rectangle) []pt {
	x0, y0, x1, y1 := float64(r.Min.X), float64(r.Min.Y), float64(r.Max.X), float64(r.Max.Y)
	return []pt{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
}

// orient — выпуклый многоугольник с положительной площадью: внутри — где cross ≥ 0.
func orient(p []pt) []pt {
	var a float64
	for i := range p {
		q := p[(i+1)%len(p)]
		a += p[i].x*q.y - q.x*p[i].y
	}
	if a >= 0 {
		return p
	}
	r := make([]pt, len(p))
	for i := range p {
		r[i] = p[len(p)-1-i]
	}
	return r
}

// inside — точка внутри выпуклого clip с допуском eps пикселей.
func inside(clip []pt, p pt, eps float64) bool {
	clip = orient(clip)
	for i := range clip {
		a, b := clip[i], clip[(i+1)%len(clip)]
		l := math.Hypot(b.x-a.x, b.y-a.y)
		if (b.x-a.x)*(p.y-a.y)-(b.y-a.y)*(p.x-a.x) < -eps*l {
			return false
		}
	}
	return true
}

// clipPoly — часть выпуклого subj внутри выпуклого clip (Сазерленд — Ходжмен).
func clipPoly(subj, clip []pt) []pt {
	clip = orient(clip)
	out := subj
	for i := range clip {
		if len(out) == 0 {
			break
		}
		a, b := clip[i], clip[(i+1)%len(clip)]
		side := func(p pt) float64 { return (b.x-a.x)*(p.y-a.y) - (b.y-a.y)*(p.x-a.x) }
		in := out
		out = make([]pt, 0, len(in)+1)
		for j := range in {
			p, q := in[j], in[(j+1)%len(in)]
			sp, sq := side(p), side(q)
			if sp >= 0 {
				out = append(out, p)
			}
			if (sp >= 0) != (sq >= 0) {
				t := sp / (sp - sq)
				out = append(out, pt{p.x + t*(q.x-p.x), p.y + t*(q.y-p.y)})
			}
		}
	}
	return out
}

// rotJob — подготовленное фото путей 2 и 3: окно оригинала im (уже без бюджета декода)
// и всё, чтобы положить его на холст в свою очередь.
type rotJob struct {
	im   *image.NRGBA
	m    f64.Aff3        // пиксель im → пиксель холста
	cell image.Rectangle // путь 2: ячейка в px (∩ клип); пусто — путь 3
	poly []pt            // путь 3: маска в px холста
	bb   image.Rectangle // путь 3: её габарит ∩ клип
	// Путь 3 с формой места (044): маска полосы умножается на покрытие формы рамки frame,
	// повёрнутой на rot; k — px на мм.
	shape geom.Outline
	frame layout.Rect
	rot   float64
	k     float64
	win   *winSampler // маска окна рамочки (079); с ней всегда путь 3
}

// prepareRot готовит фото путей 2 и 3; clip — куда можно рисовать (холст или область фона).
// nil без ошибки — рисовать нечего.
func prepareRot(ph Photo, dpi int, clip image.Rectangle) (*rotJob, error) {
	if err := checkRotPhoto(ph); err != nil {
		return nil, err
	}
	k := float64(dpi) / 25.4
	if ph.Img.W*k < 1e-3 || ph.Img.H*k < 1e-3 {
		// Фото меньше тысячной пикселя не видно, а Transform при таком уменьшении выделял
		// бы веса ~2/масштаб.
		return nil, nil
	}
	g := newGeom(ph, k)
	job, poly, covers := rotShape(ph, dpi, clip, g, k)
	if job == nil {
		return nil, nil
	}
	u0, v0, u1, v1, ok := visibleSpan(ph, g, k, poly)
	if !ok {
		return nil, nil
	}
	src, err := cropSource(ph, k, covers, u0, v0, u1, v1)
	if err != nil || src == nil {
		return nil, err
	}
	job.im = src.im
	job.m = rotMatrix(ph, g, k, src)
	return job, nil
}

// checkRotPhoto отсекает фото без кадра и с нечисловой геометрией (файл мог быть подделан).
func checkRotPhoto(ph Photo) error {
	if ph.Img.W <= 0 || ph.Img.H <= 0 {
		return fmt.Errorf("%s: у фото нет кадра", baseName(ph.Path))
	}
	for _, v := range []float64{ph.Rot, ph.ImgRot, ph.Pivot.X, ph.Pivot.Y, ph.Pivot.W, ph.Pivot.H, ph.Cell.X, ph.Cell.Y, ph.Cell.W, ph.Cell.H, ph.Img.X, ph.Img.Y, ph.Img.W, ph.Img.H, ph.Outline.Radius} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("%s: некорректная геометрия фото", baseName(ph.Path))
		}
	}
	return nil
}

// rotShape выбирает путь 2 или 3 и заполняет в job область рисования (cell или poly+bb).
// poly — маска в px холста; covers — рамка целиком внутри фото. job == nil — рисовать нечего.
func rotShape(ph Photo, dpi int, clip image.Rectangle, g rotGeom, k float64) (job *rotJob, poly []pt, covers bool) {
	W, H := ph.Img.W, ph.Img.H
	photo := []pt{g.fwd(0, 0), g.fwd(W, 0), g.fwd(W, H), g.fwd(0, H)}
	turned := turnedCell(ph)
	frame := framePoly(turned, ph.Rot, k)
	covers = true
	for _, p := range frame {
		if !inside(photo, p, 1e-3) {
			covers = false
			break
		}
	}
	job = &rotJob{shape: ph.Outline, frame: turned, rot: ph.Rot, k: k}
	if ph.Rot == 0 && covers && !hasShape(ph.Outline) && ph.Win == nil {
		// Путь 2: клип — ячейка в px, как у пути 1; окно — под её пиксели.
		job.cell = image.Rect(px(ph.Cell.X, dpi), px(ph.Cell.Y, dpi), px(ph.Cell.X+ph.Cell.W, dpi), px(ph.Cell.Y+ph.Cell.H, dpi)).Intersect(clip)
		if job.cell.Empty() {
			return nil, nil, covers
		}
		return job, rectPoly(job.cell), covers
	}
	poly = frame
	if !covers {
		poly = clipPoly(frame, photo)
	}
	poly = clipPoly(poly, rectPoly(clip))
	if len(poly) < 3 {
		return nil, nil, covers
	}
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range poly {
		x0, y0, x1, y1 = min(x0, p.x), min(y0, p.y), max(x1, p.x), max(y1, p.y)
	}
	job.bb = image.Rect(int(math.Floor(x0)), int(math.Floor(y0)), int(math.Ceil(x1)), int(math.Ceil(y1))).Intersect(clip)
	if job.bb.Empty() {
		return nil, nil, covers
	}
	job.poly = poly
	return job, poly, covers
}

// visibleSpan — видимая часть poly в осях файла (мм, уже с зеркалом) с запасом в 2 px
// печати: пиксели у края клипа не должны попасть за окно. ok = false — окно пусто.
func visibleSpan(ph Photo, g rotGeom, k float64, poly []pt) (u0, v0, u1, v1 float64, ok bool) {
	W, H := ph.Img.W, ph.Img.H
	u0, v0, u1, v1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range poly {
		u, v := g.inv(p)
		u0, v0, u1, v1 = min(u0, u), min(v0, v), max(u1, u), max(v1, v)
	}
	m := 2 / k
	u0, v0, u1, v1 = max(u0-m, 0), max(v0-m, 0), min(u1+m, W), min(v1+m, H)
	if u1 <= u0 || v1 <= v0 {
		return 0, 0, 0, 0, false
	}
	if ph.FlipH {
		u0, u1 = W-u1, W-u0
	}
	if ph.FlipV {
		v0, v1 = H-v1, H-v0
	}
	return u0, v0, u1, v1, true
}

// rotSrc — вырезанное окно оригинала: сама картинка, где оно лежит в файле (win, px),
// размер файла (sw, sh) и поле padEdge.
type rotSrc struct {
	im     *image.NRGBA
	win    image.Rectangle
	sw, sh float64
	pad    int
}

// cropSource читает оригинал и вырезает окно [u0,u1]×[v0,v1] (мм в осях файла).
// nil без ошибки — окно пусто.
func cropSource(ph Photo, k float64, covers bool, u0, v0, u1, v1 float64) (*rotSrc, error) {
	W, H := ph.Img.W, ph.Img.H
	src, release, err := imgfile.OpenOriented(ph.Path)
	if err != nil {
		return nil, photoErr(ph.Path, err)
	}
	defer release()
	b := src.Bounds()
	sw, sh := float64(b.Dx()), float64(b.Dy())
	win := image.Rect(int(math.Floor(u0/W*sw))-1, int(math.Floor(v0/H*sh))-1, int(math.Ceil(u1/W*sw))+1, int(math.Ceil(v1/H*sh))+1).
		Intersect(image.Rect(0, 0, b.Dx(), b.Dy()))
	if win.Empty() {
		return nil, nil
	}
	// Окно крупнее печати больше чем вдвое — Lanczos до размера печати. Только уменьшение:
	// размеры кадра из .salbum могут быть подделаны, растягивание просило бы гигабайты.
	// Копия окна отпускает бюджет декода до своей очереди на холст.
	printW := max(1, int(math.Round(float64(win.Dx())/sw*W*k)))
	printH := max(1, int(math.Round(float64(win.Dy())/sh*H*k)))
	crop := imaging.Crop(src, win.Add(b.Min))
	if win.Dx() > 2*printW || win.Dy() > 2*printH {
		crop = imaging.Resize(crop, min(printW, win.Dx()), min(printH, win.Dy()), imaging.Lanczos)
	}
	pad := 0
	if !covers {
		// Край фото внутри маски: за край продолжаем крайние пиксели, иначе сглаженная
		// маска легла бы на прозрачное и край вышел бы ступенькой.
		pad = 2
		crop = padEdge(crop, pad)
	}
	applyTone(crop, ph.Tone) // пути 2 и 3: до поворота, как путь 1 — после уменьшения
	fadeAlpha(crop, ph.Fade) // пути 2 и 3: draw выберет Over по альфе
	return &rotSrc{im: crop, win: win, sw: sw, sh: sh, pad: pad}, nil
}

// rotMatrix — аффинное преобразование «пиксель окна → пиксель холста».
func rotMatrix(ph Photo, g rotGeom, k float64, s *rotSrc) f64.Aff3 {
	W, H := ph.Img.W, ph.Img.H
	// Пиксель окна x ∈ [0, n] → u в осях файла, затем зеркало: u = au + bu·x.
	axis := func(lo, n int, full, size float64, flip bool) (a, step float64) {
		a, step = float64(lo)/full*size, float64(n)/full*size
		if flip {
			a, step = size-a, -step
		}
		return a, step
	}
	sb := s.im.Bounds()
	au, bu := axis(s.win.Min.X, s.win.Dx(), s.sw, W, ph.FlipH)
	av, bv := axis(s.win.Min.Y, s.win.Dy(), s.sh, H, ph.FlipV)
	bu /= float64(sb.Dx() - 2*s.pad)
	bv /= float64(sb.Dy() - 2*s.pad)
	au -= bu * float64(sb.Min.X+s.pad)
	av -= bv * float64(sb.Min.Y+s.pad)
	return f64.Aff3{
		k * g.c * bu, -k * g.s * bv, k * (g.c*au - g.s*av + g.tx),
		k * g.s * bu, k * g.c * bv, k * (g.s*au + g.c*av + g.ty),
	}
}

// padEdge — копия im с полем n пикселей из крайних; внутренняя часть остаётся на (n, n).
func padEdge(im *image.NRGBA, n int) *image.NRGBA {
	b := im.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w+2*n, h+2*n))
	for y := range h + 2*n {
		sy := min(max(y-n, 0), h-1)
		row := im.Pix[sy*im.Stride : sy*im.Stride+4*w]
		dst := out.Pix[y*out.Stride : (y+1)*out.Stride]
		copy(dst[4*n:], row)
		for x := range n {
			copy(dst[4*x:4*x+4], row[:4])
			copy(dst[4*(n+w+x):4*(n+w+x)+4], row[4*(w-1):])
		}
	}
	return out
}

// stripPx — пикселей в полосе: временные буферы пути 3 (RGBA, маска, растр — ~9 байт
// на пиксель) на всех потоках вместе не больше ~20 МБ, а не целая страница.
const stripPx = 1 << 18

// stripRows — высота полосы в строках. Не зависит от числа потоков: маска vector строится
// в координатах полосы, и от границ полос зависят хеши — результат не должен меняться
// с GOMAXPROCS.
const stripRows = 32

// strips делит r на полосы высотой не больше stripRows и stripPx: Transform однопоточный,
// а полосы независимы (каждая пишет свои строки холста).
func strips(r image.Rectangle) []image.Rectangle {
	rows := max(1, min(stripPx/max(r.Dx(), 1), stripRows))
	var out []image.Rectangle
	for y := r.Min.Y; y < r.Max.Y; y += rows {
		out = append(out, image.Rect(r.Min.X, y, r.Max.X, min(y+rows, r.Max.Y)))
	}
	return out
}

// inParallel вызывает fn для каждой полосы; worker — номер потока (свои буферы у каждого).
// Паника в потоке — ошибка: recover в paintSafe ловит только свою горутину, а паника
// воркера без recover уронила бы весь процесс. После сбоя новые полосы не берутся.
func inParallel(parts []image.Rectangle, workers int, fn func(worker int, r image.Rectangle)) error {
	var next atomic.Int64
	var wg sync.WaitGroup
	var failed atomic.Bool
	var err error
	for w := range min(workers, len(parts)) {
		wg.Go(func() {
			defer func() {
				if p := recover(); p != nil && failed.CompareAndSwap(false, true) {
					err = fmt.Errorf("%v", p) // пишет один поток, читаем после Wait
				}
			}()
			for i := int(next.Add(1)) - 1; i < len(parts) && !failed.Load(); i = int(next.Add(1)) - 1 {
				fn(w, parts[i])
			}
		})
	}
	wg.Wait()
	return err
}

// draw кладёт фото на холст.
func (j *rotJob) draw(canvas *image.RGBA) error {
	workers := min(runtime.GOMAXPROCS(0), 8)
	if !j.cell.Empty() {
		op := draw.Over
		if j.im.Opaque() {
			op = draw.Src
		}
		return inParallel(strips(j.cell), workers, func(_ int, r image.Rectangle) {
			dst, _ := canvas.SubImage(r).(*image.RGBA)
			xdraw.BiLinear.Transform(dst, j.m, j.im, j.im.Bounds(), op, nil)
		})
	}
	parts := strips(j.bb)
	w, rows := j.bb.Dx(), parts[0].Dy()
	type bufs struct {
		pix, alpha, form []uint8
		z                vector.Rasterizer
	}
	per := make([]*bufs, min(workers, len(parts)))
	return inParallel(parts, workers, func(wi int, r image.Rectangle) {
		b := per[wi]
		if b == nil {
			b = &bufs{pix: make([]uint8, 4*w*rows), alpha: make([]uint8, w*rows)}
			per[wi] = b
		}
		n := r.Dx() * r.Dy()
		tmp := &image.RGBA{Pix: b.pix[:4*n], Stride: 4 * w, Rect: r}
		clear(tmp.Pix)
		xdraw.BiLinear.Transform(tmp, j.m, j.im, j.im.Bounds(), draw.Src, nil)
		// Маска — с нуля: так vector пишет покрытие прямо в Pix.
		mask := &image.Alpha{Pix: b.alpha[:n], Stride: w, Rect: image.Rect(0, 0, w, r.Dy())}
		b.z.Reset(w, r.Dy())
		b.z.DrawOp = draw.Src // маска перезаписывается целиком, чистить буфер не нужно
		for i, p := range j.poly {
			x, y := float32(p.x-float64(r.Min.X)), float32(p.y-float64(r.Min.Y))
			if i == 0 {
				b.z.MoveTo(x, y)
			} else {
				b.z.LineTo(x, y)
			}
		}
		b.z.ClosePath()
		b.z.Draw(mask, mask.Rect, image.Opaque, image.Point{})
		if hasShape(j.shape) {
			if b.form == nil {
				b.form = make([]uint8, w*rows)
			}
			form := &image.Alpha{Pix: b.form[:n], Stride: w, Rect: mask.Rect}
			drawShape(&b.z, form, r.Min, j.frame, j.rot, j.shape, j.k)
			mulMask(mask.Pix, form.Pix)
		}
		if j.win != nil {
			j.win.mulInto(mask.Pix, w, r)
		}
		draw.DrawMask(canvas, r, tmp, r.Min, mask, image.Point{}, draw.Over)
	})
}

// backdropPhoto — фон как фото с поворотом вокруг центра рамки (повёрнутую вокруг угла
// рамку сдвигаем так, чтобы центр стоял там же); clip — область в px. ok = false — фона не видно.
// Наклон фото (ImgRot, 045) — как у места: при прямой рамке путь 2 (ячейка — вся рамка, клип —
// область), при повёрнутой fold складывает углы, и идёт путь 2 или 3.
func backdropPhoto(b Backdrop, dpi int) (ph Photo, clip image.Rectangle, ok bool) {
	if b.Rot == 0 && b.ImgRot == 0 {
		ph, ok = b.photo() // прежний путь дословно
		return ph, image.Rectangle{}, ok
	}
	s, c := sincosDeg(b.Rot)
	hw, hh := b.Box.W/2, b.Box.H/2
	cx, cy := b.Box.X+hw*c-hh*s, b.Box.Y+hw*s+hh*c
	cell := layout.Rect{X: cx - hw, Y: cy - hh, W: b.Box.W, H: b.Box.H}
	img := b.Img
	img.X += cell.X
	img.Y += cell.Y
	ph = fold(Photo{Cell: cell, Img: img, Rot: b.Rot, ImgRot: b.ImgRot, Path: b.Path, FlipH: b.FlipH, FlipV: b.FlipV, Fade: b.Fade})
	if _, quarter := quarterOf(ph.ImgRot); ph.Rot == 0 && quarter {
		// Рамка под ¼ оборота снова прямая, наклона нет (путь 1): клип — она сама в пределах области.
		ph.Cell = intersect(ph.Cell, b.Area)
		return ph, image.Rectangle{}, ph.Cell.W > 0 && ph.Cell.H > 0
	}
	// Наклон или рамка под углом: ячейка — вся рамка (от её центра считается поворот),
	// рисуется только в области.
	clip = image.Rect(px(b.Area.X, dpi), px(b.Area.Y, dpi), px(b.Area.X+b.Area.W, dpi), px(b.Area.Y+b.Area.H, dpi))
	return ph, clip, !clip.Empty()
}
