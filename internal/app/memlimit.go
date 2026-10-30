package app

import (
	"math"
	"os"
	"runtime/debug"
	"sync"
)

// memLimit — мягкий предел кучи Go: окно и прогрев держат до 8 декодов по ≈ 80 МБ и 24 превью
// в очереди хвостов, а GOGC=100 добавляет столько же мусора сверху — пик Sys 1,9 ГБ (L1 после 090 G1).
// С пределом сборщик срабатывает раньше; живое (≈ 1,05 ГБ) ниже него, скорость окна не меняется (090M-D1).
const memLimit = 1536 << 20

var (
	memMu    sync.Mutex
	memOurs  bool  // предел поставил applyMemoryLimit, а не GOMEMLIMIT
	memLifts int   // экспортов, снявших предел (флаг exporting пускает один, счётчик — на стык двух)
	memPrev  int64 // предел до первого снятия
)

// applyMemoryLimit ставит memLimit, если GOMEMLIMIT не задан: заданный руками (замер, отладка) главнее.
func applyMemoryLimit() {
	memMu.Lock()
	defer memMu.Unlock()
	_, set := os.LookupEnv("GOMEMLIMIT")
	memOurs = !set
	if set {
		return
	}
	debug.SetMemoryLimit(memLimit)
}

// liftMemoryLimit снимает наш предел на время экспорта: 2 холста до 480 МБ и 6 слотов фото по ≈ 100 МБ
// живого больше 1,5 ГБ, и под пределом сборщик крутился бы без конца (090M-D3). Возвращает возврат
// прежнего предела; повторный вызов возврата ничего не делает. Предел из GOMEMLIMIT не трогаем.
func liftMemoryLimit() (restore func()) {
	memMu.Lock()
	defer memMu.Unlock()
	if !memOurs {
		return func() {}
	}
	if memLifts == 0 {
		memPrev = debug.SetMemoryLimit(math.MaxInt64)
	}
	memLifts++
	var once sync.Once
	return func() {
		once.Do(func() {
			memMu.Lock()
			defer memMu.Unlock()
			if memLifts--; memLifts == 0 {
				debug.SetMemoryLimit(memPrev)
			}
		})
	}
}
