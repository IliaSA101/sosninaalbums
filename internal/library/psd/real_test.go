package psd

import (
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// refsDir ищет docs/refs/fsfera-psd вверх от рабочей папки теста (без личных путей); "" — нет.
// Папка лежит вне git в основном дереве, поэтому из рабочей копии агента идём вверх, пока не найдём.
func refsDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		p := filepath.Join(dir, "docs", "refs", "fsfera-psd")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
		up := filepath.Dir(dir)
		if up == dir {
			return ""
		}
		dir = up
	}
}

// mmOf переводит px документа в мм при dpi.
func mmOf(px, dpi float64) float64 { return px / dpi * 25.4 }

// hasGuide: есть направляющая оси horz в want ± realTol мм.
func hasGuide(in Info, horz bool, want float64) bool {
	for _, g := range in.Guides {
		if g.Horz == horz && math.Abs(mmOf(g.Pos, in.DPI)-want) <= realTol {
			return true
		}
	}
	return false
}

// realTol — допуск, мм: положения в шаблоне округлены до пикселя 300 dpi (0,085 мм) и самим
// автором шаблона (по вертикали обрезы расходятся до 0,18 мм, research §2).
const realTol = 0.2

// need — ожидаемая направляющая.
type need struct {
	horz bool
	mm   float64
	what string
}

// realWant — ожидание по файлу: размер при 300 dpi и зоны (research §2).
type realWant struct {
	file        string
	w, h        int
	bleed, safe float64 // разворот: обрез и безопасная зона от внешнего края файла
	fold, spine float64 // обложка: загиб и ширина корешка; fold == 0 — это разворот
}

func (c realWant) needs(w, h float64) []need {
	var n []need
	if c.fold == 0 {
		for _, z := range []struct {
			v    float64
			what string
		}{{c.bleed, "обрез"}, {c.safe, "безопасная"}} {
			n = append(n, need{false, z.v, z.what + " слева"}, need{false, w - z.v, z.what + " справа"},
				need{true, z.v, z.what + " сверху"}, need{true, h - z.v, z.what + " снизу"})
		}
		return append(n, need{false, w / 2, "сгиб"})
	}
	return append(n,
		need{false, c.fold, "загиб слева"}, need{false, w - c.fold, "загиб справа"},
		need{true, c.fold, "загиб сверху"}, need{true, h - c.fold, "загиб снизу"},
		need{false, w/2 - c.spine/2, "корешок слева"}, need{false, w/2 + c.spine/2, "корешок справа"})
}

// TestRealFotosferaPSD разбирает настоящие шаблоны Фотосферы (docs/refs/fsfera-psd, вне git) и
// сверяет размер, dpi и направляющие с числами research §2. Без папки (CI, чистый клон) пропускается.
func TestRealFotosferaPSD(t *testing.T) {
	dir := refsDir()
	if dir == "" {
		t.Skip("нет docs/refs/fsfera-psd (шаблоны вне git)")
	}
	for _, c := range []realWant{
		{file: "spread_20x30.psd", w: 4748, h: 3224, bleed: 2, safe: 10},
		{file: "spread_25x25.psd", w: 5929, h: 3000, bleed: 2, safe: 8},
		{file: "spread_30x30.psd", w: 6638, h: 3354, bleed: 2, safe: 10},
		{file: "Cover_thin_20x30_31-34.psd", w: 5575, h: 3732, fold: 20, spine: 20},
		{file: "Cover_thin_21x30_18-22.psd", w: 5528, h: 3874, fold: 16, spine: 14},
		{file: "Cover_thin_30x30_18-22.psd", w: 7394, h: 3850, fold: 20, spine: 14},
		{file: "Cover_thick_30x30_10-12.psd", w: 7394, h: 3850, fold: 20, spine: 14},
		{file: "Cover_2Xcardboard_30x30_10.psd", w: 7441, h: 3850, fold: 20, spine: 18},
	} {
		t.Run(c.file, func(t *testing.T) { checkReal(t, dir, c) })
	}
}

func checkReal(t *testing.T, dir string, c realWant) {
	f, err := os.Open(filepath.Join(dir, c.file))
	if err != nil {
		t.Skip("файла нет:", err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	var img *image.NRGBA
	var in Info
	d := testkit.Fastest(1, func() { img, in, err = Preview(f, st.Size(), 4096) })
	if lim := testkit.Limit(10 * time.Second); d > lim {
		t.Errorf("разбор %v дольше %v", d, lim)
	}
	if err != nil {
		t.Fatal(err)
	}
	if in.W != c.w || in.H != c.h || in.DPI != 300 || in.Mode != modeRGB || in.Depth != 8 || !in.Merged || !in.Image || img == nil {
		t.Fatalf("заголовок: %+v", in)
	}
	k := (max(in.W, in.H) + 4095) / 4096
	if b := img.Bounds(); b.Dx() != (in.W+k-1)/k || b.Dy() != (in.H+k-1)/k {
		t.Fatalf("превью %v (k=%d)", b, k)
	}
	if !hasContrast(img) {
		t.Fatal("превью однотонное: сведённое прочитано неверно")
	}
	// ReadInfo без картинки даёт те же направляющие.
	if in2, err := ReadInfo(f, st.Size()); err != nil || len(in2.Guides) != len(in.Guides) {
		t.Fatalf("ReadInfo: %v %d/%d", err, len(in2.Guides), len(in.Guides))
	}
	for _, n := range c.needs(mmOf(float64(in.W), in.DPI), mmOf(float64(in.H), in.DPI)) {
		if !hasGuide(in, n.horz, n.mm) {
			t.Errorf("нет направляющей «%s» (%.2f мм, горизонтальная %v): %+v", n.what, n.mm, n.horz, in.Guides)
		}
	}
}

// hasContrast: на превью есть заметный разброс яркости (шаблон — белый лист с цветными полями).
func hasContrast(img *image.NRGBA) bool {
	lo, hi := 255, 0
	for i := 0; i < len(img.Pix); i += 4 {
		v := (int(img.Pix[i]) + int(img.Pix[i+1]) + int(img.Pix[i+2])) / 3
		lo, hi = min(lo, v), max(hi, v)
	}
	return hi-lo > 40
}
