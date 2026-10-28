package app

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Стикеры (спека 048 Ф2, design §8): постановка и замена из каталога, SVG через окно.

// stickerAt — копия стикера id из снимка проекта (nil — нет).
func stickerAt(a *App, id string) *model.Sticker {
	p, _ := a.editor().Snapshot()
	s, i := p.FindSticker(id)
	if s == nil {
		return nil
	}
	return s.Stickers[i].Clone()
}

// importSticker — свой стикер-растр через ImportDecorPaths; ID плитки (my:<hex>).
func importSticker(t *testing.T, a *App, w, h int) string {
	t.Helper()
	file := testkit.PNG(t, testkit.TempDir(t), "свой.png", w, h, color.NRGBA{R: 200, A: 255})
	res, err := a.ImportDecorPaths(decor.KindSticker, "mine", []string{file})
	if err != nil || len(res.Added) != 1 || len(res.Skipped) != 0 {
		t.Fatalf("загрузка стикера: %v %+v", err, res)
	}
	return res.Added[0].ID
}

// firstBuiltin — первая встроенная плитка стикера с пропорцией 1 (рамка без растяжения).
func firstBuiltin(t *testing.T, a *App) string {
	t.Helper()
	cat, err := a.DecorCatalog(decor.KindSticker)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range cat.Items {
		if !it.Own && it.Aspect == 1 {
			return it.ID
		}
	}
	t.Fatal("нет встроенного квадратного стикера")
	return ""
}

func near(a, b float64) bool { return math.Abs(a-b) < 0.011 }

// Встроенный и свой стикеры на двух листах: по центру страницы, ¼ её ширины, один шаг на
// каждый, ID в итоге, недавние новые первыми. Ошибки (неизвестная плитка, лист) — без правки
// и без недавних. Отмена снимает стикер; круг записи и чтения сохраняет рисунок и ключ своего.
func TestAddSticker(t *testing.T) {
	a := decorApp(t)
	builtin := firstBuiltin(t, a)
	own := importSticker(t, a, 60, 30)
	ed := a.editor()
	sw, sh := ed.SheetSize()

	rev := ed.View().Rev
	for _, bad := range []string{"sa:st-нет", "", "my:0123456789abcdef", "sa:dots"} {
		r, err := a.AddSticker("s1", 0, bad)
		if !errors.Is(err, decor.ErrNotFound) || r.ID != "" || r.View.Rev != rev {
			t.Fatalf("%q: %v, id %q, rev %d→%d", bad, err, r.ID, rev, r.View.Rev)
		}
	}
	// Нет листа — холостая без ошибки, но и без ID: выделять нечего.
	if r, err := a.AddSticker("нет", 0, builtin); err != nil || r.ID != "" || r.View.Rev != rev {
		t.Fatalf("нет листа: %v, id %q, rev %d→%d", err, r.ID, rev, r.View.Rev)
	}
	if rec := prefs.RecentDecor(a.cfgDir, decor.KindSticker); len(rec) != 0 {
		t.Fatalf("ошибка попала в недавние: %v", rec)
	}

	r1, err := a.AddSticker("s1", -5, builtin) // страница зажимается в 0
	if err != nil || r1.ID == "" || r1.View.Rev != rev+1 || !r1.View.CanUndo {
		t.Fatalf("встроенный: %v %+v", err, r1.ID)
	}
	r2, err := a.AddSticker("s2", 1, own)
	if err != nil || r2.ID == "" || r2.ID == r1.ID || r2.View.Rev != rev+2 {
		t.Fatalf("свой: %v %q", err, r2.ID)
	}
	st1, st2 := stickerAt(a, r1.ID), stickerAt(a, r2.ID)
	if st1 == nil || st1.Art == nil || st1.Photo != "" || !near(st1.X+st1.W/2, sw/4) || !near(st1.W, sw/8) || !near(st1.H, st1.W) {
		t.Fatalf("встроенный на странице 0 листа %v×%v: %+v", sw, sh, st1)
	}
	if st2 == nil || st2.Art != nil || st2.Photo != model.LibPhotoID(own) || !near(st2.X+st2.W/2, sw*3/4) || !near(st2.H, st2.W/2) {
		t.Fatalf("свой на странице 1: %+v", st2)
	}
	if ref, _ := ed.Ref(st2.Photo); ref.Lib != own || ref.Path == "" {
		t.Fatalf("свой без ключа или пути: %+v", ref)
	}
	if rec := prefs.RecentDecor(a.cfgDir, decor.KindSticker); !slices.Equal(rec, []string{own, builtin}) {
		t.Errorf("недавние %v", rec)
	}

	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := ed.Save(path); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if s, i := p.FindSticker(r1.ID); s == nil || s.Stickers[i].Art == nil || s.Stickers[i].Art.Kind != st1.Art.Kind || !near(s.Stickers[i].W, st1.W) {
		t.Errorf("встроенный после чтения: %v", s)
	}
	if s, i := p.FindSticker(r2.ID); s == nil || s.Stickers[i].Photo != st2.Photo || p.Photos[st2.Photo].Lib != own {
		t.Errorf("свой после чтения: %v %+v", s, p.Photos[st2.Photo])
	}

	if v := a.Undo(); v.Rev == r2.View.Rev || stickerAt(a, r2.ID) != nil || stickerAt(a, r1.ID) == nil {
		t.Fatal("отмена не сняла последний стикер")
	}
}

// «Заменить…»: центр, ширина и поворот стоят, высота — по новой пропорции; та же плитка —
// холостая без шага; неизвестная — ошибка без правки; отмена возвращает рисунок.
func TestReplaceSticker(t *testing.T) {
	a := decorApp(t)
	builtin := firstBuiltin(t, a)
	own := importSticker(t, a, 80, 40)
	r, err := a.AddSticker("s1", 1, builtin)
	if err != nil {
		t.Fatal(err)
	}
	ed := a.editor()
	st := stickerAt(a, r.ID)
	ed.SetStickerBox("s1", r.ID, st.X, st.Y, st.W, st.H, 30, "")
	before := stickerAt(a, r.ID)

	v, err := a.ReplaceSticker("s1", r.ID, own)
	got := stickerAt(a, r.ID)
	if err != nil || got.Art != nil || got.Photo != model.LibPhotoID(own) || got.Rot != 30 ||
		!near(got.W, before.W) || !near(got.H, before.W/2) || !near(got.X+got.W/2, before.X+before.W/2) || !near(got.Y+got.H/2, before.Y+before.H/2) {
		t.Fatalf("замена: %v %+v, было %+v", err, got, before)
	}
	if v2, err := a.ReplaceSticker("s1", r.ID, own); err != nil || v2.Rev != v.Rev {
		t.Errorf("та же плитка — правка: %v rev %d→%d", err, v.Rev, v2.Rev)
	}
	if v2, err := a.ReplaceSticker("s1", r.ID, "sa:st-нет"); !errors.Is(err, decor.ErrNotFound) || v2.Rev != v.Rev {
		t.Errorf("неизвестная плитка: %v rev %d→%d", err, v.Rev, v2.Rev)
	}
	if rec := prefs.RecentDecor(a.cfgDir, decor.KindSticker); !slices.Equal(rec, []string{own, builtin}) {
		t.Errorf("недавние %v", rec)
	}
	// Холостая замена (неизвестный стикер, чужой лист) в недавние не кладёт: builtin не всплывает.
	for _, c := range [][2]string{{"s1", "нет"}, {"s2", r.ID}} {
		if v2, err := a.ReplaceSticker(c[0], c[1], builtin); err != nil || v2.Rev != v.Rev {
			t.Errorf("холостая %v: %v rev %d→%d", c, err, v.Rev, v2.Rev)
		}
	}
	if rec := prefs.RecentDecor(a.cfgDir, decor.KindSticker); !slices.Equal(rec, []string{own, builtin}) {
		t.Errorf("холостая замена попала в недавние: %v", rec)
	}
	a.Undo()
	if got := stickerAt(a, r.ID); got.Art == nil || got.Photo != "" || !near(got.H, before.H) {
		t.Fatalf("отмена замены: %+v", got)
	}
}

// Без библиотеки — ошибка без правки и без паники.
func TestStickerNoLibrary(t *testing.T) {
	a := bgApp(t)
	rev := a.editor().View().Rev
	if r, err := a.AddSticker("s1", 0, "sa:st-heart"); !errors.Is(err, errNoLibrary) || r.View.Rev != rev || r.ID != "" {
		t.Fatalf("AddSticker: %v %+v", err, r.ID)
	}
	if v, err := a.ReplaceSticker("s1", "x", "sa:st-heart"); !errors.Is(err, errNoLibrary) || v.Rev != rev {
		t.Fatalf("ReplaceSticker: %v", err)
	}
	if it, err := a.AddDecorPNG("0123456789abcdef", ""); !errors.Is(err, errNoLibrary) || it.ID != "" {
		t.Fatalf("AddDecorPNG: %v %+v", err, it)
	}
}

// pngBase64 — непрозрачный PNG w×h в base64, как присылает окно.
func pngBase64(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = 200, 255
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

// SVG стикера: загрузка вместе с растром даёт растр в Added и задание в Svg; PNG от окна по
// токену — плитка «своё», которую можно поставить; токен одноразовый, чужой размер — отказ.
// Фоны заданий не дают (Svg — пустой срез, не nil). Запись своего без своих в альбоме —
// холостая перепривязка без шага.
func TestImportStickerSVG(t *testing.T) {
	a := decorApp(t)
	dir := testkit.TempDir(t)
	svg := filepath.Join(dir, "Лента.svg")
	if err := os.WriteFile(svg, []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50"><rect width="10" height="10" fill="#c00"/></svg>`), 0o644); err != nil {
		t.Fatal(err)
	}
	evil := filepath.Join(dir, "вредный.svg")
	if err := os.WriteFile(evil, []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><script>alert(1)</script></svg>`), 0o644); err != nil {
		t.Fatal(err)
	}
	raster := testkit.PNG(t, dir, "растр.png", 30, 30, color.NRGBA{G: 200, A: 255})
	res, err := a.ImportDecorPaths(decor.KindSticker, "mine", []string{svg, raster, evil})
	if err != nil || len(res.Added) != 1 || len(res.Svg) != 1 || len(res.Skipped) != 1 || res.Skipped[0].Name != "вредный" {
		t.Fatalf("загрузка: %v %+v", err, res)
	}
	job := res.Svg[0]
	if job.Token == "" || job.Name != "Лента" || job.W != 2*job.H || max(job.W, job.H) != decor.MaxStickerSide {
		t.Fatalf("задание: %+v", job)
	}
	if raw, err := base64.StdEncoding.DecodeString(job.Data); err != nil || !strings.Contains(string(raw), "<rect") {
		t.Fatalf("данные задания: %v", err)
	}

	rev := a.editor().View().Rev
	if _, err := a.AddDecorPNG(job.Token, pngBase64(t, job.W, job.H+1)); err == nil {
		t.Fatal("чужой размер принят")
	}
	if _, err := a.AddDecorPNG(job.Token, pngBase64(t, job.W, job.H)); !errors.Is(err, decor.ErrToken) {
		t.Fatalf("токен после отказа живой: %v", err)
	}

	// Новое задание на тот же SVG: токен прежнего израсходован.
	again, err := a.ImportDecorPaths(decor.KindSticker, "mine", []string{svg})
	if err != nil || len(again.Svg) != 1 {
		t.Fatalf("повтор SVG: %v %+v", err, again)
	}
	it, err := a.AddDecorPNG(again.Svg[0].Token, pngBase64(t, job.W, job.H))
	if err != nil || !it.Own || it.Name != "Лента" || !model.ValidLibKey(it.ID) || !near(it.Aspect, 2) {
		t.Fatalf("PNG от окна: %v %+v", err, it)
	}
	if _, err := a.AddDecorPNG(again.Svg[0].Token, pngBase64(t, job.W, job.H)); !errors.Is(err, decor.ErrToken) {
		t.Errorf("токен второй раз: %v", err)
	}
	if v := a.editor().View(); v.Rev != rev || v.CanUndo {
		t.Errorf("запись своего — правка проекта: rev %d→%d", rev, v.Rev)
	}
	cat, err := a.DecorCatalog(decor.KindSticker)
	if err != nil || !slices.ContainsFunc(cat.Items, func(c DecorItem) bool { return c.ID == it.ID }) {
		t.Fatalf("плитки нет в каталоге: %v", err)
	}
	r, err := a.AddSticker("s1", 0, it.ID)
	if st := stickerAt(a, r.ID); err != nil || st == nil || !near(st.H, st.W/2) {
		t.Fatalf("SVG-стикер на лист: %v %+v", err, st)
	}

	bg, err := a.ImportDecorPaths(decor.KindBg, "mine", []string{testkit.JPEG(t, dir, "фон.jpg", 40, 30)})
	if err != nil || bg.Svg == nil || len(bg.Svg) != 0 {
		t.Errorf("фоны: %v %+v", err, bg.Svg)
	}
}

// «Копировать оформление» (049 Ф2): результат — GridResult с видом (Left 0), места и «Авто»
// не трогаются; повтор — тот же rev; план до правки; ошибка области — текущий вид.
func TestAppCopyDecor(t *testing.T) {
	a := gridApp(t)
	a.editor().SetBackground("s1", model.ScopeLeft, "#aabbcc")
	to := []model.GridTarget{{Spread: "s2", Scope: model.ScopeSpread}}
	if plan := a.PlanCopyDecor("s1", model.ScopeLeft, to, true, true); plan != (model.GridPlan{}) {
		t.Fatalf("план: %+v", plan)
	}
	r, err := a.CopyDecor("s1", model.ScopeLeft, to, true, true)
	if err != nil || r.Left != 0 || r.View.Spreads[1].BgSet != [2]string{"#aabbcc", "#aabbcc"} || r.View.Spreads[1].Grid || len(r.View.Spreads[0].Places) != 2 {
		t.Fatalf("CopyDecor: %v, %+v", err, r.View.Spreads)
	}
	rev := r.View.Rev
	if r, err = a.CopyDecor("s1", model.ScopeLeft, to, true, true); err != nil || r.View.Rev != rev {
		t.Fatalf("повтор: %v, rev %d → %d", err, rev, r.View.Rev)
	}
	if r, err = a.CopyDecor("s1", "nope", to, false, false); err == nil || r.View.Rev != rev || len(r.View.Spreads) != 2 {
		t.Fatalf("ошибка области: %v, %+v", err, r)
	}
}

// syncBuf — буфер log под замком: net/http пишет в log из своей горутины.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// idleNoise — строка net/http об ответе в простаивающем соединении (шум log.txt в wails dev).
const idleNoise = "Unsolicited response received on idle HTTP channel"

// idleNoiseLog делает запрос к серверу, который отвечает 200 по keep-alive, а затем пишет
// «400 Bad Request» в простаивающее соединение (так делает Vite в wails dev), и возвращает
// всё, что за это время попало в log. Паузы — через testkit.Limit: на занятой машине readLoop
// клиента читает лишнее позже; шум ждём до срока, а не фиксированную паузу.
func idleNoiseLog(t *testing.T, tr *http.Transport) string {
	t.Helper()
	var buf syncBuf
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	sent := make(chan struct{})
	go func() {
		defer close(sent)
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		if _, err := http.ReadRequest(bufio.NewReader(c)); err != nil {
			return
		}
		io.WriteString(c, "HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
		time.Sleep(testkit.Limit(50 * time.Millisecond)) // клиент успевает вернуть соединение в простой
		io.WriteString(c, "HTTP/1.1 400 Bad Request\r\n\r\n")
	}()

	cli := &http.Client{Transport: tr, Timeout: testkit.Limit(5 * time.Second)}
	resp, err := cli.Get("http://" + ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	<-sent
	// readLoop клиента успевает прочитать лишнее: шум пришёл — дальше не ждём.
	for end := time.Now().Add(testkit.Limit(2 * time.Second)); time.Now().Before(end) && !strings.Contains(buf.String(), idleNoise); {
		time.Sleep(10 * time.Millisecond)
	}
	tr.CloseIdleConnections()
	return buf.String()
}

// Сторож шума в log.txt: обычный Transport жалуется на ответ в простаивающем соединении
// (контроль: тест ловит именно эту строку), с quietTransport соединение не простаивает.
func TestQuietTransportNoIdleNoise(t *testing.T) {
	if out := idleNoiseLog(t, &http.Transport{}); !strings.Contains(out, idleNoise) {
		t.Fatalf("контроль: обычный Transport не дал %q, log: %q", idleNoise, out)
	}
	tr := &http.Transport{}
	quietTransport(tr)
	if out := idleNoiseLog(t, tr); strings.Contains(out, idleNoise) {
		t.Fatalf("quietTransport не заглушил шум: %q", out)
	}
}

// HMR Vite — WebSocket через обратный прокси: Upgrade с отключёнными keep-alive обязан работать.
func TestQuietTransportKeepsUpgrade(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, rw, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer c.Close()
		io.WriteString(rw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		rw.Flush()
		line, _ := rw.ReadString('\n')
		io.WriteString(rw, "echo "+line)
		rw.Flush()
	}))
	defer backend.Close()
	u, _ := url.Parse(backend.URL)
	proxy := httputil.NewSingleHostReverseProxy(u)
	tr := &http.Transport{}
	quietTransport(tr)
	proxy.Transport = tr
	front := httptest.NewServer(proxy)
	defer front.Close()

	c, err := net.Dial("tcp", front.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(testkit.Limit(5 * time.Second)))
	io.WriteString(c, "GET /ws HTTP/1.1\r\nHost: x\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("статус %d, ждали 101", resp.StatusCode)
	}
	io.WriteString(c, "ping\n")
	got, _ := br.ReadString('\n')
	if got != "echo ping\n" {
		t.Fatalf("после Upgrade пришло %q", got)
	}
}
