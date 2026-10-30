package view_test

import (
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Подписи двух и более мест одного фото на листе различимы номером места (владелец 23.10, QA 0.24).

// labelsOf — подписи вхождений фото id.
func labelsOf(uses []model.PhotoUse) []string {
	got := []string{}
	for _, u := range uses {
		got = append(got, u.Label)
	}
	return got
}

// Копии фото a на разворотах: одно место, два места на одном развороте (первое и третье из
// четырёх), по месту на разных разворотах; фото без вхождений — пустой список.
func TestPhotoUsesLabelsNumbered(t *testing.T) {
	a2, a3 := model.CopyID("a", 2), model.CopyID("a", 3)
	for _, tc := range []struct {
		name   string
		spread [][]string // Photos разворотов 1 и 2
		want   []string
	}{
		{"одно место", [][]string{{"a", "b"}, {"b"}}, []string{"Разворот 1 — место"}},
		{"два места на одном развороте", [][]string{{"a", "b", a2, "c"}, {"b"}}, []string{"Разворот 1 — место 1", "Разворот 1 — место 3"}},
		{"три места, номер — позиция в ряду", [][]string{{"b", "a", a2, a3}, {"b"}}, []string{"Разворот 1 — место 2", "Разворот 1 — место 3", "Разворот 1 — место 4"}},
		{"разные развороты", [][]string{{"a", "b"}, {a2}}, []string{"Разворот 1 — место", "Разворот 2 — место"}},
		{"два и одно", [][]string{{"a", a2}, {a3}}, []string{"Разворот 1 — место 1", "Разворот 1 — место 2", "Разворот 2 — место"}},
		{"нет вхождений", [][]string{{"b"}, {"b"}}, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := usesEd(t)
			edtest.Change(e, func(p *model.Project) {
				for _, id := range []string{"a", "b", "c", a2, a3} {
					p.Photos[id] = p.Photos["z"]
				}
				for i, ph := range tc.spread {
					var grid []*model.Place // Photos в сетке — фото непустых мест
					for j, id := range ph {
						grid = append(grid, edtest.Gp(float64(10+j*60), 10, 50, 50, id))
					}
					p.Spreads[i].Grid, p.Spreads[i].Photos = grid, ph
				}
			})
			uses, _ := e.PhotoUses("a")
			if got := labelsOf(uses); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("подписи: %q, ждали %q", got, tc.want)
			}
		})
	}
}

// Повторы других видов на листе: фон — областью, стикеры — по порядку; одиночные вхождения
// и виды на разных листах остаются прежними.
func TestPhotoUsesLabelsOtherKinds(t *testing.T) {
	e := usesEd(t)
	edtest.Change(e, func(p *model.Project) {
		s1 := p.Spreads[0]
		s1.BgPhotoR = &model.Backdrop{Photo: "x", W: 100, H: 100}
		s1.Stickers = append(s1.Stickers, &model.Sticker{ID: "c0ffee0000a2", Photo: "x", X: 50, Y: 10, W: 30, H: 30})
	})
	uses, _ := e.PhotoUses("x")
	want := []string{"Обложка — фон", "Разворот 1 — место", "Разворот 1 — фон слева", "Разворот 1 — фон справа",
		"Разворот 1 — стикер 1", "Разворот 1 — стикер 2", "Разворот 1 — рамочка", "Разворот 2 — фон"}
	if got := labelsOf(uses); !reflect.DeepEqual(got, want) {
		t.Fatalf("подписи:\n%q\nждали\n%q", got, want)
	}
}
