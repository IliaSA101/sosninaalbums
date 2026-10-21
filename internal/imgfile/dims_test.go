package imgfile

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// pngHeader — PNG с настоящим заголовком w×h и без данных: DecodeConfig хватает IHDR,
// а 15000² одного цвета (~1 ГБ в памяти) в тесте не нужны.
func pngHeader(w, h uint32) []byte {
	var b bytes.Buffer
	b.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6 // 8 бит, RGBA
	chunk := append([]byte("IHDR"), ihdr...)
	_ = binary.Write(&b, binary.BigEndian, uint32(13))
	b.Write(chunk)
	_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return b.Bytes()
}

func readFile(t *testing.T, p string) []byte {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDimsOf(t *testing.T) {
	dir := testkit.TempDir(t)
	jpg := readFile(t, testkit.JPEG(t, dir, "a.jpg", 20, 10))
	png := readFile(t, testkit.PNG(t, dir, "a.png", 8, 6, color.NRGBA{A: 255}))
	var g bytes.Buffer
	if err := gif.Encode(&g, image.NewPaletted(image.Rect(0, 0, 8, 8), color.Palette{color.Black, color.White}), nil); err != nil {
		t.Fatal(err)
	}
	huge := readFile(t, testkit.HugeJPEG(t, dir, "h.jpg", 20000, 20000)) // 400 Мп

	ok := []struct {
		name   string
		data   []byte
		w, h   int
		format string
	}{{"jpeg", jpg, 20, 10, "jpeg"}, {"png", png, 8, 6, "png"}}
	for _, c := range ok {
		d, err := DimsOf(bytes.NewReader(c.data), 0)
		if err != nil || d.W != c.w || d.H != c.h || d.Format != c.format || d.Color == nil {
			t.Errorf("%s: %+v, %v", c.name, d, err)
		}
	}

	if _, err := DimsOf(&g, 0); err == nil || errors.Is(err, ErrTooBig) {
		t.Errorf("gif: ждали отказ формата, получено %v", err)
	}
	if _, err := DimsOf(bytes.NewReader([]byte("не картинка")), 0); err == nil {
		t.Error("мусор принят")
	}
	if _, err := DimsOf(bytes.NewReader(nil), 0); err == nil {
		t.Error("пустой поток принят")
	}
	// 0×0 и нулевая сторона — отказ (не ErrTooBig).
	for _, z := range [][]byte{pngHeader(0, 0), pngHeader(0, 10), pngHeader(10, 0), readFile(t, testkit.HugeJPEG(t, dir, "z.jpg", 0, 0))} {
		if _, err := DimsOf(bytes.NewReader(z), 0); err == nil || errors.Is(err, ErrTooBig) {
			t.Errorf("нулевой размер: %v", err)
		}
	}

	// Предел: ровно в пределе проходит, на пиксель больше — ErrTooBig с размерами и пределом.
	if _, err := DimsOf(bytes.NewReader(png), 48); err != nil {
		t.Errorf("ровно в пределе: %v", err)
	}
	_, err := DimsOf(bytes.NewReader(png), 47)
	if !errors.Is(err, ErrTooBig) || !strings.Contains(err.Error(), "8x6") {
		t.Errorf("на пиксель больше: %v", err)
	}
	_, err = DimsOf(bytes.NewReader(huge), 100_000_000)
	if !errors.Is(err, ErrTooBig) || !strings.Contains(err.Error(), "20000x20000") || !strings.Contains(err.Error(), "100 Мп") {
		t.Errorf("400 Мп при пределе 100 Мп: %v", err)
	}
	// maxPx = 0 — MaxPixels (250 Мп): прежний текст; больше MaxPixels не бывает.
	_, err = DimsOf(bytes.NewReader(huge), 0)
	if !errors.Is(err, ErrTooBig) || !strings.Contains(err.Error(), "больше 250 Мп") {
		t.Errorf("maxPx=0: %v", err)
	}
	if _, err = DimsOf(bytes.NewReader(huge), 10*MaxPixels); !errors.Is(err, ErrTooBig) {
		t.Errorf("предел выше MaxPixels не зажат: %v", err)
	}
	if _, err = DimsOf(bytes.NewReader(pngHeader(15000, 15000)), 0); err != nil { // 225 Мп < 250 Мп
		t.Errorf("15000² при MaxPixels: %v", err)
	}

	// Раздутый PNG 15000² при пределе рамочки — отказ за миллисекунды (079-35).
	big := pngHeader(15000, 15000)
	var gotErr error
	d := testkit.Fastest(5, func() { _, gotErr = DimsOf(bytes.NewReader(big), 64_000_000) })
	if !errors.Is(gotErr, ErrTooBig) {
		t.Errorf("PNG 15000²: %v", gotErr)
	}
	if lim := testkit.Limit(20 * time.Millisecond); d > lim {
		t.Errorf("отказ за %v, предел %v", d, lim)
	}
}

func TestFileDimsNotRegular(t *testing.T) {
	dir := testkit.TempDir(t)
	if _, err := FileDims(dir, 0); err == nil {
		t.Error("каталог принят")
	}
	if _, err := FileDims(filepath.Join(dir, "нет.png"), 0); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("нет файла: %v", err)
	}
	p := testkit.PNG(t, dir, "a.png", 8, 6, color.NRGBA{A: 255})
	d, err := FileDims(p, 0)
	if err != nil || d.W != 8 || d.H != 6 || d.Format != "png" {
		t.Errorf("обычный файл: %+v, %v", d, err)
	}
	if _, err := FileDims(p, 10); !errors.Is(err, ErrTooBig) {
		t.Errorf("предел файла: %v", err)
	}
}

// waitWaiting ждёт, пока в бюджете встанет n ожидающих (без пауз-угадайки).
func waitWaiting(t *testing.T, b *Budget, n int) {
	t.Helper()
	deadline := time.Now().Add(testkit.Limit(5 * time.Second))
	for b.Waiting() < n {
		if time.Now().After(deadline) {
			t.Fatalf("ожидающих %d, ждали %d", b.Waiting(), n)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestBudgetAcquireCtx(t *testing.T) {
	ctx := context.Background()

	// Отмена в ожидании: context.Canceled, Used прежний.
	b := NewBudget(100)
	b.Acquire(80)
	cctx, cancel := context.WithCancel(ctx)
	res := make(chan error, 1)
	go func() { res <- b.AcquireCtx(cctx, 50) }()
	waitWaiting(t, b, 1)
	select {
	case err := <-res:
		t.Fatalf("прошёл при занятом бюджете: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-res:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ждали Canceled: %v", err)
		}
	case <-time.After(testkit.Limit(time.Second)):
		t.Fatal("отмена не разбудила ожидающего")
	}
	if b.Used() != 80 {
		t.Fatalf("Used %d, ждали 80", b.Used())
	}

	// Освобождение — ожидающий проходит.
	go func() { res <- b.AcquireCtx(ctx, 50) }()
	waitWaiting(t, b, 1)
	b.Release(80)
	select {
	case err := <-res:
		if err != nil {
			t.Fatalf("после освобождения: %v", err)
		}
	case <-time.After(testkit.Limit(time.Second)):
		t.Fatal("освобождение не разбудило ожидающего")
	}
	if b.Used() != 50 {
		t.Fatalf("Used %d, ждали 50", b.Used())
	}
	b.Release(50)

	// Запрос больше лимита: ждёт, пока освободится всё, и идёт один.
	b.Acquire(10)
	go func() { res <- b.AcquireCtx(ctx, 500) }()
	waitWaiting(t, b, 1)
	select {
	case err := <-res:
		t.Fatalf("большой прошёл при занятом бюджете: %v", err)
	default:
	}
	b.Release(10)
	if err := <-res; err != nil {
		t.Fatalf("большой после освобождения: %v", err)
	}
	if b.Used() != 500 {
		t.Fatalf("Used %d, ждали 500", b.Used())
	}
	b.Release(500)

	// Отменённый ctx — отказ сразу, даже если место есть; Used не тронут.
	dead, kill := context.WithCancel(ctx)
	kill()
	if err := b.AcquireCtx(dead, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("отменённый ctx при свободном бюджете: %v", err)
	}
	if b.Used() != 0 {
		t.Fatalf("Used %d после отказа", b.Used())
	}
	// Ноль и пустой бюджет — обычный путь.
	if err := b.AcquireCtx(ctx, 0); err != nil {
		t.Fatalf("n=0: %v", err)
	}
	b.Release(0)
}

// 50 горутин с случайными отменами: после всего бюджет пуст, ни одного зависшего.
func TestBudgetAcquireCtxRace(t *testing.T) {
	b := NewBudget(100)
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Go(func() {
			r := rand.New(rand.NewPCG(uint64(i), 7))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if r.IntN(2) == 0 {
				time.AfterFunc(time.Duration(r.IntN(3000))*time.Microsecond, cancel)
			}
			n := int64(10 + r.IntN(60))
			if i%10 == 0 {
				n = 300 // больше лимита
			}
			if err := b.AcquireCtx(ctx, n); err != nil {
				if !errors.Is(err, context.Canceled) {
					t.Errorf("горутина %d: %v", i, err)
				}
				return
			}
			time.Sleep(time.Duration(r.IntN(1000)) * time.Microsecond)
			b.Release(n)
		})
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(testkit.Limit(20 * time.Second)):
		t.Fatal("горутины зависли")
	}
	if b.Used() != 0 {
		t.Fatalf("Used() = %d, ждали 0", b.Used())
	}
}

// Общий бюджет занят: OpenOrientedCtx ждёт и по отмене возвращает ctx.Err(), не тронув бюджет;
// после освобождения тот же файл открывается.
func TestOpenOrientedCtxCancel(t *testing.T) {
	p := testkit.JPEG(t, testkit.TempDir(t), "a.jpg", 20, 20)
	DecodeBudget.Acquire(DecodeBudgetPixels)
	held := true
	defer func() {
		if held {
			DecodeBudget.Release(DecodeBudgetPixels)
		}
	}()
	before := DecodeBudget.Used()
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(30*time.Millisecond, cancel)
	start := time.Now()
	_, _, err := OpenOrientedCtx(ctx, p)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ждали Canceled: %v", err)
	}
	if d := time.Since(start); d > testkit.Limit(500*time.Millisecond) {
		t.Errorf("отмена заметна только через %v", d)
	}
	if got := DecodeBudget.Used(); got != before {
		t.Fatalf("бюджет тронут: %d вместо %d", got, before)
	}
	DecodeBudget.Release(DecodeBudgetPixels)
	held = false
	img, release, err := OpenOrientedCtx(context.Background(), p)
	if err != nil {
		t.Fatalf("после освобождения: %v", err)
	}
	release()
	if img.Bounds().Dx() != 20 {
		t.Fatalf("размер %v", img.Bounds())
	}
	// Уже отменённый ctx — отказ сразу (проверка до захвата бюджета).
	dead, kill := context.WithCancel(context.Background())
	kill()
	if _, _, err := OpenOrientedCtx(dead, p); !errors.Is(err, context.Canceled) {
		t.Fatalf("отменённый ctx: %v", err)
	}
	if DecodeBudget.Used() != 0 {
		t.Fatalf("Used %d", DecodeBudget.Used())
	}
}

// probeCalls — сколько раз image.DecodeConfig дошёл до чужого (не JPEG/PNG) разборщика.
var probeCalls atomic.Int32

func init() {
	image.RegisterFormat("saprobe", "SAPROBE", func(io.Reader) (image.Image, error) {
		return nil, errors.New("не нужен")
	}, func(io.Reader) (image.Config, error) {
		probeCalls.Add(1)
		return image.Config{ColorModel: color.RGBAModel, Width: 1, Height: 1}, nil
	})
}

// Формат — по магическим байтам: зарегистрированный чужой разборщик (как TIFF/BMP/GIF от imaging)
// не запускается вовсе, а не отсекается после разбора; JPEG и PNG — своими декодерами.
func TestDimsOfMagicOnly(t *testing.T) {
	if _, err := DimsOf(strings.NewReader("SAPROBE и что угодно"), 0); err == nil {
		t.Fatal("чужой формат принят")
	}
	if n := probeCalls.Load(); n != 0 {
		t.Fatalf("чужой разборщик запущен %d раз", n)
	}
	// Подпись PNG на месте, IHDR битый — ошибка PNG, не «формат не тот» и не паника.
	if _, err := DimsOf(bytes.NewReader([]byte(pngMagic+"мусор")), 0); err == nil {
		t.Fatal("битый PNG принят")
	}
	// Короче подписи и одна FF D8 — отказ без паники.
	for _, b := range [][]byte{{0x89, 'P'}, {0xFF}, {0xFF, 0xD8}} {
		if _, err := DimsOf(bytes.NewReader(b), 0); err == nil {
			t.Fatalf("% x принят", b)
		}
	}
	// Сбой чтения — та же ошибка, а не «формат не поддерживается».
	boom := errors.New("диск отвалился")
	if _, err := DimsOf(io.MultiReader(bytes.NewReader([]byte{0xFF}), errReader{boom}), 0); !errors.Is(err, boom) {
		t.Fatalf("сбой чтения: %v", err)
	}
}

type errReader struct{ err error }

func (r errReader) Read([]byte) (int, error) { return 0, r.err }

// FuzzDimsOf — любые байты: без паники; принятое — JPEG или PNG с размером в пределе.
func FuzzDimsOf(f *testing.F) {
	f.Add(pngHeader(8, 6))
	f.Add(pngHeader(15000, 15000))
	f.Add(testkit.RawJPEG())
	f.Add([]byte("SAPROBE"))
	f.Add([]byte{})
	const maxPx = 64_000_000
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := DimsOf(bytes.NewReader(data), maxPx)
		if err != nil {
			return
		}
		if d.Format != "jpeg" && d.Format != "png" {
			t.Fatalf("формат %q", d.Format)
		}
		if d.W <= 0 || d.H <= 0 || int64(d.W)*int64(d.H) > maxPx {
			t.Fatalf("размер %dx%d", d.W, d.H)
		}
	})
}
