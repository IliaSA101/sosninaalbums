package backup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// tenCopies — 10 копий с разным содержимым, по минуте; возвращает файл и ключ.
func tenCopies(t *testing.T, s *Store, c *clock) (string, string) {
	t.Helper()
	p := albumFile(t, `{}`)
	key := Key(p)
	for i := range Keep {
		rewrite(t, p, fmt.Sprintf(`{"n":%d}`, i))
		if err := s.Force(p, key); err != nil {
			t.Fatal(err)
		}
		c.t = c.t.Add(time.Minute)
	}
	return p, key
}

// Файл совпал с самой новой копией — дубль не пишется (Force и Before).
func TestBackupSkipsDuplicate(t *testing.T) {
	s, c := newStore(t)
	p := albumFile(t, `{"a":1}`)
	key := Key(p)
	for range 3 {
		if err := s.Force(p, key); err != nil {
			t.Fatal(err)
		}
		c.t = c.t.Add(time.Hour)
		if err := s.Before(p, key); err != nil {
			t.Fatal(err)
		}
	}
	if got := names(t, s, key); len(got) != 1 {
		t.Fatalf("дубли: %v", got)
	}
}

// ForceKeep: при 10 копиях самая давняя (восстанавливаемая) остаётся, уходит следующая за ней.
func TestBackupForceKeepProtects(t *testing.T) {
	s, c := newStore(t)
	p, key := tenCopies(t, s, c)
	before := names(t, s, key)
	oldest := before[Keep-1]
	rewrite(t, p, `{"n":"текущий"}`)
	if err := s.ForceKeep(p, key, oldest); err != nil {
		t.Fatal(err)
	}
	got := names(t, s, key)
	if len(got) != Keep || !slices.Contains(got, oldest) || slices.Contains(got, before[Keep-2]) {
		t.Fatalf("копии: %v", got)
	}
}

// Часы шли вперёд, потом их вернули: имена «из будущего» в списке — не позже now, только что
// записанная копия не удаляется, хоть по имени она самая давняя; Every не держит копии.
func TestBackupClockBack(t *testing.T) {
	s, c := newStore(t)
	actual := c.t
	c.t = actual.Add(24 * time.Hour)
	p, key := tenCopies(t, s, c)
	c.t = actual
	list, err := s.List(key)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range list {
		if e.Time.After(actual) {
			t.Fatalf("время из будущего: %v", e.Time)
		}
	}
	rewrite(t, p, `{"n":"сейчас"}`)
	if err := s.Before(p, key); err != nil { // прошлая копия «позже» now — всё равно пора
		t.Fatal(err)
	}
	fresh := actual.Format(timeLayout) + ext
	if got := names(t, s, key); len(got) != Keep || !slices.Contains(got, fresh) {
		t.Fatalf("свежая копия удалена: %v", got)
	}
}

// Prune: now раньше самой новой копии — по возрасту ничего не удаляется; часы догнали — удаляется.
func TestBackupPruneClockBehind(t *testing.T) {
	s, c := newStore(t)
	p := albumFile(t, `{}`)
	future := c.t.Add(time.Hour)
	now := c.t
	c.t = future
	if err := s.Force(p, "свежий-00000000"); err != nil {
		t.Fatal(err)
	}
	c.t = now
	old := filepath.Join(s.Root, "старый-00000000", "20200101-000000.salbum")
	if err := os.MkdirAll(filepath.Dir(old), 0o755); err != nil {
		t.Fatal(err)
	}
	rewrite(t, old, `{}`)
	if err := s.Prune(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("при отстающих часах удалено по возрасту: %v", err)
	}
	c.t = future.Add(time.Minute)
	if err := s.Prune(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); err == nil {
		t.Fatal("старая копия осталась")
	}
}

// trim: остаток записи старше часа удаляется, свежий — нет; папка альбома держит albumMax байт,
// самая новая копия остаётся, даже если одна больше предела.
func TestBackupTrimTmpAndBytes(t *testing.T) {
	s, c := newStore(t)
	p := albumFile(t, `{}`)
	key := Key(p)
	if err := s.Force(p, key); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.Root, key)
	tmp := func(name string, age time.Duration) string {
		f := filepath.Join(dir, name)
		rewrite(t, f, "x")
		if err := os.Chtimes(f, c.t.Add(-age), c.t.Add(-age)); err != nil {
			t.Fatal(err)
		}
		return f
	}
	stale, fresh := tmp("20261009-150000.salbum.123.tmp", 2*time.Hour), tmp("20261009-152000.salbum.456.tmp", 10*time.Minute)
	s.albumMax = 250
	body := strings.Repeat("я", 50) // 100 байт
	for i := range 4 {
		c.t = c.t.Add(time.Minute)
		rewrite(t, p, body[:len(body)-2*i])
		if err := s.Force(p, key); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("старый .tmp остался")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("свежий .tmp удалён: %v", err)
	}
	list, err := s.List(key)
	if err != nil {
		t.Fatal(err)
	}
	var total int64
	for _, e := range list {
		total += e.Size
	}
	if len(list) != 2 || total > s.albumMax {
		t.Fatalf("предел папки: %d копий, %d байт", len(list), total)
	}
	s.albumMax = 10
	c.t = c.t.Add(time.Minute)
	rewrite(t, p, body)
	if err := s.Force(p, key); err != nil {
		t.Fatal(err)
	}
	if got := names(t, s, key); len(got) != 1 || got[0] != c.t.Format(timeLayout)+ext {
		t.Fatalf("самая новая копия: %v", got)
	}
}

// PruneDue — раз в pruneEvery записей; дубли не считаются.
func TestBackupPruneDue(t *testing.T) {
	s, _ := newStore(t)
	p := albumFile(t, `{}`)
	key := Key(p)
	for i := range pruneEvery {
		if s.PruneDue() {
			t.Fatalf("рано: %d записей", i)
		}
		rewrite(t, p, fmt.Sprintf(`{"n":%d}`, i))
		if err := s.Force(p, key); err != nil {
			t.Fatal(err)
		}
		if err := s.Force(p, key); err != nil { // дубль
			t.Fatal(err)
		}
	}
	if !s.PruneDue() || s.PruneDue() {
		t.Fatal("PruneDue не сработал один раз")
	}
	var none *Store
	if none.PruneDue() {
		t.Error("nil-хранилище")
	}
}

// Права копий вне Windows: папки 0700, файлы 0600.
func TestBackupModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("права Unix")
	}
	s, _ := newStore(t)
	p := albumFile(t, `{}`)
	key := Key(p)
	if err := s.Force(p, key); err != nil {
		t.Fatal(err)
	}
	for f, want := range map[string]os.FileMode{
		s.Root:                     0o700,
		filepath.Join(s.Root, key): 0o700,
		filepath.Join(s.Root, key, names(t, s, key)[0]): 0o600,
	} {
		if st, err := os.Stat(f); err != nil || st.Mode().Perm() != want {
			t.Errorf("%s: %v, %v", f, st.Mode().Perm(), err)
		}
	}
}

// Inside видит папку копий и под другим именем: \\?\-путь, \\localhost\C$, junction.
func TestBackupInsideAliases(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("пути Windows")
	}
	s, _ := newStore(t)
	if err := os.MkdirAll(s.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		t.Fatal(err)
	}
	aliases := []string{`\\?\` + root}
	if vol := filepath.VolumeName(root); len(vol) == 2 {
		unc := `\\localhost\` + vol[:1] + `$` + root[2:]
		if _, err := os.Stat(unc); err == nil { // админ-шары может не быть
			aliases = append(aliases, unc)
		}
	}
	link := filepath.Join(testkit.TempDir(t), "ссылка")
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, root).CombinedOutput() //nolint:gosec // пути теста
	if err != nil {
		t.Logf("junction не создан (%v: %s)", err, out)
	} else {
		t.Cleanup(func() { _ = os.Remove(link) }) // сначала ссылку: иначе очистка ушла бы внутрь Root
		aliases = append(aliases, link)
	}
	for _, a := range aliases {
		for _, p := range []string{a, filepath.Join(a, "свой.salbum"), filepath.Join(a, "x", "y.salbum")} {
			if !s.Inside(p) {
				t.Errorf("Inside(%q) = false", p)
			}
		}
	}
	if s.Inside(filepath.Join(filepath.Dir(link), "другой.salbum")) || s.Inside(`\\?\`+filepath.Dir(root)) {
		t.Error("соседняя папка — внутри")
	}
}
