package app

import (
	"bytes"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Раздача /media: миниатюры, шрифты и прочее содержимое окна.

// readColor — imgfile.ReadColor; подменяется в тестах (/media не должен читать недостающие).
var readColor = imgfile.ReadColor

// media отдаёт миниатюры: /media/t/<id> и /media/p/<id>; шрифты — /media/font/<файл>;
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
