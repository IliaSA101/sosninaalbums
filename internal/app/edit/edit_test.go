package edit

import (
	"slices"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// newMethods — Methods поверх проекта p без библиотеки фото; views — виды, разосланные Changed
// (по порядку), как событие view у App.
func newMethods(t testing.TB, p *model.Project) (m *Methods, views func() []model.View) {
	t.Helper()
	ed, err := album.NewEditor(p, "")
	if err != nil {
		t.Fatal(err)
	}
	var (
		mu   sync.Mutex
		seen []model.View
	)
	m = New(Host{
		Editor: func() *album.Editor { return ed },
		Changed: func(v model.View) model.View {
			mu.Lock()
			defer mu.Unlock()
			seen = append(seen, v)
			return v
		},
		Notice: func(string) {},
		Infos:  func([]string) []model.PhotoInfo { return nil },
		InfosOrRefs: func(ids []string) []model.PhotoInfo { // ленты нет: сведения из проекта, как у App
			var out []model.PhotoInfo
			for _, id := range ids {
				if r, ok := ed.Ref(id); ok {
					out = append(out, model.PhotoInfo{ID: id, Path: r.Path, W: r.W, H: r.H, Size: r.Size})
				}
			}
			return out
		},
		Templates: func() []model.Template { return nil },
	})
	return m, func() []model.View {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(seen)
	}
}

// undo и redo — как App.Undo и App.Redo (без перечитывания папок): шаг и рассылка вида.
func (m *Methods) undo() model.View { return m.changed(m.editor().Undo()) }
func (m *Methods) redo() model.View { return m.changed(m.editor().Redo()) }

// spreadsMethods — альбом без папок: развороты s1, s2, … с заданным составом фото.
func spreadsMethods(t *testing.T, spreads ...[]string) (m *Methods, views func() []model.View) {
	t.Helper()
	p := model.New()
	for i, ids := range spreads {
		p.Spreads = append(p.Spreads, &model.Spread{ID: "s" + string(rune('1'+i)), Photos: slices.Clone(ids)})
		for _, id := range ids {
			p.Photos[id] = model.PhotoRef{Path: `C:\` + id + ".jpg", W: 300, H: 200}
		}
	}
	return newMethods(t, p)
}

// gridMethods — разворот-сетка s1 [a, пусто] и «Авто» s2 [b].
func gridMethods(t *testing.T) *Methods {
	t.Helper()
	p := model.New()
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{"a"}, Grid: []*model.Place{
			{ID: "00000000000a", X: 12, Y: 12, W: 100, H: 100, Photo: "a"},
			{ID: "00000000000b", X: 312, Y: 12, W: 100, H: 100}}},
		{ID: "s2", Photos: []string{"b"}},
	}
	p.Photos["a"] = model.PhotoRef{Path: `C:\a.jpg`, W: 300, H: 200}
	p.Photos["b"] = model.PhotoRef{Path: `C:\b.jpg`, W: 300, H: 200}
	m, _ := newMethods(t, p)
	return m
}

// photosOf — составы разворотов вида по порядку.
func photosOf(v model.View) [][]string {
	out := make([][]string, len(v.Spreads))
	for i, s := range v.Spreads {
		out[i] = s.Photos
	}
	return out
}

func equalComposition(got [][]string, want ...[]string) bool {
	return slices.EqualFunc(got, want, slices.Equal[[]string])
}
