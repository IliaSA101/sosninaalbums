package view_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/view"
)

// Вхождения фото для карточки (084 design §3): порядок листов и видов, подписи, только чтение.

// labels — подписи вхождений и индексы листов одной строкой на вхождение (остальные поля
// проверяет TestPhotoUsesPick).
func labels(uses []model.PhotoUse) []string {
	out := make([]string, len(uses))
	for i, u := range uses {
		out[i] = use(u.Sheet, u.Label)
	}
	return out
}

func use(sheet int, label string) string {
	b, _ := json.Marshal(model.PhotoUse{Sheet: sheet, Label: label})
	return string(b)
}

// usesEd — обложка (лист 0) и два разворота: на первом место с фото x и место с фото y, на
// втором — фото z. Фото x ставится ещё фоном обложки, фоном левой страницы и стикером первого
// разворота, рамочкой-картинкой места y и фоном второго разворота.
func usesEd(t *testing.T) *album.Editor {
	t.Helper()
	e := edtest.CoverEd(t, "c", []*model.Place{edtest.Gp(10, 10, 100, 100, "x"), edtest.Gp(150, 10, 100, 100, "y")}, []string{"z"})
	edtest.Change(e, func(p *model.Project) {
		s1, s2, cv := p.Spreads[0], p.Spreads[1], p.Cover.Sheet
		cv.BgPhoto = &model.Backdrop{Photo: "x", W: 100, H: 100}
		s1.BgPhotoL = &model.Backdrop{Photo: "x", W: 100, H: 100}
		s1.Stickers = []*model.Sticker{{ID: "c0ffee0000a1", Photo: "x", X: 10, Y: 10, W: 30, H: 30}}
		s1.Grid[1].Border = &model.Border{Photo: "x"}
		s2.BgPhoto = &model.Backdrop{Photo: "x", W: 100, H: 100}
	})
	return e
}

// Место, два фона разворотов, фон обложки, стикер и рамочка: шесть вхождений по ряду листов
// (обложка первой), внутри листа — место, фон, стикер, рамочка. Соседние фото не путаются.
func TestPhotoUsesKinds(t *testing.T) {
	e := usesEd(t)
	uses, used := e.PhotoUses("x")
	want := []string{
		use(0, "Обложка — фон"),
		use(1, "Разворот 1 — место"),
		use(1, "Разворот 1 — фон"),
		use(1, "Разворот 1 — стикер"),
		use(1, "Разворот 1 — рамочка"),
		use(2, "Разворот 2 — фон"),
	}
	if got := labels(uses); !reflect.DeepEqual(got, want) {
		t.Fatalf("вхождения x:\n%v\nждали\n%v", got, want)
	}
	if used != "Использовано 6 раз" {
		t.Fatalf("строка: %q", used)
	}
	for id, w := range map[string][]string{"y": {use(1, "Разворот 1 — место")}, "z": {use(2, "Разворот 2 — место")}, "c": {use(0, "Обложка — место")}} {
		if got, _ := e.PhotoUses(id); !reflect.DeepEqual(labels(got), w) {
			t.Errorf("вхождения %s: %v, ждали %v", id, labels(got), w)
		}
	}
}

// PhotoUses — только чтение: ни шага отмены, ни rev, ни dirty.
func TestPhotoUsesReadOnly(t *testing.T) {
	e := usesEd(t)
	rev, undo, dirty := edtest.Rev(e), edtest.UndoLen(e), edtest.Dirty(e)
	e.PhotoUses("x")
	e.PhotoUses("нет")
	if edtest.Rev(e) != rev || edtest.UndoLen(e) != undo || edtest.Dirty(e) != dirty {
		t.Fatalf("чтение изменило редактор: rev %d→%d, отмена %d→%d", rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}
}

// Правка меняет вхождения сразу, отмена возвращает прежние (сц. 6).
func TestPhotoUsesAfterEditUndo(t *testing.T) {
	e, _, s2 := edtest.NewTwo(t)
	if _, used := e.PhotoUses("a"); used != "Использовано 1 раз" {
		t.Fatalf("до правки: %q", used)
	}
	edtest.SetBg(t, e, s2, model.ScopeSpread, "a")
	uses, used := e.PhotoUses("a")
	if used != "Использовано 2 раза" || !reflect.DeepEqual(labels(uses), []string{use(0, "Разворот 1 — место"), use(1, "Разворот 2 — фон")}) {
		t.Fatalf("после фона: %q %v", used, labels(uses))
	}
	e.Undo()
	if _, used := e.PhotoUses("a"); used != "Использовано 1 раз" {
		t.Fatalf("после отмены: %q", used)
	}
}

// Проект-лист виньетки (031): подписи «Лист N», обложки нет — индекс листа с нуля.
func TestPhotoUsesSheetProject(t *testing.T) {
	e := edtest.Open(t, model.NewSheet(200, 300), "")
	edtest.NewSpread(e, -1, edtest.Photos("a"))
	uses, _ := e.PhotoUses("a")
	if got, want := labels(uses), []string{use(1, "Лист 2 — место")}; !reflect.DeepEqual(got, want) {
		t.Fatalf("лист: %v, ждали %v", got, want)
	}
}

// Не стоит нигде (и пустой ID) — пустой срез, не nil: в JSON «[]», а не null.
func TestPhotoUsesNone(t *testing.T) {
	e := usesEd(t)
	for _, id := range []string{"нет", ""} {
		uses, used := e.PhotoUses(id)
		if uses == nil || len(uses) != 0 || used != "В альбоме не использовано" {
			t.Fatalf("%q: %#v %q", id, uses, used)
		}
		if b, _ := json.Marshal(uses); string(b) != "[]" {
			t.Fatalf("%q: JSON %s", id, b)
		}
	}
	empty := edtest.Open(t, model.New(), "")
	if uses, _ := empty.PhotoUses("x"); uses == nil || len(uses) != 0 {
		t.Fatalf("пустой альбом: %#v", uses)
	}
}

func TestUsedText(t *testing.T) {
	cases := map[int]string{
		-1:  "В альбоме не использовано",
		0:   "В альбоме не использовано",
		1:   "Использовано 1 раз",
		2:   "Использовано 2 раза",
		4:   "Использовано 4 раза",
		5:   "Использовано 5 раз",
		11:  "Использовано 11 раз",
		12:  "Использовано 12 раз",
		14:  "Использовано 14 раз",
		21:  "Использовано 21 раз",
		22:  "Использовано 22 раза",
		112: "Использовано 112 раз",
		124: "Использовано 124 раза",
	}
	for n, want := range cases {
		if got := view.UsedText(n); got != want {
			t.Errorf("%d: %q, ждали %q", n, got, want)
		}
	}
}
