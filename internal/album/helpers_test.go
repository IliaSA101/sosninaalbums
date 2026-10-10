package album

import (
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

func ip(v int) *int { return &v }

func setMarks(t *testing.T, e *Editor, ph []model.PhotoInfo, pt model.MarkPatch) model.View {
	t.Helper()
	v, err := e.SetMarks(ph, pt)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// markRows — Marks() в виде, удобном для сравнения: nil-поле — -9 / "-".
func markRows(e *Editor) map[string][3]any {
	out := map[string][3]any{}
	for id, m := range e.Marks() {
		r := [3]any{-9, "-", -9}
		if m.Rating != nil {
			r[0] = *m.Rating
		}
		if m.Label != nil {
			r[1] = *m.Label
		}
		if m.Flag != nil {
			r[2] = *m.Flag
		}
		out[id] = r
	}
	return out
}

func wantMarks(t *testing.T, e *Editor, want map[string][3]any) {
	t.Helper()
	if got := markRows(e); !reflect.DeepEqual(got, want) {
		t.Fatalf("отметки %v, ждали %v", got, want)
	}
}

func photo(id string) model.PhotoInfo {
	return model.PhotoInfo{ID: id, Path: "/" + id + ".jpg", W: 300, H: 200}
}

func photos(ids ...string) []model.PhotoInfo {
	out := make([]model.PhotoInfo, len(ids))
	for i, id := range ids {
		out[i] = photo(id)
	}
	return out
}

// Обёртки для тестов, где отказ не ждут: ошибка — сразу фатальная паника.
func newSpread(e *Editor, at int, ph []model.PhotoInfo) model.View {
	v, err := e.NewSpread(at, ph)
	if err != nil {
		panic(err)
	}
	return v
}

// newEditor — копия edtest.Open (внутренний тест edtest не импортирует): отказ — Fatal.
func newEditor(t testing.TB, p *model.Project, path string) *Editor {
	t.Helper()
	e, err := NewEditor(p, path)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// newTwo — проект с двумя разворотами: [a b] и [c].
func newTwo(t *testing.T) (*Editor, string, string) {
	t.Helper()
	e := newEditor(t, model.New(), "")
	newSpread(e, -1, photos("a", "b"))
	v := newSpread(e, -1, photos("c"))
	if len(v.Spreads) != 2 {
		t.Fatalf("развороты: %d", len(v.Spreads))
	}
	return e, v.Spreads[0].ID, v.Spreads[1].ID
}
