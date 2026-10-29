package imgfile

import (
	"context"
	"sync"
)

// DecodeBudgetPixels — сколько пикселей оригиналов можно держать в памяти одновременно
// (миниатюры и экспорт вместе): ~4 байта на пиксель, иначе параллельные декоды
// больших файлов съедают память целиком.
const DecodeBudgetPixels int64 = 500_000_000

// Budget — взвешенный семафор. Запрос больше всего лимита не отклоняется:
// он ждёт, пока не освободится всё, и идёт один.
type Budget struct {
	mu    sync.Mutex
	wake  chan struct{} // закрывается при каждом Release и заменяется новым: будит всех ожидающих
	limit int64
	used  int64
	wait  int // сколько AcquireCtx сейчас ждёт места
}

func NewBudget(limit int64) *Budget {
	return &Budget{limit: limit, wake: make(chan struct{})}
}

// DecodeBudget общий для всей программы.
var DecodeBudget = NewBudget(DecodeBudgetPixels)

// Acquire блокируется, пока n не поместится в бюджет (или пока не освободится всё).
func (b *Budget) Acquire(n int64) {
	_ = b.AcquireCtx(context.Background(), n)
}

// AcquireCtx — Acquire с отменой: ctx отменён до или во время ожидания — ctx.Err(), бюджет не
// тронут. Уже отменённый ctx — отказ сразу, даже если место есть (остановка предсказуема).
func (b *Budget) AcquireCtx(ctx context.Context, n int64) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		b.mu.Lock()
		if b.used == 0 || b.used+n <= b.limit {
			b.used += n
			b.mu.Unlock()
			return nil
		}
		wake := b.wake
		b.wait++
		b.mu.Unlock()
		select {
		case <-wake:
			b.endWait()
		case <-ctx.Done():
			b.endWait()
			return ctx.Err()
		}
	}
}

// Release возвращает n в бюджет.
func (b *Budget) Release(n int64) {
	b.mu.Lock()
	b.used -= n
	close(b.wake)
	b.wake = make(chan struct{})
	b.mu.Unlock()
}

// Used — сколько занято сейчас (для тестов).
func (b *Budget) Used() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used
}

func (b *Budget) endWait() {
	b.mu.Lock()
	b.wait--
	b.mu.Unlock()
}

// Waiting — сколько вызовов AcquireCtx сейчас ждут места. Нужен тестам: «ожидающий уже встал»
// без пауз-угадайки.
func (b *Budget) Waiting() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.wait
}
