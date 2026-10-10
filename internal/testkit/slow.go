package testkit

import (
	"os"
	"strconv"
	"time"
)

// Limit — порог времени для теста вида «не зависло / не стало квадратичным».
// Абсолютные миллисекунды ломаются от нагрузки (параллельные полные прогоны,
// антивирус), поэтому порог = базовый × RaceSlowdown × SA_TEST_SLOW (по умолчанию 1;
// на заведомо занятой машине можно задать 4, 10). Базовый берут с запасом в
// десятки раз над обычным временем: зависание и квадратичный рост уходят в
// секунды и порог всё равно пробивают.
func Limit(base time.Duration) time.Duration {
	k := 1.0
	if s := os.Getenv("SA_TEST_SLOW"); s != "" {
		if v, err := strconv.ParseFloat(s, 64); err == nil && v >= 1 {
			k = v
		}
	}
	return time.Duration(float64(base) * RaceSlowdown * k)
}

// Fastest запускает fn n раз и возвращает самое короткое время. Всплеск нагрузки
// портит один прогон, а не все подряд; настоящее зависание или квадратичный
// рост медленны в каждом прогоне, так что минимум их не прячет.
func Fastest(n int, fn func()) time.Duration {
	best := time.Duration(-1)
	for range max(n, 1) {
		start := time.Now()
		fn()
		if d := time.Since(start); best < 0 || d < best {
			best = d
		}
	}
	return best
}
