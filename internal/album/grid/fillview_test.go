package grid_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// photoIDs — n новых фото f1…fn: ни одно не в альбоме.
func photoIDs(n int) []model.PhotoInfo {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("f%d", i+1)
	}
	return edtest.Photos(ids...)
}

// Число в виде (FillFrom, CoverFill) — столько мест реально займёт FillPlaces с этого листа:
// при фото ≥ N остаток left = фото − N, при меньшем числе фото — 0 и ровно столько фото посажено.
func TestFillViewCountMatchesFill(t *testing.T) {
	for _, from := range []string{model.CoverID, "s1", "s2"} {
		e := fillEd(t)
		v := e.View()
		n := v.CoverFill
		if from != model.CoverID {
			i, _ := edtest.P(e).Find(from)
			n = v.FillFrom[i]
		}
		if n == 0 {
			t.Fatalf("%s: число 0 — тест ничего не проверяет", from)
		}
		for _, photos := range []int{0, 1, n - 1, n, n + 1, n + 5} {
			e := fillEd(t)
			empty := e.View().EmptyPlaces
			_, left, _ := e.FillPlaces(photoIDs(photos), from)
			if want := max(photos-n, 0); left != want {
				t.Errorf("с %s, %d фото, число %d: left %d, ждали %d", from, photos, n, left, want)
			}
			// Посажено = min(фото, число): пустых мест убыло ровно на это (вид пересчитан).
			if got := empty - e.View().EmptyPlaces; got != min(photos, n) {
				t.Errorf("с %s, %d фото: посажено %d, ждали %d", from, photos, got, min(photos, n))
			}
		}
	}
}

// Полный круг: после посадки с обложки числа в виде нулевые, холостая правка вид не трогает.
func TestFillViewAfterFill(t *testing.T) {
	e := fillEd(t)
	v := e.View()
	if len(v.FillFrom) != 2 || v.CoverFill < v.FillFrom[0] || v.FillFrom[0] < v.FillFrom[1] {
		t.Fatalf("числа не убывают: обложка %d, %v", v.CoverFill, v.FillFrom)
	}
	e.FillPlaces(photoIDs(v.CoverFill), model.CoverID)
	v = e.View()
	if v.CoverFill != 0 || v.FillFrom[0] != 0 || v.FillFrom[1] != 0 || v.EmptyPlaces != 0 {
		t.Errorf("после посадки: обложка %d, %v, пустых %d", v.CoverFill, v.FillFrom, v.EmptyPlaces)
	}
	// Отмена возвращает числа.
	e.Undo()
	if w := e.View(); w.CoverFill == 0 || w.FillFrom[0] == 0 {
		t.Errorf("после Undo: обложка %d, %v", w.CoverFill, w.FillFrom)
	}
}

// Без обложки CoverFill — 0; число развороту — из его же мест; несколько развороту сразу.
func TestFillViewNoCover(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(12, 12, 100, 100, ""), edtest.Gp(150, 12, 100, 100, "")}, []string{"x"})
	v := e.View()
	if v.CoverFill != 0 || len(v.FillFrom) != 2 || v.FillFrom[0] != v.FillFrom[1]+2 {
		t.Errorf("без обложки: %d %v", v.CoverFill, v.FillFrom)
	}
}

// В JSON FillFrom — всегда список: нет разворотов и вид печати дают [], а не null (UI читает .length).
func TestFillViewJSONList(t *testing.T) {
	p := model.New()
	e, err := album.NewEditor(p, "")
	if err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]model.View{"пустой": e.View(), "печать": printOf(e)} {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"fillFrom":[]`) || !strings.Contains(string(data), `"coverFill":0`) {
			t.Errorf("%s: нет fillFrom:[] / coverFill:0 в JSON", name)
		}
	}
	// Вид печати альбома с пустыми местами — тоже без чисел.
	if pv := printOf(fillEd(t)); len(pv.FillFrom) != 0 || pv.CoverFill != 0 {
		t.Errorf("печать: %v %d", pv.FillFrom, pv.CoverFill)
	}
}

func printOf(e *album.Editor) model.View {
	_, v := e.Snapshot()
	return v
}

// Дельта (075) несёт шапку целиком: смена только чисел «Заполнить» — не «холостая» для вида.
func TestFillViewDeltaHead(t *testing.T) {
	e := fillEd(t)
	a := e.View()
	b := a
	b.FillFrom = append([]int{}, a.FillFrom...)
	b.FillFrom[0]++
	if model.SameView(a, b) {
		t.Error("смена FillFrom осталась незамеченной дельтой")
	}
	c := a
	c.CoverFill++
	if model.SameView(a, c) {
		t.Error("смена CoverFill осталась незамеченной дельтой")
	}
	if !model.SameView(a, e.View()) {
		t.Error("два вида одного состояния различаются")
	}
}
