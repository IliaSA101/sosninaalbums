//go:build qa

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Зонд win-qa: чтение скрипта и запись событий. Тесты тоже под тегом qa: go test -tags qa ./internal/app -run QaProbe.

func TestQaProbeScript(t *testing.T) {
	dir := testkit.TempDir(t)
	write := func(name string, n int) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(strings.Repeat("a", n)), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if s, err := qaProbeScript(write("ok.js", 10)); err != nil || s != strings.Repeat("a", 10) {
		t.Fatalf("годный скрипт: %q %v", s, err)
	}
	if _, err := qaProbeScript(write("edge.js", qaProbeMaxJS)); err != nil {
		t.Fatalf("ровно предел: %v", err)
	}
	if s, err := qaProbeScript(write("empty.js", 0)); err != nil || s != "" {
		t.Fatalf("пустой файл: %q %v", s, err)
	}
	bad := map[string]string{
		"больше предела": write("big.js", qaProbeMaxJS+1),
		"пустой путь":    "",
		"относительный":  "x.js",
		"нет файла":      filepath.Join(dir, "none.js"),
		"папка":          dir,
	}
	for name, p := range bad {
		if _, err := qaProbeScript(p); err == nil {
			t.Errorf("%s: ожидалась ошибка", name)
		}
	}
}

func TestQaProbeSink(t *testing.T) {
	p := filepath.Join(testkit.TempDir(t), "out.jsonl")
	s := &qaProbeSink{path: p}
	s.add(`{"step":"a"}`)
	s.add()                                      // нет данных
	s.add(42)                                    // не строка
	s.add("с\nпереводом")                        // многострочная запись
	s.add(strings.Repeat("x", qaProbeMaxLine+1)) // длиннее предела
	s.add(`{"step":"b"}`)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	bad := `{"step":"qa-bad-record"}`
	want := `{"step":"a"}` + "\n" + bad + "\n" + bad + "\n" + bad + "\n" + bad + "\n" + `{"step":"b"}` + "\n"
	if string(b) != want {
		t.Fatalf("файл записей:\n%s\nожидалось:\n%s", b, want)
	}
}

func TestQaProbeSinkLimit(t *testing.T) {
	p := filepath.Join(testkit.TempDir(t), "out.jsonl")
	s := &qaProbeSink{path: p}
	line := strings.Repeat("y", qaProbeMaxLine-1)
	for range qaProbeMaxOut/qaProbeMaxLine + 5 {
		s.add(line)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() > qaProbeMaxOut {
		t.Fatalf("файл записей %d Б больше предела %d", st.Size(), qaProbeMaxOut)
	}
}
