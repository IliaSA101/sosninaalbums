package decor

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// writePNG пишет PNG w×h: левая половина красная непрозрачная, правая прозрачная.
func writePNG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	return testkit.PNG(t, dir, name, w, h, color.NRGBA{200, 10, 10, 255})
}

var (
	renderMu    sync.Mutex
	renderCache = map[[2]int][]byte{}
)

// renderPNG — PNG w×h, как его прислало бы окно из SVG (однотонный, с альфой; кэшируется).
func renderPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	renderMu.Lock()
	defer renderMu.Unlock()
	if b, ok := renderCache[[2]int{w, h}]; ok {
		return b
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+3] = 200, 255
	}
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	renderCache[[2]int{w, h}] = buf.Bytes()
	return buf.Bytes()
}

func b64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }

// clearPNG — PNG из готовой картинки (прозрачные и почти прозрачные для AddPNG).
func clearPNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func decodePNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// stickerSet — библиотека с тремя стикерами в «Свадьбе» (малый PNG, JPEG, широкий PNG > 4096 px)
// и двумя отказами (нет файла, не картинка).
type stickerSet struct {
	s           *Store
	cfg         string
	files       []string
	small, wide string
	res         ImportResult
}

func newStickerSet(t *testing.T) stickerSet {
	t.Helper()
	src := testkit.TempDir(t)
	st := stickerSet{small: writePNG(t, src, "Звезда.png", 60, 40), wide: writePNG(t, src, "Лента.png", 5000, 100)}
	st.s, st.cfg = openTemp(t)
	jpg := testkit.JPEG(t, src, "Фото.jpg", 50, 30)
	st.files = []string{st.small, jpg, st.wide, filepath.Join(src, "нет.png"), writeFile(t, src, "битый.png", []byte("не картинка"))}
	res, err := st.s.Import(KindSticker, "wedding", st.files)
	if err != nil || len(res.Added) != 3 || len(res.Skipped) != 2 || len(res.Svg) != 0 {
		t.Fatalf("итог %+v, %v", res, err)
	}
	st.res = res
	return st
}

// TestDecorImportSticker — стикеры JPEG и PNG: копия под ключом от байтов исходника, малый PNG
// байт в байт, длинная сторона > 4096 уменьшается в PNG (пропорция и альфа остаются), записи
// правильные, временных файлов нет.
func TestDecorImportSticker(t *testing.T) {
	st := newStickerSet(t)
	s, res := st.s, st.res
	for i, it := range res.Added {
		if it.ID != keyOf(t, st.files[i]) || !it.Own || it.Theme != "wedding" || it.Aspect <= 0 {
			t.Errorf("плитка %d: %+v", i, it)
		}
	}
	if b, _ := os.ReadFile(mustFind(t, s, res.Added[0].ID)); !bytes.Equal(b, mustRead(t, st.small)) {
		t.Error("малый PNG изменён")
	}
	wp := mustFind(t, s, res.Added[2].ID)
	if filepath.Ext(wp) != ".png" || filepath.Base(filepath.Dir(wp)) != "Стикеры" {
		t.Errorf("широкий лежит в %s", wp)
	}
	img := decodePNG(t, wp)
	if b := img.Bounds(); b.Dx() != 4096 || b.Dy() != 82 {
		t.Errorf("широкий после уменьшения %v, ждали 4096×82", b)
	}
	if _, _, _, a := img.At(10, 40).RGBA(); a == 0 {
		t.Error("левая половина стала прозрачной")
	}
	if _, _, _, a := img.At(4000, 40).RGBA(); a != 0 {
		t.Error("правая половина потеряла прозрачность")
	}
	if e := s.entryOf(KindSticker, res.Added[2].ID); e == nil || e.W != 4096 || e.H != 82 || e.Ext != "png" || e.Bytes <= 0 {
		t.Errorf("запись широкого: %+v", e)
	}
	if left, _ := filepath.Glob(filepath.Join(st.cfg, RootName, "Стикеры", tmpPrefix+"*")); len(left) != 0 {
		t.Errorf("остатки импорта: %v", left)
	}
}

// TestDecorImportStickerAgain — повтор — «уже есть» (и для уменьшенного: ключ от исходника);
// скрытый возвращается в свою тему и не дублируется; тот же файл как фон — другой вид.
func TestDecorImportStickerAgain(t *testing.T) {
	st := newStickerSet(t)
	s, res := st.s, st.res
	again, _ := s.Import(KindSticker, "mine", []string{st.small, st.wide})
	if len(again.Added) != 0 || len(again.Skipped) != 2 || !strings.Contains(again.Skipped[0].Why, "уже есть") || !strings.Contains(again.Skipped[1].Why, "уже есть") {
		t.Errorf("повтор: %+v", again)
	}
	if err := s.Hide(KindSticker, res.Added[0].ID); err != nil {
		t.Fatal(err)
	}
	back, _ := s.Import(KindSticker, "mine", []string{st.small})
	if len(back.Added) != 1 || back.Added[0].ID != res.Added[0].ID || back.Added[0].Theme != "wedding" {
		t.Errorf("возврат скрытого: %+v", back)
	}
	if bg, _ := s.Import(KindBg, "mine", []string{st.small}); len(bg.Added) != 1 {
		t.Errorf("тот же файл как фон: %+v", bg)
	}
	if n, _ := s.usage(KindSticker); n != 3 {
		t.Errorf("стикеров в реестре %d, ждали 3", n)
	}
}

// TestDecorImportStickerReload — Save → Load: после открытия реестр и файлы на месте, плитки свои,
// скрытый остаётся скрытым.
func TestDecorImportStickerReload(t *testing.T) {
	st := newStickerSet(t)
	res := st.res
	if err := st.s.Hide(KindSticker, res.Added[1].ID); err != nil {
		t.Fatal(err)
	}
	s2 := Open(st.cfg)
	cat, err := s2.Catalog(KindSticker, nil, 600, 300)
	if err != nil {
		t.Fatal(err)
	}
	var own []string
	for _, it := range cat.Items {
		if it.Own {
			own = append(own, it.ID)
		}
	}
	if len(own) != 2 || own[0] == res.Added[1].ID || own[1] == res.Added[1].ID {
		t.Errorf("после открытия свои %v, ждали два без скрытого", own)
	}
	if e := s2.entryOf(KindSticker, res.Added[2].ID); e == nil || e.W != 4096 || e.H != 82 || e.Theme != "wedding" {
		t.Errorf("запись после открытия: %+v", e)
	}
	if p, ok := s2.Find(res.Added[1].ID); !ok || !fileExists(p) {
		t.Errorf("Find скрытого после открытия: %q %v", p, ok)
	}
	if readReg(t, st.cfg).Items[1].Kind != KindSticker {
		t.Error("вид записи не записан")
	}
}

// TestDecorImportStickerLimits — пределы и отказы: больше 30 МБ, чужое расширение, 100 за раз,
// ссылка не на локальный диск, пустой список; ничего из отказанного не остаётся на диске.
func TestDecorImportStickerLimits(t *testing.T) {
	src := testkit.TempDir(t)
	s, cfg := openTemp(t)
	big := writeFile(t, src, "огромный.png", make([]byte, MaxStickerBytes+1))
	exact := writeFile(t, src, "ровно.png", append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, MaxStickerBytes-8)...)) // на пределе, но не картинка
	gif := writeFile(t, src, "анимация.gif", []byte("GIF89a"))
	res, err := s.Import(KindSticker, "mine", []string{big, exact, gif, "rel.png", ""})
	if err != nil || len(res.Added) != 0 || len(res.Skipped) != 5 {
		t.Fatalf("итог %+v, %v", res, err)
	}
	if !strings.Contains(res.Skipped[0].Why, "30 МБ") || !strings.Contains(res.Skipped[2].Why, "JPEG, PNG и SVG") || !strings.Contains(res.Skipped[3].Why, "локальном") {
		t.Errorf("причины: %+v", res.Skipped)
	}
	if strings.Contains(res.Skipped[1].Why, "30 МБ") {
		t.Errorf("файл ровно на пределе отклонён по размеру: %q", res.Skipped[1].Why)
	}
	for _, sk := range res.Skipped {
		if strings.Contains(sk.Why, src) {
			t.Errorf("путь в причине: %q", sk.Why)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(cfg, RootName, "Стикеры", "*")); len(left) != 0 {
		t.Errorf("на диске осталось %v", left)
	}
	// 100 за раз: сто первый — пропуск.
	var many []string
	for i := range MaxImport + 1 {
		many = append(many, writePNG(t, src, "m"+string(rune('a'+i%26))+string(rune('a'+i/26))+".png", 8+i, 8))
	}
	res, _ = s.Import(KindSticker, "mine", many)
	if len(res.Added) != MaxImport || len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Why, "100") {
		t.Errorf("сто один файл: добавлено %d, пропущено %+v", len(res.Added), res.Skipped)
	}
	if res, err := s.Import(KindSticker, "mine", nil); err != nil || len(res.Added)+len(res.Skipped)+len(res.Svg) != 0 {
		t.Errorf("пустой список: %+v %v", res, err)
	}
}

// svgClean — чистый SVG для тестов; ключ — от его байт.
var svgClean = []byte(svgDoc(`<rect width="10" height="10" fill="#c00"/>`))

// TestDecorImportSVG — чистый SVG не пишет запись сразу, а выдаёт окну задание (токен, SVG в
// base64, размер PNG); вредный и слишком большой — отказ с причиной; после AddPNG запись под
// ключом SVG; тот же SVG — «уже есть», скрытый возвращается без задания.
func TestDecorImportSVG(t *testing.T) {
	src := testkit.TempDir(t)
	s, cfg := openTemp(t)
	good := writeFile(t, src, "Лента.svg", svgClean)
	evil := writeFile(t, src, "вредный.svg", []byte(svgDoc(`<script>alert(1)</script>`)))
	huge := writeFile(t, src, "огромный.SVG", append([]byte(svgDoc(`<!--`)), bytes.Repeat([]byte("x"), MaxSVGBytes)...))
	res, err := s.Import(KindSticker, "wedding", []string{good, evil, huge})
	if err != nil || len(res.Added) != 0 || len(res.Svg) != 1 || len(res.Skipped) != 2 {
		t.Fatalf("итог %+v, %v", res, err)
	}
	job := res.Svg[0]
	if len(job.Token) != 16 || job.Name != "Лента" || job.W != 4096 || job.H != 2048 {
		t.Errorf("задание: %+v", job)
	}
	if raw, err := base64.StdEncoding.DecodeString(job.Data); err != nil || !bytes.Equal(raw, svgClean) {
		t.Errorf("Data — не исходный SVG: %v", err)
	}
	if !strings.Contains(res.Skipped[0].Why, "script") || !strings.Contains(res.Skipped[1].Why, "МБ") {
		t.Errorf("причины: %+v", res.Skipped)
	}
	key := keyOf(t, good)
	if _, ok := s.Find(key); ok || s.entryOf(KindSticker, key) != nil {
		t.Fatal("запись появилась до PNG")
	}
	it, err := s.AddPNG(job.Token, b64(renderPNG(t, job.W, job.H)))
	if err != nil || it.ID != key || !it.Own || it.Theme != "wedding" || it.Name != "Лента" || it.W != 4096 || it.H != 2048 {
		t.Fatalf("AddPNG: %+v, %v", it, err)
	}
	p := mustFind(t, s, key)
	if filepath.Ext(p) != ".png" || filepath.Base(filepath.Dir(p)) != "Стикеры" {
		t.Errorf("файл: %s", p)
	}
	// Повтор SVG — «уже есть», задания нет; скрытый — возвращается сразу.
	again, _ := s.Import(KindSticker, "mine", []string{good})
	if len(again.Svg) != 0 || len(again.Skipped) != 1 || !strings.Contains(again.Skipped[0].Why, "уже есть") {
		t.Errorf("повтор: %+v", again)
	}
	if err := s.Hide(KindSticker, key); err != nil {
		t.Fatal(err)
	}
	back, _ := s.Import(KindSticker, "mine", []string{good})
	if len(back.Svg) != 0 || len(back.Added) != 1 || back.Added[0].ID != key {
		t.Errorf("возврат скрытого: %+v", back)
	}
	// Файл стёрт руками, запись осталась: тот же SVG снова просит PNG и возвращает файл.
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	s2 := Open(cfg)
	redo, _ := s2.Import(KindSticker, "mine", []string{good})
	if len(redo.Svg) != 1 {
		t.Fatalf("после стёртого файла: %+v", redo)
	}
	if _, err := s2.AddPNG(redo.Svg[0].Token, b64(renderPNG(t, 4096, 2048))); err != nil {
		t.Fatal(err)
	}
	if p2, ok := s2.Find(key); !ok || !fileExists(p2) {
		t.Error("файл не вернулся")
	}
	if n := len(readReg(t, cfg).Items); n != 1 {
		t.Errorf("в реестре %d записей, ждали 1", n)
	}
}

// fixClock подменяет часы токенов; возвращает функцию «прошло d».
func fixClock(t *testing.T) func(d time.Duration) {
	t.Helper()
	now := time.Now()
	old := nowFn
	nowFn = func() time.Time { return now }
	t.Cleanup(func() { nowFn = old })
	return func(d time.Duration) { now = now.Add(d) }
}

// jobFor — Import чистого SVG под именем name; возвращает задание.
func jobFor(t *testing.T, s *Store, dir, name, body string) SvgJob {
	t.Helper()
	res, err := s.Import(KindSticker, "mine", []string{writeFile(t, dir, name+".svg", []byte(svgDoc(body)))})
	if err != nil || len(res.Svg) != 1 {
		t.Fatalf("Import: %+v, %v", res, err)
	}
	return res.Svg[0]
}

// TestAddDecorPNG — PNG от окна: только по живому одноразовому токену, только PNG, размер как
// заказан. Токен расходуется при любом отказе; просроченный и чужой не работают; в памяти ≤ 50.
func TestAddDecorPNG(t *testing.T) {
	adv := fixClock(t)
	src := testkit.TempDir(t)
	s, cfg := openTemp(t)
	ok := renderPNG(t, 4096, 2048)

	if _, err := s.AddPNG("0123456789abcdef", b64(ok)); !errors.Is(err, ErrToken) {
		t.Errorf("чужой токен: %v", err)
	}
	if _, err := s.AddPNG("", b64(ok)); !errors.Is(err, ErrToken) {
		t.Errorf("пустой токен: %v", err)
	}
	// Любая порча PNG — отказ, токен сгорает: второй попытки с тем же токеном нет.
	jpg := mustRead(t, testkit.JPEG(t, src, "x.jpg", 4096, 2048))
	cases := map[string]string{
		"не base64":        "!!!",
		"пустое":           "",
		"JPEG":             b64(jpg),
		"текст":            b64([]byte("не картинка")),
		"размер не тот":    b64(renderPNG(t, 4096, 2047)),
		"меньше заказа":    b64(renderPNG(t, 64, 32)),
		"повёрнутый":       b64(renderPNG(t, 2048, 4096)),
		"обрезанное тело":  b64(ok[:len(ok)/2]),
		"только сигнатура": b64([]byte("\x89PNG\r\n\x1a\n")),
		"прозрачная":       b64(clearPNG(t, image.NewPaletted(image.Rect(0, 0, 4096, 2048), color.Palette{color.NRGBA{}}))),
	}
	for name, data := range cases {
		job := jobFor(t, s, src, "a"+strings.ReplaceAll(name, " ", "-"), `<rect width="`+name+`"/>`)
		if _, err := s.AddPNG(job.Token, data); err == nil {
			t.Errorf("%s: принято", name)
		}
		if _, err := s.AddPNG(job.Token, b64(ok)); !errors.Is(err, ErrToken) {
			t.Errorf("%s: токен пережил отказ: %v", name, err)
		}
	}
	if n := len(s.entries); n != 0 {
		t.Errorf("отказы оставили %d записей", n)
	}
	if left, _ := filepath.Glob(filepath.Join(cfg, RootName, "Стикеры", "*")); len(left) != 0 {
		t.Errorf("на диске осталось %v", left)
	}
	// Слишком длинный base64 — отказ до разбора.
	big := jobFor(t, s, src, "big", `<rect width="big"/>`)
	if _, err := s.AddPNG(big.Token, strings.Repeat("A", MaxPNGBase64+1)); err == nil || errors.Is(err, ErrToken) {
		t.Errorf("48 МБ + 1: %v", err)
	}
	// Прозрачный целиком — окно не нарисовало SVG (аудит 8391ede): своя причина; один видимый пиксель — принят.
	empty, dot := image.NewNRGBA(image.Rect(0, 0, 4096, 2048)), image.NewNRGBA(image.Rect(0, 0, 4096, 2048))
	dot.Pix[len(dot.Pix)-1] = 1
	blankJob, dotJob := jobFor(t, s, src, "blank", `<rect width="blank"/>`), jobFor(t, s, src, "dot", `<rect width="dot"/>`)
	if _, err := s.AddPNG(blankJob.Token, b64(clearPNG(t, empty))); !errors.Is(err, errSVGBlank) {
		t.Errorf("прозрачный PNG: %v", err)
	}
	if _, err := s.AddPNG(dotJob.Token, b64(clearPNG(t, dot))); err != nil {
		t.Errorf("один видимый пиксель: %v", err)
	}
	// Одноразовость: после успеха тот же токен не работает.
	good := jobFor(t, s, src, "good", `<rect width="good"/>`)
	if _, err := s.AddPNG(good.Token, b64(ok)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddPNG(good.Token, b64(ok)); !errors.Is(err, ErrToken) {
		t.Errorf("повтор токена: %v", err)
	}
	// Срок — 10 минут: на границе ещё жив, позже — нет.
	late := jobFor(t, s, src, "late", `<rect width="late"/>`)
	edge := jobFor(t, s, src, "edge", `<rect width="edge"/>`)
	adv(svgTokenTTL)
	if _, err := s.AddPNG(edge.Token, b64(ok)); err != nil {
		t.Errorf("токен ровно в срок: %v", err)
	}
	adv(time.Second)
	if _, err := s.AddPNG(late.Token, b64(ok)); !errors.Is(err, ErrToken) {
		t.Errorf("просроченный токен: %v", err)
	}
	// Не больше 50 живых: пятьдесят первый вытесняет самый старый.
	var jobs []SvgJob
	for i := range maxSVGTokens + 1 {
		jobs = append(jobs, jobFor(t, s, src, "n"+string(rune('a'+i%26))+string(rune('a'+i/26)), `<rect width="`+string(rune('a'+i%26))+string(rune('a'+i/26))+`"/>`))
		adv(time.Millisecond)
	}
	s.svg.mu.Lock()
	live := len(s.svg.m)
	s.svg.mu.Unlock()
	if live != maxSVGTokens {
		t.Errorf("токенов в памяти %d, ждали %d", live, maxSVGTokens)
	}
	if _, err := s.AddPNG(jobs[0].Token, b64(ok)); !errors.Is(err, ErrToken) {
		t.Errorf("вытесненный токен: %v", err)
	}
	if _, err := s.AddPNG(jobs[maxSVGTokens].Token, b64(ok)); err != nil {
		t.Errorf("последний токен: %v", err)
	}
}

// TestAddDecorPNGReadOnly — реестр новее программы: PNG не принимается, токен остаётся жив.
func TestAddDecorPNGReadOnly(t *testing.T) {
	src := testkit.TempDir(t)
	s, cfg := openTemp(t)
	job := jobFor(t, s, src, "a", `<rect/>`)
	writeReg(t, cfg, `{"version":2,"items":[]}`)
	s2 := Open(cfg)
	s2.svg.m = s.svg.m // токен выдан до того, как реестр стал чужим
	if _, err := s2.AddPNG(job.Token, b64(renderPNG(t, 4096, 2048))); !errors.Is(err, ErrReadOnly) {
		t.Errorf("новый реестр: %v", err)
	}
	if res, err := s2.Import(KindSticker, "mine", []string{filepath.Join(src, "a.svg")}); !errors.Is(err, ErrReadOnly) || len(res.Svg) != 0 {
		t.Errorf("импорт при новом реестре: %+v %v", res, err)
	}
}

// TestStickerPresets — встроенные стикеры: ID по грамматике, без повторов и без совпадений с
// фонами, темы из списка, параметры уже зажаты, генератор есть, каждый генератор набора в деле.
func TestStickerPresets(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range bgPresets {
		seen[p.id] = true
	}
	themes := map[string]int{}
	for _, p := range stickerPresets {
		rest, ok := strings.CutPrefix(p.id, presetPrefix+"st-")
		if !ok || !gen.ValidKind(rest) || seen[p.id] {
			t.Errorf("ID %q", p.id)
		}
		seen[p.id] = true
		if !isTheme(p.theme) || p.theme == ThemeMine || p.name == "" {
			t.Errorf("%s: тема %q, имя %q", p.id, p.theme, p.name)
		}
		themes[p.theme]++
		if !gen.IsArt(p.spec.Kind) {
			t.Errorf("%s: %q — не стикер", p.id, p.spec.Kind)
		}
		if got := gen.Clamp(p.spec); got.Kind != p.spec.Kind || len(got.Inks) != len(p.spec.Inks) || got.Scale != 0 || got.Seed != 0 {
			t.Errorf("%s: параметры не зажаты: %+v → %+v", p.id, p.spec, got)
		}
	}
	if n := len(stickerPresets); n < 36 || n > 48 {
		t.Errorf("встроенных стикеров %d, ждали ~40", n)
	}
	for _, th := range []string{"basic", "wedding", "kids", "kindergarten", "school", "grad", "holiday"} {
		if themes[th] < 3 {
			t.Errorf("в теме %s всего %d стикеров", th, themes[th])
		}
	}
	// Каждый генератор набора есть хотя бы в одном пресете.
	used := map[string]bool{}
	for _, p := range stickerPresets {
		used[p.spec.Kind] = true
	}
	for _, m := range gen.Arts() {
		if !used[m.Kind] {
			t.Errorf("генератор %s не вынесен ни в один пресет", m.Kind)
		}
	}
}

// TestStickerCatalog — каталог стикеров: встроенные и свои плитки, недавние; URL встроенных
// разбирается как /media/gen/art; Sticker(id) разбирает встроенный и свой, а чужое — нет.
func TestStickerCatalog(t *testing.T) {
	src := testkit.TempDir(t)
	s, _ := openTemp(t)
	own, err := s.Import(KindSticker, "mine", []string{writePNG(t, src, "a.png", 80, 40)})
	if err != nil || len(own.Added) != 1 {
		t.Fatal(own, err)
	}
	cat, err := s.Catalog(KindSticker, []string{own.Added[0].ID, "sa:st-heart"}, 600, 300)
	if err != nil || len(cat.Items) != len(stickerPresets)+1 || !equalStrings(cat.Recent, []string{own.Added[0].ID, "sa:st-heart"}) {
		t.Fatalf("каталог: %d плиток, недавние %v, %v", len(cat.Items), cat.Recent, err)
	}
	for _, it := range cat.Items {
		if it.Own {
			if it.Aspect != 2 || !strings.HasPrefix(it.URL, LibPath+"t/") {
				t.Errorf("свой: %+v", it)
			}
			continue
		}
		path, raw, _ := strings.Cut(it.URL, "?")
		req, ok := gen.Parse(path, raw+"&px=128")
		if !ok || !req.Art || it.Aspect <= 0 || it.Color != "" {
			t.Errorf("встроенный %+v: %v", it, ok)
		}
	}
	// Sticker: встроенный — параметры генератора и пропорция вставки; свой — фото с ключом.
	st, err := s.Sticker("sa:st-plate-dark")
	if err != nil || st.Art == nil || st.Art.Kind != "plate" || len(st.Art.Inks) != 2 || st.Aspect != 3 || st.Photo.ID != "" {
		t.Errorf("встроенный: %+v, %v", st, err)
	}
	st, err = s.Sticker(own.Added[0].ID)
	if err != nil || st.Art != nil || st.Photo.Lib != own.Added[0].ID || st.Photo.Path == "" || st.Photo.W != 80 || st.Aspect != 2 {
		t.Errorf("свой: %+v, %v", st, err)
	}
	for _, id := range []string{"", "sa:st-нет", "sa:gold-dots", "my:ffffffffffffffff", "../x", "my:../../x"} {
		if _, err := s.Sticker(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Sticker(%q) = %v", id, err)
		}
	}
	// Фон под тем же ключом стикером не становится.
	bg, _ := s.Import(KindBg, "mine", []string{writePNG(t, src, "b.png", 90, 40)})
	if _, err := s.Sticker(bg.Added[0].ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("фон как стикер: %v", err)
	}
}

// TestMediaGenArt — /media/gen/art отдаёт каждую плитку каталога стикеров PNG нужного размера,
// растяжимые — в заказанной рамке (в том числе крайней), а мусор и узор как стикер — 404.
func TestMediaGenArt(t *testing.T) {
	s, _ := openTemp(t)
	h := s.Handler(nil)
	cat, err := s.Catalog(KindSticker, nil, 600, 300)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range cat.Items {
		path, query, _ := strings.Cut(it.URL, "?")
		rec := get(h, http.MethodGet, path, query+"&px=96")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", it.URL, rec.Code)
		}
		cfg, err := png.DecodeConfig(rec.Body)
		if err != nil || max(cfg.Width, cfg.Height) != 96 || cfg.Width < 1 || cfg.Height < 1 {
			t.Errorf("%s: %+v, %v", it.URL, cfg, err)
		}
	}
	for q, want := range map[string][2]int{
		"k=wave-band&a=500&v=1&px=4096":   {4096, 8},
		"k=plate&a=0.002&v=1&px=4096":     {8, 4096},
		"k=plate&a=2&v=1&px=200":          {200, 100},
		"k=wave-band&a=1000000&v=1&px=64": {64, 0},    // выше предела — зажим до 500
		"k=star&a=3&v=1&px=100":           {100, 100}, // у нерастяжимого пропорция своя
	} {
		rec := get(h, http.MethodGet, "/media/gen/art", q)
		cfg, err := png.DecodeConfig(rec.Body)
		if rec.Code != http.StatusOK || err != nil || cfg.Width != want[0] || (want[1] != 0 && cfg.Height != want[1]) {
			t.Errorf("%s: %d %+v %v, ждали %v", q, rec.Code, cfg, err, want)
		}
	}
	for _, q := range []string{"k=dots&v=1&px=64", "k=nope&v=1&px=64", "k=star&v=1&px=64&w=10", "k=star&a=NaN&v=1&px=64", "k=star&v=1"} {
		if rec := get(h, http.MethodGet, "/media/gen/art", q); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", q, rec.Code)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
