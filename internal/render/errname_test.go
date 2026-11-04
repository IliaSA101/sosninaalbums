package render

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Текст ошибки по фото несёт только имя файла: путь уходит в log.txt и в тост.
// Причина остаётся в цепочке (errors.Is).
func TestPhotoErrNoDir(t *testing.T) {
	for _, path := range []string{`C:\Users\x\photos\a.jpg`, `/home/x/photos/a.jpg`} {
		pe := &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
		other := errors.New("битый файл " + path)
		for _, in := range []error{pe, other} {
			err := photoErr(path, in)
			got := err.Error()
			if strings.Contains(got, "Users") || strings.Contains(got, "home") || !strings.HasPrefix(got, "a.jpg: ") {
				t.Errorf("%q: текст %q", path, got)
			}
		}
		if err := photoErr(path, pe); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%q: причина потеряна: %v", path, err)
		}
		if err := photoErr(path, other); !errors.Is(err, other) {
			t.Errorf("%q: причина потеряна: %v", path, err)
		}
	}
	if photoErr("a.jpg", nil) != nil {
		t.Error("nil должен остаться nil")
	}
}

// Рендер не пускает каталог в текст ошибки на всех путях: чтение (пути 1 и 3), нет кадра,
// сбой при обработке.
func TestRenderErrorsNoDir(t *testing.T) {
	opt := Options{PageW: 50, PageH: 40, DPI: 100}
	cell := layout.Rect{X: 5, Y: 5, W: 20, H: 20}
	for _, path := range []string{`C:\Users\x\photos\a.jpg`, `/home/x/photos/a.jpg`} {
		rot := coverPhoto(cell, 1.5, path)
		rot.Rot = 15
		noFrame := Photo{Cell: cell, Path: path}
		noFrameRot := noFrame
		noFrameRot.Rot = 15
		for name, ph := range map[string]Photo{"путь 1": coverPhoto(cell, 1.5, path), "путь 3": rot, "нет кадра": noFrame, "нет кадра, наклон": noFrameRot} {
			_, err := Render(Spread{Photos: []Photo{ph}}, opt)
			if err == nil {
				t.Fatalf("%s %q: ждали ошибку", name, path)
			}
			if got := err.Error(); strings.Contains(got, "Users") || strings.Contains(got, "home") || !strings.Contains(got, "a.jpg") {
				t.Errorf("%s %q: текст %q", name, path, got)
			}
			if name == "путь 1" || name == "путь 3" {
				if !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("%s %q: причина потеряна: %v", name, path, err)
				}
			}
		}
	}
}
