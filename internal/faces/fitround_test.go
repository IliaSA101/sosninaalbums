package faces

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"sync"
	"testing"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Круг «подгон → повторный детект» (спека 130 G7, design §10): лицо ищется на фото, geom.FaceCrop
// ставит кадр, окно места рисует render как экспорт, детектор ищет лицо в готовом окне, и доли
// лица сверяются с нормой. Тест живёт в faces: у пакета нет правила depguard, а geom, layout,
// render и imgfile faces не импортируют (цикла нет). Решение 0.27-130-60.

const (
	fitDPI    = 300 // как экспорт
	fitPageW  = 70.0
	fitPageH  = 90.0
	tolFrame  = 0.1  // на каждый кадр: доля окна, ловит зеркало, поворот и масштаб
	tolBoxMed = 0.02 // медиана по местам без флага
	tolBoxP90 = 0.05
	tolBoxMax = 0.08
	tolEyeP90 = 0.04
	tolEyeMax = 0.06
	maxLost   = 0.02 // доля мест без флага, где в окне нет лица даже на пороге 0,5 (решение 0.27-130-62)
	maxWeak   = 0.15 // доля мест без флага, где лицо есть только на пороге 0,5, а не 0,8
	fitMinOK  = 20   // без этого круг пуст: «все в допусках» ничего не значит
	statMinN  = 10   // медиана и доли 90 % считаются от такого числа мест, меньше — только предельные допуски
)

// fitWin — окно места на листе: мм левой страницы.
type fitWin struct {
	Name string
	Rect layout.Rect
}

var (
	// winMarina — место виньетки Марины (спека: 47,5 × 63,3 мм).
	winMarina = fitWin{"47,5×63,3", layout.Rect{X: 10, Y: 10, W: 47.5, H: 63.3}}
	winSquare = fitWin{"55×55", layout.Rect{X: 8, Y: 8, W: 55, H: 55}}
)

// fitVariant — четверти по часовой и зеркало фото: детектор видит картинку после Turn без зеркала,
// зеркало идёт в FaceCrop и в рендер.
type fitVariant struct {
	Q            int
	FlipH, FlipV bool
}

// fitVariants — по кругу для проходов с поворотом и зеркалом. Зеркало в проекте идёт в осях файла до
// поворота, поэтому на экране оно горизонтальное только при FlipH на чётных четвертях и FlipV на
// нечётных; остальные сочетания ставят лицо вверх ногами, а перевёрнутое лицо детектор не ищет
// (его и не «подгоняют»: решение 0.27-130-61).
var fitVariants = []fitVariant{
	{0, true, false}, {1, false, false}, {1, false, true}, {2, false, false}, {2, true, false}, {3, false, false}, {3, false, true},
}

// fitRes — итог круга по одному фото.
type fitRes struct {
	Name string
	Skip string // причина, почему круг не считался (нет лица)
	Many bool   // на фото несколько лиц: взято ближайшее к центру из похожих
	Err  error
	Win  fitWin
	Var  fitVariant
	Norm geom.FaceNorm
	Crop geom.Crop
	Miss geom.FaceMiss
	Flag geom.FaceFlag
	Got  *Face       // лицо в окне после подгона; nil — не найдено
	Weak bool        // Got == nil, но сеть с порогом 0,5 видит лицо там, где его поставил подгон
	Pic  *image.RGBA // окно места (для листа)
}

// pickFace — выбор лица как в design §3: наибольшая рамка; если вторая ≥ 70 % первой, это
// «несколько лиц» (many) и берётся ближайшее к центру картинки.
func pickFace(fs []Face) (f Face, many, ok bool) {
	if len(fs) == 0 {
		return Face{}, false, false
	}
	s := slices.Clone(fs)
	sort.SliceStable(s, func(i, j int) bool { return s[i].Box.H > s[j].Box.H })
	f = s[0]
	if len(s) > 1 && s[1].Box.H >= 0.7*s[0].Box.H {
		many = true
		dist := func(f Face) float64 {
			return math.Hypot(f.Box.X+f.Box.W/2-0.5, f.Box.Y+f.Box.H/2-0.5)
		}
		for _, c := range s {
			if c.Box.H >= 0.7*s[0].Box.H && dist(c) < dist(f) {
				f = c
			}
		}
	}
	return f, many, true
}

func px(mm float64) int { return int(math.Round(mm / 25.4 * fitDPI)) }

// turnedCopy — копия фото, повёрнутая на q четвертей против часовой, в tmp: в проекте такой
// файл (снимок «на боку») показывается с R = 90·q и встаёт прямо, а детектор видит Turn(файл, q).
// Без копии лицо на окне лежало бы боком, и повторный детект проверял бы не подгон.
func turnedCopy(img image.Image, name, tmp string, q int) (string, error) {
	rot := imaging.Clone(img)
	switch q {
	case 1:
		rot = imaging.Rotate90(img)
	case 2:
		rot = imaging.Rotate180(img)
	case 3:
		rot = imaging.Rotate270(img)
	}
	p := filepath.Join(tmp, fmt.Sprintf("q%d-%s", q, name))
	return p, imaging.Save(rot, p, imaging.JPEGQuality(92))
}

// openVariant открывает фото для варианта; при четвертях — повёрнутую копию (её путь возвращается:
// render рисует из файла).
func openVariant(path, tmp, name string, v fitVariant) (image.Image, string, func(), error) {
	img, release, err := imgfile.OpenOriented(path)
	if err != nil || v.Q == 0 {
		return img, path, release, err
	}
	cp, err := turnedCopy(img, name, tmp, v.Q)
	release()
	if err != nil {
		return nil, path, func() {}, err
	}
	img, release, err = imgfile.OpenOriented(cp)
	return img, cp, release, err
}

// fitOne — круг для одного фото; tmp — папка для повёрнутых копий.
func fitOne(path, tmp string, w fitWin, v fitVariant, n geom.FaceNorm) fitRes {
	r := fitRes{Name: filepath.Base(path), Win: w, Var: v, Norm: n}
	img, path, release, err := openVariant(path, tmp, r.Name, v)
	if err != nil {
		r.Err = err
		return r
	}
	defer release()
	b := img.Bounds()
	aspect := float64(b.Dx()) / float64(b.Dy())
	f, many, ok := pickFace(Detect(Turn(img, v.Q)))
	if !ok {
		r.Skip = "без лица"
		return r
	}
	r.Many = many
	box := layout.Rect{X: f.Box.X, Y: f.Box.Y, W: f.Box.W, H: f.Box.H}
	eye := f.EyeLine()
	cur := geom.Crop{S: 1, R: float64(90 * v.Q)}
	r.Crop, r.Miss = geom.FaceCrop(w.Rect, geom.Outline{}, aspect, cur, v.FlipH, v.FlipV, box, geom.Pt{X: eye.X, Y: eye.Y}, n)
	r.Flag = r.Miss.Flag()
	if r.Miss.Bad {
		r.Err = fmt.Errorf("FaceCrop: негодный вход %+v", box)
		return r
	}
	sp := render.Spread{
		Bg: [2]color.RGBA{{255, 255, 255, 255}, {255, 255, 255, 255}},
		Photos: []render.Photo{{
			Cell: w.Rect, Img: geom.PlaceImageIn(w.Rect, aspect, r.Crop, geom.Outline{}), Path: path,
			FlipH: v.FlipH, FlipV: v.FlipV, ImgRot: r.Crop.R,
		}},
	}
	canvas, err := render.Render(sp, render.Options{PageW: fitPageW, PageH: fitPageH, DPI: fitDPI})
	if err != nil {
		r.Err = err
		return r
	}
	rect := image.Rect(px(w.Rect.X), px(w.Rect.Y), px(w.Rect.X+w.Rect.W), px(w.Rect.Y+w.Rect.H))
	win := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	for y := range rect.Dy() {
		for x := range rect.Dx() {
			win.SetRGBA(x, y, canvas.RGBAAt(rect.Min.X+x, rect.Min.Y+y))
		}
	}
	r.Pic = win
	// Лицо в окне — ближайшее к тому месту, куда его поставил подгон: у групповых фото соседнее
	// лицо могло бы оказаться «ближе к центру», а проверяется геометрия, а не выбор лица.
	// Если у фото было несколько лиц и ожидаемого места никто не занял — лицо потеряно.
	_, we, wx := wantShare(r)
	found, ok := nearestFace(Detect(win), wx, we)
	if !ok || (r.Many && !nearBy(found, wx, we)) {
		r.Weak = weakNear(win, wx, we)
	}
	if ok {
		g := found
		if !r.Many || nearBy(g, wx, we) {
			r.Got = &g
		}
	}
	return r
}

// nearBy — середина глаз лица в пределах кадра ±0,1 от (cx, eye).
func nearBy(f Face, cx, eye float64) bool {
	_, ey, x := gotShare(f)
	return math.Hypot(x-cx, ey-eye) <= tolFrame
}

// weakNear — есть ли в окне лицо с оценкой от 0,5, у которого глаза в пределах кадра от (cx, eye):
// детектор на границе уверенности (профиль, закрытое лицо) не доказывает промах подгона, а лица
// там, куда подгон его поставил, нет совсем — доказывает.
func weakNear(img image.Image, cx, eye float64) bool {
	n, err := loadNet()
	if err != nil {
		return false
	}
	in, ok := prepareInput(img, inputLong)
	if !ok {
		return false
	}
	outs, err := n.run(in.d, in.H, in.W, 1, &memTrack{})
	if err != nil {
		return false
	}
	cs, err := decode(outs, in.W, in.H, 0.5)
	if err != nil {
		return false
	}
	for _, c := range cs {
		if f := c.face(in.w, in.h); nearBy(f, cx, eye) {
			return true
		}
	}
	return false
}

// nearestFace — лицо, у которого середина глаз (по X — центр рамки) ближе всего к (cx, eye).
func nearestFace(fs []Face, cx, eye float64) (best Face, ok bool) {
	bd := math.Inf(1)
	for _, f := range fs {
		_, ey, x := gotShare(f)
		if d := math.Hypot(x-cx, ey-eye); d < bd {
			best, bd, ok = f, d, true
		}
	}
	return best, ok
}

// fitAll гоняет круг по фото в нескольких горутинах (Detect безопасен для параллельных вызовов);
// порядок результатов — порядок путей.
func fitAll(paths []string, tmp string, w fitWin, variant func(i int) fitVariant, n geom.FaceNorm) []fitRes {
	res := make([]fitRes, len(paths))
	next := make(chan int)
	var wg sync.WaitGroup
	for range max(1, min(4, runtime.NumCPU()-1)) {
		wg.Go(func() {
			for i := range next {
				res[i] = fitOne(paths[i], tmp, w, variant(i), n)
			}
		})
	}
	for i := range paths {
		next <- i
	}
	close(next)
	wg.Wait()
	return res
}

// gotShare — доли лица, найденного в окне: высота рамки, линия глаз от верха, центр по X.
func gotShare(f Face) (size, eye, cx float64) {
	return f.Box.H, f.EyeLine().Y, f.Box.X + f.Box.W/2
}

// wantShare — доли, которые обещает невязка: норма плюс промах упора (мелкое, у края).
func wantShare(r fitRes) (size, eye, cx float64) {
	return r.Norm.Size * (1 + r.Miss.Size), r.Norm.Eye + r.Miss.DY/r.Win.Rect.H, 0.5 + r.Miss.DX/r.Win.Rect.W
}

// fitStats — цифры круга по местам без флага (отклонения от нормы, доли высоты окна).
type fitStats struct {
	N, Skipped, Flagged, Lost, Weak int
	Box, Eye, CX                    []float64 // со знаком
}

func collect(res []fitRes) fitStats {
	var s fitStats
	for _, r := range res {
		switch {
		case r.Skip != "":
			s.Skipped++
		case r.Flag != geom.FaceOK:
			s.Flagged++
		case r.Got == nil && r.Weak:
			s.Weak++
		case r.Got == nil:
			s.Lost++
		default:
			size, eye, cx := gotShare(*r.Got)
			s.N++
			s.Box = append(s.Box, size-r.Norm.Size)
			s.Eye = append(s.Eye, eye-r.Norm.Eye)
			s.CX = append(s.CX, cx-0.5)
		}
	}
	return s
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := slices.Clone(v)
	slices.Sort(s)
	return (s[(len(s)-1)/2] + s[len(s)/2]) / 2
}

func sigma(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	var m, q float64
	for _, x := range v {
		m += x
	}
	m /= float64(len(v))
	for _, x := range v {
		q += (x - m) * (x - m)
	}
	return math.Sqrt(q / float64(len(v)))
}

// within — доля значений в ±tol; maxAbs — наибольшее по модулю.
func within(v []float64, tol float64) float64 {
	if len(v) == 0 {
		return 1
	}
	n := 0
	for _, x := range v {
		if math.Abs(x) <= tol {
			n++
		}
	}
	return float64(n) / float64(len(v))
}

func maxAbs(v []float64) float64 {
	m := 0.0
	for _, x := range v {
		m = max(m, math.Abs(x))
	}
	return m
}

// checkRound проверяет круг: кадр каждого места ±0,1 от обещанного невязкой, точность — по
// местам без флага. Возвращает цифры для отчёта.
func checkRound(t *testing.T, label string, res []fitRes) fitStats {
	t.Helper()
	for _, r := range res {
		if r.Err != nil {
			t.Errorf("%s %s: %v", label, r.Name, r.Err)
			continue
		}
		if r.Skip != "" {
			continue
		}
		if r.Got == nil {
			continue // потеряно: счёт и предел доли — в checkAccuracy
		}
		gs, ge, gx := gotShare(*r.Got)
		ws, we, wx := wantShare(r)
		if math.Abs(gs-ws) > tolFrame || math.Abs(ge-we) > tolFrame || math.Abs(gx-wx) > tolFrame {
			t.Errorf("%s %s [%s %+v, флаг %d]: в окне рамка %.3f глаза %.3f центр %.3f, ждали %.3f / %.3f / %.3f (±%.1f)",
				label, r.Name, r.Win.Name, r.Var, r.Flag, gs, ge, gx, ws, we, wx, tolFrame)
		}
	}
	return logStats(t, label, res)
}

// logStats считает цифры круга и пишет их в журнал теста.
func logStats(t *testing.T, label string, res []fitRes) fitStats {
	t.Helper()
	s := collect(res)
	t.Logf("%s: мест без флага %d, с флагом %d, без лица %d, лицо на пороге оценки %d, потеряно %d", label, s.N, s.Flagged, s.Skipped, s.Weak, s.Lost)
	t.Logf("%s: рамка медиана %+.3f σ %.3f в ±%.2f: %.0f %% макс %.3f; глаза медиана %+.3f σ %.3f в ±%.2f: %.0f %% макс %.3f; центр X медиана %+.3f σ %.3f",
		label, median(s.Box), sigma(s.Box), tolBoxP90, 100*within(s.Box, tolBoxP90), maxAbs(s.Box),
		median(s.Eye), sigma(s.Eye), tolEyeP90, 100*within(s.Eye, tolEyeP90), maxAbs(s.Eye), median(s.CX), sigma(s.CX))
	return s
}

// checkAccuracy — допуски design §10 по местам без флага.
func checkAccuracy(t *testing.T, label string, s fitStats, minN int) {
	t.Helper()
	tot := float64(s.N + s.Weak + s.Lost)
	if float64(s.Lost) > maxLost*tot {
		t.Errorf("%s: лица нет в окне даже на пороге 0,5 у %d из %.0f мест без флага (предел %.0f %%): подгон ставит лицо не туда", label, s.Lost, tot, 100*maxLost)
	}
	if float64(s.Weak) > maxWeak*tot {
		t.Errorf("%s: лицо на пороге оценки у %d из %.0f мест без флага (предел %.0f %%)", label, s.Weak, tot, 100*maxWeak)
	}
	if s.N < minN {
		t.Fatalf("%s: мест без флага %d (нужно хотя бы %d) — круг пуст", label, s.N, minN)
	}
	if s.N >= statMinN { // на двух-трёх местах медиана и «90 %» — шум, а не точность
		if m := median(s.Box); math.Abs(m) > tolBoxMed {
			t.Errorf("%s: медиана рамки %+.3f вне ±%.2f", label, m, tolBoxMed)
		}
		if m := median(s.Eye); math.Abs(m) > tolBoxMed {
			t.Errorf("%s: медиана глаз %+.3f вне ±%.2f", label, m, tolBoxMed)
		}
		if p := within(s.Box, tolBoxP90); p < 0.9 {
			t.Errorf("%s: рамка в ±%.2f у %.0f %% мест, нужно 90 %%", label, tolBoxP90, 100*p)
		}
		if p := within(s.Eye, tolEyeP90); p < 0.9 {
			t.Errorf("%s: глаза в ±%.2f у %.0f %% мест, нужно 90 %%", label, tolEyeP90, 100*p)
		}
	}
	if m := maxAbs(s.Box); m > tolBoxMax {
		t.Errorf("%s: рамка уходит на %.3f, предел %.2f", label, m, tolBoxMax)
	}
	if m := maxAbs(s.Eye); m > tolEyeMax {
		t.Errorf("%s: глаза уходят на %.3f, предел %.2f", label, m, tolEyeMax)
	}
}

// photoList — jpg папки по имени; на быстром уровне — каждый четвёртый.
func photoList(t *testing.T, dir string) []string {
	t.Helper()
	all, err := filepath.Glob(filepath.Join(dir, "*.jpg"))
	if err != nil || len(all) == 0 {
		t.Fatalf("в %s нет jpg (%v)", dir, err)
	}
	slices.Sort(all)
	if !testkit.Lite() {
		return all
	}
	var out []string
	for i := 0; i < len(all); i += 4 {
		out = append(out, all[i])
	}
	return out
}

// roundTrip — два прохода по папке: место Марины без поворота и квадрат с четвертями и зеркалом.
func roundTrip(t *testing.T, dir string) (plain, turned []fitRes) {
	paths := photoList(t, dir)
	tmp := testkit.TempDir(t)
	plain = fitAll(paths, tmp, winMarina, func(int) fitVariant { return fitVariant{} }, geom.DefaultFaceNorm)
	turned = fitAll(paths, tmp, winSquare, func(i int) fitVariant { return fitVariants[i%len(fitVariants)] }, geom.DefaultFaceNorm)
	return plain, turned
}

// TestFitRoundTripStock: сток .testphotos/stock, подгон по норме и повторный детект окна. Нет фото — пропуск.
func TestFitRoundTripStock(t *testing.T) {
	dir := photosDir("stock")
	if dir == "" {
		t.Skip("нет .testphotos/stock (фото вне git, docs/QA.md «Подготовка»)")
	}
	skipHeavy(t)
	plain, turned := roundTrip(t, dir)
	checkRound(t, "сток, место 47,5×63,3", plain)
	checkRound(t, "сток, квадрат, четверти и зеркало", turned)
	// Места — фото × окно: оба прохода одним рядом, иначе у второго мало мест для «90 %».
	all := logStats(t, "сток, всего", append(plain, turned...))
	minN := fitMinOK
	if testkit.Lite() {
		minN = statMinN // на быстром уровне берётся каждое четвёртое фото
	}
	checkAccuracy(t, "сток", all, minN)
}

// TestFitRoundTripPortraits: те же допуски на 10 детских портретах (.testphotos/portraits).
// Мест мало, поэтому медиана и доли считаются по одному проходу, число дошедших до нормы — в журнал.
func TestFitRoundTripPortraits(t *testing.T) {
	dir := photosDir("portraits")
	if dir == "" {
		t.Skip("нет .testphotos/portraits (docs/qa/stock-photos.md)")
	}
	skipHeavy(t)
	paths := photoList(t, dir)
	res := fitAll(paths, testkit.TempDir(t), winMarina, func(int) fitVariant { return fitVariant{} }, geom.DefaultFaceNorm)
	s := checkRound(t, "портреты", res)
	for _, r := range res {
		got := "нет лица в окне"
		if r.Got != nil {
			size, eye, cx := gotShare(*r.Got)
			got = fmt.Sprintf("рамка %.3f глаза %.3f центр %.3f", size, eye, cx)
		}
		t.Logf("портреты %s: %s, флаг %d, S=%.2f; %s", r.Name, map[bool]string{true: "пропуск: " + r.Skip, false: "подогнано"}[r.Skip != ""], r.Flag, r.Crop.S, got)
	}
	// Детские портреты — крупные планы: лицо в кадре крупнее нормы, S уже 1, и флаг «крупное»
	// законен (решение 0.27-130-61). Допуски те же, но мест без флага мало: хотя бы одно.
	t.Logf("портреты: до нормы дошло %d из %d", s.N, len(paths))
	checkAccuracy(t, "портреты", s, 1)
}

// TestPickFace: наибольшая рамка; похожие по размеру — ближайшая к центру; пусто — нет лица.
func TestPickFace(t *testing.T) {
	if _, _, ok := pickFace(nil); ok {
		t.Fatal("пустой список дал лицо")
	}
	big := Face{Box: Rect{X: 0.1, Y: 0.1, W: 0.2, H: 0.4}}
	small := Face{Box: Rect{X: 0.4, Y: 0.3, W: 0.1, H: 0.2}}
	if f, many, _ := pickFace([]Face{small, big}); many || f.Box != big.Box {
		t.Fatalf("наибольшее не выбрано: %+v many=%v", f, many)
	}
	near := Face{Box: Rect{X: 0.4, Y: 0.3, W: 0.2, H: 0.3}} // 75 % от big, у центра
	if f, many, _ := pickFace([]Face{big, near}); !many || f.Box != near.Box {
		t.Fatalf("похожие по размеру: ждали ближайшее к центру, получили %+v many=%v", f, many)
	}
}

// TestFitStats: медиана, σ и доли на известных рядах, пустой ряд не падает.
func TestFitStats(t *testing.T) {
	v := []float64{-0.03, 0.01, 0.0, 0.02}
	if m := median(v); math.Abs(m-0.005) > 1e-12 {
		t.Errorf("медиана %g", m)
	}
	if p := within(v, 0.02); p != 0.75 {
		t.Errorf("доля в допуске %g", p)
	}
	if m := maxAbs(v); m != 0.03 {
		t.Errorf("наибольшее %g", m)
	}
	if s := sigma([]float64{1, 1, 1}); s != 0 {
		t.Errorf("σ ровного ряда %g", s)
	}
	if median(nil) != 0 || sigma(nil) != 0 || within(nil, 0.1) != 1 || maxAbs(nil) != 0 {
		t.Error("пустой ряд должен давать нули")
	}
}
