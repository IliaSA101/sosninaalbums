package salbum

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Рамочки мест и прозрачность фото (079), формат v14.

const borderLib = "my:00112233445566ff"

// borderProject — две страницы мест: встроенная рамочка с цветами и толщиной, та же без цветов
// (толщина вида), рамочка-картинка из библиотеки у места с фото и у пустого, картинка без lib (после
// «Собрать в папку») с Open; прозрачность у двух фото, у третьего — ноль. Обложка с рамочкой.
func borderProject(dir string) *model.Project {
	p := model.New()
	var ids []string
	for _, name := range []string{"a.jpg", "b.jpg", "c.jpg", "frame.png"} {
		path := filepath.Join(dir, name)
		ids = append(ids, library.IDFor(path))
		p.Photos[ids[len(ids)-1]] = model.PhotoRef{Path: path, W: 300, H: 200}
	}
	a, b, c, own := ids[0], ids[1], ids[2], ids[3]
	lib := model.LibPhotoID(borderLib)
	p.Photos[lib] = model.PhotoRef{Lib: borderLib, W: 3000, H: 4000}
	ra, rb := p.Photos[a], p.Photos[b]
	ra.Fade, rb.Fade = 0.35, 1
	p.Photos[a], p.Photos[b] = ra, rb
	pl := func(id string, x float64, photo string, br *model.Border) *model.Place {
		return &model.Place{ID: id, X: x, Y: 20, W: 80, H: 100, Photo: photo, Border: br}
	}
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{a, b}, Grid: []*model.Place{
			pl("c0ffee000001", 20, a, &model.Border{Kind: "mat", Inks: []string{"#fbf8f1", "#c8a24a"}, Width: 6.5, Preset: "sa:fr-mat"}),
			pl("c0ffee000002", 120, b, &model.Border{Photo: lib, Scale: 1.25, Cut: [4]float64{0.14, 0.14, 0.14, 0.2}}),
			pl("c0ffee000003", 220, "", &model.Border{Photo: lib, Cut: [4]float64{0.1, 0.1, 0.1, 0.1}}),
		}},
		{ID: "s2", Photos: []string{c}, Grid: []*model.Place{
			pl("c0ffee000004", 20, c, &model.Border{Kind: "line"}),
			pl("c0ffee000005", 120, "", &model.Border{Photo: own, Open: true}),
		}},
	}
	pr, _ := model.CoverPresetOf(model.CoverDefault)
	p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID, Grid: []*model.Place{
		{ID: "c0ffee000006", X: 300, Y: 50, W: 60, H: 80, Border: &model.Border{Kind: "polaroid", Width: 4}}}}, Spec: pr.Spec, Layout: model.CoverFront}
	return p
}

// borders — рамочки мест всех листов по порядку (обложка первой).
func borders(p *model.Project) []*model.Border {
	var out []*model.Border
	for _, s := range p.Sheets() {
		for _, pl := range s.Grid {
			out = append(out, pl.Border)
		}
	}
	return out
}

// TestSalbumBorderRoundTrip — Save → Load → Save: рамочки и прозрачность на месте, файл тот же;
// в файле v14, ноль прозрачности не пишется, у записи рамочки нет свойств места; мусор из файла
// зажимается при чтении.
func TestSalbumBorderRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	p := borderProject(dir)
	want := borders(p)
	path := filepath.Join(dir, "a.salbum")
	first, q := saveLoad(t, p, path)
	if f := fileJSON(t, path); f.Version != 16 || bytes.Count(first, []byte(`"border"`)) != 6 || bytes.Count(first, []byte(`"fade"`)) != 2 {
		t.Fatalf("файл v%d, рамочек %d, прозрачностей %d:\n%s", f.Version, bytes.Count(first, []byte(`"border"`)), bytes.Count(first, []byte(`"fade"`)), first)
	}
	if got := borders(q); !reflect.DeepEqual(got, want) {
		t.Fatalf("рамочки после круга:\n%+v\nждали\n%+v", got, want)
	}
	ids := q.Spreads[0].Photos
	if q.Photos[ids[0]].Fade != 0.35 || q.Photos[ids[1]].Fade != 1 || q.Photos[q.Spreads[1].Photos[0]].Fade != 0 {
		t.Fatalf("прозрачность: %+v", q.Photos)
	}
	lib := model.LibPhotoID(borderLib)
	if r, ok := q.Photos[lib]; !ok || r.Lib != borderLib || r.Path != "" || r.W != 3000 {
		t.Fatalf("запись своей рамочки: %+v %v", r, ok)
	}
	second, r := saveLoad(t, q, path)
	if !bytes.Equal(first, second) || !bytes.Equal(model.Marshal(q), model.Marshal(r)) {
		t.Fatalf("второй круг:\n%s\n%s", first, second)
	}

	// Мусор: оба поля сразу, срез за пределом, пять цветов, огромный размер, чужая запись.
	bad := bytes.Replace(first, []byte(`"kind": "line"`), []byte(`"kind": "line", "photo": "`+lib+`"`), 1)
	bad = bytes.Replace(bad, []byte(`"scale": 1.25`), []byte(`"scale": 1e308`), 1)
	bad = bytes.Replace(bad, []byte(`"w": 6.5`), []byte(`"w": -3`), 1)
	bad = bytes.Replace(bad, []byte(`"fade": 0.35`), []byte(`"fade": 7`), 1)
	if bytes.Equal(bad, first) {
		t.Fatal("заготовка мусора не сработала")
	}
	m := mustDecode(t, bad, dir)
	g0, g1 := m.Spreads[0].Grid, m.Spreads[1].Grid
	if g1[0].Border != nil || g0[1].Border.Scale != model.MaxBorderScale || g0[0].Border.Width != 0 || m.Photos[ids[0]].Fade != 1 {
		t.Fatalf("зажим: %+v %+v %+v, прозрачность %v", g1[0].Border, g0[1].Border, g0[0].Border, m.Photos[ids[0]].Fade)
	}
}

// Файл v13 (0.18) без рамочек открывается как v14: поля нулевые, первая запись делает .v13.bak,
// в записи рамочек и прозрачности нет.
func TestV13OpensWithoutBorders(t *testing.T) {
	dir := testkit.TempDir(t)
	p := seatProject(dir)
	data, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	src := bytes.Replace(data, []byte(`"version": 16,`), []byte(`"version": 13,`), 1)
	if bytes.Equal(src, data) {
		t.Fatal("заготовка v13 без номера версии")
	}
	path := filepath.Join(dir, "old.salbum")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if q.From() != 13 || q.Version != model.FormatVersion {
		t.Fatalf("from %d, версия %d", q.From(), q.Version)
	}
	for _, b := range borders(q) {
		if b != nil {
			t.Fatalf("рамочка в файле v13: %+v", b)
		}
	}
	if err := BackupOld(path, q.From()); err != nil {
		t.Fatal(err)
	}
	if err := Save(q, path); err != nil {
		t.Fatal(err)
	}
	if bak, err := os.ReadFile(path + ".v13.bak"); err != nil || !bytes.Equal(bak, src) {
		t.Fatalf(".v13.bak: %v", err)
	}
	if out, _ := os.ReadFile(path); bytes.Contains(out, []byte(`"border"`)) || bytes.Contains(out, []byte(`"fade"`)) || fileJSON(t, path).Version != 16 {
		t.Fatalf("запись v13:\n%s", out)
	}
}

// Рамочка с обоими полями (kind и photo) снимается зажимом уже после первой чистки ссылок:
// запись её картинки, больше никому не нужная, второй PruneRefs после ClampGrid убирает из Photos.
func TestLoadPrunesPhotoOfClampedBorder(t *testing.T) {
	dir := testkit.TempDir(t)
	p := model.New()
	a := filepath.Join(dir, "a.jpg")
	q := filepath.Join(dir, "q.png")
	aid, qid := library.IDFor(a), library.IDFor(q)
	p.Photos[aid] = model.PhotoRef{Path: a, W: 300, H: 200}
	p.Photos[qid] = model.PhotoRef{Path: q, W: 300, H: 200}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{aid}, Grid: []*model.Place{
		{ID: "c0ffee000001", X: 20, Y: 20, W: 80, H: 100, Photo: aid, Border: &model.Border{Photo: qid}},
		{ID: "c0ffee000002", X: 120, Y: 20, W: 80, H: 100, Border: &model.Border{Photo: qid}},
	}}}
	data, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	// Оба поля сразу: ClampBorder рамочку снимает, ссылка на q остаётся только в её записи.
	bad := bytes.ReplaceAll(data, []byte(`"border": {`), []byte(`"border": {"kind": "line", `))
	if bytes.Equal(bad, data) || !bytes.Contains(bad, []byte(`"`+qid+`"`)) {
		t.Fatalf("заготовка не сработала:\n%s", data)
	}
	m := mustDecode(t, bad, dir)
	for _, pl := range m.Spreads[0].Grid {
		if pl.Border != nil {
			t.Fatalf("рамочка с двумя полями не снята: %+v", pl.Border)
		}
	}
	if _, ok := m.Photos[qid]; ok {
		t.Fatalf("запись снятой рамочки осталась: %+v", m.Photos)
	}
	if _, ok := m.Photos[aid]; !ok {
		t.Fatal("запись фото места пропала")
	}
}
