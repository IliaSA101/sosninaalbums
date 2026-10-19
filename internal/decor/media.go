package decor

import (
	"bytes"
	"container/list"
	"image"
	"image/png"
	"log"
	"net/http"
	"net/url"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Отдача картинок библиотеки (design §5.5); подключает App.media по префиксам GenPath, LibPath.
// Из запроса в файловую систему идёт только ключ из 16 hex, и то лишь если он есть в реестре.

const (
	// GenPath — превью узоров и стикеров программы (URL строит gen.Query).
	GenPath = "/media/gen/"
	// LibPath — свои плитки (t) и превью (p): /media/lib/<t|p>/<16 hex>.
	LibPath = "/media/lib/"
	// mediaCacheMax — PNG превью в памяти: плитки окна и узоры холста при прокрутке.
	mediaCacheMax = 64 << 20
)

// genSem — одновременных рисований: картинка 4096² — 64 МБ RGBA, больше четырёх — сотни МБ.
var genSem = make(chan struct{}, min(max(1, runtime.NumCPU()/2), 4))

// Handler — обработчик GenPath и LibPath; cache — миниатюры App (nil — /media/lib отвечает 404).
// Мусор, чужой путь и лишние параметры — 404; паника пишется в лог, ответ — 404.
func (s *Store) Handler(cache *library.Cache) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Ответ пишется только после рисования: паника до него — чистый 404.
		defer func() {
			if v := recover(); v != nil {
				log.Printf("ПАНИКА в media decor: %v\n%s", v, debug.Stack())
				http.NotFound(w, r)
			}
		}()
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, GenPath):
			s.serveGen(w, r)
		case strings.HasPrefix(r.URL.Path, LibPath):
			s.serveLib(w, r, cache)
		default:
			http.NotFound(w, r)
		}
	})
}

// guard — защита ответа /media: ответ пишется только после рисования, поэтому паника до него —
// чистый 404, а в лог идёт стек. Зовётся через defer в Handler и ServeFramePic.
func guard(w http.ResponseWriter, r *http.Request) {
	if v := recover(); v != nil {
		log.Printf("ПАНИКА в media decor: %v\n%s", v, debug.Stack())
		http.NotFound(w, r)
	}
}

// serveGen — /media/gen/pat|art|frame: разбор и зажим — gen.Parse, gen.ParseFrame (px 16..4096,
// картинка ≤ 4096²).
func (s *Store) serveGen(w http.ResponseWriter, r *http.Request) {
	var draw func() image.Image
	if r.URL.Path == gen.FramePath {
		req, ok := gen.ParseFrame(r.URL.RawQuery)
		if !ok {
			http.NotFound(w, r)
			return
		}
		draw = func() image.Image { return drawFrame(req) }
	} else {
		req, ok := gen.Parse(r.URL.Path, r.URL.RawQuery)
		if !ok {
			http.NotFound(w, r)
			return
		}
		draw = func() image.Image { return drawGen(req) }
	}
	key := r.URL.Path + "?" + r.URL.RawQuery
	data, hit := s.media.get(key)
	if !hit {
		select {
		case genSem <- struct{}{}:
		case <-r.Context().Done(): // окно ушло со страницы, пока ждали слот
			return
		}
		var err error
		data, err = drawPNG(draw)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		s.media.put(key, data)
	}
	w.Header().Set("Content-Type", "image/png")
	// Генератор заморожен (решение №90): ответ по тому же адресу не меняется.
	w.Header().Set("Cache-Control", "max-age=31536000, immutable")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
}

// drawPNG рисует и кодирует ответ в занятом слоте genSem; слот освобождается и при панике
// генератора (её ловит guard) — иначе после стольких паник, сколько слотов, /media/gen висел бы.
func drawPNG(draw func() image.Image) ([]byte, error) {
	defer func() { <-genSem }()
	return encodePNG(draw())
}

// encodePNG — PNG быстрым сжатием: превью живёт в памяти и на экране, не в файле.
func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// drawGen, drawFrame — рисование ответа (var: тест подставляет паникующий генератор).
var (
	drawGen   = func(req gen.Request) image.Image { return req.Draw() }
	drawFrame = func(req gen.FrameRequest) image.Image { return req.Draw() }
)

// serveLib — /media/lib/<t|p>/<16 hex>[?v=…&px=…]: ключ по грамматике и только из реестра.
func (s *Store) serveLib(w http.ResponseWriter, r *http.Request, cache *library.Cache) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, LibPath), "/")
	if cache == nil || len(parts) != 2 || (parts[0] != "t" && parts[0] != "p") || !libQueryOK(r.URL.RawQuery) {
		http.NotFound(w, r)
		return
	}
	path, ok := s.registered(libPrefix + parts[1])
	if !ok {
		http.NotFound(w, r)
		return
	}
	info, err := library.ReadInfo(path) // цвет: от него ключ кэша и пересчёт Adobe RGB
	if err != nil {
		http.NotFound(w, r)
		return
	}
	file, err := cache.File(info, parts[0] == "p")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	ct := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(file), ".png") {
		ct = "image/png"
	}
	w.Header().Set("Content-Type", ct)
	// Не immutable: файл под ключом тот же, но кэш миниатюр могут почистить.
	w.Header().Set("Cache-Control", "max-age=86400")
	http.ServeFile(w, r, file) //nolint:gosec // file — путь из кэша миниатюр по файлу реестра, не из запроса
}

// libQueryOK — у /media/lib допустимы только v и px (UI дописывает &px= ко всем плиткам).
func libQueryOK(raw string) bool {
	vals, err := url.ParseQuery(raw)
	if err != nil {
		return false
	}
	for k, v := range vals {
		if len(v) != 1 {
			return false
		}
		switch k {
		case "v":
			if _, err := strconv.ParseUint(v[0], 10, 16); err != nil {
				return false
			}
		case "px":
			if n, err := strconv.Atoi(v[0]); err != nil || n < gen.MinPx || n > gen.MaxPx {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// registered — файл своего, если ключ есть в реестре (любого вида, в том числе скрытый: он
// может стоять в альбоме) или в наборе (картинка, не шаблон) и файл найден сканом или импортом.
func (s *Store) registered(key string) (string, bool) {
	if !model.ValidLibKey(key) {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.entries {
		if e.Key != key || e.Kind == KindSpread { // шаблон — не картинка: его плитка в /media/tpl
			continue
		}
		if p, ok := s.files[e.Kind][key]; ok {
			return p, true
		}
	}
	for _, kind := range []string{KindBg, KindSticker, KindFrame} {
		if p, ok := s.files[kind][key]; ok && s.inPack(kind, key) {
			return p, true
		}
	}
	return "", false
}

// mediaCache — LRU готовых PNG по адресу запроса, предел по байтам.
type mediaCache struct {
	mu    sync.Mutex
	limit int
	size  int
	order *list.List // фронт — свежие
	byKey map[string]*list.Element
}

type mediaEntry struct {
	key  string
	data []byte
}

func newMediaCache(limit int) *mediaCache {
	return &mediaCache{limit: limit, order: list.New(), byKey: map[string]*list.Element{}}
}

func (c *mediaCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.byKey[key]
	if !ok {
		return nil, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*mediaEntry).data, true
}

func (c *mediaCache) put(key string, data []byte) {
	if len(data) > c.limit/4 { // одна огромная картинка не выталкивает все плитки
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.byKey[key]; ok {
		return
	}
	c.byKey[key] = c.order.PushFront(&mediaEntry{key, data})
	c.size += len(data)
	for c.size > c.limit {
		el := c.order.Back()
		me := el.Value.(*mediaEntry)
		c.order.Remove(el)
		delete(c.byKey, me.key)
		c.size -= len(me.data)
	}
}
