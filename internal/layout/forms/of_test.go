package forms

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// allForms — все значения Form: Of возвращает только их.
var allForms = []Form{Mosaic, Strip, Rows, Grid, Air, Asym, Spine, Hero, Pair, Contrast, Ladder, Frame, Bleed}

// TestFormOf: вид по ключу и геометрии — каждый вид, старые ключи и граничные случаи.
func TestFormOf(t *testing.T) {
	asL, asP := same(12, aspL), same(12, aspP)
	for _, c := range []struct {
		key  string
		as   []float64
		want Form
	}{
		{"B[full:0]", asL, Bleed}, {"P[E(0)|1]", asL, Bleed}, {"B[pages:0,1]", asL, Bleed},
		{"L(0,1,2)", asL, Ladder}, {"A(0,1,2,3)", asL, Ladder}, {"P[L(0,1,2)|H(3,4)]", asL, Ladder},
		{"H(m0,1,m2)", asL, Contrast}, {"Q[0|H(m1,2)]", asL, Contrast},
		{"Q[H(0,1)|H(2,3,4)]", asL, Strip}, {"QU[H(0,1)|H(2,3)]", asL, Strip}, {"Q[0|H(1,2)]", asL, Strip},
		{"U(H(0,1,2,3))", asL, Strip}, {"D(H(0,1,2,3))", asL, Strip}, {"H(0,1,2,3,4)", asL, Strip},
		{"H(0,1,2)", asP, Mosaic}, // одиночный ряд во всю высоту — не лента
		{"S(H(0,1,2))", asL, Air},
		{"G(H(0,1,2),H(3,4,5))", asL, Grid}, {"G(H(0,1,2),H(3,4,5),6)", asL, Grid},
		{"S(G(H(0,1),H(2,3)))", asL, Grid}, {"Q[G(H(0,1),H(2,3))|G(H(4,5),6)]", asL, Grid},
		{"G(H(0,1),H(2,3,4),H(5,6))", asL, Rows}, {"G(H(0,1,2),H(3,4,5,6))", asL, Rows}, {"G(0,1)", asL, Rows},
		{"P[S(0)|G(H(1,2),H(3,4))]", asL, Asym}, {"P[G(H(0,1),H(2,3))|D(4)]", asL, Asym},
		{"P[U(H(0,1))|G(H(2,3),H(4,5))]", asL, Asym},
		{"H(V(0,1,2),3,V(4,5,6))", asL, Frame},
		{"P[S(H(0,1))|S(H(2,3))]", asL, Air},
		{"V(H(0,1),H(2,3))", asL, Mosaic}, {"H(V(0,1),V(2,3))", asL, Mosaic},
	} {
		as := c.as[:len(leavesOfKey(c.key))]
		p := geomSq
		var rects []layout.Rect
		if c.want != Bleed || c.key == "B[full:0]" {
			// Старые ключи «в край» на 1,5 не строятся (обрезка) — им хватает ключа.
			if r, ok := layout.Build(c.key, as, p); ok {
				rects = r.Rects
			} else if c.want != Bleed {
				t.Fatalf("%q: Build отказал", c.key)
			}
		}
		if got := Of(c.key, rects, p); got != c.want {
			t.Errorf("%q: вид %s, ждали %s", c.key, got, c.want)
		}
	}
}

// leavesOfKey — позиции фото ключа (для длины набора в тестах).
func leavesOfKey(key string) []int {
	var out []int
	for i := 0; i < len(key); {
		if key[i] < '0' || key[i] > '9' {
			i++
			continue
		}
		n := 0
		for i < len(key) && key[i] >= '0' && key[i] <= '9' {
			n, i = n*10+int(key[i]-'0'), i+1
		}
		out = append(out, n)
	}
	return out
}

// TestFormOfHeroes: главное по геометрии — «через корешок», «главное фото», «пара крупных»;
// без главного и мелкая раскладка — воздух по заполнению (синтетические Rects).
func TestFormOfHeroes(t *testing.T) {
	r := func(x, y, w, h float64) layout.Rect { return layout.Rect{X: x, Y: y, W: w, H: h} }
	small := []layout.Rect{r(320, 12, 60, 40), r(390, 12, 60, 40), r(460, 12, 60, 40)}
	with := func(rs ...layout.Rect) []layout.Rect { return append(slices.Clone(small), rs...) }
	for _, c := range []struct {
		name string
		p    layout.Params
		rs   []layout.Rect
		want Form
	}{
		{"главное слева", geomSq, with(r(12, 12, 200, 150)), Hero},
		{"через корешок", geomSq, with(r(200, 100, 200, 150)), Spine},
		{"то же без layflat — главное", geomFlat, with(r(200, 100, 200, 150)), Hero},
		{"пара", geomSq, with(r(12, 12, 100, 100), r(150, 12, 100, 100)), Pair},
		{"главного нет, заполнение мало", geomSq, with(r(12, 12, 60, 40)), Air},
		{"главного нет, заполнено", geomSq, []layout.Rect{r(12, 12, 280, 280), r(300, 12, 288, 280), r(100, 100, 270, 270), r(20, 20, 260, 260)}, Mosaic},
	} {
		if got := Of("V(H(0,1),H(2,3))", c.rs, c.p); got != c.want {
			t.Errorf("%s: вид %s, ждали %s", c.name, got, c.want)
		}
	}
}

// TestFormOfLayouts: ряд Layouts (старые ключи) не получает видов, которых в старой грамматике
// нет (лесенка, мелкое — крупное); в основном он — мозаика, и разметка не падает.
func TestFormOfLayouts(t *testing.T) {
	count := map[Form]int{}
	for n := 2; n <= 9; n++ {
		for _, p := range []layout.Params{geomSq, geomFlat, geomWide} {
			as := mixed(n, uint64(n))
			for _, r := range layout.Layouts(as, p, 24) {
				f := Of(r.Key, r.Rects, p)
				if f == Ladder || f == Contrast || !slices.Contains(allForms, f) {
					t.Fatalf("%q: вид %q из старой грамматики", r.Key, f)
				}
				count[f]++
			}
		}
	}
	if count[Mosaic] == 0 || count[Mosaic] < count[Bleed] {
		t.Errorf("мозаик в ряду Layouts мало: %v", count)
	}
}

// TestFormOfGarbage: мусорные и обрезанные ключи не роняют Of и дают известный вид (вход из
// файла — недоверенный); nil и пустые Rects допустимы.
func TestFormOfGarbage(t *testing.T) {
	alphabet := []byte("HVGLAmPQSUDOIEB[](),|-:0123456789")
	r := rng(7)
	keys := []string{"", "(", ")", "Q", "P[", "P[|", "Q[", "S(", ")))", "B[", "G(", "L(", "H(", "QU", "P[]", "Q[]", "S()", "U(|)", "P[S(|)|]"}
	for range 3000 {
		b := make([]byte, int(r.unit()*24))
		for i := range b {
			b[i] = alphabet[int(r.unit()*float64(len(alphabet)))]
		}
		keys = append(keys, string(b))
	}
	for _, k := range keys {
		for _, rs := range [][]layout.Rect{nil, {}, {{X: 1, Y: 1, W: 5, H: 5}}} {
			if f := Of(k, rs, geomSq); !slices.Contains(allForms, f) {
				t.Fatalf("%q: вид %q", k, f)
			}
		}
	}
}

// TestName: у каждого вида есть подпись, у неизвестного — пусто.
func TestName(t *testing.T) {
	seen := map[string]Form{}
	for _, f := range allForms {
		n := Name(f)
		if n == "" {
			t.Errorf("у вида %s нет подписи", f)
		}
		if other, dup := seen[n]; dup {
			t.Errorf("подпись %q у %s и %s", n, f, other)
		}
		seen[n] = f
	}
	if Name("нет такого") != "" || Name("") != "" {
		t.Error("у неизвестного вида подпись не пуста")
	}
	if Name(Strip) != "Лента" || Name(Ladder) != "Лесенка" {
		t.Errorf("подписи: %q %q", Name(Strip), Name(Ladder))
	}
}
