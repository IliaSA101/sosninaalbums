package looks

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// lookOfsTpl — макет: обложка (лицевая 2 места, задняя 3), шаги: 4 места, виньетка (места листа
// строит виньетка, 5 мест шаблона не считаются), страница с 3 местами, повтор 2, финал 1.
func lookOfsTpl() model.AlbumTpl {
	tpl := func(pages, n int) model.SpreadTpl {
		return model.SpreadTpl{V: model.TplV, Pages: pages, Ratio: 1, Places: make([]model.Box, n)}
	}
	return model.AlbumTpl{
		V: model.LookV,
		Cover: &model.CoverTpl{
			Front: &model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Places: make([]model.Box, 2)},
			Back:  &model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Places: make([]model.Box, 3)},
		},
		Steps: []model.LookStep{
			{Tpl: tpl(2, 4)},
			{Role: model.RoleVignette, Tpl: tpl(2, 5), Vig: &model.LookVig{Tpl: "x", Teachers: 1, Pupils: 3}},
			{Tpl: tpl(1, 3)},
			{Role: model.RoleRepeat, Tpl: tpl(2, 2)},
			{Tpl: tpl(2, 1)},
		},
	}
}

// Сдвиг шага: лицевая обложки с 0, задняя после неё, шаги — после мест предыдущих; виньетка
// даёт 0 мест; у страницы места одной страницы (не ×2 LookPlaces); шаг-повтор считается один раз.
func TestLookStepOffsets(t *testing.T) {
	l := lookOfsTpl()
	if got := lookBackShift(l.Cover); got != 2 {
		t.Errorf("задняя после лицевой: сдвиг %d, ждали 2", got)
	}
	for i, want := range map[int]int{
		0:  0,      // обложка: лицевая с начала
		1:  2 + 3,  // после обеих сторон обложки
		2:  5 + 4,  // после шага 1
		3:  9 + 0,  // после виньетки: она ничего не заняла
		4:  9 + 3,  // после страницы: 3, а не 6
		5:  12 + 2, // после повтора
		99: 14 + 1, // за последним шагом — сумма всех
		-3: 0,      // мусорный индекс
	} {
		if got := lookOffset(l, i); got != want {
			t.Errorf("шаг %d: сдвиг %d, ждали %d", i, got, want)
		}
	}
	if got, want := lookSeats(l.Steps[2]), model.LookPlaces(l, 2)/2; got != want {
		t.Errorf("страница: %d мест, LookPlaces/2 = %d", got, want)
	}
	if lookSeats(l.Steps[1]) != 0 {
		t.Error("виньетка рисует места шаблона")
	}
	// Без обложки: шаги от нуля. Без шагов и пустой макет — 0, без паники.
	l.Cover = nil
	if got := lookOffset(l, 2); got != 4 {
		t.Errorf("без обложки: %d, ждали 4", got)
	}
	if lookBackShift(nil) != 0 || lookOffset(model.AlbumTpl{}, 5) != 0 {
		t.Error("пустой макет")
	}
	if got := lookBackShift(&model.CoverTpl{Back: &model.SpreadTpl{Places: make([]model.Box, 4)}}); got != 0 {
		t.Errorf("лицевой нет: задняя с 0, получили %d", got)
	}
}
