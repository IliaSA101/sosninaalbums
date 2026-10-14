package prefs

import (
	"bytes"
	"errors"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func testTpl(name string) model.Template {
	return model.Template{ID: model.NewID(), Name: name, Scope: model.TplSpread,
		Boxes: []model.Box{{W: 0.5, H: 0.5}, {X: 0.5, Y: 0.5, W: 0.5, H: 0.5}}}
}

// loadLocked — LoadTemplates, пока открытие файла падает (замок антивируса).
func loadLocked(t *testing.T, dir string) Templates {
	t.Helper()
	old := openTplFile
	t.Cleanup(func() { openTplFile = old })
	openTplFile = func(string) (*os.File, error) { return nil, os.ErrPermission }
	tp := LoadTemplates(dir)
	openTplFile = old
	if !tp.unread || tp.bad {
		t.Fatalf("ошибка открытия: unread=%v bad=%v", tp.unread, tp.bad)
	}
	return tp
}

func noBad(t *testing.T, dir string) {
	t.Helper()
	if bad, _ := filepath.Glob(filepath.Join(dir, "templates.bad-*.json")); len(bad) != 0 {
		t.Fatalf("исправный файл ушёл в .bad: %v", bad)
	}
}

// Файл не открылся при старте, но исправен: Store перечитывает его и дописывает новый
// шаблон, ничего не уводя в .bad.
func TestTemplatesOpenErrorRereads(t *testing.T) {
	dir := testkit.TempDir(t)
	var first Templates
	if err := first.Store(dir, []model.Template{testTpl("старый")}, os.Rename); err != nil {
		t.Fatal(err)
	}
	tp := loadLocked(t, dir)
	if err := tp.Store(dir, []model.Template{testTpl("новый")}, os.Rename); err != nil {
		t.Fatal(err)
	}
	noBad(t, dir)
	re := LoadTemplates(dir)
	if l := re.List(); len(l) != 2 || l[0].Name != "старый" || l[1].Name != "новый" {
		t.Fatalf("файл после записи: %+v", l)
	}
	if l := tp.List(); len(l) != 2 || tp.unread {
		t.Fatalf("память после записи: %+v unread=%v", l, tp.unread)
	}
}

// Файл снова не открылся — ошибка, файл не переписан и не уведён в .bad.
func TestTemplatesOpenErrorTwice(t *testing.T) {
	dir := testkit.TempDir(t)
	content := []byte(`{"version":1,"templates":[]}`)
	if err := os.WriteFile(TemplatesPath(dir), content, 0o644); err != nil {
		t.Fatal(err)
	}
	tp := loadLocked(t, dir)
	openTplFile = func(string) (*os.File, error) { return nil, os.ErrPermission }
	if err := tp.Store(dir, []model.Template{testTpl("n")}, os.Rename); err == nil {
		t.Fatal("запись при неоткрытом файле прошла")
	}
	noBad(t, dir)
	if got, _ := os.ReadFile(TemplatesPath(dir)); !bytes.Equal(got, content) {
		t.Fatalf("файл переписан: %s", got)
	}
	if !tp.unread {
		t.Fatal("после повторной ошибки отметка unread снята")
	}
}

// Файл не открылся при старте, а при перечитывании оказался битым — уходит в .bad.
func TestTemplatesOpenErrorThenBroken(t *testing.T) {
	dir := testkit.TempDir(t)
	content := []byte(`{"version":1,"templates":[`)
	if err := os.WriteFile(TemplatesPath(dir), content, 0o644); err != nil {
		t.Fatal(err)
	}
	tp := loadLocked(t, dir)
	if err := tp.Store(dir, []model.Template{testTpl("n")}, os.Rename); err != nil {
		t.Fatal(err)
	}
	bad, err := filepath.Glob(filepath.Join(dir, "templates.bad-*.json"))
	if err != nil || len(bad) != 1 {
		t.Fatalf("битый файл должен уйти в .bad: %v %v", bad, err)
	}
	if old, _ := os.ReadFile(bad[0]); !bytes.Equal(old, content) {
		t.Fatal("в .bad лежит не исходный файл")
	}
	re := LoadTemplates(dir)
	if l := re.List(); len(l) != 1 || l[0].Name != "n" {
		t.Fatalf("новый файл: %+v", l)
	}
}

// Битый файл (ревью 038) — пустой список, но не пустой файл: до первой записи он не тронут, при
// ней уходит в .bad с прежним содержимым. Второй битый не затирает первую копию; пустой файл —
// тоже битый; файл с пустым списком — исправный.
func TestTemplatesBrokenKeepsCopy(t *testing.T) {
	dir := testkit.TempDir(t)
	bads := func() []string {
		l, _ := filepath.Glob(filepath.Join(dir, "templates.bad-*.json"))
		return l
	}
	var copies [][]byte
	for i, content := range [][]byte{[]byte(`{"version":1,"templates":[{"id":`), nil} {
		if err := os.WriteFile(TemplatesPath(dir), content, 0o644); err != nil {
			t.Fatal(err)
		}
		tp := LoadTemplates(dir)
		if !tp.bad || len(tp.List()) != 0 || len(bads()) != i {
			t.Fatalf("%d: bad=%v, шаблонов %d, копий %d", i, tp.bad, len(tp.List()), len(bads()))
		}
		if got, _ := os.ReadFile(TemplatesPath(dir)); !bytes.Equal(got, content) {
			t.Fatalf("%d: файл тронут до записи", i)
		}
		if err := tp.Store(dir, []model.Template{testTpl("a"), testTpl("b")}, os.Rename); err != nil {
			t.Fatal(err)
		}
		copies = append(copies, content)
		l := bads()
		if len(l) != i+1 {
			t.Fatalf("%d: копий %v", i, l)
		}
		var got [][]byte // имена по времени, в ту же секунду у второй «-1»: сверка без порядка
		for _, path := range l {
			b, _ := os.ReadFile(path)
			got = append(got, b)
		}
		for _, want := range copies {
			if !slices.ContainsFunc(got, func(b []byte) bool { return bytes.Equal(b, want) }) {
				t.Fatalf("%d: нет копии %q среди %q", i, want, got)
			}
		}
		if re := LoadTemplates(dir); re.bad || re.cut || len(re.List()) != 2 {
			t.Fatalf("%d: новый файл: bad=%v cut=%v, шаблонов %d", i, re.bad, re.cut, len(re.List()))
		}
	}
	if err := os.WriteFile(TemplatesPath(dir), []byte(`{"version":1,"templates":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if tp := LoadTemplates(dir); tp.bad || tp.cut || tp.Store(dir, nil, os.Rename) != nil || len(bads()) != 2 {
		t.Fatalf("пустой список: bad=%v cut=%v, копий %d", tp.bad, tp.cut, len(bads()))
	}
}

// bigTemplates — n шаблонов по places мест с углом и долями полной точности (как их
// пишет интерфейс), имя — 60 знаков «<» (до 038 JSON экранировал их по 6 байт).
func bigTemplates(n, places int) []model.Template {
	r := rand.New(rand.NewPCG(147, 1))
	list := make([]model.Template, n)
	for i := range list {
		boxes := make([]model.Box, places)
		for j := range boxes {
			w, h := 0.01+r.Float64()*0.3, 0.01+r.Float64()*0.3
			boxes[j] = model.Box{X: -w/2 + r.Float64()*(1-w/2), Y: -h/2 + r.Float64()*(1-h/2), W: w, H: h, Rot: -179 + r.Float64()*358}
		}
		list[i] = model.Template{ID: model.NewID(), Name: strings.Repeat("<", model.MaxTemplateName), Scope: model.TplSpread, Boxes: boxes}
	}
	return list
}

// bigVigTemplates — bigTemplates шаблонами виньетки (038): роли (18 учителей, групповое,
// остальные ученики), шапка, стиль подписей с шаблонами имени и стиль шапки. Углы мест
// снимает зажим (у виньетки их нет).
func bigVigTemplates(n, places int) []model.Template {
	list := bigTemplates(n, places)
	caps := model.VigCaps(304)
	head := model.TextStyle{Font: "cormorant", Size: 28, Color: "#333333", Align: "center"}
	for i := range list {
		roles := make([]string, places)
		for j := range roles {
			switch {
			case j < model.MaxTeachers:
				roles[j] = model.RoleTeacher
			case j == model.MaxTeachers:
				roles[j] = model.RoleGroup
			default:
				roles[j] = model.RolePupil
			}
		}
		list[i].Kind = model.TplVignette
		list[i].Vig = &model.VigTpl{
			VigGeo: model.VigGeo{Roles: roles, Head: &model.Box{X: 0.0512345, Y: 0.0212345, W: 0.9012345, H: 0.1212345},
				Pages: 1 + i%2, Aspect: 0.6712345, Ratio: "4:5"},
			Style: model.VigStyle{Caps: &caps, TText: model.TeacherCaption, Head: &head},
		}
	}
	return list
}

// 147: предельный список (200 шаблонов × 64 места с углом) пишется файлом меньше 1 МБ
// и читается обратно с точностью 1e-5 — раньше ≈ 2 МБ, при запуске считался битым.
// 038: то же для 200 шаблонов виньетки — с ролями, шапкой и стилем.
func TestTemplatesMaxFitsFile(t *testing.T) {
	for name, gen := range map[string]func(int, int) []model.Template{"сетки": bigTemplates, "виньетки": bigVigTemplates} {
		t.Run(name, func(t *testing.T) {
			dir := testkit.TempDir(t)
			list := model.ClampTemplates(gen(model.MaxTemplates, model.MaxPlaces))
			if len(list) != model.MaxTemplates || len(list[0].Boxes) != model.MaxPlaces || list[0].Name != strings.Repeat("<", model.MaxTemplateName) {
				t.Fatalf("список не предельный: %d шаблонов, %d мест, имя %q", len(list), len(list[0].Boxes), list[0].Name)
			}
			var tp Templates
			if err := tp.Store(dir, list, os.Rename); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(TemplatesPath(dir))
			if err != nil || len(data) > model.MaxTemplatesFile {
				t.Fatalf("файл %d байт (предел %d): %v", len(data), model.MaxTemplatesFile, err)
			}
			t.Logf("200 × 64 с углом, %s: %d КБ", name, len(data)>>10)
			re := LoadTemplates(dir)
			back := re.List()
			if re.bad || re.unread || len(back) != len(list) {
				t.Fatalf("чтение: bad=%v unread=%v, шаблонов %d", re.bad, re.unread, len(back))
			}
			// В памяти после записи — ровно то, что прочитается после перезапуска.
			if !slices.EqualFunc(tp.List(), back, func(x, y model.Template) bool {
				return x.ID == y.ID && slices.Equal(x.Boxes, y.Boxes) && reflect.DeepEqual(x.Vig, y.Vig)
			}) {
				t.Fatal("память и файл расходятся")
			}
			for i, tpl := range list {
				nearTpl(t, i, back[i], tpl)
			}
		})
	}
}

// nearTpl — шаблон после круга файла совпадает с исходным, доли и пропорция — до 1e-5.
func nearTpl(t *testing.T, i int, got, want model.Template) {
	t.Helper()
	if got.ID != want.ID || got.Name != want.Name || len(got.Boxes) != len(want.Boxes) || got.Kind != want.Kind {
		t.Fatalf("шаблон %d: %+v", i, got)
	}
	for j, b := range want.Boxes {
		nearBox(t, got.Boxes[j], b, i, j)
	}
	v, w := got.Vig, want.Vig
	if w == nil {
		if v != nil {
			t.Fatalf("шаблон %d: у сетки появилась виньетка %+v", i, v)
		}
		return
	}
	if v == nil || !slices.Equal(v.Roles, w.Roles) || v.Pages != w.Pages || v.Ratio != w.Ratio ||
		math.Abs(v.Aspect-w.Aspect) > 1e-5 || !reflect.DeepEqual(v.Style, w.Style) {
		t.Fatalf("шаблон %d виньетки: %+v, ждали %+v", i, v, w)
	}
	nearBox(t, *v.Head, *w.Head, i, -1)
}

// nearBox — место после круга файла совпадает с исходным до 1e-5 (j = -1 — шапка).
func nearBox(t *testing.T, g, b model.Box, i, j int) {
	t.Helper()
	for k, d := range [...]float64{g.X - b.X, g.Y - b.Y, g.W - b.W, g.H - b.H, g.Rot - b.Rot} {
		if math.Abs(d) > 1e-5 {
			t.Fatalf("шаблон %d место %d поле %d: %+v ≠ %+v", i, j, k, g, b)
		}
	}
}

// Данные больше 1 МБ не пишутся: понятная ошибка, файл и память прежние; битый файл
// не уводится в .bad (отметка остаётся до удачной записи). Пустой список пишется.
func TestTemplatesStoreTooBig(t *testing.T) {
	huge := bigTemplates(2*model.MaxTemplates, model.MaxPlaces) // мимо зажима: Store проверяет байты
	t.Run("исправный файл", func(t *testing.T) {
		dir := testkit.TempDir(t)
		var tp Templates
		if err := tp.Store(dir, []model.Template{testTpl("a"), testTpl("b")}, os.Rename); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(TemplatesPath(dir))
		err := tp.Store(dir, huge, os.Rename)
		if !errors.Is(err, model.ErrTemplatesFull) || !strings.HasPrefix(err.Error(), "Шаблонов слишком много") {
			t.Fatalf("ждали ErrTemplatesFull: %v", err)
		}
		if got, _ := os.ReadFile(TemplatesPath(dir)); !bytes.Equal(got, before) {
			t.Fatal("файл изменён")
		}
		if l := tp.List(); len(l) != 2 || l[0].Name != "a" || l[1].Name != "b" {
			t.Fatalf("память изменена: %d шаблонов", len(l))
		}
		if err := tp.Store(dir, nil, os.Rename); err != nil || len(tp.List()) != 0 {
			t.Fatalf("пустой список: %v %d", err, len(tp.List()))
		}
		if l := LoadTemplates(dir); len(l.List()) != 0 || l.bad {
			t.Fatalf("пустой файл: %+v", l)
		}
	})
	t.Run("битый файл", func(t *testing.T) {
		dir := testkit.TempDir(t)
		content := []byte("{не json")
		if err := os.WriteFile(TemplatesPath(dir), content, 0o644); err != nil {
			t.Fatal(err)
		}
		tp := LoadTemplates(dir)
		if err := tp.Store(dir, huge, os.Rename); !errors.Is(err, model.ErrTemplatesFull) {
			t.Fatalf("ждали ErrTemplatesFull: %v", err)
		}
		noBad(t, dir)
		if got, _ := os.ReadFile(TemplatesPath(dir)); !bytes.Equal(got, content) || !tp.bad {
			t.Fatalf("битый файл тронут: %q bad=%v", got, tp.bad)
		}
		if err := tp.Store(dir, []model.Template{testTpl("n")}, os.Rename); err != nil {
			t.Fatal(err)
		}
		if bad, _ := filepath.Glob(filepath.Join(dir, "templates.bad-*.json")); len(bad) != 1 {
			t.Fatalf("после удачной записи битый — в .bad: %v", bad)
		}
	})
}

// Файл прежних версий (с отступами, доли полной точности) читается как был.
func TestTemplatesIndentedFileReads(t *testing.T) {
	dir := testkit.TempDir(t)
	old := "{\n  \"version\": 2,\n  \"templates\": [\n    {\n      \"id\": \"0123456789ab\",\n      \"name\": \"Старый\",\n" +
		"      \"scope\": \"page\",\n      \"boxes\": [\n        {\n          \"x\": 0.123456789012,\n          \"y\": 0.1,\n" +
		"          \"w\": 0.3333333333333333,\n          \"h\": 0.25,\n          \"rot\": -12.3\n        }\n      ]\n    }\n  ]\n}"
	if err := os.WriteFile(TemplatesPath(dir), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	tp := LoadTemplates(dir)
	want := model.Box{X: 0.123456789012, Y: 0.1, W: 0.3333333333333333, H: 0.25, Rot: -12.3}
	if l := tp.List(); tp.bad || len(l) != 1 || l[0].Name != "Старый" || l[0].Boxes[0] != want {
		t.Fatalf("старый файл: bad=%v %+v", tp.bad, l)
	}
}

// Аудит 038: файл меньше 1 МБ, но после зажима (роли по числу мест) больше — последние
// шаблоны отбрасываются при чтении, иначе отказывала бы любая запись, даже удаление.
func TestTemplatesClampOverflow(t *testing.T) {
	dir := testkit.TempDir(t)
	list := bigVigTemplates(model.MaxTemplates, model.MaxPlaces)
	pad := func(n int) {
		for i := range list {
			caps := *list[i].Vig.Style.Caps
			caps.Text = strings.Repeat("я", n)
			list[i].Vig.Style.Caps, list[i].Vig.Style.TText = &caps, strings.Repeat("я", n)
			list[i].Vig.Roles = nil // зажим допишет «pupil» каждому месту
			for j := range list[i].Boxes {
				list[i].Boxes[j].Rot = 0
			}
		}
	}
	pad(0)
	base, _ := model.MarshalTemplates(list)
	pad(min(model.MaxCaptionRunes, (model.MaxTemplatesFile*95/100-len(base))/len(list)/4))
	data, _ := model.MarshalTemplates(list)
	grown, _ := model.MarshalTemplates(model.ClampTemplates(list))
	if len(data) > model.MaxTemplatesFile || len(grown) <= model.MaxTemplatesFile {
		t.Fatalf("файл %d байт, после зажима %d: случай не воспроизведён", len(data), len(grown))
	}
	if err := os.WriteFile(TemplatesPath(dir), data, 0o644); err != nil {
		t.Fatal(err)
	}
	tp := LoadTemplates(dir)
	l := tp.List()
	if tp.bad || !tp.cut || len(l) == 0 || len(l) >= len(list) || l[0].ID != list[0].ID || l[len(l)-1].ID != list[len(l)-1].ID {
		t.Fatalf("чтение: bad=%v cut=%v, шаблонов %d из %d", tp.bad, tp.cut, len(l), len(list))
	}
	if out, _ := model.MarshalTemplates(l); len(out) > model.MaxTemplatesFile {
		t.Fatalf("прочитанный список не ложится в файл: %d байт", len(out))
	}
	// Удаление одного проходит; файл читается целиком, без потерь.
	if err := tp.Store(dir, l[1:], os.Rename); err != nil {
		t.Fatalf("удаление: %v", err)
	}
	if re := LoadTemplates(dir); re.bad || re.cut || len(re.List()) != len(l)-1 || re.List()[0].ID != l[1].ID {
		t.Fatalf("после удаления: bad=%v cut=%v, шаблонов %d", re.bad, re.cut, len(re.List()))
	}
	// Отброшенные зажимом не пропали: исходный файл целиком — в .bad (ревью 038; было: копии нет).
	bad, _ := filepath.Glob(filepath.Join(dir, "templates.bad-*.json"))
	if len(bad) != 1 {
		t.Fatalf("копий исходного файла: %v", bad)
	}
	if old, _ := os.ReadFile(bad[0]); !bytes.Equal(old, data) {
		t.Fatal("в .bad не исходный файл")
	}
}
