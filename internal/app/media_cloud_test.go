package app

// 160-M1, доправка 5: фото проекта в облаке (OneDrive «только в сети») не скачивается фоновыми и массовыми путями.

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io/fs"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/contactsheet"
	"github.com/IliaSA101/sosninaalbums/internal/faces"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// cloudRig — App с кэшем миниатюр и двумя фото только в проекте (ленты нет): «stub» лежит в облаке, «plain» — обычный.
// Любое чтение заголовка заглушки (readColor) и её открытие — провал. Фото ленты в реестре не держим.
type cloudRig struct {
	a           *App
	cacheDir    string
	stub, plain library.Photo
	colorReads  atomic.Int64 // вызовы readColor для заглушки
	stubIsCloud atomic.Bool  // шов cloudStub: «stub.jpg» в облаке
}

func newCloudRig(t *testing.T) *cloudRig {
	t.Helper()
	r := &cloudRig{a: newApp(t)}
	r.a.emit = func(string, ...any) {}
	r.cacheDir = testkit.TempDir(t)
	cache, err := library.NewCache(r.cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	r.a.cache = cache
	dir := testkit.TempDir(t)
	mk := func(name string, w, h int) library.Photo {
		p := testkit.JPEG(t, dir, name, w, h)
		return library.Photo{ID: library.IDFor(p), Path: p, Name: name, W: w, H: h}
	}
	r.stub, r.plain = mk("stub.jpg", 64, 48), mk("plain.jpg", 48, 64)
	for _, p := range []library.Photo{r.stub, r.plain} {
		if _, err := r.a.editor().NewSpread(-1, []model.PhotoInfo{{ID: p.ID, Path: p.Path, W: p.W, H: p.H}}); err != nil {
			t.Fatal(err)
		}
	}
	r.stubIsCloud.Store(true)
	oldD, oldC := cloudStub, readColor
	t.Cleanup(func() { cloudStub, readColor = oldD, oldC })
	cloudStub = func(path string) bool { return r.stubIsCloud.Load() && filepath.Base(path) == "stub.jpg" }
	readColor = func(path string) (imgfile.Color, string) {
		if filepath.Base(path) == "stub.jpg" {
			r.colorReads.Add(1)
			// Фото с Adobe-профилем: ключ кэша зависит от цвета, поиск копии без заголовка обязан его перебрать.
			return imgfile.ColorAdobe, "Adobe RGB"
		}
		return imgfile.ColorNone, ""
	}
	return r
}

// cloudCacheFiles — число файлов в кэше миниатюр: генерация оригинала оставила бы след.
func cloudCacheFiles(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// TestMediaCloudProjectPhoto: /media/t|p|o для фото проекта в облаке. Готовая копия отдаётся без чтения заголовка и
// оригинала (ключ кэша — по любому классу цвета); копии нет — 404 без скачивания и без новых файлов в кэше;
// оригинал не отдаётся; обычное фото рядом работает как прежде. Блоки независимы.
func TestMediaCloudProjectPhoto(t *testing.T) {
	r := newCloudRig(t)
	a := r.a
	thumb, prev := r.warmBeforeCloud(t)

	t.Run("готовая копия", func(t *testing.T) {
		for u, want := range map[string][]byte{"/media/t/": thumb, "/media/p/": prev} {
			rec := origReq(a, http.MethodGet, u+r.stub.ID)
			if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), want) {
				t.Errorf("%s: код %d, %d байт (ждали копию %d байт)", u, rec.Code, rec.Body.Len(), len(want))
			}
		}
		if n := r.colorReads.Load(); n != 0 {
			t.Errorf("заголовок заглушки прочитан %d раз", n)
		}
	})
	t.Run("копии нет", func(t *testing.T) {
		// Копию убираем: ключ кэша другого файла не подойдёт, поэтому берём новую заглушку.
		fresh := testkit.JPEG(t, testkit.TempDir(t), "stub.jpg", 40, 30)
		id := library.IDFor(fresh)
		if _, err := a.editor().NewSpread(-1, []model.PhotoInfo{{ID: id, Path: fresh, W: 40, H: 30}}); err != nil {
			t.Fatal(err)
		}
		before := cloudCacheFiles(t, r.cacheDir)
		for _, u := range []string{"/media/t/", "/media/p/", "/media/o/"} {
			if rec := origReq(a, http.MethodGet, u+id); rec.Code != http.StatusNotFound {
				t.Errorf("%s: код %d, ожидался 404", u, rec.Code)
			}
		}
		if n := cloudCacheFiles(t, r.cacheDir); n != before {
			t.Errorf("в кэше появились файлы: было %d, стало %d (оригинал заглушки читался)", before, n)
		}
		if n := r.colorReads.Load(); n != 0 {
			t.Errorf("заголовок заглушки прочитан %d раз", n)
		}
	})
	t.Run("оригинал не отдаётся", func(t *testing.T) {
		if rec := origReq(a, http.MethodGet, "/media/o/"+r.stub.ID); rec.Code != http.StatusNotFound {
			t.Errorf("/media/o: код %d, ожидался 404", rec.Code)
		}
	})
	t.Run("обычное фото рядом", func(t *testing.T) {
		for _, u := range []string{"/media/t/", "/media/p/", "/media/o/"} {
			if rec := origReq(a, http.MethodGet, u+r.plain.ID); rec.Code != http.StatusOK {
				t.Errorf("%s: код %d", u, rec.Code)
			}
		}
	})
	t.Run("облако ушло — как обычное", func(t *testing.T) {
		r.stubIsCloud.Store(false)
		defer r.stubIsCloud.Store(true)
		old := readColor
		readColor = func(string) (imgfile.Color, string) { return imgfile.ColorNone, "" } // Adobe оригиналом не отдаётся (415)
		defer func() { readColor = old }()
		if rec := origReq(a, http.MethodGet, "/media/o/"+r.stub.ID); rec.Code != http.StatusOK {
			t.Errorf("скачанное: /media/o код %d", rec.Code)
		}
	})
}

// TestCloudProjectPhotoSkippedByBackground: агент (превью, контактный лист) и поиск лиц фото в облаке без копии не читают:
// для них оно «только в облаке» (ErrCloudOnly, клетка «×», Missing с Cloud), оригинал не открывается, заголовок не
// читается. Копии и остальные читатели — cloudcopy_test.go.
func TestCloudProjectPhotoSkippedByBackground(t *testing.T) {
	r := newCloudRig(t)
	a := r.a
	ctx := context.Background()

	t.Run("mediaPhoto", func(t *testing.T) {
		if _, ok, gone := a.mediaPhoto(r.stub.ID); ok || gone {
			t.Errorf("заглушка: ok=%v gone=%v, ожидалось ok=false", ok, gone)
		}
		if _, ok, gone := a.mediaPhoto(r.plain.ID); !ok || gone {
			t.Errorf("обычное фото: ok=%v gone=%v", ok, gone)
		}
		if p, ok, gone, cloud := a.mediaPhotoCloud(r.stub.ID); !ok || gone || !cloud || p.Path != r.stub.Path || p.W != 0 {
			t.Errorf("mediaPhotoCloud: %+v %v %v %v", p, ok, gone, cloud)
		}
	})
	t.Run("превью агента", func(t *testing.T) {
		if _, err := a.agentsPreview(ctx, r.stub.ID, 200); !errors.Is(err, agents.ErrCloudOnly) {
			t.Errorf("agentsPreview заглушки: %v, ожидалась ErrCloudOnly", err)
		}
		if b, err := a.agentsPreview(ctx, r.plain.ID, 200); err != nil || len(b) == 0 {
			t.Errorf("agentsPreview обычного: %d байт, %v", len(b), err)
		}
	})
	t.Run("контактный лист", func(t *testing.T) {
		if c := a.sheetCell(ctx, r.stub.ID, 100); c.state != contactsheet.Broken {
			t.Errorf("клетка заглушки: %v, ожидалась Broken", c.state)
		}
		if c := a.sheetCell(ctx, r.plain.ID, 100); c.state != contactsheet.Ready {
			t.Errorf("клетка обычного: %v", c.state)
		}
	})
	t.Run("поиск лиц", func(t *testing.T) {
		var calls atomic.Int64
		a.faceSt.detect = func(image.Image) []faces.Face { calls.Add(1); return nil }
		h := model.FaceHit{Photo: r.stub.ID}
		a.faceJob(ctx, &h)
		if !h.Missing || !h.Cloud || calls.Load() != 0 {
			t.Errorf("заглушка: Missing=%v Cloud=%v, детектор вызван %d раз", h.Missing, h.Cloud, calls.Load())
		}
		ok := model.FaceHit{Photo: r.plain.ID}
		a.faceJob(ctx, &ok)
		if ok.Missing || calls.Load() != 1 {
			t.Errorf("обычное: Missing=%v, детектор вызван %d раз", ok.Missing, calls.Load())
		}
	})
	if n := r.colorReads.Load(); n != 0 {
		t.Errorf("заголовок заглушки прочитан %d раз", n)
	}
	// Копии обычного фото в кэше есть, заглушки — нет: оригинал заглушки не декодировался.
	if _, ok := a.cache.Cached(r.stub, true); ok {
		t.Error("для заглушки появилась копия: оригинал читался")
	}
}

// warmBeforeCloud строит копии заглушки, пока она ещё «на диске» (кэш по Adobe-ключу), и переводит её в облако.
func (r *cloudRig) warmBeforeCloud(t *testing.T) (thumb, prev []byte) {
	t.Helper()
	a := r.a
	r.stubIsCloud.Store(false)
	for _, k := range []struct {
		u   string
		dst *[]byte
	}{{"/media/t/", &thumb}, {"/media/p/", &prev}} {
		rec := origReq(a, http.MethodGet, k.u+r.stub.ID)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s до облака: код %d", k.u, rec.Code)
		}
		*k.dst = rec.Body.Bytes()
	}
	r.stubIsCloud.Store(true)
	r.colorReads.Store(0)
	return thumb, prev
}
