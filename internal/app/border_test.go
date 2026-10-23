package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum/collect"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Рамочки мест в App (079 G6): ApplyBorder из каталога, /media/f/<id> и сверка экрана с печатью.

const (
	brSheet  = "s1"
	brPlace  = "000000000001"
	brEmpty  = "000000000002"
	brPhoto  = "a"
	brFrame  = "f0000000000000f1" // запись рамочки без lib (как после «Собрать в папку»)
	brMMtoPx = 10                 // 254 dpi: 10 px на мм, края окна — на целых пикселях
)

// ringFrame — PNG w×h: непрозрачная кайма l, t, r, b px цвета c, внутри прозрачное окно.
func ringFrame(t testing.TB, dir string, w, h, l, top, r, b int, c color.NRGBA) string {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			if x < l || y < top || x >= w-r || y >= h-b {
				im.SetNRGBA(x, y, c)
			}
		}
	}
	path := filepath.Join(dir, "рамка.png")
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, im); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// pngHeaderOnly — PNG с настоящим IHDR w×h и без данных: DecodeConfig его читает, а гигантская
// картинка в памяти теста не нужна.
func pngHeaderOnly(t testing.TB, path string, w, h int) {
	t.Helper()
	var ihdr [13]byte
	binary.BigEndian.PutUint32(ihdr[0:], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(h))
	ihdr[8], ihdr[9] = 8, 6 // 8 бит, RGBA
	var buf bytes.Buffer
	buf.WriteString("\x89PNG\r\n\x1a\n")
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(ihdr)))
	chunk := append([]byte("IHDR"), ihdr[:]...)
	buf.Write(chunk)
	_ = binary.Write(&buf, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

// borderApp — App с листом s1: место с фото brPhoto (60×50 мм) и пустое место; у места с фото —
// рамочка-картинка frame (запись brFrame без lib, срез cut). Библиотека открыта.
func borderApp(t *testing.T, frame string, fw, fh int, cut [4]float64) *App {
	t.Helper()
	a := decorApp(t)
	// Фото — сплошной цвет: «кайма» в полкартинки закрывает всё (testkit.PNG наполовину прозрачен).
	photo := ringFrame(t, testkit.TempDir(t), 120, 100, 60, 50, 60, 50, color.NRGBA{R: 220, G: 40, B: 40, A: 255})
	p := model.New()
	p.Photos[brPhoto] = model.PhotoRef{Path: photo, W: 120, H: 100}
	p.Photos[brFrame] = model.PhotoRef{Path: frame, W: fw, H: fh}
	p.Spreads = []*model.Spread{{ID: brSheet, Photos: []string{brPhoto}, Grid: []*model.Place{
		{ID: brPlace, X: 20, Y: 10, W: 60, H: 50, Photo: brPhoto, Border: &model.Border{Photo: brFrame, Cut: cut}},
		{ID: brEmpty, X: 100, Y: 10, W: 40, H: 40},
	}}}
	a.ed.Store(mustEditor(t, p, ""))
	return a
}

// getMedia — ответ /media на GET path.
func getMedia(a *App, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	a.media(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// mediaPNG — картинка ответа /media/f по адресу из View с дописанным px.
func mediaPNG(t *testing.T, a *App, url string, px int) *image.NRGBA {
	t.Helper()
	rec := getMedia(a, url+"&px="+strconv.Itoa(px))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("%s: %d %q", url, rec.Code, rec.Body.String())
	}
	im, err := png.Decode(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	out := image.NewNRGBA(im.Bounds())
	for y := im.Bounds().Min.Y; y < im.Bounds().Max.Y; y++ {
		for x := im.Bounds().Min.X; x < im.Bounds().Max.X; x++ {
			out.Set(x, y, im.At(x, y))
		}
	}
	return out
}

// placeOf — вид места id листа.
func placeOf(t *testing.T, v model.View, id string) model.PlaceView {
	t.Helper()
	for _, pv := range v.Spreads[0].Places {
		if pv.ID == id {
			return pv
		}
	}
	t.Fatalf("места %s нет", id)
	return model.PlaceView{}
}

// lerp8 — a·(1−k) + b·k в байтах.
func lerp8(a, b uint8, k float64) float64 { return float64(a)*(1-k) + float64(b)*k }

// screenVsPrint — расхождение экрана с печатью на месте с рамочкой: наибольшее и число пикселей
// сверх ±2/255 — отдельно на полупрозрачном крае накладки (0 < альфа < 255) и на остальном.
type screenVsPrint struct {
	worstEdge, overEdge, worstFlat, overFlat int
}

// compareFrameMedia собирает экран, как ячейка (страница, фото в окне FrameView.Win под маской
// /media/f &m=1, накладка /media/f поверх), и сверяет с куском экспорта того же места при 254 dpi.
func compareFrameMedia(t *testing.T, a *App) screenVsPrint {
	t.Helper()
	pp, v := a.editor().Snapshot()
	pv := placeOf(t, v, brPlace)
	b := pv.Border
	if b == nil || !b.Pic || !strings.HasPrefix(b.URL, "/media/f/"+brFrame+"?") || !strings.HasPrefix(b.MaskURL, "/media/f/"+brFrame+"?") ||
		!strings.Contains(b.URL, "c=0.08,0.12,0.12,0.15") || strings.Contains(b.URL, "px=") {
		t.Fatalf("адреса рамочки: %+v", b)
	}
	fv := v.Spreads[0].Frames[0]
	canvas, err := render.Render(export.FromView(pp, v)[0], render.Options{PageW: pp.Settings.PageW, PageH: pp.Settings.PageH, DPI: 254})
	if err != nil {
		t.Fatal(err)
	}
	pw, ph := int(pv.W*brMMtoPx), int(pv.H*brMMtoPx)
	ov, mask := mediaPNG(t, a, b.URL, pw), mediaPNG(t, a, b.MaskURL, pw)
	if ov.Rect.Dx() != pw || ov.Rect.Dy() != ph || mask.Rect != ov.Rect {
		t.Fatalf("размер ответа: накладка %v, маска %v, ждали %d×%d", ov.Rect, mask.Rect, pw, ph)
	}
	ox, oy := int(pv.X*brMMtoPx), int(pv.Y*brMMtoPx)
	page := canvas.RGBAAt(ox-5, oy-5) // страница рядом с местом
	photo := canvas.RGBAAt(int((fv.Win.X+fv.Win.W/2)*brMMtoPx), int((fv.Win.Y+fv.Win.H/2)*brMMtoPx))
	win := image.Rect(int((fv.Win.X-pv.X)*brMMtoPx+0.5), int((fv.Win.Y-pv.Y)*brMMtoPx+0.5),
		int((fv.Win.X+fv.Win.W-pv.X)*brMMtoPx+0.5), int((fv.Win.Y+fv.Win.H-pv.Y)*brMMtoPx+0.5))
	// Окно — по срезу альбома (0,08·600, 0,12·400, …), а не по мете картинки (0,1 / 0,15).
	if photo.R < 200 || win != image.Rect(48, 48, 528, 440) {
		t.Fatalf("фото в окне %v, окно %v (ждали 48,48–528,440 px)", photo, win)
	}
	var r screenVsPrint
	for y := range ph {
		for x := range pw {
			c := [3]float64{float64(page.R), float64(page.G), float64(page.B)}
			if image.Pt(x, y).In(win) { // .clip ячейки — окно; mask-image — внутри него
				k := float64(mask.NRGBAAt(x, y).A) / 255
				c = [3]float64{lerp8(page.R, photo.R, k), lerp8(page.G, photo.G, k), lerp8(page.B, photo.B, k)}
			}
			o := ov.NRGBAAt(x, y)
			k := float64(o.A) / 255
			c = [3]float64{c[0]*(1-k) + float64(o.R)*k, c[1]*(1-k) + float64(o.G)*k, c[2]*(1-k) + float64(o.B)*k}
			got := canvas.RGBAAt(ox+x, oy+y)
			d := int(max(absF(c[0]-float64(got.R)), absF(c[1]-float64(got.G)), absF(c[2]-float64(got.B))) + 0.5)
			worst, over := &r.worstFlat, &r.overFlat
			if o.A > 0 && o.A < 255 {
				worst, over = &r.worstEdge, &r.overEdge
			}
			*worst = max(*worst, d)
			if d > 2 {
				*over++
			}
		}
	}
	return r
}

// TestFrameMediaEqualsExport — экран (/media/f) совпадает с куском экспорта того же места и dpi
// (design §12): срез в альбоме не равен мете картинки (окно шире прозрачной области слева и сверху,
// уже — справа и снизу), запись без lib (как после «Собрать в папку»). Картинка ≤ 2048 px — экран и
// печать берут один источник: ±2/255 везде. Картинка 3000×2000 — больше копии экрана (2048): печать
// уменьшает исходник, экран — свою копию (Lanczos дважды); маска окна у обоих — из копии ≤ 1024 px
// (gen.WindowMask) и сходится в ±2, а на полупрозрачном крае накладки (полоса сглаживания внутреннего
// края рамки шириной ~1 px) двойное уменьшение даёт 6/255 (допуск 8) — глазом не видно; вне края — ±2.
func TestFrameMediaEqualsExport(t *testing.T) {
	rim := color.NRGBA{R: 30, G: 90, B: 160, A: 255}
	cut := [4]float64{0.08, 0.12, 0.12, 0.15}
	for _, c := range []struct {
		w, h, rim int
		edge      int // допуск на полупрозрачном крае накладки, /255
	}{{1500, 1000, 150, 2}, {3000, 2000, 300, 8}} {
		frame := ringFrame(t, testkit.TempDir(t), c.w, c.h, c.rim, c.rim, c.rim, c.rim, rim) // мета [0,1 0,15 0,1 0,15]
		r := compareFrameMedia(t, borderApp(t, frame, c.w, c.h, cut))
		t.Logf("%d×%d: край накладки — наибольшее %d/255, сверх ±2 %d px; остальное — %d/255, сверх ±2 %d px",
			c.w, c.h, r.worstEdge, r.overEdge, r.worstFlat, r.overFlat)
		if r.overFlat > 0 || r.worstEdge > c.edge {
			t.Errorf("%d×%d: экран и печать расходятся: %+v", c.w, c.h, r)
		}
	}
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// /media/f отдаёт только запись под рамочкой места (079-22, 079-35): обычное фото альбома, пустой
// и чужой ID — 404 до диска; пропавшая (a.missing) — 404 без проверки тома; том не отвечает — 404;
// картинка 15000² по заголовку — 404 быстро; рабочая — PNG.
func TestMediaFrameRefuses(t *testing.T) {
	dir := testkit.TempDir(t)
	frame := ringFrame(t, dir, 300, 200, 30, 30, 30, 30, color.NRGBA{G: 120, A: 255})
	a := borderApp(t, frame, 300, 200, [4]float64{0.1, 0.15, 0.1, 0.15})
	url := placeOf(t, a.editor().View(), brPlace).Border.URL
	q := url[strings.IndexByte(url, '?'):] + "&px=256"
	if rec := getMedia(a, url+"&px=256"); rec.Code != http.StatusOK {
		t.Fatalf("рамочка: %d", rec.Code)
	}
	old := reachable
	t.Cleanup(func() { reachable = old })
	probed := 0
	reachable = func(p string) bool { probed++; return old(p) }
	for name, path := range map[string]string{
		"фото места": "/media/f/" + brPhoto + q,
		"пустой ID":  "/media/f/" + q,
		"чужой":      "/media/f/0123456789abcdef" + q,
		"вложенный":  "/media/f/" + brFrame + "/x" + q,
		"без px":     url,
	} {
		if rec := getMedia(a, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, ждали 404", name, rec.Code)
		}
	}
	if probed != 1 { // только «без px»: запись та, путь проверен, отказ — у разбора запроса
		t.Errorf("проверок тома %d, ждали 1 (чужие ID — до диска)", probed)
	}

	// Пропавшая: в списке недостающих — 404, том и файл не трогаются.
	reachable = func(string) bool { t.Error("пропавшая: проверка тома"); return false }
	a.mu.Lock()
	a.missing = []string{brFrame}
	a.mu.Unlock()
	if rec := getMedia(a, url+"&px=256"); rec.Code != http.StatusNotFound {
		t.Fatalf("пропавшая: %d", rec.Code)
	}
	a.mu.Lock()
	a.missing = nil
	a.mu.Unlock()

	// Том не отвечает (отключённый сетевой диск): 404, файл не открывается, хотя он на месте. UNC в
	// Editor не попадает (проверка проекта и искатель библиотеки режут его раньше), CheckLocalPath
	// здесь — вторая линия.
	reachable = func(string) bool { return false }
	if rec := getMedia(a, url+"&px=256"); rec.Code != http.StatusNotFound {
		t.Fatalf("мёртвый том: %d", rec.Code)
	}

	// Огромная по заголовку — отказ без декода (079-35), быстро.
	reachable = old
	huge := filepath.Join(dir, "огромная.png")
	pngHeaderOnly(t, huge, 15000, 15000)
	p, _ := a.editor().Snapshot()
	p.Photos[brFrame] = model.PhotoRef{Path: huge, W: 300, H: 200}
	a.ed.Store(mustEditor(t, p, ""))
	t0 := time.Now()
	if rec := getMedia(a, url+"&px=256"); rec.Code != http.StatusNotFound {
		t.Fatalf("15000²: %d", rec.Code)
	}
	if d := time.Since(t0); d > testkit.Limit(200*time.Millisecond) {
		t.Fatalf("15000²: отказ за %v", d)
	}

	// Имя устройства (C:\a\CON.png): отказ CheckLocalPath, до проверки тома и файла (на Windows 11
	// «CON.png» — файл, суждение ОС подменено, как в paths). Только Windows: на других ОС устройств нет.
	t.Run("устройство", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("имена устройств — только Windows")
		}
		// Проверку проекта путь проходит, пока ОС считает «CON.png» файлом (Windows 11); у /media/f та же
		// запись на Windows 10 — устройство.
		device := false
		t.Cleanup(paths.SetDeviceOracle(func(el string) bool { return device && strings.EqualFold(el, "CON.png") }))
		p, _ = a.editor().Snapshot()
		p.Photos[brFrame] = model.PhotoRef{Path: `C:\a\CON.png`, W: 300, H: 200}
		a.ed.Store(mustEditor(t, p, ""))
		device = true
		reachable = func(string) bool { t.Error("устройство: проверка тома"); return false }
		a.mu.Lock()
		a.missing = nil
		a.mu.Unlock()
		if rec := getMedia(a, url+"&px=256"); rec.Code != http.StatusNotFound {
			t.Fatalf("устройство: %d", rec.Code)
		}
	})
}

// Одна картинка (больше 4096 px) загружена и в «Фоны», и в «Рамочки» (079-40): ключ и запись Photos
// общие, путь записи — фона (исходник). Рамочка берёт копию из «Рамочек» (4096 px) — до и после
// повторного открытия альбома: /media/f отвечает, выгрузка проходит, фон печатается исходником.
func TestFrameSharedWithBg(t *testing.T) {
	a := decorApp(t)
	file := ringFrame(t, testkit.TempDir(t), 4200, 300, 30, 30, 30, 30, color.NRGBA{R: 60, G: 90, B: 200, A: 255})
	key := importBg(t, a, file)[0]
	res, err := a.ImportDecorPaths(decor.KindFrame, "mine", []string{file})
	if err != nil || len(res.Added) != 1 || res.Added[0].ID != key {
		t.Fatalf("рамочка: %v %+v, ключ фона %s", err, res, key)
	}
	photo := ringFrame(t, testkit.TempDir(t), 120, 100, 60, 50, 60, 50, color.NRGBA{R: 220, A: 255})
	p := model.New()
	p.Photos[brPhoto] = model.PhotoRef{Path: photo, W: 120, H: 100}
	p.Spreads = []*model.Spread{{ID: brSheet, Photos: []string{brPhoto}, Grid: []*model.Place{
		{ID: brPlace, X: 20, Y: 10, W: 140, H: 10, Photo: brPhoto}}}}
	a.ed.Store(mustEditor(t, p, ""))
	if _, err := a.ApplyBackground(brSheet, "spread", key); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ApplyBorder(brSheet, []string{brPlace}, key); err != nil {
		t.Fatal(err)
	}
	id := model.LibPhotoID(key)
	check := func(stage string) {
		t.Helper()
		ref, _ := a.editor().Ref(id)
		fp, ok := a.editor().BorderPath(id)
		if !ok || fp == ref.Path || frameSide(t, ref.Path) != 4200 || frameSide(t, fp) != decor.MaxFrameSide {
			t.Fatalf("%s: рамочка %q (%v), запись %q", stage, fp, ok, ref.Path)
		}
		b := placeOf(t, a.editor().View(), brPlace).Border
		for _, u := range []string{b.URL, b.MaskURL} {
			if rec := getMedia(a, u+"&px=512"); rec.Code != http.StatusOK {
				t.Fatalf("%s: %s → %d", stage, u, rec.Code)
			}
		}
		pp, v := a.editor().Snapshot()
		sp := export.FromView(pp, v)[0]
		if len(sp.Borders) != 1 || sp.Borders[0].Path != fp || len(sp.Backdrops) != 1 || sp.Backdrops[0].Path != ref.Path ||
			sp.Photos[0].Win == nil || sp.Photos[0].Win.Path != fp {
			t.Fatalf("%s: задание печати %+v", stage, sp)
		}
		if _, err := render.Render(sp, render.Options{PageW: pp.Settings.PageW, PageH: pp.Settings.PageH, DPI: 100}); err != nil {
			t.Fatalf("%s: выгрузка: %v", stage, err)
		}
	}
	check("до сохранения")
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := a.editor().Save(path); err != nil {
		t.Fatal(err)
	}
	lp, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a.ed.Store(mustEditor(t, lp, path))
	check("после открытия")
}

// frameSide — длинная сторона картинки по заголовку.
func frameSide(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close() //nolint:errcheck // чтение
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	return max(cfg.Width, cfg.Height)
}

// Щелчок по плитке рамочки (079 §8): встроенная на место с фото и пустое — один шаг, плитка в
// недавних, адрес накладки /media/gen/frame и толщина вида (defWidth); повтор — без шага; "" —
// снять, повтор снятия — без шага; своя из библиотеки — запись с lib, накладка /media/f отвечает;
// неизвестная плитка — ошибка без шага.
func TestAppApplyBorder(t *testing.T) {
	a := decorApp(t)
	p := model.New()
	p.Photos[brPhoto] = model.PhotoRef{Path: testkit.AbsPath("a.jpg"), W: 300, H: 200}
	p.Spreads = []*model.Spread{{ID: brSheet, Photos: []string{brPhoto}, Grid: []*model.Place{
		{ID: brPlace, X: 20, Y: 10, W: 60, H: 50, Photo: brPhoto}, {ID: brEmpty, X: 100, Y: 10, W: 40, H: 40},
	}}}
	a.ed.Store(mustEditor(t, p, ""))
	cat, err := a.DecorCatalog(decor.KindFrame)
	if err != nil || len(cat.Items) == 0 {
		t.Fatalf("каталог рамочек: %v %d", err, len(cat.Items))
	}
	item := cat.Items[0].ID
	ids := []string{brPhoto, brEmpty} // фото — по ID фото, пустое — по ID места
	r := a.editor().View().Rev
	v, err := a.ApplyBorder(brSheet, ids, item)
	if err != nil || v.Rev != r+1 || !v.CanUndo {
		t.Fatalf("ApplyBorder: %v, rev %d → %d", err, r, v.Rev)
	}
	for _, id := range []string{brPlace, brEmpty} {
		b := placeOf(t, a.editor().View(), id).Border
		if b == nil || b.Preset != item || !strings.HasPrefix(b.URL, "/media/gen/frame?") || b.DefWidth <= 0 || b.Pic {
			t.Fatalf("рамочка места %s: %+v", id, b)
		}
		if rec := getMedia(a, b.URL+"&px=256"); rec.Code != http.StatusOK {
			t.Fatalf("накладка встроенной %s: %d", b.URL, rec.Code)
		}
	}
	if got := recentFrames(t, a); !slices.Equal(got, []string{item}) {
		t.Fatalf("недавние: %v", got)
	}
	if v, err = a.ApplyBorder(brSheet, ids, item); err != nil || v.Rev != r+1 {
		t.Fatalf("повтор: %v, rev %d", err, v.Rev)
	}
	if v, err = a.ApplyBorder(brSheet, ids, ""); err != nil || v.Rev != r+2 || placeOf(t, a.editor().View(), brEmpty).Border != nil {
		t.Fatalf("снять: %v, rev %d", err, v.Rev)
	}
	if v, err = a.ApplyBorder(brSheet, ids, ""); err != nil || v.Rev != r+2 {
		t.Fatalf("повтор снятия: %v, rev %d", err, v.Rev)
	}
	if v, err = a.ApplyBorder(brSheet, ids, "sa:fr-нет"); err == nil || v.Rev != r+2 {
		t.Fatalf("неизвестная плитка: %v, rev %d", err, v.Rev)
	}

	file := ringFrame(t, testkit.TempDir(t), 300, 200, 30, 30, 30, 30, color.NRGBA{B: 200, A: 255})
	res, err := a.ImportDecorPaths(decor.KindFrame, "mine", []string{file})
	if err != nil || len(res.Added) != 1 {
		t.Fatalf("загрузка рамочки: %v %+v", err, res)
	}
	own := res.Added[0].ID
	if v, err = a.ApplyBorder(brSheet, []string{brPlace}, own); err != nil || v.Rev != r+3 {
		t.Fatalf("своя: %v, rev %d", err, v.Rev)
	}
	b := placeOf(t, a.editor().View(), brPlace).Border
	ref, _ := a.editor().Ref(model.LibPhotoID(own))
	if b == nil || !b.Pic || b.Lib != own || ref.Lib != own || b.MaskURL == "" || b.DefWidth != 0 {
		t.Fatalf("своя: %+v, запись %+v", b, ref)
	}
	if rec := getMedia(a, b.URL+"&px=256"); rec.Code != http.StatusOK {
		t.Fatalf("накладка своей: %d", rec.Code)
	}
	if got := recentFrames(t, a); !slices.Equal(got, []string{own, item}) {
		t.Fatalf("недавние после своей: %v", got)
	}
}

// recentFrames — недавние плитки окна «Рамочки».
func recentFrames(t *testing.T, a *App) []string {
	t.Helper()
	cat, err := a.DecorCatalog(decor.KindFrame)
	if err != nil {
		t.Fatal(err)
	}
	return cat.Recent
}

// «Загрузить своё…» в окне «Рамочки»: свой заголовок и фильтр (PNG и SVG — JPEG окна не имеет),
// своя последняя папка — папки фонов и стикеров не сдвигаются.
func TestImportFrameDialog(t *testing.T) {
	a := decorApp(t)
	dir := testkit.TempDir(t)
	file := ringFrame(t, dir, 300, 200, 30, 30, 30, 30, color.NRGBA{B: 90, A: 255})
	old := openFilesDialog
	t.Cleanup(func() { openFilesDialog = old })
	var opts wr.OpenDialogOptions
	openFilesDialog = func(_ context.Context, o wr.OpenDialogOptions) ([]string, error) {
		opts = o
		return []string{file}, nil
	}
	res, err := a.ImportDecor(decor.KindFrame, "mine")
	if err != nil || len(res.Added) != 1 || opts.Title != "Свои рамочки" || len(opts.Filters) != 1 || opts.Filters[0].Pattern != "*.png;*.svg" {
		t.Fatalf("рамочки: %v %+v, диалог %+v", err, res, opts)
	}
	if got := prefs.DialogDir(a.cfgDir, dirFrame); got != dir {
		t.Errorf("папка рамочек %q, ждали %q", got, dir)
	}
	if prefs.DialogDir(a.cfgDir, dirDecor) == dir || prefs.DialogDir(a.cfgDir, dirSticker) == dir {
		t.Error("рамочки сдвинули папку фонов или стикеров")
	}
}

// relinkLib для рамочек: своя рамочка альбома без файла (альбом с другого компьютера) находится,
// когда тот же PNG загружен в библиотеку, — запись получает путь, /media/f начинает отвечать.
func TestImportFrameRelinks(t *testing.T) {
	a := decorApp(t)
	file := ringFrame(t, testkit.TempDir(t), 300, 200, 30, 30, 30, 30, color.NRGBA{R: 200, A: 255})
	res, err := a.ImportDecorPaths(decor.KindFrame, "mine", []string{file})
	if err != nil || len(res.Added) != 1 {
		t.Fatalf("загрузка: %v %+v", err, res)
	}
	key := res.Added[0].ID
	id := model.LibPhotoID(key)
	p := model.New()
	p.Photos[brPhoto] = model.PhotoRef{Path: testkit.AbsPath("a.jpg"), W: 300, H: 200}
	p.Photos[id] = model.PhotoRef{Lib: key, W: 300, H: 200} // путь не найден на этом компьютере
	p.Spreads = []*model.Spread{{ID: brSheet, Photos: []string{brPhoto}, Grid: []*model.Place{
		{ID: brPlace, X: 20, Y: 10, W: 60, H: 50, Photo: brPhoto, Border: &model.Border{Photo: id, Cut: [4]float64{0.1, 0.15, 0.1, 0.15}}},
	}}}
	a.ed.Store(mustEditor(t, p, ""))
	url := placeOf(t, a.editor().View(), brPlace).Border.URL + "&px=256"
	if rec := getMedia(a, url); rec.Code != http.StatusNotFound {
		t.Fatalf("без файла: %d", rec.Code)
	}
	r := a.editor().View().Rev
	if _, err := a.ImportDecorPaths(decor.KindFrame, "mine", []string{file}); err != nil { // «уже есть» — тоже находит
		t.Fatal(err)
	}
	ref, _ := a.editor().Ref(id)
	if ref.Path == "" || a.editor().View().Rev != r+1 {
		t.Fatalf("не нашлась: %+v, rev %d → %d", ref, r, a.editor().View().Rev)
	}
	if rec := getMedia(a, url); rec.Code != http.StatusOK {
		t.Fatalf("после загрузки: %d", rec.Code)
	}
}

// TestMediaFrameAfterCollect — после «Собрать в папку» (079-22, 079-27) у записи рамочки lib снят,
// файл лежит в «Свои фоны и стикеры» папки сбора; собранный альбом открыт — накладка и маска по
// адресам из View отвечают (ключом библиотеки URL было бы не построить), фото места — по-прежнему 404.
func TestMediaFrameAfterCollect(t *testing.T) {
	e := newCollectEnv(t, 1)
	a := e.a
	a.openDecor()
	t.Cleanup(func() { salbum.SetLibFinder(nil); salbum.SetFrameFinder(nil) })
	file := ringFrame(t, testkit.TempDir(t), 300, 200, 30, 30, 30, 30, color.NRGBA{R: 90, A: 255})
	res, err := a.ImportDecorPaths(decor.KindFrame, "mine", []string{file})
	if err != nil || len(res.Added) != 1 {
		t.Fatalf("загрузка: %v %+v", err, res)
	}
	p, _ := a.editor().Snapshot()
	s := p.Spreads[0]
	s.Grid = []*model.Place{{ID: brPlace, X: 5, Y: 5, W: 50, H: 30, Photo: s.Photos[0]}}
	a.ed.Store(mustEditor(t, p, e.file))
	if _, err := a.ApplyBorder(s.ID, []string{brPlace}, res.Added[0].ID); err != nil {
		t.Fatal(err)
	}
	if dir, msg, _, stopped := e.run(t, false); msg != "" || stopped || dir != e.book() {
		t.Fatalf("сбор: %q %q %v", dir, msg, stopped)
	}
	lp, err := salbum.Load(e.albumFile())
	if err != nil {
		t.Fatal(err)
	}
	a.ed.Store(mustEditor(t, lp, e.albumFile()))
	v := a.editor().View()
	b := placeOf(t, v, brPlace).Border
	if b == nil || !b.Pic || b.Lib != "" || b.MaskURL == "" {
		t.Fatalf("рамочка собранного: %+v", b)
	}
	ref, _ := a.editor().Ref(b.Photo)
	if want := filepath.Join(e.book(), collect.DirOwn); ref.Lib != "" || filepath.Dir(ref.Path) != want {
		t.Fatalf("запись рамочки: %+v, ждали файл в %s", ref, want)
	}
	for _, u := range []string{b.URL, b.MaskURL} {
		if rec := getMedia(a, u+"&px=256"); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("%s после сбора: %d", u, rec.Code)
		}
	}
	q := b.URL[strings.IndexByte(b.URL, '?'):] + "&px=256"
	if rec := getMedia(a, "/media/f/"+v.Spreads[0].Photos[0]+q); rec.Code != http.StatusNotFound {
		t.Fatalf("фото места через /media/f: %d", rec.Code)
	}
}

// Пропавшие рамочки (079-38): своя из библиотеки без файла и картинка без lib, чей файл стёрт, —
// в State.missing и в событии missing; своя — ещё и в View.libPhotos (полоса считает её «своей»);
// у рамочки пустого места — тоже. Фото места на месте — не пропавшее.
func TestFrameMissing(t *testing.T) {
	const key = "my:00112233445566aa"
	own := model.LibPhotoID(key)
	a := borderApp(t, filepath.Join(testkit.TempDir(t), "нет.png"), 300, 200, [4]float64{0.1, 0.15, 0.1, 0.15})
	p, _ := a.editor().Snapshot()
	p.Photos[own] = model.PhotoRef{Lib: key, W: 300, H: 200} // библиотека этого компьютера её не знает
	p.Spreads[0].Grid[1].Border = &model.Border{Photo: own}
	a.ed.Store(mustEditor(t, p, ""))
	var events [][]string
	a.emit = func(name string, data ...any) {
		if name == "missing" {
			events = append(events, data[0].([]string))
		}
	}
	a.refreshMissing()
	waitMissing(t, a)
	want := []string{brFrame, own}
	slices.Sort(want)
	if m := a.GetState().Missing; !slices.Equal(m, want) {
		t.Fatalf("State.missing %v, ждали %v", m, want)
	}
	if len(events) != 1 || !slices.Equal(events[0], want) {
		t.Fatalf("событие missing: %v", events)
	}
	if lib := a.editor().View().LibPhotos; !slices.Equal(lib, []string{own}) {
		t.Fatalf("View.libPhotos %v", lib)
	}
	if n := a.MissingNames(); n[brFrame] != "нет.png" || n[own] != "" {
		t.Errorf("имена пропавших: %v", n)
	}
}

// «Найти…» (RelinkFolderPath) находит картинку рамочки без lib по имени и размерам (079-38): путь
// новый, ID пересчитан и в рамочке, папка поиска в папки альбома не попадает (PNG рамочки — не лента),
// /media/f снова отвечает, один шаг отмены. Своя с lib папкой не ищется — её находит RelinkLib.
func TestRelinkFolderFrame(t *testing.T) {
	const key = "my:00112233445566aa"
	own := model.LibPhotoID(key)
	old := filepath.Join(testkit.TempDir(t), "рамка.png") // файла нет: альбом с другого компьютера
	dir := testkit.TempDir(t)
	moved := ringFrame(t, dir, 300, 200, 30, 30, 30, 30, color.NRGBA{G: 150, A: 255})
	a := borderApp(t, old, 300, 200, [4]float64{0.1, 0.15, 0.1, 0.15})
	p, _ := a.editor().Snapshot()
	p.Photos[own] = model.PhotoRef{Lib: key, W: 300, H: 200}
	p.Spreads[0].Grid[1].Border = &model.Border{Photo: own}
	folders := slices.Clone(p.Folders)
	a.ed.Store(mustEditor(t, p, ""))
	r := a.editor().View()
	res, err := a.RelinkFolderPath(dir)
	if err != nil || res.Found != 1 || res.Missing != 0 || res.View.Rev != r.Rev+1 {
		t.Fatalf("RelinkFolderPath: %v %+v (rev %d)", err, res, r.Rev)
	}
	b := placeOf(t, res.View, brPlace).Border
	ref, _ := a.editor().Ref(b.Photo)
	if b.Photo == brFrame || ref.Path != moved || ref.Lib != "" || !strings.HasPrefix(b.URL, "/media/f/"+b.Photo+"?") {
		t.Fatalf("рамочка после поиска: %+v, запись %+v", b, ref)
	}
	if q, _ := a.editor().Snapshot(); !slices.Equal(q.Folders, folders) {
		t.Fatalf("папки альбома %v, было %v", q.Folders, folders)
	}
	if ownRef, _ := a.editor().Ref(own); ownRef.Path != "" {
		t.Fatalf("своя с lib перепривязана папкой: %+v", ownRef)
	}
	if rec := getMedia(a, b.URL+"&px=256"); rec.Code != http.StatusOK {
		t.Fatalf("/media/f после поиска: %d", rec.Code)
	}
	if v := a.editor().Undo(); placeOf(t, v, brPlace).Border.Photo != brFrame {
		t.Fatalf("отмена: %+v", placeOf(t, v, brPlace).Border)
	}
}
