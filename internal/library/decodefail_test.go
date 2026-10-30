package library

import (
	"context"
	"errors"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// cutJPEG — JPEG w×h, обрезанный до keep байт (заголовок цел).
func cutJPEG(t *testing.T, dir, name string, w, h, keep int) string {
	t.Helper()
	whole := testkit.JPEG(t, dir, "whole-"+name, w, h)
	data, err := os.ReadFile(whole)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(whole)
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data[:keep], 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// Усечённый JPEG: DecodeError с причиной «файл повреждён»; много запросов сразу (и превью, и
// миниатюра) декодируют файл один раз; после перезаписи ключ другой — файл читается.
func TestDecodeFailCachedOnce(t *testing.T) {
	dir := testkit.TempDir(t)
	c, n := tailCache(t, dir, 4, 2)
	p1 := cutJPEG(t, dir, "a.jpg", 400, 300, 900)
	p2 := cutJPEG(t, dir, "b.jpg", 300, 200, 700)
	good := testkit.JPEG(t, dir, "good.jpg", 40, 30)
	photos := []Photo{{ID: IDFor(p1), Path: p1, Name: "a.jpg"}, {ID: IDFor(p2), Path: p2, Name: "b.jpg"}}

	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := range 20 {
		wg.Go(func() {
			for _, p := range photos {
				_, err := c.File(p, i%2 == 0)
				errs <- err
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		var de *DecodeError
		if !errors.As(err, &de) || !strings.HasPrefix(de.Why, "файл повреждён: ") || len(de.Why) > whyMax+len("…") {
			t.Fatalf("ошибка: %v, ждали DecodeError «файл повреждён»", err)
		}
	}
	if got := n.Load(); got != 2 {
		t.Fatalf("декодов оригинала %d, ждали 2 (по одному на файл)", got)
	}
	if _, err := c.File(photos[0], false); err == nil || n.Load() != 2 {
		t.Fatalf("повторный запрос: ошибка %v, декодов %d", err, n.Load())
	}
	if _, err := c.File(Photo{ID: IDFor(good), Path: good, Name: "good.jpg"}, false); err != nil {
		t.Fatalf("целый файл: %v", err)
	}

	// Перезаписан целым: другие размер и время — другой ключ, неудача не залипает.
	testkit.JPEG(t, dir, "a.jpg", 50, 40)
	if _, err := c.File(photos[0], false); err != nil {
		t.Fatalf("после перезаписи: %v", err)
	}
}

// Не порча содержимого — не DecodeError и не запоминается: пропавший файл, отмена, сбой диска,
// подмена во время чтения.
func TestDecodeFailOnlyContent(t *testing.T) {
	dir := testkit.TempDir(t)
	good := testkit.JPEG(t, dir, "g.jpg", 40, 30)
	for name, err := range map[string]error{
		"нет файла": &fs.PathError{Op: "open", Path: good, Err: fs.ErrNotExist},
		"отмена":    context.Canceled,
		"диск":      &fs.PathError{Op: "read", Path: good, Err: errors.New("i/o error")},
		"подмена":   imgfile.ErrChanged,
	} {
		if got := decodeFail(good, err); got != err {
			t.Errorf("%s: %v, ждали ту же ошибку", name, got)
		}
	}
	// Сбой декодера на настоящем файле — DecodeError; текст ошибки декодера режется по пределу.
	long := errors.New(strings.Repeat("ж", 5000))
	var de *DecodeError
	if !errors.As(decodeFail(good, long), &de) || len(de.Why) > whyMax+len("…") || !strings.Contains(de.Why, "ж") {
		t.Fatalf("длинная причина: %v", de)
	}
	if !strings.HasSuffix(de.Why, "…") || strings.ContainsRune(de.Why, '�') {
		t.Errorf("обрезка не по руне: %q", de.Why[len(de.Why)-8:])
	}
	// Файл пропал, пока строили причину: причина по самой ошибке, без паники.
	gone := filepath.Join(dir, "gone.jpg")
	if !errors.As(decodeFail(gone, errors.New("unexpected EOF")), &de) || !strings.Contains(de.Why, "unexpected EOF") {
		t.Errorf("пропавший файл: %v", de)
	}
}

// Ошибка открытия оригинала не про порчу повторяется (не запоминается).
func TestDecodeFailTransientNotCached(t *testing.T) {
	dir := testkit.TempDir(t)
	c, n := tailCache(t, dir, 2, 1)
	good := testkit.JPEG(t, dir, "g.jpg", 40, 30)
	c.open = func(string) (image.Image, func(), error) {
		n.Add(1)
		return nil, nil, imgfile.ErrChanged
	}
	p := Photo{ID: IDFor(good), Path: good, Name: "g.jpg"}
	for range 3 {
		if _, err := c.File(p, false); !errors.Is(err, imgfile.ErrChanged) {
			t.Fatalf("ошибка %v", err)
		}
	}
	if n.Load() != 3 {
		t.Fatalf("декодов %d, ждали 3: временная ошибка не должна запоминаться", n.Load())
	}
}
