package model

import (
	"math"
	"testing"
)

// albumBgProject — два разворота и обложка; фон альбома — фото «bg», которого нет ни на одном листе.
func albumBgProject() *Project {
	p := New()
	p.Photos["bg"] = PhotoRef{Path: "/lib/bg.jpg", W: 4, H: 3, Lib: "my:0123456789abcdef", Frame: &Frame{S: 2}}
	p.Photos["a"] = PhotoRef{Path: "/a.jpg", W: 4, H: 3}
	p.Spreads = []*Spread{{ID: "s1", Photos: []string{"a"}}, {ID: "s2"}}
	p.Cover = &Cover{Sheet: &Spread{ID: CoverID}}
	p.AlbumBg = &AlbumBg{Photo: "bg"}
	return p
}

// TestOnDecorAlbumBg — фото фона альбома держит свою запись (PruneRefs), как фото фона листа, и
// теряет свойства места.
func TestOnDecorAlbumBg(t *testing.T) {
	p := albumBgProject()
	if !p.OnDecor()["bg"] {
		t.Fatal("фото фона альбома не в OnDecor")
	}
	p.PruneRefs()
	if r, ok := p.Photos["bg"]; !ok || r.Frame != nil {
		t.Fatalf("запись фона альбома: %+v, %v", r, ok)
	}
	p.AlbumBg = &AlbumBg{Pat: &Pattern{Kind: "dots"}}
	if p.OnDecor()["bg"] || len(p.OnDecor()) != 0 {
		t.Errorf("узор: %v", p.OnDecor())
	}
	p.PruneRefs()
	if _, ok := p.Photos["bg"]; ok {
		t.Error("запись без дела осталась")
	}
	p.AlbumBg = nil
	if len(p.OnDecor()) != 0 {
		t.Error("без фона альбома")
	}
}

// TestRenameRefsAlbumBg — ссылка фона альбома переименовывается вместе с листами; копия значения,
// а не правка на месте (снимок отмены мог делить указатель).
func TestRenameRefsAlbumBg(t *testing.T) {
	p := albumBgProject()
	was := p.AlbumBg
	p.RenameRefs(map[string]string{"bg": "bg2", "a": "a2"})
	if p.AlbumBg.Photo != "bg2" || was.Photo != "bg" || p.Spreads[0].Photos[0] != "a2" {
		t.Fatalf("фон %q (прежний %q), лист %v", p.AlbumBg.Photo, was.Photo, p.Spreads[0].Photos)
	}
	p.RenameRefs(map[string]string{"x": "y"})
	if p.AlbumBg.Photo != "bg2" {
		t.Error("чужая пара")
	}
	p.AlbumBg = nil
	p.RenameRefs(map[string]string{"": "z"}) // без фона альбома и с пустым ключом — не паника
	p.AlbumBg = &AlbumBg{Pat: &Pattern{Kind: "dots"}}
	p.RenameRefs(map[string]string{"": "z"})
	if p.AlbumBg.Photo != "" {
		t.Error("узор получил фото")
	}
}

// TestEmptyNoAlbumBg — лист со скрытым фоном альбома не пустой: Normalize его не удалит.
func TestEmptyNoAlbumBg(t *testing.T) {
	before := New()
	before.Spreads = []*Spread{{ID: "s", Photos: []string{"a"}}}
	p := New()
	p.Spreads = []*Spread{{ID: "s", NoAlbumBg: true}}
	p.Normalize(before)
	if len(p.Spreads) != 1 {
		t.Fatal("лист с noAlbumBg удалён")
	}
	p.Spreads[0].NoAlbumBg = false
	p.Normalize(before)
	if len(p.Spreads) != 0 {
		t.Fatal("пустой лист остался")
	}
}

// TestAlbumBgOn — фон альбома на листах без своего фона; не на обложке и не на скрытом.
func TestAlbumBgOn(t *testing.T) {
	p := albumBgProject()
	s1, s2 := p.Spreads[0], p.Spreads[1]
	if !p.AlbumBgOn(s1) || !p.AlbumBgOn(s2) || p.AlbumBgOn(p.Cover.Sheet) || p.AlbumBgOn(nil) {
		t.Fatal("листы без своего фона, обложка")
	}
	s1.PatR = &Pattern{Kind: "dots"}
	s2.BgPhotoL = &Backdrop{Photo: "a"}
	if p.AlbumBgOn(s1) || p.AlbumBgOn(s2) {
		t.Error("свой фон на области — фон альбома на всём листе выключен (098-11)")
	}
	s1.PatR, s2.BgPhotoL = nil, nil
	s2.NoAlbumBg = true
	s2.BgL = "#ff0000" // цвет страниц своим фоном не считается
	s1.BgL = "#ff0000"
	if !p.AlbumBgOn(s1) || p.AlbumBgOn(s2) {
		t.Error("скрытый лист или цвет страницы")
	}
	p.AlbumBg = nil
	if p.AlbumBgOn(s1) {
		t.Error("без фона альбома")
	}
}

func TestClampAlbumBg(t *testing.T) {
	p := albumBgProject()
	cases := []struct {
		name  string
		in    *AlbumBg
		sheet bool
		want  *AlbumBg
	}{
		{"нет", nil, false, nil},
		{"пустой", &AlbumBg{}, false, nil},
		{"оба", &AlbumBg{Photo: "bg", Pat: &Pattern{Kind: "dots"}}, false, nil},
		{"фото без записи", &AlbumBg{Photo: "нет"}, false, nil},
		{"негодный узор", &AlbumBg{Pat: &Pattern{Kind: "Bad Kind"}}, false, nil},
		{"узор без кадра", &AlbumBg{Pat: &Pattern{Kind: "dots"}, Frame: &Frame{S: 2}, FlipH: true, Fade: 1.7, Pages: true}, false,
			&AlbumBg{Pat: &Pattern{Kind: "dots"}, Fade: 1, Pages: true}},
		{"фото", &AlbumBg{Photo: "bg", Frame: &Frame{S: 9, X: 3, R: 95.3}, FlipV: true, Fade: math.NaN(), Pages: true}, true,
			&AlbumBg{Photo: "bg", Frame: &Frame{S: MaxScale, X: 1, R: 5.3}, FlipV: true}},
		{"кадр по умолчанию", &AlbumBg{Photo: "bg", Frame: &Frame{S: 1, R: 90}, Fade: 0.123}, false,
			&AlbumBg{Photo: "bg", Fade: 0.12}},
	}
	for _, c := range cases {
		p.Settings.Sheet = c.sheet
		p.AlbumBg = c.in
		p.ClampAlbumBg()
		if !sameAlbumBg(p.AlbumBg, c.want) {
			t.Errorf("%s: %+v, ждали %+v", c.name, p.AlbumBg, c.want)
		}
		if c.in != nil && p.AlbumBg == c.in {
			t.Errorf("%s: зажим правит значение на месте", c.name)
		}
	}
}

func sameAlbumBg(a, b *AlbumBg) bool {
	if a == nil || b == nil {
		return a == b
	}
	return Marshal(&Project{AlbumBg: a}) != nil && string(Marshal(&Project{AlbumBg: a})) == string(Marshal(&Project{AlbumBg: b}))
}
