package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"image/color"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Карточка фото (084 design §4): путь по ID, кэш, недостающее, вхождения после правки.

var gray = color.NRGBA{R: 128, G: 128, B: 128, A: 255}

// countReads подменяет readAbout счётчиком поверх настоящего чтения.
func countReads(t *testing.T) *atomic.Int32 {
	t.Helper()
	var n atomic.Int32
	old := readAbout
	readAbout = func(p string) (library.About, error) { n.Add(1); return old(p) }
	t.Cleanup(func() { readAbout = old })
	return &n
}

// shotJPEG — JPEG w×h с EXIF камеры Canon в dir.
func shotJPEG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	return testkit.JPEGSegments(t, dir, name, w, h, testkit.ExifShot(false, testkit.ShotFields{
		Make: "Canon", Model: "Canon EOS R6", FNumber: [2]uint32{28, 10}, Exposure: [2]uint32{1, 200}, ISO: 400,
	}))
}

func mustAbout(t *testing.T, a *App, id string) PhotoAbout {
	t.Helper()
	got, err := a.PhotoAbout(id)
	if err != nil {
		t.Fatalf("PhotoAbout(%s): %v", id, err)
	}
	return got
}

// Фото из ленты (не в альбоме): строки файла и съёмки; PNG без EXIF — «Нет данных о съёмке».
// Несколько фото сразу — сведения не путаются; пустые срезы — «[]» в JSON.
func TestPhotoAboutLibraryPhoto(t *testing.T) {
	a := newApp(t)
	dir := testkit.TempDir(t)
	jp := shotJPEG(t, dir, "a.jpg", 60, 40)
	pp := testkit.PNG(t, dir, "b.png", 30, 20, gray)
	for _, p := range []string{jp, pp} {
		a.byID[library.IDFor(p)] = library.Photo{ID: library.IDFor(p), Path: p, Name: filepath.Base(p)}
	}
	j := mustAbout(t, a, library.IDFor(jp))
	if j.Note != "" || !reflect.DeepEqual(j.Shot, []string{"Canon EOS R6", "f/2,8 · 1/200 с · ISO 400"}) {
		t.Fatalf("JPEG: %+v", j)
	}
	if j.ID != library.IDFor(jp) || j.File == "" || j.Used != "В альбоме не использовано" || j.Uses == nil || len(j.Uses) != 0 {
		t.Fatalf("JPEG: %+v", j)
	}
	p := mustAbout(t, a, library.IDFor(pp))
	if p.Note != noteNoShot || p.Shot == nil || len(p.Shot) != 0 || p.File == "" || p.File == j.File {
		t.Fatalf("PNG: %+v", p)
	}
	b, _ := json.Marshal(p)
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"shot", "uses"} {
		if v, ok := raw[k].([]any); !ok || len(v) != 0 {
			t.Fatalf("%s в JSON: %v (%s)", k, raw[k], b)
		}
	}
}

// Папка не открыта: путь — из проекта, вхождения — по альбому.
func TestPhotoAboutProjectOnly(t *testing.T) {
	a := newApp(t)
	jp := shotJPEG(t, testkit.TempDir(t), "a.jpg", 60, 40)
	id := library.IDFor(jp)
	v, err := a.editor().NewSpread(-1, []model.PhotoInfo{{ID: id, Path: jp, W: 60, H: 40}})
	if err != nil {
		t.Fatal(err)
	}
	got := mustAbout(t, a, id)
	want := []model.PhotoUse{{Sheet: 0, Label: "Разворот 1 — место", Kind: model.UsePlace, Spread: v.Spreads[0].ID,
		Page: "Разворот 1", What: "место", Pick: id}}
	if got.Note != "" || len(got.Shot) != 2 || got.Used != "Использовано 1 раз" || !reflect.DeepEqual(got.Uses, want) {
		t.Fatalf("%+v", got)
	}
}

// Неизвестный ID (и пустой) — ошибка, файл не читается.
func TestPhotoAboutUnknownID(t *testing.T) {
	a := newApp(t)
	reads := countReads(t)
	for _, id := range []string{"нет", ""} {
		if got, err := a.PhotoAbout(id); err == nil {
			t.Fatalf("%q: нет ошибки, %+v", id, got)
		}
	}
	if reads.Load() != 0 {
		t.Fatalf("чтений: %d", reads.Load())
	}
}

// Недостающее фото, мёртвый том и пропавший файл — «Файл не найден», вхождения есть; в первых
// двух случаях диск не трогается.
func TestPhotoAboutMissing(t *testing.T) {
	a, id, _ := missingApp(t)
	reads := countReads(t)
	check := func(name string) {
		t.Helper()
		got := mustAbout(t, a, id)
		if got.Note != noteMissing || got.File != "" || len(got.Shot) != 0 || got.Used != "Использовано 1 раз" || len(got.Uses) != 1 {
			t.Fatalf("%s: %+v", name, got)
		}
	}
	a.mu.Lock()
	a.missing = []string{id}
	a.mu.Unlock()
	check("недостающее")
	a.mu.Lock()
	a.missing = nil
	a.mu.Unlock()
	old := reachable
	reachable = func(string) bool { return false }
	t.Cleanup(func() { reachable = old })
	check("мёртвый том")
	if reads.Load() != 0 {
		t.Fatalf("диск тронут: %d чтений", reads.Load())
	}
	reachable = func(string) bool { return true }
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	check("файла нет") // пересчёт недостающих ещё не прошёл: Stat не находит файл
	check("повтор")
	if buf.Len() != 0 || a.about.logged[id] {
		t.Fatalf("пропавший файл попал в журнал: %q", buf.String())
	}
	// Превью того же пропавшего файла: 404 и тоже без строки в журнале.
	c, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c
	w := httptest.NewRecorder()
	a.media(http.NotFoundHandler()).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/media/p/"+id, nil))
	if w.Code != http.StatusNotFound || buf.Len() != 0 {
		t.Fatalf("превью пропавшего: %d, журнал %q", w.Code, buf.String())
	}
}

// Прочая ошибка чтения (не «нет файла») по-прежнему пишется в журнал — раз на ID.
func TestPhotoAboutReadErrorLogged(t *testing.T) {
	a, id, _ := missingApp(t)
	path := shotJPEG(t, testkit.TempDir(t), "a.jpg", 30, 20)
	a.mu.Lock()
	a.byID = map[string]library.Photo{id: {ID: id, Path: path}}
	a.mu.Unlock()
	old := readAbout
	readAbout = func(string) (library.About, error) { return library.About{}, errors.New("битый файл") }
	t.Cleanup(func() { readAbout = old })
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	for range 2 {
		if got := mustAbout(t, a, id); got.Note != noteMissing {
			t.Fatalf("%+v", got)
		}
	}
	if !a.about.logged[id] || bytes.Count(buf.Bytes(), []byte("битый файл")) != 1 {
		t.Fatalf("журнал %q", buf.String())
	}
}

// Нелокальный путь (UNC, поток NTFS) в ленте — «Файл не найден» без пробы тома и без чтения:
// UNC не трогается вовсе (утечка NTLM), мёртвый сетевой диск не вешает вызов (аудит 0.21).
func TestPhotoAboutUnsafePath(t *testing.T) {
	a := newApp(t)
	reads := countReads(t)
	var probes atomic.Int32
	old := reachable
	reachable = func(string) bool { probes.Add(1); return true }
	t.Cleanup(func() { reachable = old })
	for _, p := range []string{`\\127.0.0.1\c$\a.jpg`, `//host/share/a.jpg`, `C:\a.jpg:zone`} {
		id := library.IDFor(p)
		a.byID[id] = library.Photo{ID: id, Path: p}
		if got := mustAbout(t, a, id); got.Note != noteMissing || got.File != "" {
			t.Errorf("%q: %+v", p, got)
		}
	}
	if reads.Load() != 0 || probes.Load() != 0 {
		t.Fatalf("чтений %d, проб тома %d", reads.Load(), probes.Load())
	}
}

// Второй вызов берёт сведения из кэша; файл изменился (размер, время) — читается заново.
func TestPhotoAboutCache(t *testing.T) {
	a := newApp(t)
	dir := testkit.TempDir(t)
	jp := shotJPEG(t, dir, "a.jpg", 60, 40)
	id := library.IDFor(jp)
	a.byID[id] = library.Photo{ID: id, Path: jp}
	reads := countReads(t)
	first := mustAbout(t, a, id)
	mustAbout(t, a, id)
	if reads.Load() != 1 {
		t.Fatalf("чтений после двух вызовов: %d", reads.Load())
	}
	// Тот же файл, другой размер: новая запись кэша.
	data, err := os.ReadFile(testkit.PNG(t, dir, "b.png", 80, 50, gray))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jp, data, 0o600); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(jp, later, later); err != nil {
		t.Fatal(err)
	}
	second := mustAbout(t, a, id)
	if reads.Load() != 2 || second.File == first.File || second.Note != noteNoShot {
		t.Fatalf("после замены: чтений %d, %+v", reads.Load(), second)
	}
}

// Переполненный кэш сбрасывается целиком, а не растёт.
func TestPhotoAboutCacheLimit(t *testing.T) {
	var c aboutCache
	for i := range aboutMax + 3 {
		c.put(aboutKey{path: string(rune('a' + i%26)), size: int64(i)}, library.About{})
	}
	if len(c.m) != 3 {
		t.Fatalf("записей: %d", len(c.m))
	}
}

// Новый фон с тем же фото — новое вхождение сразу, отмена — прежние (сц. 6); файл из кэша.
func TestPhotoAboutAfterEdit(t *testing.T) {
	a := newApp(t)
	dir := testkit.TempDir(t)
	jp, other := shotJPEG(t, dir, "a.jpg", 60, 40), testkit.JPEG(t, dir, "b.jpg", 40, 30)
	id := library.IDFor(jp)
	if _, err := a.editor().NewSpread(-1, []model.PhotoInfo{{ID: id, Path: jp, W: 60, H: 40}}); err != nil {
		t.Fatal(err)
	}
	v, err := a.editor().NewSpread(-1, []model.PhotoInfo{{ID: library.IDFor(other), Path: other, W: 40, H: 30}})
	if err != nil {
		t.Fatal(err)
	}
	reads := countReads(t)
	if got := mustAbout(t, a, id); got.Used != "Использовано 1 раз" {
		t.Fatalf("до правки: %+v", got)
	}
	if _, err := a.SetBackdrop(v.Spreads[1].ID, model.ScopeSpread, id); err != nil {
		t.Fatal(err)
	}
	got := mustAbout(t, a, id)
	want := []model.PhotoUse{
		{Sheet: 0, Label: "Разворот 1 — место", Kind: model.UsePlace, Spread: v.Spreads[0].ID, Page: "Разворот 1", What: "место", Pick: id},
		{Sheet: 1, Label: "Разворот 2 — фон", Kind: model.UseBackdrop, Spread: v.Spreads[1].ID, Page: "Разворот 2", What: "фон", Pick: model.ScopeSpread},
	}
	if got.Used != "Использовано 2 раза" || !reflect.DeepEqual(got.Uses, want) {
		t.Fatalf("после фона: %+v", got)
	}
	a.Undo()
	if got := mustAbout(t, a, id); got.Used != "Использовано 1 раз" || len(got.Uses) != 1 {
		t.Fatalf("после отмены: %+v", got)
	}
	if reads.Load() != 1 {
		t.Fatalf("чтений файла: %d (правка не меняет файл)", reads.Load())
	}
}
