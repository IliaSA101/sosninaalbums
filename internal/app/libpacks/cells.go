package libpacks

import (
	"bytes"
	"container/list"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// Клетки листа образцов (спека 144, design §5, security Н6–Н8, требования 7–8): лист разбирает
// только Go (baseline YCbCr/Gray, размеры до Decode), WebView получает перекодированные клетки.

const (
	cellBudget   = 8 << 20 // байт закодированных клеток в LRU
	cellsMax     = 2 << 20 // клетки одного листа больше — лист отвергается
	cellQuality  = 90
	sheetMaxArea = 4 << 20 // пикселей листа: оценка памяти Decode (как в fetch)
	maxMarkers   = 64      // сегментов с длиной до SOS включительно
)

var (
	// ErrNoCell — клетки нет: набор или ряд не найден, лист не лежит на диске или не годится.
	ErrNoCell = errors.New("образец недоступен")
	errSheet  = errors.New("лист образцов не годится")
)

// cellEntry — разобранный лист в LRU: клетки по рядам.
type cellEntry struct {
	key  string
	rows [][][]byte
	size int
}

// cellCache — LRU закодированных клеток, слот разбора и плохие листы. Ключ — sha256 листа и
// раскладка (w, h, ряды): правка rows при тех же байтах и общий лист двух наборов не отдают чужих
// клеток (Н6, Н8).
type cellCache struct {
	mu    sync.Mutex
	items map[string]*list.Element
	order *list.List // front — свежие
	bytes int
	bad   map[string]bool
	slot  chan struct{} // один разбор за раз; запрос ушедшей с экрана плитки не ждёт (select по ctx)
}

func newCellCache() *cellCache {
	return &cellCache{items: map[string]*list.Element{}, order: list.New(), bad: map[string]bool{}, slot: make(chan struct{}, 1)}
}

// layoutKey — sha256 листа и его раскладка в сериализованном виде.
func layoutKey(p *fetch.Preview) string {
	var b strings.Builder
	b.WriteString(p.SHA256)
	b.WriteString("|" + strconv.Itoa(p.W) + "x" + strconv.Itoa(p.H))
	for _, r := range p.Rows {
		b.WriteString("|" + r.Kind + ":" + strconv.Itoa(r.W) + "x" + strconv.Itoa(r.H))
		for _, it := range r.Items {
			b.WriteString("," + it)
		}
	}
	return b.String()
}

// get — клетка row/i из LRU; индекс сверяется с длиной закэшированного ряда.
func (c *cellCache) get(key string, row, i int) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	e := el.Value.(*cellEntry)
	if row < 0 || row >= len(e.rows) || i < 0 || i >= len(e.rows[row]) {
		return nil, false
	}
	c.order.MoveToFront(el)
	return e.rows[row][i], true
}

func (c *cellCache) put(e *cellEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if old, ok := c.items[e.key]; ok {
		c.bytes -= old.Value.(*cellEntry).size
		c.order.Remove(old)
	}
	c.items[e.key] = c.order.PushFront(e)
	c.bytes += e.size
	for c.bytes > cellBudget && c.order.Len() > 1 {
		last := c.order.Back()
		le := last.Value.(*cellEntry)
		c.bytes -= le.size
		delete(c.items, le.key)
		c.order.Remove(last)
	}
}

func (c *cellCache) isBad(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bad[key]
}

func (c *cellCache) markBad(key string) {
	c.mu.Lock()
	c.bad[key] = true
	c.mu.Unlock()
}

// clearBad — новый список: плохие листы пробуются заново (один раз на список).
func (c *cellCache) clearBad() {
	c.mu.Lock()
	clear(c.bad)
	c.mu.Unlock()
}

// sheetBad — лист отвергнут разбором (не baseline, не те размеры, клетки слишком велики): плиток
// такого листа нет.
func (s *Service) sheetBad(p *fetch.Preview) bool { return s.cells.isBad(layoutKey(p)) }

// Cell — закодированная клетка (JPEG) i ряда kind набора packID по сохранённому списку; сети нет.
// ErrNoCell — набора, ряда, индекса или годного листа нет; ctx.Err() — запрос ушёл, пока ждал слот.
func (s *Service) Cell(ctx context.Context, packID, kind string, i int) ([]byte, error) {
	cat, _, _ := s.cached()
	if cat == nil || !fetch.ValidID(packID) {
		return nil, ErrNoCell
	}
	for _, e := range cat.Packs { // общий каталог: только чтение
		if e.ID == packID && e.Preview != nil {
			return s.cellOf(ctx, packID, e.Preview, kind, i)
		}
	}
	return nil, ErrNoCell
}

// cellOf — клетка листа p набора id (id — для журнала): LRU, иначе разбор листа под слотом (второй взгляд в LRU: 24 плитки
// одного листа — один разбор).
func (s *Service) cellOf(ctx context.Context, id string, p *fetch.Preview, kind string, i int) ([]byte, error) {
	row := -1
	for r := range p.Rows {
		if p.Rows[r].Kind == kind {
			row = r
		}
	}
	if row < 0 || i < 0 || i >= len(p.Rows[row].Items) {
		return nil, ErrNoCell
	}
	key := layoutKey(p)
	if b, ok := s.cells.get(key, row, i); ok {
		return b, nil
	}
	if s.cells.isBad(key) {
		return nil, ErrNoCell
	}
	select {
	case s.cells.slot <- struct{}{}:
		defer func() { <-s.cells.slot }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if b, ok := s.cells.get(key, row, i); ok {
		return b, nil
	}
	if s.cells.isBad(key) {
		return nil, ErrNoCell
	}
	data, err := s.readPreview(p)
	if err != nil {
		return nil, ErrNoCell // листа нет: не «плохой» — его докачает задание
	}
	rows, err := s.splitSheet(data, p)
	if err != nil {
		s.cells.markBad(key)
		// Без общего счётчика prevLog (иначе строка может не попасть в log.txt): markBad даёт раз на ключ и список.
		log.Printf("библиотека: образцы — лист набора %s: %v", id, err)
		return nil, ErrNoCell
	}
	e := &cellEntry{key: key, rows: rows}
	for _, r := range rows {
		for _, c := range r {
			e.size += len(c)
		}
	}
	s.cells.put(e)
	return rows[row][i], nil
}

// sheetWhy — errSheet с причиной для журнала (ни адресов, ни путей).
func sheetWhy(why string) error { return fmt.Errorf("%w: %s", errSheet, why) }

// splitSheet — проверка вида JPEG до Decode, разбор и нарезка клеток (кодирование q90 в память).
// Любое отклонение — errSheet с причиной, без паники.
func (s *Service) splitSheet(data []byte, p *fetch.Preview) ([][][]byte, error) {
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, sheetWhy("заголовок не читается")
	}
	if cfg.Width != p.W || cfg.Height != p.H || cfg.Width < 1 || cfg.Height < 1 || cfg.Width*cfg.Height > sheetMaxArea {
		return nil, sheetWhy("размеры не те")
	}
	if cfg.ColorModel != color.YCbCrModel && cfg.ColorModel != color.GrayModel {
		return nil, sheetWhy("не YCbCr и не серый") // CMYK/YCCK: applyBlack и память до ≈ 100 МБ
	}
	if !baselineFrame(data) {
		return nil, sheetWhy("не baseline") // прогрессивный: коэффициенты 256 байт на блок на компонент
	}
	s.decodes.Add(1)
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, sheetWhy("не разбирается")
	}
	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok || img.Bounds() != image.Rect(0, 0, p.W, p.H) {
		return nil, sheetWhy("границы не те")
	}
	rows := make([][][]byte, len(p.Rows))
	total, y := 0, 0
	for ri, r := range p.Rows {
		rows[ri] = make([][]byte, len(r.Items))
		for j := range r.Items {
			rect := image.Rect(j*r.W, y, (j+1)*r.W, y+r.H)
			if !rect.In(img.Bounds()) {
				return nil, sheetWhy("клетка вне листа")
			}
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, sub.SubImage(rect), &jpeg.Options{Quality: cellQuality}); err != nil {
				return nil, sheetWhy("клетка не кодируется")
			}
			if total += buf.Len(); total > cellsMax {
				return nil, sheetWhy("клетки больше 2 МБ")
			}
			rows[ri][j] = buf.Bytes()
		}
		y += r.H
	}
	return rows, nil
}

// baselineFrame — проход по маркерам до первого SOS: кадр один и это SOF0 или SOF1; не больше
// maxMarkers сегментов с длиной (SOF и SOS считаются), длины в пределах тела. Прочее (SOF2 и далее,
// арифметическое кодирование, конец до SOS, любой маркер без длины до SOS) — false без Decode.
func baselineFrame(data []byte) bool {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return false
	}
	pos, frames, segs := 2, 0, 0
	for {
		for pos < len(data) && data[pos] == 0xFF && pos+1 < len(data) && data[pos+1] == 0xFF {
			pos++ // заполнитель 0xFF перед маркером
		}
		if pos+2 > len(data) || data[pos] != 0xFF {
			return false
		}
		m := data[pos+1]
		pos += 2
		switch {
		case m == 0x00 || m == 0x01 || m >= 0xD0 && m <= 0xD8 || m == 0xD9:
			// Маркеры без длины (FF00, TEM, RSTn, SOI, EOI) до SOS: image/jpeg читает их не как
			// сегмент с длиной, и такой поток мог бы спрятать SOF2/SOS под «длиной» (ревью 144 Г2).
			return false
		}
		if pos+2 > len(data) {
			return false
		}
		if segs++; segs > maxMarkers {
			return false // SOF и SOS в счёте: 64 проходит, 65 — отказ (одно правило с library-pack.mjs)
		}
		n := int(binary.BigEndian.Uint16(data[pos:]))
		if n < 2 || pos+n > len(data) {
			return false
		}
		switch {
		case m == 0xC0 || m == 0xC1:
			if frames++; frames > 1 {
				return false
			}
		case m >= 0xC2 && m <= 0xCF && m != 0xC4 && m != 0xC8 && m != 0xCC:
			return false // прогрессивный, без потерь, арифметический, иерархический
		case m == 0xDA:
			return frames == 1
		}
		pos += n
	}
}
