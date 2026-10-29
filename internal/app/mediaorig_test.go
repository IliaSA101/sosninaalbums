package app

// 128-13: /media/o/<id> — оригинал фото как есть для зума выше 1:1 (G2).

import (
	"bytes"
	"context"
	"image/color"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// colorRed — заливка тестовых PNG.
var colorRed = color.NRGBA{R: 200, A: 255}

// origApp — App без кэша миниатюр (оригиналу он не нужен) и dir для файлов.
func origApp(t *testing.T) (*App, string) {
	t.Helper()
	a := newApp(t)
	a.emit = func(string, ...any) {}
	return a, testkit.TempDir(t)
}

// origPhoto кладёт фото в реестр ленты с заявленными размерами и цветом.
func origPhoto(a *App, path string, w, h int, c imgfile.Color) string {
	p := library.Photo{ID: library.IDFor(path), Path: path, Name: filepath.Base(path), W: w, H: h, Color: c}
	a.mu.Lock()
	a.photos = append(a.photos, p)
	a.byID[p.ID] = p
	a.mu.Unlock()
	return p.ID
}

// origReq — запрос method path к media с заголовками hdr (пары имя, значение).
func origReq(a *App, method, path string, hdr ...string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(method, path, nil)
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Set(hdr[i], hdr[i+1])
	}
	a.media(http.NotFoundHandler()).ServeHTTP(rec, r)
	return rec
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Файл отдаётся байт в байт; тип — по содержимому (WebP под .jpg остаётся WebP), заголовки
// безопасности и no-cache; фото проекта (не из ленты) отдаётся так же.
func TestMediaOriginalAsIs(t *testing.T) {
	a, dir := origApp(t)
	jpg := testkit.JPEG(t, dir, "a.jpg", 60, 40)
	png := testkit.PNG(t, dir, "b.png", 30, 20, colorRed)
	webp := filepath.Join(dir, "c.jpg") // расширение лжёт
	if err := os.WriteFile(webp, readFile(t, filepath.Join("..", "imgfile", "testdata", "lossy16.webp")), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ path, ct string }{{jpg, "image/jpeg"}, {png, "image/png"}, {webp, "image/webp"}} {
		id := origPhoto(a, c.path, 60, 40, imgfile.ColorNone)
		rec := origReq(a, http.MethodGet, "/media/o/"+id)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: код %d", c.path, rec.Code)
		}
		if !bytes.Equal(rec.Body.Bytes(), readFile(t, c.path)) {
			t.Errorf("%s: байты не совпали с файлом", c.path)
		}
		h := rec.Header()
		if h.Get("Content-Type") != c.ct || h.Get("X-Content-Type-Options") != "nosniff" ||
			h.Get("Cache-Control") != "no-store" || h.Get("Last-Modified") == "" {
			t.Errorf("%s: заголовки %v", c.path, h)
		}
		if h.Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("%s: CORS не ставится: %v", c.path, h)
		}
	}
	// Range не поддерживается (на WebView2 тело копится целиком): ответ — полный файл, 200.
	id := origPhoto(a, jpg, 60, 40, imgfile.ColorSRGB)
	rec := origReq(a, http.MethodGet, "/media/o/"+id, "Range", "bytes=0-9")
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), readFile(t, jpg)) {
		t.Errorf("Range: %d, %d байт", rec.Code, rec.Body.Len())
	}

	// Фото проекта, чья папка не открыта: путь из проекта по ID, цвет — по файлу.
	pid := library.IDFor(png)
	_, err := a.editor().NewSpread(-1, []model.PhotoInfo{{ID: pid, Path: png, W: 30, H: 20}})
	if err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	delete(a.byID, pid)
	a.mu.Unlock()
	if rec := origReq(a, http.MethodGet, "/media/o/"+pid); rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), readFile(t, png)) {
		t.Errorf("фото проекта: %d", rec.Code)
	}
}

// 304 нельзя (аудит 128): Wails на WebView2 превращает его в 500 и пишет ошибку в журнал. Условные
// заголовки игнорируются — ответ 200 с телом и no-store.
func TestMediaOriginalNotModified(t *testing.T) {
	a, dir := origApp(t)
	path := testkit.JPEG(t, dir, "a.jpg", 60, 40)
	id := origPhoto(a, path, 60, 40, imgfile.ColorNone)
	first := origReq(a, http.MethodGet, "/media/o/"+id)
	lm := first.Header().Get("Last-Modified")
	if first.Code != http.StatusOK || lm == "" {
		t.Fatalf("первый: %d, %q", first.Code, lm)
	}
	for _, hdr := range [][]string{
		{"If-Modified-Since", lm},
		{"If-None-Match", `"x"`},
		{"If-Range", lm, "Range", "bytes=1-5"},
		{"If-Unmodified-Since", "Mon, 02 Jan 2006 15:04:05 GMT"},
		{"If-Match", `"x"`},
	} {
		rec := origReq(a, http.MethodGet, "/media/o/"+id, hdr...)
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), first.Body.Bytes()) {
			t.Errorf("%v: код %d, тело %d байт (ждали 200 и полный файл)", hdr, rec.Code, rec.Body.Len())
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%v: Cache-Control %q", hdr, rec.Header().Get("Cache-Control"))
		}
	}
}

// Новая дата файла — новые байты (замена файла под тем же путём), условный запрос не мешает.
func TestMediaOriginalNewMtime(t *testing.T) {
	a, dir := origApp(t)
	path := testkit.JPEG(t, dir, "a.jpg", 60, 40)
	id := origPhoto(a, path, 60, 40, imgfile.ColorNone)
	lm := origReq(a, http.MethodGet, "/media/o/"+id).Header().Get("Last-Modified")
	repl := testkit.JPEG(t, dir, "new.jpg", 80, 50)
	if err := os.Rename(repl, path); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	rec := origReq(a, http.MethodGet, "/media/o/"+id, "If-Modified-Since", lm)
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), readFile(t, path)) {
		t.Fatalf("после замены: %d", rec.Code)
	}
	if rec.Header().Get("Last-Modified") == lm {
		t.Error("дата ответа осталась прежней")
	}
}

// Цвет не none/srgb — 415 без открытия файла (путь вымышленный: открытие дало бы 404); для фото
// проекта цвет читается по файлу.
func TestMediaOriginalColor415(t *testing.T) {
	a, dir := origApp(t)
	path := testkit.JPEG(t, dir, "a.jpg", 60, 40)
	for _, c := range []imgfile.Color{imgfile.ColorAdobe, imgfile.ColorP3, imgfile.ColorICC, imgfile.ColorOther} {
		id := origPhoto(a, filepath.Join(dir, "нет-"+string(c)+".jpg"), 60, 40, c)
		if rec := origReq(a, http.MethodGet, "/media/o/"+id); rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("%s: код %d, ждали 415", c, rec.Code)
		}
	}
	for _, c := range []imgfile.Color{imgfile.ColorNone, imgfile.ColorSRGB} {
		id := origPhoto(a, path, 60, 40, c)
		if rec := origReq(a, http.MethodGet, "/media/o/"+id); rec.Code != http.StatusOK {
			t.Errorf("%s: код %d, ждали 200", c, rec.Code)
		}
	}
	// Фото проекта с Adobe: цвет даёт readColor.
	old := readColor
	t.Cleanup(func() { readColor = old })
	readColor = func(string) (imgfile.Color, string) { return imgfile.ColorAdobe, "Adobe RGB" }
	png := testkit.PNG(t, dir, "b.png", 30, 20, colorRed)
	pid := library.IDFor(png)
	if _, err := a.editor().NewSpread(-1, []model.PhotoInfo{{ID: pid, Path: png, W: 30, H: 20}}); err != nil {
		t.Fatal(err)
	}
	if rec := origReq(a, http.MethodGet, "/media/o/"+pid); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("проект, Adobe: код %d", rec.Code)
	}
}

// Больше 100 Мп — 415: по размерам реестра (без диска) и по заголовку самого файла (реестр
// устарел или у фото проекта размеров нет). Ровно 100 Мп проходит.
func TestMediaOriginalPixels415(t *testing.T) {
	a, dir := origApp(t)
	small := testkit.JPEG(t, dir, "a.jpg", 60, 40)
	for _, c := range []struct {
		w, h int
		code int
	}{{10000, 10000, 200}, {10001, 10000, 415}, {20000, 20000, 415}, {1 << 31, 1 << 31, 415}} {
		id := origPhoto(a, small, c.w, c.h, imgfile.ColorNone)
		if rec := origReq(a, http.MethodGet, "/media/o/"+id); rec.Code != c.code {
			t.Errorf("реестр %d×%d: код %d, ждали %d", c.w, c.h, rec.Code, c.code)
		}
	}
	// Заголовок файла: огромный кадр при мелких размерах в реестре.
	for _, c := range []struct {
		w, h int
		code int
	}{{10000, 10000, 200}, {10001, 10000, 415}, {12000, 12000, 415}} {
		path := testkit.HugeJPEG(t, dir, "h"+strconv.Itoa(c.w)+"x"+strconv.Itoa(c.h)+".jpg", c.w, c.h)
		id := origPhoto(a, path, 60, 40, imgfile.ColorNone)
		if rec := origReq(a, http.MethodGet, "/media/o/"+id); rec.Code != c.code {
			t.Errorf("заголовок %d×%d: код %d, ждали %d", c.w, c.h, rec.Code, c.code)
		}
	}
}

// Не JPEG/PNG/WebP и битые входы — 415, а не паника и не отдача: GIF, текст и HTML под .jpg,
// пустой и обрезанный файл, усечённые заголовки, анимированный WebP.
func TestMediaOriginalType415(t *testing.T) {
	captureLog(t)
	a, dir := origApp(t)
	jpg := readFile(t, testkit.JPEG(t, dir, "src.jpg", 60, 40))
	png := readFile(t, testkit.PNG(t, dir, "src.png", 30, 20, colorRed))
	webp := readFile(t, filepath.Join("..", "imgfile", "testdata", "lossy16.webp"))
	bad := map[string][]byte{
		"gif":        []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"),
		"текст":      []byte("просто текст, не картинка"),
		"html":       []byte("<html><script>alert(1)</script></html>"),
		"пустой":     {},
		"1 байт":     {0xFF},
		"jpeg-обрыв": jpg[:30],
		"jpeg-2":     jpg[:2],
		"png-обрыв":  png[:20],
		"png-8":      png[:8],
		"webp-обрыв": webp[:14],
		"webp-анимация": append([]byte("RIFF\x1a\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00\x02\x00\x00\x00"),
			make([]byte, 12)...),
	}
	for name, data := range bad {
		path := filepath.Join(dir, "bad-"+name+".jpg")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		id := origPhoto(a, path, 60, 40, imgfile.ColorNone)
		rec := origReq(a, http.MethodGet, "/media/o/"+id)
		if rec.Code != http.StatusUnsupportedMediaType {
			t.Errorf("%s: код %d, ждали 415", name, rec.Code)
		}
		if rec.Body.Len() > 100 {
			t.Errorf("%s: в теле %d байт (отдан файл?)", name, rec.Body.Len())
		}
	}
	// Каталог вместо файла — не обычный файл: 404.
	id := origPhoto(a, dir, 60, 40, imgfile.ColorNone)
	if rec := origReq(a, http.MethodGet, "/media/o/"+id); rec.Code != http.StatusNotFound {
		t.Errorf("каталог: код %d, ждали 404", rec.Code)
	}
}

// Пропавшее — 404 без обращения к диску: ни проверки тома, ни чтения цвета, ни открытия файла.
// Файл, удалённый после сканирования, — тоже 404.
func TestMediaOriginalMissing404NoDisk(t *testing.T) {
	a, dir := origApp(t)
	path := filepath.Join(dir, "нет.jpg")
	id := library.IDFor(path)
	if _, err := a.editor().NewSpread(-1, []model.PhotoInfo{{ID: id, Path: path, W: 30, H: 20}}); err != nil {
		t.Fatal(err)
	}
	oldR, oldC := reachable, readColor
	t.Cleanup(func() { reachable, readColor = oldR, oldC })
	reachable = func(string) bool { t.Error("пропавшее: проверка тома"); return false }
	readColor = func(string) (imgfile.Color, string) {
		t.Error("пропавшее: чтение цвета")
		return "", ""
	}
	a.mu.Lock()
	a.missing = []string{id}
	a.mu.Unlock()
	rec := origReq(a, http.MethodGet, "/media/o/"+id)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("пропавшее: %d", rec.Code)
	}
	if a.mediaMiss.Load() != 0 {
		t.Error("пропавшее записано в промахи журнала")
	}

	// Удалён после сканирования, список недостающих ещё пуст: 404 при открытии.
	reachable, readColor = oldR, oldC
	a.mu.Lock()
	a.missing = nil
	a.mu.Unlock()
	gone := testkit.JPEG(t, dir, "gone.jpg", 60, 40)
	gid := origPhoto(a, gone, 60, 40, imgfile.ColorNone)
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	if rec := origReq(a, http.MethodGet, "/media/o/"+gid); rec.Code != http.StatusNotFound {
		t.Errorf("удалённый: %d", rec.Code)
	}
	// Том не отвечает: 404 до открытия файла.
	live := testkit.JPEG(t, dir, "live.jpg", 60, 40)
	lid := origPhoto(a, live, 60, 40, imgfile.ColorNone)
	reachable = func(string) bool { return false }
	if rec := origReq(a, http.MethodGet, "/media/o/"+lid); rec.Code != http.StatusNotFound {
		t.Errorf("мёртвый том: %d", rec.Code)
	}
}

// Неизвестный ID, пустой, с путём и с кодированным путём — 404; путь из адреса к диску не идёт.
func TestMediaOriginalUnknownID(t *testing.T) {
	captureLog(t)
	a, dir := origApp(t)
	path := testkit.JPEG(t, dir, "a.jpg", 60, 40)
	origPhoto(a, path, 60, 40, imgfile.ColorNone)
	oldR, oldC := reachable, readColor
	t.Cleanup(func() { reachable, readColor = oldR, oldC })
	reachable = func(string) bool { t.Error("чужой ID: проверка тома"); return false }
	readColor = func(string) (imgfile.Color, string) { t.Error("чужой ID: чтение цвета"); return "", "" }
	for _, id := range []string{"", "0123456789abcdef", "../a.jpg", "a/b", "%2Fetc%2Fpasswd", strings.TrimPrefix(path, "/"), "..%2F..%2Fetc%2Fpasswd"} {
		if rec := origReq(a, http.MethodGet, "/media/o/"+id); rec.Code != wantRefusal("/media/o/"+id) {
			t.Errorf("ID %q: код %d, ждали %d", id, rec.Code, wantRefusal("/media/o/"+id))
		}
	}
}

// HEAD — заголовки как у GET (в том числе длина), тела нет.
func TestMediaOriginalHeadNoBody(t *testing.T) {
	a, dir := origApp(t)
	path := testkit.JPEG(t, dir, "a.jpg", 60, 40)
	id := origPhoto(a, path, 60, 40, imgfile.ColorNone)
	rec := origReq(a, http.MethodHead, "/media/o/"+id)
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("HEAD: %d, тело %d байт", rec.Code, rec.Body.Len())
	}
	get := origReq(a, http.MethodGet, "/media/o/"+id)
	for _, k := range []string{"Content-Type", "Content-Length", "Last-Modified", "Cache-Control", "X-Content-Type-Options"} {
		if rec.Header().Get(k) == "" || rec.Header().Get(k) != get.Header().Get(k) {
			t.Errorf("%s: HEAD %q, GET %q", k, rec.Header().Get(k), get.Header().Get(k))
		}
	}
	// HEAD с отказом — тоже отказ, а не пустой 200.
	bad := origPhoto(a, filepath.Join(dir, "нет.jpg"), 60, 40, imgfile.ColorAdobe)
	if rec := origReq(a, http.MethodHead, "/media/o/"+bad); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("HEAD, Adobe: %d", rec.Code)
	}
}

// Прочие методы — 405 с Allow, в том числе для неизвестного ID (метод проверяется первым).
func TestMediaOriginalMethod405(t *testing.T) {
	a, dir := origApp(t)
	path := testkit.JPEG(t, dir, "a.jpg", 60, 40)
	id := origPhoto(a, path, 60, 40, imgfile.ColorNone)
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodOptions} {
		for _, target := range []string{id, "неизвестный"} {
			rec := origReq(a, m, "/media/o/"+target)
			if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
				t.Errorf("%s %s: код %d, Allow %q", m, target, rec.Code, rec.Header().Get("Allow"))
			}
		}
	}
}

// Файл больше предела в байтах — 415 (на WebView2 тело копится в памяти): граница включена,
// разреженный файл сверх реального предела отклоняется по Stat, без чтения.
func TestMediaOriginalTooBig415(t *testing.T) {
	a, dir := origApp(t)
	path := testkit.JPEG(t, dir, "a.jpg", 60, 40)
	size := int64(len(readFile(t, path)))
	old := origMaxBytes
	t.Cleanup(func() { origMaxBytes = old })
	id := origPhoto(a, path, 60, 40, imgfile.ColorNone)
	origMaxBytes = size
	if rec := origReq(a, http.MethodGet, "/media/o/"+id); rec.Code != http.StatusOK {
		t.Errorf("ровно предел: %d", rec.Code)
	}
	origMaxBytes = size - 1
	if rec := origReq(a, http.MethodGet, "/media/o/"+id); rec.Code != http.StatusUnsupportedMediaType || rec.Body.Len() > 100 {
		t.Errorf("на байт больше: %d, тело %d байт", rec.Code, rec.Body.Len())
	}
	// Настоящий предел: JPEG с нулями в хвосте (разреженный файл, места не занимает).
	origMaxBytes = old
	big := testkit.JPEG(t, dir, "big.jpg", 60, 40)
	if err := os.Truncate(big, 300<<20); err != nil {
		t.Skipf("разреженный файл не создан: %v", err)
	}
	bid := origPhoto(a, big, 60, 40, imgfile.ColorNone)
	if rec := origReq(a, http.MethodGet, "/media/o/"+bid); rec.Code != http.StatusUnsupportedMediaType || rec.Body.Len() > 100 {
		t.Errorf("300 МБ: %d, тело %d байт", rec.Code, rec.Body.Len())
	}
}

// Отдача одна за раз: пока слот занят, второй запрос ждёт; ушедший клиент слот не получает и
// ничего не пишет; после освобождения запрос проходит.
func TestMediaOriginalOneSlot(t *testing.T) {
	a, dir := origApp(t)
	path := testkit.JPEG(t, dir, "a.jpg", 60, 40)
	id := origPhoto(a, path, 60, 40, imgfile.ColorNone)
	origSlots <- struct{}{}
	released := false
	release := func() {
		if !released {
			released = true
			<-origSlots
		}
	}
	t.Cleanup(release)
	gone, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- getMediaCtx(gone, a, "/media/o/"+id) }()
	select {
	case rec := <-done:
		t.Fatalf("слот занят, а запрос отвечен: %d", rec.Code)
	case <-time.After(200 * time.Millisecond): // ждёт, а не отказывает по таймеру
	}
	cancel()
	select {
	case rec := <-done:
		if !unanswered(rec) {
			t.Errorf("клиент ушёл: ответ %d", rec.Code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ушедший клиент не снял ожидание слота")
	}
	release()
	if rec := origReq(a, http.MethodGet, "/media/o/"+id); rec.Code != http.StatusOK {
		t.Errorf("слот свободен: %d", rec.Code)
	}
	if len(origSlots) != 0 {
		t.Error("слот не возвращён после ответа")
	}
}

// Относительный путь у фото ленты — 404 без обращения к диску (CheckLocalPath): файл лежит в
// рабочей папке и открылся бы, не будь проверки.
func TestMediaOriginalRelativePath404(t *testing.T) {
	a, dir := origApp(t)
	testkit.JPEG(t, dir, "rel.jpg", 60, 40)
	t.Chdir(dir)
	oldR := reachable
	t.Cleanup(func() { reachable = oldR })
	reachable = func(string) bool { return true } // том не судит; граница путей — CheckLocalPath
	id := origPhoto(a, "rel.jpg", 60, 40, imgfile.ColorNone)
	if rec := origReq(a, http.MethodGet, "/media/o/"+id); rec.Code != http.StatusNotFound {
		t.Errorf("относительный путь: %d, ждали 404", rec.Code)
	}
}
