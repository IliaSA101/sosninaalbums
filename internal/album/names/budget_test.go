package names_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Автоимена у предела знаков (0.14 decisions 31): имена — удобство, фото кладут намеренно —
// бросок, «Поставить на места» и «Заполнить» проходят без имён тем же одним шагом.

// textSheets — листы без фото с блоками текста на n знаков (по MaxTextRunes, по MaxTexts на лист).
func textSheets(n int) []*model.Spread {
	var out []*model.Spread
	for n > 0 {
		s := &model.Spread{ID: model.NewID()}
		for n > 0 && len(s.Texts) < model.MaxTexts {
			k := min(n, model.MaxTextRunes)
			s.Texts = append(s.Texts, &model.TextBlock{ID: model.NewID(), Auto: true, Text: strings.Repeat("я", k), Style: model.PlainStyle()})
			n -= k
		}
		out = append(out, s)
	}
	return out
}

// brim — запас знаков проекта занят до последнего знака: длина добора ищется на копии тем же
// счётом, что у правки (CheckTotals), затем ставится одной правкой.
func brim(t *testing.T, e *album.Editor) {
	t.Helper()
	fits := func(n int) bool {
		p := edtest.P(e).Clone()
		p.Spreads = append(p.Spreads, textSheets(n)...)
		return p.CheckTotals(p) == nil
	}
	lo, hi := 0, model.MaxTotalRunes // fits(lo) и !fits(hi+1)
	for lo < hi {
		if mid := (lo + hi + 1) / 2; fits(mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	if lo == 0 || fits(lo+1) {
		t.Fatalf("добор %d знаков: запас не исчерпан", lo)
	}
	edtest.Change(e, func(p *model.Project) { p.Spreads = append(p.Spreads, textSheets(lo)...) })
	if fits(1) {
		t.Fatal("после добора остался запас")
	}
}

// TestAutoNamesOverBudget — у проекта без запаса бросок двух «человеческих» файлов, «Поставить
// на места» и «Заполнить» проходят без имён (named пуст), каждый — один шаг отмены; фото
// переживают Save → Load; Ctrl+Z снимает шаг целиком.
func TestAutoNamesOverBudget(t *testing.T) {
	e := capsEd(t, 6, nameTpl)
	brim(t, e)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	_, out, err := e.DropPhotos("s1", "", []model.PhotoInfo{named("a", "иванов_петя.jpg"), named("b", "петрова_анна.jpg")}, -1, "")
	if err != nil || out.Named != nil || out.Ask {
		t.Fatalf("бросок у предела: %v, %+v", err, out)
	}
	if edtest.Rev(e) != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("бросок: шагов %d, rev +%d — нужен один шаг", edtest.UndoLen(e)-undo, edtest.Rev(e)-rev)
	}
	pl := ""
	for _, g := range edtest.P(e).Spreads[0].Grid {
		if g.Photo == "" {
			pl = g.ID
			break
		}
	}
	_, left, nm, err := e.PlacePhotos("s1", pl, []model.PhotoInfo{named("c", "сидоров_олег.jpg")})
	if err != nil || left != 0 || nm != nil || edtest.UndoLen(e) != undo+2 {
		t.Fatalf("PlacePhotos: %v, left %d, named %v, шагов %d", err, left, nm, edtest.UndoLen(e)-undo)
	}
	_, left, nm = e.FillPlaces([]model.PhotoInfo{named("d", "смирнова_ольга.jpg"), named("f", "морозов_иван.jpg")}, "s1")
	if left != 0 || nm != nil || edtest.UndoLen(e) != undo+3 {
		t.Fatalf("FillPlaces: left %d, named %v, шагов %d", left, nm, edtest.UndoLen(e)-undo)
	}
	got := edtest.P(e).Spreads[0].Photos
	for _, id := range []string{"a", "b", "c", "d", "f"} {
		if !slices.Contains(got, id) || who(e, id) != "" {
			t.Fatalf("%s: на листе %v, имя %q", id, got, who(e, id))
		}
	}
	q := edtest.SaveLoad(t, e)
	if n := len(q.Spreads[0].Photos); n != 5 {
		t.Fatalf("после Save → Load фото на листе %d, ждали 5", n)
	}
	for _, r := range q.Photos {
		if r.Person != nil {
			t.Fatalf("после Save → Load имя у %s: %+v", r.Path, r.Person)
		}
	}
	e.Undo()
	if slices.Contains(edtest.P(e).Spreads[0].Photos, "d") || !slices.Contains(edtest.P(e).Spreads[0].Photos, "c") {
		t.Fatalf("Ctrl+Z после «Заполнить»: %v", edtest.P(e).Spreads[0].Photos)
	}
}

// TestAutoNamesRefused — проект уже за пределом знаков (состояние мимо правок: старый файл,
// обход): повтор без автоимён тоже отказ. PlacePhotos возвращает ErrTextBudget, named пуст,
// left = 0, правка не применена (ни шага, ни rev); Save → Load и Ctrl+Z состояние не меняют.
func TestAutoNamesRefused(t *testing.T) {
	e := capsEd(t, 6, nameTpl)
	brim(t, e)
	p := edtest.P(e)
	p.Spreads = append(p.Spreads, textSheets(1)...) // живой проект: на знак больше запаса
	pl := p.Spreads[0].Grid[0].ID
	ph := []model.PhotoInfo{named("a", "иванов_петя.jpg"), named("b", "Иванов Петя.jpg")}
	rev, undo, photos := edtest.Rev(e), edtest.UndoLen(e), len(p.Photos)
	_, left, nm, err := e.PlacePhotos("s1", pl, ph)
	if !errors.Is(err, model.ErrTextBudget) || left != 0 || nm != nil {
		t.Fatalf("PlacePhotos за пределом: err %v, left %d, named %v", err, left, nm)
	}
	if edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
		t.Fatalf("отказ: rev +%d, шагов +%d — правка не должна применяться", edtest.Rev(e)-rev, edtest.UndoLen(e)-undo)
	}
	if got := edtest.P(e); len(got.Spreads[0].Photos) != 0 || len(got.Photos) != photos {
		t.Fatalf("после отказа: фото на листе %v, в проекте %d (было %d)", got.Spreads[0].Photos, len(got.Photos), photos)
	}
	e.Undo() // снимается добор знаков (brim), а не отказанная правка
	if edtest.UndoLen(e) != undo-1 {
		t.Fatalf("Ctrl+Z: шагов %d, ждали %d", edtest.UndoLen(e), undo-1)
	}
}
