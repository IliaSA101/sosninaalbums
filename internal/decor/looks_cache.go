package decor

import (
	"container/list"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Кэш разобранных макетов для Look (G6a, 0.28-131-210): LookHost.Find читает и разбирает файл до
// 512 КБ под замком Editor, /media/look — на каждый запрос плитки. Запись действительна, пока путь из
// индекса, размер и время правки файла те же (os.Stat перед каждым попаданием): установка, обновление
// и удаление набора, SaveLook, переименование и пересканирование не требуют своих сбросов.
const (
	lookCacheMax   = 16      // макетов: окно макетов держит несколько, альбом — один; плитки идут подряд по одному
	lookCacheBytes = 4 << 20 // сумма размеров файлов: разобранное дерево в несколько раз больше файла
)

type lookCacheEnt struct {
	key, path string
	size      int64
	mod       time.Time
	tpl       model.AlbumTpl // не меняется после put: наружу уходит только копия (cloneLook)
}

// parsedLooks — LRU разобранных макетов; под своим замком, Store.mu не берёт.
type parsedLooks struct {
	mu    sync.Mutex
	ll    *list.List // спереди — свежие
	m     map[string]*list.Element
	bytes int64
}

// get — разбор по ключу, если файл тот же (путь, размер, время правки); иначе запись снимается.
func (c *parsedLooks) get(key, path string, st os.FileInfo) (model.AlbumTpl, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.m[key]
	if !ok {
		return model.AlbumTpl{}, false
	}
	e := el.Value.(*lookCacheEnt)
	if e.path != path || e.size != st.Size() || !e.mod.Equal(st.ModTime()) {
		c.remove(el)
		return model.AlbumTpl{}, false
	}
	c.ll.MoveToFront(el)
	return e.tpl, true
}

// put запоминает разбор; st снят до чтения файла — подмена между статом и чтением даст другой стат
// при следующем попадании. Дерево t отдаёт вызывающий, больше им не пользуясь.
func (c *parsedLooks) put(key, path string, st os.FileInfo, t model.AlbumTpl) {
	if st.Size() > lookCacheBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.m[key]; ok {
		c.remove(el)
	}
	if c.m == nil {
		c.m, c.ll = map[string]*list.Element{}, list.New()
	}
	c.m[key] = c.ll.PushFront(&lookCacheEnt{key: key, path: path, size: st.Size(), mod: st.ModTime(), tpl: t})
	c.bytes += st.Size()
	for c.ll.Len() > lookCacheMax || c.bytes > lookCacheBytes {
		c.remove(c.ll.Back())
	}
}

func (c *parsedLooks) drop(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.m[key]; ok {
		c.remove(el)
	}
}

func (c *parsedLooks) remove(el *list.Element) {
	e := c.ll.Remove(el).(*lookCacheEnt)
	delete(c.m, e.key)
	c.bytes -= e.size
}

// cloneLook — глубокая копия макета: кэш не отдаёт общие срезы и указатели (Find правит зажимом,
// плитка — шаблонами шагов).
func cloneLook(t model.AlbumTpl) model.AlbumTpl {
	v := reflect.ValueOf(&t).Elem()
	deepCopy(v)
	return t
}

// deepCopy заменяет указатели, срезы и словари внутри v (адресуемого) их копиями. Неэкспортируемые
// поля остаются как есть (в дереве макета их нет — проверяет TestCloneLookDeep).
func deepCopy(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		n := reflect.New(v.Type().Elem())
		n.Elem().Set(v.Elem())
		deepCopy(n.Elem())
		v.Set(n)
	case reflect.Slice:
		if v.IsNil() {
			return
		}
		n := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		reflect.Copy(n, v)
		for i := range n.Len() {
			deepCopy(n.Index(i))
		}
		v.Set(n)
	case reflect.Map:
		if v.IsNil() {
			return
		}
		n := reflect.MakeMapWithSize(v.Type(), v.Len())
		for it := v.MapRange(); it.Next(); {
			val := reflect.New(v.Type().Elem()).Elem()
			val.Set(it.Value())
			deepCopy(val)
			n.SetMapIndex(it.Key(), val)
		}
		v.Set(n)
	case reflect.Array:
		for i := range v.Len() {
			deepCopy(v.Index(i))
		}
	case reflect.Struct:
		for i := range v.NumField() {
			if f := v.Field(i); f.CanSet() {
				deepCopy(f)
			}
		}
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		n := reflect.New(v.Elem().Type()).Elem()
		n.Set(v.Elem())
		deepCopy(n)
		v.Set(n)
	}
}
