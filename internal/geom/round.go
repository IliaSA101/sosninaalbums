package geom

import "math"

// Round2 — до 0.01 мм: рамка, сдвинутая на шум, — холостая правка. Минус ноль не оставляет:
// в JSON «-0» ≠ «0», и повтор правки давал бы лишний шаг отмены.
func Round2(v float64) float64 {
	v = math.Round(v*100) / 100
	if v == 0 {
		return 0
	}
	return v
}

// ceil01 и floor01 — округление до 0.01 вверх и вниз с допуском на шум float.
func ceil01(v float64) float64  { return math.Ceil(v*100-1e-4) / 100 }
func floor01(v float64) float64 { return math.Floor(v*100+1e-4) / 100 }

// ClampF — v в [lo, hi]; при lo > hi — lo.
func ClampF(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// Floor2 — вниз до 0.01 мм (с допуском на двоичную запись): верхняя граница
// зажима, чтобы округление не вывело место за край разворота.
func Floor2(v float64) float64 { return math.Floor(v*100+1e-6) / 100 }

// clampIn01 — v (на сетке 0.01) в [lo, hi], границы округлены внутрь до 0.01;
// между ними нет сотых — lo вверх. −0 (ceil от −0.0001) — 0: иначе в файле "-0".
func clampIn01(v, lo, hi float64) float64 {
	in, out := ceil01(lo), floor01(hi)
	if in <= out {
		in = ClampF(v, in, out)
	}
	if in == 0 {
		return 0
	}
	return in
}
