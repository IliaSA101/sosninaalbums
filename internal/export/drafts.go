package export

import (
	"fmt"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// printable — вид листа для печати: без текстов-заготовок (131, 0.27-131-5: заготовка не
// печатается). Копия: Texts исходного вида не меняется, он живёт в снимке и у холста.
// Плитка шаблона и пипетка рисуют заготовки — они берут fromSheet напрямую.
func printable(s model.SpreadView) model.SpreadView {
	if !slices.ContainsFunc(s.Texts, func(t model.TextView) bool { return t.Draft }) {
		return s
	}
	s.Texts = slices.DeleteFunc(slices.Clone(s.Texts), func(t model.TextView) bool { return t.Draft })
	return s
}

// DraftSummary — что не попадёт в печать по плану: пустые места сетки и заготовки.
type DraftSummary struct {
	Spreads []int // номера (с 1) разворотов, где есть пустые места или заготовки
	Empty   int   // пустых мест
	Drafts  int   // заготовок (с обложкой)
	Cover   bool  // заготовки есть на обложке плана
	// Blank, BlankCover — листы плана и обложка, где кроме заготовок печатать нечего (TakeBlank):
	// о них спрашивает только этот вопрос (0.28-131-19).
	Blank      []int
	BlankCover bool
}

// TakeBlank — забирает из пустых для печати (BlankSpreads, BlankCover) листы с заготовками плана:
// лист только с заготовкой — один вопрос, о заготовках, а не ещё и «пустой» до папки
// (0.28-131-19). Возвращает оставшиеся пустые; blank не меняет. Пустой лист без мест, поэтому
// номер из Spreads у пустого листа — номер листа с заготовкой плана.
func (d *DraftSummary) TakeBlank(blank []int, cover bool) ([]int, bool) {
	rest := make([]int, 0, len(blank))
	for _, n := range blank {
		if slices.Contains(d.Spreads, n) {
			d.Blank = append(d.Blank, n)
		} else {
			rest = append(rest, n)
		}
	}
	d.BlankCover = cover && d.Cover
	return rest, cover && !d.Cover
}

// DraftSummaryFor — сводка по выбранному, как EmptySummaryFor, плюс заготовки: pl == nil — весь
// альбом; у цели-страницы считаются места и тексты, заходящие на неё (pageW — ширина страницы, мм).
// Обложка входит, если она в плане.
func DraftSummaryFor(v model.View, pl *Plan, pageW float64) DraftSummary {
	var d DraftSummary
	d.Spreads, d.Empty = EmptySummaryFor(v, pl, pageW)
	for i, s := range v.Spreads {
		k := 0
		for _, t := range s.Texts {
			r := geom.TextBox{X: t.X, Y: t.Y, W: t.W, H: t.H, Rot: t.Rot}.AABB()
			if t.Draft && (pl == nil || pl.wants(i+1, layout.Rect{X: r.X, Y: r.Y, W: r.W, H: r.H}, pageW)) {
				k++
			}
		}
		if k == 0 {
			continue
		}
		d.Drafts += k
		if !slices.Contains(d.Spreads, i+1) {
			d.Spreads = append(d.Spreads, i+1)
		}
	}
	slices.Sort(d.Spreads)
	if v.Cover != nil && (pl == nil || pl.Cover != nil) {
		for _, t := range v.Cover.Sheet.Texts {
			if t.Draft {
				d.Drafts++
				d.Cover = true
			}
		}
	}
	return d
}

// EmptyDraftWarning — вопрос перед экспортом о пустых местах и заготовках; "" — нечего. Без
// заготовок — прежний текст EmptyWarning. Номеров не больше 10, остальные — «…и ещё K».
func EmptyDraftWarning(d DraftSummary) string {
	if d.Drafts == 0 {
		return EmptyWarning(d.Spreads, d.Empty)
	}
	head := "На "
	if d.Cover {
		head += "обложке"
		if len(d.Spreads) > 0 {
			head += " и "
		}
	}
	switch len(d.Spreads) {
	case 0:
	case 1:
		head += "развороте " + numList(d.Spreads)
	default:
		head += "разворотах " + numList(d.Spreads)
	}
	parts := make([]string, 0, 2)
	tail := "Заготовки не печатаются."
	if d.Empty > 0 {
		parts = append(parts, fmt.Sprintf("пустые места: %d", d.Empty))
		tail = "Заготовки не печатаются, на месте пустых мест будет фон."
	}
	parts = append(parts, fmt.Sprintf("заготовки: %d", d.Drafts))
	return fmt.Sprintf("%s %s. %s%s Выгрузить?", head, strings.Join(parts, ", "), tail, blankTail(d.Blank, d.BlankCover))
}

// blankTail — что выйдет пустым без заготовок (TakeBlank); "" — ничего.
func blankTail(nums []int, cover bool) string {
	switch {
	case cover && len(nums) == 0:
		return " Обложка выйдет пустой."
	case cover && len(nums) == 1:
		return " Обложка и разворот " + numList(nums) + " выйдут пустыми."
	case cover:
		return " Обложка и развороты " + numList(nums) + " выйдут пустыми."
	case len(nums) == 1:
		return " Разворот " + numList(nums) + " выйдет пустым."
	case len(nums) > 1:
		return " Развороты " + numList(nums) + " выйдут пустыми."
	}
	return ""
}
