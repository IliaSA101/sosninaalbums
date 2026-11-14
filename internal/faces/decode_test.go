package faces

import (
	"math"
	"testing"
)

// synthHeads — выходы голов для входа 64×64 (сетки 8×8, 4×4, 2×2), всё нули: оценка 0.
func synthHeads() map[string]headOut {
	outs := map[string]headOut{}
	for _, s := range strides {
		n := (64 / s) * (64 / s)
		outs["cls_"+itoa(s)] = headOut{n: n, c: 1, d: make([]float32, n)}
		outs["obj_"+itoa(s)] = headOut{n: n, c: 1, d: make([]float32, n)}
		outs["bbox_"+itoa(s)] = headOut{n: n, c: 4, d: make([]float32, 4*n)}
		outs["kps_"+itoa(s)] = headOut{n: n, c: 10, d: make([]float32, 10*n)}
	}
	return outs
}

func itoa(n int) string { return map[int]string{8: "8", 16: "16", 32: "32"}[n] }

// put ставит срабатывание в ячейку (r, c) уровня s: cls, obj, рамка (смещение центра и логарифм размера в долях шага).
func put(outs map[string]headOut, s, r, c int, cls, obj float32, b [4]float32, k [10]float32) {
	i := r*(64/s) + c
	outs["cls_"+itoa(s)].d[i] = cls
	outs["obj_"+itoa(s)].d[i] = obj
	copy(outs["bbox_"+itoa(s)].d[4*i:], b[:])
	copy(outs["kps_"+itoa(s)].d[10*i:], k[:])
}

// TestDecodeSynthetic: выдуманные выходы голов → ожидаемые рамки, точки, оценки и склейка.
func TestDecodeSynthetic(t *testing.T) {
	outs := synthHeads()
	ln2, ln3 := float32(math.Log(2)), float32(math.Log(3))
	// уровень 8, ячейка (2,3): центр (3,5+0,5)·8 = (28, 18), размер 2·8 × 3·8 → рамка x 20, y 6, 16×24
	put(outs, 8, 2, 3, 0.81, 1, [4]float32{0.5, 0.25, ln2, ln3},
		[10]float32{-0.25, -0.125, 0.25, -0.125, 0, 0.125, -0.25, 0.375, 0.25, 0.375})
	// двойник того же лица на уровне 16: почти та же рамка, оценка ниже — склейка его уберёт
	put(outs, 16, 1, 1, 0.7225, 1, [4]float32{0.6, 0.6, float32(math.Log(1)), float32(math.Log(1.5))}, [10]float32{})
	// другое лицо на уровне 32, ячейка (1,0): центр (0,5·32, 1,5·32) = (16, 48)
	put(outs, 32, 1, 0, 0.9025, 1, [4]float32{0.5, 0.5, 0, 0}, [10]float32{})
	// ниже порога, выше единицы (зажим до 1: оценка √(1·0,65) = 0,806), отрицательная (зажим до 0), NaN
	put(outs, 8, 6, 6, 0.79, 0.79, [4]float32{0.5, 0.5, 0, 0}, [10]float32{})
	put(outs, 8, 7, 0, 2, 0.65, [4]float32{0.5, 0.5, 0, 0}, [10]float32{})
	put(outs, 8, 7, 7, -3, 1, [4]float32{0.5, 0.5, 0, 0}, [10]float32{})
	put(outs, 8, 0, 7, float32(math.NaN()), 1, [4]float32{0.5, 0.5, 0, 0}, [10]float32{})

	cs, err := decode(outs, 64, 64, 0.8)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 4 { // 0,9 · зажатая 0,806 · двойник 0,85 · 0,95 (NaN, 0,79 и отрицательная отсеяны)
		t.Fatalf("кандидатов %d, ждали 4: %+v", len(cs), cs)
	}
	first := cs[0] // порядок обхода: уровень 8, строки сверху вниз
	near := func(name string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 1e-5 {
			t.Errorf("%s = %.6f, ждали %.6f", name, got, want)
		}
	}
	near("score", first.score, 0.9)
	near("x", first.x, 20)
	near("y", first.y, 6)
	near("w", first.w, 16)
	near("h", first.h, 24)
	near("глаз0 x", first.pts[0][0], (-0.25+3)*8)
	near("глаз0 y", first.pts[0][1], (-0.125+2)*8)
	near("глаз1 x", first.pts[1][0], (0.25+3)*8)
	near("нос y", first.pts[2][1], (0.125+2)*8)
	near("рот1 y", first.pts[4][1], (0.375+2)*8)

	f := first.face(64, 32) // в долях картинки 64×32: y и высота делятся на 32
	near("доля x", f.Box.X, 20.0/64)
	near("доля y", f.Box.Y, 6.0/32)
	near("доля h", f.Box.H, 24.0/32)
	near("глаза: середина y", f.EyeLine().Y, (-0.125+2)*8/32)

	// склейка: двойник (IoU с первым > 0,3) уходит, остаются 0,95 → 0,9 → 0,806 по убыванию
	fs := make([]Face, len(cs))
	for i, c := range cs {
		fs[i] = c.face(64, 64)
	}
	kept := nms(fs, 0.3)
	if len(kept) != 3 {
		t.Fatalf("после склейки %d лиц, ждали 3", len(kept))
	}
	for i := 1; i < len(kept); i++ {
		if kept[i].Score > kept[i-1].Score {
			t.Errorf("не по убыванию оценки: %.3f после %.3f", kept[i].Score, kept[i-1].Score)
		}
	}
	near("лучшая оценка", kept[0].Score, 0.95)
	for _, k := range kept {
		if math.Abs(k.Score-0.85) < 1e-6 {
			t.Errorf("двойник 0,85 не склеен")
		}
	}
}

// TestDecodeSizeMismatch: головы не по размеру входа — ошибка, а не чтение за краем.
func TestDecodeSizeMismatch(t *testing.T) {
	outs := synthHeads()
	if _, err := decode(outs, 128, 64, 0.8); err == nil {
		t.Error("головы 64×64 приняты за вход 128×64")
	}
	delete(outs, "kps_16")
	if _, err := decode(outs, 64, 64, 0.8); err == nil {
		t.Error("без головы kps_16 ошибки нет")
	}
}

// TestPlausibleEyes: фильтр «середина глаз на 0,2…0,7 высоты рамки».
func TestPlausibleEyes(t *testing.T) {
	box := Rect{0.2, 0.1, 0.2, 0.4}
	at := func(frac float64) Face {
		y := box.Y + frac*box.H
		return Face{Box: box, Eyes: [2]Pt{{0.25, y}, {0.35, y}}}
	}
	for _, c := range []struct {
		frac float64
		want bool
	}{{0.046, false}, {0.19, false}, {0.21, true}, {0.32, true}, {0.556, true}, {0.69, true}, {0.71, false}, {0.84, false}} {
		if got := at(c.frac).plausible(); got != c.want {
			t.Errorf("глаза на %.3f высоты рамки: %v, ждали %v", c.frac, got, c.want)
		}
	}
	if (Face{}).plausible() || (Face{Box: Rect{W: 1}}).plausible() {
		t.Error("рамка нулевой высоты принята")
	}
	// порядок глаз неважен: середина одна
	f := at(0.4)
	f.Eyes[0], f.Eyes[1] = f.Eyes[1], f.Eyes[0]
	if !f.plausible() {
		t.Error("перестановка глаз сломала фильтр")
	}
}

func TestIoU(t *testing.T) {
	a := Rect{0, 0, 1, 1}
	if v := iou(a, a); math.Abs(v-1) > 1e-12 {
		t.Errorf("iou с собой %v", v)
	}
	if v := iou(a, Rect{2, 2, 1, 1}); v != 0 {
		t.Errorf("iou непересекающихся %v", v)
	}
	if v := iou(a, Rect{0.5, 0, 1, 1}); math.Abs(v-1.0/3) > 1e-12 {
		t.Errorf("iou половины %v, ждали 1/3", v)
	}
	if v := iou(Rect{}, Rect{}); v != 0 {
		t.Errorf("iou пустых рамок %v", v)
	}
}

// TestThresholdsPinned: пороги детектора закреплены числами, а не именами констант — иначе
// правка константы молча правила бы и тесты (Version кэша точек меняется вместе с ними).
func TestThresholdsPinned(t *testing.T) {
	if minScore != 0.8 || nmsIoU != 0.3 || eyeMin != 0.2 || eyeMax != 0.7 || inputLong != 640 {
		t.Errorf("пороги: оценка %v, склейка %v, глаза %v…%v, вход %v — менять с новой Version и спекой",
			minScore, nmsIoU, eyeMin, eyeMax, inputLong)
	}
}

// TestNMSThreshold: склейка рамок — по IoU больше 0,3 (число из cv::FaceDetectorYN, не константа
// кода): при 0,29 лица остаются оба, при 0,32 — одно, с лучшей оценкой.
func TestNMSThreshold(t *testing.T) {
	box := func(dx float64, score float64) Face { return Face{Score: score, Box: Rect{dx, 0, 10, 10}} }
	// пересечение (10−dx)·10, объединение 200 − пересечение
	for _, c := range []struct {
		dx   float64
		want int
	}{{5.5, 2}, {5.2, 1}, {0, 1}, {20, 2}} {
		got := nms([]Face{box(0, 0.9), box(c.dx, 0.95)}, 0.3)
		if len(got) != c.want {
			t.Errorf("сдвиг %v (IoU %.3f): рамок %d, ждали %d", c.dx, iou(Rect{0, 0, 10, 10}, Rect{c.dx, 0, 10, 10}), len(got), c.want)
		}
		if c.want == 1 && got[0].Score != 0.95 {
			t.Errorf("сдвиг %v: осталась рамка с оценкой %v, ждали лучшую", c.dx, got[0].Score)
		}
	}
	// склейка в Detect идёт по тому же порогу, что в коде
	if got := nms([]Face{box(0, 0.9), box(5.2, 0.8)}, nmsIoU); len(got) != 1 {
		t.Errorf("nmsIoU = %v не склеил рамки с IoU 0,316", nmsIoU)
	}
}
