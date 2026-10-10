package model

import (
	"math"
	"strings"
	"testing"
	"unicode"

	"github.com/IliaSA101/sosninaalbums/internal/text"
)

func TestClampFrame(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	for _, c := range []struct {
		in   *Frame
		want *Frame
	}{
		{nil, nil},
		{&Frame{S: 1}, nil},
		{&Frame{S: nan, X: inf, Y: -inf}, nil},
		{&Frame{S: 0.5, X: 0.0001}, nil}, // шум округления — тоже сброс
		{&Frame{S: 10, X: -3, Y: 3}, &Frame{S: 4, X: -1, Y: 1}},
		{&Frame{S: 1.23456, X: 0.12345, Y: nan}, &Frame{S: 1.235, X: 0.123}},
	} {
		got := ClampFrame(c.in)
		if (got == nil) != (c.want == nil) || got != nil && *got != *c.want {
			t.Errorf("ClampFrame(%+v) = %+v, ждали %+v", c.in, got, c.want)
		}
	}
}

func TestClampColors(t *testing.T) {
	if c, ok := ClampColor("#ABCdef"); !ok || c != "#abcdef" {
		t.Fatalf("цвет: %q", c)
	}
	for _, bad := range []string{"", "#abc", "red", "#gggggg", "#1234567", "abcdef1", "#12345\x00"} {
		if c, ok := ClampColor(bad); ok || c != "" {
			t.Errorf("цвет %q пропущен", bad)
		}
	}
	if st := ClampStyle(TextStyle{Color: "#FFF"}); st.Color != text.DefaultColor || st.Font != text.DefaultFont {
		t.Fatalf("стиль: %+v", st)
	}
	p := New()
	p.Settings.Bg = "#FFF"
	p.Spreads = []*Spread{{ID: "s", BgL: "#AA0000", BgR: "url(x)"}}
	p.ClampDesign()
	if p.Settings.Bg != "" || p.Spreads[0].BgL != "#aa0000" || p.Spreads[0].BgR != "" {
		t.Fatalf("фон: %q %q %q", p.Settings.Bg, p.Spreads[0].BgL, p.Spreads[0].BgR)
	}
}

func TestClampTexts(t *testing.T) {
	p := New()
	nan := math.NaN()
	long := strings.Repeat("я", 1500)
	many := strings.Repeat("строка\r\n", 70)
	blocks := []*TextBlock{
		nil,
		{ID: "c0ffee000001", Text: "  \n\t", W: 100}, // пустой — выброшен
		{ID: "c0ffee000001", Text: "a\u202eb\x07c\u2028d", W: nan, X: nan, Y: math.Inf(-1)},
		{ID: "c0ffee000001", Text: long, W: 5, X: 1e6, Y: 1e6},   // повтор ID
		{ID: "C0FFEE000002", Text: many, W: 1e6, X: -10, Y: -10}, // ID не в нижнем регистре
		{ID: "x", Text: "ok", W: 100, Style: TextStyle{Font: "marck", Bold: true, Size: 1e9, Align: "?"}},
	}
	for range 30 {
		blocks = append(blocks, &TextBlock{ID: NewID(), Text: "t", W: 50})
	}
	p.Spreads = []*Spread{{ID: "s", Texts: blocks}}
	long2 := strings.Repeat("ж", 400)
	p.Photos["p"] = PhotoRef{Path: "/p.jpg", Caption: &Caption{Text: long2, Style: TextStyle{Size: nan}}}
	p.Photos["q"] = PhotoRef{Path: "/q.jpg", Caption: &Caption{Text: " \r\n "}}
	p.ClampDesign()
	p.FitTexts()
	ts := p.Spreads[0].Texts
	if len(ts) != MaxTexts {
		t.Fatalf("блоков %d", len(ts))
	}
	seen := map[string]bool{}
	sw, ph := 2*p.Settings.PageW, p.Settings.PageH
	for i, b := range ts {
		h := text.Cached(b.Text, b.Style, b.W).H
		if !IsTextID(b.ID) || seen[b.ID] || b.W < MinTextW || b.W > sw || b.X < 0 || b.X+b.W > sw+1e-9 || b.Y < 0 || b.Y+h > ph+1e-9 && b.Y != 0 {
			t.Fatalf("блок %d: %+v (h %.1f)", i, b, h)
		}
		seen[b.ID] = true
		for _, r := range b.Text {
			if r != '\n' && (unicode.IsControl(r) || unicode.Is(unicode.Cf, r)) {
				t.Fatalf("блок %d: управляющий знак %U", i, r)
			}
		}
	}
	if ts[0].Text != "abc\nd" || ts[0].W != 0.4*p.Settings.PageW || ts[0].X != 0 || ts[0].Y != 0 {
		t.Fatalf("NaN и чистка: %+v", ts[0])
	}
	if n := len([]rune(ts[1].Text)); n != MaxTextRunes || ts[1].ID == "c0ffee000001" {
		t.Fatalf("длина %d, id %s", n, ts[1].ID)
	}
	if n := strings.Count(ts[2].Text, "\n") + 1; n != MaxTextLines || strings.Contains(ts[2].Text, "\r") {
		t.Fatalf("строк %d", n)
	}
	if st := ts[3].Style; st.Bold || st.Size != text.MaxSize || st.Align != "center" {
		t.Fatalf("стиль: %+v", st)
	}
	if c := p.Photos["p"].Caption; c == nil || len([]rune(c.Text)) != MaxCaptionRunes || c.Style.Size != text.DefaultSize {
		t.Fatalf("подпись: %+v", c)
	}
	if p.Photos["q"].Caption != nil {
		t.Fatal("пустая подпись должна сниматься")
	}
	// Второй проход ничего не меняет: холостая правка остаётся холостой.
	before := Marshal(p)
	p.ClampDesign()
	p.FitTexts()
	if string(before) != string(Marshal(p)) {
		t.Fatal("зажим не идемпотентен")
	}
}

// Кадр по умолчанию: без Frame в PhotoRef — S = 1 без сдвига (проверка ушла из
// TestFramesParity при переносе в geom).
func TestDefaultFrame(t *testing.T) {
	if (PhotoRef{}).FrameOrDefault() != (Frame{S: 1}) {
		t.Fatal("кадр по умолчанию")
	}
}
