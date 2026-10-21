package roundtrip_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Golden v3 (testdata/v3.salbum.json, без зеркала) открывается как текущая версия без потерь:
// номер меняется, данные те же; первая запись делает .v3.bak и пишет FormatVersion.
func TestLoadV3AsV4(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "v3.salbum.json"))
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

// Golden v5 (testdata/v5.salbum.json: fade, отметки, два текста) открывается как текущая версия (026: v6, 038: v7, 045: v8):
// меняется только номер; первая запись делает .v5.bak с байтами исходника и пишет FormatVersion.
func TestMigrateV5ToV6(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "v5.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || edtest.From(p) != 5 || len(p.Spreads[0].Texts) != 2 || p.Spreads[0].BgPhotoR == nil || p.Spreads[0].BgPhotoR.Fade != 0.35 {
		t.Fatalf("версия %d, from %d, разворот %+v", p.Version, edtest.From(p), p.Spreads[0])
	}
	e := edtest.Open(t, p, path)
	if !edtest.Backup(e) || len(e.Marks()) != 2 {
		t.Fatalf("бэкап %v, отметок %d", edtest.Backup(e), len(e.Marks()))
	}
	before := model.Marshal(p)
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	if bak, err := os.ReadFile(path + ".v5.bak"); err != nil || !bytes.Equal(bak, data) {
		t.Fatalf(".v5.bak: %v, совпадает с исходником: %v", err, bytes.Equal(bak, data))
	}
	if f := fileJSON(t, path); f.Version != model.FormatVersion || len(f.Marks) != 2 {
		t.Fatalf("в файле версия %d, отметок %d", f.Version, len(f.Marks))
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, model.Marshal(q)) || edtest.From(q) != model.FormatVersion {
		t.Fatalf("потери при v5 → v6:\n%s\n%s", before, model.Marshal(q))
	}
}

// tieFile — файл версии v: по развороту «Авто» (Layout "") на каждый набор размеров фото.
func tieFile(t *testing.T, dir string, v int, sets [][][2]int) []byte {
	t.Helper()
	f := salbum.FileProject{Version: v, Settings: model.DefaultSettings(), Folders: []salbum.FileFolder{},
		Photos: map[string]salbum.FilePhoto{}}
	for si, set := range sets {
		s := &model.Spread{ID: fmt.Sprintf("5e1f0a2b%04d", si)}
		for i, wh := range set {
			path := filepath.Join(dir, fmt.Sprintf("s%d-%d.jpg", si, i))
			id := library.IDFor(path)
			f.Photos[id] = salbum.FilePhoto{PhotoRef: model.PhotoRef{Path: path, W: wh[0], H: wh[1]}}
			s.Photos = append(s.Photos, id)
		}
		f.Spreads = append(f.Spreads, s)
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// tieCase — первый набор фото (горизонтальные, вертикальные, квадраты), где прежний порядок
// ничьих (LegacyLayouts) выбирает другую лучшую раскладку, чем нынешний, и оба ключа.
func tieCase(t *testing.T, prm layout.Params) (set [][2]int, legacy, now string) {
	t.Helper()
	kinds := [][2]int{{300, 200}, {200, 300}, {200, 200}}
	for n := 2; n <= 6; n++ {
		for code := range int(math.Pow(3, float64(n))) {
			set = set[:0]
			asp := make([]float64, n)
			for i, c := 0, code; i < n; i, c = i+1, c/3 {
				wh := kinds[c%3]
				set = append(set, wh)
				asp[i] = float64(wh[0]) / float64(wh[1])
			}
			a, b := layout.LegacyLayouts(asp, prm, model.VariantLimit), layout.Layouts(asp, prm, model.VariantLimit)
			if a[0].Key != b[0].Key {
				return set, a[0].Key, b[0].Key
			}
		}
	}
	t.Fatal("нет набора, где прежний порядок ничьих выбирает другую раскладку")
	return nil, "", ""
}

// tieParams — геометрия файлов tieFile при «Любые фото»: так файлы до v11 считали «Авто»
// (064), ничьи сравниваются в нём, а закрепление сгиба (pinFoldAuto) — отдельный шаг.
func tieParams(t *testing.T, dir string) layout.Params {
	t.Helper()
	prm := edtest.MustDecode(t, tieFile(t, dir, 5, nil), dir).Params()
	prm.FoldAny = true
	return prm
}

// Альбом до 0.10 (v5): у «Авто» с Layout "" Load записывает ключ раскладки, которую выбирал
// прежний порядок ничьих (решение 95), если он отличается от нынешней лучшей; где совпадает —
// остаётся "". Файл v6 по ничьим не мигрирует (показывает нынешнюю лучшую «Любых фото» — её же
// мог закрепить pinFoldAuto). Ключ ничьей сгиб не трогает (064, TestLoadV5FoldLegacy). Save →
// Load ничего не меняет.
func TestLoadV5KeepsTieLayout(t *testing.T) {
	dir := testkit.TempDir(t)
	set, legacy, now := tieCase(t, tieParams(t, dir))
	same := [][2]int{{300, 200}}
	path := filepath.Join(dir, "a.salbum")
	if err := os.WriteFile(path, tieFile(t, dir, 5, [][][2]int{set, same}), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Spreads[0].Layout; got != legacy {
		t.Fatalf("v5: Layout %q, ждали прежний %q (нынешний %q)", got, legacy, now)
	}
	if r, _, ok := p.Layout(p.Spreads[0]); !ok || r.Key != legacy {
		t.Fatalf("v5: действует %q, ждали %q", r.Key, legacy)
	}
	if got := p.Spreads[1].Layout; got != "" {
		t.Fatalf("v5 без ничьей: Layout %q, ждали \"\"", got)
	}
	if p.Settings.FoldAny || !p.Settings.Layflat {
		t.Fatalf("v5: layflat %v, foldAny %v — ждали «Только широкие»", p.Settings.Layflat, p.Settings.FoldAny)
	}
	v6 := edtest.MustDecode(t, tieFile(t, dir, 6, [][][2]int{set}), dir)
	if r, _, _ := v6.Layout(v6.Spreads[0]); v6.Spreads[0].Layout == legacy || r.Key != now {
		t.Fatalf("v6 мигрировал по ничьим: Layout %q, действует %q (ждали %q)", v6.Spreads[0].Layout, r.Key, now)
	}
	e := edtest.Open(t, p, path)
	before := model.Marshal(p)
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, model.Marshal(q)) || edtest.From(q) != model.FormatVersion || q.Spreads[0].Layout != legacy {
		t.Fatalf("Save → Load изменил альбом: Layout %q\n%s\n%s", q.Spreads[0].Layout, before, model.Marshal(q))
	}
}

// «Авто» с "variant": -1 в файле v4 и v5 открывается (номер зажимается к первому варианту ряда),
// а не падает на индексе ряда; прежний выбор при ничьей тот же, что у variant 0.
func TestLoadNegativeVariant(t *testing.T) {
	dir := testkit.TempDir(t)
	set, legacy, _ := tieCase(t, tieParams(t, dir))
	for _, v := range []int{4, 5} {
		data := tieFile(t, dir, v, [][][2]int{set})
		data = bytes.Replace(data, []byte(`"id":"5e1f0a2b0000"`), []byte(`"id":"5e1f0a2b0000","variant":-1`), 1)
		if !bytes.Contains(data, []byte(`"variant":-1`)) {
			t.Fatal("variant -1 не попал в файл")
		}
		p := edtest.MustDecode(t, data, dir)
		if got := p.Spreads[0].Layout; got != legacy || p.Spreads[0].Variant != 0 {
			t.Fatalf("v%d: Layout %q, variant %d, ждали %q и 0", v, got, p.Spreads[0].Variant, legacy)
		}
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
	data, err := os.ReadFile(filepath.Join("..", "testdata", "v4-dev.salbum.json"))
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
	if p.Version != model.FormatVersion || model.FormatVersion != 15 {
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
	junk := slices.Repeat([]string{"нет"}, model.MaxPlaces+model.MaxTexts+model.MaxStickers)
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

// Безопасная зона (028 А) — в v6 без нового номера: файл без поля (v5 и ранние v6)
// открывается с 8 благодаря заполнению DefaultSettings() до разбора; 12 и 0 («без зоны»)
// переживают запись и чтение.
func TestLoadSafeDefault(t *testing.T) {
	if model.FormatVersion != 15 {
		t.Fatalf("FormatVersion %d: безопасная зона — с v6 (decisions 0.10, запись 2), v7 — vig.own (038), v8 — наклон фона (045), v9 — ключи форм (051), v10 — фото корешка (061), v11 — правило сгиба (064), обрез и разметка разворотов (066), v12 — тон фото (068), v13 — табличка места (071), v14 — рамочки и прозрачность фото (079), v15 — подборки фото (083)", model.FormatVersion)
	}
	data, err := os.ReadFile(filepath.Join("..", "testdata", "v5.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	v6 := bytes.Replace(data, []byte(`"version": 5`), []byte(`"version": 6`), 1)
	if bytes.Contains(data, []byte(`"safe"`)) || bytes.Equal(v6, data) {
		t.Fatal("голден v5: есть safe или нет номера версии")
	}
	dir := testkit.TempDir(t)
	for name, raw := range map[string][]byte{"v5": data, "v6 без safe": v6} {
		if p := edtest.MustDecode(t, raw, dir); p.Settings.Safe != model.DefaultSafe || p.Settings.PageW != 281 {
			t.Fatalf("%s: safe %v, pageW %v", name, p.Settings.Safe, p.Settings.PageW)
		}
	}
	path := filepath.Join(dir, "safe.salbum")
	for _, safe := range []float64{12, 0} {
		e := edtest.Open(t, edtest.MustDecode(t, v6, dir), "")
		st := e.View().Settings
		st.Safe = safe
		e.SetSettings(st)
		if err := e.Save(path); err != nil {
			t.Fatal(err)
		}
		if raw, err := os.ReadFile(path); err != nil || !bytes.Contains(raw, []byte(`"safe"`)) {
			t.Fatalf("safe %v не записан в файл: %v", safe, err)
		}
		q, err := salbum.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if q.Settings.Safe != safe || q.Version != model.FormatVersion {
			t.Fatalf("после круга safe %v (ждали %v), версия %d", q.Settings.Safe, safe, q.Version)
		}
	}
}

// Миграция v5 при последовательном счёте раскладок (без параллели) даёт те же ключи,
// что и параллельная: ветка migRows без model.Each.
func TestLoadV5KeepsTieLayoutSequential(t *testing.T) {
	dir := testkit.TempDir(t)
	set, legacy, _ := tieCase(t, tieParams(t, dir))
	data := tieFile(t, dir, 5, [][][2]int{set, {{300, 200}}})
	par := edtest.MustDecode(t, data, dir)
	model.SetParallelLayouts(false)
	t.Cleanup(func() { model.SetParallelLayouts(true) })
	seq := edtest.MustDecode(t, data, dir)
	if got := seq.Spreads[0].Layout; got != legacy || seq.Spreads[1].Layout != "" {
		t.Fatalf("последовательно: Layout %q и %q, ждали %q и \"\"", got, seq.Spreads[1].Layout, legacy)
	}
	if !bytes.Equal(model.Marshal(par), model.Marshal(seq)) {
		t.Fatalf("параллельно и последовательно разные результаты:\n%s\n%s", model.Marshal(par), model.Marshal(seq))
	}
}
