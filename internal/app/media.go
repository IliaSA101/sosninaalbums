package app

import (
	"bytes"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
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
		a.mu.Lock()
		p, ok := a.byID[parts[1]]
		_, gone := slices.BinarySearch(a.missing, parts[1]) // missingOf отдаёт по порядку
		a.mu.Unlock()
		if !ok && gone {
			// Файла нет (или том не отвечает): не читаем, иначе обработчик ждёт таймаута SMB.
			http.NotFound(w, r)
			return
		}
		if !ok {
			// Фото из проекта, чья папка сейчас не открыта.
			if ref, found := a.editor().Ref(parts[1]); found && paths.CheckLocalPath(ref.Path) == nil && reachable(ref.Path) {
				col, prof := readColor(ref.Path) // от цвета зависит ключ кэша и пересчёт
				p, ok = library.Photo{ID: parts[1], Path: ref.Path, Name: filepath.Base(ref.Path), Color: col, Profile: prof}, true
			}
		}
		if !ok {
			// Путь запроса в лог не пишем: промахов бывают сотни подряд.
			logRare(a.mediaMiss.Add(1), "media: фото не найдено")
			http.NotFound(w, r)
			return
		}
		file, err := a.cache.File(p, parts[0] == "p")
		if err != nil {
			logRare(a.mediaMiss.Add(1), "превью: %v", paths.NoPath(err))
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
	})
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

// serveFont отдаёт встроенный шрифт. Имя ищется в белом списке text.FontFile:
// путь из запроса в файловую систему не попадает. Не immutable: файлы могут
// смениться с версией, а кэш WebView2 переживает обновление.
func serveFont(w http.ResponseWriter, r *http.Request, name string) {
	data, ok := text.FontFile(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "font/ttf")
	w.Header().Set("Cache-Control", "max-age=86400")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
