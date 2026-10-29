package packprep

import (
	"encoding/json"
	"fmt"
	"image"
	"slices"
	"sort"
	"strings"
)

// Бюджет набора (спека 129, design §1): предметов по видам и имён на весь набор.
const (
	BudgetBg          = 12
	BudgetFrame       = 8
	BudgetCorner      = 8
	BudgetDivider     = 6
	BudgetStickerEach = 36 // стикеров на один стиль
	BudgetNames       = 106
	MinStickerSide    = 600 // длинная сторона стикера меньше — флаг (как sheetcut.MinSidePx)
)

// Upscale — увеличение фона: «апскейл ×N от W px».
type Upscale struct {
	From   int     `json:"from"`
	Factor float64 `json:"factor"`
}

// Output — один предмет набора в отчёте. N — номер в порядке манифеста (стикеры по стилям, уголки,
// разделители, рамки, фоны): тот же номер на листах проверки.
type Output struct {
	N           int      `json:"n"`
	File        string   `json:"file"`
	Kind        string   `json:"kind"`
	Style       string   `json:"style,omitempty"`
	Family      string   `json:"family"`
	Name        string   `json:"name"`
	Theme       string   `json:"theme"`
	Src         string   `json:"src"`
	Index       int      `json:"index,omitempty"` // номер нарезки в источнике, с 1
	W           int      `json:"w"`
	H           int      `json:"h"`
	CmW         float64  `json:"cmW"` // при 300 dpi
	CmH         float64  `json:"cmH"`
	Bytes       int      `json:"bytes"`
	Stretch     *float64 `json:"stretch,omitempty"` // рамка: «тянется, %»
	Upscale     *Upscale `json:"upscale,omitempty"` // фон: увеличен
	Similar     int      `json:"similar,omitempty"` // «похоже на №N»
	SimilarDist int      `json:"similarDist,omitempty"`
	Flags       []string `json:"flags,omitempty"`

	item  int // номер предмета рецепта
	order int // порядок появления: тай-брейк сортировки
	thumb *image.NRGBA
	hash  Hash
}

// Report — итог прогона (_check/report.json). Errors не пуст — набор не готов (код выхода 1).
type Report struct {
	ID       string           `json:"id"`
	Version  int              `json:"version"`
	OK       bool             `json:"ok"`
	Counts   map[string]int   `json:"counts"`
	Styles   map[string]int   `json:"styles,omitempty"` // стикеров по стилям
	Names    int              `json:"names"`            // имён всего (бюджет 106)
	Items    []*Output        `json:"items"`
	Families map[string][]int `json:"families,omitempty"` // семейство → номера предметов
	Errors   []string         `json:"errors,omitempty"`
	Flags    []string         `json:"flags,omitempty"` // сводка флагов предметов: «№N файл: текст»
	Skipped  []string         `json:"skipped,omitempty"`
	Notes    []string         `json:"notes,omitempty"`
}

// JSON — отчёт в JSON.
func (r *Report) JSON() ([]byte, error) { return json.MarshalIndent(r, "", "  ") }

// Text — отчёт для человека: счётчики, ошибки и флаги.
func (r *Report) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "набор %s v%d: предметов %d (фонов %d, рамок %d, уголков %d, разделителей %d, стикеров %d), имён %d из %d\n",
		r.ID, r.Version, len(r.Items), r.Counts[KindBg], r.Counts[KindFrame], r.Counts[KindCorner], r.Counts[KindDivider], r.Counts[KindSticker], r.Names, BudgetNames)
	for _, s := range sortedKeys(r.Styles) {
		fmt.Fprintf(&b, "  стикеров в стиле «%s»: %d из %d\n", s, r.Styles[s], BudgetStickerEach)
	}
	for _, s := range r.Skipped {
		fmt.Fprintf(&b, "пропущено: %s\n", s)
	}
	for _, f := range r.Flags {
		fmt.Fprintf(&b, "? %s\n", f)
	}
	for _, n := range r.Notes {
		fmt.Fprintf(&b, "! %s\n", n)
	}
	for _, e := range r.Errors {
		fmt.Fprintf(&b, "ОШИБКА: %s\n", e)
	}
	return b.String()
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// kindRank — порядок в манифесте после стикеров: уголки → разделители → рамки → фоны.
var kindRank = map[string]int{KindSticker: 0, KindCorner: 1, KindDivider: 2, KindFrame: 3, KindBg: 4}

// ManifestKind — вид в манифесте: уголки и разделители — стикеры.
func ManifestKind(kind string) string {
	if kind == KindCorner || kind == KindDivider {
		return KindSticker
	}
	return kind
}

// arrange упорядочивает предметы как в манифесте (стикеры по стилям в порядке появления, затем уголки,
// разделители, рамки, фоны; внутри — порядок рецепта и нарезки) и нумерует с 1.
func arrange(outs []*Output) {
	styleAt := map[string]int{}
	for _, o := range outs {
		if o.Kind == KindSticker {
			if _, ok := styleAt[o.Style]; !ok {
				styleAt[o.Style] = len(styleAt)
			}
		}
	}
	slices.SortStableFunc(outs, func(a, b *Output) int {
		if a.Kind != b.Kind {
			return kindRank[a.Kind] - kindRank[b.Kind]
		}
		if a.Kind == KindSticker && a.Style != b.Style {
			return styleAt[a.Style] - styleAt[b.Style]
		}
		return a.order - b.order
	})
	for i, o := range outs {
		o.N = i + 1
	}
}

// budget — счётчики и нарушения бюджета.
func (r *Report) budget() {
	r.Counts = map[string]int{KindBg: 0, KindFrame: 0, KindCorner: 0, KindDivider: 0, KindSticker: 0}
	r.Styles = map[string]int{}
	for _, o := range r.Items {
		r.Counts[o.Kind]++
		if o.Kind == KindSticker {
			r.Styles[o.Style]++
		}
	}
	r.Names = len(r.Items)
	for _, l := range []struct {
		kind  string
		limit int
		word  string
	}{{KindBg, BudgetBg, "фонов"}, {KindFrame, BudgetFrame, "рамок"}, {KindCorner, BudgetCorner, "уголков"}, {KindDivider, BudgetDivider, "разделителей"}} {
		if r.Counts[l.kind] > l.limit {
			r.fail("%s %d, бюджет набора — не больше %d", l.word, r.Counts[l.kind], l.limit)
		}
	}
	for _, s := range sortedKeys(r.Styles) {
		if r.Styles[s] > BudgetStickerEach {
			r.fail("стикеров в стиле «%s» %d, бюджет — не больше %d", s, r.Styles[s], BudgetStickerEach)
		}
	}
	if r.Names > BudgetNames {
		r.fail("имён в наборе %d, бюджет — не больше %d", r.Names, BudgetNames)
	}
}

func (r *Report) fail(format string, a ...any) {
	r.Errors = append(r.Errors, fmt.Sprintf(format, a...))
}

// markSimilar ставит «похоже на №N»: тот же вид, расстояние dHash не больше SimilarMax. Подсказка,
// не отсев. Уголки одного листа не сравниваются — четверти и есть повороты друг друга.
func markSimilar(outs []*Output) {
	for j, b := range outs {
		best, near := SimilarMax+1, (*Output)(nil)
		for _, a := range outs[:j] {
			if a.Kind != b.Kind || (a.Kind == KindCorner && a.item == b.item) {
				continue
			}
			if d := Distance(a.hash, b.hash); d < best {
				best, near = d, a
			}
		}
		if near != nil {
			b.Similar, b.SimilarDist = near.N, best
			b.Flags = append(b.Flags, fmt.Sprintf("похоже на №%d (dHash %d из 64)", near.N, best))
		}
	}
}

// collect сводит флаги предметов и семейства в отчёт.
func (r *Report) collect() {
	r.Families = map[string][]int{}
	for _, o := range r.Items {
		r.Families[o.Family] = append(r.Families[o.Family], o.N)
		for _, f := range o.Flags {
			r.Flags = append(r.Flags, fmt.Sprintf("№%d %s: %s", o.N, o.File, f))
		}
	}
	r.OK = len(r.Errors) == 0
}
