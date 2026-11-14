package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/app/assoc"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// newerAlbum — файл альбома, сохранённый версией appVer (на формат новее программы).
func newerAlbum(t *testing.T, dir, name, appVer string) string {
	t.Helper()
	path := filepath.Join(dir, name+projectExt)
	body := fmt.Sprintf(`{"version":%d,"app":%q}`, model.FormatVersion+1, appVer)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// newerExes — настоящие файлы «программ» в своей папке; возвращает пути по именам.
func newerExes(t *testing.T, names ...string) []string {
	t.Helper()
	dir := testkit.TempDir(t)
	out := make([]string, 0, len(names))
	for _, n := range names {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

// newerApp — App 0.25.0 с реестром в памяти; временной считается несуществующая папка (тестовые
// файлы лежат в TMPDIR, и иначе их приняли бы за запуск из архива).
func newerApp(t *testing.T, reg *fakeReg) *assocRig {
	t.Helper()
	reg.self = myExe
	r := newAssocRig(t, reg)
	r.tempFn = func() string {
		return filepath.Join(string(filepath.Separator), "нет-такой-временной")
	}
	return r
}

// Файл новее программы: OpenProject/OpenRecent/OpenExternal (общий openPath) отвечают
// State{cancelled, newer} без ошибки, прежний альбом на месте; Have — только если подходящая
// версия есть на компьютере; старт отдаёт Newer первым GetState один раз.
func TestOpenNewerState(t *testing.T) {
	exes := newerExes(t, "SosninaAlbums-0.26.0.exe", "SosninaAlbums-0.27.0.exe", "SosninaAlbums-0.25.9.exe", "other.exe")
	e26, e27, e259, other := exes[0], exes[1], exes[2], exes[3]
	dir := testkit.TempDir(t)
	cases := []struct {
		name string
		st   osx.AssocState
		app  string // версия в файле
		have string
	}{
		{"нет других версий", osx.AssocState{}, "0.26.0", ""},
		{"есть 0.26 по связи", osx.AssocState{ProgID: progOurs, Exe: e26, AppVersion: "0.26.0"}, "0.26.0", "0.26"},
		{"установка без своего exe", osx.AssocState{InstallDir: filepath.Dir(e26), InstallVersion: "0.26.1"}, "0.26.0", ""},
		{"версия ниже нужной", osx.AssocState{ProgID: progOurs, Exe: e259, AppVersion: "0.25.9"}, "0.26.0", ""},
		{"выше нужной — подходит", osx.AssocState{ProgID: progOurs, Exe: e27, AppVersion: "0.27.0"}, "0.26.0", "0.27"},
		{"в файле версии нет — любая новее", osx.AssocState{ProgID: progOurs, Exe: e259, AppVersion: "0.25.9"}, "", "0.25"},
		{"чужое имя exe", osx.AssocState{ProgID: progOurs, Exe: other, AppVersion: "0.26.0"}, "0.26.0", ""},
		{"exe пропал", osx.AssocState{ProgID: progOurs, Exe: filepath.Join(dir, "SosninaAlbums-0.26.0.exe"), AppVersion: "0.26.0"}, "0.26.0", ""},
	}
	for _, c := range cases {
		reg := &fakeReg{st: c.st}
		a := newerApp(t, reg)
		before := a.editor()
		path := newerAlbum(t, dir, "Свадьба", c.app)
		st, err := a.openPath(path)
		if err != nil || !st.Cancelled || st.Newer == nil {
			t.Errorf("%s: %+v, %v", c.name, st, err)
			continue
		}
		want := NewerInfo{Name: "Свадьба" + projectExt, Need: assoc.ShortVer(c.app), Mine: "0.25", Have: c.have}
		if *st.Newer != want {
			t.Errorf("%s: %+v, ждали %+v", c.name, *st.Newer, want)
		}
		if a.editor() != before || a.editor().Path() != "" {
			t.Errorf("%s: прежний альбом заменён", c.name)
		}
		if len(a.loadConfig().Recent) != 0 {
			t.Errorf("%s: файл попал в «Последние»", c.name)
		}
	}
	// Если подходящих несколько — берётся новейшая, а не первая.
	reg := &fakeReg{st: osx.AssocState{ProgID: progOurs, Exe: e26, AppVersion: "0.26.0", InstallDir: filepath.Dir(e27), InstallVersion: "0.27.0"}}
	a := newerApp(t, reg)
	_ = os.Rename(e27, filepath.Join(filepath.Dir(e27), osx.InstallExeName))
	st, _ := a.openPath(newerAlbum(t, dir, "Два", "0.26.0"))
	if st.Newer == nil || st.Newer.Have != "0.27" {
		t.Errorf("новейшая из двух: %+v", st.Newer)
	}
	// Обычный файл открывается как раньше: Newer нет.
	good := filepath.Join(dir, "ok"+projectExt)
	if err := salbum.Save(model.New(), good); err != nil {
		t.Fatal(err)
	}
	if st, err := a.openPath(good); err != nil || st.Cancelled || st.Newer != nil {
		t.Errorf("обычный файл: %+v, %v", st, err)
	}
	// Старт: Newer в первом GetState, во втором уже нет; GetState без отказа — пустой.
	b := newerApp(t, &fakeReg{})
	if b.GetState().Newer != nil {
		t.Error("Newer без отказа")
	}
	b.startNewer("", true, &salbum.NewerError{Version: model.FormatVersion + 1, App: "0.26.0"})
	first, second := b.GetState(), b.GetState()
	if first.Newer == nil || !first.Newer.Draft || first.Newer.Name != "" || first.Newer.Need != "0.26" || second.Newer != nil {
		t.Errorf("старт: %+v, повтор %+v", first.Newer, second.Newer)
	}
}

// Окно «из новой версии»: две цифры совпали (файл 0.25.1, программа 0.25.0) — три цифры у файла, программы
// и найденной копии; разошлись — две, как раньше.
func TestOpenNewerStateSameMinor(t *testing.T) {
	exes := newerExes(t, "SosninaAlbums-0.25.1.exe", "SosninaAlbums-0.26.0.exe")
	dir := testkit.TempDir(t)
	cases := []struct {
		name       string
		exe        string
		ver, app   string
		need, mine string
		have       string
	}{
		{"совпали", exes[0], "0.25.1", "0.25.1", "0.25.1", "0.25.0", "0.25.1"},
		{"совпали, копии нет", "", "", "0.25.1", "0.25.1", "0.25.0", ""},
		{"разошлись", exes[1], "0.26.0", "0.26.0", "0.26", "0.25", "0.26"},
	}
	for _, c := range cases {
		reg := &fakeReg{}
		if c.exe != "" {
			reg.st = osx.AssocState{ProgID: progOurs, Exe: c.exe, AppVersion: c.ver}
		}
		a := newerApp(t, reg)
		st, err := a.openPath(newerAlbum(t, dir, "Свадьба", c.app))
		if err != nil || st.Newer == nil {
			t.Errorf("%s: %+v, %v", c.name, st, err)
			continue
		}
		if got := *st.Newer; got.Need != c.need || got.Mine != c.mine || got.Have != c.have {
			t.Errorf("%s: need=%q mine=%q have=%q; ждали %q %q %q", c.name, got.Need, got.Mine, got.Have, c.need, c.mine, c.have)
		}
	}
}

// OpenInNewer: альбом сохраняется, новая версия запускается как `--after-pid <pid> <файл>`
// (у черновика без файла), программа закрывается; сбой запуска — ошибка, программа жива.
func TestOpenInNewer(t *testing.T) {
	exes := newerExes(t, "SosninaAlbums-0.26.0.exe")
	exe := exes[0]
	reg := &fakeReg{st: osx.AssocState{ProgID: progOurs, Exe: exe, AppVersion: "0.26.0"}}
	dir := testkit.TempDir(t)
	newerErr := &salbum.NewerError{Version: model.FormatVersion + 1, App: "0.26.0"}

	// rec подменяет запуск и выход; run сбрасывает его перед каждым шагом.
	var rec struct {
		args  [][]string
		quits int
	}
	run := func(a *assocRig) {
		rec.args, rec.quits = nil, 0
		a.startFn = func(e string, as ...string) error {
			rec.args = append(rec.args, append([]string{e}, as...))
			return nil
		}
		a.quitFn = func() { rec.quits++ }
	}

	// Нет отказа — нечего открывать.
	a := newerApp(t, reg)
	run(a)
	if err := a.OpenInNewer(); !errors.Is(err, assoc.ErrNoNewer) || len(rec.args) != 0 || rec.quits != 0 {
		t.Errorf("без отказа: %v, запусков %d, выходов %d", err, len(rec.args), rec.quits)
	}

	// Альбом: несохранённая правка пишется, запуск с файлом, один выход.
	path := newerAlbum(t, dir, "Свадьба", "0.26.0")
	if st, err := a.openPath(path); err != nil || st.Newer == nil || st.Newer.Have != "0.26" {
		t.Fatalf("отказ: %+v, %v", st, err)
	}
	a.changed(a.editor().View())
	run(a)
	if err := a.OpenInNewer(); err != nil {
		t.Fatal(err)
	}
	want := []string{exe, afterPIDFlag, strconv.Itoa(os.Getpid()), path}
	if len(rec.args) != 1 || fmt.Sprint(rec.args[0]) != fmt.Sprint(want) || rec.quits != 1 {
		t.Errorf("запуск: %v (ждали %v), выходов %d", rec.args, want, rec.quits)
	}
	if _, err := os.Stat(a.draftPath()); err != nil {
		t.Errorf("перед запуском не сохранено: %v", err)
	}

	// Черновик: без файла.
	a.setNewer("", true, newerErr)
	run(a)
	if err := a.OpenInNewer(); err != nil || len(rec.args) != 1 || len(rec.args[0]) != 3 || rec.quits != 1 {
		t.Errorf("черновик: %v, %v, выходов %d", err, rec.args, rec.quits)
	}

	// Сбой запуска: ошибка, не выходим, альбом прежний.
	a.setNewer(path, false, newerErr)
	before := a.editor()
	rec.quits = 0
	a.startFn = func(string, ...string) error { return errors.New("отказ ОС") }
	if err := a.OpenInNewer(); err == nil || rec.quits != 0 || a.editor() != before {
		t.Errorf("сбой запуска: %v, выходов %d", err, rec.quits)
	}

	// exe пропал после поиска: ошибка, запуска нет.
	run(a)
	if err := os.Remove(exe); err != nil {
		t.Fatal(err)
	}
	if err := a.OpenInNewer(); !errors.Is(err, assoc.ErrNoNewer) || len(rec.args) != 0 || rec.quits != 0 {
		t.Errorf("exe пропал: %v, запусков %d, выходов %d", err, len(rec.args), rec.quits)
	}

	// Найденная во временной папке (запуск из архива) не запускается.
	tmpExe := newerExes(t, "SosninaAlbums-0.26.0.exe")[0]
	b := newerApp(t, &fakeReg{st: osx.AssocState{ProgID: progOurs, Exe: tmpExe, AppVersion: "0.26.0"}})
	b.tempFn = func() string { return filepath.Dir(tmpExe) }
	if info := b.setNewer(path, false, newerErr); info.Have != "" {
		t.Errorf("exe во временной папке предложен: %+v", info)
	}
}
