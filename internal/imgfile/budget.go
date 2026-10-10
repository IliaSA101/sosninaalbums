package imgfile

import "sync"

// DecodeBudgetPixels — сколько пикселей оригиналов можно держать в памяти одновременно
// (миниатюры и экспорт вместе): ~4 байта на пиксель, иначе параллельные декоды
// больших файлов съедают память целиком.
const DecodeBudgetPixels int64 = 500_000_000

// Budget — взвешенный семафор. Запрос больше всего лимита не отклоняется:
// он ждёт, пока не освободится всё, и идёт один.
type Budget struct {
	mu    sync.Mutex
	cond  *sync.Cond
	limit int64
	used  int64
}

func NewBudget(limit int64) *Budget {
	b := &Budget{limit: limit}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// DecodeBudget общий для всей программы.
var DecodeBudget = NewBudget(DecodeBudgetPixels)

// Acquire блокируется, пока n не поместится в бюджет (или пока не освободится всё).
func (b *Budget) Acquire(n int64) {
	b.mu.Lock()
	for b.used > 0 && b.used+n > b.limit {
		b.cond.Wait()
	}
	b.used += n
	b.mu.Unlock()
}

// Release возвращает n в бюджет.
func (b *Budget) Release(n int64) {
	b.mu.Lock()
	b.used -= n
	b.mu.Unlock()
	b.cond.Broadcast()
}

// Used — сколько занято сейчас (для тестов).
func (b *Budget) Used() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used
}
