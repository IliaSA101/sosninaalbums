package app

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestPagePx (123 §1.3): страница по px файла — чистый ответ (проект и его rev не меняются), всегда
// при 300 dpi, даже если dpi альбома другой; ошибка — строкой под полем, размеры нулевые.
func TestPagePx(t *testing.T) {
	e, _ := printEnv(t, 1, 304, 203, "fabrika-photo")
	p, _ := e.a.editor().Snapshot()
	st := p.Settings
	st.DPI = 150 // px считаются при 300 dpi выгрузки, а не при dpi альбома
	e.a.editor().SetSettings(st)
	_, v0 := e.a.editor().Snapshot()
	for _, c := range []struct {
		w, h   int
		pw, ph float64
	}{{7181, 2398, 304, 203}, {7180, 2397, 303.95, 202.95}, {7087, 2362, 300, 200}} {
		if got := e.a.PagePx(c.w, c.h); got.Err != "" || got.PageW != c.pw || got.PageH != c.ph {
			t.Errorf("%d×%d: %+v, ждали %v × %v", c.w, c.h, got, c.pw, c.ph)
		}
	}
	for _, c := range []struct {
		w, h int
		want string
	}{{90000, 2398, "Разворот 90000 px при 300 dpi — страница больше 1000 мм"}, {0, 0, "больше нуля"}, {-7181, 2398, "больше нуля"}, {7181, 100, "меньше 50 мм"}} {
		if got := e.a.PagePx(c.w, c.h); got.Err != c.want && !strings.Contains(got.Err, c.want) || got.PageW != 0 || got.PageH != 0 {
			t.Errorf("%d×%d: %+v, ждали ошибку «%s»", c.w, c.h, got, c.want)
		}
	}
	if _, v := e.a.editor().Snapshot(); v.Rev != v0.Rev {
		t.Errorf("PagePx правит проект: rev %d → %d", v0.Rev, v.Rev)
	}
}

// TestPxExportSizes (123, критерий приёмки): ввод px 7180×2397 разворота и обложки на 1 px больше
// шаблона → файлы выгрузки для Фабрики ровно этих размеров, без запрета «не формат» (страница и
// сторонка в допуске). Один разворот — корешок 8 мм, поэтому лист 7819×2836, а не 8031 (то же при
// 40 разворотах — TestSetCoverPx).
func TestPxExportSizes(t *testing.T) {
	if testing.Short() {
		t.Skip("рендер файлов 7180×2397 и 8031×2836 — не быстрый уровень")
	}
	e, _ := printEnv(t, 1, 304, 203, "fabrika-photo")
	pg := e.a.PagePx(7180, 2397)
	p, _ := e.a.editor().Snapshot()
	st := p.Settings
	st.PageW, st.PageH = pg.PageW, pg.PageH
	e.a.editor().SetSettings(st)
	if _, err := e.a.SetCoverPx(7819, 2836); err != nil {
		t.Fatal(err)
	}
	if msg := e.export(t, ExportOptions{Print: "fabrika"}); msg != "" || len(e.asked) != 0 {
		t.Fatalf("выгрузка: %q, вопросы %q", msg, e.asked)
	}
	bk := filepath.Join(e.dir, "Альбом")
	if got := pxOf(t, filepath.Join(bk, "1-01.jpg")); got != [2]int{7180, 2397} {
		t.Errorf("разворот %v, ждали 7180×2397", got)
	}
	if got := pxOf(t, filepath.Join(bk, "1-00.jpg")); got != [2]int{7819, 2836} {
		t.Errorf("обложка %v, ждали 7819×2836", got)
	}
}

// TestPxExportSizes40 (123, критерий приёмки): 40 разворотов — корешок 26, обложка 8031×2836 и
// разворот 7180×2397 выходят файлами ровно этих размеров (в TestPxExportSizes один разворот —
// лист короче), крайние развороты 1-01 и 1-40 тоже.
func TestPxExportSizes40(t *testing.T) {
	if testing.Short() {
		t.Skip("рендер 41 файла 7180×2397 — не быстрый уровень")
	}
	e, _ := printEnv(t, 40, 304, 203, "fabrika-photo")
	pg := e.a.PagePx(7180, 2397)
	p, _ := e.a.editor().Snapshot()
	st := p.Settings
	st.PageW, st.PageH = pg.PageW, pg.PageH
	e.a.editor().SetSettings(st)
	if _, err := e.a.SetCoverPx(8031, 2836); err != nil {
		t.Fatal(err)
	}
	if msg := e.export(t, ExportOptions{Print: "fabrika"}); msg != "" || len(e.asked) != 0 {
		t.Fatalf("выгрузка: %q, вопросы %q", msg, e.asked)
	}
	bk := filepath.Join(e.dir, "Альбом")
	if got := pxOf(t, filepath.Join(bk, "1-00.jpg")); got != [2]int{8031, 2836} {
		t.Errorf("обложка %v, ждали 8031×2836", got)
	}
	for _, f := range []string{"1-01.jpg", "1-40.jpg"} {
		if got := pxOf(t, filepath.Join(bk, f)); got != [2]int{7180, 2397} {
			t.Errorf("%s: %v, ждали 7180×2397", f, got)
		}
	}
}
