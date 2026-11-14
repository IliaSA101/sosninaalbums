package imgfile

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// fakeInfo — сведения о файле с заданным режимом (Sys пустой).
type fakeInfo struct{ mode fs.FileMode }

func (fakeInfo) Name() string        { return "x" }
func (fakeInfo) Size() int64         { return 1 }
func (f fakeInfo) Mode() fs.FileMode { return f.mode }
func (fakeInfo) ModTime() time.Time  { return time.Time{} }
func (f fakeInfo) IsDir() bool       { return f.mode.IsDir() }
func (fakeInfo) Sys() any            { return nil }

// TestRegularOrCloud: обычный файл — да без обращения к диску; ModeIrregular — только если по пути облачная
// заглушка (шов cloudStubAt); папка, ссылка, канал, устройство и nil — нет, облачную проверку не зовут.
func TestRegularOrCloud(t *testing.T) {
	calls := 0
	old := cloudStubAt
	t.Cleanup(func() { cloudStubAt = old })
	for name, c := range map[string]struct {
		st    fs.FileInfo
		stub  bool
		want  bool
		calls int
	}{
		"nil":                  {nil, true, false, 0},
		"обычный":              {fakeInfo{0o644}, true, true, 0},
		"пустой режим":         {fakeInfo{0}, false, true, 0},
		"папка":                {fakeInfo{fs.ModeDir}, true, false, 0},
		"ссылка":               {fakeInfo{fs.ModeSymlink}, true, false, 0},
		"канал":                {fakeInfo{fs.ModeNamedPipe}, true, false, 0},
		"устройство":           {fakeInfo{fs.ModeDevice}, true, false, 0},
		"irregular, заглушка":  {fakeInfo{fs.ModeIrregular}, true, true, 1},
		"irregular, не облако": {fakeInfo{fs.ModeIrregular}, false, false, 1},
	} {
		calls = 0
		cloudStubAt = func(string) bool { calls++; return c.stub }
		if got := RegularOrCloud("p", c.st); got != c.want || calls != c.calls {
			t.Errorf("%s: RegularOrCloud = %v (проверок облака %d), ожидалось %v (%d)", name, got, calls, c.want, c.calls)
		}
	}
}

// TestRegularOrCloudDisk: на настоящем диске обычный файл — да, папка и пропавший путь — нет; вне Windows
// ModeIrregular (канал не он) облачной не бывает.
func TestRegularOrCloudDisk(t *testing.T) {
	dir := testkit.TempDir(t)
	p := filepath.Join(dir, "a.jpg")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]bool{p: true, dir: false} {
		st, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := RegularOrCloud(path, st); got != want {
			t.Errorf("%s: %v, ожидалось %v", path, got, want)
		}
	}
	if cloudStubPath(filepath.Join(dir, "нет.jpg")) {
		t.Error("пропавший путь — не заглушка")
	}
	if cloudStubPath(p) {
		t.Error("обычный файл — не заглушка")
	}
}

// TestDehydrated: атрибуты «данные в облаке» (OFFLINE, RECALL_ON_OPEN, RECALL_ON_DATA_ACCESS) — да; обычные,
// скрытый, архивный, 0 — нет; nil и сведения без атрибутов — нет.
func TestDehydrated(t *testing.T) {
	for attrs, want := range map[uint32]bool{
		0: false, 0x20: false, 0x2: false, 0x400: false, // архивный, скрытый, точка повторной обработки
		0x1000: true, 0x40000: true, 0x400000: true, 0x400020: true, 0x440000: true,
	} {
		if got := attrsDehydrated(attrs); got != want {
			t.Errorf("attrsDehydrated(%#x) = %v, ожидалось %v", attrs, got, want)
		}
	}
	if Dehydrated(nil) || Dehydrated(fakeInfo{0o644}) {
		t.Error("nil и сведения без атрибутов не облачные")
	}
}
