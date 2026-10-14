package model

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// randomTexts — проект с блоками у краёв и за краями: углы, выравнивания, «по тексту».
func randomTexts(r *rand.Rand, n int) *Project {
	p := New()
	words := []string{"Текст", "Иван и Мария", "AVATAR WAVE", "очень длинная строка про свадьбу в саду у реки", "раз\nдва\nтри"}
	aligns := []string{"left", "center", "right"}
	s := &Spread{ID: NewID()}
	for range n {
		st := plainStyle
		st.Align = aligns[r.IntN(3)]
		st.Size = 6 + r.Float64()*60
		tb := &TextBlock{ID: NewID(), X: r.Float64()*700 - 50, Y: r.Float64()*400 - 50, W: r.Float64() * 700,
			Rot: geom.NormDeg(r.Float64()*400 - 200), Auto: r.IntN(2) == 0, Text: words[r.IntN(len(words))], Style: st}
		if r.IntN(4) == 0 {
			tb.Rot = 0
		}
		s.Texts = append(s.Texts, tb)
		if len(s.Texts) == MaxTexts {
			p.Spreads = append(p.Spreads, s)
			s = &Spread{ID: NewID()}
		}
	}
	if len(s.Texts) > 0 {
		p.Spreads = append(p.Spreads, s)
	}
	return p
}

func TestFitTextsIdempotent(t *testing.T) {
	r := rand.New(rand.NewPCG(8, 11))
	for i := range 20 {
		p := randomTexts(r, 40)
		if i%2 == 1 {
			p.Settings.PageW, p.Settings.PageH = 50+r.Float64()*300, 50+r.Float64()*300
		}
		p.FitTexts()
		once := Marshal(p)
		p.FitTexts()
		if string(once) != string(Marshal(p)) {
			t.Fatalf("прогон %d: второй fitTexts поменял проект", i)
		}
		sw, ph := 2*p.Settings.PageW, p.Settings.PageH
		for _, s := range p.Spreads {
			for _, tb := range s.Texts {
				a := BoxOf(tb).AABB()
				if a.W <= sw && (a.X < -1e-6 || a.X+a.W > sw+1e-6) || a.W > sw && math.Abs(a.X) > 0.01 {
					t.Fatalf("прогон %d: блок за разворотом по x: %+v, aabb %+v", i, *tb, a)
				}
				if a.H <= ph && (a.Y < -1e-6 || a.Y+a.H > ph+1e-6) {
					t.Fatalf("прогон %d: блок за разворотом по y: %+v, aabb %+v", i, *tb, a)
				}
			}
		}
	}
}

func TestFitTextsRotated(t *testing.T) {
	p := New()
	tb := &TextBlock{ID: NewID(), X: 590, Y: -10, W: 100, Rot: 30, Text: "Текст у края", Style: plainStyle}
	p.Spreads = []*Spread{{ID: NewID(), Texts: []*TextBlock{tb}}}
	p.FitTexts()
	a := BoxOf(tb).AABB()
	if a.X < 0 || a.X+a.W > 600+1e-9 || a.Y < 0 || a.X+a.W < 600-0.011 || a.Y > 0.011 || tb.Rot != 30 || tb.W != 100 {
		t.Fatalf("повёрнутый у края: %+v, aabb %+v", *tb, a)
	}
	if math.Abs(tb.X*100-math.Round(tb.X*100)) > 1e-6 {
		t.Fatalf("x не округлён до сотых: %v", tb.X)
	}
}

// BenchmarkFitTexts2000 — fitTexts на 2000 блоках (MaxTotalTexts) с тёплым кэшем; цель ≤ 20 мс.
func BenchmarkFitTexts2000(b *testing.B) {
	p := randomTexts(rand.New(rand.NewPCG(1, 2)), MaxTotalTexts)
	p.FitTexts()
	b.ResetTimer()
	for b.Loop() {
		p.FitTexts()
	}
}

// Ширина «по тексту» — наименьшая, при которой текст встаёт одной строкой.
func TestAutoW(t *testing.T) {
	tb := &TextBlock{Text: "Анна и Пётр AVATAR", Style: plainStyle}
	w := AutoW(nil, tb, 1000)
	if n := len(text.Cached(tb.Text, tb.Style, w).Lines); n != 1 {
		t.Fatalf("в ширину %v — %d строк", w, n)
	}
	if nat := text.NaturalWidth(tb.Text, tb.Style); w < nat || w > nat+0.02+1e-9 {
		t.Fatalf("ширина %v при естественной %v", w, nat)
	}
	if w = AutoW(nil, tb, 20); w != 20 {
		t.Fatalf("предел: %v", w)
	}
	if got := New().TextMaxW(nil); got != 284 {
		t.Fatalf("textMaxW %v", got)
	}
}
