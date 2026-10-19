package roundtrip_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Рамочки мест и прозрачность фото (079), формат v14.

var writeGolden14 = flag.Bool("golden14", false, "перезаписать testdata/v14-border.salbum.json из borderGolden")

const (
	frameKey   = "my:0f1e2d3c4b5a6978" // своя рамочка, которой нет в библиотеке
	golden14   = "v14-border.salbum.json"
	goldenUnkn = "frame-from-0-21" // вид новее программы: хранится, окно = место
)

// borderGolden — альбом голдена v14 (пути под goldenBase, файлов нет): встроенная рамочка у места с
// фото и у пустого, своя из библиотеки без файла, своя без lib (после «Собрать в папку»), неизвестный
// вид, рамочка на обложке, прозрачность двух фото.
func borderGolden() *model.Project {
	p := model.New()
	id := func(rel string, w, h int, fade float64) string {
		path := filepath.Join(goldenBase, rel)
		p.Photos[library.IDFor(path)] = model.PhotoRef{Path: path, W: w, H: h, Size: 1000, Fade: fade}
		return library.IDFor(path)
	}
	a, b, c := id(`Фото\a.jpg`, 3000, 2000, 0.35), id(`Фото\b.jpg`, 2000, 3000, 0), id(`Фото\c.jpg`, 3000, 2000, 1)
	collected := id(`Свои фоны и стикеры\0f1e2d3c4b5a6978.png`, 1200, 1600, 0)
	own := model.LibPhotoID(frameKey)
	p.Photos[own] = model.PhotoRef{Lib: frameKey, W: 3000, H: 4000, Size: 54321}
	pl := func(n int, x float64, photo string, b *model.Border) *model.Place {
		return &model.Place{ID: fmt.Sprintf("c0ffee%06d", n), X: x, Y: 30, W: 80, H: 110, Photo: photo, Border: b}
	}
	p.Spreads = []*model.Spread{
		{ID: "5e1f0a2b3c41", Photos: []string{a, b}, Grid: []*model.Place{
			pl(1, 20, a, &model.Border{Kind: "mat", Inks: []string{"#fbf8f1", "#c8a24a"}, Width: 8, Preset: "sa:fr-mat"}),
			pl(2, 110, b, &model.Border{Photo: own, Scale: 1.1, Cut: [4]float64{0.1412, 0.1388, 0.1412, 0.2}}),
			pl(3, 200, "", &model.Border{Kind: "line", Width: 1.5}),
			pl(4, 400, "", &model.Border{Photo: collected, Open: true}),
		}},
		{ID: "5e1f0a2b3c42", Photos: []string{}, Grid: []*model.Place{
			pl(5, 20, "", &model.Border{Kind: goldenUnkn, Inks: []string{"#123456"}, Width: 3}),
			pl(6, 120, "", &model.Border{Photo: own, Cut: [4]float64{0.45, 0.45, 0.45, 0.45}}),
		}},
	}
	pr, _ := model.CoverPresetOf(model.CoverDefault)
	p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID, Photos: []string{c}, Grid: []*model.Place{
		{ID: "c0ffee000007", X: 320, Y: 60, W: 90, H: 120, Photo: c, Shape: model.ShapeOval, Border: &model.Border{Kind: "polaroid", Width: 4}}}},
		Spec: pr.Spec, Layout: model.CoverFront}
	return p
}

// TestBorderGoldenV14 — голден v14 (079-26): Decode → Encode = исходник байт в байт (он же сид FuzzLoad
// и база совместимости для 0.21); рамочки, запись своей без файла и без lib, неизвестный вид,
// прозрачность — на месте; View сериализуется, у всех семи мест рамочка, окно неизвестного = место.
func TestBorderGoldenV14(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("голден с путями C:\\album")
	}
	path := filepath.Join("..", "testdata", golden14)
	if *writeGolden14 {
		data, err := salbum.Encode(borderGolden(), goldenBase)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.TrimSuffix(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), []byte("\n"))
	salbum.SetLibFinder(nil) // своей рамочки нет в библиотеке этого компьютера
	p, err := salbum.Decode(data, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || edtest.From(p) != 14 {
		t.Fatalf("версия %d, from %d", p.Version, edtest.From(p))
	}
	want := borderGolden()
	if got, w := sheetBorders(p), sheetBorders(want); !reflect.DeepEqual(got, w) {
		t.Fatalf("рамочки:\n%+v\nждали\n%+v", got, w)
	}
	own := model.LibPhotoID(frameKey)
	if r := p.Photos[own]; r.Lib != frameKey || r.Path != "" || r.W != 3000 || r.H != 4000 {
		t.Fatalf("своя без файла: %+v", r)
	}
	for id, r := range want.Photos {
		if p.Photos[id].Fade != r.Fade {
			t.Fatalf("прозрачность %s: %v, ждали %v", id, p.Photos[id].Fade, r.Fade)
		}
	}
	out, err := salbum.Encode(p, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("Decode → Encode v14 отличается от голдена:\n%s", out)
	}
	if err := salbum.Check(p); err != nil {
		t.Fatal(err)
	}
	if err := edtest.GridInvariants(p); err != nil {
		t.Fatal(err)
	}
	goldenView(t, edtest.Open(t, p, "").View())
}

// goldenView — вид голдена v14: сериализуется, у всех семи мест рамочка, окно неизвестного = место.
func goldenView(t *testing.T, v model.View) {
	t.Helper()
	if _, err := json.Marshal(v); err != nil {
		t.Fatalf("View не сериализуется: %v", err)
	}
	n := 0
	for _, s := range append(v.Spreads, v.Cover.Sheet) {
		for _, pv := range s.Places {
			if pv.Border == nil {
				t.Fatalf("место %s без рамочки в виде", pv.ID)
			}
			n++
			if pv.Border.Kind == goldenUnkn && (pv.Border.Known || pv.Border.Ins != [4]float64{}) {
				t.Fatalf("неизвестный вид: %+v", pv.Border)
			}
		}
	}
	if n != 7 {
		t.Fatalf("мест с рамочкой в виде %d, ждали 7", n)
	}
}

// sheetBorders — рамочки мест всех листов по порядку (обложка первой).
func sheetBorders(p *model.Project) []*model.Border {
	var out []*model.Border
	for _, s := range p.Sheets() {
		for _, pl := range s.Grid {
			out = append(out, pl.Border)
		}
	}
	return out
}

// v13 (0.18) открывается как v14 без потерь: только номер, первая запись делает .v13.bak с байтами
// исходника; файл новее v14 — ErrNewer. Рамочек и прозрачности у старого файла нет.
func TestVersionsV13ToV14(t *testing.T) {
	oldVersionOpens(t, 13)
	src, err := os.ReadFile(filepath.Join("..", "testdata", "v6.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := testkit.TempDir(t)
	p := edtest.MustDecode(t, bytes.Replace(src, []byte(`"version": 6`), []byte(`"version": 13`), 1), dir)
	for _, b := range sheetBorders(p) {
		if b != nil {
			t.Fatalf("рамочка у файла v13: %+v", b)
		}
	}
	for id, r := range p.Photos {
		if r.Fade != 0 {
			t.Fatalf("прозрачность у %s в файле v13", id)
		}
	}
}

// TestBorderLibMissing — своя рамочка на другом компьютере (как TestLibRefMissing): Load без
// искателя — рамочка и запись целы, путь пуст; Save → Load с искателем — нашлась, срез и ID те же.
func TestBorderLibMissing(t *testing.T) {
	dir := testkit.TempDir(t)
	own := model.LibPhotoID(frameKey)
	cut := [4]float64{0.1412, 0.1388, 0.1412, 0.2}
	p := model.New()
	p.Photos[own] = model.PhotoRef{Lib: frameKey, W: 3000, H: 4000}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{}, Grid: []*model.Place{
		{ID: "c0ffee000001", X: 10, Y: 10, W: 60, H: 80, Border: &model.Border{Photo: own, Cut: cut}},
		{ID: "c0ffee000002", X: 100, Y: 10, W: 60, H: 80, Border: &model.Border{Photo: own, Cut: cut, Scale: 0.8}},
	}}}
	path := filepath.Join(dir, "a.salbum")
	salbum.SetLibFinder(nil)
	t.Cleanup(func() { salbum.SetLibFinder(nil) })
	if err := salbum.Save(p, path); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	g := q.Spreads[0].Grid
	if r, ok := q.Photos[own]; !ok || r.Path != "" || g[0].Border == nil || g[1].Border == nil || g[0].Border.Cut != cut {
		t.Fatalf("без искателя: %+v %v, рамочки %+v %+v", r, ok, g[0].Border, g[1].Border)
	}
	if err := salbum.Save(q, path); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "Библиотека", "0f1e2d3c4b5a6978.png")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	edtest.LibFinder(t, map[string]string{frameKey: file})
	r, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	g = r.Spreads[0].Grid
	if ref := r.Photos[own]; ref.Path != file || g[0].Border.Photo != own || g[0].Border.Cut != cut || g[1].Border.Scale != 0.8 {
		t.Fatalf("с искателем: %+v, рамочки %+v %+v", ref, g[0].Border, g[1].Border)
	}
	if err := edtest.GridInvariants(r); err != nil {
		t.Fatal(err)
	}
}

// TestBorderLibRekey — ID записей рамочки при чтении: своя — от ключа (LibPhotoID), без lib — от
// пути (IDFor), рамочки двух мест на одной записи переводятся обе и остаются разными указателями.
func TestBorderLibRekey(t *testing.T) {
	dir := testkit.TempDir(t)
	file := filepath.Join(dir, "Свои фоны и стикеры", "f.png")
	data := fmt.Appendf(nil, `{"version":14,"settings":{"pageW":300,"pageH":300},"spreads":[{"id":"s1","photos":[],"grid":[
{"id":"c0ffee000001","x":10,"y":10,"w":60,"h":80,"border":{"photo":"x1","cut":[0.1,0.1,0.1,0.1]}},
{"id":"c0ffee000002","x":100,"y":10,"w":60,"h":80,"border":{"photo":"x1","scale":1.5}},
{"id":"c0ffee000003","x":200,"y":10,"w":60,"h":80,"border":{"photo":"x2","open":true}}]}],
"photos":{"x1":{"path":"","lib":%q,"w":300,"h":400},"x2":{"path":%q,"w":300,"h":400}}}`, frameKey, filepath.ToSlash(file))
	salbum.SetLibFinder(nil)
	p := edtest.MustDecode(t, data, dir)
	g := p.Spreads[0].Grid
	own, path := model.LibPhotoID(frameKey), library.IDFor(file)
	if g[0].Border.Photo != own || g[1].Border.Photo != own || g[2].Border.Photo != path || g[0].Border == g[1].Border {
		t.Fatalf("ID рамочек: %+v %+v %+v", g[0].Border, g[1].Border, g[2].Border)
	}
	if _, ok := p.Photos["x1"]; ok || len(p.Photos) != 2 {
		t.Fatalf("записи: %+v", p.Photos)
	}
	if err := edtest.GridInvariants(p); err != nil {
		t.Fatal(err)
	}
}
