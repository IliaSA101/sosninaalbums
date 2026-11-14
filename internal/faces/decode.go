package faces

import (
	"fmt"
	"math"
	"sort"
)

// strides — шаги голов YuNet: на каждый уровень по четыре выхода (cls, obj, bbox, kps).
var strides = [...]int{8, 16, 32}

// cand — кандидат в пикселях входа сети.
type cand struct {
	score      float64
	x, y, w, h float64
	pts        [5][2]float64 // правый глаз человека (слева на снимке), левый, нос, углы рта
}

// decode собирает кандидатов с оценкой не ниже thr из выходов голов (как cv::FaceDetectorYN):
// оценка — корень из cls·obj, рамка — центр в ячейке и логарифм размера, точки — смещения от ячейки.
// W — ширина входа сети в пикселях; число ячеек на уровне сверяется с выходом.
func decode(outs map[string]headOut, W, H int, thr float64) ([]cand, error) {
	var res []cand
	for _, s := range strides {
		cols, rows := W/s, H/s
		cls, obj := outs[fmt.Sprintf("cls_%d", s)], outs[fmt.Sprintf("obj_%d", s)]
		bb, kp := outs[fmt.Sprintf("bbox_%d", s)], outs[fmt.Sprintf("kps_%d", s)]
		if cls.n != cols*rows || obj.n != cls.n || bb.n != cls.n || kp.n != cls.n ||
			cls.c != 1 || obj.c != 1 || bb.c != 4 || kp.c != 10 {
			return nil, fmt.Errorf("faces: головы уровня %d не по размеру входа %dx%d", s, W, H)
		}
		for i := range cls.n {
			sc := math.Sqrt(clamp01(float64(cls.d[i])) * clamp01(float64(obj.d[i])))
			if sc < thr || math.IsNaN(sc) {
				continue
			}
			r, c := i/cols, i%cols
			b, k := bb.d[4*i:4*i+4], kp.d[10*i:10*i+10]
			fs := float64(s)
			cx, cy := (float64(c)+float64(b[0]))*fs, (float64(r)+float64(b[1]))*fs
			bw, bh := math.Exp(float64(b[2]))*fs, math.Exp(float64(b[3]))*fs
			f := cand{score: sc, x: cx - bw/2, y: cy - bh/2, w: bw, h: bh}
			for j := range f.pts {
				f.pts[j] = [2]float64{(float64(k[2*j]) + float64(c)) * fs, (float64(k[2*j+1]) + float64(r)) * fs}
			}
			res = append(res, f)
		}
	}
	return res, nil
}

func clamp01(v float64) float64 { return math.Min(1, math.Max(0, v)) }

// nms оставляет лучшие рамки: рамка с IoU больше thr к уже взятой отбрасывается. Порядок —
// по убыванию оценки; при равенстве остаётся порядок обхода, поэтому результат повторяем.
func nms(fs []Face, thr float64) []Face {
	sort.SliceStable(fs, func(i, j int) bool { return fs[i].Score > fs[j].Score })
	var keep []Face
	for _, f := range fs {
		ok := true
		for _, g := range keep {
			if iou(f.Box, g.Box) > thr {
				ok = false
				break
			}
		}
		if ok {
			keep = append(keep, f)
		}
	}
	return keep
}

func iou(a, b Rect) float64 {
	x1, y1 := math.Max(a.X, b.X), math.Max(a.Y, b.Y)
	x2, y2 := math.Min(a.X+a.W, b.X+b.W), math.Min(a.Y+a.H, b.Y+b.H)
	in := math.Max(0, x2-x1) * math.Max(0, y2-y1)
	u := a.W*a.H + b.W*b.H - in
	if u <= 0 {
		return 0
	}
	return in / u
}
