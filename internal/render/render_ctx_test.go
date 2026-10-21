package render

import (
	"context"
	"errors"
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Бюджет декода занят (прогрев миниатюр): RenderCtx по отмене возвращает ctx.Err() сразу, а не
// после ожидания места (080-8); после освобождения тот же лист рисуется. Фото — прямое, с
// поворотом на произвольный угол (путь 3) и со стикером-файлом: каждый путь ждёт бюджет сам.
func TestRenderCtxCancelWaitingBudget(t *testing.T) {
	dir := testkit.TempDir(t)
	a := testkit.JPEG(t, dir, "a.jpg", 300, 200)
	b := testkit.JPEG(t, dir, "b.jpg", 300, 200)
	rotated := coverPhoto(layout.Rect{X: 60, Y: 10, W: 30, H: 20}, 1.5, b)
	rotated.Rot = 7
	s := Spread{Photos: []Photo{
		coverPhoto(layout.Rect{X: 10, Y: 10, W: 40, H: 30}, 1.5, a),
		rotated,
		coverPhoto(layout.Rect{X: 110, Y: 10, W: 40, H: 30}, 1.5, a),
	}}
	opt := Options{PageW: 100, PageH: 60, DPI: 100}

	imgfile.DecodeBudget.Acquire(imgfile.DecodeBudgetPixels)
	held := true
	defer func() {
		if held {
			imgfile.DecodeBudget.Release(imgfile.DecodeBudgetPixels)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(30*time.Millisecond, cancel)
	start := time.Now()
	img, err := RenderCtx(ctx, s, opt)
	if !errors.Is(err, context.Canceled) || img != nil {
		t.Fatalf("ждали Canceled без холста: %v, %v", img != nil, err)
	}
	if d := time.Since(start); d > testkit.Limit(200*time.Millisecond) {
		t.Errorf("отмена в ожидании бюджета заметна через %v", d)
	}
	if err.Error() != context.Canceled.Error() { // отмена идёт наверх без обёртки «фото: …»
		t.Errorf("отмена обёрнута: %v", err)
	}

	imgfile.DecodeBudget.Release(imgfile.DecodeBudgetPixels)
	held = false
	if imgfile.DecodeBudget.Used() != 0 {
		t.Fatalf("бюджет не пуст: %d", imgfile.DecodeBudget.Used())
	}
	// Холостая правка: без отмены RenderCtx = Render, холст тот же.
	want, err := Render(s, opt)
	if err != nil {
		t.Fatalf("после освобождения: %v", err)
	}
	got, err := RenderCtx(context.Background(), s, opt)
	if err != nil || string(got.Pix) != string(want.Pix) {
		t.Fatalf("RenderCtx без отмены отличается от Render: %v", err)
	}
	// Уже отменённый ctx — отказ до любой работы.
	dead, kill := context.WithCancel(context.Background())
	kill()
	if _, err := RenderCtx(dead, s, opt); !errors.Is(err, context.Canceled) {
		t.Fatalf("отменённый ctx: %v", err)
	}
	// Пустой лист: тоже отказ (отмена важнее «нечего рисовать»).
	if _, err := RenderCtx(dead, Spread{}, opt); !errors.Is(err, context.Canceled) {
		t.Fatalf("пустой лист, отменённый ctx: %v", err)
	}
	if imgfile.DecodeBudget.Used() != 0 {
		t.Fatalf("бюджет не пуст после отмен: %d", imgfile.DecodeBudget.Used())
	}
}

// TestRenderCtxCanceledBefore — уже отменённый ctx: отказ до выделения холста, даже у листа без
// файлов и при свободном бюджете (холст в сотни МБ не выделяется зря).
func TestRenderCtxCanceledBefore(t *testing.T) {
	opt := Options{PageW: 50, PageH: 60, DPI: 100}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if img, err := RenderCtx(ctx, Spread{}, opt); !errors.Is(err, context.Canceled) || img != nil {
		t.Fatalf("ждали context.Canceled без холста, получили %v", err)
	}
	if _, err := RenderCtx(context.Background(), Spread{}, opt); err != nil {
		t.Fatal(err)
	}
	// Порядок: отмена проверяется раньше размера холста — на огромном листе без неё была бы
	// ошибка размера, с ней — Canceled, и ничего не выделялось.
	huge := Options{PageW: 100000, PageH: 100000, DPI: 600}
	if _, err := RenderCtx(context.Background(), Spread{}, huge); err == nil || errors.Is(err, context.Canceled) {
		t.Fatalf("огромный лист без отмены: ждали ошибку размера, получили %v", err)
	}
	if img, err := RenderCtx(ctx, Spread{}, huge); !errors.Is(err, context.Canceled) || img != nil {
		t.Fatalf("огромный лист, отменённый ctx: ждали Canceled, получили %v", err)
	}
}

// TestBorderBufferCancel — буфер повёрнутой рамочки берёт место в бюджете при укладке: отмена
// прерывает и это ожидание, холст не тронут.
func TestBorderBufferCancel(t *testing.T) {
	frame := ringPNG(t, 120, 160, 20, color.NRGBA{20, 140, 40, 255})
	b := BorderDraw{Box: layout.Rect{X: 10, Y: 10, W: 30, H: 20}, Rot: 20, Pic: true, Path: frame,
		Cut: [4]float64{20.0 / 120, 20.0 / 160, 20.0 / 120, 20.0 / 160}}
	canvas := image.NewRGBA(image.Rect(0, 0, 200, 200))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	paint, err := prepareBorder(ctx, b, nil, canvas.Bounds(), 100) // картинка читается при свободном бюджете
	if err != nil || paint == nil {
		t.Fatalf("подготовка: %v", err)
	}
	imgfile.DecodeBudget.Acquire(imgfile.DecodeBudgetPixels)
	held := true
	release := func() {
		if held {
			held = false
			imgfile.DecodeBudget.Release(imgfile.DecodeBudgetPixels)
		}
	}
	defer release()
	got := make(chan error, 1)
	go func() { got <- paint(canvas) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-got:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ждали context.Canceled, получили %v", err)
		}
	case <-time.After(testkit.Limit(2 * time.Second)):
		t.Fatal("отмена не прервала ожидание буфера")
	}
	for _, v := range canvas.Pix {
		if v != 0 {
			t.Fatal("холст тронут после отмены")
		}
	}
	release()
	if used := imgfile.DecodeBudget.Used(); used != 0 {
		t.Fatalf("бюджет не пуст: %d", used)
	}
}
