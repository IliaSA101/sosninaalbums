package faces

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"testing"
)

// refOut — эталон одного выхода сети из testdata/ref.json (снят onnxruntime скриптом yunet-ref.py).
type refOut struct {
	Shape []int     `json:"shape"`
	Mean  float64   `json:"mean"`
	Idx   []int     `json:"idx"`
	Val   []float64 `json:"val"`
}

type refFile struct {
	ModelSHA string            `json:"model_sha256"`
	H        int               `json:"h"` // h и w нет в ref.json (там 640×640)
	W        int               `json:"w"`
	Outputs  map[string]refOut `json:"outputs"`
}

// syntheticInput — вход h×w по целочисленной формуле (та же — в testdata/yunet-ref.py),
// чтобы вход совпадал до бита и разница шла только от вывода.
func syntheticInput(h, w int) []float32 {
	d := make([]float32, 3*h*w)
	for c := range 3 {
		for y := range h {
			for x := range w {
				d[c*h*w+y*w+x] = float32((x*3 + y*5 + c*61 + ((x * y) >> 6)) & 255)
			}
		}
	}
	return d
}

// TestYuNetParity: вывод на Go совпадает с onnxruntime на всех 12 выходах — 64 значения с
// |Δ| ≤ 1e-4 и среднее во float64 с |Δ| ≤ 1e-5. Сумма не годится: у Go и onnxruntime она
// расходится в последних разрядах, а поэлементно выходы совпадают до 1e-5 (design §10).
// Два размера: 640×640 (как записан в модели) и 448×640 (рабочий вход кадра 3:2, у onnxruntime
// форма входа правится в памяти — testdata/yunet-ref.py).
func TestYuNetParity(t *testing.T) {
	for _, c := range []struct {
		file string
		h, w int
	}{{"testdata/ref.json", 640, 640}, {"testdata/ref-448x640.json", 448, 640}} {
		t.Run(fmt.Sprintf("%dx%d", c.h, c.w), func(t *testing.T) { parity(t, c.file, c.h, c.w) })
	}
}

func parity(t *testing.T, file string, h, w int) {
	var ref refFile
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &ref); err != nil {
		t.Fatal(err)
	}
	if ref.H != 0 && (ref.H != h || ref.W != w) {
		t.Fatalf("%s снят на входе %d×%d, тест гоняет %d×%d", file, ref.H, ref.W, h, w)
	}
	if got := sha(modelFile); got != ref.ModelSHA {
		t.Fatalf("эталон снят на другой модели: %s, вшита %s", ref.ModelSHA, got)
	}
	n, err := loadNet()
	if err != nil {
		t.Fatal(err)
	}
	for _, workers := range []int{1, 3} { // и один поток, и несколько: результат один
		outs, err := n.run(syntheticInput(h, w), h, w, workers, &memTrack{})
		if err != nil {
			t.Fatal(err)
		}
		if len(outs) != 12 || len(ref.Outputs) != 12 {
			t.Fatalf("выходов %d, эталонов %d, ждали 12", len(outs), len(ref.Outputs))
		}
		for _, name := range sortedKeys(ref.Outputs) {
			got, ok := outs[name]
			if !ok {
				t.Fatalf("нет выхода %s", name)
			}
			compareOut(t, name, workers, got, ref.Outputs[name])
		}
	}
}

func sortedKeys(m map[string]refOut) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// compareOut сверяет выход с эталоном: форма, 64 значения (1e-4), среднее во float64 (1e-5).
func compareOut(t *testing.T, name string, workers int, got headOut, want refOut) {
	t.Helper()
	if want.Shape[0] != 1 || got.n != want.Shape[1] || got.c != want.Shape[2] {
		t.Fatalf("%s: форма [1 %d %d], эталон %v", name, got.n, got.c, want.Shape)
	}
	maxd := 0.0
	for k, i := range want.Idx {
		maxd = math.Max(maxd, math.Abs(float64(got.d[i])-want.Val[k]))
	}
	if maxd > 1e-4 {
		t.Errorf("%s (потоков %d): |Δ| по 64 значениям %.2e > 1e-4", name, workers, maxd)
	}
	var sum float64
	for _, v := range got.d {
		sum += float64(v)
	}
	dm := math.Abs(sum/float64(len(got.d)) - want.Mean)
	if dm > 1e-5 {
		t.Errorf("%s (потоков %d): среднее расходится на %.2e > 1e-5", name, workers, dm)
	}
	if workers == 1 {
		t.Logf("%s: max|Δ| %.1e, Δсреднего %.1e", name, maxd, dm)
	}
}
