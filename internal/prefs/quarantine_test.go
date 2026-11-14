package prefs

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Поломки подряд: прежний .bad не затирается, второй файл идёт под именем со временем, третий в ту
// же секунду — с номером. Все три на месте, содержимое каждого своё; после каждой — повторный
// импорт из shares.json, и он в журнале.
func TestProjectsQuarantineKeepsEarlier(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	dir, ra, _ := v1With(t)
	path := ProjectsPath(dir)
	at := time.Date(2026, 11, 9, 13, 4, 5, 0, time.UTC)
	breaks := []string{`{"version":1,"projects":{`, `[1`, `{"projects":{}}`}
	want := []string{path + ".bad", path + ".bad.20261109-130405", path + ".bad.20261109-130405-2"}
	for i, content := range breaks {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		p := NewProjects(dir, testRules)
		p.now = func() time.Time { return at }
		if got, ok := p.Get(puid(1)); !ok || got.ID != ra.ID {
			t.Fatalf("поломка %d: импорт не сработал: %v %+v", i, ok, got)
		}
		if err := os.Remove(path); err != nil { // импорт записал новый файл; следующая поломка кладёт свой
			t.Fatal(err)
		}
	}
	for i, name := range want {
		if got, err := os.ReadFile(name); err != nil || string(got) != breaks[i] {
			t.Errorf("%s: %q, %v (ждали %q)", name, got, err, breaks[i])
		}
	}
	if n := strings.Count(buf.String(), "после карантина повторный импорт, записей: 2"); n != 3 {
		t.Errorf("строк о повторном импорте %d, ждали 3:\n%s", n, buf.String())
	}
}

// У shares.json источника импорта нет: строки о нём в журнале нет, а второй битый файл не
// затирает первый.
func TestSharesQuarantineKeepsEarlier(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	dir := testkit.TempDir(t)
	path := SharesPath(dir)
	for _, content := range []string{`{"version":1,`, `[2`} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		s := NewShares(dir, testRules)
		s.now = func() time.Time { return time.Date(2026, 11, 9, 1, 2, 3, 0, time.UTC) }
		if _, ok := s.Get(albumPath(dir, "a")); ok {
			t.Fatal("запись из битого файла")
		}
	}
	for name, content := range map[string]string{path + ".bad": `{"version":1,`, path + ".bad.20261109-010203": `[2`} {
		if got, err := os.ReadFile(name); err != nil || string(got) != content {
			t.Errorf("%s: %q, %v", name, got, err)
		}
	}
	if strings.Contains(buf.String(), "повторный импорт") {
		t.Errorf("у shares.json нет импорта:\n%s", buf.String())
	}
}
