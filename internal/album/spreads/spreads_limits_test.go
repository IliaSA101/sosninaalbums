package spreads_test

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/spreads"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func manyPhotos(n int) []model.PhotoInfo {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = "p" + strconv.Itoa(i)
	}
	return edtest.Photos(ids...)
}

func TestSpreadPhotoLimit(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	v, err := e.NewSpread(-1, manyPhotos(model.MaxPhotos))
	if len(v.Spreads) != 1 || err != nil {
		t.Fatal("на пределе разворот должен создаться")
	}
	id := v.Spreads[0].ID
	// Лишнее фото — отказ с текстом, проект не меняется.
	rev := e.View().Rev
	v, err = e.AddPhotos(id, edtest.Photos("extra"), -1)
	if err == nil || !strings.Contains(err.Error(), "16") || v.Rev != rev {
		t.Fatalf("ждали отказ без правки: err=%v rev=%d/%d", err, v.Rev, rev)
	}
	// Переставить своё же фото на пределе можно.
	if _, err = e.AddPhotos(id, edtest.Photos("p0"), -1); err != nil {
		t.Fatal("перенос внутри разворота на пределе не должен отказывать")
	}
	if v, err = e.NewSpread(-1, manyPhotos(model.MaxPhotos+1)); err == nil || len(v.Spreads) != 1 {
		t.Fatal("новый разворот сверх предела должен отказать")
	}
	edtest.NewSpread(e, -1, edtest.Photos("x"))
	if _, err = e.MovePhoto("x", id, -1); err == nil {
		t.Fatal("MovePhoto сверх предела должен отказать")
	}
	// Автосборка режет по MaxSpreadPhotos.
	v, _ = edtest.Open(t, model.New(), "").AutoBuild(manyPhotos(40), -1, 20)
	for _, s := range v.Spreads {
		if len(s.Photos) > model.MaxSpreadPhotos {
			t.Fatalf("разворот из %d фото", len(s.Photos))
		}
	}
}

func TestDuplicatesDontBypassLimit(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	dup := append(manyPhotos(model.MaxPhotos), manyPhotos(model.MaxPhotos)...) // 2*MaxPhotos записей, MaxPhotos разных
	v, err := e.NewSpread(-1, dup)
	if err != nil || len(v.Spreads) != 1 || len(v.Spreads[0].Photos) != model.MaxPhotos {
		t.Fatalf("повторы должны схлопнуться: err=%v", err)
	}
	// Лишнее фото вместе с повтором: отказ, откат без шага отмены.
	rev, id := e.View().Rev, v.Spreads[0].ID
	v, err = e.AddPhotos(id, append(edtest.Photos("p0"), edtest.Photos("extra", "extra")...), -1)
	if err == nil || v.Rev != rev || len(v.Spreads[0].Photos) != model.MaxPhotos {
		t.Fatalf("ждали откат: err=%v rev=%d/%d", err, v.Rev, rev)
	}
	if p, _ := e.Snapshot(); p.Photos["extra"].Path != "" {
		t.Fatal("фото из отклонённой правки осталось в проекте")
	}
	if v = e.Undo(); len(v.Spreads) != 0 {
		t.Fatal("отказ не должен был создать шаг отмены")
	}
}

func TestClampSettings(t *testing.T) {
	got := model.ClampSettings(model.Settings{PageW: math.NaN(), PageH: 5000, Margin: math.Inf(1), Gap: -3, DPI: 99999})
	d := model.DefaultSettings()
	if got.PageW != d.PageW || got.PageH != 1000 || got.Margin != d.Margin || got.Gap != 0 || got.DPI != 1200 {
		t.Fatalf("clamp: %+v", got)
	}
	got = model.ClampSettings(model.Settings{PageW: 10, PageH: 200, Margin: 100, Gap: 100, DPI: 10})
	if got.PageW != 50 || got.Margin != 12.5 || got.Gap != 12.5 || got.DPI != 72 {
		t.Fatalf("clamp малых: %+v", got)
	}
	e := edtest.Open(t, model.New(), "")
	if v := e.SetSettings(model.Settings{PageW: 1e9, PageH: 300, DPI: 300}); v.Settings.PageW != 1000 {
		t.Fatalf("SetSettings не зажал: %+v", v.Settings)
	}
}

// AutoBuild сверх MaxSpreads — ошибка с понятным текстом, проект не меняется.
func TestAutoBuildSpreadLimitError(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.Change(e, func(p *model.Project) {
		for range model.MaxSpreads {
			p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}})
		}
	})
	rev := edtest.Rev(e)
	v, err := e.AutoBuild(edtest.Photos("a", "b"), -1, 2)
	if err == nil || !strings.Contains(err.Error(), strconv.Itoa(model.MaxSpreads)) || !strings.Contains(err.Error(), "разворотов") {
		t.Fatalf("ждали ошибку про %d разворотов, получили %v", model.MaxSpreads, err)
	}
	if len(v.Spreads) != model.MaxSpreads || edtest.Rev(e) != rev || len(edtest.P(e).Photos) != 0 {
		t.Fatalf("проект изменился: %d разворотов, rev %d→%d", len(v.Spreads), rev, edtest.Rev(e))
	}
	// Без переполнения — ошибки нет.
	e2 := edtest.Open(t, model.New(), "")
	if v, err := e2.AutoBuild(edtest.Photos("a", "b"), -1, 2); err != nil || len(v.Spreads) != 1 {
		t.Fatalf("обычная автосборка: err=%v", err)
	}
}

// AddFolder сверх MaxFolders — ошибка без шага отмены; уже добавленная — холостая.
func TestAddFolderLimit(t *testing.T) {
	dir := testkit.TempDir(t)
	e := edtest.Open(t, model.New(), "")
	edtest.Change(e, func(p *model.Project) {
		for i := range model.MaxFolders {
			p.Folders = append(p.Folders, filepath.Join(dir, fmt.Sprint(i)))
		}
	})
	n, rev := edtest.UndoLen(e), e.View().Rev
	if _, err := e.AddFolder(filepath.Join(dir, "ещё")); err == nil {
		t.Fatal("папка сверх предела добавилась")
	}
	if v, err := e.AddFolder(filepath.Join(dir, "0")); err != nil || v.Rev != rev || edtest.UndoLen(e) != n {
		t.Fatalf("повтор папки: %v, rev %d→%d", err, rev, v.Rev)
	}
}

// Тело AddFolder мимо обёртки Editor: UNC, устройство, относительный и пустой путь — та же
// ошибка, что у обёртки, папка не добавляется; локальный абсолютный — добавляется.
func TestAddFolderBodyChecksPath(t *testing.T) {
	p := model.New()
	for _, dir := range []string{`\\server\share\фото`, `\\?\C:\фото`, `\\.\C:\фото`, "фото", ""} {
		err := spreads.AddFolder(p, dir)
		if want := paths.CheckLocalPath(dir); err == nil || want == nil || err.Error() != want.Error() {
			t.Errorf("путь %q: ошибка %v, нужна %v", dir, err, want)
		}
	}
	if len(p.Folders) != 0 {
		t.Fatalf("непроверенный путь в проекте: %q", p.Folders)
	}
	dir := testkit.TempDir(t)
	if err := spreads.AddFolder(p, dir); err != nil || len(p.Folders) != 1 {
		t.Fatalf("локальная папка: %v, папок %d", err, len(p.Folders))
	}
}
