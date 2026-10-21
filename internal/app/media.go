package app

import (
	"bytes"
	"errors"
	"io/fs"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Раздача /media: миниатюры, шрифты и прочее содержимое окна.

// readColor — imgfile.ReadColor; подменяется в тестах (/media не должен читать недостающие).
var readColor = imgfile.ReadColor

// media отдаёт миниатюры: /media/t/<id> и /media/p/<id>; рамочки-картинки — /media/f/<id>
// (serveFramePic); шрифты — /media/font/<файл>;
// библиотека фонов — /media/gen/*, /media/lib/* (serveDecor), плитки разворотов — /media/tpl/*
// (serveTpl); разметки — /media/cover-guide и
// /media/spread-guide (serveMarkup).
func (a *App) media(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/media/") {
			next.ServeHTTP(w, r)
			return
		}
		defer guard("media")
		if name, ok := strings.CutPrefix(r.URL.Path, "/media/font/"); ok {
			serveFont(w, r, name)
			return
		}
		if a.serveMarkup(w, r) {
			return
		}
		if a.serveDecor(w, r) || a.serveTpl(w, r) {
			return
		}
		// До общего разбора /media/<t|p>/<id>: иначе «f» ушло бы в миниатюру.
		if id, ok := strings.CutPrefix(r.URL.Path, "/media/f/"); ok {
			a.serveFramePic(w, r, id)
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/media/"), "/")
		if len(parts) != 2 || a.cache == nil {
			http.NotFound(w, r)
			return
		}
		p, ok, gone := a.mediaPhoto(parts[1])
		if gone {
			http.NotFound(w, r) // пропавшее и прежнего альбома — не промах, в журнал не пишем
			return
		}
		if !ok {
			// Путь запроса в лог не пишем: промахов бывают сотни подряд.
			logRare(a.mediaMiss.Add(1), "media: фото не найдено")
			http.NotFound(w, r)
			return
		}
		a.serveThumb(w, r, p, parts[0] == "p")
	})
}

// mediaPhoto — фото для /media/<t|p>/<id>: из реестра открытых папок, иначе из проекта, чья папка
// сейчас не открыта. gone — файла нет (или том не отвечает, и у фото ленты тоже): диск не трогаем;
// и фото прежнего реестра, которого нет в альбоме (запрос плитки прежнего альбома после смены).
func (a *App) mediaPhoto(id string) (p library.Photo, ok, gone bool) {
	a.mu.Lock()
	p, ok = a.byID[id]
	_, gone = slices.BinarySearch(a.missing, id) // missingOf отдаёт по порядку
	_, old := a.mediaSt.old[id]
	a.mu.Unlock()
	if ok {
		// Фото ленты из папки недоступного тома (088-4): плитка сразу серая, обработчик не ждёт SMB.
		if !reachable(p.Path) {
			return p, false, true
		}
		return p, true, false
	}
	if gone {
		// Не читаем, иначе обработчик ждёт таймаута SMB.
		return p, false, true
	}
	ref, found := a.editor().Ref(id)
	if !found && old {
		return p, false, true
	}
	if !found || paths.CheckLocalPath(ref.Path) != nil || !reachable(ref.Path) {
		return p, false, false
	}
	col, prof := readColor(ref.Path) // от цвета зависит ключ кэша и пересчёт
	return library.Photo{ID: id, Path: ref.Path, Name: filepath.Base(ref.Path), Color: col, Profile: prof}, true, false
}

// mediaState — снятые запросы миниатюр и прежний реестр фото (090 G2).
type mediaState struct {
	// cancelled — запросы /media/t|p, снятые клиентом (плитка ушла с экрана до ответа); их число
	// пишет строка журнала прогрева: в exe так видно, снимает ли WebView2 запросы.
	cancelled atomic.Int64
	// old — ID, убранные из реестра прежними rescan, под a.mu: браузер ещё досылает из очереди
	// плитки прежнего альбома после «Новый» или «Открыть» — это не промах, журнал не засоряем.
	old map[string]struct{}
}

// oldIDsMax — предел прежнего реестра: дальше он начинается заново (память при сотнях смен).
const oldIDsMax = 100_000

// forget запоминает ID из prev, которых нет в next; вернувшиеся в реестр вычёркивает. Под a.mu.
func (m *mediaState) forget(prev, next map[string]library.Photo) {
	if m.old == nil || len(m.old)+len(prev) > oldIDsMax {
		m.old = make(map[string]struct{}, len(prev))
	}
	for id := range prev {
		if _, ok := next[id]; !ok {
			m.old[id] = struct{}{}
		}
	}
	for id := range next {
		delete(m.old, id)
	}
}

// mediaCancelled — снятые клиентом запросы миниатюр: фото ленты и картинки библиотеки.
func (a *App) mediaCancelled() int64 {
	n := a.mediaSt.cancelled.Load()
	if s := a.decorLib.Load(); s != nil {
		n += s.Cancelled()
	}
	return n
}

// serveThumb отдаёт файл миниатюры (или превью) из кэша; пропавший файл — 404 без записи в журнал.
// Запрос, снятый клиентом, ответа не получает и в журнал не пишется: начатая генерация остаётся в
// кэше, ждавший место — не генерирует (090-D4).
func (a *App) serveThumb(w http.ResponseWriter, r *http.Request, p library.Photo, preview bool) {
	ctx := r.Context()
	file, err := a.cache.FileCtx(ctx, p, preview)
	if ctx.Err() != nil { // и когда генерация успела доделаться: отвечать некому
		a.mediaSt.cancelled.Add(1)
		return
	}
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) { // файл пропал — не сбой, хватит 404
			logRare(a.mediaMiss.Add(1), "превью: %v", paths.NoPath(err))
		}
		http.NotFound(w, r)
		return
	}
	// Тип задаём сами: копия бывает PNG (прозрачные картинки), а реестровый тип Windows ненадёжен.
	ct := "image/jpeg"
	if strings.EqualFold(filepath.Ext(file), ".png") {
		ct = "image/png"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "max-age=31536000, immutable")
	http.ServeFile(w, r, file) //nolint:gosec // file — путь из кэша миниатюр по ID из реестра, не из запроса
}

// serveFramePic — накладка и маска рамочки-картинки места (079 design §5, 079-22, 079-35). ID —
// только запись под рамочкой места (Editor.BorderIDs, затем BorderPath): обычное фото альбома так
// не нарисовать.
// Пропавшая — 404 без диска; путь — CheckLocalPath и reachable до любого Stat (как /media/p:
// UNC и мёртвый сетевой том не вешают обработчик). Разбор запроса и рисунок — decor.
func (a *App) serveFramePic(w http.ResponseWriter, r *http.Request, raw string) {
	id, known := knownID(a.editor().BorderIDs(), raw)
	a.mu.Lock()
	_, gone := slices.BinarySearch(a.missing, id)
	a.mu.Unlock()
	if !known || gone {
		http.NotFound(w, r)
		return
	}
	path, ok := a.editor().BorderPath(id)
	if !ok || paths.CheckLocalPath(path) != nil || !reachable(path) {
		logRare(a.mediaMiss.Add(1), "media: рамочка не найдена")
		http.NotFound(w, r)
		return
	}
	decor.ServeFramePic(w, r, path)
}

// knownID — ID из ids, равный raw из адреса (079-40): в Editor и к файлу идёт строка альбома, а не
// запроса — поток данных из URL обрывается здесь (gosec G703 видел его до os.Stat).
func knownID(ids []string, raw string) (string, bool) {
	for _, id := range ids {
		if id == raw {
			return id, true
		}
	}
	return "", false
}

// serveFont отдаёт встроенный шрифт или своё начертание «<16 hex>.<ttf|otf>» зарегистрированной
// гарнитуры (082). Имя ищется в белом списке text.FontFile: путь из запроса в файловую систему не
// попадает. Встроенные не immutable: файлы могут смениться с версией, а кэш WebView2 переживает
// обновление; своё — immutable: имя — хэш содержимого.
func serveFont(w http.ResponseWriter, r *http.Request, name string) {
	data, ok := text.FontFile(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "font/ttf")
	w.Header().Set("X-Content-Type-Options", "nosniff") // своё начертание — недоверенные байты
	w.Header().Set("Cache-Control", "max-age=86400")
	if faceFile.MatchString(name) {
		if strings.HasSuffix(name, ".otf") {
			w.Header().Set("Content-Type", "font/otf")
		}
		w.Header().Set("Cache-Control", "max-age=31536000, immutable")
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
