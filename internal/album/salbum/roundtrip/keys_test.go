package roundtrip_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Ключи форм раскладки (спека 051), формат v9.

// v8 (0.12) открывается как v9: меняется только номер, первая запись делает .v8.bak с байтами
// исходника; файл новее v9 (как v9 для 0.12) не открывается — ErrNewer.
func TestVersionsV8ToV9(t *testing.T) { oldVersionOpens(t, 8) }

// TestLoadNewKeys: ключи форм 051 у нескольких разворотов переживают Save → Load → Save
// и действуют (View строит по ним); ключ длиннее layout.MaxKeyLen при чтении — «Авто»,
// прочий негодный ключ остаётся (годность зависит от пропорций и настроек).
func TestLoadNewKeys(t *testing.T) {
	dir := testkit.TempDir(t)
	keys := []string{"Q[H(0,1)|H(2,3)]", "QD[0|1]", "H(m0,1,m2)", "L(0,1,2)", "A(0,1,2,3)",
		"G(H(0,1,2),H(3,4))", "U(H(0,1,2,3))", "S(G(0,1))", "P[U(H(0,1))|D(G(2,3))]"}
	long := strings.Repeat("H(", layout.MaxKeyLen) + "0" + strings.Repeat(",1)", layout.MaxKeyLen)
	var names []string
	var spreads []*model.Spread
	addSpread := func(n int, key string) {
		s := &model.Spread{ID: "s" + string(rune('a'+len(spreads))), Layout: key}
		for range n {
			name := "p" + string(rune('a'+len(names)/26)) + string(rune('a'+len(names)%26))
			names = append(names, name)
			s.Photos = append(s.Photos, name)
		}
		spreads = append(spreads, s)
	}
	for _, k := range keys {
		addSpread(len(strings.FieldsFunc(k, func(r rune) bool { return r < '0' || r > '9' })), k)
	}
	addSpread(2, long)
	addSpread(2, "Z(0,1)")
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
	want := append(append([]string{}, keys...), "", "Z(0,1)")
	check := func(what string, p *model.Project) {
		t.Helper()
		if len(p.Spreads) != len(want) {
			t.Fatalf("%s: разворотов %d", what, len(p.Spreads))
		}
		for i, s := range p.Spreads {
			if s.Layout != want[i] {
				t.Errorf("%s: разворот %d — ключ %.40q, ждали %q", what, i, s.Layout, want[i])
			}
		}
	}
	check("Load", p)
	e := edtest.Open(t, p, path)
	for i, s := range e.View().Spreads[:len(keys)] {
		if s.Layout != keys[i] || len(s.Rects) != len(s.Photos) {
			t.Errorf("View: разворот %d по ключу %q, ждали %q", i, s.Layout, keys[i])
		}
	}
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	if f := fileJSON(t, path); f.Version != model.FormatVersion || model.FormatVersion != 19 {
		t.Fatalf("версия в файле %d", f.Version)
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
