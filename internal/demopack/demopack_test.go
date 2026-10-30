package demopack

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
)

// Настоящий zip читается один раз на весь прогон: 17 МБ.
var demoZip = sync.OnceValues(func() ([]byte, error) { return os.ReadFile(File) })

func zipBytes(t *testing.T) []byte {
	t.Helper()
	b, err := demoZip()
	if err != nil {
		t.Fatalf("zip примера не читается: %v", err)
	}
	return b
}

func openDemo(t *testing.T) *pack.Archive {
	t.Helper()
	z := zipBytes(t)
	a, err := pack.OpenReader(bytes.NewReader(z), int64(len(z)), "0.29.0")
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	return a
}

// row — строка снимка ключей: `[донор] вид ключ [путь]`.
type row struct{ donor, kind, key, path string }

func readKeys(t *testing.T, name string, withDonor bool) []row {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var out []row
	for line := range strings.SplitSeq(strings.TrimSuffix(string(b), "\n"), "\n") {
		f := strings.Split(line, " ")
		var r row
		if withDonor {
			r.donor, f = f[0], f[1:]
		}
		if len(f) < 2 || len(f) > 3 {
			t.Fatalf("%s: строка %q", name, line)
		}
		r.kind, r.key = f[0], f[1]
		if len(f) == 3 {
			r.path = f[2]
		}
		out = append(out, r)
	}
	return out
}

// TestDemoZipValid — zip проходит ту же проверку, что набор из файла, и каждый предмет сходится
// с манифестом по размеру и sha256.
func TestDemoZipValid(t *testing.T) {
	a := openDemo(t)
	if a.Manifest.ID != ID || a.Manifest.Version != Version {
		t.Fatalf("манифест %s v%d", a.Manifest.ID, a.Manifest.Version)
	}
	if len(a.Manifest.Items) != 18 {
		t.Fatalf("предметов %d, ждали 18", len(a.Manifest.Items))
	}
	for i, it := range a.Manifest.Items {
		if err := a.Extract(i, io.Discard); err != nil {
			t.Errorf("%s: %v", it.File, err)
		}
	}
}

// TestDemoBudget — файл в бюджете pre-commit и выпуска.
func TestDemoBudget(t *testing.T) {
	if n := len(zipBytes(t)); n > MaxBudget {
		t.Fatalf("zip %d байт больше бюджета %d МиБ", n, MaxBudget>>20)
	}
}

// TestDemoConstMatchesFile — константы совпадают с настоящим zip: по ним программа принимает
// файл рядом с exe и ассет релиза, строка «Скачать» берёт описание без каталога.
func TestDemoConstMatchesFile(t *testing.T) {
	z := zipBytes(t)
	if len(z) != Size {
		t.Errorf("Size = %d, файл %d", Size, len(z))
	}
	sum := sha256.Sum256(z)
	if got := hex.EncodeToString(sum[:]); got != SHA256 {
		t.Errorf("SHA256 = %s, файл %s", SHA256, got)
	}
	if File != "wedding-navy-watercolor-demo-1.zip" || Asset != "wedding-navy-watercolor-demo-v1/"+File {
		t.Errorf("File %q, Asset %q", File, Asset)
	}
	m := openDemo(t).Manifest
	if m.Title != Title || m.About != About || m.License != License {
		t.Errorf("описание не сходится с манифестом: %q / %q / %q", m.Title, m.About, m.License)
	}
	n := map[string]int{}
	for _, it := range m.Items {
		n[it.Kind]++
	}
	if n["bg"] != Bg || n["sticker"] != Sticker || n["spread"] != Spread || n["frame"] != Frame || n["album"] != 1 {
		t.Errorf("счётчики %v, константы %d/%d/%d/%d + макет", n, Bg, Sticker, Spread, Frame)
	}
	if len(Related) != 3 {
		t.Errorf("Related %v", Related)
	}
}

// itemKeys — ключи предметов так же, как library-pack-demo.mjs: `my:` + 16 hex sha256 байтов файла.
func itemKeys(t *testing.T, a *pack.Archive) []row {
	t.Helper()
	var out []row
	for i, it := range a.Manifest.Items {
		var buf bytes.Buffer
		if err := a.Extract(i, &buf); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(buf.Bytes())
		r := row{kind: it.Kind, key: "my:" + hex.EncodeToString(sum[:])[:16]}
		if it.Kind == "spread" || it.Kind == "album" {
			r.path = it.File
		}
		out = append(out, r)
	}
	return out
}

// TestDemoKeysPublished — снимок demo-v1.keys совпадает с предметами zip по порядку; каждая
// строка есть у своего донора (иначе уход примера и псевдоним макета не сойдутся); путь макета
// тот же, что в -layouts.
func TestDemoKeysPublished(t *testing.T) {
	want := itemKeys(t, openDemo(t))
	got := readKeys(t, "demo-v1.keys", true)
	if len(got) != len(want) {
		t.Fatalf("строк %d, предметов %d", len(got), len(want))
	}
	donors := map[string][]row{}
	for i, g := range got {
		if g.kind != want[i].kind || g.key != want[i].key || g.path != want[i].path {
			t.Errorf("строка %d: %v, предмет %v", i+1, g, want[i])
		}
		if _, ok := donors[g.donor]; !ok {
			donors[g.donor] = readKeys(t, g.donor+"-v1.keys", false)
		}
		found := false
		for _, s := range donors[g.donor] {
			found = found || s.kind == g.kind && s.key == g.key && s.path == g.path
		}
		if !found {
			t.Errorf("%s: нет предмета %s %s у донора", g.donor, g.kind, g.key)
		}
	}
	var names []string
	for d := range donors {
		names = append(names, d)
	}
	if len(names) != len(Related) {
		t.Fatalf("доноры %v, Related %v", names, Related)
	}
	for _, r := range Related {
		if _, ok := donors[r]; !ok {
			t.Errorf("донор %s из Related не встречается в demo-v1.keys", r)
		}
	}
	if last := got[len(got)-1]; last.kind != "album" || last.path != "looks/wedding-navy-watercolor.json" {
		t.Errorf("макет альбома: %v", last)
	}
}

// TestDemoKeysFrozen — снимки v1 заморожены: v2 примера только добавляет (0.30-142-11), правка
// этих файлов теряла бы закрытые альбомы. Новая версия — новый файл demo-v2.keys.
func TestDemoKeysFrozen(t *testing.T) {
	frozen := map[string]string{
		"demo-v1.keys":                            "46d9c119bc9143f877c5b98289612723f3b065d48347f894d73b24c9755e4b16",
		"wedding-navy-watercolor-v1.keys":         "c0ef10c14b3f91a8d76b6a71e52f8596d7507e542468eaa4ab5b5520bf39fce8",
		"wedding-navy-watercolor-frames-v1.keys":  "62b9b8b903e7ff378473ce4f9f5e0822a57523a2e98983d5475be880927e59c7",
		"wedding-navy-watercolor-layouts-v1.keys": "236b36011e2bf1ba82f4cf0942606cd79172af39deac2a1c0e8ac33d950a936a",
	}
	for name, want := range frozen {
		b, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("%s изменён (sha256 %s): снимок v1 заморожен", name, got)
		}
	}
}
