package model

import (
	"fmt"
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// Точные пиксели файла (123, 0.26-123-33): загрузчик типографии требует размер файла до
// пикселя, а мм хранятся с шагом 0,01. Здесь — обратная задача «px → мм», чистые функции.

// PrintDPI — плотность файла для типографии: выгрузка «Для Фотосферы/Фабрики» всегда 300 dpi,
// поэтому поля «Файл разворота/обложки, px» считают при ней, а не при dpi альбома.
const PrintDPI = 300

// PageTol — допуск совпадения размера страницы с форматом, мм (123 §1.2): меньше половины
// расстояния между ближайшими разными форматами (1 мм: 201/200, 251/250, 286/285); при 300 dpi
// это ≤ 3 px. Значение зеркалит PAGE_TOL интерфейса, паритет — testdata/parity/pagetol.json.
const PageTol = 0.25

// pageTolEps — запас на шум float: 303,75 и 304 отстоят ровно на допуск, а 202,95 и 203,2 —
// на допуск плюс 1e-14.
const pageTolEps = 1e-9

// SameMm — размеры a и b (мм) совпадают в пределах PageTol.
func SameMm(a, b float64) bool { return math.Abs(a-b) <= PageTol+pageTolEps }

// Px — мм в пиксели при dpi, как render.px: round(мм / 25,4 · dpi). model рендер не импортирует,
// паритет закрепляет TestPxParity (internal/export).
func Px(mm float64, dpi int) int { return int(math.Round(mm / 25.4 * float64(dpi))) }

// pxSearch — число мм из сетки 0,01 вокруг exact (±5 шагов), для которого px(мм) == target;
// ok=false — таких нет. Одному px отвечают 2–4 числа сетки (шаг 0,01 мм — это 0,01/25,4·dpi px,
// меньше 1 px при dpi < 2540: решение есть всегда, 123 §1.1). Берётся самое круглое (целое, потом
// кратное 0,5, 0,1, 0,05), при равенстве — ближайшее к exact: файл шаблона 7181 × 2398 даёт
// страницу 304 × 203, а не 303,99 × 202,99, и размер, введённый из круглых мм, возвращается этими мм.
func pxSearch(exact float64, target int, px func(mm float64) int) (float64, bool) {
	c0 := geom.Round2(exact)
	best, bestRank, bestD, ok := 0.0, 0, 0.0, false
	for i := -5; i <= 5; i++ {
		c := geom.Round2(c0 + float64(i)*0.01)
		if px(c) != target {
			continue
		}
		rank, d := roundRank(c), math.Abs(c-exact)
		if !ok || rank < bestRank || rank == bestRank && d < bestD {
			best, bestRank, bestD, ok = c, rank, d, true
		}
	}
	return best, ok
}

// roundRank — «круглость» числа сетки 0,01: 0 — целое, 1 — кратно 0,5, 2 — 0,1, 3 — 0,05, 4 — прочее.
func roundRank(mm float64) int {
	k := int(math.Round(mm * 100))
	for i, m := range [4]int{100, 50, 10, 5} {
		if k%m == 0 {
			return i
		}
	}
	return 4
}

// checkPxArgs — dpi и размеры в px годны для подбора.
func checkPxArgs(dpi int, sizes ...int) error {
	if dpi < minDPI || dpi > maxDPI {
		return fmt.Errorf("плотность %d dpi вне %d…%d", dpi, minDPI, maxDPI)
	}
	for _, v := range sizes {
		if v <= 0 {
			return fmt.Errorf("размер файла должен быть больше нуля: %d px", v)
		}
	}
	return nil
}

// rangeErr — ошибка, если требуемый размер exact (мм) вне lo…hi с запасом в 5 шагов подбора.
// what — «Разворот 90000 px при 300 dpi — страница больше 1000 мм».
func rangeErr(what string, px, dpi int, noun string, exact, lo, hi float64) error {
	const slack = 0.05
	switch {
	case exact > hi+slack:
		return fmt.Errorf("%s %d px при %d dpi — %s больше %s мм", what, px, dpi, noun, MmText(hi))
	case exact < lo-slack:
		return fmt.Errorf("%s %d px при %d dpi — %s меньше %s мм", what, px, dpi, noun, MmText(lo))
	}
	return nil
}

// PageFromPx — страница (мм, шаг 0,01), при которой разворот из двух страниц при dpi выходит
// ровно W×H px: Px(2·pw) == W, Px(ph) == H. Вне пределов ClampSettings — ошибка.
// Пример: 7180×2397 при 300 dpi — 303,95 × 202,95.
func PageFromPx(w, h, dpi int) (pw, ph float64, err error) {
	if err = checkPxArgs(dpi, w, h); err != nil {
		return 0, 0, err
	}
	ew, eh := float64(w)*25.4/float64(dpi)/2, float64(h)*25.4/float64(dpi)
	if err = rangeErr("Разворот", w, dpi, "страница", ew, minPage, maxPage); err != nil {
		return 0, 0, err
	}
	if err = rangeErr("Высота", h, dpi, "страница", eh, minPage, maxPage); err != nil {
		return 0, 0, err
	}
	pw, okW := pxSearch(ew, w, func(mm float64) int { return Px(2*mm, dpi) })
	ph, okH := pxSearch(eh, h, func(mm float64) int { return Px(mm, dpi) })
	if !okW || !okH || pw < minPage || pw > maxPage || ph < minPage || ph > maxPage {
		return 0, 0, fmt.Errorf("разворот %d × %d px при %d dpi не подобрать: страница вне %s…%s мм",
			w, h, dpi, MmText(minPage), MmText(maxPage))
	}
	return pw, ph, nil
}

// SideFromPx — сторонка обложки (мм, шаг 0,01), при которой лист обложки (CoverGeo: заворот,
// сторонка, расстав, корешок spine, расстав, сторонка, заворот) выходит ровно wpx × hpx px.
// Заворот и расстав — из spec, корешок — на текущем числе разворотов: сторонка хранится в мм,
// при другом числе разворотов лист = сторонка + корешок по таблице, как у встроенных (123 §1.1).
func SideFromPx(wpx, hpx, dpi int, spec CoverSpec, spine float64) (sw, sh float64, err error) {
	if err = checkPxArgs(dpi, wpx, hpx); err != nil {
		return 0, 0, err
	}
	wrap, hinge := spec.Wrap, spec.Hinge
	ew := (float64(wpx)*25.4/float64(dpi) - 2*wrap - 2*hinge - spine) / 2
	eh := float64(hpx)*25.4/float64(dpi) - 2*wrap
	if err = rangeErr("Ширина обложки", wpx, dpi, "сторонка", ew, coverMinSide, coverMaxSide); err != nil {
		return 0, 0, err
	}
	if err = rangeErr("Высота обложки", hpx, dpi, "сторонка", eh, coverMinSide, coverMaxSide); err != nil {
		return 0, 0, err
	}
	sw, okW := pxSearch(ew, wpx, func(mm float64) int {
		x, _ := coverAxes(wrap, mm, hinge, spine, 0)
		return Px(x[7], dpi)
	})
	sh, okH := pxSearch(eh, hpx, func(mm float64) int {
		_, y := coverAxes(wrap, 0, hinge, spine, mm)
		return Px(y[3], dpi)
	})
	if !okW || !okH || sw < coverMinSide || sw > coverMaxSide || sh < coverMinSide || sh > coverMaxSide {
		return 0, 0, fmt.Errorf("обложку %d × %d px при %d dpi не подобрать: сторонка вне %s…%s мм",
			wpx, hpx, dpi, MmText(coverMinSide), MmText(coverMaxSide))
	}
	return sw, sh, nil
}

// FitTolPx — на сколько px по оси выгрузка вправе подогнать файл к px формата (123 §5): допуск
// формата на оба края разворота, Px(2·PageTol) = 6 px при 300 dpi (≤ 3 px на сторону). Чужие
// форматы (201/200 — 24 px) никогда не подгоняются.
func FitTolPx(dpi int) int { return Px(2*PageTol, dpi) }

// pxNat — размер файла разворота страницы w × h мм при PrintDPI (как render.CanvasSize).
func pxNat(w, h float64) (nw, nh int) { return Px(2*w, PrintDPI), Px(h, PrintDPI) }

// FitPx — размер файла разворота, к которому выгрузка подгоняет холст страницы w × h мм формата f
// (render.Options.Fit). ok=false — подгонка не нужна: у формата нет Px, файл уже точен или
// расхождение больше FitTolPx (страница не этого формата).
func (f PrintFormat) FitPx(w, h float64) (fw, fh int, ok bool) {
	if f.Px == ([2]int{}) {
		return 0, 0, false
	}
	nw, nh := pxNat(w, h)
	tol := FitTolPx(PrintDPI)
	if abs(nw-f.Px[0]) > tol || abs(nh-f.Px[1]) > tol || nw == f.Px[0] && nh == f.Px[1] {
		return 0, 0, false
	}
	return f.Px[0], f.Px[1], true
}
