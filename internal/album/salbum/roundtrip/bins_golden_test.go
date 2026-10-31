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
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Подборки фото в файле (083, v15): голден вида записи bins (корень и членство в marks).

var writeGolden15Bins = flag.Bool("golden15bins", false, "перезаписать testdata/v15-bins.salbum.json из binsGolden")

const (
	golden15Bins = "v15-bins.salbum.json"
	binGoldMust  = "b3f9a01c2"
	binGoldPar   = "b07d5e6aa"
)

// binsGolden — альбом под base: фото на развороте, две подборки (порядок создания, не по алфавиту),
// отметки: оценка и две подборки (ID пишутся по возрастанию), только подборка, только оценка.
func binsGolden(base string) *model.Project {
	p := model.New()
	a, b, c := filepath.Join(base, "Фото", "a.jpg"), filepath.Join(base, "Фото", "b.jpg"), filepath.Join(base, "Фото", "c.jpg")
	for _, path := range []string{a, b, c} {
		p.Photos[library.IDFor(path)] = model.PhotoRef{Path: path, W: 3000, H: 2000, Size: 1000}
	}
	p.Spreads = []*model.Spread{{ID: "5e1f0a2b3c41", Photos: []string{library.IDFor(a), library.IDFor(b), library.IDFor(c)}}}
	r := 4
	p.SetLoaded(model.Side{
		Bins: []model.Bin{{ID: binGoldMust, Name: "Обязательно"}, {ID: binGoldPar, Name: "Для родителей"}},
		Marks: map[string]model.Mark{
			library.IDFor(a): {Path: a, Rating: &r, Bins: []string{binGoldPar, binGoldMust}},
			library.IDFor(b): {Path: b, Bins: []string{binGoldMust}},
			library.IDFor(c): {Path: c, Rating: &r},
		},
	})
	return p
}

// TestBinsGoldenV15 — голден v15 с подборками (083): Decode → Encode = исходник байт в байт (он же
// сид FuzzLoad), подборки и членство на месте. Пути — от C:\album, поэтому только Windows.
func TestBinsGoldenV15(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("голден с путями C:\\album")
	}
	path := filepath.Join("..", "testdata", golden15Bins)
	if *writeGolden15Bins {
		data, err := salbum.Encode(binsGolden(goldenBase), goldenBase)
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
	if p.Version != model.FormatVersion || edtest.From(p) != 15 {
		t.Fatalf("версия %d, from %d", p.Version, edtest.From(p))
	}
	side := p.Loaded()
	if want := binsGolden(goldenBase).Loaded().Bins; !reflect.DeepEqual(side.Bins, want) || len(side.Marks) != 3 {
		t.Fatalf("подборки %v, отметок %d", side.Bins, len(side.Marks))
	}
	a := side.Marks[library.IDFor(filepath.Join(goldenBase, "Фото", "a.jpg"))]
	if a.Rating == nil || *a.Rating != 4 || !reflect.DeepEqual(a.Bins, []string{binGoldPar, binGoldMust}) {
		t.Fatalf("отметка a: %+v", a)
	}
	out, err := salbum.Encode(p, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	// Голден остаётся файлом v15 (база совместимости): запись отличается только номером версии.
	wantFile := bytes.Replace(data, []byte(`"version": 15,`), fmt.Appendf(nil, `"version": %d,`, model.FormatVersion), 1)
	if bytes.Equal(wantFile, data) && model.FormatVersion != 15 {
		t.Fatal("в голдене нет номера версии 15")
	}
	if !bytes.Equal(out, wantFile) {
		t.Fatalf("Decode → Encode v15 отличается от голдена (кроме номера версии):\n%s", out)
	}
}
