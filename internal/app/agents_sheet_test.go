package app

// Лист-сетка агента на настоящем App (151, Т3): Host.Sheet читает миниатюры через library.Cache, рисует
// лист пакетом contactsheet. Служба над ним (выбор фото, уровни, страницы) — internal/app/agents/sheet_test.go.

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/contactsheet"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// sheetApp — App с кэшем миниатюр и n фото в ленте (a.byID); ids — по порядку.
func sheetApp(t *testing.T, n, w, h int) (a *App, dir string, ids []string) {
	t.Helper()
	a = newApp(t)
	c, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c
	dir = testkit.TempDir(t)
	for i := range n {
		ids = append(ids, addStripPhoto(t, a, testkit.JPEG(t, dir, "p"+strconv.Itoa(i)+".jpg", w, h), w, h))
	}
	return a, dir, ids
}

// addStripPhoto кладёт файл path в ленту App и возвращает ID.
func addStripPhoto(t *testing.T, a *App, path string, w, h int) string {
	t.Helper()
	id := library.IDFor(path)
	a.mu.Lock()
	a.byID[id] = library.Photo{ID: id, Path: path, Name: filepath.Base(path), W: w, H: h}
	a.mu.Unlock()
	return id
}

func sheetStates(ctx context.Context, t *testing.T, a *App, ids []string, grid, side int) ([]byte, []string) {
	t.Helper()
	jpg, states, err := a.agentsHost().Sheet(ctx, agents.SheetReq{IDs: ids, First: 1, Grid: grid, Side: side})
	if err != nil {
		t.Fatalf("Sheet: %v", err)
	}
	return jpg, states
}

// jpegMarkers — маркеры JPEG до SOS по независимому разбору; в данных скана маркеров, кроме RST/EOI, нет.
func jpegMarkers(t *testing.T, b []byte) []byte {
	t.Helper()
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 || b[len(b)-2] != 0xFF || b[len(b)-1] != 0xD9 {
		t.Fatal("не JPEG: нет SOI или EOI")
	}
	out := []byte{0xD8}
	for i := 2; i+4 <= len(b); {
		if b[i] != 0xFF {
			t.Fatalf("нет маркера на %d", i)
		}
		out = append(out, b[i+1])
		if b[i+1] == 0xDA {
			break
		}
		i += 2 + int(b[i+2])<<8 + int(b[i+3])
	}
	return out
}

// TestContactSheetBroken: пустой ID, пропавший, усечённый и не-картинка — «×», а хорошие фото на месте;
// лист собран и в пределе.
func TestContactSheetBroken(t *testing.T) {
	a, dir, ids := sheetApp(t, 2, 600, 400)
	gone := addStripPhoto(t, a, testkit.JPEG(t, dir, "gone.jpg", 600, 400), 600, 400)
	trunc := addStripPhoto(t, a, truncated(t, dir, "trunc.jpg", 400, 300, 700), 400, 300)
	text := filepath.Join(dir, "text.jpg")
	if err := os.WriteFile(text, []byte("не картинка"), 0o644); err != nil {
		t.Fatal(err)
	}
	notImg := addStripPhoto(t, a, text, 100, 100)
	if err := os.Remove(a.byID[gone].Path); err != nil {
		t.Fatal(err)
	}
	all := []string{ids[0], "", gone, trunc, notImg, ids[1], "нет-такого"}
	jpg, states := sheetStates(context.Background(), t, a, all, 3, 768)
	want := []string{"ok", "broken", "broken", "broken", "broken", "ok", "broken"}
	if !reflect.DeepEqual(states, want) {
		t.Fatalf("состояния %v, ждали %v", states, want)
	}
	if cfg, err := jpeg.DecodeConfig(bytes.NewReader(jpg)); err != nil || cfg.Width != 768 || len(jpg) > contactsheet.MaxJPEG {
		t.Fatalf("лист: %v %+v, %d байт", err, cfg, len(jpg))
	}
}

// TestContactSheetUNC: UNC, закрытый том и чужой ID — «×» без обращения к диску и без записи в кэш.
func TestContactSheetUNC(t *testing.T) {
	pr := stubReachable(t)
	a, cacheDir, ids := uncApp(t)
	_, states := sheetStates(context.Background(), t, a, []string{ids["good"], ids["closed"], ids["unc1"], ids["unc2"], ids["strip"], "нет-такого"}, 3, 768)
	if want := []string{"ok", "broken", "broken", "broken", "broken", "broken"}; !reflect.DeepEqual(states, want) {
		t.Fatalf("состояния %v, ждали %v", states, want)
	}
	for _, unc := range []string{"srv/share", `srv\share`} {
		if pr.saw(unc) {
			t.Errorf("UNC %q дошёл до проверки тома", unc)
		}
	}
	if !pr.saw("closed_ref") || !pr.saw("closed_strip") {
		t.Errorf("тома закрытых фото не проверены: %v", pr.list)
	}
	for _, f := range agCacheFiles(t, cacheDir) {
		if strings.Contains(f, "closed") || strings.Contains(f, "unc") {
			t.Errorf("в кэш попал отказанный файл %s", f)
		}
	}
}

// TestContactSheetDeadline: остаток срока меньше запаса листа — все клетки pending, лист из заглушек всё равно
// собран и вернулся в срок; со сроком побольше те же фото готовы; снятый вызов — ошибка без листа.
func TestContactSheetDeadline(t *testing.T) {
	a, _, ids := sheetApp(t, 5, 600, 400)
	req := agents.SheetReq{IDs: ids, First: 1, Grid: 3, Side: 768}
	// Срок не масштабируется (SA_TEST_SLOW): он должен быть меньше запаса листа в любой среде; вышел бы
	// целиком до конца рисунка — ошибка, а не лист.
	tight, cancel := context.WithTimeout(context.Background(), sheetSlack-100*time.Millisecond)
	defer cancel()
	jpg, states, err := a.agentsHost().Sheet(tight, req)
	if err != nil || len(jpg) == 0 {
		t.Fatalf("тесный срок: %v, %d байт", err, len(jpg))
	}
	for i, s := range states {
		if s != "pending" {
			t.Errorf("клетка %d при сроке меньше запаса: %s", i, s)
		}
	}
	// Заглушки не записали ничего в кэш: повторный вызов со сроком берёт всё заново и успевает.
	roomy, cancel2 := context.WithTimeout(context.Background(), testkit.Limit(30*time.Second))
	defer cancel2()
	_, states = sheetStates(roomy, t, a, ids, 3, 768)
	for i, s := range states {
		if s != "ok" {
			t.Errorf("клетка %d со сроком: %s", i, s)
		}
	}
	// Вызов снят: лист не нужен.
	cc, stop := context.WithCancel(context.Background())
	stop()
	if _, _, err := a.agentsHost().Sheet(cc, req); !errors.Is(err, context.Canceled) {
		t.Fatalf("снятый вызов: %v", err)
	}
}

// TestContactSheetNoMeta: метаданные оригинала (EXIF, COM, имя файла) не доходят до листа — только пиксели.
func TestContactSheetNoMeta(t *testing.T) {
	a := newApp(t)
	c, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c
	dir := testkit.TempDir(t)
	const secret = "ZQSECRETMETA"
	p := testkit.JPEGSegments(t, dir, "ZQSECRETNAME.jpg", 600, 400,
		testkit.Segment(0xE1, []byte("Exif\x00\x00"+secret)), testkit.Segment(0xFE, []byte(secret)), testkit.Segment(0xE1, []byte("http://ns.adobe.com/xap/1.0/\x00"+secret)))
	id := addStripPhoto(t, a, p, 600, 400)
	jpg, states := sheetStates(context.Background(), t, a, []string{id}, 3, 768)
	if states[0] != "ok" {
		t.Fatalf("состояние %v", states)
	}
	for _, bad := range []string{secret, "ZQSECRETNAME", "Exif", "xap"} {
		if bytes.Contains(jpg, []byte(bad)) {
			t.Errorf("в листе есть %q", bad)
		}
	}
	for _, m := range jpegMarkers(t, jpg) { // только SOI, DQT, DHT, SOF0, SOS
		if m != 0xD8 && m != 0xDB && m != 0xC4 && m != 0xC0 && m != 0xDA {
			t.Errorf("маркер %#x в листе", m)
		}
	}
}

// TestContactSheetBudget: 8 × 8 на 1568 при готовых миниатюрах — не больше 600 КБ и быстро (>1 с лист
// не нужен: вызов инструмента ограничен 20 с вместе с кодированием службы).
func TestContactSheetBudget(t *testing.T) {
	n := testkit.Steps(64, 16)
	a, _, ids := sheetApp(t, n, 600, 400)
	sheetStates(context.Background(), t, a, ids, 8, 1568) // прогрев кэша миниатюр
	var jpg []byte
	var states []string
	d := testkit.Fastest(3, func() { jpg, states = sheetStates(context.Background(), t, a, ids, 8, 1568) })
	if limit := testkit.Limit(time.Second); d > limit {
		t.Errorf("лист 8 × 8 на готовых миниатюрах: %v, предел %v", d, limit)
	}
	if len(jpg) > contactsheet.MaxJPEG || len(jpg) == 0 {
		t.Errorf("JPEG %d байт, предел %d", len(jpg), contactsheet.MaxJPEG)
	}
	for i, s := range states {
		if s != "ok" {
			t.Fatalf("клетка %d: %s", i, s)
		}
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(jpg))
	if err != nil || max(cfg.Width, cfg.Height) > 1568 {
		t.Errorf("размер листа: %v %+v", err, cfg)
	}
}

// TestContactSheetCold: холодные миниатюры (оригинал 3000 × 2000 → 400 px) берутся классом прогрева; замер —
// сколько клеток в секунду на этой машине. Все клетки при щедром сроке готовы, число в журнал теста.
func TestContactSheetCold(t *testing.T) {
	n := testkit.Steps(36, 12)
	w, h := 3000, 2000 // 6 Мп; замер на 4000 × 3000 (12 Мп) в отчёте: ≈ 8 шт/с на Ubuntu
	if testing.Short() {
		w, h = 1600, 1200
	}
	a, _, ids := sheetApp(t, n, w, h)
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(60*time.Second))
	defer cancel()
	start := time.Now()
	jpg, states := sheetStates(ctx, t, a, ids, 6, 1056)
	d := time.Since(start)
	pending := 0
	for _, s := range states {
		switch s {
		case "pending":
			pending++
		case "broken":
			t.Fatalf("клетка broken: %v", states)
		}
	}
	t.Logf("холодных миниатюр: %d за %v — %.1f шт/с, pending %d, лист %d КБ", n, d.Round(time.Millisecond), float64(n)/d.Seconds(), pending, len(jpg)>>10)
	if pending > 0 {
		t.Errorf("при сроке %v не успели %d из %d", testkit.Limit(60*time.Second), pending, n)
	}
	// Повтор: миниатюры готовы, вызов быстрый.
	if d2 := testkit.Fastest(2, func() { sheetStates(ctx, t, a, ids, 6, 1056) }); d2 > d/2 && d > 200*time.Millisecond {
		t.Errorf("повтор на готовых миниатюрах %v, холодный %v", d2, d)
	}
}
