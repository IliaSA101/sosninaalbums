package library

import (
	"context"
	"image"
	"log"
	"runtime/debug"
	"slices"
)

// tail — превью 2048, которое запрос окна не стал кодировать до ответа (090 design §3): пиксели уже
// уменьшены из того же декода, оригинал второй раз не читается. Поля started и запись в Cache.tails —
// под Cache.mu; done закрывается, когда превью записано или хвост выброшен.
type tail struct {
	key     string
	prev    image.Image
	alpha   bool
	started bool
	done    chan struct{}
}

// queueTail ставит хвост в очередь горутины хвостов. false — очередь полна (память ограничена) или
// кэш закрыт: тогда превью кодирует сам запрос.
func (c *Cache) queueTail(t *tail) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || len(c.queue) >= c.tailMax {
		return false
	}
	c.tails[t.key] = t
	c.queue = append(c.queue, t)
	if !c.tailRun {
		c.tailRun = true
		c.bg.Go(c.tailWorker)
	}
	return true
}

func (c *Cache) hasTail(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tails[key] != nil
}

// takeTail — превью нужно сейчас (холст или прогрев): не начатый хвост запрос забирает и кодирует
// на своём месте — горутина хвостов стоит, пока идут запросы окна; начатый — ждёт его.
func (c *Cache) takeTail(ctx context.Context, t *tail, cl prio) error {
	c.mu.Lock()
	started := t.started
	c.mu.Unlock()
	if started {
		return wait(ctx, t.done)
	}
	release, err := c.sched.acquire(ctx, cl)
	if err != nil {
		return err
	}
	defer release()
	if !c.claimTail(t) { // пока ждали место, хвост взял другой или его выбросил Close
		release()
		return wait(ctx, t.done)
	}
	c.runTail(t)
	return nil
}

// claimTail помечает хвост начатым и убирает из очереди; false — его уже взяли или выбросили.
func (c *Cache) claimTail(t *tail) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t.started || c.tails[t.key] != t {
		return false
	}
	t.started = true
	if i := slices.Index(c.queue, t); i >= 0 {
		c.queue = slices.Delete(c.queue, i, i+1)
	}
	return true
}

// runTail кодирует превью хвоста. Не записалось — превью сделает прогрев или запрос холста полным путём.
func (c *Cache) runTail(t *tail) {
	defer func() {
		if r := recover(); r != nil { // горутина хвостов: паника кодера не роняет программу
			log.Printf("ПАНИКА при превью в фоне: %v\n%s", r, debug.Stack())
		}
		c.mu.Lock()
		c.endTail(t)
		c.mu.Unlock()
	}()
	_ = writeCopy(t.prev, c.base(t.key, true), t.alpha, true)
}

// endTail снимает хвост (под mu): ждущие проснутся и найдут превью или сделают его сами.
func (c *Cache) endTail(t *tail) {
	if c.tails[t.key] == t {
		delete(c.tails, t.key)
	}
	t.prev = nil
	close(t.done)
}

// tailWorker — одна горутина хвостов в классе прогрева: превью кодируются, когда запросы окна кончились.
// Выходит, когда очередь пуста (следующий хвост запустит её снова) или кэш закрыт.
func (c *Cache) tailWorker() {
	for {
		release, err := c.sched.acquire(c.life, bg)
		c.mu.Lock()
		if err != nil || len(c.queue) == 0 {
			c.tailRun = false
			c.mu.Unlock()
			if release != nil {
				release()
			}
			return
		}
		t := c.queue[0]
		c.queue = slices.Delete(c.queue, 0, 1)
		t.started = true
		c.mu.Unlock()
		c.runTail(t)
		release()
	}
}
