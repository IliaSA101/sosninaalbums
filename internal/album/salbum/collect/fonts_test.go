package collect

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Свои шрифты в плане сбора (082-8, 082-13): имена в «Шрифтах», повторы и уже лежащие файлы,
// запрет переноса, чужие расширения, сводка.

// fontFile — файл библиотеки шрифтов с содержимым data.
func fontFile(t *testing.T, dir, name, data string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAddFontsPlan(t *testing.T) {
	lib, out := testkit.TempDir(t), testkit.TempDir(t)
	reg := fontFile(t, lib, "0123456789abcdef.ttf", "обычный")
	bold := fontFile(t, lib, "1123456789abcdef.otf", "жирный")
	bi := fontFile(t, lib, "2123456789abcdef.ttf", "жирный курсив")
	lim := fontFile(t, lib, "3123456789abcdef.ttf", "запрет")
	odd := fontFile(t, lib, "4123456789abcdef.woff", "веб")
	slash := fontFile(t, lib, "5123456789abcdef.ttf", "косая черта")
	it := fontFile(t, lib, "6123456789abcdef.ttf", "курсив")
	fonts := map[string][]FontSrc{
		"my:aaaaaaaaaaaaaaaa": {{Path: reg, Name: "Lato"}, {Path: bold, Name: "Lato", Bold: true}},
		"my:bbbbbbbbbbbbbbbb": {{Path: bi, Name: "Bodoni Moda", Bold: true, Italic: true}, {Path: lim, Name: "Bodoni Moda", Italic: true, Restricted: true}},
		"my:cccccccccccccccc": {{Path: odd, Name: "Web"}, {Path: filepath.Join(lib, "нет.ttf"), Name: "Нет"}},
		"my:dddddddddddddddd": {{Path: slash, Name: "A/B: C"}, {Path: it, Name: "A/B: C", Italic: true}},
	}
	// «Шрифты» в папке сбора: чужой файл под именем обычного Lato — копия ляжет под « (2)».
	if err := os.MkdirAll(filepath.Join(out, DirFonts), 0o755); err != nil {
		t.Fatal(err)
	}
	fontFile(t, filepath.Join(out, DirFonts), "Lato-Regular.ttf", "чужой файл другой длины")

	var pl Plan
	if err := pl.AddFonts(fonts, 2, out, os.Stat); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range pl.Items {
		got = append(got, it.Rel)
		if it.Kind != KindFont || it.Skip || len(it.Refs) != 0 {
			t.Errorf("элемент: %+v", it)
		}
	}
	want := []string{DirFonts + "/Lato-Regular (2).ttf", DirFonts + "/Lato-Bold.otf", DirFonts + "/Bodoni Moda-BoldItalic.ttf", DirFonts + "/A B C-Regular.ttf", DirFonts + "/A B C-Italic.ttf"} // запретные знаки — пробелом (paths.FileName)
	if !slices.Equal(got, want) {
		t.Fatalf("имена %q, ждали %q", got, want)
	}
	in := pl.Info()
	if in.Fonts != 5 || !slices.Equal(in.FontsRestricted, []string{"Bodoni Moda"}) || in.MissingFonts != 2 || in.Bytes != pl.Bytes || pl.Bytes == 0 {
		t.Fatalf("сводка: %+v", in)
	}

	// Скопированные файлы при повторе — «уже на месте»: байты не считаются.
	if _, err := Copy(context.Background(), out, &pl, nil); err != nil {
		t.Fatal(err)
	}
	var again Plan
	if err := again.AddFonts(fonts, 0, out, os.Stat); err != nil {
		t.Fatal(err)
	}
	for i, it := range again.Items {
		if !it.Skip || it.Rel != pl.Items[i].Rel {
			t.Errorf("повтор: %+v", it)
		}
	}
	if again.Bytes != 0 || again.Info().MissingFonts != 0 {
		t.Fatalf("повтор: байт %d", again.Bytes)
	}

	// Пусто — ни элементов, ни запретов; список запретов в сводке — не nil.
	var none Plan
	if err := none.AddFonts(nil, 0, "", os.Stat); err != nil || len(none.Items) != 0 || none.Info().FontsRestricted == nil {
		t.Fatalf("пусто: %+v %v", none, err)
	}
}

// Шрифты — элементы плана без записей альбома: собранный альбом их путей не получает, таблица
// шрифтов копии — как у исходного.
func TestAlbumKeepsFonts(t *testing.T) {
	out := testkit.TempDir(t)
	p := model.New()
	p.Fonts = map[string]model.FontRef{"my:aaaaaaaaaaaaaaaa": {Name: "Lato"}}
	pl := Plan{Items: []Item{{Kind: KindFont, Src: filepath.Join(out, "x.ttf"), Rel: DirFonts + "/Lato-Regular.ttf", ModTime: time.Now()}}}
	q, _ := Album(p, model.Side{}, out, pl)
	if len(q.Folders) != 0 || q.Fonts["my:aaaaaaaaaaaaaaaa"].Name != "Lato" {
		t.Fatalf("собранный: папки %v, шрифты %v", q.Folders, q.Fonts)
	}
}
