package app

// 160-M1, хвост: агент (превью), контактный лист, поиск лиц, цвета фото и карточка фото не открывают заглушку облака.
// Готовая копия кэша берётся (подходящего размера), без неё — причина «файл только в облаке» и запись в Unread.

import (
	"context"
	"errors"
	"image"
	"os"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/contactsheet"
	"github.com/IliaSA101/sosninaalbums/internal/faces"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// cloudReaders — результаты чтения фото id пятью читателями фото; детектор лиц считает вызовы.
type cloudReaders struct {
	preview    []byte
	previewErr error
	cell       sheetCell
	hit        model.FaceHit
	detects    int64
	colors     []string
	colorsErr  error
	sheet      *image.RGBA // кадр листа для агента (get_spread_preview)
	sheetErr   error
}

// readAll вызывает читателей на фото id (лист sid нужен для цветов). Работа с диском — только через кэш.
func readAll(t *testing.T, a *App, id, sid string) cloudReaders {
	t.Helper()
	var calls atomic.Int64
	a.faceSt.detect = func(image.Image) []faces.Face { calls.Add(1); return nil }
	ctx := context.Background()
	var r cloudReaders
	r.preview, r.previewErr = a.agentsPreview(ctx, id, 200)
	r.cell = a.sheetCell(ctx, id, 100)
	r.hit = model.FaceHit{Photo: id}
	a.faceJob(ctx, &r.hit)
	r.detects = calls.Load()
	r.colors, r.colorsErr = a.colorPick.PhotoColors(sid)
	r.sheet, r.sheetErr = a.colorPick.Sheet(ctx, sid, 200)
	return r
}

// sheetColors — число разных цветов в кадре листа: пустой лист — одна заливка, фото добавляет остальные.
func sheetColors(img *image.RGBA) int {
	seen := map[[4]uint8]bool{}
	for i := 0; i+3 < len(img.Pix); i += 4 {
		seen[[4]uint8(img.Pix[i:i+4])] = true
	}
	return len(seen)
}

// wantOtherSizeShown: осталась копия другого размера — записи «только в облаке» нет, а лист агента (если кадр
// получен) несёт фото, а не пустую заливку.
func wantOtherSizeShown(t *testing.T, a *App, id string, got cloudReaders) {
	t.Helper()
	if why, bad := a.GetState().Unread[id]; bad {
		t.Errorf("есть копия другого размера, а Unread[%s] = %q", id, why)
	}
	if got.sheetErr != nil || got.sheet == nil || sheetColors(got.sheet) < 2 {
		t.Errorf("лист агента: %v; фото с одной копией должно быть в кадре", got.sheetErr)
	}
}

// sheetWith — ID листа, на котором стоит фото id.
func sheetWith(t *testing.T, a *App, id string) string {
	t.Helper()
	for _, s := range a.editor().View().Spreads {
		for _, ph := range s.Photos {
			if ph == id {
				return s.ID
			}
		}
	}
	t.Fatalf("фото %s не стоит ни на одном развороте", id)
	return ""
}

// wantCloudReasons — все читатели отказали по облаку: ошибка агента ErrCloudOnly, клетка «×», у лица Missing с Cloud
// (детектор не вызван), цвета пусты; фото записано в Unread с причиной; оригинал не читался, в кэше ничего не прибавилось.
func wantCloudReasons(t *testing.T, a *App, id string, got cloudReaders, cacheFilesBefore, cacheFiles int, colorReads int64) {
	t.Helper()
	if !errors.Is(got.previewErr, agents.ErrCloudOnly) {
		t.Errorf("превью агента: %v, ждали ErrCloudOnly", got.previewErr)
	}
	if got.cell.state != contactsheet.Broken {
		t.Errorf("клетка листа: %v, ждали Broken", got.cell.state)
	}
	if !got.hit.Missing || !got.hit.Cloud || got.detects != 0 {
		t.Errorf("лица: Missing=%v Cloud=%v, детектор вызван %d раз", got.hit.Missing, got.hit.Cloud, got.detects)
	}
	if got.colorsErr != nil || len(got.colors) != 0 {
		t.Errorf("цвета: %v, %v", got.colors, got.colorsErr)
	}
	if why := a.GetState().Unread[id]; why != library.WhyCloudOnly {
		t.Errorf("Unread[%s] = %q, ждали причину «в облаке»", id, why)
	}
	if cacheFiles != cacheFilesBefore {
		t.Errorf("в кэше появились файлы: было %d, стало %d (оригинал читался)", cacheFilesBefore, cacheFiles)
	}
	if colorReads != 0 {
		t.Errorf("заголовок заглушки прочитан %d раз", colorReads)
	}
}

// wantCopyUsed — читатели взяли копию: превью агента есть, клетка готова, лицо искали, цвета найдены; записи нет.
func wantCopyUsed(t *testing.T, a *App, id string, got cloudReaders, colorReads int64) {
	t.Helper()
	if got.previewErr != nil || len(got.preview) == 0 {
		t.Errorf("превью агента: %d байт, %v", len(got.preview), got.previewErr)
	}
	if got.cell.state != contactsheet.Ready {
		t.Errorf("клетка листа: %v, ждали Ready", got.cell.state)
	}
	if got.hit.Missing || got.hit.Cloud || got.detects != 1 {
		t.Errorf("лица: Missing=%v Cloud=%v, детектор вызван %d раз", got.hit.Missing, got.hit.Cloud, got.detects)
	}
	if got.colorsErr != nil || len(got.colors) == 0 {
		t.Errorf("цвета: %v, %v", got.colors, got.colorsErr)
	}
	if why, bad := a.GetState().Unread[id]; bad {
		t.Errorf("есть копия, а Unread[%s] = %q", id, why)
	}
	if colorReads != 0 {
		t.Errorf("заголовок заглушки прочитан %d раз", colorReads)
	}
}

// TestCloudReadersProject: фото проекта в облаке. Копии есть — берутся; копии нет — причина, а не «нет файла»;
// остался один размер — смотрящие (агент, лист) берут любой, расчёты (лица, цвета) только свой. Блоки независимы.
func TestCloudReadersProject(t *testing.T) {
	t.Run("копии есть", func(t *testing.T) {
		r := newCloudRig(t)
		r.warmBeforeCloud(t)
		sid := sheetWith(t, r.a, r.stub.ID)
		got := readAll(t, r.a, r.stub.ID, sid)
		wantCopyUsed(t, r.a, r.stub.ID, got, r.colorReads.Load())
	})
	t.Run("копий нет", func(t *testing.T) {
		r := newCloudRig(t)
		sid := sheetWith(t, r.a, r.stub.ID)
		before := cloudCacheFiles(t, r.cacheDir)
		got := readAll(t, r.a, r.stub.ID, sid)
		wantCloudReasons(t, r.a, r.stub.ID, got, before, cloudCacheFiles(t, r.cacheDir), r.colorReads.Load())
	})
	t.Run("только миниатюра", func(t *testing.T) {
		r := newCloudRig(t)
		r.warmBeforeCloud(t)
		rm, ok := r.a.cache.Cached(r.stub, true)
		if !ok {
			t.Fatal("превью не построено")
		}
		if err := os.Remove(rm); err != nil { // как Trim: превью ушло первым
			t.Fatal(err)
		}
		got := readAll(t, r.a, r.stub.ID, sheetWith(t, r.a, r.stub.ID))
		if got.previewErr != nil || len(got.preview) == 0 || got.cell.state != contactsheet.Ready {
			t.Errorf("смотрящие: превью %d байт, %v, клетка %v; ждали миниатюру вместо превью", len(got.preview), got.previewErr, got.cell.state)
		}
		if !got.hit.Missing || !got.hit.Cloud || got.detects != 0 {
			t.Errorf("лица берут только превью: Missing=%v Cloud=%v, вызовов детектора %d", got.hit.Missing, got.hit.Cloud, got.detects)
		}
		if len(got.colors) == 0 {
			t.Error("цвета считаются по миниатюре, а она есть")
		}
		wantOtherSizeShown(t, r.a, r.stub.ID, got)
	})
	t.Run("только превью", func(t *testing.T) {
		r := newCloudRig(t)
		r.warmBeforeCloud(t)
		tm, ok := r.a.cache.Cached(r.stub, false)
		if !ok {
			t.Fatal("миниатюра не построена")
		}
		if err := os.Remove(tm); err != nil {
			t.Fatal(err)
		}
		got := readAll(t, r.a, r.stub.ID, sheetWith(t, r.a, r.stub.ID))
		if got.previewErr != nil || got.cell.state != contactsheet.Ready {
			t.Errorf("смотрящие: превью %v, клетка %v; ждали превью вместо миниатюры", got.previewErr, got.cell.state)
		}
		if got.hit.Missing || got.detects != 1 {
			t.Errorf("лица берут превью, а оно есть: Missing=%v, вызовов %d", got.hit.Missing, got.detects)
		}
		if len(got.colors) != 0 {
			t.Errorf("цвета берут только миниатюру, а её нет: %v", got.colors)
		}
		wantOtherSizeShown(t, r.a, r.stub.ID, got)
	})
	t.Run("облако ушло — как обычное", func(t *testing.T) {
		r := newCloudRig(t)
		r.stubIsCloud.Store(false) // файл скачан: настоящее чтение
		got := readAll(t, r.a, r.stub.ID, sheetWith(t, r.a, r.stub.ID))
		wantCopyUsed(t, r.a, r.stub.ID, got, 0) // заголовок читается законно: файл на диске
	})
}

// TestCloudReadersRibbon: фото ленты стало заглушкой после сканирования (копию отказывает Cache.stubCopy): те же ответы.
func TestCloudReadersRibbon(t *testing.T) {
	t.Run("копии есть", func(t *testing.T) {
		r := newRibbonRig(t)
		r.warmBeforeCloud(t) // копии stub построены, пока файл на диске
		got := readAll(t, r.a, r.stub.ID, sheetWith(t, r.a, r.stub.ID))
		wantCopyUsed(t, r.a, r.stub.ID, got, r.colorReads.Load())
	})
	t.Run("копий нет", func(t *testing.T) {
		r := newRibbonRig(t)
		before := cloudCacheFiles(t, r.cacheDir)
		sid := sheetWith(t, r.a, r.stub.ID)
		got := readAll(t, r.a, r.stub.ID, sid)
		wantCloudReasons(t, r.a, r.stub.ID, got, before, cloudCacheFiles(t, r.cacheDir), r.colorReads.Load())
	})
	t.Run("только миниатюра", func(t *testing.T) {
		r := newRibbonRig(t)
		r.warmBeforeCloud(t)
		rm, _ := r.a.cache.Cached(r.stub, true)
		if err := os.Remove(rm); err != nil {
			t.Fatal(err)
		}
		got := readAll(t, r.a, r.stub.ID, sheetWith(t, r.a, r.stub.ID))
		if got.previewErr != nil || got.cell.state != contactsheet.Ready {
			t.Errorf("смотрящие: превью %v, клетка %v", got.previewErr, got.cell.state)
		}
		if !got.hit.Missing || !got.hit.Cloud {
			t.Errorf("лица берут только превью: Missing=%v Cloud=%v", got.hit.Missing, got.hit.Cloud)
		}
	})
}

// TestCloudPhotoAbout: карточка фото в облаке не открывает файл (заголовок скачал бы его): заметка вместо сведений,
// чтений нет; уже прочитанное (кэш) отдаётся как есть; после скачивания читается как обычное.
func TestCloudPhotoAbout(t *testing.T) {
	r := newCloudRig(t)
	reads := countReads(t)
	got := mustAbout(t, r.a, r.stub.ID)
	if got.Note != noteCloud || got.File != "" || len(got.Shot) != 0 || reads.Load() != 0 {
		t.Errorf("заглушка: %+v, чтений файла %d", got, reads.Load())
	}
	if got.Uses == nil || got.Used == "" {
		t.Errorf("использование фото в карточке пропало: %+v", got)
	}
	r.stubIsCloud.Store(false)
	got = mustAbout(t, r.a, r.stub.ID)
	if got.Note == noteCloud || got.File == "" || reads.Load() != 1 {
		t.Errorf("файл скачан: %+v, чтений %d", got, reads.Load())
	}
	r.stubIsCloud.Store(true) // кэш сведений: уже прочитанное отдаётся и у заглушки
	if again := mustAbout(t, r.a, r.stub.ID); again.File != got.File || again.Note == noteCloud || reads.Load() != 1 {
		t.Errorf("кэш: %+v, чтений %d", again, reads.Load())
	}
}
