package decor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestPrintsStore (123 Б1): свои типографии — круг Save → закрыли → Open → список; замена под тем
// же ключом; несколько штук сразу; удаление стирает файл и запись; лишнее не принимается.
func TestPrintsStore(t *testing.T) {
	cfg := testkit.TempDir(t)
	s := Open(cfg)
	if len(s.Prints()) != 0 {
		t.Fatal("в пустой библиотеке есть типографии")
	}
	a, err := s.SavePrint("", "Первая", []byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.SavePrint("", "Вторая", []byte(`{"b":2}`))
	if err != nil || a == b {
		t.Fatalf("второй ключ: %v %q %q", err, a, b)
	}
	if !strings.HasPrefix(a, "my:") || filepath.Ext(s.files[KindPrint][a]) != ".saprint" || filepath.Base(filepath.Dir(s.files[KindPrint][a])) != "Типографии" {
		t.Errorf("файл: %q", s.files[KindPrint][a])
	}
	// Круг: новый Store читает реестр и папку.
	got := Open(cfg).Prints()
	if len(got) != 2 || got[0].Name != "Первая" || got[1].Name != "Вторая" || !bytes.Equal(got[1].Data, []byte(`{"b":2}`)) {
		t.Fatalf("после перезапуска: %+v", got)
	}
	// Замена — тот же ключ, новые байты, порядок прежний; чужой ключ — ErrNotFound.
	if k, err := s.SavePrint(a, "Первая-2", []byte(`{"a":3}`)); err != nil || k != a {
		t.Fatalf("замена: %v %q", err, k)
	}
	if got = Open(cfg).Prints(); got[0].Key != a || got[0].Name != "Первая-2" || string(got[0].Data) != `{"a":3}` {
		t.Errorf("после замены: %+v", got[0])
	}
	if _, err := s.SavePrint("my:0123456789abcdef", "x", []byte("{}")); err != ErrNotFound {
		t.Errorf("чужой ключ: %v", err)
	}
	// Края: пусто, огромное, без названия.
	for name, c := range map[string]struct {
		n    string
		data []byte
	}{"пусто": {"x", nil}, "огромный": {"x", bytes.Repeat([]byte("a"), MaxPrintFile+1)}, "без названия": {"  ", []byte("{}")}} {
		if _, err := s.SavePrint("", c.n, c.data); err == nil {
			t.Errorf("%s: принят", name)
		}
	}
	// Удаление: файл, запись, повтор.
	path := s.files[KindPrint][a]
	if err := s.DeletePrint(a); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("файл остался")
	}
	if got = Open(cfg).Prints(); len(got) != 1 || got[0].Key != b {
		t.Errorf("после удаления: %+v", got)
	}
	if err := s.DeletePrint(a); err != ErrNotFound {
		t.Errorf("повторное удаление: %v", err)
	}
}

// TestPrintsLimit — больше MaxPrints не помещается, замена существующей — помещается.
func TestPrintsLimit(t *testing.T) {
	s := Open(testkit.TempDir(t))
	var first string
	for i := range MaxPrints {
		k, err := s.SavePrint("", "Т"+string(rune('a'+i%26))+string(rune('a'+i/26)), []byte("{}"))
		if err != nil {
			t.Fatal(i, err)
		}
		if i == 0 {
			first = k
		}
	}
	if _, err := s.SavePrint("", "лишняя", []byte("{}")); err != ErrPrintLimit {
		t.Errorf("сверх предела: %v", err)
	}
	if _, err := s.SavePrint(first, "первая", []byte(`{"x":1}`)); err != nil {
		t.Errorf("замена у предела: %v", err)
	}
	if len(s.Prints()) != MaxPrints {
		t.Errorf("в списке %d", len(s.Prints()))
	}
}

// TestPrintsScanIgnoresStray — чужой файл в папке Типографии (не от ключа, не .saprint, обрывок
// записи) списком не становится, обрывок записи стирается при открытии.
func TestPrintsScanIgnoresStray(t *testing.T) {
	cfg := testkit.TempDir(t)
	dir := filepath.Join(cfg, RootName, "Типографии")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	left := filepath.Join(dir, "0123456789abcdef.saprint.123.tmp")
	for _, n := range []string{"readme.txt", "0123456789abcdef.json", "../x.saprint", "0123456789abcdef.saprint.bak"} {
		_ = os.WriteFile(filepath.Join(dir, filepath.Base(n)), []byte("{}"), 0o644)
	}
	_ = os.WriteFile(left, []byte("{}"), 0o644)
	if got := Open(cfg).Prints(); len(got) != 0 {
		t.Errorf("постороннее стало типографией: %+v", got)
	}
	if _, err := os.Stat(left); !os.IsNotExist(err) {
		t.Error("обрывок записи не убран")
	}
}
