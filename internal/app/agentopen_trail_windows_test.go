//go:build windows

package app

// М5 на настоящей Windows: Win32 срезает хвостовые точки и пробелы в каждом элементе пути. Тест сначала проверяет
// гипотезу (путь `<шаг>\.. \..\Вне\secret.jpg` действительно открывает файл вне места), затем — что тихий путь по
// таким строкам диска не трогает и отвечает вопросом place. Правило — настоящее (quietTrail не подменён).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func TestOpenQuietTrailingDotWindows(t *testing.T) {
	r := newOQRig(t)
	outside := filepath.Join(filepath.Dir(r.place), "Вне")
	testkit.JPEG(t, outside, "secret.jpg", 40, 30)
	s := string(filepath.Separator)
	sneaky := r.shoot + s + ".. " + s + ".. " + s + "Вне" + s + "secret.jpg"
	dir := r.shoot + s + "a." + s + "x.jpg"
	// Гипотеза: что Win32 делает с такими именами. Результат — в журнале (go test -v), на проверку не влияет.
	abs, err := filepath.Abs(sneaky)
	_, serr := os.Stat(sneaky)
	t.Logf("гипотеза М5: Abs(%q) = %q (%v); Stat открывает файл вне места: %v", sneaky, abs, err, serr == nil)
	// Что срезает Win32 в серединах и концах имён: существующая папка «Чужая» в месте и её псевдонимы с хвостом.
	alien := filepath.Join(r.place, "Чужая")
	testkit.JPEG(t, alien, "secret.jpg", 40, 30)
	for _, q := range []string{alien + ".", alien + " ", alien + s + "secret.jpg.", alien + s + "secret.jpg ",
		alien + "." + s + "secret.jpg", alien + " " + s + "secret.jpg", r.shoot + s + ".. " + s + "Чужая" + s + "secret.jpg",
		r.shoot + s + "..." + s + "Чужая" + s + "secret.jpg", r.shoot + s + ". " + s + "secret.jpg"} {
		_, perr := os.Stat(q)
		a, _ := filepath.Abs(q)
		t.Logf("проба М5: Stat(%q) успех=%v, Abs=%q, правило видит хвост=%v", q, perr == nil, a, paths.TrailingDot(q))
	}
	if !paths.TrailingDot(sneaky) || !paths.TrailingDot(dir) {
		t.Fatalf("на Windows правило должно видеть хвост: %v %v", paths.TrailingDot(sneaky), paths.TrailingDot(dir))
	}
	touched := r.watchTouch(t)
	for name, p := range map[string]string{"dotdot": sneaky, "dir_dot": dir} {
		alb := r.rawAlbum(t, "Обход_"+name, salbum.FileProject{Loose: loose(p)})
		why, asked := r.ranOrAsked(t, r.ask(r.req(alb)))
		if !asked || why != whyPlace {
			t.Errorf("%s: вопрос %v, Why = %q, ждали place", name, asked, why)
		}
		if r.a.editor().Path() != r.prev {
			t.Fatalf("%s: альбом сменился: %q", name, r.a.editor().Path())
		}
	}
	if touched.Load() != 0 {
		t.Errorf("диск тронут по путям с хвостом или вне мест: %d раз", touched.Load())
	}
}
