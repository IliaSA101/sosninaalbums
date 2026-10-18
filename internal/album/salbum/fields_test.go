package salbum

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Формат 0.5.0 в v4 (design §1): файлы dev-сборки 0.4.0 открываются как раньше,
// угол места — omitempty, Load зажимает угол и место.

// Load: угол места — geom.NormDeg (почти ноль — 0, тогда прежний зажим), место под углом
// держится hold; итог неподвижен — повторный круг файла ничего не меняет.
func TestLoadPlaceRotClamp(t *testing.T) {
	dir := testkit.TempDir(t)
	_, ids := gridFile(t, dir, []string{"a"}, nil)
	pl := func(id string, x, y, w, h, rot float64) *model.Place {
		return &model.Place{ID: id, X: x, Y: y, W: w, H: h, Rot: rot}
	}
	data, _ := gridFile(t, dir, []string{"a"}, []*model.Spread{{ID: "aaaaaaaaaaaa", Photos: []string{ids["a"]}, Grid: []*model.Place{
		{ID: "c0ffee000001", X: 10, Y: 10, W: 100, H: 60, Photo: ids["a"], Rot: 400},
		pl("c0ffee000002", 100, 100, 80, 40, -180),
		pl("c0ffee000003", -10, 10, 100, 50, 0.04),
		pl("c0ffee000004", -60, 100, 100, 20, 90),
		pl("c0ffee000005", 900, -300, 80, 40, 145),
		pl("c0ffee000006", 50, 50, 40, 40, 1e300),
	}}})
	p := mustDecode(t, data, dir)
	g := p.Spreads[0].Grid
	want := []model.Place{
		{X: 10, Y: 10, W: 100, H: 60, Rot: 40},
		{X: 100, Y: 100, W: 80, H: 40, Rot: 180},
		{X: 0, Y: 10, W: 100, H: 50},
		{X: -40, Y: 100, W: 100, H: 20, Rot: 90},
		{X: 540, Y: 0, W: 80, H: 40, Rot: 145}, // разворот 600×300: центр в r = 20 от края
	}
	for i, w := range want {
		got := *g[i]
		got.ID, got.Photo = "", ""
		if got != w || math.Signbit(got.X) && got.X == 0 || math.Signbit(got.Y) && got.Y == 0 { // −0 — «-0» в файле
			t.Fatalf("место %d: %+v, ждали %+v", i, got, w)
		}
	}
	if r := g[5].Rot; r != geom.NormDeg(r) || r <= -180 || r > 180 {
		t.Fatalf("огромный угол: %v", r)
	}
	again, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	if q := mustDecode(t, again, dir); !bytes.Equal(model.Marshal(p), model.Marshal(q)) {
		t.Fatalf("второй круг меняет места:\n%s\n%s", model.Marshal(p), model.Marshal(q))
	}
}

// Страж полей Place: каждое поле места переживает круг файла. Новое поле без тега
// или с потерей в Load этот тест уронит — дописать его в full.
func TestPlaceFieldsRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	_, ids := gridFile(t, dir, []string{"a"}, nil)
	full := model.Place{ID: "c0ffee000001", X: 20.5, Y: 30.25, W: 100, H: 60, Photo: ids["a"], Fit: model.FitContain, Rot: 30,
		Role: model.RoleTeacher, Shape: model.ShapeOval, Person: model.Person{Last: "Сидорова", First: "Ольга", Subject: "Математика"}}
	// Форма и скругление (044) у одного места не живут вместе: скругление — у второго; у него же
	// табличка (071) только с предметом — пустое место учителя.
	round := full
	round.ID, round.Photo, round.Y, round.Shape, round.Round = "c0ffee000002", "", 150, "", 12
	round.Person = model.Person{Subject: "Физика"}
	v, r := reflect.ValueOf(full), reflect.ValueOf(round)
	for i := range v.NumField() {
		if v.Field(i).IsZero() && r.Field(i).IsZero() {
			t.Fatalf("поле Place.%s не задано в тесте круга", v.Type().Field(i).Name)
		}
	}
	pl, pr := full, round
	// Роль живёт только у виньетки (031): разворот — виньетка с двумя учителями.
	data, _ := gridFile(t, dir, []string{"a"}, []*model.Spread{{ID: "aaaaaaaaaaaa", Photos: []string{ids["a"]}, Grid: []*model.Place{&pl, &pr},
		Kind: model.KindVignette, Vig: &model.Vignette{Tpl: "t-left", Teachers: 2, Ratio: model.VigRatio}}})
	p := mustDecode(t, data, dir)
	if g := p.Spreads[0].Grid; len(g) != 2 || *g[0] != full || *g[1] != round {
		t.Fatalf("круг: %+v, ждали %+v и %+v", g, full, round)
	}
}

// Блок прежнего «Заголовка» — обычный закреплённый блок: x, y, w как были.
// Плохие name и rot из файла зажимаются, файл не отвергается.
func TestLoadTitleIsText(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	data := `{"version":3,"settings":{"pageW":300,"pageH":300,"margin":12,"gap":3,"layflat":true,"dpi":300},
"spreads":[{"id":"s1","photos":[],"name":"Сборы\n\u202eневесты` + strings.Repeat("а", 100) + `",
"texts":[{"id":"0123456789ab","x":12,"y":20,"w":276,"text":"Заголовок","style":{"font":"cormorant","size":28,"color":"#333333","align":"center"}},
{"id":"0123456789ac","x":100,"y":100,"w":50,"rot":-540,"text":"Угол","style":{"font":"montserrat","size":14,"color":"#222222","align":"left"}}]}],"photos":{}}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s := p.Spreads[0]
	if tt := s.Texts[0]; tt.Auto || tt.Rot != 0 || tt.X != 12 || tt.Y != 20 || tt.W != 276 || tt.Style.Font != "cormorant" {
		t.Fatalf("заголовок: %+v", *tt)
	}
	if s.Texts[1].Rot != 180 {
		t.Fatalf("угол −540 → %v", s.Texts[1].Rot)
	}
	if !strings.HasPrefix(s.Name, "Сборы невесты") || len([]rune(s.Name)) != model.MaxNameRunes {
		t.Fatalf("название: %q", s.Name)
	}
}

// Файл версии новее текущей — ErrNewer, проект не открывается.
func TestLoadV6Newer(t *testing.T) {
	data, err := json.Marshal(FileProject{Version: model.FormatVersion + 1, Settings: model.DefaultSettings()})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); !errors.Is(err, ErrNewer) {
		t.Fatalf("v%d: %v", model.FormatVersion+1, err)
	}
}

// Файл: R пишется только при ненулевом значении, читается с зажимом (подделанное ±45
// → ±45,1, огромное — по модулю 360); второй круг ничего не меняет.
func TestFrameRFile(t *testing.T) {
	dir := testkit.TempDir(t)
	var f FileProject
	data, ids := gridFile(t, dir, []string{"a", "b", "c", "d"}, nil)
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	set := func(n string, fr model.Frame) {
		fp := f.Photos[ids[n]]
		fp.Frame = &fr
		f.Photos[ids[n]] = fp
	}
	set("a", model.Frame{S: 2, R: 45})
	set("b", model.Frame{S: 1, R: -135})
	set("c", model.Frame{S: 1.5, R: 1e300})
	set("d", model.Frame{S: 1, X: 0.5})
	f.Spreads = []*model.Spread{{ID: "aaaaaaaaaaaa", Photos: []string{ids["a"], ids["b"], ids["c"], ids["d"]}}}
	data, _ = json.Marshal(f)
	p := mustDecode(t, data, dir)
	for n, want := range map[string]float64{"a": 45.1, "b": -135.1, "d": 0} {
		if got := p.Photos[ids[n]].FrameOrDefault().R; got != want {
			t.Errorf("%s: R %v, ждали %v", n, got, want)
		}
	}
	if r := p.Photos[ids["c"]].FrameOrDefault().R; r != model.ClampR(r) || math.Abs(r) > 180 {
		t.Errorf("огромный R: %v", r)
	}
	again, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	if q := mustDecode(t, again, dir); !bytes.Equal(model.Marshal(p), model.Marshal(q)) {
		t.Fatalf("второй круг меняет кадры:\n%s\n%s", model.Marshal(p), model.Marshal(q))
	}
	if data := model.Marshal(&model.Project{Photos: map[string]model.PhotoRef{"x": {Frame: &model.Frame{S: 2}}}}); !strings.Contains(string(data), `"frame":{"s":2,"x":0,"y":0}`) {
		t.Fatalf("без R кадр пишется как раньше: %s", data)
	}
}

// Файл, перепривязка, склейка и копирование оформления с фоном-фото.

func TestBackdropRekey(t *testing.T) {
	p := model.New()
	p.Photos["old"] = model.PhotoRef{Path: "/x/a.jpg", W: 3, H: 2}
	p.Photos["dup"] = model.PhotoRef{Path: "/x/a.jpg", W: 3, H: 2}
	p.Spreads = []*model.Spread{{ID: "s", BgPhoto: &model.Backdrop{Photo: "old", W: 600, H: 300}}, {ID: "t", BgPhotoL: &model.Backdrop{Photo: "dup", W: 300, H: 300}}}
	Rekey(p, func(path string) string { return "id:" + path })
	if len(p.Photos) != 1 || p.Spreads[0].BgPhoto.Photo != "id:/x/a.jpg" || p.Spreads[1].BgPhotoL.Photo != "id:/x/a.jpg" {
		t.Fatalf("rekey: %v %+v %+v", p.Photos, p.Spreads[0].BgPhoto, p.Spreads[1].BgPhotoL)
	}
}

// bdFile — файл проекта с фото ph и разворотами sp; пути ведут во временную папку.
func bdFile(t *testing.T, dir string, names []string, sp []*model.Spread) []byte {
	t.Helper()
	data, _ := gridFile(t, dir, names, sp)
	return data
}

func TestBackdropLoadDropsUnknown(t *testing.T) {
	dir := testkit.TempDir(t)
	_, ids := gridFile(t, dir, []string{"a", "bg"}, nil)
	good := func() *model.Backdrop { return &model.Backdrop{Photo: ids["bg"], X: 5, Y: 6, W: 100, H: 50, Rot: 10} }
	data := bdFile(t, dir, []string{"a", "bg"}, []*model.Spread{
		{ID: "s0", Photos: []string{ids["a"]}, BgPhoto: &model.Backdrop{Photo: "нет", W: 600, H: 300}},
		{ID: "s1", BgPhotoL: &model.Backdrop{Photo: "", W: 300, H: 300}, BgPhotoR: good()},
		{ID: "s2", BgPhoto: good(), BgPhotoL: good(), BgPhotoR: good()},
		{ID: "s3", BgPhotoL: &model.Backdrop{Photo: ids["bg"], X: 1e300, Y: -1e300, W: -5, H: 0, Rot: 725}},
		{ID: "s4", BgPhotoL: &model.Backdrop{Photo: ids["bg"], X: 9000, Y: -9000, W: 50, H: 1e9, Rot: 725, Frame: &model.Frame{S: 9, X: 3}}},
	})
	p := mustDecode(t, data, dir)
	if len(p.Spreads) != 5 {
		t.Fatalf("разворотов %d", len(p.Spreads))
	}
	s := p.Spreads
	if s[0].BgPhoto != nil || s[1].BgPhotoL != nil || s[1].BgPhotoR == nil {
		t.Fatalf("неизвестное фото не выброшено: %+v %+v", s[0], s[1])
	}
	if s[2].BgPhoto == nil || s[2].BgPhotoL != nil || s[2].BgPhotoR != nil {
		t.Fatalf("взаимоисключение: %+v", s[2])
	}
	if b := s[3].BgPhotoL; b == nil || b.X != 0 || b.W != 300 || b.H != 300 || b.Rot != 0 {
		t.Fatalf("мусорная рамка: %+v", b)
	}
	b := s[4].BgPhotoL
	cx, cy := b.TBox().Center()
	if b.W != 50 || b.H != 900 || b.Rot != 5 || cx < -0.01 || cx > 300.01 || cy < -0.01 || cy > 300.01 || b.Frame == nil || b.Frame.S != 4 || b.Frame.X != 1 {
		t.Fatalf("зажим: %+v %+v", b, b.Frame)
	}
	if _, ok := p.Photos[ids["bg"]]; !ok {
		t.Fatal("фото фона выброшено при чтении")
	}
}

// Разворот только с фоном-фото переживает чтение, деление и повторы (empty).
func TestBackdropSplitAndDedupeLoad(t *testing.T) {
	dir := testkit.TempDir(t)
	names := make([]string, model.MaxPhotos+2)
	for i := range names {
		names[i] = "p" + strconv.Itoa(i)
	}
	names = append(names, "bg")
	_, ids := gridFile(t, dir, names, nil)
	refs := make([]string, model.MaxPhotos+2)
	for i := range refs {
		refs[i] = ids[names[i]]
	}
	bg := func() *model.Backdrop { return &model.Backdrop{Photo: ids["bg"], W: 600, H: 300} }
	data := bdFile(t, dir, names, []*model.Spread{
		{ID: "aaaaaaaaaaaa", Photos: refs, BgPhoto: bg()},
		{ID: "bbbbbbbbbbbb", Photos: []string{refs[0]}, BgPhotoR: &model.Backdrop{Photo: ids["bg"], W: 300, H: 300}},
		{ID: "cccccccccccc", Photos: []string{refs[1]}},
	})
	p := mustDecode(t, data, dir)
	if len(p.Spreads) != 3 {
		t.Fatalf("разворотов %d, ждали 3 (часть, повторы)", len(p.Spreads))
	}
	if p.Spreads[0].BgPhoto == nil || p.Spreads[1].BgPhoto != nil {
		t.Fatalf("фон при делении: %+v / %+v", p.Spreads[0], p.Spreads[1])
	}
	if s := p.Spreads[2]; s.ID != "bbbbbbbbbbbb" || len(s.Photos) != 0 || s.BgPhotoR == nil {
		t.Fatalf("разворот с фоном удалён dedupe: %+v", s)
	}
}

// Круг v4: фоны на трёх разворотах, с поворотом, кадром и зеркалом, переживают запись и чтение.
func TestBackdropFileRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	mk := func(name string) (string, string) {
		path := filepath.Join(dir, "Отбор", name)
		putFile(t, path, 10)
		return path, library.IDFor(path)
	}
	pa, a := mk("a.jpg")
	pb, b := mk("b.jpg")
	pc, c := mk("c.jpg")
	p := model.New()
	p.Folders = []string{filepath.Join(dir, "Отбор")}
	for id, path := range map[string]string{a: pa, b: pb, c: pc} {
		p.Photos[id] = model.PhotoRef{Path: path, W: 300, H: 200, Size: 10}
	}
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{a}, BgPhoto: &model.Backdrop{Photo: b, X: 10.5, Y: -3, W: 400, H: 250.25, Rot: 33.3, Frame: &model.Frame{S: 2, X: -0.5, Y: 0.25}, FlipH: true}},
		{ID: "s2", BgPhotoL: &model.Backdrop{Photo: c, X: 0, Y: 0, W: 300, H: 300, FlipV: true}, BgPhotoR: &model.Backdrop{Photo: c, X: 300, Y: 300, W: 300, H: 300, Rot: -90}},
		{ID: "s3", BgPhotoR: &model.Backdrop{Photo: a, X: 320, Y: 10, W: 100, H: 100}},
	}
	path := filepath.Join(dir, "a.salbum")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if bytes.Count(data, []byte(`"bgPhoto`)) != 4 || bytes.Contains(data, []byte(`"rot": 0`)) {
		t.Fatalf("поля фона в файле:\n%s", data)
	}
	if f := fileJSON(t, path); f.Version != model.FormatVersion {
		t.Fatalf("версия %d", f.Version)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if x, y := model.Marshal(p), model.Marshal(q); !bytes.Equal(x, y) {
		t.Fatalf("потери при записи:\n%s\n%s", x, y)
	}
}

// Load: разворот с 64 текстами (спека 026) открывается целиком, с 65 — остаются первые 64.
func TestLoadTexts64(t *testing.T) {
	dir := testkit.TempDir(t)
	blocks := func(n int) []*model.TextBlock {
		out := make([]*model.TextBlock, n)
		for i := range out {
			out[i] = &model.TextBlock{ID: model.NewID(), X: float64(i % 8 * 60), Y: float64(i / 8 * 30), W: 50, Text: "т", Style: model.PlainStyle()}
		}
		return out
	}
	full, over := blocks(64), blocks(65)
	q := mustDecode(t, bdFile(t, dir, nil, []*model.Spread{{ID: "s0", Texts: full}, {ID: "s1", Texts: over}}), dir)
	for i, s := range q.Spreads {
		if len(s.Texts) != 64 || s.Texts[63].ID != []string{full[63].ID, over[63].ID}[i] {
			t.Fatalf("разворот %d: блоков %d", i, len(s.Texts))
		}
	}
}

// Файл: 0 не пишется, 0.4 переживает круг, мусор при чтении зажат, версия — текущая.
func TestBackdropFadeFile(t *testing.T) {
	dir := testkit.TempDir(t)
	_, ids := gridFile(t, dir, []string{"a", "bg"}, nil)
	bd := func(f float64) *model.Backdrop { return &model.Backdrop{Photo: ids["bg"], W: 300, H: 300, Fade: f} }
	data := bdFile(t, dir, []string{"a", "bg"}, []*model.Spread{
		{ID: "s0", Photos: []string{ids["a"]}, BgPhotoL: bd(0.4), BgPhotoR: bd(0)},
		{ID: "s1", BgPhotoL: bd(-1), BgPhotoR: bd(2)},
		{ID: "s2", BgPhoto: bd(0.12345)},
	})
	q := mustDecode(t, data, dir)
	got := []float64{q.Spreads[0].BgPhotoL.Fade, q.Spreads[0].BgPhotoR.Fade,
		q.Spreads[1].BgPhotoL.Fade, q.Spreads[1].BgPhotoR.Fade, q.Spreads[2].BgPhoto.Fade}
	if !slices.Equal(got, []float64{0.4, 0, 0, 1, 0.12}) {
		t.Fatalf("прозрачность после чтения: %v", got)
	}
	path := filepath.Join(dir, "a.salbum")
	if err := Save(q, path); err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(path)
	if bytes.Count(saved, []byte(`"fade"`)) != 3 || !bytes.Contains(saved, []byte(`"fade": 0.4`)) {
		t.Fatalf("поле fade в файле:\n%s", saved)
	}
	if f := fileJSON(t, path); f.Version != model.FormatVersion {
		t.Fatalf("версия %d", f.Version)
	}
	r, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if r.Spreads[0].BgPhotoL.Fade != 0.4 || r.Spreads[0].BgPhotoR.Fade != 0 {
		t.Fatalf("круг записи: %+v %+v", r.Spreads[0].BgPhotoL, r.Spreads[0].BgPhotoR)
	}
	// Число вне float64 (1e309) encoding/json не разбирает: файл отвергается так же,
	// как с таким числом в любом другом поле, — без паники и без частичного проекта.
	big := bytes.Replace(data, []byte(`"fade":0.4`), []byte(`"fade":1e309`), 1)
	if bytes.Equal(big, data) {
		t.Fatal("подмена fade не нашла поле")
	}
	if _, err := Decode(big, dir); err == nil {
		t.Fatal("1e309 в fade принят")
	}
}

func TestSaveOverwritesAndKeepsNoTmp(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	p := model.New()
	for range 2 { // второй раз — поверх существующего
		if err := Save(p, path); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	noTmp(t, path)
}

// noTmp — рядом с path не осталось временных <имя>.*.tmp.
func noTmp(t *testing.T, path string) {
	t.Helper()
	left, err := filepath.Glob(path + ".*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	if len(left) > 0 {
		t.Fatalf(".tmp остался: %v", left)
	}
}

// Жёсткая ссылка на месте .tmp не должна пропускать запись в чужой файл.
func TestSaveIgnoresHardlinkedTmp(t *testing.T) {
	dir := testkit.TempDir(t)
	victim := filepath.Join(dir, "victim.txt")
	orig := []byte("чужие данные")
	if err := os.WriteFile(victim, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "a.salbum")
	if err := os.Link(victim, path+".tmp"); err != nil {
		t.Skipf("жёсткие ссылки недоступны: %v", err)
	}
	if err := Save(model.New(), path); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, orig) {
		t.Fatalf("чужой файл изменён: %q", got)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
}

// Папка на месте постоянного <имя>.tmp записи не мешает (временное имя случайное) и не тронута.
func TestSaveDirAtTmp(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	inner := filepath.Join(path+".tmp", "чужое.txt")
	if err := os.Mkdir(path+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inner, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Save(model.New(), path); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(inner); err != nil || string(got) != "x" {
		t.Fatalf("папка на месте .tmp тронута: %q, %v", got, err)
	}
	noTmp(t, path)
}
