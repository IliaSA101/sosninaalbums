package export

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// BlankSpreads — номера (с 1) разворотов из плана, где печатать нечего: ни фото, ни текстов,
// ни подписей, ни фото на фоне, ни узора, ни стикера, ни мест, ни своего цвета страниц. Название
// не печатается, поэтому разворот с одним названием тоже пустой (Spread.empty тут не подходит).
// Фон, узор и стикер с Fade ≥ 1, узор и стикер неизвестного генератора не печатаются — не считаются.
func BlankSpreads(v model.View, pl *Plan) []int {
	var out []int
	for i, s := range v.Spreads {
		if pl.mask[i+1] == 0 {
			continue
		}
		if len(s.Photos) == 0 && len(s.Texts) == 0 && len(s.Captions) == 0 && !shownBackdrop(s) && !shownPattern(s) &&
			!shownSticker(s) && len(s.Places) == 0 && s.BgSet == [2]string{} {
			out = append(out, i+1)
		}
	}
	return out
}

// shownBackdrop — на развороте есть фон, который попадёт в печать (Fade < 1).
func shownBackdrop(s model.SpreadView) bool {
	for _, b := range s.Backdrops {
		if b.Fade < 1 {
			return true
		}
	}
	return false
}

// shownPattern — на листе есть узор, который попадёт в печать (048): видимый и известный.
func shownPattern(s model.SpreadView) bool {
	for _, pv := range s.Patterns {
		if pv.Known && pv.Pat.Fade < 1 {
			return true
		}
	}
	return false
}

// shownSticker — на листе есть стикер, который попадёт в печать (048): видимый, свой или
// известного генератора.
func shownSticker(s model.SpreadView) bool {
	for _, st := range s.Stickers {
		if st.Known && st.Fade < 1 {
			return true
		}
	}
	return false
}

// BlankWarning — текст вопроса перед экспортом о пустых разворотах; "" — пустых нет.
// white — фон альбома белый. Номеров не больше 10, остальные — «…и ещё K».
func BlankWarning(nums []int, white bool) string {
	if len(nums) == 0 {
		return ""
	}
	list := numList(nums)
	head, tail := "Развороты %s пустые — выгрузить %s?", "белыми"
	if len(nums) == 1 {
		head, tail = "Разворот %s пустой — выгрузить %s?", "белым"
	}
	if !white {
		tail = "с фоном альбома"
	}
	return fmt.Sprintf(head, list, tail)
}

// numList — номера через запятую, не больше 10, остальные — «…и ещё K».
func numList(nums []int) string {
	const maxShown = 10
	parts := make([]string, 0, maxShown)
	for _, n := range nums[:min(len(nums), maxShown)] {
		parts = append(parts, strconv.Itoa(n))
	}
	list := strings.Join(parts, ", ")
	if len(nums) > maxShown {
		list += fmt.Sprintf(" …и ещё %d", len(nums)-maxShown)
	}
	return list
}

// PlanFields — пустые поля (по алфавиту) и фото без имени на листах, которые пишет план:
// развороты из pl.mask и обложка, если она в плане. По всему альбому (View.Unfilled) нельзя —
// «Только обложка» или диапазон спросили бы о листах, которые не выгружаются. При выгрузке
// одной страницы разворота вопрос учитывает поля всего разворота: лишний вопрос, данные целы.
func PlanFields(v model.View, pl *Plan) (unfilled []string, unnamed int) {
	add := func(s model.SpreadView) {
		unfilled = append(unfilled, s.Unfilled...)
		unnamed += s.Unnamed
	}
	if pl.Cover != nil && v.Cover != nil {
		add(v.Cover.Sheet)
	}
	for i, s := range v.Spreads {
		if pl.mask[i+1] != 0 {
			add(s)
		}
	}
	slices.Sort(unfilled)
	return slices.Compact(unfilled), unnamed
}

// FieldsWarning — вопрос перед экспортом о пустых полях подписей (030): unfilled —
// имена полей, unnamed — фото без имени (оба — из PlanFields); "" — всё заполнено.
func FieldsWarning(unfilled []string, unnamed int) string {
	var parts []string
	if len(unfilled) > 0 {
		parts = append(parts, "Не заполнены поля: "+strings.Join(unfilled, ", "))
	}
	if unnamed > 0 {
		head := "фото без имени"
		if len(parts) == 0 {
			head = "Фото без имени"
		}
		parts = append(parts, fmt.Sprintf("%s: %d", head, unnamed))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ") + ". В выгрузке они будут пустыми. Продолжить?"
}
