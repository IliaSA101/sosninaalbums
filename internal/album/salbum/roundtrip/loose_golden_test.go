package roundtrip_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
)

// Отдельные фото в файле (097, v16): голден вида записи loose.

var writeGolden16Loose = flag.Bool("golden16loose", false, "перезаписать testdata/v16-loose.salbum.json из looseGolden")

const golden16Loose = "v16-loose.salbum.json"

// looseGolden — альбом под base с папкой и двумя добавлениями: файл внутри альбома (path + abs),
// файл вне его выше двух «..» (только абсолютный) и пропавший с запасным путём (Alt пишется в abs).
func looseGolden(base string) *model.Project {
	p := model.New()
	p.Folders = []string{filepath.Join(base, "Март")}
	p.Loose = []model.LooseAdd{
		{ID: "3fa9c01b7d22", At: 1792501500, Files: []model.LooseFile{
			{Path: filepath.Join(base, "Телефон", "20261019_181522.jpg")},
			{Path: `D:\Загрузки\bouquet.jpg`},
		}},
		{ID: "0a0b0c0d0e0f", At: 1792588000, Files: []model.LooseFile{
			{Path: filepath.Join(filepath.Dir(base), "Телефон", "2.png"), Alt: `C:\Ивановы\Телефон\2.png`},
		}},
	}
	return p
}

// TestLooseGoldenV16 — голден v16 с отдельными фото: Decode → Encode = исходник с текущим номером
// версии байт в байт (он же сид FuzzLoad), добавления по порядку, запасной путь на месте. Пути — от C:\album: только Windows.
func TestLooseGoldenV16(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("голден с путями C:\\album")
	}
	path := filepath.Join("..", "testdata", golden16Loose)
	if *writeGolden16Loose {
		data, err := salbum.Encode(looseGolden(goldenBase), goldenBase)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.TrimSuffix(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), []byte("\n"))
	p, err := salbum.Decode(data, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || edtest.From(p) != 16 {
		t.Fatalf("версия %d, from %d", p.Version, edtest.From(p))
	}
	if want := looseGolden(goldenBase).Loose; !reflect.DeepEqual(p.Loose, want) {
		t.Fatalf("отдельные:\n%+v\nждали\n%+v", p.Loose, want)
	}
	out, err := salbum.Encode(p, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	wantFile := bytes.Replace(data, []byte(`"version": 16,`), fmt.Appendf(nil, `"version": %d,`, model.FormatVersion), 1)
	if bytes.Equal(wantFile, data) && model.FormatVersion != 16 {
		t.Fatal("в голдене нет номера версии 16")
	}
	if !bytes.Equal(out, wantFile) {
		t.Fatalf("Decode → Encode отличается от голдена (кроме номера версии):\n%s", out)
	}
	if err := edtest.SourcesInvariant(p); err != nil {
		t.Fatal(err)
	}
}
