package decor

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// ringImage — рамочка w×h: непрозрачная кайма l, t, r, b px, внутри прозрачное окно.
func ringImage(w, h, l, t, r, b int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			if x < l || y < t || x >= w-r || y >= h-b {
				img.SetNRGBA(x, y, color.NRGBA{150, 90, 40, 255})
			}
		}
	}
	return img
}

// cornersImage — уголки: непрозрачны только квадраты n px по углам, окно достаёт до краёв.
func cornersImage(w, h, n int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			if (x < n || x >= w-n) && (y < n || y >= h-n) {
				img.SetNRGBA(x, y, color.NRGBA{150, 90, 40, 255})
			}
		}
	}
	return img
}

func pngBytes(t testing.TB, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// frameDir — папка рамочек библиотеки.
func frameDir(cfg string) string { return filepath.Join(cfg, RootName, kindDirs[KindFrame]) }

// frameSet — библиотека с тремя рамочками (кольцо, уголки, лента > 4096 px) и семью отказами.
type frameSet struct {
	s     *Store
	cfg   string
	src   string
	ring  string
	res   ImportResult
	items map[string]Item
}

func newFrameSet(t *testing.T) frameSet {
	t.Helper()
	fs := frameSet{src: testkit.TempDir(t), items: map[string]Item{}}
	fs.s, fs.cfg = openTemp(t)
	fs.ring = writeFile(t, fs.src, "Кольцо.png", pngBytes(t, ringImage(120, 160, 24, 32, 24, 32)))
	corners := writeFile(t, fs.src, "Уголки.png", pngBytes(t, cornersImage(100, 100, 20)))
	solid := writeFile(t, fs.src, "Сплошная.png", pngBytes(t, ringImage(100, 100, 50, 50, 50, 50)))
	tiny := writeFile(t, fs.src, "Щель.png", pngBytes(t, ringImage(100, 100, 45, 45, 45, 45))) // окно 1 % площади
	jpg := testkit.JPEG(t, fs.src, "Фото.jpg", 50, 30)
	jpgAsPng := writeFile(t, fs.src, "Ложь.png", mustRead(t, jpg))
	wide := writeFile(t, fs.src, "Лента.png", pngBytes(t, ringImage(4200, 300, 20, 20, 20, 20)))
	files := []string{fs.ring, corners, solid, tiny, jpg, jpgAsPng, writeFile(t, fs.src, "битый.png", []byte("не картинка")),
		filepath.Join(fs.src, "нет.png"), wide, writeFile(t, fs.src, "a.gif", []byte("GIF89a"))}
	res, err := fs.s.Import(KindFrame, "wedding", files)
	if err != nil || len(res.Added) != 3 || len(res.Skipped) != 7 || len(res.Svg) != 0 {
		t.Fatalf("итог %+v, %v", res, err)
	}
	fs.res = res
	for _, it := range res.Added {
		fs.items[it.Name] = it
	}
	return fs
}

// TestDecorImportFrame — рамочка-картинка (сц. 7): PNG с окном и открытые уголки принимаются;
// JPEG, JPEG под именем .png, сплошной PNG, щель и битый файл — отказ с причиной без пути,
// копии не остаётся; длинная сторона > 4096 уменьшается; Find находит рамочку.
func TestDecorImportFrame(t *testing.T) {
	fs := newFrameSet(t)
	why := map[string]string{}
	for _, k := range fs.res.Skipped {
		why[k.Name] = k.Why
	}
	for name, part := range map[string]string{"Сплошная": "нет прозрачного окна", "Щель": "нет прозрачного окна", "Фото": "PNG с прозрачностью",
		"Ложь": "PNG с прозрачностью", "a": "PNG и SVG"} {
		if !strings.Contains(why[name], part) {
			t.Errorf("%s: причина %q", name, why[name])
		}
	}
	if strings.Contains(why["Сплошная"], fs.src) {
		t.Errorf("причина с путём: %q", why["Сплошная"])
	}
	assertNoTmp(t, frameDir(fs.cfg))
	if list, _ := os.ReadDir(frameDir(fs.cfg)); len(list) != 3 { // отказанные копий не оставили
		t.Errorf("в папке рамочек %d файлов, ждали 3", len(list))
	}
	for _, it := range fs.res.Added {
		if !it.Own || it.Theme != "wedding" || !strings.HasPrefix(it.ID, "my:") || !strings.HasPrefix(it.URL, LibPath+"t/") {
			t.Errorf("%s: плитка %+v", it.Name, it)
		}
		if p, ok := fs.s.Find(it.ID); !ok || filepath.Dir(p) != frameDir(fs.cfg) || filepath.Ext(p) != ".png" {
			t.Errorf("%s: Find %q %v", it.Name, p, ok)
		}
	}
	if w := fs.items["Лента"]; w.W != MaxFrameSide || w.H < 290 || w.H > 295 {
		t.Errorf("лента: %dx%d (ждали 4096×293)", w.W, w.H)
	}
}

// TestDecorFrameWindow — окно при импорте: кольцо 0,2 со всех сторон, уголки открыты (трети);
// после перезапуска окно в памяти пусто — Frame считает лениво и тот же результат.
func TestDecorFrameWindow(t *testing.T) {
	fs := newFrameSet(t)
	ring := fs.items["Кольцо"]
	got, err := fs.s.Frame(ring.ID)
	if err != nil || got.Open || got.Photo.Lib != ring.ID || got.Photo.W != 120 || got.Photo.H != 160 || got.Photo.Path == "" {
		t.Fatalf("Frame кольца: %+v %v", got, err)
	}
	for i, want := range [4]float64{0.2, 0.2, 0.2, 0.2} {
		if d := got.Cut[i] - want; d > 0.002 || d < -0.002 {
			t.Errorf("срез кольца %v", got.Cut)
		}
	}
	if fc, err := fs.s.Frame(fs.items["Уголки"].ID); err != nil || !fc.Open || fc.Cut != gen.ClampCut([4]float64{1.0 / 3, 1.0 / 3, 1.0 / 3, 1.0 / 3}) {
		t.Errorf("Frame уголков: %+v %v", fc, err)
	}
	if again, err := Open(fs.cfg).Frame(ring.ID); err != nil || again.Cut != got.Cut || again.Open != got.Open {
		t.Errorf("после перезапуска: %+v %v", again, err)
	}
	bg := importOK(t, fs.s, "basic", testkit.JPEG(t, fs.src, "bg.jpg", 40, 30)) // не рамочка, нет такого, мусор
	for _, id := range []string{bg[0].ID, "my:0123456789abcdef", "sa:fr-nope", "sa:nope", "../x", ""} {
		if _, err := fs.s.Frame(id); err != ErrNotFound {
			t.Errorf("Frame(%q): %v", id, err)
		}
	}
}

// TestDecorImportFrameAgain — повтор файла — «уже есть», скрытая возвращается со своей темой,
// реестр только для чтения закрывает импорт.
func TestDecorImportFrameAgain(t *testing.T) {
	fs := newFrameSet(t)
	id := fs.items["Кольцо"].ID
	again, err := fs.s.Import(KindFrame, "kids", []string{fs.ring})
	if err != nil || len(again.Added) != 0 || len(again.Skipped) != 1 || !strings.Contains(again.Skipped[0].Why, "уже есть") {
		t.Errorf("повтор: %+v %v", again, err)
	}
	if err := fs.s.Hide(KindFrame, id); err != nil {
		t.Fatal(err)
	}
	back, err := fs.s.Import(KindFrame, "kids", []string{fs.ring})
	if err != nil || len(back.Added) != 1 || back.Added[0].ID != id || back.Added[0].Theme != "wedding" {
		t.Errorf("возврат скрытой: %+v %v", back, err)
	}
	fs.s.mu.Lock()
	fs.s.readOnly = true
	fs.s.mu.Unlock()
	if _, err := fs.s.Import(KindFrame, "mine", []string{fs.ring}); err != ErrReadOnly {
		t.Errorf("только чтение: %v", err)
	}
}

// TestDecorFindFrame — одна картинка в «Рамочках» и в «Фонах» (079-40): ключ общий, Find отдаёт фон
// (исходник 4200 px), FindFrame — рамочку (копия 4096 px); и после перезапуска. Ключ только фона,
// чужой и не по грамматике — FindFrame не находит.
func TestDecorFindFrame(t *testing.T) {
	fs := newFrameSet(t)
	wide, ring := fs.items["Лента"], fs.items["Кольцо"]
	res, err := fs.s.Import(KindBg, "mine", []string{filepath.Join(fs.src, "Лента.png"), testkit.JPEG(t, fs.src, "Фон.jpg", 40, 30)})
	if err != nil || len(res.Added) != 2 || res.Added[0].ID != wide.ID {
		t.Fatalf("фоны: %+v %v", res, err)
	}
	bgOnly := res.Added[1].ID
	bgDir := filepath.Join(fs.cfg, RootName, kindDirs[KindBg])
	for _, s := range []*Store{fs.s, Open(fs.cfg)} {
		if p, ok := s.Find(wide.ID); !ok || filepath.Dir(p) != bgDir {
			t.Errorf("Find ленты: %q %v — ждали фон", p, ok)
		}
		for _, id := range []string{wide.ID, ring.ID} {
			if p, ok := s.FindFrame(id); !ok || filepath.Dir(p) != frameDir(fs.cfg) {
				t.Errorf("FindFrame %s: %q %v", id, p, ok)
			}
		}
		for _, id := range []string{bgOnly, "my:0123456789abcdef", "sa:fr-line", "", "my:../../x"} {
			if p, ok := s.FindFrame(id); ok || p != "" {
				t.Errorf("FindFrame(%q) = %q — ждали отказ", id, p)
			}
		}
	}
}

// TestDecorFrameScan — в папке рамочек учитываются только <16 hex>.png: JPEG, чужие имена и
// остатки импорта не находятся, остатки убираются.
func TestDecorFrameScan(t *testing.T) {
	fs := newFrameSet(t)
	dir := frameDir(fs.cfg)
	stray := writeFile(t, dir, "0123456789abcdef.jpg", mustRead(t, testkit.JPEG(t, fs.src, "s.jpg", 20, 20)))
	junk := writeFile(t, dir, tmpPrefix+"123.tmp", []byte("x"))
	reopened := Open(fs.cfg)
	if _, ok := reopened.Find("my:0123456789abcdef"); ok {
		t.Error("JPEG в папке рамочек найден")
	}
	if fileExists(junk) || !fileExists(stray) {
		t.Errorf("остаток импорта: tmp есть=%v, чужой файл есть=%v", fileExists(junk), fileExists(stray))
	}
	if _, ok := reopened.Find(fs.items["Кольцо"].ID); !ok {
		t.Error("рамочка после перезапуска не найдена")
	}
}

// TestDecorImportFrameSVG — SVG-рамочка: задание окну с токеном вида frame; PNG с окном принят и
// получает окно, сплошной PNG от окна — отказ (токен израсходован, записи нет).
func TestDecorImportFrameSVG(t *testing.T) {
	src := testkit.TempDir(t)
	s, cfg := openTemp(t)
	svg := writeFile(t, src, "Рамка.svg", svgClean)
	res, err := s.Import(KindFrame, "wedding", []string{svg})
	if err != nil || len(res.Svg) != 1 || len(res.Added) != 0 || len(res.Skipped) != 0 {
		t.Fatalf("итог %+v, %v", res, err)
	}
	job := res.Svg[0]
	if _, err := s.AddPNG(job.Token, b64(pngBytes(t, ringImage(job.W, job.H, job.W/10, job.H/8, job.W/10, job.H/8)))); err != nil {
		t.Fatalf("AddPNG: %v", err)
	}
	cat, _ := s.Catalog(KindFrame, nil, 0, 0)
	var own []Item
	for _, it := range cat.Items {
		if it.Own {
			own = append(own, it)
		}
	}
	if len(own) != 1 || own[0].Name != "Рамка" || !fileExists(filepath.Join(frameDir(cfg), strings.TrimPrefix(own[0].ID, "my:")+".png")) {
		t.Fatalf("плитки своих: %+v", own)
	}
	if fs, err := s.Frame(own[0].ID); err != nil || fs.Open || fs.Cut[0] < 0.09 || fs.Cut[1] < 0.12 {
		t.Errorf("окно SVG-рамочки: %+v %v", fs, err)
	}
	// Тот же SVG, но окно нарисовало сплошное (в нём одно окно, а ключ тот же): отказ без записи.
	if err := s.Hide(KindFrame, own[0].ID); err != nil {
		t.Fatal(err)
	}
	res, err = s.Import(KindFrame, "wedding", []string{svg}) // скрытая возвращается без нового токена
	if err != nil || len(res.Added) != 1 || len(res.Svg) != 0 {
		t.Fatalf("возврат: %+v %v", res, err)
	}
	other := writeFile(t, src, "Другая.svg", []byte(svgDoc(`<rect width="20" height="10" fill="#0c0"/>`)))
	res, err = s.Import(KindFrame, "wedding", []string{other})
	if err != nil || len(res.Svg) != 1 {
		t.Fatalf("второй SVG: %+v %v", res, err)
	}
	job = res.Svg[0]
	if _, err := s.AddPNG(job.Token, b64(renderPNG(t, job.W, job.H))); err == nil || !strings.Contains(err.Error(), "нет прозрачного окна") {
		t.Errorf("сплошной PNG от окна: %v", err)
	}
	if _, err := s.AddPNG(job.Token, b64(pngBytes(t, ringImage(job.W, job.H, job.W/10, job.H/8, job.W/10, job.H/8)))); err != ErrToken {
		t.Errorf("токен после отказа: %v", err)
	}
	assertNoTmp(t, frameDir(cfg))
	if n, _ := s.usage(KindFrame); n != 1 {
		t.Errorf("своих рамочек %d, ждали 1", n)
	}
}
