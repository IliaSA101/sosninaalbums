package salbum_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Golden v3 (testdata/v3.salbum.json, без зеркала) открывается как текущая версия без потерь:
// номер меняется, данные те же; первая запись делает .v3.bak и пишет FormatVersion.
func TestLoadV3AsV4(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "v3.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || edtest.From(p) != 3 || len(p.Spreads) != 2 || len(p.Photos) != 3 {
		t.Fatalf("версия %d, from %d, развороты %d, фото %d", p.Version, edtest.From(p), len(p.Spreads), len(p.Photos))
	}
	for id, r := range p.Photos {
		if r.FlipH || r.FlipV {
			t.Fatalf("%s: зеркало из v3", id)
		}
	}
	// ID фото пересчитываются по путям (rekey).
	a := p.Photos[library.IDFor(filepath.ToSlash(dir)+"/Отбор/a.jpg")]
	if a.Frame == nil || a.Frame.S != 1.35 || a.Caption == nil || a.Caption.Text != "Первый танец" ||
		p.Spreads[0].Name != "Сборы" || len(p.Spreads[0].Texts) != 1 || len(p.Spreads[1].Grid) != 2 {
		t.Fatalf("данные v3 потеряны: %+v %+v", a, p.Spreads[0])
	}
	e := edtest.Open(t, p, path)
	if !edtest.Backup(e) {
		t.Fatal("запись поверх v3 должна делать бэкап")
	}
	before := model.Marshal(p)
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".v3.bak"); err != nil {
		t.Fatalf("нет .v3.bak: %v", err)
	}
	if f := fileJSON(t, path); f.Version != model.FormatVersion {
		t.Fatalf("версия в файле %d", f.Version)
	}
	if saved, _ := os.ReadFile(path); bytes.Contains(saved, []byte("flip")) {
		t.Fatalf("в файле без зеркала появились flip-поля:\n%s", saved)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, model.Marshal(q)) {
		t.Fatalf("потери при v3 → текущая:\n%s\n%s", before, model.Marshal(q))
	}
}

// Круг v4: зеркало по обеим осям переживает запись и чтение, пишется только когда включено.
func TestFileV4RoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	ph := filepath.Join(dir, "Отбор", "a.jpg")
	edtest.PutFile(t, ph, 10)
	p, id := onePhoto(ph)
	p.Folders = []string{filepath.Join(dir, "Отбор")}
	r := p.Photos[id]
	r.Size, r.Frame, r.FlipH, r.FlipV = 10, &model.Frame{S: 2, X: -0.5, Y: 0.25}, true, true
	p.Photos[id] = r
	other := filepath.Join(dir, "Отбор", "b.jpg")
	edtest.PutFile(t, other, 10)
	idb := library.IDFor(other)
	p.Photos[idb] = model.PhotoRef{Path: other, W: 200, H: 300, Size: 10, FlipV: true}
	p.Spreads[0].Photos = append(p.Spreads[0].Photos, idb)
	path := filepath.Join(dir, "a.salbum")
	if err := salbum.Save(p, path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if bytes.Count(data, []byte(`"flipH": true`)) != 1 || bytes.Count(data, []byte(`"flipV": true`)) != 2 {
		t.Fatalf("флаги в файле:\n%s", data)
	}
	if f := fileJSON(t, path); f.Version != model.FormatVersion {
		t.Fatalf("версия %d", f.Version)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Photos[id]; !got.FlipH || !got.FlipV || got.Frame == nil || *got.Frame != *r.Frame {
		t.Fatalf("фото a: %+v", got)
	}
	if got := q.Photos[idb]; got.FlipH || !got.FlipV {
		t.Fatalf("фото b: %+v", got)
	}
	if a, b := model.Marshal(p), model.Marshal(q); !bytes.Equal(a, b) {
		t.Fatalf("потери при записи:\n%s\n%s", a, b)
	}
	if edtest.From(q) != model.FormatVersion || edtest.Backup(edtest.Open(t, q, path)) {
		t.Fatal("файл текущей версии не требует бэкапа")
	}
}

// ClampSettings зажимает и цвет страниц: не #rrggbb — белый.
func TestClampSettingsBg(t *testing.T) {
	s := model.DefaultSettings()
	for in, want := range map[string]string{"#AABBCC": "#aabbcc", "#FFF": "", "url(x)": "", "": ""} {
		s.Bg = in
		if got := model.ClampSettings(s).Bg; got != want {
			t.Errorf("Bg %q → %q, ждали %q", in, got, want)
		}
	}
	e := edtest.Open(t, model.New(), "")
	s.Bg = "javascript:1"
	if v := e.SetSettings(s); v.Settings.Bg != "" {
		t.Fatalf("SetSettings не зажал Bg: %q", v.Settings.Bg)
	}
}

// Golden 0.4.0 (testdata/v4-dev.salbum.json) открывается без потерь (с 017 — v4 → v5 с бэкапом);
// запись не добавляет полей 0.5.0 местам; круг Save/Load — те же данные.
func TestLoadV4Dev(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "v4-dev.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f salbum.FileProject
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if edtest.From(p) != 4 || !edtest.Backup(edtest.Open(t, p, path)) || len(p.Spreads) != 2 || len(p.Photos) != 4 {
		t.Fatalf("from %d, развороты %d, фото %d", edtest.From(p), len(p.Spreads), len(p.Photos))
	}
	s0, w0 := p.Spreads[0], f.Spreads[0]
	if s0.Name != w0.Name || s0.BgL != w0.BgL || len(s0.Texts) != 1 || *s0.Texts[0] != *w0.Texts[0] {
		t.Fatalf("разворот 1: %+v", s0)
	}
	bd, wbd := *s0.BgPhotoR, *w0.BgPhotoR
	if bd.Frame == nil || *bd.Frame != *wbd.Frame || bd.Photo == "" {
		t.Fatalf("фон: %+v", bd)
	}
	bd.Photo, bd.Frame, wbd.Photo, wbd.Frame = "", nil, "", nil
	if bd != wbd {
		t.Fatalf("фон: %+v, ждали %+v", bd, wbd)
	}
	g, wg := p.Spreads[1].Grid, f.Spreads[1].Grid
	if len(g) != len(wg) {
		t.Fatalf("мест %d", len(g))
	}
	for i, pl := range g {
		want := *wg[i]
		want.Photo = pl.Photo // ID фото пересчитаны по путям (rekey)
		if *pl != want || (pl.Photo == "") != (wg[i].Photo == "") {
			t.Fatalf("место %d: %+v, ждали %+v", i, *pl, *wg[i])
		}
	}
	ref := func(n string) model.PhotoRef {
		return p.Photos[library.IDFor(filepath.ToSlash(dir)+"/Отбор/"+n+".jpg")]
	}
	if a := ref("a"); a.Frame == nil || a.Frame.S != 1.35 || a.Caption == nil || !a.Caption.Over || a.Caption.Text != "Первый танец" {
		t.Fatalf("фото a: %+v", a)
	}
	if b, c := ref("b"), ref("c"); !b.FlipV || b.FlipH || !c.FlipH || c.Frame == nil || c.Frame.S != 2 {
		t.Fatalf("фото b, c: %+v %+v", b, c)
	}
	before := model.Marshal(p)
	if err := salbum.Save(p, path); err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(path)
	var raw struct {
		Spreads []struct {
			Grid []map[string]json.RawMessage `json:"grid"`
		} `json:"spreads"`
	}
	if err := json.Unmarshal(saved, &raw); err != nil {
		t.Fatal(err)
	}
	old := []string{"id", "x", "y", "w", "h", "photo", "fit"}
	for _, s := range raw.Spreads {
		for _, pl := range s.Grid {
			for k := range pl {
				if !slices.Contains(old, k) {
					t.Fatalf("у места 0.4.0 появилось поле %q:\n%s", k, saved)
				}
			}
		}
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, model.Marshal(q)) {
		t.Fatalf("потери при круге:\n%s\n%s", before, model.Marshal(q))
	}
}

// Первая запись поверх v2 копирует исходник в .v2.bak, один раз.
func TestBackupOnFirstSaveV2(t *testing.T) {
	dir := testkit.TempDir(t)
	path := writeV2(t, dir, salbum.FilePhoto{PhotoRef: model.PhotoRef{Path: "a.jpg", W: 3, H: 2}})
	orig, _ := os.ReadFile(path)
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	e := edtest.Open(t, p, path)
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	bak := path + ".v2.bak"
	if got, _ := os.ReadFile(bak); !bytes.Equal(got, orig) {
		t.Fatal("в .v2.bak не исходные байты")
	}
	if fileJSON(t, path).Version != model.FormatVersion {
		t.Fatal("файл не записан в текущей версии")
	}
	os.WriteFile(bak, []byte("метка"), 0o644)
	edtest.NewSpread(e, -1, edtest.Photos("x"))
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(bak); string(got) != "метка" {
		t.Fatal("копия перезаписана")
	}
	if _, err := os.Stat(path + ".v1.bak"); err == nil {
		t.Fatal("лишняя .v1.bak")
	}
}

// Поля 008 в v3: название, угол, «по тексту» переживают запись и чтение.
func TestSaveLoadV3Names(t *testing.T) {
	p := model.New() // без фото: их пути в файле проверяет Load
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{}}}
	e, s1 := edtest.Open(t, p, ""), "s1"
	e.SetSpreadName(s1, "Сборы")
	a := edtest.AddText(t, e, s1, 0)
	b := edtest.AddText(t, e, s1, 1)
	e.SetTextAngle(a, 30)
	e.SetTextBox(b, 320, 40, 150)
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	for _, s := range []string{`"name": "Сборы"`, `"rot": 30`, `"auto": true`} {
		if !strings.Contains(string(data), s) {
			t.Fatalf("в файле нет %s", s)
		}
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || model.FormatVersion != 5 {
		t.Fatalf("версия %d", p.Version)
	}
	ta, tb := *edtest.P(e).Spreads[0].Texts[0], *edtest.P(e).Spreads[0].Texts[1]
	got := p.Spreads[0]
	if got.Name != "Сборы" || *got.Texts[0] != ta || *got.Texts[1] != tb || tb.Auto || !ta.Auto || ta.Rot != 30 {
		t.Fatalf("после чтения: %q %+v %+v", got.Name, *got.Texts[0], *got.Texts[1])
	}
}

// Load чинит Order последним шагом: после открытия холостая правка остаётся холостой.
func TestLoadOrderRepair(t *testing.T) {
	dir := testkit.TempDir(t)
	_, ids := gridFile(t, dir, []string{"a", "b", "c"}, nil)
	tb := func(id string) *model.TextBlock {
		return &model.TextBlock{ID: id, X: 20, Y: 20, W: 60, Text: "Текст", Style: model.TextStyle{Size: 20}}
	}
	const p1, p2, p3, t1, t2 = "c0ffee000001", "c0ffee000002", "c0ffee000003", "c0ffee0000a1", "c0ffee0000a2"
	junk := slices.Repeat([]string{"нет"}, model.MaxPlaces+model.MaxTexts)
	data, _ := gridFile(t, dir, []string{"a", "b", "c"}, []*model.Spread{
		{ID: "aaaaaaaaaaa1", Photos: []string{ids["a"], ids["b"]}, Texts: []*model.TextBlock{tb(t1), tb(t2)},
			Grid:  []*model.Place{box(p1, 10, 10, 100, 100, ids["a"]), box(p2, 60, 60, 100, 100, ids["b"]), box(p3, 300, 10, 50, 50, "")},
			Order: []string{"нет", t1, p2, t1, t2, p2}},
		// «Авто» с Order — Order выбрасывается.
		{ID: "aaaaaaaaaaa2", Photos: []string{ids["c"]}, Texts: []*model.TextBlock{tb("c0ffee0000b1")},
			Order: []string{"c0ffee0000b1"}},
		// Огромный Order обрезается до починки: верные ID за мусором не доживают.
		{ID: "aaaaaaaaaaa3", Photos: []string{}, Texts: []*model.TextBlock{tb("c0ffee0000c1")},
			Grid:  []*model.Place{box("c0ffee000004", 10, 10, 50, 50, "")},
			Order: append(junk, "c0ffee0000c1", "c0ffee000004")},
	})
	p := edtest.MustDecode(t, data, dir)
	if got, want := p.Spreads[0].Order, []string{t1, p1, p2, p3, t2}; !slices.Equal(got, want) {
		t.Fatalf("Order %v, ждали %v", got, want)
	}
	if p.Spreads[1].Order != nil || p.Spreads[2].Order != nil {
		t.Fatalf("Order «Авто» %v, обрезанный %v", p.Spreads[1].Order, p.Spreads[2].Order)
	}
	e := edtest.Open(t, p, "")
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	edtest.Change(e, func(*model.Project) {})
	edtest.Same(t, e, rev, undo, "первая правка после открытия")
	// Второй круг файла ничего не меняет.
	again, err := salbum.Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	if q := edtest.MustDecode(t, again, dir); !bytes.Equal(model.Marshal(p), model.Marshal(q)) {
		t.Fatalf("второй круг:\n%s\n%s", model.Marshal(p), model.Marshal(q))
	}
}
