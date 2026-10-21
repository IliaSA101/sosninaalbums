package roundtrip_test

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Фото корешка обложки (спека 061), формат v10.

// v9 (0.13) открывается как v10 без потерь: меняется только номер, первая запись делает
// .v9.bak с байтами исходника; файл новее v10 — ErrNewer.
func TestVersionsV9ToV10(t *testing.T) { oldVersionOpens(t, 9) }

// TestLoadV9CoverBoth — обложка 0.13 «Лицевая + задняя» (фикстура v9) открывается как «Задняя +
// корешок + лицевая» с пустым корешком: фото, кадры, плашка и надпись те же, в виде три области
// и пустая зона корешка; Save пишет текущую версию и .v9.bak с исходными байтами, второй круг без потерь.
func TestLoadV9CoverBoth(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "testdata", "v9-cover.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || model.FormatVersion != 16 || edtest.From(p) != 9 {
		t.Fatalf("версия %d, from %d", p.Version, edtest.From(p))
	}
	c := p.Cover
	if c == nil || c.Layout != model.CoverBoth || c.SpineBg != "#203040" || len(c.Sheet.Texts) != 1 || len(c.Sheet.Grid) != 1 {
		t.Fatalf("обложка: %+v", c)
	}
	s := c.Sheet
	l, r := s.BgPhotoL, s.BgPhotoR
	if l == nil || r == nil || s.BgPhoto != nil || s.BgPhotoS != nil || l.Frame == nil || l.Frame.R != -2.5 || !r.FlipH || r.Fade != 0.2 {
		t.Fatalf("фоны обложки: %+v %+v %+v", l, r, s.BgPhotoS)
	}
	if !strings.HasSuffix(p.Photos[l.Photo].Path, "05.jpg") || !strings.HasSuffix(p.Photos[r.Photo].Path, "06.jpg") {
		t.Fatalf("фото фонов: %q %q", p.Photos[l.Photo].Path, p.Photos[r.Photo].Path)
	}
	e := edtest.Open(t, p, path)
	cv := e.View().Cover
	if len(cv.Scopes) != 3 || cv.Scopes[1].Scope != model.ScopeSpine || len(cv.Drop) != 1 || cv.Drop[0].Kind != "spine" {
		t.Fatalf("вид обложки: области %+v, зоны %+v", cv.Scopes, cv.Drop)
	}
	before := model.Marshal(p)
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	if bak, err := os.ReadFile(path + ".v9.bak"); err != nil || !bytes.Equal(bak, src) {
		t.Fatalf(".v9.bak: %v, совпадает с исходником: %v", err, bytes.Equal(bak, src))
	}
	data, _ := os.ReadFile(path)
	if f := fileJSON(t, path); f.Version != 16 || bytes.Contains(data, []byte(`"bgPhotoS"`)) {
		t.Fatalf("файл v%d:\n%s", f.Version, data)
	}
	q, err := salbum.Load(path)
	if err != nil || !bytes.Equal(before, model.Marshal(q)) {
		t.Fatalf("потери v9 → v11: %v\n%s\n%s", err, before, model.Marshal(q))
	}
}

// TestSaveV10Spine — фото корешка с рамкой «Поворот» 90°, кадром, наклоном, зеркалом и
// прозрачностью переживает Save → Load → Save → Load рядом с задней и лицевой.
func TestSaveV10Spine(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a"))
	edtest.NewSpread(e, -1, edtest.Photos("b"))
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SetCoverLayout("both"); err != nil {
		t.Fatal(err)
	}
	for _, bg := range [][2]string{{model.ScopeLeft, "l"}, {model.ScopeRight, "r"}, {model.ScopeSpine, "s"}} {
		edtest.SetBg(t, e, model.CoverID, bg[0], bg[1])
	}
	a := edtest.P(e).BdArea(edtest.P(e).Cover.Sheet, model.ScopeSpine)
	e.SetBackdropBox(model.CoverID, model.ScopeSpine, a.X, 40, 200, 40, 90, "")
	e.SetBackdropFrame(model.CoverID, model.ScopeSpine, 1.6, 0.3, -0.4)
	e.SetBackdropAngle(model.CoverID, model.ScopeSpine, 3.5, "")
	e.FlipBackdrop(model.CoverID, model.ScopeSpine, "v")
	e.SetBackdropFade(model.CoverID, model.ScopeSpine, 0.25, "")
	want := *edtest.P(e).Cover.Sheet.BgPhotoS
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	if f := fileJSON(t, path); f.Version != 16 || !bytes.Contains(first, []byte(`"bgPhotoS"`)) {
		t.Fatalf("файл v%d без фото корешка", f.Version)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got := q.Cover.Sheet.BgPhotoS
	if got == nil || got.Rot != 90 || got.W != 200 || got.H != 40 || *got.Frame != *want.Frame || !got.FlipV || got.Fade != 0.25 ||
		q.Cover.Sheet.BgPhotoL == nil || q.Cover.Sheet.BgPhotoR == nil {
		t.Fatalf("после чтения: %+v, было %+v", got, want)
	}
	// Второй круг (ID фото после первого Load — по путям) ничего не меняет.
	again := filepath.Join(filepath.Dir(path), "b.salbum")
	if err := salbum.Save(q, again); err != nil {
		t.Fatal(err)
	}
	if q2, err := salbum.Load(again); err != nil || !bytes.Equal(model.Marshal(q), model.Marshal(q2)) {
		t.Fatalf("второй круг: %v\n%s\n%s", err, model.Marshal(q), model.Marshal(q2))
	}
}

// TestLoadSpineInvariant — фото корешка из чужого файла: у разворота и у обложки не в both
// снимается со строкой в log.txt (фото без дела уходит из Photos); вместе с одним фото через
// лист — тоже. В both остаётся.
func TestLoadSpineInvariant(t *testing.T) {
	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	src, err := os.ReadFile(filepath.Join("..", "testdata", "v9-cover.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	spine := map[string]any{"photo": "aaaaaaaaaaa7", "x": 306, "y": 0, "w": 6, "h": 326}
	for _, c := range []struct {
		layout, bg    string // раскладка обложки; фото через лист
		cover, spread bool   // фото корешка у обложки, у разворота
		keep          bool
	}{
		{"both", "", true, true, true},
		{"", "", true, false, false},
		{"wrap", "", true, false, false},
		{"both", "aaaaaaaaaaa5", true, false, false},
	} {
		var f map[string]any
		if err := json.Unmarshal(src, &f); err != nil {
			t.Fatal(err)
		}
		f["version"] = 10
		cv := f["cover"].(map[string]any)
		cv["layout"] = c.layout
		sh := cv["sheet"].(map[string]any)
		if c.cover {
			sh["bgPhotoS"] = spine
		}
		if c.bg != "" {
			sh["bgPhoto"] = map[string]any{"photo": c.bg, "x": 0, "y": 0, "w": 624, "h": 326}
		}
		if c.spread {
			f["spreads"].([]any)[0].(map[string]any)["bgPhotoS"] = spine
		}
		f["photos"].(map[string]any)["aaaaaaaaaaa7"] = map[string]any{"path": "Отбор/07.jpg", "w": 3000, "h": 2000}
		data, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		buf.Reset()
		p := edtest.MustDecode(t, data, testkit.TempDir(t))
		if got := p.Cover.Sheet.BgPhotoS != nil; got != c.keep {
			t.Errorf("%q/%q: фото корешка у обложки %v, ждали %v", c.layout, c.bg, got, c.keep)
		}
		if p.Spreads[0].BgPhotoS != nil {
			t.Errorf("%q: фото корешка у разворота", c.layout)
		}
		onSpine := false
		for _, ref := range p.Photos {
			onSpine = onSpine || strings.HasSuffix(ref.Path, "07.jpg")
		}
		if onSpine != c.keep {
			t.Errorf("%q/%q: фото корешка в Photos %v, ждали %v", c.layout, c.bg, onSpine, c.keep)
		}
		if logged := strings.Contains(buf.String(), "фото корешка снято"); logged != (c.spread || !c.keep && c.bg == "") {
			t.Errorf("%q/%q: строка в log.txt %v:\n%s", c.layout, c.bg, logged, buf.String())
		}
	}
}
