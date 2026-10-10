package salbum

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// gridInvariants — инварианты сетки (design 007 §1): после Load и после каждой правки.
func gridInvariants(p *model.Project) error {
	sw, ph := 2*p.Settings.PageW, p.Settings.PageH
	placeIDs, inAlbum := map[string]bool{}, map[string]bool{}
	on2 := func(v float64) bool { return math.Abs(v*100-math.Round(v*100)) < 1e-6 }
	for i, s := range p.Spreads {
		for _, id := range s.Photos {
			if inAlbum[id] {
				return fmt.Errorf("разворот %d: фото %s стоит дважды", i, id)
			}
			inAlbum[id] = true
		}
		if err := orderInvariant(s); err != nil {
			return fmt.Errorf("разворот %d: %w", i, err)
		}
		if s.Grid == nil {
			continue
		}
		if len(s.Grid) == 0 || len(s.Grid) > model.MaxPlaces {
			return fmt.Errorf("разворот %d: мест %d", i, len(s.Grid))
		}
		if !slices.Equal(s.Photos, model.GridPhotos(s)) {
			return fmt.Errorf("разворот %d: Photos %v не по местам %v", i, s.Photos, model.GridPhotos(s))
		}
		for _, pl := range s.Grid {
			switch {
			case pl == nil:
				return fmt.Errorf("разворот %d: nil-место", i)
			case !model.IsTextID(pl.ID) || placeIDs[pl.ID]:
				return fmt.Errorf("разворот %d: ID места %q", i, pl.ID)
			case pl.Fit != "" && pl.Fit != model.FitContain:
				return fmt.Errorf("разворот %d: fit %q", i, pl.Fit)
			case pl.Photo != "" && !hasRef(p, pl.Photo):
				return fmt.Errorf("разворот %d: фото места без записи %s", i, pl.Photo)
			case pl.W < geom.MinPlace || pl.H < geom.MinPlace || !holdInside(pl, sw, ph):
				return fmt.Errorf("разворот %d: место вне разворота %+v", i, *pl)
			case !on2(pl.X) || !on2(pl.Y) || !on2(pl.W) || !on2(pl.H) || geom.NormDeg(pl.Rot) != pl.Rot:
				return fmt.Errorf("разворот %d: место не округлено %+v", i, *pl)
			}
			placeIDs[pl.ID] = true
		}
	}
	return nil
}

// orderInvariant — Order: nil у «Авто» и при порядке по умолчанию, иначе перестановка
// мест и текстов; Grid и Texts — в его относительном порядке.
func orderInvariant(s *model.Spread) error {
	if s.Order == nil {
		return nil
	}
	if len(s.Grid) == 0 {
		return fmt.Errorf("Order у «Авто»: %v", s.Order)
	}
	var grid, texts []string
	for _, pl := range s.Grid {
		grid = append(grid, pl.ID)
	}
	for _, t := range s.Texts {
		texts = append(texts, t.ID)
	}
	if slices.Equal(s.Order, append(slices.Clone(grid), texts...)) {
		return fmt.Errorf("Order по умолчанию не nil: %v", s.Order)
	}
	kind := s.Kinds()
	var og, ot []string
	for _, id := range s.Order {
		switch kind[id] {
		case model.LayerPlace:
			og = append(og, id)
		case model.LayerText:
			ot = append(ot, id)
		}
	}
	if len(s.Order) != len(grid)+len(texts) || !slices.Equal(og, grid) || !slices.Equal(ot, texts) {
		return fmt.Errorf("Order %v не перестановка мест %v и текстов %v в их порядке", s.Order, grid, texts)
	}
	return nil
}

// holdInside — hold места внутри разворота sw×ph (у ровного — само место).
func holdInside(pl *model.Place, sw, ph float64) bool {
	h := model.Hold(pl)
	return h.X >= -1e-6 && h.Y >= -1e-6 && h.X+h.W <= sw+1e-6 && h.Y+h.H <= ph+1e-6
}

func hasRef(p *model.Project, id string) bool { _, ok := p.Photos[id]; return ok }

// gridFile — файл v3 с фото по путям dir/<имя>.jpg (ID = IDFor) и разворотами.
func gridFile(t *testing.T, dir string, names []string, spreads []*model.Spread) ([]byte, map[string]string) {
	t.Helper()
	ids := map[string]string{}
	f := FileProject{Version: model.FormatVersion, Settings: model.DefaultSettings(), Folders: []FileFolder{},
		Spreads: spreads, Photos: map[string]FilePhoto{}}
	for _, n := range names {
		path := filepath.Join(dir, n+".jpg")
		ids[n] = library.IDFor(path)
		f.Photos[ids[n]] = FilePhoto{PhotoRef: model.PhotoRef{Path: path, W: 300, H: 200}}
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return data, ids
}

func mustDecode(t *testing.T, data []byte, dir string) *model.Project {
	t.Helper()
	p, err := Decode(data, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := gridInvariants(p); err != nil {
		t.Fatal(err)
	}
	return p
}

func box(id string, x, y, w, h float64, photo string) *model.Place {
	return &model.Place{ID: id, X: x, Y: y, W: w, H: h, Photo: photo}
}

// Файл v2 открывается как v3 без мест; запись — уже v3, поля на месте.
func TestLoadV2Migrates(t *testing.T) {
	dir := testkit.TempDir(t)
	path := writeV2(t, dir, FilePhoto{PhotoRef: model.PhotoRef{Path: "a.jpg", W: 3, H: 2, Frame: &model.Frame{S: 2}}})
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || p.From() != 2 || len(p.Spreads) != 1 || p.Spreads[0].Grid != nil {
		t.Fatalf("версия %d, from %d, развороты %+v", p.Version, p.From(), p.Spreads)
	}
	if r := onlyRef(t, p); r.Frame == nil || r.Frame.S != 2 {
		t.Fatalf("кадр потерян: %+v", r)
	}
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	if f := fileJSON(t, path); f.Version != model.FormatVersion {
		t.Fatalf("версия в файле %d", f.Version)
	}
	if data, _ := os.ReadFile(path); bytes.Contains(data, []byte(`"grid"`)) {
		t.Fatalf("пустая сетка попала в файл:\n%s", data)
	}
}

// Зажим мест: NaN/Inf — место выброшено, вне разворота и < 5 мм — внутрь, плохой
// и повторный ID — новый, чужое и повторное фото — пустое место, > 16 мест — лишние пустые с конца.
func TestClampPlaces(t *testing.T) {
	p := model.New()
	p.Photos["a"] = model.PhotoRef{Path: "/a.jpg", W: 3, H: 2}
	s := &model.Spread{ID: "s", Photos: []string{"a"}, Grid: []*model.Place{
		box("c0ffee000001", math.NaN(), 0, 10, 10, ""),
		box("c0ffee000002", -50, 10, 5000, 1, "a"),
		box("c0ffee000002", 590, 298.004, 20, 20, "a"),
		box("C0FFEE000003", 10.123456, 10, 10, math.Inf(1), ""),
		box("zz", 10.123456, 10, 10, 10, "нет"),
	}}
	s.Grid[1].Fit, s.Grid[4].Fit = "cover", model.FitContain
	p.Spreads = []*model.Spread{s}
	p.ClampGrid()
	g := s.Grid
	if len(g) != 3 {
		t.Fatalf("мест %d, ждали 3 (NaN и Inf выброшены)", len(g))
	}
	if *g[0] != (model.Place{ID: "c0ffee000002", X: 0, Y: 10, W: 600, H: 5, Photo: "a"}) {
		t.Errorf("место 1: %+v", *g[0])
	}
	if g[1].ID == "c0ffee000002" || g[1].Photo != "" || g[1].X != 580 || g[1].Y != 280 {
		t.Errorf("место 2 (повтор ID и фото): %+v", *g[1])
	}
	if g[2].ID == "zz" || g[2].Photo != "" || g[2].X != 10.12 || g[2].Fit != model.FitContain {
		t.Errorf("место 3: %+v", *g[2])
	}
	again := p.Clone()
	again.ClampGrid()
	if !bytes.Equal(model.Marshal(again), model.Marshal(p)) {
		t.Error("clampGrid не идемпотентна")
	}

	// Больше MaxPlaces: сначала уходят пустые с конца.
	q := model.New()
	big := &model.Spread{ID: "b"}
	for i := range model.MaxPlaces + 4 {
		big.Grid = append(big.Grid, box(model.NewID(), float64(i), 0, 10, 10, ""))
	}
	for _, i := range []int{0, model.MaxPlaces + 1, model.MaxPlaces + 3} {
		id := fmt.Sprint("f", i)
		q.Photos[id] = model.PhotoRef{Path: "/" + id, W: 1, H: 1}
		big.Grid[i].Photo = id
		big.Photos = append(big.Photos, id)
	}
	q.Spreads = []*model.Spread{big}
	q.ClampGrid()
	if len(big.Grid) != model.MaxPlaces || !slices.Equal(model.GridPhotos(big), big.Photos) {
		t.Fatalf("мест %d, фото в местах %v", len(big.Grid), model.GridPhotos(big))
	}
}

// Починка при чтении: повтор ID и фото, фото без места — новое место, больше
// MaxPlaces мест; сетка с > MaxPhotos фото режется как «Авто».
func TestLoadRepairsGrid(t *testing.T) {
	dir := testkit.TempDir(t)
	names := []string{"a", "b", "c", "d", "e"}
	for i := range model.MaxPhotos + 2 {
		names = append(names, fmt.Sprint("g", i))
	}
	_, ids := gridFile(t, dir, names, nil)
	s1 := &model.Spread{ID: "s1", Photos: []string{ids["a"], ids["b"], ids["c"]}, Grid: []*model.Place{
		box("c0ffee000001", 10, 10, 100, 100, ids["a"]),
		box("c0ffee000001", 120, 10, 100, 100, ids["a"]),
		box("c0ffee000002", 10, 120, 1, 1, ""),
	}}
	s2 := &model.Spread{ID: "s2", Photos: []string{ids["d"], ids["e"]}, Grid: []*model.Place{box("c0ffee000003", 10, 10, 100, 100, ids["d"])}}
	s3 := &model.Spread{ID: "s3"}
	for i := range model.MaxPhotos + 2 {
		s3.Grid = append(s3.Grid, box(model.NewID(), 0, 0, 10, 10, ids[fmt.Sprint("g", i)]))
	}
	data, _ := gridFile(t, dir, names, []*model.Spread{s1, s2, s3})
	p := mustDecode(t, data, dir)

	g := p.Spreads[0].Grid
	if len(g) != 3 || g[0].Photo != ids["a"] || g[1].ID == "c0ffee000001" || g[1].Photo != ids["b"] || g[2].Photo != ids["c"] || g[2].W != geom.MinPlace {
		t.Fatalf("s1: %+v %+v %+v", *g[0], *g[1], *g[2])
	}
	g = p.Spreads[1].Grid
	if len(g) != 2 || g[1].Photo != ids["e"] || g[1].X <= 0 || g[1].X+g[1].W > p.Settings.PageW {
		t.Fatalf("s2: новое место для e на левой странице: %+v", g)
	}
	if len(p.Spreads) != 4 || p.Spreads[2].Grid != nil || len(p.Spreads[2].Photos) != model.MaxPhotos || len(p.Spreads[3].Photos) != 2 {
		t.Fatalf("сетка с %d фото должна стать «Авто» и разрезаться: %d разворотов", model.MaxPhotos+2, len(p.Spreads))
	}
}

// Объединение: фото только в месте — в Photos (в порядке мест), место с фото
// без записи — пустое.
func TestLoadUnionPlaces(t *testing.T) {
	dir := testkit.TempDir(t)
	_, ids := gridFile(t, dir, []string{"a", "b"}, nil)
	data, _ := gridFile(t, dir, []string{"a", "b"}, []*model.Spread{{ID: "s", Photos: []string{ids["a"]}, Grid: []*model.Place{
		box("c0ffee000001", 10, 10, 100, 100, ids["b"]),
		box("c0ffee000002", 120, 10, 100, 100, ids["a"]),
		box("c0ffee000003", 230, 10, 100, 100, "нет-записи"),
	}}})
	p := mustDecode(t, data, dir)
	s := p.Spreads[0]
	if !slices.Equal(s.Photos, []string{ids["b"], ids["a"]}) || s.Grid[2].Photo != "" || len(p.Photos) != 2 {
		t.Fatalf("Photos %v, третье место %q, записей %d", s.Photos, s.Grid[2].Photo, len(p.Photos))
	}
}

// `"grid":[null]` не роняет разбор: nil выбрасывается до всего остального.
func TestLoadGridNull(t *testing.T) {
	dir := testkit.TempDir(t)
	path, _ := json.Marshal(filepath.Join(dir, "p.jpg"))
	for _, raw := range []string{
		`{"version":3,"spreads":[{"id":"s","photos":[],"grid":[null]}]}`,
		`{"version":3,"spreads":[{"id":"s","photos":[],"grid":null}]}`,
		`{"version":3,"spreads":[{"id":"s","photos":["p"],"grid":[null,{"id":"c0ffee000001","x":10,"y":10,"w":50,"h":50,"photo":"p"},null]}],
"photos":{"p":{"path":` + string(path) + `,"w":3,"h":2}}}`,
	} {
		p, err := Decode([]byte(raw), dir)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if err := gridInvariants(p); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if strings.Contains(raw, `"photo":"p"`) && (len(p.Spreads[0].Grid) != 1 || len(p.Spreads[0].Photos) != 1) {
			t.Fatalf("место с фото потеряно: %+v", p.Spreads[0])
		}
	}
}

// Альбом переехал: ID приводятся к путям, фото остаются в своих местах
// (P1=logo P2="" P3=date), геометрия не меняется.
func TestRekeyKeepsPlaces(t *testing.T) {
	dir := testkit.TempDir(t)
	logo, date := filepath.Join(dir, "logo.png"), filepath.Join(dir, "date.png")
	grid := []*model.Place{
		box("c0ffee000001", 10, 10, 50, 50, "logo"),
		box("c0ffee000002", 70, 10, 50, 50, ""),
		box("c0ffee000003", 130, 10, 50, 50, "date"),
	}
	f := FileProject{Version: model.FormatVersion, Settings: model.DefaultSettings(), Folders: []FileFolder{},
		Spreads: []*model.Spread{{ID: "s", Photos: []string{"logo", "date"}, Grid: grid}},
		Photos: map[string]FilePhoto{
			"logo": {PhotoRef: model.PhotoRef{Path: logo, W: 1, H: 1}},
			"date": {PhotoRef: model.PhotoRef{Path: date, W: 1, H: 1}},
		}}
	data, _ := json.Marshal(f)
	p := mustDecode(t, data, dir)
	g := p.Spreads[0].Grid
	nl, nd := library.IDFor(logo), library.IDFor(date)
	if len(g) != 3 || g[0].Photo != nl || g[1].Photo != "" || g[2].Photo != nd || g[2].X != 130 {
		t.Fatalf("места: %+v %+v %+v", *g[0], *g[1], *g[2])
	}
}

// Две записи одного пути сливаются: второе место пустеет, но остаётся.
func TestRekeyMergeEmptiesPlace(t *testing.T) {
	dir := testkit.TempDir(t)
	a := filepath.Join(dir, "a.jpg")
	f := FileProject{Version: model.FormatVersion, Settings: model.DefaultSettings(), Folders: []FileFolder{},
		Spreads: []*model.Spread{{ID: "s", Photos: []string{"x", "y"}, Grid: []*model.Place{
			box("c0ffee000001", 10, 10, 50, 50, "x"),
			box("c0ffee000002", 70, 10, 50, 50, "y"),
		}}},
		Photos: map[string]FilePhoto{
			"x": {PhotoRef: model.PhotoRef{Path: a, W: 3, H: 2, Frame: &model.Frame{S: 2}}},
			"y": {PhotoRef: model.PhotoRef{Path: a, W: 3, H: 2}},
		}}
	data, _ := json.Marshal(f)
	p := mustDecode(t, data, dir)
	s, id := p.Spreads[0], library.IDFor(a)
	if len(s.Grid) != 2 || s.Grid[0].Photo != id || s.Grid[1].Photo != "" || s.Grid[1].X != 70 || !slices.Equal(s.Photos, []string{id}) {
		t.Fatalf("сетка %+v %+v, фото %v", *s.Grid[0], *s.Grid[1], s.Photos)
	}
	if fr := p.Photos[id].Frame; fr == nil || fr.S != 2 {
		t.Fatalf("кадр от первой записи: %+v", fr)
	}
}

// dedupe: повтор фото в месте — пустое место (перед занятым), сетка, опустевшая
// от этого, не удаляется.
func TestDedupeKeepsGrid(t *testing.T) {
	dir := testkit.TempDir(t)
	_, ids := gridFile(t, dir, []string{"a", "b"}, nil)
	a, b := ids["a"], ids["b"]
	data, _ := gridFile(t, dir, []string{"a", "b"}, []*model.Spread{
		{ID: "s1", Photos: []string{a}},
		{ID: "s2", Photos: []string{a, b}, Grid: []*model.Place{box("c0ffee000001", 10, 10, 50, 50, a), box("c0ffee000002", 70, 10, 50, 50, b)}},
		{ID: "s3", Photos: []string{a}, Grid: []*model.Place{box("c0ffee000003", 10, 10, 50, 50, a)}},
	})
	p := mustDecode(t, data, dir)
	if len(p.Spreads) != 3 {
		t.Fatalf("разворотов %d, ждали 3", len(p.Spreads))
	}
	s2, s3 := p.Spreads[1], p.Spreads[2]
	if s2.Grid[0].Photo != "" || s2.Grid[1].Photo != b || !slices.Equal(s2.Photos, []string{b}) {
		t.Fatalf("s2: %+v %+v", *s2.Grid[0], *s2.Grid[1])
	}
	if len(s3.Grid) != 1 || s3.Grid[0].Photo != "" || len(s3.Photos) != 0 {
		t.Fatalf("s3: %+v", s3)
	}
}

// ID текста, совпавший с ID места (подделанный файл), — общий seen: место получает новый.
func TestLoadTextPlaceSameID(t *testing.T) {
	dir := testkit.TempDir(t)
	_, ids := gridFile(t, dir, []string{"a"}, nil)
	const same = "c0ffee000001"
	data, _ := gridFile(t, dir, []string{"a"}, []*model.Spread{
		{ID: "aaaaaaaaaaa1", Photos: []string{ids["a"]},
			Texts: []*model.TextBlock{{ID: same, X: 20, Y: 20, W: 60, Text: "Текст", Style: model.TextStyle{Size: 20}}},
			Grid:  []*model.Place{box(same, 10, 10, 100, 100, ids["a"])}, Order: []string{same}},
	})
	p := mustDecode(t, data, dir)
	s := p.Spreads[0]
	if s.Texts[0].ID != same || s.Grid[0].ID == same || !model.IsTextID(s.Grid[0].ID) {
		t.Fatalf("текст %s, место %s", s.Texts[0].ID, s.Grid[0].ID)
	}
	// Order [текст]: место недостающее — вниз, под текст; это порядок по умолчанию.
	if s.Order != nil {
		t.Fatalf("Order %v", s.Order)
	}
}

// Круг файла с полями 0.5.0: product, rot места, r кадра, order.
func TestFileV4Fields010RoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	_, ids := gridFile(t, dir, []string{"a", "b"}, nil)
	const p1, p2, t1 = "c0ffee000001", "c0ffee000002", "c0ffee0000a1"
	data, _ := gridFile(t, dir, []string{"a", "b"}, []*model.Spread{
		{ID: "aaaaaaaaaaa1", Photos: []string{ids["a"], ids["b"]},
			Texts: []*model.TextBlock{{ID: t1, X: 20, Y: 20, W: 60, Text: "Текст", Style: model.TextStyle{Size: 20}}},
			Grid: []*model.Place{{ID: p1, X: 100, Y: 80, W: 100, H: 60, Photo: ids["a"], Rot: 30},
				box(p2, 300, 80, 100, 100, ids["b"])},
			Order: []string{p1, t1, p2}},
	})
	p := mustDecode(t, data, dir)
	r := p.Photos[ids["a"]]
	r.Frame = &model.Frame{S: 1.5, X: 0.25, R: 120}
	p.Photos[ids["a"]] = r
	out, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"product": "album"`, `"rot": 30`, `"r": 120`, `"order": [`} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("в файле нет %s:\n%s", want, out)
		}
	}
	q := mustDecode(t, out, dir)
	if !bytes.Equal(model.Marshal(p), model.Marshal(q)) {
		t.Fatalf("потери при круге:\n%s\n%s", model.Marshal(p), model.Marshal(q))
	}
	if got := q.Spreads[0].Order; !slices.Equal(got, []string{p1, t1, p2}) {
		t.Fatalf("Order %v", got)
	}
}

// splitBig: первая часть — целиком исходный разворот (название, фон, тексты),
// без раскладки; остальные — только фото.
func TestSplitBigKeepsNameAndBackdrop(t *testing.T) {
	dir := testkit.TempDir(t)
	names := make([]string, model.MaxPhotos+1)
	for i := range names {
		names[i] = "p" + strconv.Itoa(i)
	}
	_, ids := gridFile(t, dir, names, nil)
	refs := make([]string, len(names))
	for i, n := range names {
		refs[i] = ids[n]
	}
	data, _ := gridFile(t, dir, names, []*model.Spread{{
		ID: "aaaaaaaaaaaa", Photos: refs, Layout: "H(0,1)", Name: "Вступление", BgL: "#112233", BgR: "#445566",
		Texts: []*model.TextBlock{{ID: model.NewID(), X: 10, Y: 10, W: 50, Text: "Привет", Style: model.TextStyle{Size: 20}}},
	}})
	p := mustDecode(t, data, dir)
	if len(p.Spreads) != 2 {
		t.Fatalf("разворотов %d, ждали 2", len(p.Spreads))
	}
	a, b := p.Spreads[0], p.Spreads[1]
	if a.ID != "aaaaaaaaaaaa" || a.Name != "Вступление" || a.BgL != "#112233" || a.BgR != "#445566" ||
		len(a.Texts) != 1 || a.Layout != "" || len(a.Photos) != model.MaxPhotos {
		t.Fatalf("первая часть: %+v", a)
	}
	if b.ID == a.ID || b.Name != "" || b.BgL != "" || len(b.Texts) != 0 || len(b.Photos) != 1 {
		t.Fatalf("вторая часть: %+v", b)
	}
}

// dedupe (Load): разворот, опустевший от повтора фото, остаётся, если на нём
// есть название, свой фон или текст; без них — удаляется.
func TestDedupeKeepsBackdropSpread(t *testing.T) {
	dir := testkit.TempDir(t)
	_, ids := gridFile(t, dir, []string{"a"}, nil)
	a := ids["a"]
	keep := []*model.Spread{
		{Name: "Вступление"},
		{BgL: "#112233"},
		{BgR: "#445566"},
		{Texts: []*model.TextBlock{{ID: model.NewID(), X: 10, Y: 10, W: 50, Text: "Привет", Style: model.TextStyle{Size: 20}}}},
	}
	spreads := []*model.Spread{{ID: "s0", Photos: []string{a}}}
	for i, s := range keep {
		s.ID, s.Photos = "k"+strconv.Itoa(i), []string{a}
		spreads = append(spreads, s)
	}
	spreads = append(spreads, &model.Spread{ID: "gone", Photos: []string{a}})
	data, _ := gridFile(t, dir, []string{"a"}, spreads)
	p := mustDecode(t, data, dir)
	if len(p.Spreads) != 1+len(keep) {
		t.Fatalf("разворотов %d, ждали %d", len(p.Spreads), 1+len(keep))
	}
	for i, s := range p.Spreads[1:] {
		if s.ID != "k"+strconv.Itoa(i) || len(s.Photos) != 0 {
			t.Fatalf("разворот %d: %+v", i, s)
		}
	}
}
