package spreads

// Серии съёмки (спека 051 §4): где между фото ленты проходит «естественная» граница разворота —
// смена папки и большая пауза во времени съёмки (EXIF). Чистая функция от PlanJob.infos:
// подсказка окна и сборка видят одни и те же границы.

import (
	"math"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Пороги серий — стартовые гипотезы спеки 051 (подбор на свадьбе владельца).
const (
	seriesWindow    = 10   // окно медианы разрывов: ±10 фото
	seriesMinGapSec = 120  // очередь короче двух минут не дробится
	seriesMedianK   = 4    // порог паузы — 4 медианных разрыва, но не меньше seriesMinGapSec
	seriesFullSec   = 1200 // пауза от 20 минут — граница полной силы
	seriesSpan      = 8    // пауза в 8 порогов — полная сила (ln 8)
	seriesMinDist   = 2    // две границы ближе: остаётся сильная
)

// seriesCuts — сила границы серии перед каждым фото: len n+1, [0] = [n] = 0, значения 0…1.
// nil — сигналов нет (одна папка, времени нет или пауз нет). Границы:
//   - смена папки (Dir пути, без учёта регистра) — 1; у своих из библиотеки (Lib) папка не сигнал;
//   - пауза между снимками: порог T₀ = max(4·m, 120 с), m — медиана положительных разрывов
//     в окне ±10 фото (местный темп съёмки); сила clamp(ln(g/T₀)/ln 8, 0, 1), от 20 минут — 1.
//     Время не растёт (две камеры, не по порядку), рядом со скачком оно откатывается назад (сдвиг
//     часов, а не пауза) или у какого-то из пары его нет — сигнала нет;
//   - две границы подряд (ближе 2 фото): остаётся сильная, иначе развороты по 1–2 фото.
func seriesCuts(infos []model.PhotoInfo) []float64 {
	n := len(infos)
	if n < 2 {
		return nil
	}
	gap := make([]float64, n+1) // gap[j] — разрыв времени перед фото j, секунды; 0 — нет сигнала
	ts := make([]int64, n+2)    // ts[i+1] — время фото i; края — 0 («времени нет»)
	for i, ph := range infos {
		ts[i+1] = ph.Taken
	}
	for j := 1; j < n; j++ {
		prev, a, b, next := ts[j-1], ts[j], ts[j+1], ts[j+2]
		// Время откатывается назад рядом со скачком (две камеры вперемешку): скачок вверх —
		// сдвиг часов, не пауза. Смотрим и на фото до пары, и на фото после: на краю ленты видно одно.
		back := (next > 0 && next < b) || prev > a
		if a > 0 && b > a && !back {
			gap[j] = float64(b - a)
		}
	}
	cuts := make([]float64, n+1)
	found := false
	for j := 1; j < n; j++ {
		c := 0.0
		if folderChanged(infos[j-1], infos[j]) {
			c = 1
		} else if gap[j] > 0 {
			c = gapStrength(gap, j)
		}
		if c > 0 {
			cuts[j], found = c, true
		}
	}
	if !found {
		return nil
	}
	thinCuts(cuts)
	return cuts
}

// folderChanged — соседние фото лежат в разных папках. Свои из библиотеки (Lib) в папках
// не лежат: для них смена папки не сигнал.
func folderChanged(a, b model.PhotoInfo) bool {
	if a.Lib != "" || b.Lib != "" {
		return false
	}
	return !strings.EqualFold(filepath.Dir(a.Path), filepath.Dir(b.Path))
}

// gapStrength — сила паузы перед фото j по местной медиане разрывов.
func gapStrength(gap []float64, j int) float64 {
	g := gap[j]
	if g >= seriesFullSec {
		return 1
	}
	var w []float64
	for k := max(1, j-seriesWindow); k <= min(len(gap)-2, j+seriesWindow); k++ {
		if gap[k] > 0 {
			w = append(w, gap[k])
		}
	}
	slices.Sort(w)
	m := math.Max(w[len(w)/2], 1) // g > 0 сам в окне: w не пуст
	t0 := math.Max(seriesMedianK*m, seriesMinGapSec)
	return min(max(math.Log(g/t0)/math.Log(seriesSpan), 0), 1)
}

// thinCuts оставляет из двух границ ближе seriesMinDist более сильную (при равенстве — раннюю).
func thinCuts(cuts []float64) {
	last, kept := 0, (*float64)(nil) // kept — оставленная граница (указатель, а не индекс: gosec G602 не видит, что last ≥ 0)
	for j := range cuts {
		c := &cuts[j]
		if *c <= 0 {
			continue
		}
		switch {
		case kept == nil || j-last >= seriesMinDist:
			last, kept = j, c
		case *c > *kept:
			*kept, last, kept = 0, j, c
		default:
			*c = 0
		}
	}
}
