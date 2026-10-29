package variety

import "sync"

// varietyGen — записей в поколении кэшей ряда «Авто» и «Других вариантов» (переменная — чтобы
// тест не наполнял кэш тысячами). Запись — до 24 раскладок (≈ 14 КБ на 16 фото): поколение ≈ 70 МБ,
// оба — до 140 МБ; поколение впятеро больше предела разворотов альбома (model.MaxSpreads).
var varietyGen = extraGen

// genCache — кэш на два поколения, как layout.cache (073): переполнилось cur — оно становится
// old, cur пустеет; запись из old при обращении переезжает в cur. Нужное переживает смену, а
// полного сброса, после которого весь View большого альбома считается заново, нет. Худший
// объём — 2·varietyGen записей. Одиночного счёта нет: двойной счёт ключа даёт ту же запись.
type genCache[V any] struct {
	mu       sync.Mutex
	cur, old map[string]V
}

// get ищет запись в обоих поколениях; найденную в old переносит в cur.
func (c *genCache[V]) get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.cur[key]; ok {
		return v, true
	}
	v, ok := c.old[key]
	if ok {
		delete(c.old, key)
		c.putLocked(key, v)
	}
	return v, ok
}

// put кладёт запись в cur, при переполнении сдвигая поколения.
func (c *genCache[V]) put(key string, v V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.putLocked(key, v)
}

func (c *genCache[V]) putLocked(key string, v V) {
	if c.cur == nil || len(c.cur) >= varietyGen {
		c.old, c.cur = c.cur, map[string]V{}
	}
	c.cur[key] = v
}

// has — запись есть в любом поколении; не переносит её (проверка прогрева не должна менять кэш).
func (c *genCache[V]) has(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.cur[key]
	if !ok {
		_, ok = c.old[key]
	}
	return ok
}

// reset — холодный кэш (бенчмарки и тесты).
func (c *genCache[V]) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cur, c.old = map[string]V{}, nil
}

// size — записей в cur и old (тест объёма).
func (c *genCache[V]) size() (cur, old int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.cur), len(c.old)
}
