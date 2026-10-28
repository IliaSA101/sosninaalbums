package cover_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestApplyPrint (123 Б1): кнопка формата типографии и изделие обложки — один шаг отмены; Undo
// возвращает и страницу, и обложку; неизвестное изделие — ошибка без шага; тот же формат и то же
// изделие — холостая правка.
func TestApplyPrint(t *testing.T) {
	e := fabrikaAlbum(t, 200, 203, 3, "fsfera-thin")
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 304, 203
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.ApplyPrint(st, "nope-1"); err == nil {
		t.Fatal("неизвестное изделие принято")
	}
	edtest.Same(t, e, rev, undo, "неизвестное изделие")
	if _, err := e.ApplyPrint(st, "fabrika-photo"); err != nil {
		t.Fatal(err)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d, ждали один", edtest.UndoLen(e)-undo)
	}
	if p := edtest.P(e); p.Settings.PageW != 304 || p.Cover.Spec.Preset != "fabrika-photo" || model.CoverFixText(p) != "" {
		t.Fatalf("после применения: %+v %q", p.Cover.Spec, model.CoverFixText(p))
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.ApplyPrint(st, "fabrika-photo"); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "холостая")
	e.Undo()
	if p := edtest.P(e); p.Settings.PageW != 200 || p.Cover.Spec.Preset != "fsfera-thin" {
		t.Fatalf("Undo не вернул оба: %+v", p.Cover.Spec)
	}
	// Без обложки изделие не мешает: меняется только формат.
	b := fabrikaAlbum(t, 200, 203, 1, "fsfera-thin")
	edtest.Change(b, func(p *model.Project) { p.Cover = nil })
	if _, err := b.ApplyPrint(st, "fabrika-photo"); err != nil || edtest.P(b).Settings.PageW != 304 {
		t.Fatalf("без обложки: %v", err)
	}
}

// TestApplyPrintRoundTrip: после ApplyPrint Save → Load даёт тот же формат и изделие; Redo повторяет
// оба; обложка со своими числами за страницей не идёт, но берёт изделие; пустые настройки не роняют.
func TestApplyPrintRoundTrip(t *testing.T) {
	e := fabrikaAlbum(t, 200, 203, 2, "fsfera-thin")
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 304, 203
	if _, err := e.ApplyPrint(st, "fabrika-photo"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if q.Settings.PageW != 304 || q.Cover == nil || q.Cover.Spec.Preset != "fabrika-photo" || !reflect.DeepEqual(q.Cover.Spec, edtest.P(e).Cover.Spec) {
		t.Fatalf("после Save → Load: %+v %+v", q.Settings, q.Cover)
	}
	want := edtest.P(e).Cover.Spec
	e.Undo()
	e.Redo()
	if p := edtest.P(e); p.Settings.PageW != 304 || !reflect.DeepEqual(p.Cover.Spec, want) {
		t.Fatalf("Redo не повторил оба: %+v", p.Cover.Spec)
	}
	// Свои числа: за страницей не идут (FollowFormat), изделие берут (ApplyPreset).
	c := fabrikaAlbum(t, 200, 203, 1, "fsfera-thin")
	edtest.Change(c, func(p *model.Project) { p.Cover.Spec.Preset, p.Cover.Spec.Wrap = model.CoverCustom, 9 })
	if _, err := c.ApplyPrint(st, "fabrika-photo"); err != nil || edtest.P(c).Cover.Spec.Preset != "fabrika-photo" {
		t.Fatalf("свои числа: %v %+v", err, edtest.P(c).Cover.Spec)
	}
	// Пустые настройки: зажим, без паники; пустое изделие — ошибка без шага.
	rev, undo := edtest.Rev(c), edtest.UndoLen(c)
	if _, err := c.ApplyPrint(model.Settings{}, ""); err == nil {
		t.Error("пустое изделие принято")
	}
	edtest.Same(t, c, rev, undo, "пустое изделие")
}

// TestApplyPrintFollowsBeforePreset: шаблонная сторонка прежнего изделия при смене страницы не
// принимается за «свою» — без FollowFormat перед ApplyPreset Нетпринт унёс бы её как числа
// пользователя.
func TestApplyPrintFollowsBeforePreset(t *testing.T) {
	e := fabrikaAlbum(t, 200, 203, 2, "fsfera-thin")
	if c := edtest.P(e).Cover.Spec; c.W <= 0 {
		t.Fatalf("исходная Фотосфера без сторонки шаблона: %+v", c)
	}
	st := edtest.P(e).Settings
	st.PageW, st.PageH = 300, 305
	if _, err := e.ApplyPrint(st, "netprint-premium"); err != nil {
		t.Fatal(err)
	}
	if c := edtest.P(e).Cover.Spec; c.Preset != "netprint-premium" || c.W != 0 || c.H != 0 {
		t.Fatalf("сторонка шаблона осталась «своей»: %+v", c)
	}
}
