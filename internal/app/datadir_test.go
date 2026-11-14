package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// dataApp — App со своей папкой данных и шаблонами list в templates.json.
func dataApp(t *testing.T, list ...model.Template) *App {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	if len(list) > 0 {
		data, err := model.MarshalTemplates(list)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(prefs.TemplatesPath(a.cfgDir), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	a.loadTemplates()
	return a
}

// myTpl — сетка страницы с местом, сдвинутым на i (разные i < 80 — разное содержимое).
func myTpl(name string, i int) model.Template {
	return model.Template{ID: model.NewID(), Name: name, Scope: model.TplPage,
		Boxes: []model.Box{{X: float64(i) / 100, Y: 0.1, W: 0.2, H: 0.3}}}
}

// stubSave подменяет диалог сохранения: отдаёт pick, считает вызовы и запоминает настройки.
func stubSave(t *testing.T, pick *string) (calls *int, opts *wr.SaveDialogOptions) {
	t.Helper()
	old := saveFileDialog
	t.Cleanup(func() { saveFileDialog = old })
	calls, opts = new(int), new(wr.SaveDialogOptions)
	saveFileDialog = func(_ context.Context, o wr.SaveDialogOptions) (string, error) {
		*calls++
		*opts = o
		return *pick, nil
	}
	return calls, opts
}

// noPathErr — ошибка есть и без пути (тост показывает её человеку).
func noPathErr(t *testing.T, tag string, err error, path string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: нет ошибки", tag)
	}
	if path != "" && strings.Contains(err.Error(), path) {
		t.Fatalf("%s: путь в тексте ошибки: %v", tag, err)
	}
}

// «Открыть папку данных»: создаёт и открывает ровно cfgDir; пустой путь, недоступная папка и
// отказ проводника — ошибка без пути.
func TestShowDataFolder(t *testing.T) {
	var opened []string
	old := openDir
	t.Cleanup(func() { openDir = old })
	openDir = func(d string) error { opened = append(opened, d); return nil }

	a := newApp(t)
	a.cfgDir = filepath.Join(testkit.TempDir(t), "SosninaAlbums")
	if err := a.ShowDataFolder(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(opened, []string{a.cfgDir}) {
		t.Fatalf("открыто %v, ждали %s", opened, a.cfgDir)
	}
	if st, err := os.Stat(a.cfgDir); err != nil || !st.IsDir() {
		t.Fatalf("папка не создана: %v", err)
	}

	empty := newApp(t)
	empty.cfgDir = ""
	if err := empty.ShowDataFolder(); !errors.Is(err, errDataDir) || len(opened) != 1 {
		t.Fatalf("пустой путь: %v %v", err, opened)
	}

	file := newApp(t)
	file.cfgDir = filepath.Join(testkit.TempDir(t), "файл")
	if err := os.WriteFile(file.cfgDir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := file.ShowDataFolder()
	noPathErr(t, "файл на месте папки", err, file.cfgDir)
	if len(opened) != 1 {
		t.Fatalf("проводник запущен: %v", opened)
	}

	openDir = func(d string) error { return errors.New("exec " + d) }
	err = a.ShowDataFolder()
	noPathErr(t, "отказ проводника", err, a.cfgDir)
	if !errors.Is(err, errDataDirOpen) {
		t.Fatal(err)
	}
}

// «Сохранить в файл…»: пусто — ошибка без диалога; отказ — Canceled; файл = MarshalTemplates
// списка, «.json» дописывается, папка диалога запоминается; сетевой путь — отказ.
func TestExportTemplates(t *testing.T) {
	pick := ""
	calls, opts := stubSave(t, &pick)

	empty := dataApp(t)
	if _, err := empty.ExportTemplates(); !errors.Is(err, errTplNone) || *calls != 0 {
		t.Fatalf("пустой список: %v, диалогов %d", err, *calls)
	}

	list := []model.Template{myTpl("Раз", 1), myTpl("Два", 2), myTpl("Три", 3)}
	a := dataApp(t, list...)
	res, err := a.ExportTemplates()
	if err != nil || !res.Canceled || *calls != 1 {
		t.Fatalf("отказ от диалога: %+v %v", res, err)
	}
	if opts.DefaultFilename != "Мои шаблоны.json" || len(opts.Filters) != 1 || opts.Filters[0].Pattern != "*.json" {
		t.Fatalf("диалог: %+v", *opts)
	}

	out := testkit.TempDir(t)
	pick = filepath.Join(out, "перенос")
	res, err = a.ExportTemplates()
	if err != nil || res.Canceled || res.Saved != 3 || res.Text != "Сохранено шаблонов: 3" {
		t.Fatalf("сохранение: %+v %v", res, err)
	}
	got, err := os.ReadFile(pick + ".json")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := model.MarshalTemplates(a.templateList())
	if !bytes.Equal(got, want) {
		t.Fatalf("файл:\n%s\nждали:\n%s", got, want)
	}
	if d := a.dialogDir(dirTemplates); d != out {
		t.Fatalf("папка диалога %q, ждали %q", d, out)
	}

	// Уже с .json (другой регистр) — имя не меняется.
	pick = filepath.Join(out, "Второй.JSON")
	if _, err := a.ExportTemplates(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pick); err != nil {
		t.Fatalf("имя с .JSON: %v", err)
	}

	pick = `\\server\share\мои.json`
	_, err = a.ExportTemplates()
	noPathErr(t, "сетевой", err, pick)
	if !errors.Is(err, errTplSaveLocal) {
		t.Fatal(err)
	}

	// Папка вместо файла — запись не удалась, ошибка без пути.
	pick = filepath.Join(out, "папка.json")
	if err := os.Mkdir(pick, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = a.ExportTemplates()
	noPathErr(t, "папка", err, pick)
}

// Дописанное «.json» не перезаписывает молча чужой файл: «x.txt» при готовом «x.txt.json» — отказ
// с именем (без папки), файл не тронут; имя с .json (замену подтвердил диалог) — перезапись;
// дописанное к свободному имени — сохранение.
func TestExportTemplatesNoSilentOverwrite(t *testing.T) {
	pick := ""
	stubSave(t, &pick)
	a := dataApp(t, myTpl("Раз", 1), myTpl("Два", 2))
	out := testkit.TempDir(t)
	busy := filepath.Join(out, "x.txt.json")
	if err := os.WriteFile(busy, []byte("чужое"), 0o600); err != nil {
		t.Fatal(err)
	}
	pick = filepath.Join(out, "x.txt")
	_, err := a.ExportTemplates()
	noPathErr(t, "занято", err, out)
	if err.Error() != "файл уже есть: x.txt.json" {
		t.Fatalf("текст: %v", err)
	}
	if got, _ := os.ReadFile(busy); string(got) != "чужое" {
		t.Fatalf("файл перезаписан: %q", got)
	}
	if _, err := os.Stat(pick); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("создан файл без .json: %v", err)
	}

	pick = busy
	if res, err := a.ExportTemplates(); err != nil || res.Saved != 2 {
		t.Fatalf("с .json: %+v %v", res, err)
	}
	if got, _ := os.ReadFile(busy); string(got) == "чужое" {
		t.Fatal("с .json: не перезаписан")
	}

	pick = filepath.Join(out, "y.txt")
	if res, err := a.ExportTemplates(); err != nil || res.Saved != 2 {
		t.Fatalf("свободное имя: %+v %v", res, err)
	}
	if _, err := os.Stat(pick + ".json"); err != nil {
		t.Fatal(err)
	}
}

// Круг: сохранить на одном «компьютере», загрузить на другом — добавлено, уже были, не вошли;
// templates.json записан и читается после перезапуска; повтор — «все уже есть», файл тот же.
func TestImportTemplatesPath(t *testing.T) {
	pick := ""
	stubSave(t, &pick)
	src := dataApp(t, myTpl("Раз", 1), myTpl("Два", 2), myTpl("Три", 3))
	pick = filepath.Join(testkit.TempDir(t), "мои.json")
	if _, err := src.ExportTemplates(); err != nil {
		t.Fatal(err)
	}

	same := myTpl("Переименован", 2) // тот же, что «Два», под другим именем
	dst := dataApp(t, same)
	res, err := dst.ImportTemplatesPath(pick)
	if err != nil || res != (TplLoad{Added: 2, Same: 1, Text: "Добавлено шаблонов: 2 · уже были: 1"}) {
		t.Fatalf("загрузка: %+v %v", res, err)
	}
	names := func(list []model.Template) []string {
		var out []string
		for _, t := range list {
			out = append(out, t.Name)
		}
		return out
	}
	if got := names(dst.templateList()); !slices.Equal(got, []string{"Переименован", "Раз", "Три"}) {
		t.Fatalf("список %v", got)
	}
	re := dataApp(t)
	re.cfgDir = dst.cfgDir
	re.loadTemplates()
	if got := names(re.templateList()); !slices.Equal(got, []string{"Переименован", "Раз", "Три"}) {
		t.Fatalf("после перезапуска %v", got)
	}

	before, _ := os.ReadFile(prefs.TemplatesPath(dst.cfgDir))
	if _, err := dst.ImportTemplatesPath(pick); !errors.Is(err, errTplAllSame) {
		t.Fatalf("повтор: %v", err)
	}
	after, _ := os.ReadFile(prefs.TemplatesPath(dst.cfgDir))
	if !bytes.Equal(before, after) {
		t.Fatal("повтор переписал templates.json")
	}

	// 198 своих + 3 из файла: 2 вошли, 1 нет; на полном списке — «уже 200».
	var many []model.Template
	for i := range 198 {
		many = append(many, model.Template{ID: model.NewID(), Scope: model.TplSpread,
			Boxes: []model.Box{{X: float64(i%70) / 100, Y: float64(i/70) / 100, W: 0.1, H: 0.1}}})
	}
	full := dataApp(t, many...)
	res, err = full.ImportTemplatesPath(pick)
	if err != nil || res.Added != 2 || res.Over != 1 || res.Text != fmt.Sprintf("Добавлено шаблонов: 2 · не вошли: 1 (не больше %d)", model.MaxTemplates) {
		t.Fatalf("предел: %+v %v", res, err)
	}
	if n := len(full.templateList()); n != model.MaxTemplates {
		t.Fatalf("шаблонов %d", n)
	}
	other := filepath.Join(testkit.TempDir(t), "другие.json")
	data, _ := model.MarshalTemplates([]model.Template{myTpl("Новый", 50)})
	if err := os.WriteFile(other, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := full.ImportTemplatesPath(other); !errors.Is(err, errTplAllOver) {
		t.Fatalf("полный список: %v", err)
	}
}

// Отказы: шаблоны и templates.json не тронуты, ошибка без пути.
func TestImportTemplatesRejects(t *testing.T) {
	a := dataApp(t, myTpl("Мой", 1))
	tplFile := prefs.TemplatesPath(a.cfgDir)
	before, err := os.ReadFile(tplFile)
	if err != nil {
		t.Fatal(err)
	}
	dir := testkit.TempDir(t)
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	big := append([]byte(`{"version":1,"templates":[],"x":"`), bytes.Repeat([]byte("a"), model.MaxTemplatesFile)...)
	folder := filepath.Join(dir, "папка.json")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, path string
		want       error
	}{
		{"не JSON", write("мусор.json", []byte("не json")), errTplFile},
		{"чужой JSON", write("чужой.json", []byte(`{"name":"x"}`)), errTplFile},
		{"не .json", write("шаблоны.txt", before), errTplFile},
		{"больше 1 МБ", write("большой.json", append(big, '"', '}')), errTplBig},
		{"новее", write("новый.json", []byte(`{"version":99,"templates":[]}`)), errTplNewerFile},
		{"0 шаблонов", write("пустой.json", []byte(`{"version":1,"templates":[]}`)), errTplEmpty},
		{"все выброшены зажимом", write("плохой.json", []byte(`{"version":1,"templates":[{"id":"x","scope":"page","boxes":[]}]}`)), errTplEmpty},
		{"сетевой", `\\server\share\мои.json`, errTplLocal},
		{"относительный", "мои.json", errTplLocal},
		{"пустой путь", "", errTplLocal},
		{"папка", folder, errTplFile},
		{"нет файла", filepath.Join(dir, "нет.json"), errTplMissing},
	}
	for _, c := range cases {
		res, err := a.ImportTemplatesPath(c.path)
		if !errors.Is(err, c.want) || res != (TplLoad{}) {
			t.Errorf("%s: %+v %v, ждали %v", c.name, res, err, c.want)
			continue
		}
		if c.path != "" && (strings.Contains(err.Error(), c.path) || strings.Contains(err.Error(), dir)) {
			t.Errorf("%s: путь в тексте ошибки: %v", c.name, err)
		}
	}
	after, _ := os.ReadFile(tplFile)
	if !bytes.Equal(before, after) || len(a.templateList()) != 1 {
		t.Fatal("отказ изменил шаблоны")
	}
}

// Свой templates.json записан более новой версией — только чтение: загрузка отказывает, файл тот же.
func TestImportTemplatesNewerStore(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	own := []byte(`{"version":99,"templates":[{"id":"a1","name":"Мой","scope":"page","boxes":[{"x":0,"y":0,"w":0.5,"h":0.5}]}]}`)
	if err := os.WriteFile(prefs.TemplatesPath(a.cfgDir), own, 0o600); err != nil {
		t.Fatal(err)
	}
	a.loadTemplates()
	src := filepath.Join(testkit.TempDir(t), "мои.json")
	data, _ := model.MarshalTemplates([]model.Template{myTpl("Новый", 5)})
	if err := os.WriteFile(src, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ImportTemplatesPath(src); !errors.Is(err, model.ErrTemplatesNewer) {
		t.Fatalf("ждали ErrTemplatesNewer: %v", err)
	}
	if got, _ := os.ReadFile(prefs.TemplatesPath(a.cfgDir)); !bytes.Equal(got, own) {
		t.Fatal("файл новее программы переписан")
	}
}

// lockedApp — App, у которого templates.json не прочитался при старте (на его месте папка:
// открыть можно, читать нельзя), а потом «замок» снят и в файле лежит list.
func lockedApp(t *testing.T, list ...model.Template) *App {
	t.Helper()
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	file := prefs.TemplatesPath(a.cfgDir)
	if err := os.Mkdir(file, 0o700); err != nil {
		t.Fatal(err)
	}
	a.loadTemplates()
	if len(a.templateList()) != 0 {
		t.Fatal("список не пуст при неоткрытом файле")
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	data, err := model.MarshalTemplates(list)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return a
}

func writeTplFile(t *testing.T, list ...model.Template) string {
	t.Helper()
	p := filepath.Join(testkit.TempDir(t), "чужие.json")
	data, err := model.MarshalTemplates(list)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// templates.json не открылся при старте: импорт сверяется с файлом, а не с пустым списком —
// без дублей, ID уникальны, тост верный (84/86, ревью).
func TestImportTemplatesAfterUnreadStart(t *testing.T) {
	a1, a2, a3 := myTpl("Раз", 1), myTpl("Два", 2), myTpl("Три", 3)
	a := lockedApp(t, a1, a2, a3)
	d := myTpl("Новый", 4)
	// a2 — тот же шаблон, что в файле; d — новый.
	res, err := a.ImportTemplatesPath(writeTplFile(t, a2, d))
	if err != nil || res != (TplLoad{Added: 1, Same: 1, Text: "Добавлено шаблонов: 1 · уже были: 1"}) {
		t.Fatalf("импорт: %+v %v", res, err)
	}
	got := a.templateList()
	ids := map[string]bool{}
	for _, tp := range got {
		if ids[tp.ID] {
			t.Fatalf("повторный ID %s: %+v", tp.ID, got)
		}
		ids[tp.ID] = true
	}
	if len(got) != 4 || got[0].ID != a1.ID || got[3].Name != "Новый" {
		t.Fatalf("список: %+v", got)
	}
	re := dataApp(t)
	re.cfgDir = a.cfgDir
	re.loadTemplates()
	if n := len(re.templateList()); n != 4 {
		t.Fatalf("после перезапуска %d шаблонов", n)
	}
}

// То же у предела: в файле 199, импорт трёх — один входит, два «не вошли», а не ErrTooManyTpl.
func TestImportTemplatesAfterUnreadStartLimit(t *testing.T) {
	var many []model.Template
	for i := range model.MaxTemplates - 1 {
		many = append(many, model.Template{ID: model.NewID(), Scope: model.TplSpread,
			Boxes: []model.Box{{X: float64(i%70) / 100, Y: float64(i/70) / 100, W: 0.1, H: 0.1}}})
	}
	a := lockedApp(t, many...)
	res, err := a.ImportTemplatesPath(writeTplFile(t, myTpl("А", 71), myTpl("Б", 72), myTpl("В", 73)))
	if err != nil || res.Added != 1 || res.Over != 2 {
		t.Fatalf("предел: %+v %v", res, err)
	}
	if n := len(a.templateList()); n != model.MaxTemplates {
		t.Fatalf("шаблонов %d", n)
	}
}

// Файл так и не открылся — импорт отказывает, templates.json не тронут (папка на месте).
func TestImportTemplatesUnreadStillLocked(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	file := prefs.TemplatesPath(a.cfgDir)
	if err := os.Mkdir(file, 0o700); err != nil {
		t.Fatal(err)
	}
	a.loadTemplates()
	if _, err := a.ImportTemplatesPath(writeTplFile(t, myTpl("А", 1))); err == nil {
		t.Fatal("импорт прошёл при нечитаемом файле")
	}
	if fi, err := os.Stat(file); err != nil || !fi.IsDir() {
		t.Fatal("templates.json тронут")
	}
}

// «Загрузить из файла…»: отказ от диалога — Canceled без правки; выбор — загрузка и папка диалога;
// отказ загрузки папку не запоминает.
func TestImportTemplates(t *testing.T) {
	a := dataApp(t)
	old := openFileDialog
	t.Cleanup(func() { openFileDialog = old })
	pick := ""
	var opts wr.OpenDialogOptions
	openFileDialog = func(_ context.Context, o wr.OpenDialogOptions) (string, error) { opts = o; return pick, nil }

	res, err := a.ImportTemplates()
	if err != nil || !res.Canceled || len(opts.Filters) != 1 || opts.Filters[0].Pattern != "*.json" {
		t.Fatalf("отказ: %+v %v %+v", res, err, opts)
	}
	if _, err := os.Stat(prefs.TemplatesPath(a.cfgDir)); !os.IsNotExist(err) {
		t.Fatalf("отказ записал templates.json: %v", err)
	}

	bad := testkit.TempDir(t)
	pick = filepath.Join(bad, "мусор.json")
	if err := os.WriteFile(pick, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ImportTemplates(); !errors.Is(err, errTplFile) || a.dialogDir(dirTemplates) == bad {
		t.Fatalf("мусор: %v, папка %q", err, a.dialogDir(dirTemplates))
	}

	dir := testkit.TempDir(t)
	pick = filepath.Join(dir, "мои.json")
	data, _ := model.MarshalTemplates([]model.Template{myTpl("Раз", 1), myTpl("Два", 2)})
	if err := os.WriteFile(pick, data, 0o600); err != nil {
		t.Fatal(err)
	}
	res, err = a.ImportTemplates()
	if err != nil || res.Added != 2 || res.Text != "Добавлено шаблонов: 2" {
		t.Fatalf("загрузка: %+v %v", res, err)
	}
	if d := a.dialogDir(dirTemplates); d != dir {
		t.Fatalf("папка диалога %q, ждали %q", d, dir)
	}
}
