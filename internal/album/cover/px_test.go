package cover_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// pxAlbum — n разворотов по одному фото на странице w×h и обложка preset.
func pxAlbum(t *testing.T, w, h float64, n int, preset string) *album.Editor {
	t.Helper()
	p := model.New()
	p.Settings.PageW, p.Settings.PageH, p.Settings.Safe = w, h, 10
	for i := range n {
		p.Photos[pid(i)] = model.PhotoRef{Path: edtest.Abs(pid(i) + ".jpg"), W: 300, H: 200}
		p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprintf("%012x", i+1), Photos: []string{pid(i)}})
	}
	e := edtest.Open(t, p, "")
	if _, err := e.CreateCover(preset, nil); err != nil {
		t.Fatal(err)
	}
	return e
}

// sheetPx — размер файла 00.jpg обложки: так же, как выгрузка, Px от листа при 300 dpi.
func sheetPx(e *album.Editor) [2]int {
	g := edtest.P(e).CoverGeo()
	return [2]int{model.Px(g.X[7], model.PrintDPI), model.Px(g.Y[3], model.PrintDPI)}
}

// TestSetCoverPx (123 §1.3): «Файл обложки, px» — сторонка из px, лист ровно этих px; изделие
// остаётся в допуске шаблона и становится «Свои размеры» за ним; один шаг отмены; тот же ввод,
// ошибка и обложки нет — без шага; Save → Load держит числа; на разном числе разворотов.
func TestSetCoverPx(t *testing.T) {
	// Фабрика 30×20, 40 разворотов (корешок 26): шаблон 8031×2835 → 310×206, цель загрузчика 8031×2836.
	e := pxAlbum(t, 304, 203, 40, "fabrika-photo")
	if g := edtest.P(e).CoverGeo(); g.Spine != 26 {
		t.Fatalf("корешок %v, ждали 26", g.Spine)
	}
	undo := edtest.UndoLen(e)
	if _, err := e.SetCoverPx(8031, 2836); err != nil {
		t.Fatal(err)
	}
	if c := spec(e); c.Preset != "fabrika-photo" || c.W != 310 || c.H != 206.1 || sheetPx(e) != [2]int{8031, 2836} {
		t.Fatalf("8031×2836: %+v → %v px", c, sheetPx(e))
	}
	if model.CoverFixText(edtest.P(e)) != "" {
		t.Error("на 1 px от шаблона — «Не по шаблону»")
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d → %d, ждали один", undo, edtest.UndoLen(e))
	}
	// Тот же ввод — холостая; шаблон 8031×2835 — сторонка 310×206 ровно.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.SetCoverPx(8031, 2836); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "тот же ввод px")
	if _, err := e.SetCoverPx(8031, 2835); err != nil || spec(e).W != 310 || spec(e).H != 206 {
		t.Fatalf("8031×2835: %v %+v", err, spec(e))
	}

	// Далеко от шаблона — «Свои размеры» с теми же числами; отмена возвращает изделие.
	if _, err := e.SetCoverPx(9000, 3200); err != nil || spec(e).Preset != model.CoverCustom || sheetPx(e) != [2]int{9000, 3200} {
		t.Fatalf("9000×3200: %v %+v → %v px", err, spec(e), sheetPx(e))
	}
	if e.Undo(); spec(e).Preset != "fabrika-photo" || spec(e).W != 310 {
		t.Fatalf("Undo: %+v", spec(e))
	}

	// Ошибки: ничего не меняют, текст называет предел.
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	for _, c := range []struct {
		w, h int
		want string
	}{{0, 2835, "больше нуля"}, {8031, -1, "больше нуля"}, {90000, 2835, "больше 1000 мм"}, {8031, 90000, "больше 1000 мм"}, {100, 100, "меньше 50 мм"}} {
		if _, err := e.SetCoverPx(c.w, c.h); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%d×%d: %v, ждали «%s»", c.w, c.h, err, c.want)
		}
	}
	edtest.Same(t, e, rev, undo, "ошибки ввода")

	// Save → Load: числа и px листа те же.
	want := spec(e)
	q := edtest.SaveLoad(t, e)
	if got := q.Cover.Spec; got.W != want.W || got.H != want.H || got.Preset != want.Preset {
		t.Fatalf("после Save → Load: %+v, ждали %+v", got, want)
	}

	// Нет обложки — ошибка без шага.
	e.DeleteCover()
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.SetCoverPx(8031, 2835); err == nil {
		t.Error("без обложки — ждали ошибку")
	}
	edtest.Same(t, e, rev, undo, "без обложки")
}

// TestSetCoverPxMany: разные изделия, страницы и числа разворотов — лист обложки всегда ровно
// введённые px (корешок берётся на текущем числе разворотов).
func TestSetCoverPxMany(t *testing.T) {
	for _, c := range []struct {
		name   string
		w, h   float64
		n      int
		preset string
	}{
		{"Фабрика 20×20, 2 разворота", 200, 203, 2, "fabrika-photo"},
		{"Фабрика 30×30, 12", 300, 305, 12, "fabrika-photo-1"},
		{"Фотосфера 30×30, 6", 281, 284, 6, "fsfera-thin"},
		{"Нетпринт, 20", 300, 305, 20, "netprint-premium"},
		{"общая страница, свои", 300, 200, 5, model.CoverCustom},
	} {
		e := pxAlbum(t, c.w, c.h, c.n, c.preset)
		for _, px := range [][2]int{{6000, 2800}, {8031, 2835}, {8032, 2836}, {7001, 3999}} {
			if _, err := e.SetCoverPx(px[0], px[1]); err != nil {
				t.Fatalf("%s %v: %v", c.name, px, err)
			}
			if got := sheetPx(e); got != px {
				t.Fatalf("%s: введено %v px, лист %v", c.name, px, got)
			}
		}
	}
}

// offByPx — Фотосфера 30×30 (страница 281×284, шаблон 286×286) с обложкой, у которой лист на 1 px
// меньше шаблона по ширине (dw = -1) и/или высоте (dh = -1): сторонка 285,95 — в допуске, изделие
// остаётся (123 §1.3).
func offByPx(t *testing.T, dw, dh int) *album.Editor {
	t.Helper()
	e := coverAlbum(t, 2)
	tpl := sheetPx(e)
	if _, err := e.SetCoverPx(tpl[0]+dw, tpl[1]+dh); err != nil {
		t.Fatal(err)
	}
	c := spec(e)
	if c.Preset != model.CoverDefault || (c.W == 286 && c.H == 286) || !model.SameMm(c.W, 286) || !model.SameMm(c.H, 286) {
		t.Fatalf("ввод %+v: %+v, ждали сторонку рядом с 286 и то же изделие", [2]int{dw, dh}, c)
	}
	return e
}

// TestApplyPresetAfterPx: «Типография» на обложке, введённой по px (сторонка в допуске шаблона),
// считает её шаблонной — Нетпринт берёт сторонку страницы (0), а не уносит 285,95 как «свою».
// Один шаг отмены, Undo возвращает; повтор — холостая; Save → Load держит страницу.
func TestApplyPresetAfterPx(t *testing.T) {
	for _, c := range []struct {
		name   string
		dw, dh int
	}{{"ширина на 1 px", -1, 0}, {"высота на 1 px", 0, -1}, {"обе", -1, -1}} {
		t.Run(c.name, func(t *testing.T) {
			e := offByPx(t, c.dw, c.dh)
			before := spec(e)
			undo := edtest.UndoLen(e)
			if _, err := e.ApplyCoverPreset("netprint-premium"); err != nil {
				t.Fatal(err)
			}
			if got := spec(e); got.Preset != "netprint-premium" || got.W != 0 || got.H != 0 {
				t.Fatalf("Нетпринт после px: %+v, ждали сторонку 0×0", got)
			}
			if edtest.UndoLen(e) != undo+1 {
				t.Fatalf("шагов отмены %d, ждали один", edtest.UndoLen(e)-undo)
			}
			rev, undo := edtest.Rev(e), edtest.UndoLen(e)
			if _, err := e.ApplyCoverPreset("netprint-premium"); err != nil {
				t.Fatal(err)
			}
			edtest.Same(t, e, rev, undo, "повтор пресета")
			if q := edtest.SaveLoad(t, e); q.Cover.Spec.W != 0 || q.Cover.Spec.H != 0 {
				t.Fatalf("Save → Load: %+v", q.Cover.Spec)
			}
			if e.Undo(); !reflect.DeepEqual(spec(e), before) {
				t.Fatalf("Undo: %+v, было %+v", spec(e), before)
			}
		})
	}
}

// TestFollowFormatAfterPx: обложка на 1 px от шаблона (сторонка в допуске) при смене формата
// Фотосферы переходит на шаблон нового формата, как точная; один шаг, Undo возвращает.
func TestFollowFormatAfterPx(t *testing.T) {
	for _, c := range []struct {
		name   string
		dw, dh int
	}{{"ширина на 1 px", -1, 0}, {"высота на 1 px", 0, -1}} {
		t.Run(c.name, func(t *testing.T) {
			e := offByPx(t, c.dw, c.dh)
			before := spec(e)
			undo := edtest.UndoLen(e)
			page(e, 201, 203) // 20×20: шаблон 206×206
			if got := spec(e); got.Preset != before.Preset || got.W != 206 || got.H != 206 {
				t.Fatalf("после смены формата: %+v, ждали шаблон 206×206", got)
			}
			if edtest.UndoLen(e) != undo+1 {
				t.Fatalf("шагов отмены %d, ждали один", edtest.UndoLen(e)-undo)
			}
			if e.Undo(); !reflect.DeepEqual(spec(e), before) {
				t.Fatalf("Undo: %+v, было %+v", spec(e), before)
			}
		})
	}
}
