package model

import (
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
)

// Ряды «Авто» (вынесены из project.go в 098: тот у предела размера).

// VariantLimit — длина ряда раскладок (и круга листания) до 16 фото.
const VariantLimit = 24

// RowLen — длина первого ряда «Авто» для n фото: до 16 — VariantLimit, с layout.ManyFrom —
// первая порция (layout.ManyBatch), следующие досчитываются по требованию (AutoRow).
func RowLen(n int) int {
	if n >= layout.ManyFrom {
		return layout.ManyBatch
	}
	return VariantLimit
}

// AutoRow — ряд «Авто» в порядке ленты (variety.Row: лучшая, формы и ряд layout вперемешку,
// 051), где стоит key, и его место в ряду (−1 — нет). До 16 фото — весь ряд VariantLimit. С
// ManyFrom — наименьший префикс из порций по ManyBatch (до ManyCap), где key есть; нет нигде —
// первая порция. more — ряд можно досчитать: он полный и короче ManyCap.
func AutoRow(asp []float64, prm layout.Params, key string) (row []layout.Result, i int, more bool) {
	if len(asp) < layout.ManyFrom {
		row = variety.RowResults(asp, prm, VariantLimit)
		return row, IndexOf(row, key), false
	}
	for n := layout.ManyBatch; n <= layout.ManyCap; n += layout.ManyBatch {
		row = variety.RowResults(asp, prm, n)
		if i = IndexOf(row, key); i >= 0 {
			return row, i, len(row) == n && n < layout.ManyCap
		}
		if len(row) < n {
			break // дальше ряд не растёт
		}
	}
	row = variety.RowResults(asp, prm, layout.ManyBatch)
	return row, -1, len(row) == layout.ManyBatch
}

// MoreRow — ряд «Авто» на n ≥ ManyFrom фото, досчитанный ещё на порцию после have вариантов
// (не длиннее ManyCap). До 16 фото — весь ряд.
func MoreRow(asp []float64, prm layout.Params, have int) (row []layout.Result, more bool) {
	if len(asp) < layout.ManyFrom {
		return variety.RowResults(asp, prm, VariantLimit), false
	}
	n := min((max(have, 0)/layout.ManyBatch+1)*layout.ManyBatch, layout.ManyCap)
	row = variety.RowResults(asp, prm, n)
	return row, len(row) == n && n < layout.ManyCap
}

// ExtraRow — «Другие варианты» разворота (041): в подборке третьим рядом, в листании и счётчике
// «i / N» — сразу после ряда «Авто» (решение 0.11 №36). Ключи строит layout: SetLayout их примет.
// С ManyFrom фото — пусто: ряд «Авто» там растёт порциями («+»), а страница поменьше делает
// фото мельче порога воздуха (k_air ≤ 16, design 041 §5).
func ExtraRow(asp []float64, prm layout.Params) []layout.Result {
	if len(asp) >= layout.ManyFrom {
		return nil
	}
	return variety.Extra(asp, prm, RowLen(len(asp)))
}

// ExtraFoldHidden — раскладок «Разброса», скрытых правилом сгиба (variety.ExtraFoldHidden): строка
// «Ещё N раскладок с фото на сгибе» в галерее. С ManyFrom фото — 0, как и ExtraRow.
func ExtraFoldHidden(asp []float64, prm layout.Params) int {
	if len(asp) >= layout.ManyFrom {
		return 0
	}
	return variety.ExtraFoldHidden(asp, prm, RowLen(len(asp)))
}

func IndexOf(rs []layout.Result, key string) int {
	for i, r := range rs {
		if r.Key == key {
			return i
		}
	}
	return -1
}
