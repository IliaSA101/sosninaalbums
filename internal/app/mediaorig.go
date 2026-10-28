package app

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// origMaxPixels — предел кадра для «как есть» (128 ступень 2): WebView держит декод картинки в
// RGBA (4 байта на пиксель), 100 Мп ≈ 400 МБ; imgfile.MaxPixels (250 Мп) для окна слишком много.
const origMaxPixels = 100_000_000

// origMaxBytes — предел размера файла в байтах. В exe (WebView2) Wails не отдаёт тело потоком:
// ответ копится в bytes.Buffer и копируется в поток WebView2 (assetserver/webview, responsewriter_windows),
// то есть пик памяти Go — 2–3 размера файла. JPEG 100 Мп весит 30–100 МБ, 8-битный PNG 100 Мп —
// до 200 МБ; 16-битный PNG такого кадра (300 МБ и больше) остаётся на превью. Переменная, чтобы
// тест проверил границу без файла в 200 МБ.
var origMaxBytes int64 = 200 << 20

// origSlots — одновременные отдачи оригинала: по одной, иначе быстрое «←→» в exe (отмены запросов
// там нет) складывает в память несколько буферов по сотне мегабайт. Лишние ждут слот.
var origSlots = make(chan struct{}, 1)

// sniffLen — сколько байт читает http.DetectContentType.
const sniffLen = 512

// serveOriginal — GET|HEAD /media/o/<id>: файл фото как есть, для зума окна выше 1:1 по превью
// (128 design §2.2). Путь — только из реестра или проекта по ID (mediaPhoto), из адреса он не
// берётся. Перекодировки и декода в Go нет, но на WebView2 тело копится в памяти целиком — отсюда
// предел размера файла и один слот отдачи (origMaxBytes, origSlots).
// Коды: 405 — не GET/HEAD; 404 — нет фото или файла (пропавшее — без диска); 415 — цвет не
// none/srgb (WebView и проект пересчитывают профили по-разному), кадр больше 100 Мп, тип не
// JPEG/PNG/WebP, файл больше origMaxBytes; иначе 200. Условные запросы (304) и Range не
// поддерживаются: Wails на WebView2 превращает 304 в 500 с записью в журнал, поэтому заголовки
// условий снимаются, ответ — всегда 200 с телом и Cache-Control: no-store.
func (a *App) serveOriginal(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}
	p, ok, gone := a.mediaPhoto(id)
	if gone || !ok {
		if !gone { // пропавшее и прежнего альбома — не промах, в журнал не пишем
			logRare(a.mediaMiss.Add(1), "media: оригинал не найден")
		}
		http.NotFound(w, r)
		return
	}
	// Цвет и размеры реестра проверяются до открытия файла: отказ без диска.
	if p.Color != imgfile.ColorNone && p.Color != imgfile.ColorSRGB || int64(p.W)*int64(p.H) > origMaxPixels {
		http.Error(w, "оригинал не отдаётся", http.StatusUnsupportedMediaType)
		return
	}
	// mediaPhoto для фото ленты путь не проверяет (он из сканера): вторая линия границы путей.
	if paths.CheckLocalPath(p.Path) != nil {
		http.NotFound(w, r)
		return
	}
	// Слот отдачи: ждём с учётом клиента, ушедшему ответ не нужен.
	select {
	case origSlots <- struct{}{}:
		defer func() { <-origSlots }()
	case <-r.Context().Done():
		return
	}
	// library.OpenLocal, не os.Open/ServeFile: файл пользователя; у ссылки проверяется цель, FIFO
	// и устройство не открываются (без ожидания писателя), os.SameFile ловит подмену после проверки.
	f, st, err := library.OpenLocal(p.Path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) { // файл пропал — не сбой, хватит 404
			logRare(a.mediaMiss.Add(1), "оригинал: %v", paths.NoPath(err))
		}
		http.NotFound(w, r)
		return
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	if st.Size() > origMaxBytes {
		http.Error(w, "оригинал не отдаётся", http.StatusUnsupportedMediaType)
		return
	}
	ct, code := origKind(f)
	if code != 0 {
		http.Error(w, "оригинал не отдаётся", code)
		return
	}
	// Условные запросы снимаем: 304 на WebView2 Wails превращает в 500 (аудит 128). Range тоже —
	// картинке он не нужен, а ответ 206 копился бы так же целиком.
	for _, k := range []string{"If-Modified-Since", "If-None-Match", "If-Range", "If-Match", "If-Unmodified-Since", "Range"} {
		r.Header.Del(k)
	}
	h := w.Header()
	h.Set("Content-Type", ct)
	h.Set("X-Content-Type-Options", "nosniff") // недоверенные байты: браузер не угадывает тип
	h.Set("Cache-Control", "no-store")         // сверки по дате нет (304 нельзя), файл могли заменить
	// Имя пустое: тип уже задан; ServeContent даёт Last-Modified и HEAD без тела.
	http.ServeContent(w, r, "", st.ModTime(), f)
}

// originalID — ID из адреса /media/o/<id>; false — адрес другой.
func originalID(urlPath string) (string, bool) {
	return strings.CutPrefix(urlPath, "/media/o/")
}

// origKind — тип по содержимому и код отказа (0 — можно отдавать). Указатель файла в конце — в начале.
func origKind(f *os.File) (ct string, code int) {
	// Тип — по содержимому, не по расширению (WebP бывает под .jpg, 125).
	head := make([]byte, sniffLen)
	n, err := io.ReadFull(f, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", http.StatusNotFound
	}
	ct = http.DetectContentType(head[:n])
	if ct != "image/jpeg" && ct != "image/png" && ct != "image/webp" {
		return "", http.StatusUnsupportedMediaType
	}
	// Размер по заголовку самого файла: у фото проекта W и H в реестре нет, у фото ленты они могли
	// устареть (файл заменили). Битый или усечённый заголовок и анимация — тоже отказ.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", http.StatusNotFound
	}
	if _, err := imgfile.DimsOf(f, origMaxPixels); err != nil {
		return "", http.StatusUnsupportedMediaType
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", http.StatusNotFound
	}
	return ct, 0
}
