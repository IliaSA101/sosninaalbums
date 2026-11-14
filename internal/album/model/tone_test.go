package model

import (
	"encoding/json"
	"testing"
)

// Тон фото (068): зажим, строка CSS, копия, зажим при Load и PruneRefs.

func TestClampTone(t *testing.T) {
	cases := []struct {
		in   *Tone
		want *Tone
	}{
		{nil, nil},
		{&Tone{}, nil}, // нули — «без тона»: в файле и снимке одно значение
		{&Tone{Gray: true}, &Tone{Gray: true}},
		{&Tone{Bright: 900, Contrast: -1_000_000_000}, &Tone{Bright: MaxTone, Contrast: -MaxTone}},
		{&Tone{Bright: -51, Contrast: 51}, &Tone{Bright: -MaxTone, Contrast: MaxTone}},
		{&Tone{Bright: 50, Contrast: -50}, &Tone{Bright: 50, Contrast: -50}},
		{&Tone{Bright: 1}, &Tone{Bright: 1}},
	}
	for _, c := range cases {
		got := ClampTone(c.in)
		if (got == nil) != (c.want == nil) || got != nil && *got != *c.want {
			t.Errorf("ClampTone(%+v) = %+v, ждали %+v", c.in, got, c.want)
		}
		if got != nil && got == c.in {
			t.Errorf("ClampTone(%+v) вернул тот же указатель", c.in)
		}
	}
}

func TestToneCSS(t *testing.T) {
	cases := []struct {
		t    *Tone
		want string
	}{
		{nil, ""},
		{&Tone{}, ""},
		{&Tone{Gray: true}, "grayscale(1)"},
		{&Tone{Bright: 20}, "brightness(120%)"},
		{&Tone{Contrast: -20}, "contrast(80%)"},
		{&Tone{Gray: true, Bright: -50, Contrast: 50}, "grayscale(1) brightness(50%) contrast(150%)"},
		{&Tone{Bright: 5, Contrast: -5}, "brightness(105%) contrast(95%)"},
	}
	for _, c := range cases {
		if got := c.t.CSS(); got != c.want {
			t.Errorf("%+v: %q, ждали %q", c.t, got, c.want)
		}
	}
}

// Copy не делит Adj: правка копии не попадает в проект мимо Editor.
func TestCopyDeepTone(t *testing.T) {
	r := PhotoRef{Adj: &Tone{Gray: true, Bright: 10}}
	c := r.Copy()
	c.Adj.Bright = 40
	if r.Adj.Bright != 10 {
		t.Fatalf("копия делит Adj: %+v", r.Adj)
	}
	if (PhotoRef{}).Copy().Adj != nil {
		t.Fatal("пустой Adj стал не nil")
	}
}

// ClampDesign зажимает тон у всех фото, в том числе вне ленты (фото только на фоне), нули — nil.
func TestClampDesignTone(t *testing.T) {
	p := New()
	p.Spreads = []*Spread{{ID: NewID(), Photos: []string{"a", "b"}}}
	p.Photos = map[string]PhotoRef{
		"a":   {W: 3, H: 2, Adj: &Tone{Bright: 900, Contrast: -900}},
		"b":   {W: 3, H: 2, Adj: &Tone{}},
		"far": {W: 3, H: 2, Adj: &Tone{Gray: true, Contrast: 77}},
		"no":  {W: 3, H: 2},
	}
	p.ClampDesign()
	if a := p.Photos["a"].Adj; a == nil || *a != (Tone{Bright: MaxTone, Contrast: -MaxTone}) {
		t.Errorf("a: %+v", a)
	}
	if b := p.Photos["b"].Adj; b != nil {
		t.Errorf("нулевой тон не снят: %+v", b)
	}
	if f := p.Photos["far"].Adj; f == nil || *f != (Tone{Gray: true, Contrast: MaxTone}) {
		t.Errorf("фото вне ленты: %+v", f)
	}
	if p.Photos["no"].Adj != nil {
		t.Error("тон появился у фото без тона")
	}
}

// Фото, оставшееся только на фоне, теряет тон вместе с кадром и зеркалом; фото в месте — нет.
func TestPruneRefsDropsTone(t *testing.T) {
	p := New()
	s := &Spread{ID: NewID(), Photos: []string{"pl", "pl2"}, BgPhotoL: &Backdrop{Photo: "bg", W: 100, H: 100},
		BgPhotoR: &Backdrop{Photo: "bg2", W: 100, H: 100}}
	p.Spreads = []*Spread{s}
	tone := &Tone{Gray: true, Bright: 10}
	p.Photos = map[string]PhotoRef{"pl": {Adj: tone}, "pl2": {Adj: &Tone{Contrast: -5}}, "bg": {Adj: tone},
		"bg2": {Adj: tone, FlipH: true}, "gone": {Adj: tone}}
	p.PruneRefs()
	if _, ok := p.Photos["gone"]; ok || len(p.Photos) != 4 {
		t.Fatalf("PruneRefs: %+v", p.Photos)
	}
	if p.Photos["pl"].Adj == nil || p.Photos["pl2"].Adj == nil {
		t.Fatalf("фото в месте потеряло тон: %+v", p.Photos)
	}
	// bg — только тон (без кадра и зеркала): условие PruneRefs должно видеть сам тон.
	for _, id := range []string{"bg", "bg2"} {
		if r := p.Photos[id]; r.Adj != nil || r.FlipH {
			t.Fatalf("%s только на фоне сохранило тон: %+v", id, r)
		}
	}
}

// В JSON — "adj" с короткими ключами; без тона поля нет.
func TestToneJSON(t *testing.T) {
	data, err := json.Marshal(PhotoRef{Path: "a", Adj: &Tone{Gray: true, Bright: -3, Contrast: 7}})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"path":"a","w":0,"h":0,"adj":{"gray":true,"b":-3,"c":7}}`; string(data) != want {
		t.Fatalf("%s, ждали %s", data, want)
	}
	data, _ = json.Marshal(PhotoRef{Path: "a"})
	if string(data) != `{"path":"a","w":0,"h":0}` {
		t.Fatalf("без тона: %s", data)
	}
}
