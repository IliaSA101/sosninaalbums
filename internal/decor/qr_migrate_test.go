package decor

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// qrPNGOf — картинка кода ссылки, как её кладёт AddQR.
func qrPNGOf(t *testing.T, link string) []byte {
	t.Helper()
	q, err := EncodeQR(link)
	if err != nil {
		t.Fatal(err)
	}
	data, err := q.PNG()
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// regMap / putRegMap — library.json как есть (map): правка «под 0.27» без зажима реестром.
func regMap(t *testing.T, cfg string) map[string]any {
	t.Helper()
	return rawRegOf(t, filepath.Join(cfg, RootName, regFile))
}

func putRegMap(t *testing.T, cfg string, m map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf) // без экранирования < > &: как пишет сама программа
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, RootName, regFile), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// as027 — реестр, каким его оставила бы 0.27: у записей key нет признака qr, нет qrScan, имена
// переименованы (rename: ключ → имя); extra добавляется в корень.
func as027(t *testing.T, cfg string, rename map[string]string, extra map[string]any) {
	t.Helper()
	m := regMap(t, cfg)
	delete(m, "qrScan")
	for _, it := range m["items"].([]any) {
		e := it.(map[string]any)
		delete(e, "qr")
		if n, ok := rename[e["key"].(string)]; ok {
			e["name"] = n
		}
	}
	for k, v := range extra {
		m[k] = v
	}
	putRegMap(t, cfg, m)
}

// stripes — двухцветный квадратный PNG той же стороны, что у кода, но без «глазков» (обычный стикер).
func stripes(t *testing.T, w int) []byte {
	t.Helper()
	img := image.NewPaletted(image.Rect(0, 0, w, w), color.Palette{color.White, color.Black})
	for y := range w {
		for x := range w {
			if (x/(w/29)+y/(w/29))%2 == 0 {
				img.Pix[y*img.Stride+x] = 1
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// addSticker — свой стикер из байтов PNG (импорт файлом).
func addSticker(t *testing.T, s *Store, name string, data []byte) string {
	t.Helper()
	p := writeFile(t, testkit.TempDir(t), name+".png", data)
	res, err := s.Import(KindSticker, ThemeMine, []string{p})
	if err != nil || len(res.Added) != 1 {
		t.Fatalf("Import %s: %+v %v", name, res, err)
	}
	return res.Added[0].ID
}

// Переименованные коды 0.23–0.27 без признака узнаются по содержимому при Open; обычные
// двухцветные квадратные стикеры, однотонные и повреждённые — нет; остальные поля реестра
// (чужие в записи и в корне) живы; повторный Open файл не переписывает; IsQR держит отказ в макет.
func TestMigrateQR(t *testing.T) {
	s, cfg := openTemp(t)
	a, _ := s.AddQR("t.me/test")
	b, _ := s.AddQR("https://example.com/gallery/very/long/path?client=1&album=2")
	c, _ := s.AddQR(strings.Repeat("x", 300) + ".ru") // версия выше
	plain := addSticker(t, s, "plain", renderPNG(t, 40, 40))
	flat := addSticker(t, s, "stripes", stripes(t, a.W))
	as027(t, cfg, map[string]string{a.ID: "Галерея Ивановых", b.ID: "Ссылка", c.ID: "Длинная"}, map[string]any{"future": []any{1.0, "x"}})
	m := regMap(t, cfg)
	m["items"].([]any)[0].(map[string]any)["newer"] = "поле новее" // чужое поле записи
	putRegMap(t, cfg, m)

	s = Open(cfg)
	for _, k := range []string{a.ID, b.ID, c.ID} {
		if !s.IsQR(k) {
			t.Errorf("код %s не узнан", k)
		}
	}
	for _, k := range []string{plain, flat} {
		if s.IsQR(k) {
			t.Errorf("обычный стикер %s помечен", k)
		}
	}
	got := regMap(t, cfg)
	if got["qrScan"] != float64(qrScanV) || got["future"] == nil {
		t.Fatalf("корень реестра: %v", got)
	}
	for _, it := range got["items"].([]any) {
		e := it.(map[string]any)
		isCode := e["key"] == a.ID || e["key"] == b.ID || e["key"] == c.ID
		if (e["qr"] == true) != isCode {
			t.Errorf("qr у %v: %v", e["key"], e["qr"])
		}
		if e["key"] == a.ID && (e["name"] != "Галерея Ивановых" || e["newer"] != "поле новее") {
			t.Errorf("запись переписана: %v", e)
		}
	}

	// Круг Open → Open: признак на диске, файл не трогается (содержимое и время).
	path := filepath.Join(cfg, RootName, regFile)
	before, _ := os.ReadFile(path)
	st0, _ := os.Stat(path)
	s2 := Open(cfg)
	after, _ := os.ReadFile(path)
	st1, _ := os.Stat(path)
	if !bytes.Equal(before, after) || !st0.ModTime().Equal(st1.ModTime()) {
		t.Error("повторный Open переписал реестр")
	}
	if !s2.IsQR(a.ID) || s2.IsQR(plain) {
		t.Error("признак не сохранился")
	}
	// Переименование записи с признаком его не сбрасывает; следующая запись реестра хранит qrScan.
	if err := s2.Rename(KindSticker, a.ID, "Ещё раз"); err != nil || !s2.IsQR(a.ID) || regMap(t, cfg)["qrScan"] != float64(qrScanV) {
		t.Errorf("Rename: %v, qrScan %v", err, regMap(t, cfg)["qrScan"])
	}
}

// Миграция не идёт по реестру новее программы (version 2 и qrScan новее) и не пишет в него;
// без файла реестра файл не создаётся.
func TestMigrateQRNewerAndEmpty(t *testing.T) {
	for name, edit := range map[string]map[string]any{"version": {"version": 2.0}, "qrScan": {"qrScan": 7.0}} {
		t.Run(name, func(t *testing.T) {
			s, cfg := openTemp(t)
			a, _ := s.AddQR("t.me/test")
			as027(t, cfg, map[string]string{a.ID: "Моя ссылка"}, edit)
			path := filepath.Join(cfg, RootName, regFile)
			before, _ := os.ReadFile(path)
			if Open(cfg).IsQR(a.ID) {
				t.Error("код узнан в реестре новее программы")
			}
			if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
				t.Error("реестр новее программы переписан")
			}
		})
	}
	cfg := testkit.TempDir(t)
	Open(cfg)
	if fileExists(filepath.Join(cfg, RootName, regFile)) {
		t.Error("Open создал реестр")
	}
}

// Битые, усечённые, пустые и огромные файлы стикеров — без паники, не помечаются, миграция
// всё равно завершается (qrScan записан).
func TestMigrateQRBroken(t *testing.T) {
	s, cfg := openTemp(t)
	a, _ := s.AddQR("t.me/test")
	good := qrPNGOf(t, "t.me/test")
	k1 := addSticker(t, s, "plain", renderPNG(t, 30, 30))
	as027(t, cfg, map[string]string{a.ID: "Ссылка"}, nil)
	s = Open(cfg)
	// Подмена содержимого файла кода и обычного стикера на мусор (имя от ключа не меняется).
	cases := map[string][]byte{
		"пусто":    {},
		"мусор":    []byte("не картинка"),
		"усечён":   good[:len(good)/2],
		"шапка":    good[:33],
		"огромный": append(append([]byte{}, good...), bytes.Repeat([]byte{0}, qrScanFile)...),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			p, _ := s.Find(a.ID)
			if err := os.WriteFile(p, data, 0o644); err != nil {
				t.Fatal(err)
			}
			as027(t, cfg, nil, nil)
			s2 := Open(cfg)
			if s2.IsQR(a.ID) || s2.IsQR(k1) {
				t.Error("испорченный файл помечен")
			}
			if regMap(t, cfg)["qrScan"] != float64(qrScanV) {
				t.Error("миграция не отмечена")
			}
		})
	}
	if isQRFile(filepath.Join(cfg, "нет-такого.png")) || isQRFile(cfg) {
		t.Error("нет файла или каталог — не код")
	}
}

// Рисунок не по правилам QR.PNG не узнаётся: сдвинутая клетка «глазка», размытый (неоднородный
// модуль), чужой размер, не палитра, не квадрат, поле не белое; настоящие коды всех размеров — да.
func TestIsQRPNG(t *testing.T) {
	good := qrPNGOf(t, "t.me/test")
	if !isQRPNG(good) {
		t.Fatal("код программы не узнан")
	}
	for _, n := range []int{21, 29, 37, 45, 53, 61} { // версии 1…
		if !isQRPNG(qrPNGOf(t, linkOfModules(t, n))) {
			t.Errorf("код %d модулей не узнан", n)
		}
	}
	img, _ := png.Decode(bytes.NewReader(good))
	p := img.(*image.Paletted)
	enc := func(m image.Image) []byte {
		var buf bytes.Buffer
		if err := png.Encode(&buf, m); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	clone := func() *image.Paletted {
		c := image.NewPaletted(p.Rect, p.Palette)
		copy(c.Pix, p.Pix)
		return c
	}
	w := p.Rect.Dx()
	mod := w / 33 // «t.me/test» — версия 2, 25 модулей и поле: 33
	flipBlock := func(mx, my int) []byte {
		c := clone()
		for y := my * mod; y < (my+1)*mod; y++ {
			for x := mx * mod; x < (mx+1)*mod; x++ {
				c.Pix[y*c.Stride+x] ^= 1
			}
		}
		return enc(c)
	}
	flip := func(x, y int) []byte { c := clone(); c.Pix[y*c.Stride+x] ^= 1; return enc(c) }
	bad := map[string][]byte{
		"точка в блоке":      flip(w/2+1, w/2+1),
		"модуль в поле":      flipBlock(1, 1),
		"модуль в глазке":    flipBlock(5, 5),
		"модуль синхролинии": flipBlock(4+9, 4+6),
		"точка в поле":       flip(1, 1),
		"точка в глазке":     flip(w/29*5+1, w/29*5+1),
		"не палитра":         enc(func() image.Image { g := image.NewGray(p.Rect); return g }()),
		"не квадрат":         enc(image.NewPaletted(image.Rect(0, 0, w, w-1), p.Palette)),
		"пустой":             enc(image.NewPaletted(p.Rect, p.Palette)),
		"цвета не ч/б": enc(func() image.Image {
			c := clone()
			c.Palette = color.Palette{color.Black, color.RGBA{R: 200, A: 255}}
			return c
		}()),
		"сторона не кратна": enc(image.NewPaletted(image.Rect(0, 0, w+1, w+1), p.Palette)),
		"плоский":           stripes(t, w),
	}
	for name, data := range bad {
		if isQRPNG(data) {
			t.Errorf("%s: узнан как код", name)
		}
	}
}

// Сбой записи реестра: признак остаётся в памяти до конца сеанса (код клиента закрыт), версия
// миграции не записана — следующий Open повторит.
func TestMigrateQRWriteFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("права каталога на Windows не запрещают запись")
	}
	s, cfg := openTemp(t)
	a, _ := s.AddQR("t.me/test")
	as027(t, cfg, map[string]string{a.ID: "Ссылка"}, nil)
	root := filepath.Join(cfg, RootName)
	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(root, 0o755) }()
	if f, err := os.CreateTemp(root, "w"); err == nil { // сидим под root — запрет не действует
		_ = f.Close()
		_ = os.Remove(f.Name())
		t.Skip("каталог доступен на запись несмотря на права")
	}
	s2 := Open(cfg)
	if !s2.IsQR(a.ID) {
		t.Error("код не закрыт в памяти при сбое записи")
	}
	if regMap(t, cfg)["qrScan"] != nil {
		t.Error("версия миграции записана при сбое")
	}
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if !Open(cfg).IsQR(a.ID) || regMap(t, cfg)["qrScan"] != float64(qrScanV) {
		t.Error("повторный Open не доделал миграцию")
	}
}

// FuzzIsQRPNG — разбор произвольных байтов не паникует и не ест память; настоящий код узнаётся.
func FuzzIsQRPNG(f *testing.F) {
	q, _ := EncodeQR("t.me/test")
	good, _ := q.PNG()
	f.Add(good)
	f.Add(good[:40])
	f.Add([]byte{})
	f.Add([]byte("\x89PNG\r\n\x1a\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > qrScanFile {
			return
		}
		_ = isQRPNG(data)
	})
}

// Не переименованная запись 0.23–0.27 («QR · …» в имени, признака нет): миграция ставит ей признак без
// разбора файла, и первое же ✎ его не теряет — ни в сеансе, ни после нового Open. То же без миграции
// (qrScan уже стоит): Rename сам фиксирует признак до смены имени.
func TestMigrateQRNamedThenRename(t *testing.T) {
	for _, withScan := range []bool{false, true} {
		s, cfg := openTemp(t)
		a, _ := s.AddQR("t.me/test")
		extra := map[string]any{}
		if withScan {
			extra["qrScan"] = float64(qrScanV)
		}
		as027(t, cfg, nil, extra)
		s = Open(cfg)
		if !withScan && regMap(t, cfg)["items"].([]any)[0].(map[string]any)["qr"] != true {
			t.Fatal("запись с именем «QR · …» не получила признак")
		}
		if err := s.Rename(KindSticker, a.ID, "Галерея Ивановых"); err != nil {
			t.Fatal(err)
		}
		if !s.IsQR(a.ID) || !Open(cfg).IsQR(a.ID) {
			t.Errorf("после ✎ код клиента не закрыт (qrScan уже стоял: %v)", withScan)
		}
	}
}

// Чужие значения с & < > в записях при миграции остаются теми же символами, а не &.
func TestMigrateQRKeepsEscapes(t *testing.T) {
	s, cfg := openTemp(t)
	a, _ := s.AddQR("t.me/test")
	plain := addSticker(t, s, "plain", renderPNG(t, 40, 40))
	as027(t, cfg, map[string]string{a.ID: "A & B", plain: "<b> & x"}, nil)
	Open(cfg)
	data, _ := os.ReadFile(filepath.Join(cfg, RootName, regFile))
	if !bytes.Contains(data, []byte("<b> & x")) || bytes.Contains(data, []byte(`\u00`)) {
		t.Errorf("значения экранированы: %s", data)
	}
}

// countRS считает байты, отданные чтением.
type countRS struct {
	*bytes.Reader
	n int
}

func (c *countRS) Read(p []byte) (int, error) {
	n, err := c.Reader.Read(p)
	c.n += n
	return n, err
}

// Крупный не-QR PNG (не палитра, не квадрат, не ч/б) отсеивается по шапке: читаются первые байты, а
// не весь файл.
func TestIsQRStreamHeaderOnly(t *testing.T) {
	noise := func(w, h int, pal bool) []byte {
		var img image.Image
		if pal {
			p := image.NewPaletted(image.Rect(0, 0, w, h), color.Palette{color.White, color.Black})
			for i := range p.Pix {
				p.Pix[i] = uint8((i * 7919 >> 3) & 1)
			}
			img = p
		} else {
			g := image.NewGray(image.Rect(0, 0, w, h))
			x := uint32(12345)
			for i := range g.Pix {
				x = x*1664525 + 1013904223
				g.Pix[i] = uint8(x >> 24)
			}
			img = g
		}
		var buf bytes.Buffer
		if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	for name, data := range map[string][]byte{
		"серый шум":           noise(1000, 1000, false),
		"палитра, не квадрат": noise(3000, 2900, true),
	} {
		if len(data) < 5000 {
			t.Fatalf("%s: файл мал для замера (%d)", name, len(data))
		}
		c := &countRS{Reader: bytes.NewReader(data)}
		if isQRStream(c) {
			t.Errorf("%s: узнан как код", name)
		}
		if c.n > 1024 {
			t.Errorf("%s: прочитано %d из %d байт", name, c.n, len(data))
		}
	}
	// Настоящий код через тот же путь узнаётся (полное чтение после шапки).
	good := qrPNGOf(t, "t.me/test")
	if !isQRStream(&countRS{Reader: bytes.NewReader(good)}) {
		t.Error("код программы не узнан по потоку")
	}
}

// Код старшей версии (много знаков не ASCII): узнаётся, миграция его помечает.
func TestMigrateQRHighVersion(t *testing.T) {
	for name, link := range map[string]string{
		"кириллица": "https://x.ru/" + strings.Repeat("ж", 480),
		"эмодзи":    "https://x.ru/" + strings.Repeat("😀", 400),
	} {
		t.Run(name, func(t *testing.T) {
			q, err := EncodeQR(link)
			if err != nil || len(q.Modules) < 100 {
				t.Fatalf("код: %d модулей, %v", len(q.Modules), err)
			}
			if !isQRPNG(qrPNGOf(t, link)) {
				t.Errorf("код %d модулей не узнан", len(q.Modules))
			}
			s, cfg := openTemp(t)
			a, err := s.AddQR(link)
			if err != nil {
				t.Fatal(err)
			}
			as027(t, cfg, map[string]string{a.ID: "Длинная"}, nil)
			if !Open(cfg).IsQR(a.ID) {
				t.Error("не помечен")
			}
		})
	}
}
