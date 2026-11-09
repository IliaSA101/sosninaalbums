package prefs

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// v1With — папка данных с годным shares.json 0.37: записи под puid(1) и puid(2).
func v1With(t *testing.T) (dir string, ra, rb ShareRecord) {
	t.Helper()
	dir = testkit.TempDir(t)
	ra, rb = rec(1), rec(2)
	ra.UID, rb.UID = puid(1), puid(2)
	writeV1(t, dir, map[string]ShareRecord{albumPath(dir, "a"): ra, albumPath(dir, "b"): rb})
	return dir, ra, rb
}

// shares.json не открылся (замок антивируса): запись отказывает, projects.json не создаётся и
// импорт не теряется — после снятия запрета записи 0.37 на месте.
func TestProjectsImportUnreadShares(t *testing.T) {
	dir, ra, rb := v1With(t)
	// Шов чтения, а не chmod 0: на Windows chmod чтение не запрещает, root читает и без прав.
	locked := errors.New("файл занят другим процессом")
	restore := setReadFault(func(path string) error {
		if path == SharesPath(dir) {
			return locked
		}
		return nil
	})
	t.Cleanup(restore)
	p := NewProjects(dir, testRules)
	if _, ok := p.Get(puid(1)); ok {
		t.Error("запись из нечитаемого файла")
	}
	if err := p.Put(puid(7), prec(7, "")); !errors.Is(err, ErrProjectsUnread) {
		t.Fatalf("Put при нечитаемом shares.json: %v", err)
	}
	if _, _, err := p.PutIfAbsent(puid(8), prec(8, "")); !errors.Is(err, ErrProjectsUnread) {
		t.Errorf("PutIfAbsent: %v", err)
	}
	if _, err := os.Lstat(ProjectsPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("projects.json создан без импорта: %v", err)
	}
	restore() // замок снят
	if err := p.Put(puid(7), prec(7, "")); err != nil {
		t.Fatal(err)
	}
	re := NewProjects(dir, testRules)
	for uid, want := range map[string]ShareRecord{puid(1): ra, puid(2): rb} {
		if got, ok := re.Get(uid); !ok || got.ID != want.ID {
			t.Errorf("запись 0.37 %s потеряна: %v %+v", uid, ok, got)
		}
	}
	if _, ok := re.Get(puid(7)); !ok {
		t.Error("новая запись потеряна")
	}
}

// Битый projects.json уходит в .bad и импорт из shares.json срабатывает (167-27) — и на первом
// чтении, и на первой записи.
func TestProjectsImportAfterBad(t *testing.T) {
	const broken = `{"version":1,"projects":{`
	for _, firstPut := range []bool{false, true} {
		name := map[bool]string{false: "первое чтение", true: "первая запись"}[firstPut]
		t.Run(name, func(t *testing.T) {
			dir, ra, rb := v1With(t)
			if err := os.WriteFile(ProjectsPath(dir), []byte(broken), 0o600); err != nil {
				t.Fatal(err)
			}
			p := NewProjects(dir, testRules)
			if firstPut {
				if err := p.Put(puid(7), prec(7, "")); err != nil {
					t.Fatal(err)
				}
			} else if got, ok := p.Get(puid(1)); !ok || got.ID != ra.ID {
				t.Fatalf("первое чтение без импорта: %v %+v", ok, got)
			}
			if got, err := os.ReadFile(ProjectsPath(dir) + ".bad"); err != nil || string(got) != broken {
				t.Errorf(".bad: %v", err)
			}
			re := NewProjects(dir, testRules)
			for uid, want := range map[string]ShareRecord{puid(1): ra, puid(2): rb} {
				if got, ok := re.Get(uid); !ok || got.ID != want.ID {
					t.Errorf("запись 0.37 %s потеряна: %v %+v", uid, ok, got)
				}
			}
			if _, ok := re.Get(puid(7)); ok != firstPut {
				t.Errorf("запись 7: %v", ok)
			}
		})
	}
}

// Давно истёкшая запись не считается занятой: PutIfAbsent кладёт новую, а не теряет обе.
func TestProjectsPutIfAbsentOverExpired(t *testing.T) {
	dir := testkit.TempDir(t)
	p := NewProjects(dir, testRules)
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	p.now = func() time.Time { return now }
	old, fresh := prec(1, ""), prec(2, "")
	old.ExpiresAt, fresh.ExpiresAt = now.Add(-31*24*time.Hour), now.Add(24*time.Hour)
	writeProjectsRaw(t, dir, 1, map[string]json.RawMessage{puid(1): mustJSON(old), puid(2): mustJSON(fresh)})

	next := prec(5, "")
	next.ExpiresAt = now.Add(48 * time.Hour)
	cur, added, err := p.PutIfAbsent(puid(1), next)
	if err != nil || !added || cur.ID != next.ID {
		t.Fatalf("над устаревшей: %+v %v %v", cur, added, err)
	}
	re := NewProjects(dir, testRules)
	if got, ok := re.Get(puid(1)); !ok || got.ID != next.ID {
		t.Errorf("под uid не новая запись: %v %+v", ok, got)
	}
	// Свежая занята: прежняя возвращается, не added.
	cur, added, err = p.PutIfAbsent(puid(2), next)
	if err != nil || added || cur.ID != fresh.ID {
		t.Errorf("над свежей: %+v %v %v", cur, added, err)
	}
}

// Битый shares.json при импорте: в журнале одна строка, а не по строке на каждое чтение; сам
// файл не тронут.
func TestProjectsImportBrokenSharesLogOnce(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	dir := testkit.TempDir(t)
	const broken = `{"version":1,"albums":{`
	if err := os.WriteFile(SharesPath(dir), []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	p := NewProjects(dir, testRules)
	for range 4 {
		p.Get(puid(1))
	}
	if n := strings.Count(buf.String(), "shares.json не разобран"); n != 1 {
		t.Errorf("записей журнала %d, ждали 1:\n%s", n, buf.String())
	}
	if got := readFile(t, SharesPath(dir)); string(got) != broken {
		t.Error("shares.json тронут")
	}
	if _, err := os.Lstat(ProjectsPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("projects.json создан из пустого импорта: %v", err)
	}
}
