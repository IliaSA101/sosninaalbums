package roundtrip_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Наклон фото фона (спека 045), формат v8.

// Save → Load сохраняет frame.r у фонов разворота и страниц; файл — v9 (поля с v8). Значения за пределом
// из файла зажимаются: четверть отбрасывается (95 → 5, 50 → −40).
func TestLoadBackdropTiltV8(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a"))
	v := edtest.NewSpread(e, -1, edtest.Photos("b"))
	s1, s2 := v.Spreads[0].ID, v.Spreads[1].ID
	edtest.SetBg(t, e, s1, "spread", "bg")
	e.SetBackdropAngle(s1, "spread", 4.5, "")
	e.FlipBackdrop(s1, "spread", "h") // наклон −4,5
	edtest.SetBg(t, e, s2, "left", "bg2")
	e.SetBackdropAngle(s2, "left", 44.9, "")
	edtest.SetBg(t, e, s2, "right", "bg3")
	e.SetBackdropFrame(s2, "right", 2, 0.5, 0) // без наклона
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if f := fileJSON(t, path); f.Version != 16 || !bytes.Contains(data, []byte(`"r": -4.5`)) || !bytes.Contains(data, []byte(`"r": 44.9`)) {
		t.Fatalf("файл v%d без наклона фона:\n%s", f.Version, data)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	// Второй круг (ID фото после первого Load — по путям) ничего не меняет.
	again := filepath.Join(filepath.Dir(path), "b.salbum")
	if err := salbum.Save(q, again); err != nil {
		t.Fatal(err)
	}
	if q2, err := salbum.Load(again); err != nil || !bytes.Equal(model.Marshal(q), model.Marshal(q2)) {
		t.Fatalf("второй круг: %v\n%s\n%s", err, model.Marshal(q), model.Marshal(q2))
	}
	if r := q.Spreads[0].BgPhoto.Frame.R; r != -4.5 || !q.Spreads[0].BgPhoto.FlipH {
		t.Fatalf("фон разворота: %+v", q.Spreads[0].BgPhoto.Frame)
	}
	if l, r := q.Spreads[1].BgPhotoL.Frame, q.Spreads[1].BgPhotoR.Frame; l.R != 44.9 || r.R != 0 || r.S != 2 {
		t.Fatalf("фоны страниц: %+v %+v", l, r)
	}
	// Чужие значения в файле: зажим при чтении.
	bad := bytes.Replace(bytes.Replace(data, []byte(`"r": -4.5`), []byte(`"r": 95`), 1), []byte(`"r": 44.9`), []byte(`"r": 50`), 1)
	if err := os.WriteFile(path, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	if q, err = salbum.Load(path); err != nil {
		t.Fatal(err)
	}
	if a, b := q.Spreads[0].BgPhoto.Frame.R, q.Spreads[1].BgPhotoL.Frame.R; a != 5 || b != -40 {
		t.Fatalf("зажим при чтении: %v, %v; ждали 5, −40", a, b)
	}
}

// v7 (0.11) открывается как текущая версия: меняется только номер, первая запись делает
// .v7.bak с байтами исходника; файл новее текущей (как v8 для 0.11) не открывается — ErrNewer.
func TestVersionsV7ToV8(t *testing.T) { oldVersionOpens(t, 7) }

// oldVersionOpens — голден v6 с номером from открывается как FormatVersion без потерь,
// первая запись кладёт .v<from>.bak с исходными байтами; файл FormatVersion+1 — ErrNewer.
func oldVersionOpens(t *testing.T, from int) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "testdata", "v6.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Replace(src, []byte(`"version": 6`), fmt.Appendf(nil, `"version": %d`, from), 1)
	if bytes.Equal(data, src) {
		t.Fatal("голден v6 без номера версии")
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
	if p.Version != model.FormatVersion || edtest.From(p) != from {
		t.Fatalf("версия %d, from %d", p.Version, edtest.From(p))
	}
	e := edtest.Open(t, p, path)
	if !edtest.Backup(e) {
		t.Fatalf("запись поверх v%d должна делать бэкап", from)
	}
	before := model.Marshal(p)
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	if bak, err := os.ReadFile(fmt.Sprintf("%s.v%d.bak", path, from)); err != nil || !bytes.Equal(bak, data) {
		t.Fatalf(".v%d.bak: %v, совпадает с исходником: %v", from, err, bytes.Equal(bak, data))
	}
	if f := fileJSON(t, path); f.Version != model.FormatVersion {
		t.Fatalf("в файле версия %d", f.Version)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, model.Marshal(q)) || edtest.From(q) != model.FormatVersion {
		t.Fatalf("потери при v%d → v%d:\n%s\n%s", from, model.FormatVersion, before, model.Marshal(q))
	}
	newer := filepath.Join(dir, "b.salbum")
	if err := os.WriteFile(newer, bytes.Replace(src, []byte(`"version": 6`), fmt.Appendf(nil, `"version": %d`, model.FormatVersion+1), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := salbum.Load(newer); !errors.Is(err, salbum.ErrNewer) {
		t.Fatalf("файл v%d: %v, ждали ErrNewer", model.FormatVersion+1, err)
	}
}

// Форма места (спека 044), тот же v8.

// shapeEd — сетка: круг, скругление 12 %, прямоугольник; файл — a.salbum во временной папке.
func shapeEd(t *testing.T) (e *album.Editor, path string) {
	t.Helper()
	e = edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(130, 10, 100, 60, "b"), edtest.Gp(10, 150, 60, 60, "")})
	g := edtest.P(e).Spreads[0].Grid
	e.SetPlaceShape("s1", []string{g[0].ID}, model.ShapeOval)
	e.SetPlaceRound("s1", []string{g[1].ID}, 12)
	return e, filepath.Join(testkit.TempDir(t), "a.salbum")
}

// Save → Load сохраняет форму и скругление; второй круг — те же байты проекта; места без
// формы пишутся без полей (omitempty).
func TestSalbumShapeRoundTrip(t *testing.T) {
	e, path := shapeEd(t)
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if f := fileJSON(t, path); f.Version != 16 || bytes.Count(data, []byte(`"shape": "oval"`)) != 1 ||
		bytes.Count(data, []byte(`"round": 12`)) != 1 || bytes.Count(data, []byte(`"shape"`)) != 1 || bytes.Count(data, []byte(`"round"`)) != 1 {
		t.Fatalf("файл v%d:\n%s", f.Version, data)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	g := q.Spreads[0].Grid
	if len(g) != 3 || g[0].Shape != model.ShapeOval || g[0].Round != 0 || g[1].Shape != "" || g[1].Round != 12 || g[2].Shape != "" || g[2].Round != 0 {
		t.Fatalf("после Load: %+v %+v %+v", *g[0], *g[1], *g[2])
	}
	again := filepath.Join(filepath.Dir(path), "b.salbum")
	if err := salbum.Save(q, again); err != nil {
		t.Fatal(err)
	}
	if q2, err := salbum.Load(again); err != nil || !bytes.Equal(model.Marshal(q), model.Marshal(q2)) {
		t.Fatalf("второй круг: %v", err)
	}
}

// Load зажимает форму из файла: чужая — прямоугольник; round −5 / 70 / 12,4 → 0 / 50 / 12;
// у круга скругления нет. Место не выбрасывается.
func TestLoadShapeV8(t *testing.T) {
	e, path := shapeEd(t)
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bad := bytes.Replace(data, []byte(`"shape": "oval"`), []byte(`"shape": "oval", "round": 30`), 1)
	bad = bytes.Replace(bad, []byte(`"round": 12`), []byte(`"round": 12.4, "shape": "heart"`), 1)
	for _, c := range []struct {
		round string
		want  float64
	}{{"-5", 0}, {"70", 50}, {"12.4", 12}} {
		body := bytes.Replace(bad, []byte(`"round": 12.4`), []byte(`"round": `+c.round), 1)
		if bytes.Equal(body, data) || !bytes.Contains(body, []byte(`"heart"`)) {
			t.Fatal("подмена не удалась")
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
		q, err := salbum.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		g := q.Spreads[0].Grid
		if len(g) != 3 || g[0].Shape != model.ShapeOval || g[0].Round != 0 || g[1].Shape != "" || g[1].Round != c.want {
			t.Fatalf("round %s: %+v %+v", c.round, *g[0], *g[1])
		}
	}
}

// Узоры фона (048), v8: Save → Load сохраняет pat/patL/patR нескольких листов; лист только с
// узором (и с неизвестным, но правильным ключом — решение №91) не пуст и не выбрасывается;
// при pat и patL остаётся pat; прозрачность за пределом зажимается; второй круг — те же байты.
// Отметка плитки preset (решение №201) хранится, у узора без неё в файле поля нет и читается
// пустой; чужое значение отбрасывается без узора.
func TestSalbumPatternRoundTrip(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	p := model.New()
	p.Spreads = []*model.Spread{
		{ID: model.NewID(), Photos: []string{}, Pat: &model.Pattern{Kind: "confetti", Inks: []string{"#aabbcc", "#112233"}, Scale: 1.5, Seed: 7, Fade: 0.25, Preset: "sa:confetti"}},
		{ID: model.NewID(), Photos: []string{}, PatL: &model.Pattern{Kind: "stripes"}, PatR: &model.Pattern{Kind: "bands", Fade: 0.5}},
		{ID: model.NewID(), Photos: []string{}, Pat: &model.Pattern{Kind: "zz-future"}},
		{ID: model.NewID(), Photos: []string{}, Pat: &model.Pattern{Kind: "dots"}, PatL: &model.Pattern{Kind: "grid-5"}},
		{ID: model.NewID(), Photos: []string{}, BgL: "#ffeedd"}, // без узоров: в файле их нет
	}
	if err := salbum.Save(p, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if f := fileJSON(t, path); f.Version != 16 || bytes.Count(data, []byte(`"pat`)) != 6 || bytes.Count(data, []byte(`"preset"`)) != 1 {
		t.Fatalf("файл v%d, узоры:\n%s", f.Version, data)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Spreads) != 5 {
		t.Fatalf("листов %d, ждали 5", len(q.Spreads))
	}
	s := q.Spreads
	if pt := s[0].Pat; pt == nil || pt.Kind != "confetti" || pt.Scale != 1.5 || pt.Seed != 7 || pt.Fade != 0.25 ||
		fmt.Sprint(pt.Inks) != "[#aabbcc #112233]" || pt.Preset != "sa:confetti" {
		t.Errorf("узор листа: %+v", pt)
	}
	if s[1].Pat != nil || s[1].PatL == nil || s[1].PatL.Kind != "stripes" || s[1].PatR == nil || s[1].PatR.Fade != 0.5 ||
		s[1].PatL.Preset != "" {
		t.Errorf("узоры страниц: %+v %+v %+v", s[1].Pat, s[1].PatL, s[1].PatR)
	}
	if s[2].Pat == nil || s[2].Pat.Kind != "zz-future" {
		t.Errorf("неизвестный ключ: %+v", s[2].Pat)
	}
	if s[3].Pat == nil || s[3].Pat.Kind != "dots" || s[3].PatL != nil {
		t.Errorf("взаимоисключение: %+v %+v", s[3].Pat, s[3].PatL)
	}
	if s[4].Pat != nil || s[4].PatL != nil || s[4].PatR != nil {
		t.Errorf("лист без узоров: %+v", s[4])
	}
	again := filepath.Join(filepath.Dir(path), "b.salbum")
	if err := salbum.Save(q, again); err != nil {
		t.Fatal(err)
	}
	if q2, err := salbum.Load(again); err != nil || !bytes.Equal(model.Marshal(q), model.Marshal(q2)) {
		t.Fatalf("второй круг: %v\n%s\n%s", err, model.Marshal(q), model.Marshal(q2))
	}
	// Чужое значение в файле: прозрачность 5 → 1.
	bad := bytes.Replace(data, []byte(`"fade": 0.5`), []byte(`"fade": 5`), 1)
	if err := os.WriteFile(path, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	if q, err = salbum.Load(path); err != nil || q.Spreads[1].PatR.Fade != 1 {
		t.Fatalf("зажим прозрачности: %v %+v", err, q.Spreads[1].PatR)
	}
	// Чужая отметка: узор остаётся, отметка — нет.
	bad = bytes.Replace(data, []byte(`"preset": "sa:confetti"`), []byte(`"preset": "../confetti"`), 1)
	if bytes.Equal(bad, data) {
		t.Fatal("в файле нет отметки плитки")
	}
	if err := os.WriteFile(path, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	if q, err = salbum.Load(path); err != nil || q.Spreads[0].Pat == nil || q.Spreads[0].Pat.Kind != "confetti" || q.Spreads[0].Pat.Preset != "" {
		t.Fatalf("чужая отметка: %v %+v", err, q.Spreads[0].Pat)
	}
}

// Свои фоны по ключу библиотеки (спека 048, design §2).

const (
	libK1 = "my:0123456789abcdef"
	libK2 = "my:fedcba9876543210"
	libK3 = "my:00000000000000aa"
)

// writePNG — настоящая картинка w×h: размеры читаются из заголовка.
func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// libFile — файл альбома с фонами-«своими»: листы s1 (фон x1), s2 (страницы x2 и x3), s3 (фон
// с записью bad).
func libFile(t *testing.T, photos map[string]salbum.FilePhoto) []byte {
	t.Helper()
	bd := func(id string) *model.Backdrop { return &model.Backdrop{Photo: id, W: 100, H: 50} }
	f := salbum.FileProject{Version: model.FormatVersion, Settings: model.DefaultSettings(),
		Spreads: []*model.Spread{
			{ID: "s1", Photos: []string{}, BgPhoto: bd("x1")},
			{ID: "s2", Photos: []string{}, BgPhotoL: bd("x2"), BgPhotoR: bd("x3")},
			{ID: "s3", Photos: []string{}, BgPhoto: bd("bad")},
		},
		Photos: photos}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Запись с lib: путь из файла не берётся (ни path, ни abs), путь — от искателя, ID — от ключа;
// ключ не по грамматике — запись и фон с ней отброшены; при записи путей нет; круг Save → Load
// даёт тот же проект; Check пускает пустой путь своего, но не плохой ключ.
func TestSalbumLibRef(t *testing.T) {
	root := testkit.TempDir(t)
	lib := filepath.Join(root, "Библиотека", "Фоны")
	f1, f3 := filepath.Join(lib, "0123456789abcdef.png"), filepath.Join(lib, "00000000000000aa.jpg")
	writePNG(t, f1, 40, 20)
	edtest.PutFile(t, f3, 10)
	edtest.LibFinder(t, map[string]string{libK1: f1, libK3: f3})
	data := libFile(t, map[string]salbum.FilePhoto{
		"x1":  {PhotoRef: model.PhotoRef{Lib: libK1, Path: `C:\Users\чужой\x.png`}, Abs: `D:\чужой\x.png`},
		"x2":  {PhotoRef: model.PhotoRef{Lib: libK2, Path: "../../../чужой.png", W: 300, H: 200}},
		"x3":  {PhotoRef: model.PhotoRef{Lib: libK3, Path: f1, W: 300, H: 200, Size: 10}},
		"bad": {PhotoRef: model.PhotoRef{Lib: "my:XYZ", Path: f1, W: 10, H: 10}},
	})
	dir := filepath.Join(root, "Альбом")
	p := edtest.MustDecode(t, data, dir)
	id1, id2, id3 := model.LibPhotoID(libK1), model.LibPhotoID(libK2), model.LibPhotoID(libK3)
	want := map[string]model.PhotoRef{
		id1: {Lib: libK1, Path: f1, W: 40, H: 20}, // размеров не было — из заголовка файла
		id2: {Lib: libK2, W: 300, H: 200},         // нет в библиотеке: путь пуст, размеры из файла
		id3: {Lib: libK3, Path: f3, W: 300, H: 200, Size: 10},
	}
	if !reflect.DeepEqual(p.Photos, want) {
		t.Fatalf("записи:\n%+v\nждали\n%+v", p.Photos, want)
	}
	s := p.Spreads
	if len(s) != 3 || s[0].BgPhoto == nil || s[0].BgPhoto.Photo != id1 || s[1].BgPhotoL == nil ||
		s[1].BgPhotoL.Photo != id2 || s[1].BgPhotoR == nil || s[1].BgPhotoR.Photo != id3 || s[2].BgPhoto != nil {
		t.Fatalf("фоны: %s", model.Marshal(p))
	}
	if err := salbum.Check(p); err != nil {
		t.Fatalf("Check: %v", err)
	}
	path := filepath.Join(dir, "a.salbum")
	if err := salbum.Save(p, path); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("Библиотека")) || bytes.Contains(out, []byte("чужой")) || bytes.Contains(out, []byte(`"abs"`)) {
		t.Fatalf("пути своих в файле:\n%s", out)
	}
	for id, fp := range fileJSON(t, path).Photos {
		if fp.Path != "" || fp.Abs != "" || fp.Lib != want[id].Lib {
			t.Errorf("запись %s в файле: %+v", id, fp)
		}
	}
	q, err := salbum.Load(path)
	if err != nil || !bytes.Equal(model.Marshal(p), model.Marshal(q)) {
		t.Fatalf("Save → Load: %v\n%s\n%s", err, model.Marshal(p), model.Marshal(q))
	}
	// Check: плохой ключ в памяти и чужой путь у своего — ошибка.
	for _, ref := range []model.PhotoRef{{Lib: "my:XYZ"}, {Lib: libK2, Path: "x.png"}} {
		q.Photos[id2] = ref
		if salbum.Check(q) == nil {
			t.Errorf("Check пропустил %+v", ref)
		}
	}
}

// ID своего — от ключа, а не от пути: путь в памяти любой (или пуст), idFor для него не зовётся;
// записи с одним ключом сливаются, ссылки фонов переводятся.
func TestLibRefID(t *testing.T) {
	abs := edtest.Abs("Съёмка/1.jpg")
	p := model.New()
	p.Photos = map[string]model.PhotoRef{
		"a": {Lib: libK1, W: 3, H: 2},
		"b": {Lib: libK1, Path: edtest.Abs("Библиотека/x.png"), W: 3, H: 2},
		"c": {Lib: libK2, Path: abs, W: 3, H: 2},
		"d": {Path: abs, W: 3, H: 2},
	}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{"d"},
		BgPhotoL: &model.Backdrop{Photo: "a"}, BgPhotoR: &model.Backdrop{Photo: "b"}}}
	var asked []string
	salbum.Rekey(p, func(path string) string {
		asked = append(asked, path)
		return library.IDFor(path)
	})
	id1, id2, idD := model.LibPhotoID(libK1), model.LibPhotoID(libK2), library.IDFor(abs)
	if len(p.Photos) != 3 || p.Photos[id1].Lib != libK1 || p.Photos[id2].Lib != libK2 || p.Photos[idD].Lib != "" {
		t.Fatalf("записи: %+v", p.Photos)
	}
	if id2 == idD || len(asked) != 1 || asked[0] != abs {
		t.Errorf("idFor звался для %q", asked)
	}
	if s := p.Spreads[0]; s.BgPhotoL.Photo != id1 || s.BgPhotoR.Photo != id1 || s.Photos[0] != idD {
		t.Errorf("ссылки: %v %s %s", s.Photos, s.BgPhotoL.Photo, s.BgPhotoR.Photo)
	}
}

// Своего нет в библиотеке этого компьютера (нет искателя, не найдено, искатель дал нелокальный
// путь) — путь пуст, фон остаётся; размеры из файла держатся, нелепые — нули; повторное
// открытие альбом не меняет.
func TestLibRefMissing(t *testing.T) {
	photos := map[string]salbum.FilePhoto{
		"x1": {PhotoRef: model.PhotoRef{Lib: libK1, W: 300, H: 200, Size: 7}},
		"x2": {PhotoRef: model.PhotoRef{Lib: libK2, W: 5000, H: 1}}, // пропорция 5000:1 — подделка
		"x3": {PhotoRef: model.PhotoRef{Lib: libK3, W: -4, H: 2}},
	}
	dir := testkit.TempDir(t)
	check := func(name string) {
		t.Helper()
		p := edtest.MustDecode(t, libFile(t, photos), dir)
		want := map[string]model.PhotoRef{
			model.LibPhotoID(libK1): {Lib: libK1, W: 300, H: 200, Size: 7},
			model.LibPhotoID(libK2): {Lib: libK2},
			model.LibPhotoID(libK3): {Lib: libK3},
		}
		if !reflect.DeepEqual(p.Photos, want) {
			t.Errorf("%s: %+v", name, p.Photos)
		}
		if p.Spreads[0].BgPhoto == nil || p.Spreads[1].BgPhotoL == nil || p.Spreads[1].BgPhotoR == nil {
			t.Errorf("%s: фон пропавшего своего снят", name)
		}
		if err := edtest.Reopen(p); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		edtest.Open(t, p, "") // Check пускает пустой путь своего
	}
	check("без искателя")
	edtest.LibFinder(t, map[string]string{})
	check("не найдено")
	edtest.LibFinder(t, map[string]string{libK1: "rel.png", libK2: `\\srv\share\x.png`, libK3: ""})
	check("нелокальный путь")
}
