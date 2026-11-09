package ownprint

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Разметка типографии (032, 066, 086): кэш превью PSD и больших JPEG/PNG и раздача файла разметки.
// Путь приходит от App только из проекта (из запроса его нет); диалог выбора, правка альбома и
// проверка тома остались в App (internal/app/markup.go): Wails, Editor и osx службе недоступны.

// maxGuideBytes — предел JPEG/PNG разметки до MarkupMaxSide для раздачи файлом: WebView
// декодирует его сам. Большие (по пикселям) идут через превью Go.
const maxGuideBytes = 100 << 20

// Слоты кэша превью в Markups.
const (
	MarkupCover = iota
	MarkupSpread
)

// Markups — кэш превью, по записи на вид разметки; нулевое значение готово к работе.
type Markups [2]markupCache

// Reset — забыть превью слота: следующий запрос строит его заново.
func (g *Markups) Reset(slot int) { g[slot].reset() }

// Serve — файл-разметка path из проекта (из запроса пути нет: ?r=<rev> лишь сбивает кэш
// WebView). Каждый раз заново: файл проходит MarkupFile. JPEG/PNG до 4096 px
// отдаются файлом, PSD, WebP и большие JPEG/PNG — превью Go из кэша слота (JPEG, PNG при прозрачности).
// Ошибка — 404 и строка в log.txt без пути. Проверка тома и договор «путь не из запроса» — у вызывающего.
func (g *Markups) Serve(w http.ResponseWriter, r *http.Request, slot int, path string) {
	m, err := library.MarkupFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-cache") // файл на диске могут заменить под тем же путём
	if !m.Big() {
		if m.Size > maxGuideBytes {
			http.NotFound(w, r)
			return
		}
		// Тип — по содержимому (Big уже увёл WebP): имя .jpg у PNG и наоборот не сбивает WebView.
		ct := "image/jpeg"
		if m.Format == "png" {
			ct = "image/png"
		}
		w.Header().Set("Content-Type", ct)
		http.ServeFile(w, r, m.Path) //nolint:gosec // m.Path — путь разметки из проекта после MarkupFile, не из запроса
		return
	}
	data, ct, err := g[slot].get(r.Context(), m.Path)
	if err == nil && len(data) == 0 {
		err = library.ErrNoMarkupImage
	}
	if err != nil {
		// PSD без сведённого — обычный случай (видны только направляющие): 404 без строки в
		// log.txt, иначе каждое обновление подложки засоряет журнал.
		// Запрос закрыт (подложку перерисовали, пока бюджет держал экспорт) — тоже без строки.
		if !errors.Is(err, library.ErrNoMarkupImage) && r.Context().Err() == nil {
			what := "превью разметки"
			if m.IsPSD() {
				what = "превью PSD"
			}
			log.Printf("разметка: %s не построено: %v", what, paths.NoPath(err))
		}
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", ct)
	_, _ = w.Write(data)
}

// MarkupPreview — построение превью (PSD, большой JPEG/PNG); подменяется только в тестах (счёт обращений).
var MarkupPreview = library.MarkupPreviewCtx

// markupKey — превью действительно, пока у файла те же путь, размер и время записи.
type markupKey struct {
	path string
	size int64
	mod  int64 // ModTime в нс: time.Time через == сравнивать нельзя
}

// markupCache — одна запись на вид разметки. Замок на время построения не держится (ожидание
// бюджета декода может быть долгим): второй запрос того же файла ждёт готовый результат на канале
// busy со своим контекстом, а не строит заново. Замок Editor здесь не берётся — путь уже прочитан.
type markupCache struct {
	mu    sync.Mutex
	key   markupKey
	data  []byte
	ctype string // Content-Type превью: PNG у PSD, JPEG или PNG у большой картинки
	err   error
	busy  chan struct{} // не nil — идёт построение busyK; закрывается по готовности
	busyK markupKey
	gen   int // растёт при reset: результат построения, начатого до сброса, не запоминается
}

// reset — забыть запись: следующий get строит превью заново.
func (c *markupCache) reset() {
	c.mu.Lock()
	c.key, c.data, c.ctype, c.err = markupKey{}, nil, "", nil
	c.gen++
	c.mu.Unlock()
}

// get — превью path и его Content-Type: из кэша, если файл не менялся, иначе заново (и ошибка
// тоже запоминается: битый файл не разбирается на каждый запрос). Отмена ctx (запрос закрыт)
// прерывает ожидание и не запоминается: следующий запрос строит превью заново.
func (c *markupCache) get(ctx context.Context, path string) ([]byte, string, error) {
	st, err := library.StatLocal(path)
	if err != nil {
		return nil, "", err
	}
	k := markupKey{path: path, size: st.Size(), mod: st.ModTime().UnixNano()}
	c.mu.Lock()
	for c.key != k && c.busy != nil && c.busyK == k {
		wait := c.busy
		c.mu.Unlock()
		select {
		case <-wait:
		case <-ctx.Done():
			return nil, "", ctx.Err()
		}
		c.mu.Lock()
	}
	if c.key == k {
		defer c.mu.Unlock()
		return c.data, c.ctype, c.err
	}
	done, gen := make(chan struct{}), c.gen
	c.busy, c.busyK = done, k
	c.mu.Unlock()

	data, ct, err := MarkupPreview(ctx, path)

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.busy == done {
		c.busy = nil
	}
	close(done)
	if c.gen == gen && (err == nil || ctx.Err() == nil) {
		c.key, c.data, c.ctype, c.err = k, data, ct, err
	}
	return data, ct, err
}
