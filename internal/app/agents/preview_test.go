package agents

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Превью (Г3): уровень 3, JPEG без метаданных, предел размера, срок вызова, места вызовов.

// gradient — гладкая картинка w×h: сжимается хорошо, цвет зависит от координат.
func gradient(w, h int) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			im.Set(x, y, color.RGBA{uint8(x * 255 / max(w, 1)), uint8(y * 255 / max(h, 1)), 128, 255})
		}
	}
	return im
}

// noise — шум: JPEG плохо сжимает, нужен для проверки предела размера.
func noise(w, h int) *image.RGBA {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	rnd := rand.New(rand.NewSource(1))
	for i := 0; i < len(im.Pix); i += 4 {
		im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = uint8(rnd.Intn(256)), uint8(rnd.Intn(256)), uint8(rnd.Intn(256)), 255
	}
	return im
}

// testJPEG — настоящий JPEG превью w×h.
func testJPEG(w, h int) ([]byte, error) { return EncodePreview(gradient(w, h), max(w, h)) }

// segments — маркеры JPEG до SOS по независимому разбору (не кодом службы).
func segments(t *testing.T, b []byte) []byte {
	t.Helper()
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		t.Fatal("нет SOI")
	}
	out := []byte{0xD8}
	for i := 2; i+4 <= len(b); {
		if b[i] != 0xFF {
			t.Fatalf("нет маркера на %d", i)
		}
		m := b[i+1]
		out = append(out, m)
		if m == 0xDA {
			break
		}
		i += 2 + int(b[i+2])<<8 + int(b[i+3])
	}
	return out
}

// cleanOutput — независимый проход по всему выходу: SOI в начале, EOI ровно в конце, до SOS только
// APP0, DQT, DHT, SOF0, SOS; в данных скана — ни одного маркера, кроме RST и EOI; нет метки секрета.
func cleanOutput(t *testing.T, b []byte) {
	t.Helper()
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 || b[len(b)-2] != 0xFF || b[len(b)-1] != 0xD9 {
		t.Fatal("выход не SOI…EOI")
	}
	if bytes.Contains(b, []byte("ZQ-secret")) {
		t.Fatal("метка секрета дошла до выхода")
	}
	i := 2
	for ; i+4 <= len(b); i += 2 + int(b[i+2])<<8 + int(b[i+3]) {
		if b[i] != 0xFF {
			t.Fatalf("нет маркера на %d", i)
		}
		switch b[i+1] {
		case 0xE0, 0xDB, 0xC4, 0xC0:
			continue
		case 0xDA:
			i += 2 + int(b[i+2])<<8 + int(b[i+3])
		default:
			t.Fatalf("маркер FF%02X до скана", b[i+1])
		}
		break
	}
	for ; i+1 < len(b); i++ {
		if b[i] != 0xFF {
			continue
		}
		switch m := b[i+1]; {
		case m == 0x00, m >= 0xD0 && m <= 0xD7:
		case m == 0xD9 && i+2 == len(b):
		default:
			t.Fatalf("маркер FF%02X в данных скана или после EOI (позиция %d из %d)", m, i, len(b))
		}
	}
}

// withSegment вставляет сегмент маркера m с данными data сразу после SOI.
func withSegment(b []byte, m byte, data []byte) []byte {
	n := len(data) + 2
	seg := append([]byte{0xFF, m, byte(n >> 8), byte(n)}, data...)
	return append(append(append([]byte{}, b[:2]...), seg...), b[2:]...)
}

// pcall — вызов инструмента с полным ответом (картинка — в Content).
func pcall(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(callCtx(t), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: ошибка протокола: %v", name, err)
	}
	return res
}

// imageOf — картинка из ответа: байты JPEG; nil — картинки нет.
func imageOf(res *mcp.CallToolResult) []byte {
	for _, c := range res.Content {
		if ic, ok := c.(*mcp.ImageContent); ok {
			return ic.Data
		}
	}
	return nil
}

func jsonOf(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(textOf(res)), &m); err != nil {
		t.Fatalf("текст ответа не JSON: %v: %q", err, textOf(res))
	}
	return m
}

// calls считает вызовы Host.Preview/Render и запоминает последние аргументы.
type calls struct {
	mu        sync.Mutex
	n         atomic.Int32
	id        string
	px        int
	inFlight  atomic.Int32
	gate      chan struct{} // не nil — Host ждёт закрытия (или конца ctx при honorCtx)
	honorCtx  bool
	respond   func() ([]byte, error)
	maxInside atomic.Int32
}

func (k *calls) host(ctx context.Context, id string, px int) ([]byte, error) {
	k.n.Add(1)
	k.mu.Lock()
	k.id, k.px = id, px
	k.mu.Unlock()
	cur := k.inFlight.Add(1)
	defer k.inFlight.Add(-1)
	for {
		m := k.maxInside.Load()
		if cur <= m || k.maxInside.CompareAndSwap(m, cur) {
			break
		}
	}
	if k.gate != nil {
		if k.honorCtx {
			select {
			case <-k.gate:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		} else {
			<-k.gate
		}
	}
	if k.respond != nil {
		return k.respond()
	}
	return testJPEG(px, px*2/3)
}

func (k *calls) last() (string, int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.id, k.px
}

// previewRig — служба с подставным Host на уровне level; k считает вызовы Preview и Render вместе.
func previewRig(t *testing.T, level int, o Options) (*rig, *mcp.ClientSession, *calls) {
	t.Helper()
	r, cs := toolsRig(t, level, o)
	k := &calls{}
	r.fx.preview, r.fx.render = k.host, k.host
	return r, cs, k
}

// TestPreviewLevels: картинки только на уровне 3; ниже — отказ со словами, и Host не вызывался.
func TestPreviewLevels(t *testing.T) {
	for _, lv := range []int{1, 2, 3} {
		_, cs, k := previewRig(t, lv, Options{})
		for _, c := range []struct {
			name string
			args map[string]any
		}{{"get_photo_preview", map[string]any{"id": idA}}, {"get_spread_preview", map[string]any{"spread": 1}}} {
			res := pcall(t, cs, c.name, c.args)
			if lv < 3 {
				if !res.IsError || !strings.Contains(textOf(res), "Картинки агенту выключены") || imageOf(res) != nil {
					t.Errorf("уровень %d, %s: %+v", lv, c.name, res)
				}
				continue
			}
			img := imageOf(res)
			if res.IsError || img == nil {
				t.Fatalf("уровень 3, %s: %q", c.name, textOf(res))
			}
			cfg, err := jpeg.DecodeConfig(bytes.NewReader(img))
			if err != nil {
				t.Fatalf("%s: картинка: %v", c.name, err)
			}
			m := jsonOf(t, res)
			if int(m["w"].(float64)) != cfg.Width || int(m["h"].(float64)) != cfg.Height || m["level"].(float64) != 3 {
				t.Errorf("%s: подпись %v не сходится с картинкой %dx%d", c.name, m, cfg.Width, cfg.Height)
			}
		}
		if lv < 3 && k.n.Load() != 0 {
			t.Errorf("уровень %d: Host вызван %d раз", lv, k.n.Load())
		}
	}
}

// TestPreviewReply: подпись, схема выхода, аргументы Host, размеры по умолчанию и из входа, обложка.
func TestPreviewReply(t *testing.T) {
	_, cs, k := previewRig(t, 3, Options{})
	schemas := outSchemas(t, cs)
	r := mustOK(t, cs, schemas, "get_photo_preview", map[string]any{"id": " " + idA + " "})
	if r.Data["id"] != idA || r.Data["file"] != "ZQ1_Ivanov.jpg" || r.Data["w"].(float64) != 768 || r.Data["h"].(float64) != 512 {
		t.Errorf("подпись фото: %v", r.Data)
	}
	if id, px := k.last(); id != idA || px != 768 {
		t.Errorf("Host получил %q %d", id, px)
	}
	mustOK(t, cs, schemas, "get_photo_preview", map[string]any{"id": idA, "size": 512})
	if _, px := k.last(); px != 512 {
		t.Errorf("size 512: Host получил %d", px)
	}
	r = mustOK(t, cs, schemas, "get_spread_preview", map[string]any{"spread": "sp1"})
	if r.Data["n"].(float64) != 1 || r.Data["id"] != "sp1" || r.Data["w"].(float64) != 1200 || r.Data["unavailable"] != nil {
		t.Errorf("подпись разворота: %v", r.Data)
	}
	if id, px := k.last(); id != "sp1" || px != 1200 {
		t.Errorf("Host получил %q %d", id, px)
	}
	// id для Host берётся из вида, а не из ввода агента (0.31-143-60): номер и id с пробелами дают тот же "sp1".
	for _, ref := range []any{1, "1", " sp1 "} {
		res := pcall(t, cs, "get_spread_preview", map[string]any{"spread": ref})
		if res.IsError {
			t.Fatalf("spread %v: %q", ref, textOf(res))
		}
		if id, _ := k.last(); id != "sp1" {
			t.Errorf("spread %v: Host получил %q, ждали sp1", ref, id)
		}
		// Порядок Content закреплён (0.31-143-60): подпись текстом, затем картинка.
		if len(res.Content) != 2 {
			t.Fatalf("в ответе %d частей, ждали 2", len(res.Content))
		}
		if _, ok := res.Content[0].(*mcp.TextContent); !ok {
			t.Errorf("Content[0] — %T, ждали текст", res.Content[0])
		}
		if _, ok := res.Content[1].(*mcp.ImageContent); !ok {
			t.Errorf("Content[1] — %T, ждали картинку", res.Content[1])
		}
	}
	r = mustOK(t, cs, schemas, "get_spread_preview", map[string]any{"spread": "cover", "size": 800})
	if r.Data["cover"] != true || r.Data["n"].(float64) != 0 {
		t.Errorf("подпись обложки: %v", r.Data)
	}
	if id, px := k.last(); id != model.CoverID || px != 800 {
		t.Errorf("Host получил %q %d", id, px)
	}
}

// TestPreviewBadSize: size вне предела схемы — отказ до Host (граница, ноль, отрицательное, дробное, строка).
func TestPreviewBadSize(t *testing.T) {
	_, cs, k := previewRig(t, 3, Options{})
	for _, c := range []struct {
		name string
		args map[string]any
	}{
		{"get_photo_preview", map[string]any{"id": idA, "size": 511}}, {"get_photo_preview", map[string]any{"id": idA, "size": 1025}},
		{"get_photo_preview", map[string]any{"id": idA, "size": 0}}, {"get_photo_preview", map[string]any{"id": idA, "size": -5}},
		{"get_photo_preview", map[string]any{"id": idA, "size": 700.5}}, {"get_photo_preview", map[string]any{"id": idA, "size": "800"}},
		{"get_photo_preview", map[string]any{}}, {"get_photo_preview", map[string]any{"id": strings.Repeat("a", 65)}},
		{"get_spread_preview", map[string]any{"spread": 1, "size": 799}}, {"get_spread_preview", map[string]any{"spread": 1, "size": 1601}},
		{"get_spread_preview", map[string]any{"spread": 1, "size": 1 << 40}}, {"get_spread_preview", map[string]any{}},
	} {
		if r := tcall(t, cs, c.name, c.args); !r.IsError || !strings.HasPrefix(r.Text, "неверные аргументы") {
			t.Errorf("%s %v: %+v", c.name, c.args, r)
		}
	}
	for _, c := range []struct {
		name string
		args map[string]any
	}{{"get_photo_preview", map[string]any{"id": idA, "size": 1024}}, {"get_spread_preview", map[string]any{"spread": 2, "size": 1600}}} {
		if r := tcall(t, cs, c.name, c.args); r.IsError {
			t.Errorf("%s %v на границе: %q", c.name, c.args, r.Text)
		}
	}
	if k.n.Load() != 2 {
		t.Errorf("Host вызван %d раз, ждали 2 (только граничные)", k.n.Load())
	}
}

// TestPreviewUnknown: неизвестное фото и разворот, пропавшее и нечитаемое фото, альбома нет — отказы
// со словами, без Host и без путей.
func TestPreviewUnknown(t *testing.T) {
	r, cs, k := previewRig(t, 3, Options{})
	want := func(name string, args map[string]any, text string) {
		t.Helper()
		res := tcall(t, cs, name, args)
		if !res.IsError || !strings.Contains(res.Text, text) {
			t.Errorf("%s %v: %+v, ждали %q", name, args, res, text)
		}
		for _, bad := range []string{"/albums", "ZQ2dir", "bad.jpg", "gone.jpg", ".jpg"} {
			if strings.Contains(res.Text, bad) {
				t.Errorf("%s: в тексте ошибки %q", name, bad)
			}
		}
	}
	want("get_photo_preview", map[string]any{"id": "ffff"}, "нет такого фото")
	want("get_photo_preview", map[string]any{"id": idC}, "файл этого фото недоступен")
	want("get_photo_preview", map[string]any{"id": idD}, "файл этого фото недоступен")
	for _, s := range []any{99, 0, "-1", "nope", "cover2"} {
		want("get_spread_preview", map[string]any{"spread": s}, "нет такого разворота")
	}
	if k.n.Load() != 0 {
		t.Errorf("Host вызван %d раз на неверные запросы", k.n.Load())
	}
	r.fx.mu.Lock()
	r.fx.view.Cover = nil
	r.fx.mu.Unlock()
	want("get_spread_preview", map[string]any{"spread": "cover"}, "нет такого разворота")
	r.fx.mu.Lock()
	r.fx.ed = 0
	r.fx.mu.Unlock()
	want("get_photo_preview", map[string]any{"id": idA}, "альбом не открыт")
	want("get_spread_preview", map[string]any{"spread": 1}, "альбом не открыт")
	if k.n.Load() != 0 {
		t.Errorf("Host вызван %d раз без альбома", k.n.Load())
	}
}

// TestPreviewUnavailable: недоступные фото разворота (место и фон) считаются в ответе, рендер идёт.
func TestPreviewUnavailable(t *testing.T) {
	r, cs, _ := previewRig(t, 3, Options{})
	r.fx.mu.Lock()
	r.fx.missing = append(r.fx.missing, idE, idA)
	r.fx.mu.Unlock()
	res := pcall(t, cs, "get_spread_preview", map[string]any{"spread": 1})
	if res.IsError || jsonOf(t, res)["unavailable"].(float64) != 2 {
		t.Fatalf("ждали unavailable=2: %q", textOf(res))
	}
}

// TestPreviewJPEG: в выходе EncodePreview только SOI, DQT, SOF0, DHT, SOS (APP0 допустим); источники —
// JPEG с EXIF, GPS, XMP, ICC (после декодирования метаданных нет) и PNG с альфой (залит белым).
func TestPreviewJPEG(t *testing.T) {
	ok := map[byte]bool{0xD8: true, 0xDB: true, 0xC0: true, 0xC4: true, 0xDA: true, 0xE0: true}
	var base bytes.Buffer
	if err := jpeg.Encode(&base, gradient(300, 200), nil); err != nil {
		t.Fatal(err)
	}
	rich := withSegment(base.Bytes(), 0xE1, append([]byte("Exif\x00\x00"), []byte("GPSLatitude ZQ-secret")...))
	rich = withSegment(rich, 0xE1, []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta>ZQ-secret</x:xmpmeta>"))
	rich = withSegment(rich, 0xE2, []byte("ICC_PROFILE\x00\x01\x01ZQ-secret"))
	rich = withSegment(rich, 0xFE, []byte("ZQ-secret comment"))
	src, err := jpeg.Decode(bytes.NewReader(rich))
	if err != nil {
		t.Fatalf("источник с метаданными не читается: %v", err)
	}
	alpha := image.NewNRGBA(image.Rect(0, 0, 400, 300)) // целиком прозрачная
	for _, im := range []image.Image{src, alpha, gradient(5000, 40), gradient(1, 1)} {
		out, err := EncodePreview(im, 768)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range segments(t, out) {
			if !ok[m] {
				t.Errorf("в превью маркер FF%02X", m)
			}
		}
		cleanOutput(t, out)
		if _, _, _, err := normalizeJPEG(out, func(int, int) bool { return true }); err != nil {
			t.Errorf("своё превью отвергнуто: %v", err)
		}
	}
	// Прозрачность — белая, а не чёрная.
	out, _ := EncodePreview(alpha, 768)
	got, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if r, g, b, _ := got.At(10, 10).RGBA(); r>>8 < 250 || g>>8 < 250 || b>>8 < 250 {
		t.Errorf("прозрачное не белое: %d %d %d", r>>8, g>>8, b>>8)
	}
	// PNG через декодер даёт те же свойства.
	var pb bytes.Buffer
	_ = png.Encode(&pb, alpha)
	pi, err := png.Decode(&pb)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EncodePreview(pi, 512); err != nil {
		t.Error(err)
	}
	// Сторона: вписано в side, не растягивается.
	small, _ := EncodePreview(gradient(100, 50), 768)
	if cfg, _ := jpeg.DecodeConfig(bytes.NewReader(small)); cfg.Width != 100 || cfg.Height != 50 {
		t.Errorf("малое растянуто: %dx%d", cfg.Width, cfg.Height)
	}
	big, _ := EncodePreview(gradient(3000, 1000), 1024)
	if cfg, _ := jpeg.DecodeConfig(bytes.NewReader(big)); cfg.Width != 1024 || cfg.Height != 341 {
		t.Errorf("вписано не по длинной стороне: %dx%d", cfg.Width, cfg.Height)
	}
	if _, err := EncodePreview(image.NewRGBA(image.Rect(0, 0, 0, 0)), 768); err == nil {
		t.Error("пустая картинка принята")
	}
}

// TestPreviewHostBadOutput: служба не верит Host. Не JPEG, оборванный, пустой, с чудовищным размером —
// «внутренняя ошибка программы». JPEG с метаданными — любыми, в том числе теми, что разбор заголовков
// пропустил бы (после скана, FF00 с «длиной», второй JPEG в хвосте, второй SOF) — перекодируется:
// картинка уходит, секрета в ней нет (0.31-143-70).
func TestPreviewHostBadOutput(t *testing.T) {
	good, err := testJPEG(300, 200)
	if err != nil {
		t.Fatal(err)
	}
	patchSOF := func(b []byte, v ...byte) []byte { // SOF0: высота и ширина
		b = append([]byte{}, b...)
		for i := 0; i+8 < len(b); i++ {
			if b[i] == 0xFF && b[i+1] == 0xC0 {
				copy(b[i+5:], v)
				break
			}
		}
		return b
	}
	seg := func(m byte, data string) []byte {
		n := len(data) + 2
		return append([]byte{0xFF, m, byte(n >> 8), byte(n)}, data...)
	}
	tail := func(sg []byte) []byte { // сегмент после данных скана, перед EOI
		return append(append(append([]byte{}, good[:len(good)-2]...), sg...), 0xFF, 0xD9)
	}
	app1 := seg(0xE1, "Exif\x00\x00GPS ZQ-secret")
	ff00 := append(append([]byte{0xFF, 0xD8, 0xFF, 0x00, 0x00, byte(2 + len(app1))}, app1...), good[2:]...)
	appended := append(append([]byte{}, good...), append(append([]byte{0xFF, 0xD8}, app1...), good[2:]...)...)
	fakeSOF := seg(0xC0, "\x08\xEA\x60\xEA\x60\x03\x01\x22\x00\x02\x11\x01\x03\x11\x01")
	twoSOF := append(append([]byte{0xFF, 0xD8}, fakeSOF...), good[2:]...)
	var pb bytes.Buffer
	_ = png.Encode(&pb, gradient(10, 10))
	const (
		refuse = iota // отказ: внутренняя ошибка
		pass          // картинка без секрета
		either        // любое из двух, но без секрета
	)
	cases := map[string]struct {
		b    []byte
		want int
	}{
		"exif": {withSegment(good, 0xE1, []byte("Exif\x00\x00ZQ-secret")), pass}, "icc": {withSegment(good, 0xE2, []byte("ICC_PROFILE\x00ZQ-secret")), pass},
		"app13": {withSegment(good, 0xED, []byte("Photoshop 3.0ZQ-secret")), pass}, "com": {withSegment(good, 0xFE, []byte("ZQ-secret")), pass},
		"COM после скана": {tail(seg(0xFE, "ZQ-secret")), pass}, "APP1 после скана": {tail(app1), pass},
		"FF00 с длиной": {ff00, either}, "второй JPEG в хвосте": {appended, pass}, "два SOF": {twoSOF, either},
		"png": {pb.Bytes(), refuse}, "truncated": {good[:len(good)/2], refuse}, "noEOI": {good[:len(good)-2], either}, "empty": {[]byte{}, refuse}, "nil": {nil, refuse},
		"soi": {[]byte{0xFF, 0xD8}, refuse}, "huge": {patchSOF(good, 0xEA, 0x60, 0xEA, 0x60), refuse}, "zero": {patchSOF(good, 0, 0, 0, 0), refuse},
		"wide": {patchSOF(good, 0x08, 0x01, 0x08, 0x01), refuse}, "text": {[]byte("secret /home/user/x.jpg"), refuse},
		"garbage": {bytes.Repeat([]byte{0xFF}, 100), refuse},
		"badlen":  {[]byte{0xFF, 0xD8, 0xFF, 0xDB, 0xFF, 0xFF, 0xFF, 0xD9}, refuse}, "badlen2": {[]byte{0xFF, 0xD8, 0xFF, 0xDB, 0x00, 0x00, 0xFF, 0xD9}, refuse},
	}
	_, cs, k := previewRig(t, 3, Options{})
	for name, c := range cases {
		k.respond = func() ([]byte, error) { return c.b, nil }
		for _, tl := range []struct {
			tool string
			args map[string]any
		}{{"get_photo_preview", map[string]any{"id": idA}}, {"get_spread_preview", map[string]any{"spread": 1}}} {
			res := pcall(t, cs, tl.tool, tl.args)
			img := imageOf(res)
			switch {
			case res.IsError && (textOf(res) != "внутренняя ошибка программы" || img != nil):
				t.Errorf("%s/%s: %+v", name, tl.tool, res)
			case res.IsError && c.want == pass:
				t.Errorf("%s/%s: картинка отвергнута: %q", name, tl.tool, textOf(res))
			case !res.IsError && c.want == refuse:
				t.Errorf("%s/%s: принято то, что должно отказать", name, tl.tool)
			case !res.IsError:
				cleanOutput(t, img)
				if strings.Contains(textOf(res), "ZQ-secret") {
					t.Errorf("%s/%s: секрет в подписи", name, tl.tool)
				}
			}
		}
	}
	k.respond = nil
	if res := pcall(t, cs, "get_photo_preview", map[string]any{"id": idA}); res.IsError {
		t.Fatalf("после мусора: %q", textOf(res))
	}
}

// TestPreviewHostOversize: Host вернул картинку больше запрошенной — отказ (в 4 раза больше токенов
// агенту не уйдёт); ровно запрошенная проходит. Для фото считается длинная сторона, для разворота — ширина.
func TestPreviewHostOversize(t *testing.T) {
	_, cs, k := previewRig(t, 3, Options{})
	for _, c := range []struct {
		tool string
		args map[string]any
		w, h int
		want bool
	}{
		{"get_photo_preview", map[string]any{"id": idA, "size": 512}, 768, 100, false},
		{"get_photo_preview", map[string]any{"id": idA, "size": 512}, 100, 768, false},
		{"get_photo_preview", map[string]any{"id": idA, "size": 512}, 512, 512, true},
		{"get_photo_preview", map[string]any{"id": idA, "size": 512}, 1, 1, true},
		{"get_spread_preview", map[string]any{"spread": 1, "size": 800}, 801, 100, false},
		{"get_spread_preview", map[string]any{"spread": 1, "size": 800}, 800, 900, true}, // высоту спека не ограничивает
		{"get_spread_preview", map[string]any{"spread": 1, "size": 800}, 2049, 10, false},
	} {
		k.respond = func() ([]byte, error) { return rawJPEG(c.w, c.h) }
		res := pcall(t, cs, c.tool, c.args)
		if ok := !res.IsError && imageOf(res) != nil; ok != c.want {
			t.Errorf("%s %v, картинка %dx%d: принята=%v, ждали %v (%q)", c.tool, c.args, c.w, c.h, ok, c.want, textOf(res))
		}
	}
}

// rawJPEG — JPEG ровно w×h без уменьшения.
func rawJPEG(w, h int) ([]byte, error) {
	var b bytes.Buffer
	err := jpeg.Encode(&b, gradient(w, h), nil)
	return b.Bytes(), err
}

// TestPreviewCap: шум 1600 px не выходит за 600 КБ (качество и сторона уступают), строка ответа на
// проводе укладывается в предел; Host с картинкой больше предела — «слишком велик».
func TestPreviewCap(t *testing.T) {
	if enc := base64.StdEncoding.EncodedLen(maxJPEG) + 4096; enc >= maxReply {
		t.Fatalf("максимальная картинка в base64 (%d) не лезет в строку ответа (%d)", enc, maxReply)
	}
	out, err := EncodePreview(noise(1600, 1000), 1600)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > maxJPEG {
		t.Fatalf("превью %d байт, предел %d", len(out), maxJPEG)
	}
	if !testkit.Lite() { // 1–2 с и тяжело под -race: полный перебор качества и сторон
		big, err := EncodePreview(noise(2048, 2048), 2048)
		if err != nil || len(big) > maxJPEG {
			t.Fatalf("шум 2048: %d байт, %v", len(big), err)
		}
		if cfg, _ := jpeg.DecodeConfig(bytes.NewReader(big)); cfg.Width >= 2048 {
			t.Errorf("шум 2048 не пришлось уменьшать (%d)? сторона должна уступить пределу", cfg.Width)
		}
	}

	_, cs, k := previewRig(t, 3, Options{})
	k.respond = func() ([]byte, error) { return out, nil }
	res := pcall(t, cs, "get_spread_preview", map[string]any{"spread": 1, "size": 1600})
	if im := imageOf(res); res.IsError || len(im) == 0 || len(im) > maxJPEG { // перекодирование шума у предела может ещё раз ужать сторону
		t.Fatalf("картинка у предела: %q", textOf(res))
	}
	if w, _ := json.Marshal(res); len(w) > maxReply {
		t.Errorf("строка ответа %d > %d", len(w), maxReply)
	}
	// Картинка чуть больше предела: в base64 она ещё влезла бы в строку ответа — отказывает именно предел байт.
	var over bytes.Buffer
	for h := 300; over.Len() <= maxJPEG; h += 20 {
		over.Reset()
		if err := jpeg.Encode(&over, noise(1024, h), &jpeg.Options{Quality: 100}); err != nil || h > 1500 {
			t.Fatalf("не подобрать картинку: %d %v", over.Len(), err)
		}
	}
	if base64.StdEncoding.EncodedLen(over.Len()) >= maxReply {
		t.Fatalf("тестовая картинка слишком велика (%d): предел байт не отделить от предела строки", over.Len())
	}
	k.respond = func() ([]byte, error) { return over.Bytes(), nil }
	for _, c := range []struct {
		tool string
		args map[string]any
	}{{"get_photo_preview", map[string]any{"id": idA}}, {"get_spread_preview", map[string]any{"spread": 1}}} {
		if r := tcall(t, cs, c.tool, c.args); !r.IsError || !strings.Contains(r.Text, "слишком велик") {
			t.Errorf("%s: %+v", c.tool, r)
		}
	}
}

// TestPreviewHostErrors: ошибки Host не выходят наружу текстом (в них бывают пути); ErrUnavailable —
// понятный отказ; паника Host — внутренняя ошибка, служба жива.
func TestPreviewHostErrors(t *testing.T) {
	_, cs, k := previewRig(t, 3, Options{})
	for name, c := range map[string]struct {
		err  error
		want string
	}{
		"path":        {fmt.Errorf("open /home/user/ZQ2dir/x.jpg: no such file"), "внутренняя ошибка программы"},
		"unavailable": {fmt.Errorf("%w: /home/user/ZQ2dir/x.jpg", ErrUnavailable), "файл этого фото недоступен"},
	} {
		k.respond = func() ([]byte, error) { return nil, c.err }
		for _, tool := range []struct {
			n string
			a map[string]any
		}{{"get_photo_preview", map[string]any{"id": idA}}, {"get_spread_preview", map[string]any{"spread": 1}}} {
			r := tcall(t, cs, tool.n, tool.a)
			if !r.IsError || !strings.Contains(r.Text, c.want) || strings.Contains(r.Text, "/home") || strings.Contains(r.Text, "ZQ2dir") {
				t.Errorf("%s/%s: %+v", name, tool.n, r)
			}
		}
	}
	k.respond = func() ([]byte, error) { panic("секрет /home/user") }
	if r := tcall(t, cs, "get_photo_preview", map[string]any{"id": idA}); !r.IsError || r.Text != "внутренняя ошибка программы" {
		t.Errorf("паника Host: %+v", r)
	}
	k.respond = nil
	if r := tcall(t, cs, "get_photo_preview", map[string]any{"id": idA}); r.IsError {
		t.Errorf("после паники: %+v", r)
	}
}

// TestPreviewDeadline: медленный Host — «время вышло» в срок, а не зависание; ctx Host отменён;
// зависший Host (не слушает ctx) держит место до своего конца; у рендера срок вдвое длиннее.
func TestPreviewDeadline(t *testing.T) {
	const d = 150 * time.Millisecond
	r, cs, k := previewRig(t, 3, Options{CallTimeout: d, MaxCalls: 1})
	_ = r
	k.gate, k.honorCtx = make(chan struct{}), true
	start := time.Now()
	res := pcall(t, cs, "get_photo_preview", map[string]any{"id": idA})
	if el := time.Since(start); !res.IsError || !strings.Contains(textOf(res), "время вышло") || el > testkit.Limit(5*time.Second) {
		t.Fatalf("медленный Host с ctx: %q за %v", textOf(res), el)
	}
	waitFor(t, "Host вышел по ctx", func() bool { return k.inFlight.Load() == 0 })

	start = time.Now()
	res = pcall(t, cs, "get_spread_preview", map[string]any{"spread": 1})
	if el := time.Since(start); !res.IsError || !strings.Contains(textOf(res), "время вышло") || el < 2*d*9/10 {
		t.Fatalf("рендер: %q за %v, ждали срок ≥ %v", textOf(res), el, 2*d)
	}
	waitFor(t, "Host вышел по ctx", func() bool { return k.inFlight.Load() == 0 })

	// Host не слушает ctx: ответ агенту приходит по сроку, место занято, пока Host не вернётся.
	k.honorCtx = false
	start = time.Now()
	res = pcall(t, cs, "get_photo_preview", map[string]any{"id": idA})
	if el := time.Since(start); !res.IsError || !strings.Contains(textOf(res), "время вышло") || el > testkit.Limit(5*time.Second) {
		t.Fatalf("зависший Host: %q за %v", textOf(res), el)
	}
	if res := pcall(t, cs, "get_album", nil); !res.IsError || !strings.Contains(textOf(res), "занято") {
		t.Errorf("место должно держаться, пока Host висит: %q", textOf(res))
	}
	close(k.gate)
	waitFor(t, "место свободно", func() bool {
		return !pcall(t, cs, "get_album", nil).IsError
	})
}

// TestPreviewSlots: места вызовов ограничены (MaxCalls), рендер разворота — один; лишние получают
// «занято», а после освобождения работают.
func TestPreviewSlots(t *testing.T) {
	_, cs, k := previewRig(t, 3, Options{MaxCalls: 2})
	k.gate = make(chan struct{})
	done := make(chan *mcp.CallToolResult, 3)
	launch := func(name string, args map[string]any) {
		go func() {
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
			if err != nil {
				res = &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}
			}
			done <- res
		}()
	}
	launch("get_photo_preview", map[string]any{"id": idA})
	launch("get_photo_preview", map[string]any{"id": idB})
	waitFor(t, "два вызова внутри Host", func() bool { return k.inFlight.Load() == 2 })
	if res := pcall(t, cs, "get_photo_preview", map[string]any{"id": idA}); !res.IsError || !strings.Contains(textOf(res), "занято") {
		t.Fatalf("третий вызов: %q", textOf(res))
	}
	close(k.gate)
	for range 2 {
		if res := <-done; res.IsError || imageOf(res) == nil {
			t.Fatalf("параллельный вызов: %q", textOf(res))
		}
	}
	if k.maxInside.Load() > 2 {
		t.Errorf("в Host одновременно %d вызовов при MaxCalls 2", k.maxInside.Load())
	}
	waitFor(t, "места свободны", func() bool { return !pcall(t, cs, "get_photo_preview", map[string]any{"id": idA}).IsError })
}

// TestPreviewOneRender: два рендера разворота одновременно не идут — второй «занято», фото и чтение
// при этом работают.
func TestPreviewOneRender(t *testing.T) {
	r, cs, _ := previewRig(t, 3, Options{})
	gate := make(chan struct{})
	var renders atomic.Int32
	r.fx.render = func(ctx context.Context, id string, px int) ([]byte, error) {
		renders.Add(1)
		<-gate
		return testJPEG(px, px/2)
	}
	r.fx.preview = nil
	done := make(chan *mcp.CallToolResult, 1)
	go func() {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_spread_preview", Arguments: map[string]any{"spread": 1}})
		if err != nil {
			res = &mcp.CallToolResult{IsError: true}
		}
		done <- res
	}()
	waitFor(t, "рендер начался", func() bool { return renders.Load() == 1 })
	if res := pcall(t, cs, "get_spread_preview", map[string]any{"spread": 2}); !res.IsError || !strings.Contains(textOf(res), "занято") {
		t.Fatalf("второй рендер: %q", textOf(res))
	}
	if res := pcall(t, cs, "get_photo_preview", map[string]any{"id": idA}); res.IsError {
		t.Fatalf("превью фото во время рендера: %q", textOf(res))
	}
	if res := pcall(t, cs, "get_album", nil); res.IsError {
		t.Fatalf("чтение во время рендера: %q", textOf(res))
	}
	if renders.Load() != 1 {
		t.Errorf("второй рендер дошёл до Host (%d)", renders.Load())
	}
	close(gate)
	if res := <-done; res.IsError {
		t.Fatalf("первый рендер: %q", textOf(res))
	}
	waitFor(t, "рендер свободен", func() bool { return !pcall(t, cs, "get_spread_preview", map[string]any{"spread": 2}).IsError })
}

// TestPreviewParallel (-race): много клиентов и вызовов сразу — каждый ответ либо картинка, либо
// «занято»; внутренних ошибок нет, в Host не больше мест, чем MaxCalls.
func TestPreviewParallel(t *testing.T) {
	r, _, k := previewRig(t, 3, Options{MaxCalls: 3})
	k.respond = func() ([]byte, error) { time.Sleep(2 * time.Millisecond); return testJPEG(512, 340) }
	var wg sync.WaitGroup
	var okN, busyN atomic.Int32
	var bad atomic.Value
	for i := range 6 {
		cs := client(t, r.sock, fmt.Sprintf("agent%d", i))
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 15 {
				name, args := "get_photo_preview", map[string]any{"id": idA}
				if j%3 == 0 {
					name, args = "get_spread_preview", map[string]any{"spread": 1}
				}
				switch msg := call2(cs, name, args); {
				case msg == "":
					okN.Add(1)
				case strings.Contains(msg, "занято"):
					busyN.Add(1)
				default:
					bad.Store(msg)
				}
			}
		}()
	}
	wg.Wait()
	if v := bad.Load(); v != nil {
		t.Fatalf("неожиданный отказ: %v", v)
	}
	if okN.Load() == 0 || k.maxInside.Load() > 3 {
		t.Errorf("успехов %d, занято %d, одновременно в Host %d (предел 3)", okN.Load(), busyN.Load(), k.maxInside.Load())
	}
}

// TestPreviewLevelDropsMidCall: окно выключило картинки, пока шёл рендер, — картинка не отдаётся.
func TestPreviewLevelDropsMidCall(t *testing.T) {
	r, cs, k := previewRig(t, 3, Options{})
	k.respond = func() ([]byte, error) {
		setLevel(r.cfg, 2)
		return testJPEG(512, 300)
	}
	res := pcall(t, cs, "get_photo_preview", map[string]any{"id": idA})
	if !res.IsError || imageOf(res) != nil || !strings.Contains(textOf(res), "Картинки агенту выключены") {
		t.Fatalf("уровень упал в середине вызова: %+v", res)
	}
}

// TestNormalizeEdges: разбор ответа Host на краях — ошибка, а не паника; хорошее проходит и очищается.
func TestNormalizeEdges(t *testing.T) {
	good, err := testJPEG(64, 48)
	if err != nil {
		t.Fatal(err)
	}
	all := func(int, int) bool { return true }
	out, w, h, err := normalizeJPEG(withSegment(good, 0xE1, []byte("Exif\x00\x00ZQ-secret")), all)
	if err != nil || w != 64 || h != 48 {
		t.Fatalf("хорошая: %d×%d %v", w, h, err)
	}
	cleanOutput(t, out)
	if _, _, _, err := normalizeJPEG(good, func(w, h int) bool { return w < 64 }); !errors.Is(err, errJPEGSize) {
		t.Errorf("fits отказал, а ответ принят: %v", err)
	}
	for i := range good { // любой обрыв — ошибка или картинка, но не паника
		if o, _, _, err := normalizeJPEG(good[:i], all); err == nil {
			cleanOutput(t, o)
		}
	}
	for i := range good[:200] { // любая испорченная позиция заголовка
		b := append([]byte{}, good...)
		b[i] ^= 0xFF
		if o, _, _, err := normalizeJPEG(b, all); err == nil {
			cleanOutput(t, o)
		}
	}
	if _, _, _, err := normalizeJPEG(nil, all); !errors.Is(err, errNotJPEG) {
		t.Error("nil принят")
	}
	wide, err := rawJPEG(maxPreviewSide+1, 8) // сама служба не принимает больше 2048, что бы ни разрешал fits
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := normalizeJPEG(wide, all); !errors.Is(err, errJPEGSize) {
		t.Errorf("шире предела принято: %v", err)
	}
}

// FuzzNormalizeJPEG: на любой вход — ошибка или чистый выход (независимый проход cleanOutput по всем
// байтам: ни APPn кроме APP0, ни COM, ни сегментов после скана, EOI ровно в конце), в пределах размера.
func FuzzNormalizeJPEG(f *testing.F) {
	good, _ := testJPEG(32, 24)
	f.Add(good)
	f.Add(withSegment(good, 0xE1, []byte("Exif")))
	f.Add(append(append([]byte{}, good...), good...))
	f.Add(append(append([]byte{}, good[:len(good)-2]...), 0xFF, 0xFE, 0x00, 0x04, 'Z', 'Q', 0xFF, 0xD9))
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xD9})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		out, w, h, err := normalizeJPEG(b, func(int, int) bool { return true })
		if err != nil {
			return
		}
		if w <= 0 || h <= 0 || max(w, h) > maxPreviewSide || len(out) > maxJPEG {
			t.Fatalf("принят выход %d×%d, %d байт", w, h, len(out))
		}
		cleanOutput(t, out)
	})
}

// TestPreviewAfterDeadline: Host уложился «в последний момент» и вернул хорошую картинку уже после
// срока — агенту «время вышло», а не запоздавший ответ.
func TestPreviewAfterDeadline(t *testing.T) {
	r := newRig(t, Options{}, "")
	setLevel(r.cfg, 3)
	good, err := testJPEG(300, 200)
	if err != nil {
		t.Fatal(err)
	}
	all := func(int, int) bool { return true }
	ctx, cancel := context.WithCancel(context.Background())
	c := &call{ctx: ctx, s: r.svc, level: 3}
	cancel()
	if _, _, _, err := c.image(all, func() ([]byte, error) { return good, nil }); !errors.Is(err, errTimeout) {
		t.Fatalf("после срока: %v", err)
	}
	c.ctx = context.Background()
	if _, w, h, err := c.image(all, func() ([]byte, error) { return good, nil }); err != nil || w != 300 || h != 200 {
		t.Fatalf("в срок: %d×%d %v", w, h, err)
	}
}
