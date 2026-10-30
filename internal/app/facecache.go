package app

import (
	"container/list"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/faces"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
)

// Кэш точек лиц (спека 130, design §7): повтор «Подогнать по лицам» и ответ на окно о правленых
// кадрах идут без поиска. Только память, на диск и в .salbum не пишется (0.27-130-8).

// faceCacheMax — записей в кэше: ≈ 1 МБ при нескольких лицах на фото.
const faceCacheMax = 4096

// faceKey — файл (путь, размер, время изменения), его цветовой класс (от него зависит превью),
// четверти поворота и версия детектора: сменился любой — прежние точки не годятся.
type faceKey struct {
	path        string
	size, mtime int64
	color       imgfile.Color
	turn        int
	version     string
}

// faceCache — LRU точек: нулевое значение годится.
type faceCache struct {
	mu  sync.Mutex
	ll  *list.List // спереди — недавние
	idx map[faceKey]*list.Element
}

type faceEntry struct {
	key  faceKey
	list []faces.Face
}

// get — точки по ключу; ok = false — поиска ещё не было. Слайс только для чтения.
func (c *faceCache) get(k faceKey) (fs []faces.Face, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.idx[k]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*faceEntry).list, true
}

// put кладёт точки (пустой список — «лица нет» — тоже ответ) и вытесняет самые давние сверх предела.
func (c *faceCache) put(k faceKey, fs []faces.Face) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.idx == nil {
		c.ll, c.idx = list.New(), map[faceKey]*list.Element{}
	}
	if el, ok := c.idx[k]; ok {
		el.Value.(*faceEntry).list = fs
		c.ll.MoveToFront(el)
		return
	}
	c.idx[k] = c.ll.PushFront(&faceEntry{key: k, list: fs})
	for c.ll.Len() > faceCacheMax {
		old := c.ll.Back()
		c.ll.Remove(old)
		delete(c.idx, old.Value.(*faceEntry).key)
	}
}

// size — записей в кэше (тесты).
func (c *faceCache) size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ll == nil {
		return 0
	}
	return c.ll.Len()
}
