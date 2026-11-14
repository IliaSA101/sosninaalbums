package roundtrip_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Ключи «Разброса» (спека 105): корень Z(…) с уровнями a…g и лист n<i> — как значения layout.
// Полей в файле нет, меняется только грамматика ключа и номер формата.

// TestScatterKeySaveLoad: Z и n у нескольких разворотов (3 и 5 фото, страницы, центр) переживают
// Save → Load → Save байт-в-байт и действуют (View строит по ключу, как layout.Build); негодный
// Z (6 детей) при чтении остаётся строкой, как прочие негодные ключи, и строится как «Авто».
func TestScatterKeySaveLoad(t *testing.T) {
	dir := testkit.TempDir(t)
	keys := []struct {
		n     int
		key   string
		valid bool // ключ строится (грамматика и Build)
	}{
		{3, "Z(a0,d1,g2)", true}, {3, "H(n0,1,n2)", true}, {3, "P[Z(g0,g1)|Z(a2)]", true},
		{5, "Z(a0,b1,c2,d3,e4)", true}, {4, "Z(g0,a1,g2,a3)", true}, {1, "Z(d0)", true}, {2, "Z(d0)", false}, // d0 на 2 фото: нет фото 1
		{6, "Z(a0,b1,c2,d3,e4,f5)", false}, {3, "Z(h0,a1,a2)", false},
	}
	var names []string
	var spreads []*model.Spread
	for i, k := range keys {
		s := &model.Spread{ID: "s" + string(rune('a'+i)), Layout: k.key}
		for range k.n {
			name := "p" + string(rune('a'+len(names)/26)) + string(rune('a'+len(names)%26))
			names = append(names, name)
			s.Photos = append(s.Photos, name)
		}
		spreads = append(spreads, s)
	}
	_, ids := gridFile(t, dir, names, spreads)
	for _, s := range spreads { // в файле ID фото — от путей
		for i, n := range s.Photos {
			s.Photos[i] = ids[n]
		}
	}
	data, _ := gridFile(t, dir, names, spreads)
	path := filepath.Join(dir, "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	check := func(what string, p *model.Project) {
		t.Helper()
		if len(p.Spreads) != len(keys) {
			t.Fatalf("%s: разворотов %d", what, len(p.Spreads))
		}
		for i, s := range p.Spreads {
			if s.Layout != keys[i].key {
				t.Errorf("%s: разворот %d — ключ %q, ждали %q", what, i, s.Layout, keys[i].key)
			}
		}
	}
	check("Load", p)
	e := edtest.Open(t, p, path)
	for i, s := range e.View().Spreads {
		if len(s.Rects) != len(s.Photos) {
			t.Fatalf("View: разворот %d — %d прямоугольников на %d фото", i, len(s.Rects), len(s.Photos))
		}
		want, ok := layout.Build(keys[i].key, edtest.P(e).Aspects(edtest.P(e).Spreads[i].Photos), edtest.P(e).Params())
		if built := s.Layout == keys[i].key; built != keys[i].valid || ok != keys[i].valid {
			t.Errorf("View: разворот %d — по ключу %v, Build %v, ждали %v (ключ %q)", i, built, ok, keys[i].valid, keys[i].key)
		} else if ok && (len(want.Rects) != len(s.Rects) || want.Rects[0] != s.Rects[0]) {
			t.Errorf("View: разворот %d — Rects не из Build: %+v и %+v", i, s.Rects[0], want.Rects[0])
		}
	}
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	if f := fileJSON(t, path); f.Version != model.FormatVersion {
		t.Fatalf("версия в файле %d, ждали %d", f.Version, model.FormatVersion)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	check("Save → Load", q)
	if err := salbum.Save(q, path); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(path); !bytes.Equal(first, again) {
		t.Fatalf("второй круг файла другой:\n%s\n%s", first, again)
	}
}

// TestScatterSetLayoutUndo: «Разброс» ставится через Editor.SetLayout одним шагом отмены,
// Undo возвращает прежнюю раскладку, повтор того же ключа и негодный Z — холостые (без шага,
// rev тот же), и поставленный ключ переживает Save → Load.
func TestScatterSetLayoutUndo(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	was := e.View().Spreads[0].Layout
	n := len(e.View().Spreads[0].Photos)
	key := scatterKey(n)
	v := e.View()
	undo := edtest.UndoLen(e)
	for _, bad := range []string{"Z()", "Z(h0)", "Z(a0,b1,c2,d3,e4,f5)", "Z(a0"} {
		if got := e.SetLayout(s1, bad); got.Rev != v.Rev || edtest.UndoLen(e) != undo {
			t.Errorf("SetLayout(%q) не холостой", bad)
		}
	}
	got := e.SetLayout(s1, key)
	if got.Spreads[0].Layout != key || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("ключ %q, шагов отмены %d → %d", got.Spreads[0].Layout, undo, edtest.UndoLen(e))
	}
	rev := got.Rev
	if e.SetLayout(s1, key).Rev != rev || edtest.UndoLen(e) != undo+1 {
		t.Error("повторный SetLayout того же ключа не холостой")
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if q.Spreads[0].Layout != key {
		t.Errorf("после Save → Load ключ %q, ждали %q", q.Spreads[0].Layout, key)
	}
	if v := e.Undo(); v.Spreads[0].Layout != was || edtest.UndoLen(e) != undo {
		t.Errorf("Undo: ключ %q, ждали %q; шагов %d", v.Spreads[0].Layout, was, edtest.UndoLen(e))
	}
}

// scatterKey — Z на n фото (2…5): уровни от a вверх, чтобы ключ был непустой и не «в порядке по умолчанию».
func scatterKey(n int) string {
	key := "Z("
	for i := range n {
		if i > 0 {
			key += ","
		}
		key += string(rune('a'+(i*3)%7)) + string(rune('0'+i))
	}
	return key + ")"
}
