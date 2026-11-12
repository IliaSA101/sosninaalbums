package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// shareJob: причины Block (не сохранён, нет разворотов, лист-виньетка) исключают выгрузку при
// любом состоянии; пропавшие фото — Refuse, а не Block (T5-7); исправный альбом — страницы и хэш сцены.
func TestShareJob(t *testing.T) {
	a := newApp(t)
	dir := testkit.TempDir(t)
	saved := filepath.Join(dir, "Свадьба.salbum")
	photo := testkit.JPEG(t, dir, "p0.jpg", 60, 40)
	album := func(spreads int) *model.Project {
		p := model.New()
		p.Settings.PageW, p.Settings.PageH = 60, 40
		p.Photos["p0"] = model.PhotoRef{Path: photo, W: 60, H: 40}
		for i := range spreads {
			p.Spreads = append(p.Spreads, &model.Spread{ID: string(rune('a' + i)), Photos: []string{"p0"}})
		}
		return p
	}
	use := func(p *model.Project, path string) {
		a.ed.Store(mustEditor(t, p, path))
	}

	use(album(1), "")
	if j := a.shareJob(); !j.Save || !strings.Contains(j.Block, "Сначала сохраните") || j.Refuse != "" {
		t.Errorf("не сохранён: %+v", j)
	}
	use(album(0), saved)
	if j := a.shareJob(); j.Block != "В альбоме нет разворотов" || j.Save || j.Title != "Свадьба" {
		t.Errorf("нет разворотов: block %q save %v title %q", j.Block, j.Save, j.Title)
	}
	sheet := model.NewSheet(204, 304)
	sheet.Spreads = album(1).Spreads
	use(sheet, saved)
	if j := a.shareJob(); !strings.Contains(j.Block, "Лист виньетки") || j.Refuse != "" {
		t.Errorf("лист-виньетка: %+v", j)
	}
	// Исправный альбом: ни Block, ни Refuse; страницы и сцена есть; больше разворотов — больше страниц.
	use(album(1), saved)
	one := a.shareJob()
	if one.Block != "" || one.Refuse != "" || one.Scene == "" || one.Pages < 2 || len(one.Spreads) != 1 {
		t.Fatalf("исправный альбом: block %q refuse %q scene %q pages %d", one.Block, one.Refuse, one.Scene, one.Pages)
	}
	use(album(3), saved)
	if three := a.shareJob(); three.Pages <= one.Pages || three.Scene == one.Scene {
		t.Errorf("3 разворота: страниц %d (было %d), сцена та же: %v", three.Pages, one.Pages, three.Scene == one.Scene)
	}
	// Пропавшее фото — отказ словами Go, но не Block: ссылку готового альбома это не прячет.
	a.exists = func(string) bool { return false }
	if j := a.shareJob(); j.Block != "" || j.Refuse == "" {
		t.Errorf("пропавшие фото: block %q refuse %q", j.Block, j.Refuse)
	}
}
