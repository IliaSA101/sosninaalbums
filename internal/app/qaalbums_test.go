package app

import (
	"bytes"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// qaGen — собранный генератор набора (scripts/qa-albums.go).
type qaGen struct{ exe string }

// buildQaGen собирает генератор так же, как kit (go build файла); без go или на лёгком уровне — пропуск.
func buildQaGen(t *testing.T, tmp string) qaGen {
	t.Helper()
	if testkit.Lite() {
		t.Skip("сборка генератора — полный уровень")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("нет go в PATH")
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "qa-albums.go"))
	if err != nil {
		t.Fatal(err)
	}
	g := qaGen{exe: filepath.Join(tmp, "qa-albums")}
	if runtime.GOOS == "windows" {
		g.exe += ".exe" // без суффикса Windows не находит собранный файл при запуске
	}
	if b, err := exec.Command(goBin, "build", "-o", g.exe, script).CombinedOutput(); err != nil {
		t.Fatalf("сборка: %v\n%s", err, b)
	}
	return g
}

func (g qaGen) run(out string, args ...string) (string, error) {
	b, err := exec.Command(g.exe, append([]string{"-out", out}, args...)...).CombinedOutput()
	return string(b), err
}

// Генератор набора для проверки exe (спека 118): альбомы читает загрузчик программы, пути только
// относительные (папку можно унести), число разворотов и фото — по контракту.
func TestQaAlbumsGenerator(t *testing.T) {
	tmp := testkit.TempDir(t)
	g := buildQaGen(t, tmp)
	cases := []struct {
		name          string
		args          []string
		photos, alpha int
	}{
		{"по умолчанию", nil, 12, 6},
		{"предел: три разворота, по фото на каждый", []string{"-photos", "3", "-spreads", "3"}, 3, 3},
		{"много фото на развороте", []string{"-photos", "30", "-spreads", "4"}, 30, 4},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := filepath.Join(tmp, string(rune('a'+i)), "набор")
			text, err := g.run(out, c.args...)
			if err != nil {
				t.Fatalf("запуск: %v\n%s", err, text)
			}
			alpha, beta := filepath.Join(out, "Alpha.salbum"), filepath.Join(out, "Beta.salbum")
			if !strings.Contains(text, "alpha="+alpha+"\n") || !strings.Contains(text, "beta="+beta+"\n") {
				t.Fatalf("вывод без alpha=/beta=: %q", text)
			}
			checkQaSet(t, out, c.photos, c.alpha)
		})
	}
	t.Run("плохие аргументы", func(t *testing.T) {
		for i, args := range [][]string{
			{"-spreads", "2"},                     // Beta короче на два: меньше трёх не бывает
			{"-photos", "2", "-spreads", "3"},     // фото меньше, чем разворотов
			{"-photos", "0"},                      // ни одного фото
			{"-spreads", "99999", "-photos", "1"}, // сверх предела
		} {
			out := filepath.Join(tmp, "плохо", string(rune('a'+i)))
			if text, err := g.run(out, args...); err == nil {
				t.Errorf("%v: принято\n%s", args, text)
			}
			if _, err := os.Stat(filepath.Join(out, "Alpha.salbum")); err == nil {
				t.Errorf("%v: альбом записан при ошибке", args)
			}
		}
		if _, err := exec.Command(g.exe).CombinedOutput(); err == nil {
			t.Error("без -out принято")
		}
	})
}

// checkQaSet — круг: Alpha и Beta читает загрузчик программы (Beta на два разворота короче, фото
// общие), файлы без абсолютных путей, после переноса папки фото находятся рядом.
func checkQaSet(t *testing.T, out string, photos, spreads int) {
	t.Helper()
	alpha, beta := filepath.Join(out, "Alpha.salbum"), filepath.Join(out, "Beta.salbum")
	pa, err := salbum.Load(alpha)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := salbum.Load(beta)
	if err != nil {
		t.Fatal(err)
	}
	if len(pa.Spreads) != spreads || len(pb.Spreads) != spreads-2 {
		t.Fatalf("разворотов Alpha %d, Beta %d, ждали %d и %d", len(pa.Spreads), len(pb.Spreads), spreads, spreads-2)
	}
	if len(pa.Photos) != photos || len(pb.Photos) == 0 || len(pb.Photos) > photos {
		t.Fatalf("фото в Alpha %d (ждали %d), в Beta %d", len(pa.Photos), photos, len(pb.Photos))
	}
	checkPhotos(t, filepath.Join(out, "photos"), photos)
	for _, f := range []string{alpha, beta} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		// Только относительные пути: ни одного абсолютного, ни запасного "abs".
		if bytes.Contains(b, []byte(`"abs"`)) || bytes.Contains(b, []byte(out)) {
			t.Errorf("%s: в файле абсолютный путь", filepath.Base(f))
		}
	}
	moved := out + "-moved"
	if err := os.Rename(out, moved); err != nil {
		t.Fatal(err)
	}
	pm, err := salbum.Load(filepath.Join(moved, "Alpha.salbum"))
	if err != nil || len(pm.Photos) != photos {
		t.Fatalf("после переноса: %v, фото %d", err, len(pm.Photos))
	}
	for _, ref := range pm.Photos {
		if ref.Alt != "" || !strings.HasPrefix(ref.Path, moved) {
			t.Fatalf("после переноса путь %q (запасной %q)", ref.Path, ref.Alt)
		}
	}
}

// checkPhotos — в папке n JPEG 1600×1067, кадры различимы по цвету фона.
func checkPhotos(t *testing.T, dir string, n int) {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil || len(ents) != n {
		t.Fatalf("в %s файлов %d (%v), ждали %d", dir, len(ents), err, n)
	}
	seen := map[[3]uint32]string{}
	for _, en := range ents {
		f, err := os.Open(filepath.Join(dir, en.Name()))
		if err != nil {
			t.Fatal(err)
		}
		img, err := jpeg.Decode(f)
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", en.Name(), err)
		}
		if b := img.Bounds(); b.Dx() != 1600 || b.Dy() != 1067 {
			t.Fatalf("%s: %dx%d", en.Name(), b.Dx(), b.Dy())
		}
		r, g, b, _ := img.At(100, 100).RGBA()
		key := [3]uint32{r >> 11, g >> 11, b >> 11} // грубо: JPEG чуть смазывает
		if prev, ok := seen[key]; ok && n <= 12 {
			t.Errorf("%s и %s одного цвета", prev, en.Name())
		}
		seen[key] = en.Name()
	}
}
