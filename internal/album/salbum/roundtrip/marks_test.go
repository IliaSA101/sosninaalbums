package roundtrip_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func loadEditor(t *testing.T, path string) *album.Editor {
	t.Helper()
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return edtest.Open(t, p, path)
}

// v4 без отметок открывается, первая запись — v6 без поля marks.
func TestLoadV4NoMarks(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "v4-dev.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	e := loadEditor(t, path)
	if len(e.Marks()) != 0 {
		t.Fatal("у v4 появились отметки")
	}
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(path)
	if f := fileJSON(t, path); f.Version != model.FormatVersion || bytes.Contains(saved, []byte(`"marks"`)) {
		t.Fatalf("версия %d, marks в файле без отметок: %v", f.Version, bytes.Contains(saved, []byte(`"marks"`)))
	}
	if _, err := os.Stat(path + ".v4.bak"); err != nil {
		t.Fatal("нет .v4.bak")
	}
}

// Save → Load: пути (относительный + abs), 0 и "" своей не теряются, порядок записей стабилен.
func TestMarksRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	far := testkit.TempDir(t)
	path := filepath.Join(dir, "Альбом", "a.salbum")
	in := filepath.Join(dir, "Отбор", "b.jpg")
	in2 := filepath.Join(dir, "Отбор", "A.jpg")
	out := filepath.Join(far, "x", "c.jpg") // вне папок проекта и без файла; папка есть
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		t.Fatal(err)
	}
	e := edtest.Open(t, model.New(), path)
	edtest.SetMarks(t, e, []model.PhotoInfo{edtest.Lib(in, model.XmpMarks{Rating: 3, Label: "red"})}, model.MarkPatch{Rating: edtest.Ip(0), Label: edtest.Sp("")})
	edtest.SetMarks(t, e, []model.PhotoInfo{edtest.Lib(in2, model.XmpMarks{}), edtest.Lib(out, model.XmpMarks{})}, model.MarkPatch{Flag: edtest.Ip(-1), Label: edtest.Sp("purple")})
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	f := fileJSON(t, path)
	byPath := map[string]int{}
	for i, m := range f.Marks {
		byPath[m.Path] = i
	}
	ia, okA := byPath["../Отбор/A.jpg"]
	ib, okB := byPath["../Отбор/b.jpg"]
	if len(f.Marks) != 3 || !okA || !okB || ia+1 != ib || f.Marks[ia].Abs != in2 {
		t.Fatalf("записи в файле: %+v", f.Marks)
	}
	if m := f.Marks[ib]; m.Rating == nil || *m.Rating != 0 || m.Label == nil || *m.Label != "" || m.Flag != nil {
		t.Fatalf("0 и \"\" своей потерялись: %+v", m)
	}
	q := loadEditor(t, path)
	if !reflect.DeepEqual(edtest.Marks(q), edtest.Marks(e)) {
		t.Fatalf("после Load %+v, ждали %+v", edtest.Marks(q), edtest.Marks(e))
	}
	first, _ := os.ReadFile(path)
	if err := q.Save(""); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(path); !bytes.Equal(first, again) {
		t.Fatal("повторная запись дала другой файл")
	}
	// Альбом переехал с фото: относительный путь — новый, ID по новому пути.
	moved := filepath.Join(testkit.TempDir(t), "переезд")
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	r := loadEditor(t, filepath.Join(moved, "Альбом", "a.salbum"))
	nb := filepath.Join(moved, "Отбор", "b.jpg")
	if m, ok := edtest.Marks(r)[library.IDFor(nb)]; !ok || m.Path != nb {
		t.Fatalf("после переезда: %+v", edtest.Marks(r))
	}
	if m, ok := edtest.Marks(r)[library.IDFor(out)]; !ok || m.Path != out {
		t.Fatal("отметка вне папки альбома потерялась")
	}
	// Черновик (WriteCopy) — только абсолютные пути.
	draft := filepath.Join(far, "draft.salbum")
	if err := r.WriteCopy(draft); err != nil {
		t.Fatal(err)
	}
	for _, m := range fileJSON(t, draft).Marks {
		if !filepath.IsAbs(m.Path) {
			t.Fatalf("в черновике относительный путь %q", m.Path)
		}
	}
}

// Битые записи отбрасываются по одной, файл открывается.
func TestMarksFileBadEntries(t *testing.T) {
	edtest.SkipNotWindows(t)
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	esc := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	ok := filepath.Join(dir, "1.jpg")
	data := `{"version":5,"marks":[
{"path":"../../../x.jpg","rating":1},
{"path":"x.jpg","abs":"\\\\srv\\share\\x.jpg","rating":1},
{"path":"C:x.jpg","rating":1},
{"path":` + esc(ok) + `,"rating":9,"label":"red"},
{"path":"1.JPG","rating":2},
{"path":"2.jpg","rating":9,"label":"pink","flag":5},
{"path":"3.jpg","rating":-1,"label":"other","flag":-1}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	e := loadEditor(t, path)
	want := map[string][3]any{
		library.IDFor(ok):                          {-9, "red", -9}, // rating 9 → nil, повтор 1.JPG отброшен
		library.IDFor(filepath.Join(dir, "3.jpg")): {-9, "-", -1},
	}
	edtest.WantMarks(t, e, want)
}

// Load сверх предела отбрасывает сначала отметки вне папок проекта.
func TestMarksLoadLimit(t *testing.T) {
	old := model.MarksLimit()
	t.Cleanup(func() { model.SetMarksLimit(old) })
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	e := edtest.Open(t, model.New(), path)
	edtest.AddFolder(e, filepath.Join(dir, "Отбор"))
	var ph []model.PhotoInfo
	for _, n := range []string{"x/1.jpg", "Отбор/2.jpg", "y/3.jpg", "Отбор/4.jpg"} {
		ph = append(ph, edtest.Lib(filepath.Join(dir, filepath.FromSlash(n)), model.XmpMarks{}))
	}
	edtest.SetMarks(t, e, ph, model.MarkPatch{Rating: edtest.Ip(1)})
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	model.SetMarksLimit(3)
	q := loadEditor(t, path)
	if edtest.MarksLen(q) != 3 {
		t.Fatalf("отметок %d", edtest.MarksLen(q))
	}
	for _, i := range []int{1, 3} {
		if _, ok := edtest.Marks(q)[ph[i].ID]; !ok {
			t.Fatalf("отброшена отметка из папки проекта %s", ph[i].Path)
		}
	}
}

// parityCase — пример для паритета effective ↔ markOf (frontend/src/panels/marks/marks.test.ts).
type parityCase struct {
	Name string          `json:"name"`
	Xmp  model.XmpMarks  `json:"xmp"`
	Own  *model.MarkView `json:"own"`
	Want struct {
		Rating    int    `json:"rating"`
		Label     string `json:"label"`
		LabelText string `json:"labelText"`
		Flag      int    `json:"flag"`
	} `json:"want"`
}

func parityCases() []parityCase {
	xmps := []model.XmpMarks{{}, {Rating: 4, Label: "red", Flag: 1}, {Rating: 2, Label: "other", LabelText: "Клиенту", Flag: -1}}
	owns := []*model.MarkView{nil, {}, {Rating: edtest.Ip(0)}, {Label: edtest.Sp("")}, {Flag: edtest.Ip(0)},
		{Rating: edtest.Ip(5), Label: edtest.Sp("purple"), Flag: edtest.Ip(-1)}, {Label: edtest.Sp("green")}}
	var out []parityCase
	for i, x := range xmps {
		for j, o := range owns {
			c := parityCase{Name: fmt.Sprintf("x%d-own%d", i, j), Xmp: x, Own: o}
			var m model.Mark
			if o != nil {
				m = model.Mark{Rating: o.Rating, Label: o.Label, Flag: o.Flag}
			}
			got := model.Effective(m, x)
			c.Want.Rating, c.Want.Label, c.Want.LabelText, c.Want.Flag = got.Rating, got.Label, got.LabelText, got.Flag
			out = append(out, c)
		}
	}
	return out
}

func TestMarksParity(t *testing.T) {
	path := filepath.Join("..", "..", "..", "..", "testdata", "parity", "marks.json")
	if *edtest.Update {
		data, err := json.MarshalIndent(parityCases(), "", "  ")
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
	var cases []parityCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 20 {
		t.Fatalf("в marks.json всего %d примеров", len(cases))
	}
	for _, c := range cases {
		var m model.Mark
		if c.Own != nil {
			m = model.Mark{Rating: c.Own.Rating, Label: c.Own.Label, Flag: c.Own.Flag}
		}
		got := model.Effective(m, c.Xmp)
		if got.Rating != c.Want.Rating || got.Label != c.Want.Label || got.LabelText != c.Want.LabelText || got.Flag != c.Want.Flag {
			t.Errorf("%s: %+v, ждали %+v", c.Name, got, c.Want)
		}
	}
}

// marksEditor — проект с 30 разворотами и n своими отметками.
func marksEditor(b *testing.B, n int) (*album.Editor, []model.PhotoInfo) {
	b.Helper()
	e := edtest.Open(b, model.New(), "")
	for i := range 30 {
		edtest.NewSpread(e, -1, edtest.Photos(fmt.Sprintf("s%da", i), fmt.Sprintf("s%db", i), fmt.Sprintf("s%dc", i)))
	}
	ph := make([]model.PhotoInfo, n)
	for i := range ph {
		ph[i] = edtest.Lib(fmt.Sprintf(`C:\Съёмки\Отбор\IMG_%05d.jpg`, i), model.XmpMarks{})
	}
	if _, err := e.SetMarks(ph, model.MarkPatch{Rating: edtest.Ip(3)}); err != nil {
		b.Fatal(err)
	}
	return e, ph
}

func BenchmarkEditWithMarks5000(b *testing.B) {
	for _, n := range []int{0, 5000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			e, _ := marksEditor(b, n)
			to := e.View().Spreads[1].ID
			for i := 0; b.Loop(); i++ {
				if _, err := e.MovePhoto("s0a", to, i%2); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSetMarks(b *testing.B) {
	e, ph := marksEditor(b, 5000)
	for i := 0; b.Loop(); i++ {
		if _, err := e.SetMarks(ph[:200], model.MarkPatch{Rating: edtest.Ip(i % 5)}); err != nil {
			b.Fatal(err)
		}
	}
}

// fuzzMarks — инварианты отметок после Load (FuzzLoad).
func fuzzMarks(t *testing.T, marks map[string]model.Mark) {
	t.Helper()
	if len(marks) > model.MarksLimit() {
		t.Fatalf("отметок %d", len(marks))
	}
	for id, m := range marks {
		if id != library.IDFor(m.Path) || paths.CheckLocalPath(m.Path) != nil || m.None() ||
			m.Rating != nil && !model.OkRating(*m.Rating) || m.Label != nil && !model.OkLabel(*m.Label) || m.Flag != nil && !model.OkFlag(*m.Flag) ||
			edtest.Alt(m) != "" && paths.CheckLocalPath(edtest.Alt(m)) != nil || strings.ContainsRune(m.Path, 0) {
			t.Fatalf("отметка %s: %+v", id, m)
		}
	}
}

// fuzzBins — подборки после Load (083): ID верные и разные, имена чистые и разные без учёта
// регистра, не больше MaxBins; членство отметок — только известные ID, по возрастанию, без повторов.
func fuzzBins(t *testing.T, side model.Side) {
	t.Helper()
	if len(side.Bins) > model.MaxBins {
		t.Fatalf("подборок %d", len(side.Bins))
	}
	for i, b := range side.Bins {
		if !model.OkBinID(b.ID) || b.Name == "" || model.CleanBinName(b.Name) != b.Name ||
			model.BinNameTaken(side.Bins[:i], b.Name, "") || slices.ContainsFunc(side.Bins[:i], func(o model.Bin) bool { return o.ID == b.ID }) {
			t.Fatalf("подборка %d: %+v из %+v", i, b, side.Bins)
		}
	}
	known := model.BinSet(side.Bins)
	for id, m := range side.Marks {
		if !slices.IsSorted(m.Bins) || len(slices.Compact(slices.Clone(m.Bins))) != len(m.Bins) ||
			slices.ContainsFunc(m.Bins, func(b string) bool { return !known[b] }) {
			t.Fatalf("членство %s: %v", id, m.Bins)
		}
	}
}

// Запись неверного типа отбрасывается одна, marks не списком — отметок нет; альбом открывается.
func TestMarksFileWrongTypes(t *testing.T) {
	dir := testkit.TempDir(t)
	good := filepath.Join(dir, "1.jpg")
	esc := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	cases := map[string]string{
		"типы": `[{"path":"2.jpg","rating":"5"},{"path":"3.jpg","rating":3.5},{"path":5,"rating":1},` +
			`{"path":"4.jpg","label":7},"строка",[1],{"path":` + esc(good) + `,"rating":4}]`,
		"объект": `{"path":"a.jpg","rating":1}`,
		"строка": `"a.jpg"`,
		"число":  `5`,
	}
	for name, marks := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name+".salbum")
			data := `{"version":5,"spreads":[{"id":"s","photos":["p"]}],"photos":{"p":{"path":"1.jpg"}},"marks":` + marks + `}`
			if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
			e := loadEditor(t, path)
			if len(edtest.P(e).Spreads) != 1 {
				t.Fatal("альбом не прочитан")
			}
			want := map[string][3]any{}
			if name == "типы" {
				want[library.IDFor(good)] = [3]any{4, "-", -9}
			}
			edtest.WantMarks(t, e, want)
		})
	}
}

// Повтор ID: остаётся запись, найденная по основному пути, хотя запасная — раньше в файле.
func TestMarksDupPrefersPrimary(t *testing.T) {
	dir := testkit.TempDir(t)
	y := filepath.Join(dir, "Y", "1.jpg")
	if err := os.MkdirAll(filepath.Dir(y), 0o755); err != nil {
		t.Fatal(err)
	}
	x := filepath.Join(dir, "X", "1.jpg") // папки нет: A уходит на запасной Y
	esc := func(s string) string { b, _ := json.Marshal(s); return string(b) }
	path := filepath.Join(dir, "a.salbum")
	data := `{"version":5,"marks":[{"path":` + esc(x) + `,"abs":` + esc(y) + `,"rating":1},{"path":` + esc(y) + `,"rating":5}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	e := loadEditor(t, path)
	edtest.WantMarks(t, e, map[string][3]any{library.IDFor(y): {5, "-", -9}})
}

// Голден альбома 0.10.0 (формат v6): обложка, виньетка листом и на половине разворота,
// поля шапки, стиль подписей листа, имена, отступ своей подписи, безопасная зона, 18 фото на
// развороте. Пути — от папки C:\album (файлов нет), поэтому тест только на Windows.
// Файл заморожен (решение 0.11 №29): не пересоздаётся. Сверка байт в байт — с исходником, где
// "version": 6 заменено на текущий номер (v7 038 и v8 045 больше ничего в нём не меняют), а после
// "layflat" дописано "foldAny": false (v11, 064: «Авто» файла правило сгиба не меняет — закреплять нечего).

const goldenBase = `C:\album`

var golden010 = filepath.Join("..", "testdata", "golden-0.10.salbum.json")

// TestGolden010 — файл 0.10 открывается как текущая версия: Encode(Decode) = исходник с новым
// номером байт в байт, ключевые поля 0.10 на месте, исходная версия 6; запись читается и пишется
// снова без изменений.
func TestGolden010(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("голден с путями C:\\album")
	}
	data, err := os.ReadFile(golden010)
	if err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Decode(data, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || edtest.From(p) != 6 || p.Settings.Safe != 5 || len(p.Spreads) != 3 || len(p.Spreads[0].Photos) < 17 {
		t.Fatalf("версия %d, from %d, safe %v, листов %d", p.Version, edtest.From(p), p.Settings.Safe, len(p.Spreads))
	}
	out, err := salbum.Encode(p, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	cur := fmt.Sprintf(`"version": %d,`, model.FormatVersion)
	want := bytes.Replace(data, []byte(`"version": 6,`), []byte(cur), 1)
	want = bytes.Replace(want, []byte(`"layflat": true,`), []byte("\"layflat\": true,\n    \"foldAny\": false,"), 1)
	if !bytes.Equal(out, want) {
		t.Fatalf("Load → Encode файла 0.10 отличается от исходника с версией %d:\n%s", model.FormatVersion, out)
	}
	// Текущая версия стабильна: чтение и запись снова не меняют байты.
	p2, err := salbum.Decode(out, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := salbum.Encode(p2, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, out2) || !bytes.Contains(out, []byte(cur)) || edtest.From(p2) != model.FormatVersion {
		t.Fatalf("Save → Load → Save v%d не стабилен, from %d:\n%s", model.FormatVersion, edtest.From(p2), out)
	}
	vig, half := p.Spreads[1], p.Spreads[2]
	if !vig.IsVignette() || vig.Vig.Tpl != "head-t-left" || vig.Vars == nil || vig.Vars.Title != "Школа №5" || vig.Vars.Lead == "" {
		t.Fatalf("виньетка листом: %+v %+v", vig.Vig, vig.Vars)
	}
	if c := vig.Caps; c == nil || c.Pad != 2 || !c.Wrap || c.Text != model.PupilCaption {
		t.Fatalf("стиль подписей листа: %+v", c)
	}
	if !half.IsVignette() || half.Vig.Tpl != "split-tg" || len(half.Grid) == 0 {
		t.Fatalf("виньетка на развороте: %+v", half.Vig)
	}
	roles := map[string]int{}
	for _, pl := range append(vig.Grid, half.Grid...) {
		roles[pl.Role]++
	}
	if roles[model.RoleTeacher] < 2 || roles[model.RolePupil] < 4 {
		t.Fatalf("роли мест: %v", roles)
	}
	ph := func(i int) model.PhotoRef { return p.Photos[library.IDFor(goldenPhoto(i))] }
	if who := ph(18).Person; who == nil || who.Last != "Петрова" || who.Middle != "Ивановна" {
		t.Fatalf("имя: %+v", who)
	}
	if c := ph(0).Caption; c == nil || c.Text != "{Год}" || c.Pad == nil || *c.Pad != 2.5 || !c.Over {
		t.Fatalf("своя подпись: %+v", c)
	}
	c := p.Cover
	if c == nil || c.Layout != model.CoverBoth || c.SpineBg != "#203040" || len(c.Sheet.Grid) == 0 || len(c.Sheet.Texts) != 1 ||
		c.Guide != goldenBase+`\Разметка\c.png` || c.GuideAlt != `D:\Старая\c.png` {
		t.Fatalf("обложка: %+v", c)
	}
	for _, s := range [][]byte{[]byte(`"guideAlt"`), []byte(`"alt"`), []byte("\n  \"vars\"")} {
		if bytes.Contains(data, s) {
			t.Fatalf("в файле поле памяти %s", s)
		}
	}
}

// goldenPhoto — путь i-го фото голдена.
func goldenPhoto(i int) string {
	return filepath.Join(goldenBase, "Отбор", fmt.Sprintf("%02d.jpg", i))
}
