package backup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// clock — подменённые часы хранилища.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newStore(t *testing.T) (*Store, *clock) {
	t.Helper()
	s := New(filepath.Join(testkit.TempDir(t), "backups"))
	c := &clock{time.Date(2026, 10, 9, 15, 30, 0, 0, time.Local)}
	s.now = c.now
	return s, c
}

func albumFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(testkit.TempDir(t), "Ивановы.salbum")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func rewrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func names(t *testing.T, s *Store, key string) []string {
	t.Helper()
	list, err := s.List(key)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(list))
	for i, e := range list {
		out[i] = e.Name
	}
	return out
}

// Первая перезапись за сеанс — копия; через 5 минут — нет; через 11 — есть. Копия — байт в байт.
func TestBackupFirstThenThrottle(t *testing.T) {
	s, c := newStore(t)
	const body = `{"version":4,"spreads":[{"id":"a"},{"id":"b"}],"photos":{"x":{},"y":{},"z":{}}}`
	p := albumFile(t, body)
	key := Key(p)
	if err := s.Before(p, key); err != nil {
		t.Fatal(err)
	}
	c.t = c.t.Add(5 * time.Minute)
	if err := s.Before(p, key); err != nil {
		t.Fatal(err)
	}
	if got := names(t, s, key); len(got) != 1 || got[0] != "20261009-153000.salbum" {
		t.Fatalf("через 5 минут: %v", got)
	}
	c.t = c.t.Add(6 * time.Minute)
	if err := s.Before(p, key); err != nil { // тот же файл — дубль не пишется
		t.Fatal(err)
	}
	if got := names(t, s, key); len(got) != 1 {
		t.Fatalf("дубль: %v", got)
	}
	const body2 = `{"version":4,"spreads":[{"id":"a"},{"id":"b"}],"photos":{"x":{},"y":{},"z":{}}} `
	rewrite(t, p, body2)
	if err := s.Before(p, key); err != nil {
		t.Fatal(err)
	}
	list, err := s.List(key)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "20261009-154100.salbum" {
		t.Fatalf("через 11 минут: %+v", list)
	}
	if e := list[0]; e.Spreads != 2 || e.Photos != 3 || e.Size != int64(len(body2)) || !e.Time.Equal(c.t) {
		t.Errorf("строка списка: %+v", e)
	}
	data, err := os.ReadFile(filepath.Join(s.Root, key, list[0].Name))
	if err != nil || string(data) != body2 {
		t.Fatalf("копия не байт в байт: %q, %v", data, err)
	}
	// Force — без учёта 10 минут; файла нет — копировать нечего.
	rewrite(t, p, body)
	if err := s.Force(p, key); err != nil || len(names(t, s, key)) != 3 {
		t.Fatalf("Force: %v, %v", err, names(t, s, key))
	}
	if err := s.Force(p+".нет", "другой-00000000"); err != nil || len(names(t, s, "другой-00000000")) != 0 {
		t.Fatalf("копия несуществующего файла: %v", err)
	}
}

// В папке альбома остаются 10 последних; «-2» новее копии без номера в ту же секунду.
func TestBackupKeepTen(t *testing.T) {
	s, c := newStore(t)
	p := albumFile(t, `{}`)
	key := Key(p)
	for i := range 13 {
		rewrite(t, p, fmt.Sprintf(`{"n":%d}`, i))
		if err := s.Force(p, key); err != nil {
			t.Fatal(err)
		}
		c.t = c.t.Add(time.Minute)
	}
	got := names(t, s, key)
	if len(got) != Keep || got[0] != "20261009-154200.salbum" || got[Keep-1] != "20261009-153300.salbum" {
		t.Fatalf("копии: %v", got)
	}
	c.t = c.t.Add(-time.Minute) // та же секунда, что у последней
	rewrite(t, p, `{"n":13}`)
	if err := s.Force(p, key); err != nil {
		t.Fatal(err)
	}
	if got := names(t, s, key); got[0] != "20261009-154200-2.salbum" || len(got) != Keep {
		t.Fatalf("занятое имя: %v", got)
	}
}

func TestBackupKeyAndNames(t *testing.T) {
	dir := testkit.TempDir(t)
	a := Key(filepath.Join(dir, "Ивановы.salbum"))
	if !strings.HasPrefix(a, "Ивановы-") || len(a) != len("Ивановы-")+8 {
		t.Fatalf("ключ %q", a)
	}
	// Тот же путь в другом регистре — тот же ключ; другая папка — другой.
	if b := Key(filepath.Join(strings.ToUpper(dir), "ИВАНОВЫ.salbum")); !strings.EqualFold(a, b) || a[len(a)-8:] != b[len(b)-8:] {
		t.Errorf("регистр: %q и %q", a, b)
	}
	if b := Key(filepath.Join(dir, "другая", "Ивановы.salbum")); b == a {
		t.Error("разные папки — один ключ")
	}
	for in, want := range map[string]string{
		` .Свадьба<>:"|?*` + "\x01" + `. `: "Свадьба",
		"...":                           "альбом",
		strings.Repeat("я", 50):         strings.Repeat("я", maxName),
		strings.Repeat("я", 39) + " .x": strings.Repeat("я", 39),
	} {
		if got := cleanName(in); got != want || utf8.RuneCountInString(got) > maxName {
			t.Errorf("cleanName(%q) = %q, ждали %q", in, got, want)
		}
	}
	for _, k := range []string{"", "..", `a\b`, "a/b", "C:x", DraftKey + "/.."} {
		if validKey(k) {
			t.Errorf("ключ %q принят", k)
		}
	}
	if !validKey(DraftKey) || !validKey(a) {
		t.Error("свой ключ отвергнут")
	}
	s, _ := newStore(t)
	for _, name := range []string{`..\x`, `C:\x`, "x.salbum", "20261009-153000.salbum.bak", "20261009-153000-1000.salbum", `..\..\20261009-153000.salbum`} {
		if _, err := s.File(a, name); err == nil {
			t.Errorf("имя %q принято", name)
		}
	}
	if _, err := s.File(`..`, "20261009-153000.salbum"); err == nil {
		t.Error("ключ .. принят")
	}
}

// Prune трогает только свои файлы копий в обычных подпапках: чужие файлы и папки остаются.
// Старше 180 дней — удаляются; сверх 30 альбомов — самые давние.
func TestBackupPruneOnlyOwnDirs(t *testing.T) {
	s, c := newStore(t)
	p := albumFile(t, `{}`)
	write := func(rel string) string {
		f := filepath.Join(s.Root, rel)
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return f
	}
	foreign := []string{write("readme.txt"), write(`чужая\20200101-000000.txt`), write(`старый-00000000\заметка.txt`)}
	old := write(`старый-00000000\20200101-000000.salbum`)
	for i := range MaxDirs + 2 {
		if err := s.Force(p, fmt.Sprintf("a%02d-00000000", i)); err != nil {
			t.Fatal(err)
		}
		c.t = c.t.Add(time.Minute)
	}
	if err := s.Prune(); err != nil {
		t.Fatal(err)
	}
	for _, f := range foreign {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("чужое удалено: %s", f)
		}
	}
	if _, err := os.Stat(old); err == nil {
		t.Error("копия старше 180 дней осталась")
	}
	for i, want := range map[int]bool{0: false, 1: false, 2: true, MaxDirs + 1: true} {
		dir := filepath.Join(s.Root, fmt.Sprintf("a%02d-00000000", i))
		if _, err := os.Stat(dir); (err == nil) != want {
			t.Errorf("альбом %d: есть=%v, ждали %v", i, err == nil, want)
		}
	}
}

// Ссылка в папке копий (на альбом или вместо папки альбома) не читается и не удаляется.
func TestBackupNotFollowLinks(t *testing.T) {
	s, _ := newStore(t)
	p := albumFile(t, `{}`)
	if err := s.Force(p, "свой-00000000"); err != nil {
		t.Fatal(err)
	}
	outside := testkit.TempDir(t)
	victim := filepath.Join(outside, "20200101-000000.salbum")
	if err := os.WriteFile(victim, []byte(`{"spreads":[{}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(s.Root, "ссылка-00000000")
	if err := os.Symlink(outside, link); err != nil {
		// Без прав на символьные ссылки — junction: его создаёт любой пользователь.
		out, jerr := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput() //nolint:gosec // пути теста
		if jerr != nil {
			t.Skipf("нет ни ссылки (%v), ни junction (%v: %s)", err, jerr, out)
		}
	}
	if list, err := s.List("ссылка-00000000"); err != nil || len(list) != 0 {
		t.Errorf("список через ссылку: %v, %v", list, err)
	}
	if _, err := s.File("ссылка-00000000", "20200101-000000.salbum"); err == nil {
		t.Error("файл через ссылку отдан")
	}
	if err := s.Force(p, "ссылка-00000000"); err == nil {
		t.Error("копия записана через ссылку")
	}
	if err := s.Prune(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(victim); err != nil {
		t.Fatalf("файл за ссылкой удалён: %v", err)
	}
	if ents, _ := os.ReadDir(outside); len(ents) != 1 {
		t.Errorf("за ссылкой записано: %d файлов", len(ents))
	}
}

func TestBackupInside(t *testing.T) {
	s, _ := newStore(t)
	for p, want := range map[string]bool{
		s.Root: true,
		filepath.Join(strings.ToUpper(s.Root), "x", "y.salbum"): true,
		s.Root + "2": false,
		filepath.Join(filepath.Dir(s.Root), "y.salbum"): false,
	} {
		if s.Inside(p) != want {
			t.Errorf("Inside(%q) = %v", p, !want)
		}
	}
	var none *Store
	if none.Inside(s.Root) || none.Force(s.Root, "x") != nil || none.Prune() != nil {
		t.Error("nil-хранилище")
	}
}
