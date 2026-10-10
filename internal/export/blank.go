package export

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// BlankSpreads — номера (с 1) разворотов из плана, где печатать нечего: ни фото, ни текстов,
// ни подписей, ни фото на фоне, ни мест, ни своего цвета страниц. Название не печатается,
// поэтому разворот с одним названием тоже пустой (Spread.empty тут не подходит).
// Фон с Fade ≥ 1 не печатается — не считается.
func BlankSpreads(v model.View, pl *Plan) []int {
	var out []int
	for i, s := range v.Spreads {
		if pl.mask[i+1] == 0 {
			continue
		}
		if len(s.Photos) == 0 && len(s.Texts) == 0 && len(s.Captions) == 0 && !shownBackdrop(s) &&
			len(s.Places) == 0 && s.BgSet == [2]string{} {
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

// BlankWarning — текст вопроса перед экспортом о пустых разворотах; "" — пустых нет.
// white — фон альбома белый. Номеров не больше 10, остальные — «…и ещё K».
func BlankWarning(nums []int, white bool) string {
	if len(nums) == 0 {
		return ""
	}
	const maxShown = 10
	parts := make([]string, 0, maxShown)
	for _, n := range nums[:min(len(nums), maxShown)] {
		parts = append(parts, strconv.Itoa(n))
	}
	list := strings.Join(parts, ", ")
	if len(nums) > maxShown {
		list += fmt.Sprintf(" …и ещё %d", len(nums)-maxShown)
	}
	head, tail := "Развороты %s пустые — выгрузить %s?", "белыми"
	if len(nums) == 1 {
		head, tail = "Разворот %s пустой — выгрузить %s?", "белым"
	}
	if !white {
		tail = "с фоном альбома"
	}
	return fmt.Sprintf(head, list, tail)
}
