package model

import "math"

// Свой кадр фото (098 design §3): PhotoRef.Asp — пропорции окна, в котором стоит Frame. Видимая
// часть фото зависит только от пропорций ячейки и кадра (замер design, расхождение 1,7e-16),
// поэтому ячейка «Авто» с пропорциями Asp показывает ту же часть снимка, что ручное место.

// Пределы своего кадра: те же, что у кадра из файла (salbum.maxAspect) — узкое ручное место
// бывает длиннее 1 : 10.
const (
	minAsp = 0.01
	maxAsp = 100
)

// ClampAsp — свой кадр к допустимому: NaN, Inf, ≤ 0 и вне minAsp…maxAsp — 0 (нет своего кадра);
// шаг 1e-4.
func ClampAsp(a float64) float64 {
	if math.IsNaN(a) || math.IsInf(a, 0) || a < minAsp || a > maxAsp {
		return 0
	}
	return math.Round(a*1e4) / 1e4
}

// cropTol — порог «правленого» фото (спека 098 §2 п. 3): окно места отличается по пропорциям от
// целого фото больше чем на 1 %. Меньше — глазом не видно, свой кадр только дробил бы кэш раскладок.
const cropTol = 0.01

// KeepCrops — свой кадр фото мест сетки s (098 design §3.2): у места с фото окно рамочки
// (BorderWindow) сравнивается с пропорциями целого фото (PhotoAspects); отличие больше cropTol —
// Asp = пропорции окна, иначе 0. «Целиком» показывает фото без обрезки — у него тоже 0. Ответ —
// сколько фото получат свой кадр; apply = false — только счёт (галочка «Вернуть авто?»).
func KeepCrops(p *Project, s *Spread, apply bool) int {
	n := 0
	for _, pl := range s.Grid {
		r, ok := p.Photos[pl.Photo]
		if pl.Photo == "" || !ok {
			continue
		}
		asp := 0.0
		if win, _ := p.BorderWindow(pl); pl.Fit != FitContain && win.W > 0 && win.H > 0 {
			w, e := win.W/win.H, p.PhotoAspects([]string{pl.Photo})[0]
			if math.Abs(w/e-1) > cropTol {
				asp = ClampAsp(w)
			}
		}
		if asp > 0 {
			n++
		}
		if apply && r.Asp != asp {
			r.Asp = asp
			p.Photos[pl.Photo] = r
		}
	}
	return n
}
