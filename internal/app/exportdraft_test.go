package app

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Заготовки макета (131 G2, 0.27-131-5) в выгрузке через App: вопрос и пиксели.

// draftBlock — крупная чёрная надпись на всю первую страницу: на белом фоне заметна в пикселях.
func draftBlock(txt string, draft bool) *model.TextBlock {
	st := model.DefaultCaptionStyle()
	st.Size, st.Color = 60, "#000000"
	return &model.TextBlock{ID: "d00000000001", X: 2, Y: 2, W: 56, Text: txt, Style: st, Draft: draft}
}

// decodeJPEG — разобранный файл экспорта.
func decodeJPEG(t *testing.T, path string) image.Image {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func sameImage(a, b image.Image) bool {
	if a.Bounds() != b.Bounds() {
		return false
	}
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			if a.At(x, y) != b.At(x, y) {
				return false
			}
		}
	}
	return true
}

// TestExportAsksDrafts — вопрос после выбора папки называет пустые места и заготовки; «нет»
// ничего не пишет; только заготовки — свой текст; без заготовок и пустых мест вопроса нет.
func TestExportAsksDrafts(t *testing.T) {
	e := newExportEnv(t, 3)
	p, _ := e.a.editor().Snapshot()
	p.Spreads[0].Texts = []*model.TextBlock{draftBlock("[Имена]", true)}
	p.Spreads[2].Grid = []*model.Place{{ID: "00000000a001", X: 5, Y: 5, W: 20, H: 20}}
	p.Spreads[2].Photos = nil
	e.a.ed.Store(mustEditor(t, p, ""))
	e.answer = false
	want := "На разворотах 1, 3 пустые места: 1, заготовки: 1. Заготовки не печатаются, на месте пустых мест будет фон. Выгрузить?"
	if ok, err := e.a.Export(ExportOptions{DPI: 72}); ok || err != nil || len(e.asked) != 1 || e.asked[0] != want {
		t.Fatalf("вопрос: %v %v %q", ok, err, e.asked)
	}
	if e.exportBusy() {
		t.Fatal("«нет» не снял флаг экспорта")
	}
	// Диапазон без третьего листа — пустых мест нет, заготовка осталась.
	e.asked = nil
	if ok, err := e.a.Export(ExportOptions{DPI: 72, Range: "1-2"}); ok || err != nil || len(e.asked) != 1 ||
		e.asked[0] != "На развороте 1 заготовки: 1. Заготовки не печатаются. Выгрузить?" {
		t.Fatalf("только заготовка: %v %v %q", ok, err, e.asked)
	}
	// Диапазон без обоих листов — вопроса нет, файл пишется.
	e.asked, e.answer = nil, true
	if msg := e.export(t, ExportOptions{Range: "2"}); msg != "" || len(e.asked) != 0 {
		t.Fatalf("лист без заготовок: %q, вопросы %q", msg, e.asked)
	}
}

// TestExportOneQuestionDraftOnly — лист только с заготовкой: один вопрос, о заготовках после
// папки (0.28-131-19), а не ещё и «разворот пустой» до неё; «да» — файл пишется, вопрос один.
func TestExportOneQuestionDraftOnly(t *testing.T) {
	e := newExportEnv(t, 2)
	p, _ := e.a.editor().Snapshot()
	p.Spreads[1].Photos = nil
	p.Spreads[1].Texts = []*model.TextBlock{draftBlock("[Имена]", true)}
	e.a.ed.Store(mustEditor(t, p, ""))
	want := "На развороте 2 заготовки: 1. Заготовки не печатаются. Разворот 2 выйдет пустым. Выгрузить?"
	e.answer = false
	if ok, err := e.a.Export(ExportOptions{DPI: 72}); ok || err != nil || len(e.asked) != 1 || e.asked[0] != want {
		t.Fatalf("«нет»: %v %v %q", ok, err, e.asked)
	}
	e.asked, e.answer = nil, true
	if msg := e.export(t, ExportOptions{DPI: 72}); msg != "" || len(e.asked) != 1 || e.asked[0] != want {
		t.Fatalf("«да»: %q, вопросы %q", msg, e.asked)
	}
}

// TestExportSkipsDraft — через App: заготовка в JPEG не попадает (кадр равен кадру без неё),
// а обычная надпись с той же строкой видна (проверка, что тест видит текст).
func TestExportSkipsDraft(t *testing.T) {
	e := newExportEnv(t, 1)
	shot := func(tb *model.TextBlock, name string) image.Image {
		t.Helper()
		p, _ := e.a.editor().Snapshot()
		p.Spreads[0].Texts = nil
		if tb != nil {
			p.Spreads[0].Texts = []*model.TextBlock{tb}
		}
		e.a.ed.Store(mustEditor(t, p, ""))
		e.dir = filepath.Join(filepath.Dir(e.dir), name)
		e.asked = nil
		if msg := e.export(t, ExportOptions{Pages: true}); msg != "" {
			t.Fatalf("экспорт %s: %q", name, msg)
		}
		return decodeJPEG(t, filepath.Join(e.dir, "01-1.jpg"))
	}
	none := shot(nil, "none")
	draft := shot(draftBlock("MMMM", true), "draft")
	plain := shot(draftBlock("MMMM", false), "plain")
	if !sameImage(none, draft) {
		t.Error("заготовка попала в JPEG")
	}
	if sameImage(none, plain) {
		t.Error("обычная надпись не видна в JPEG: тест слеп")
	}
}
