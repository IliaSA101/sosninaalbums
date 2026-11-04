//go:build windows

package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestCloudProjectPhotoNoFetch (160-M1, доправка 5): настоящая заглушка Cloud Files API как фото проекта. /media/t|p|o,
// превью агента и поиск лиц её не скачивают (колбэк FETCH_DATA не вызывается). Готовую копию Windows-тест не проверяет:
// скачанная заглушка перестаёт быть заглушкой (её проверяет переносимый TestMediaCloudProjectPhoto). Блоки независимы.
func TestCloudProjectPhotoNoFetch(t *testing.T) {
	root := testkit.NewCloudRoot(t)
	// Обычный JPEG: ExportJPEG несёт метку нашей выгрузки.
	photo, err := os.ReadFile(testkit.JPEG(t, testkit.TempDir(t), "src.jpg", 64, 48))
	if err != nil {
		t.Fatal(err)
	}
	p := root.Stub(t, "cloud.jpg", photo)
	a := newApp(t)
	a.emit = func(string, ...any) {}
	cache, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = cache
	id := library.IDFor(p)
	if _, err := a.editor().NewSpread(-1, []model.PhotoInfo{{ID: id, Path: p, W: 64, H: 48}}); err != nil {
		t.Fatal(err)
	}

	t.Run("media", func(t *testing.T) {
		for _, u := range []string{"/media/t/", "/media/p/", "/media/o/"} {
			if rec := origReq(a, http.MethodGet, u+id); rec.Code != http.StatusNotFound {
				t.Errorf("%s: код %d, ожидался 404", u, rec.Code)
			}
		}
	})
	t.Run("агент", func(t *testing.T) {
		if _, err := a.agentsPreview(context.Background(), id, 200); !errors.Is(err, agents.ErrCloudOnly) {
			t.Errorf("agentsPreview: %v, ожидалась ErrCloudOnly", err)
		}
		h := model.FaceHit{Photo: id}
		a.faceJob(context.Background(), &h)
		if !h.Missing || !h.Cloud {
			t.Error("поиск лиц не пропустил заглушку с причиной «в облаке»")
		}
	})
	if n := root.Fetches(); n != 0 {
		t.Errorf("заглушка скачана: колбэк вызван %d раз", n)
	}
	for _, msg := range root.Errors() {
		t.Errorf("поставщик: %s", msg)
	}
}
