package prefs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// puid — годный uid (32 знака hex нижнего регистра) по номеру.
func puid(n int) string { return fmt.Sprintf("%032x", n+1) }

// prec — запись проекта по номеру: поля те же, что у rec, привязана к path.
func prec(n int, path string) ProjectRecord { return ProjectOf(rec(n), path) }

// writeV1 пишет shares.json 0.37: записи с uid, ключи — как в файле (fileKey).
func writeV1(t *testing.T, dir string, recs map[string]ShareRecord) {
	t.Helper()
	items := map[string]json.RawMessage{}
	for p, r := range recs {
		items[fileKey(p)] = mustJSON(r)
	}
	writeSharesRaw(t, dir, 1, items)
}

func writeProjectsRaw(t *testing.T, dir string, version int, items map[string]json.RawMessage) {
	t.Helper()
	data, err := json.Marshal(projectsFile{Version: version, Projects: items})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ProjectsPath(dir), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Круг Put → новый объект → Get для нескольких проектов; пусто, права, пароль чужого компьютера.
func TestProjectsFile(t *testing.T) {
	dir := testkit.TempDir(t)
	p := NewProjects(dir, testRules)

	// Пусто: чтение не создаёт файл.
	if _, ok := p.Get(puid(0)); ok {
		t.Fatal("запись из пустоты")
	}
	if _, err := os.Lstat(ProjectsPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("чтение создало файл: %v", err)
	}

	const n = 5
	for i := range n {
		r := prec(i, albumPath(dir, fmt.Sprint(i)))
		r.PassUnknown = i%2 == 1
		if err := p.Put(puid(i), r); err != nil {
			t.Fatal(err)
		}
	}
	re := NewProjects(dir, testRules)
	for i := range n {
		got, ok := re.Get(puid(i))
		want := prec(i, filepath.Clean(albumPath(dir, fmt.Sprint(i))))
		if i%2 == 1 {
			want.PassUnknown, want.Password = true, "" // пароль чужого компьютера в запись не идёт
		}
		if !ok || !sameProject(got, want) {
			t.Errorf("запись %d: %v\n got %+v\nwant %+v", i, ok, got, want)
		}
	}
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(ProjectsPath(dir)); err != nil || st.Mode().Perm() != 0o600 {
			t.Errorf("права projects.json: %v, %v", st, err)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "projects.json.*")); len(left) != 0 {
		t.Errorf("остатки записи: %v", left)
	}
	if _, err := os.Lstat(SharesPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("shares.json появился: %v", err)
	}

}

// Update, Delete: правка доходит до диска; записи нет — fn не зовётся, файл не пишется.
func TestProjectsEdit(t *testing.T) {
	dir := testkit.TempDir(t)
	const n = 5
	re := NewProjects(dir, testRules)
	for i := range n {
		if err := re.Put(puid(i), prec(i, albumPath(dir, fmt.Sprint(i)))); err != nil {
			t.Fatal(err)
		}
	}
	got, ok, err := re.Update(puid(1), func(r *ProjectRecord) { r.Title, r.Path = "Новое", "" })
	if err != nil || !ok || got.Title != "Новое" || got.Path != "" {
		t.Fatalf("Update: %+v %v %v", got, ok, err)
	}
	if again, _ := NewProjects(dir, testRules).Get(puid(1)); again.Title != "Новое" || again.Path != "" {
		t.Errorf("правка не на диске: %+v", again)
	}
	before := readFile(t, ProjectsPath(dir))
	called := false
	if _, ok, err := re.Update(puid(99), func(*ProjectRecord) { called = true }); ok || err != nil || called {
		t.Errorf("Update без записи: %v %v %v", ok, err, called)
	}
	if err := re.Delete(puid(99)); err != nil {
		t.Errorf("Delete без записи: %v", err)
	}
	if !bytes.Equal(before, readFile(t, ProjectsPath(dir))) {
		t.Error("холостые правки переписали файл")
	}
	// Правка не по правилам — отказ, запись прежняя.
	if _, _, err := re.Update(puid(2), func(r *ProjectRecord) { r.ID = "x" }); !errors.Is(err, ErrShareRecord) {
		t.Errorf("Update с негодным id: %v", err)
	}

	// Удаление всех: файл остаётся (проекты не импортируются заново), список пуст.
	for i := range n {
		if err := re.Delete(puid(i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := NewProjects(dir, testRules).Get(puid(0)); ok {
		t.Error("запись пережила Delete")
	}
	if _, err := os.Lstat(ProjectsPath(dir)); err != nil {
		t.Errorf("пустой файл исчез: %v", err)
	}
}

// Ключ — uid: негодный (верхний регистр, длина, не hex, пусто) не принимается и не читается.
func TestProjectsKeys(t *testing.T) {
	dir := testkit.TempDir(t)
	p := NewProjects(dir, testRules)
	bad := []string{"", "0123456789ABCDEF0123456789ABCDEF", "0123456789abcdef", "0123456789abcdef0123456789abcdeg",
		"0123456789abcdef0123456789abcdef0", "/a/b.salbum"}
	for _, u := range bad {
		if err := p.Put(u, prec(1, albumPath(dir, "a"))); !errors.Is(err, ErrProjectUID) {
			t.Errorf("Put(%q): %v", u, err)
		}
		if _, _, err := p.PutIfAbsent(u, prec(1, "")); !errors.Is(err, ErrProjectUID) {
			t.Errorf("PutIfAbsent(%q): %v", u, err)
		}
		if _, _, err := p.Update(u, func(*ProjectRecord) {}); !errors.Is(err, ErrProjectUID) {
			t.Errorf("Update(%q): %v", u, err)
		}
		if err := p.Delete(u); !errors.Is(err, ErrProjectUID) {
			t.Errorf("Delete(%q): %v", u, err)
		}
		if _, ok := p.Get(u); ok {
			t.Errorf("Get(%q)", u)
		}
	}
	if _, err := os.Lstat(ProjectsPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("негодный ключ создал файл: %v", err)
	}
	// В файле негодные ключи отбрасываются, годный остаётся.
	items := map[string]json.RawMessage{puid(1): mustJSON(prec(1, "")), strings.ToUpper(puid(11)): mustJSON(prec(2, "")),
		"/a/b.salbum": mustJSON(prec(3, ""))}
	writeProjectsRaw(t, dir, 1, items)
	re := NewProjects(dir, testRules)
	if _, ok := re.Get(puid(1)); !ok {
		t.Error("годный ключ потерян")
	}
	if recs, _ := re.load(); len(recs) != 1 {
		t.Errorf("негодные ключи прочитаны: %d", len(recs))
	}
	// Негодные записи (адрес, id, пароль) тоже отбрасываются.
	badRec := prec(4, "")
	badRec.Origin = "ftp://x"
	if err := p.Put(puid(4), badRec); !errors.Is(err, ErrShareRecord) {
		t.Errorf("Put с негодным адресом: %v", err)
	}
}

// Импорт: записи shares.json уходят в projects.json по uid, путь-ключ — в path; записи без
// годного uid пропускаются; shares.json не меняется.
func TestProjectsImportV1(t *testing.T) {
	dir := testkit.TempDir(t)
	a, b, c := albumPath(dir, "a"), albumPath(dir, "b"), albumPath(dir, "c")
	ra, rb, rc := rec(1), rec(2), rec(3)
	ra.UID, rb.UID, rc.UID = puid(1), puid(2), "" // у c uid нет: чужая запись 0.31
	writeV1(t, dir, map[string]ShareRecord{a: ra, b: rb, c: rc})
	before := readFile(t, SharesPath(dir))

	p := NewProjects(dir, testRules)
	got, ok := p.Get(puid(1))
	if !ok || got.Path != fileKey(a) || got.ID != ra.ID || got.Title != ra.Title || got.Password != ra.Password ||
		!got.ExpiresAt.Equal(ra.ExpiresAt) || !got.PublishedAt.Equal(ra.PublishedAt) || got.Scene != ra.Scene {
		t.Errorf("импорт a: %v %+v", ok, got)
	}
	if got, ok := p.Get(puid(2)); !ok || got.Path != fileKey(b) || got.ID != rb.ID {
		t.Errorf("импорт b: %v %+v", ok, got)
	}
	if recs, _ := p.load(); len(recs) != 2 {
		t.Errorf("записей %d, ждали 2 (без uid пропущена)", len(recs))
	}
	// Файл лёг сразу, права те же, что у shares.json; новый объект читает его, а не shares.json.
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(ProjectsPath(dir)); err != nil || st.Mode().Perm() != 0o600 {
			t.Errorf("права projects.json: %v, %v", st, err)
		}
	}
	if !bytes.Equal(before, readFile(t, SharesPath(dir))) {
		t.Error("shares.json изменён импортом")
	}
	// Одноразово: shares.json дополнили — projects.json уже есть, повторного импорта нет.
	rd := rec(4)
	rd.UID = puid(4)
	writeV1(t, dir, map[string]ShareRecord{a: ra, b: rb, albumPath(dir, "d"): rd})
	if _, ok := NewProjects(dir, testRules).Get(puid(4)); ok {
		t.Error("импорт повторился при готовом projects.json")
	}
	// Удалили всё — тоже не возвращается из shares.json.
	re := NewProjects(dir, testRules)
	for _, u := range []string{puid(1), puid(2)} {
		if err := re.Delete(u); err != nil {
			t.Fatal(err)
		}
	}
	if recs, _ := NewProjects(dir, testRules).load(); len(recs) != 0 {
		t.Errorf("записи вернулись из shares.json: %d", len(recs))
	}
}

// shares.json после любой работы с projects.json — побайтно прежний (откат на 0.37), в том числе
// битый и «новее»: они не уходят в .bad и не переписываются.
func TestProjectsKeepsSharesV1(t *testing.T) {
	a := rec(1)
	a.UID = puid(1)
	cases := map[string]func(t *testing.T, dir string){
		"исправный": func(t *testing.T, dir string) { writeV1(t, dir, map[string]ShareRecord{albumPath(dir, "a"): a}) },
		"битый": func(t *testing.T, dir string) {
			if err := os.WriteFile(SharesPath(dir), []byte(`{"version":1,"albums":{`), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"пустой": func(t *testing.T, dir string) {
			if err := os.WriteFile(SharesPath(dir), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"новее": func(t *testing.T, dir string) {
			writeSharesRaw(t, dir, 2, map[string]json.RawMessage{fileKey(albumPath(dir, "a")): mustJSON(a)})
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			dir := testkit.TempDir(t)
			setup(t, dir)
			before := readFile(t, SharesPath(dir))
			p := NewProjects(dir, testRules)
			p.Get(puid(1))
			if err := p.Put(puid(7), prec(7, albumPath(dir, "z"))); err != nil {
				t.Fatal(err)
			}
			if _, _, err := p.Update(puid(7), func(r *ProjectRecord) { r.Gone = true }); err != nil {
				t.Fatal(err)
			}
			if err := p.Delete(puid(7)); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, readFile(t, SharesPath(dir))) {
				t.Error("shares.json изменён")
			}
			if _, err := os.Lstat(SharesPath(dir) + ".bad"); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("shares.json уведён в .bad: %v", err)
			}
			// 0.37 (Shares) по-прежнему читает свой файл так же, как до 0.38.
			if name == "исправный" {
				if got, ok := NewShares(dir, testRules).Get(albumPath(dir, "a")); !ok || got.ID != a.ID {
					t.Errorf("Shares не читает свой файл: %v %+v", ok, got)
				}
			}
		})
	}
}

// Дубли uid в shares.json: победитель не зависит от порядка чтения (map в Go читается в случайном
// порядке — 100 чтений с нуля). Порядок: PublishedAt, ExpiresAt, ключ-путь, id.
func TestSharesMigrateDupTieStable(t *testing.T) {
	t0 := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	mk := func(n int, pub, exp time.Time) ShareRecord {
		r := rec(n)
		r.UID, r.PublishedAt, r.ExpiresAt = puid(1), pub, exp
		return r
	}
	exp := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	cases := []struct {
		name string
		recs map[string]ShareRecord // ключ — имя альбома
		want string                 // победитель — имя альбома
	}{
		{"позднее PublishedAt", map[string]ShareRecord{
			"a": mk(1, t0, exp), "b": mk(2, t0.Add(time.Hour), exp), "c": mk(3, t0.Add(-time.Hour), exp)}, "b"},
		{"равные даты — позднее ExpiresAt", map[string]ShareRecord{
			"a": mk(1, t0, exp), "b": mk(2, t0, exp.Add(time.Hour)), "c": mk(3, t0, exp.Add(-time.Hour))}, "b"},
		{"полностью равные — меньший ключ-путь", map[string]ShareRecord{
			"c": mk(3, t0, exp), "a": mk(1, t0, exp), "b": mk(2, t0, exp), "d": mk(4, t0, exp)}, "a"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := testkit.TempDir(t)
			in := map[string]ShareRecord{}
			for name, r := range tc.recs {
				in[albumPath(dir, name)] = r
			}
			writeV1(t, dir, in)
			want := tc.recs[tc.want]
			for i := range 100 {
				_ = os.Remove(ProjectsPath(dir))
				got, ok := NewProjects(dir, testRules).Get(puid(1))
				if !ok || got.ID != want.ID || got.Path != fileKey(albumPath(dir, tc.want)) {
					t.Fatalf("чтение %d: победитель %+v (%v), ждали %s", i, got, ok, tc.want)
				}
			}
		})
	}
	// Дубль ушёл целиком: другие uid не задеты.
	dir := testkit.TempDir(t)
	other := rec(9)
	other.UID = puid(9)
	writeV1(t, dir, map[string]ShareRecord{albumPath(dir, "a"): mk(1, t0, exp), albumPath(dir, "b"): mk(2, t0, exp), albumPath(dir, "o"): other})
	if recs, _ := NewProjects(dir, testRules).load(); len(recs) != 2 {
		t.Errorf("записей %d, ждали 2", len(recs))
	}
}

// Ключ shares.json на Windows и macOS — путь в нижнем регистре: название «по имени файла» всё
// равно узнаётся (сравнение через paths.Fold); введённое руками — нет.
func TestSharesMigrateFoldedTitleAuto(t *testing.T) {
	dir := testkit.TempDir(t)
	p := filepath.Join(dir, "Альбомы", "Иванов Мария.salbum")
	auto, typed := rec(1), rec(2)
	auto.UID, auto.Title = puid(1), "Иванов Мария"
	typed.UID, typed.Title = puid(2), "Мария и Пётр"
	writeV1(t, dir, map[string]ShareRecord{p: auto, filepath.Join(dir, "Альбомы", "Другой.salbum"): typed})

	pr := NewProjects(dir, testRules)
	got, ok := pr.Get(puid(1))
	if !ok || got.Path != fileKey(p) {
		t.Fatalf("импорт: %v %+v", ok, got)
	}
	if !got.TitleAuto(nil) {
		t.Errorf("название по имени файла не узнано (путь %q, название %q)", got.Path, got.Title)
	}
	if t2, _ := pr.Get(puid(2)); t2.TitleAuto(nil) {
		t.Errorf("введённое руками названо автоматическим: %+v", t2)
	}
}

// TitleAuto/TitleIs: очистка названия, регистр, запись без пути.
func TestProjectsTitleAuto(t *testing.T) {
	up := func(s string) string { return strings.TrimSpace(strings.ToUpper(s)) }
	r := ProjectRecord{Title: "ИВАНОВ", Path: filepath.Join(string(filepath.Separator), "x", "иванов.salbum")}
	if want := paths.Fold("ИВАНОВ") == paths.Fold("иванов"); r.TitleAuto(nil) != want {
		t.Errorf("без очистки: %v, ждали %v (регистр по правилам ОС)", r.TitleAuto(nil), want)
	}
	if !r.TitleAuto(up) {
		t.Error("очистка не применена к имени файла")
	}
	r.Path = ""
	if r.TitleAuto(up) {
		t.Error("запись без пути: признак неизвестен, ждали false")
	}
	if !r.TitleIs("иванов", up) {
		t.Error("TitleIs по названию задания")
	}
	r.Title = "Другое"
	if r.TitleIs("иванов", up) {
		t.Error("разные названия равны")
	}
}

// Файл пишется с "version" первым: по нему peekShareVersion узнаёт «новее» в файле, который не
// разобрать целиком, — он не уходит в .bad (как у shares.json).
func TestProjectsVersionFirst(t *testing.T) {
	dir := testkit.TempDir(t)
	p := NewProjects(dir, testRules)
	if err := p.Put(puid(1), prec(1, albumPath(dir, "a"))); err != nil {
		t.Fatal(err)
	}
	data := readFile(t, ProjectsPath(dir))
	if !strings.HasPrefix(string(data), "{\n  \"version\": 1,") || peekShareVersion(data) != 1 {
		t.Errorf("version не первой: %.40q", data)
	}
	// Файл будущей версии с телом, которое не разобрать, — «новее», не битый.
	for name, body := range map[string]string{
		"тело другого вида": `{"version":2,"projects":"x"}`,
		"усечён":            `{"version":3,"projects":{"` + puid(1) + `":{"origin":`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := testkit.TempDir(t)
			if err := os.WriteFile(ProjectsPath(dir), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			p := NewProjects(dir, testRules)
			if err := p.Put(puid(1), prec(1, "")); !errors.Is(err, ErrProjectsNewer) {
				t.Errorf("Put: %v", err)
			}
			if got := readFile(t, ProjectsPath(dir)); string(got) != body {
				t.Error("файл новой версии изменён")
			}
			if _, err := os.Lstat(ProjectsPath(dir) + ".bad"); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("файл новой версии уведён в .bad: %v", err)
			}
		})
	}
}

// Нелокальные пути в path (UNC, \\?\, устройство, относительный, с управляющими знаками)
// читаются как "": запись остаётся, но ни к какому файлу не привязана. Put чистит так же.
func TestSharesV2BadPath(t *testing.T) {
	bad := []string{"//srv/share/a.salbum", `\\srv\share\a.salbum`, `\\?\C:\a.salbum`, `\\.\pipe\x`,
		`C:\CON`, `C:\dir\NUL.salbum`, "CON", "относительный/a.salbum", "a.salbum", "/a/b\x00.salbum", "/a/b\n.salbum",
		"/a/\xff.salbum"}
	dir := testkit.TempDir(t)
	items := map[string]json.RawMessage{}
	for i, p := range bad {
		if utf8.ValidString(p) { // json.Marshal подменил бы негодный UTF-8 знаком U+FFFD: его проверяет Put ниже
			items[puid(i)] = mustJSON(prec(i, p))
		}
	}
	good := albumPath(dir, "ok")
	items[puid(len(bad))] = mustJSON(prec(len(bad), good))
	writeProjectsRaw(t, dir, 1, items)

	p := NewProjects(dir, testRules)
	for i, path := range bad {
		if !utf8.ValidString(path) {
			continue
		}
		got, ok := p.Get(puid(i))
		if !ok || got.Path != "" || got.Own(path) {
			t.Errorf("путь %q: запись %v, path %q", path, ok, got.Path)
		}
	}
	if got, ok := p.Get(puid(len(bad))); !ok || got.Path != filepath.Clean(good) || !got.Own(good) {
		t.Errorf("годный путь: %v %+v", ok, got)
	}
	for i, path := range bad {
		if err := p.Put(puid(100+i), prec(i, path)); err != nil {
			t.Fatalf("Put(%q): %v", path, err)
		}
		if got, _ := NewProjects(dir, testRules).Get(puid(100 + i)); got.Path != "" {
			t.Errorf("Put сохранил путь %q", got.Path)
		}
	}
	// Сравнение путей: Clean и регистр по правилам ОС.
	r := prec(1, good+string(filepath.Separator)+".")
	if r.Own("") || (ProjectRecord{}).Own(good) {
		t.Error("пустой путь ничей")
	}
	if !prec(1, good).Own(filepath.Join(filepath.Dir(good), ".", filepath.Base(good))) {
		t.Error("Clean не учтён")
	}
}

// Файл новее программы: записи читаются, писать нельзя, файл цел.
func TestProjectsNewerReadOnly(t *testing.T) {
	dir := testkit.TempDir(t)
	writeProjectsRaw(t, dir, sharesVersion+1, map[string]json.RawMessage{puid(1): mustJSON(prec(1, albumPath(dir, "a")))})
	before := readFile(t, ProjectsPath(dir))
	p := NewProjects(dir, testRules)
	if _, ok := p.Get(puid(1)); !ok {
		t.Error("запись новой версии не прочитана")
	}
	if err := p.Put(puid(2), prec(2, "")); !errors.Is(err, ErrProjectsNewer) {
		t.Errorf("Put: %v", err)
	}
	if _, _, err := p.PutIfAbsent(puid(3), prec(3, "")); !errors.Is(err, ErrProjectsNewer) {
		t.Errorf("PutIfAbsent: %v", err)
	}
	if _, _, err := p.Update(puid(1), func(r *ProjectRecord) { r.Gone = true }); !errors.Is(err, ErrProjectsNewer) {
		t.Errorf("Update: %v", err)
	}
	if err := p.Delete(puid(1)); !errors.Is(err, ErrProjectsNewer) {
		t.Errorf("Delete: %v", err)
	}
	if !bytes.Equal(before, readFile(t, ProjectsPath(dir))) {
		t.Error("файл новой версии изменён")
	}
}

// PutIfAbsent: пусто — кладёт; занято — возвращает прежнюю и ничего не пишет; вдвойне — под замком:
// из многих одновременных ровно одна выигрывает.
func TestProjectsPutIfAbsent(t *testing.T) {
	dir := testkit.TempDir(t)
	p := NewProjects(dir, testRules)
	first := prec(1, "")
	cur, added, err := p.PutIfAbsent(puid(1), first)
	if err != nil || !added || cur.ID != first.ID {
		t.Fatalf("первая: %+v %v %v", cur, added, err)
	}
	before := readFile(t, ProjectsPath(dir))
	cur, added, err = p.PutIfAbsent(puid(1), prec(2, albumPath(dir, "x")))
	if err != nil || added || cur.ID != first.ID || cur.Path != "" {
		t.Fatalf("вторая затёрла: %+v %v %v", cur, added, err)
	}
	if !bytes.Equal(before, readFile(t, ProjectsPath(dir))) {
		t.Error("отказ PutIfAbsent переписал файл")
	}
	if _, _, err := p.PutIfAbsent(puid(5), ProjectRecord{ID: "x"}); !errors.Is(err, ErrShareRecord) {
		t.Errorf("негодная запись: %v", err)
	}

	// Гонка: 12 объектов (как несколько ходов) на один uid.
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		winners []string
	)
	for i := range 12 {
		wg.Go(func() {
			cur, added, err := NewProjects(dir, testRules).PutIfAbsent(puid(7), prec(10+i, ""))
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if added {
				winners = append(winners, cur.ID)
			}
		})
	}
	wg.Wait()
	got, _ := NewProjects(dir, testRules).Get(puid(7))
	if len(winners) != 1 || winners[0] != got.ID {
		t.Errorf("победителей %v, на диске %s", winners, got.ID)
	}
}

// Битый, пустой, слишком большой и «не тот» файл — в .bad; усечённые и огромные значения внутри
// записей — без паники и без записи; после .bad работа идёт дальше.
func TestProjectsBadFile(t *testing.T) {
	many := map[string]json.RawMessage{}
	for i := range maxShares + 1 {
		many[puid(i)] = json.RawMessage(`{}`)
	}
	manyData, _ := json.Marshal(projectsFile{Version: 1, Projects: many})
	cases := map[string]string{
		"битый JSON":     `{"version":1,"projects":{`,
		"пустой":         ``,
		"нет версии":     `{"projects":{}}`,
		"массив":         `[]`,
		"больше предела": `{"version":1,"projects":{"x":"` + strings.Repeat("a", maxSharesFile) + `"}}`,
		"записей > 1000": string(manyData),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			dir := testkit.TempDir(t)
			if err := os.WriteFile(ProjectsPath(dir), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			p := NewProjects(dir, testRules)
			if _, ok := p.Get(puid(0)); ok {
				t.Fatal("запись из битого файла")
			}
			if got, err := os.ReadFile(ProjectsPath(dir) + ".bad"); err != nil || string(got) != content {
				t.Errorf(".bad: %v", err)
			}
			if err := p.Put(puid(1), prec(1, "")); err != nil {
				t.Fatal(err)
			}
			if _, ok := NewProjects(dir, testRules).Get(puid(1)); !ok {
				t.Error("новая запись не записалась после .bad")
			}
		})
	}
	// Записи с негодными и огромными полями: отброшены, остальные целы.
	dir := testkit.TempDir(t)
	huge := prec(3, "")
	huge.Title = strings.Repeat("я", 100000)
	hugePass := prec(4, "")
	hugePass.Password = strings.Repeat("p", maxSharePass+1)
	writeProjectsRaw(t, dir, 1, map[string]json.RawMessage{
		puid(1): mustJSON(prec(1, "")), puid(2): json.RawMessage(`{"expires_at":"x"}`), puid(3): mustJSON(huge),
		puid(4): mustJSON(hugePass), puid(5): json.RawMessage(`null`), puid(6): json.RawMessage(`[1]`),
	})
	p := NewProjects(dir, testRules)
	if _, ok := p.Get(puid(1)); !ok {
		t.Error("годная запись потеряна")
	}
	if got, ok := p.Get(puid(3)); !ok || len([]rune(got.Title)) > maxShareTitle {
		t.Errorf("огромное название не усечено: %v %d", ok, len([]rune(got.Title)))
	}
	for _, u := range []string{puid(2), puid(4), puid(5), puid(6)} {
		if _, ok := p.Get(u); ok {
			t.Errorf("негодная запись %s принята", u)
		}
	}
}

// Предел записей и файла: больше — ErrProjectsFull, файл прежний.
func TestProjectsLimit(t *testing.T) {
	dir := testkit.TempDir(t)
	p := NewProjects(dir, testRules)
	items := map[string]json.RawMessage{}
	for i := range maxShares {
		items[puid(i)] = mustJSON(prec(i%90, ""))
	}
	writeProjectsRaw(t, dir, 1, items)
	if err := p.Put(puid(maxShares), prec(1, "")); !errors.Is(err, ErrProjectsFull) {
		t.Errorf("Put сверх предела: %v", err)
	}
	if _, _, err := p.PutIfAbsent(puid(maxShares), prec(1, "")); !errors.Is(err, ErrProjectsFull) {
		t.Errorf("PutIfAbsent сверх предела: %v", err)
	}
	if err := p.Put(puid(0), prec(2, "")); err != nil {
		t.Errorf("замена при полном списке: %v", err)
	}
}

// Истёкшая больше 30 дней назад запись выбрасывается при записи, свежая и без срока остаются.
func TestProjectsExpire(t *testing.T) {
	dir := testkit.TempDir(t)
	p := NewProjects(dir, testRules)
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return now }
	old, fresh, none := prec(1, ""), prec(2, ""), prec(3, "")
	old.ExpiresAt, fresh.ExpiresAt, none.ExpiresAt = now.Add(-31*24*time.Hour), now.Add(-29*24*time.Hour), time.Time{}
	writeProjectsRaw(t, dir, 1, map[string]json.RawMessage{puid(1): mustJSON(old), puid(2): mustJSON(fresh), puid(3): mustJSON(none)})
	if err := p.Put(puid(4), prec(4, "")); err != nil {
		t.Fatal(err)
	}
	re := NewProjects(dir, testRules)
	if _, ok := re.Get(puid(1)); ok {
		t.Error("давно истёкшая запись осталась")
	}
	for _, u := range []string{puid(2), puid(3), puid(4)} {
		if _, ok := re.Get(u); !ok {
			t.Errorf("запись %s потеряна", u)
		}
	}
}

// Без папки данных записи живут в памяти, импорта нет.
func TestProjectsNoDir(t *testing.T) {
	p := NewProjects("", testRules)
	if ProjectsPath("") != "" {
		t.Error("путь без папки")
	}
	if err := p.Put(puid(1), prec(1, "")); err != nil {
		t.Fatal(err)
	}
	if _, added, err := p.PutIfAbsent(puid(1), prec(2, "")); err != nil || added {
		t.Errorf("PutIfAbsent в памяти: %v %v", added, err)
	}
	if got, ok := p.Get(puid(1)); !ok || got.ID != rec(1).ID {
		t.Errorf("запись в памяти: %v %+v", ok, got)
	}
	if err := p.Delete(puid(1)); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Get(puid(1)); ok {
		t.Error("запись пережила Delete")
	}
}

// FuzzProjectsFile: любое содержимое projects.json — без паники; принятые записи проходят
// правила повторно без изменений.
func FuzzProjectsFile(f *testing.F) {
	good, _ := json.Marshal(projectsFile{Version: 1, Projects: map[string]json.RawMessage{puid(1): mustJSON(prec(1, "/a/b.salbum"))}})
	for _, seed := range []string{"", "{}", "[]", "null", `{"version":1}`, `{"version":1,"projects":null}`, string(good),
		`{"version":9,"projects":{"` + puid(1) + `":{}}}`, `{"version":1,"projects":{"` + puid(1) + `":{"path":"\\\\?\\x","expires_at":"x"}}}`} {
		f.Add([]byte(seed))
	}
	dir := testkit.TempDir(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		if err := os.WriteFile(ProjectsPath(dir), data, 0o600); err != nil {
			t.Fatal(err)
		}
		_ = os.Remove(ProjectsPath(dir) + ".bad")
		p := NewProjects(dir, testRules)
		recs, _ := p.load()
		for k, r := range recs {
			if again, ok := testRules.cleanProject(r); !ok || !sameProject(again, r) {
				t.Fatalf("принятая запись %q не идемпотентна: %+v", k, r)
			}
		}
	})
}
