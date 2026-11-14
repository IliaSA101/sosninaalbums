package facefit

import (
	"context"
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// fakeHost — Host без App: пустой альбом, фото нет, события и правки считаются.
func fakeHost(t *testing.T, exportBusy bool) (Host, *int) {
	t.Helper()
	ed, err := album.NewEditor(model.New(), "")
	if err != nil {
		t.Fatal(err)
	}
	changed := new(int)
	return Host{
		Editor: func() *album.Editor { return ed },
		Photo:  func(string) (library.Photo, bool, bool, bool) { return library.Photo{}, false, false, false },
		Copy:   func(context.Context, string, library.Photo, bool) (string, error) { return "", errors.New("нет") },
		Ctx:    context.Background,
		Go: func(fn func()) bool {
			fn()
			return true
		},
		ExportBusy: func() bool { return exportBusy },
		Emit:       func(string, ...any) {},
		Changed:    func(v model.View) model.View { *changed++; return v },
	}, changed
}

// TestNewNeedsHost: пустое обязательное поле Host — паника с его именем, а не nil-вызов позже.
func TestNewNeedsHost(t *testing.T) {
	h, _ := fakeHost(t, false)
	for name, drop := range map[string]func(*Host){
		"Editor":     func(h *Host) { h.Editor = nil },
		"Photo":      func(h *Host) { h.Photo = nil },
		"Copy":       func(h *Host) { h.Copy = nil },
		"Ctx":        func(h *Host) { h.Ctx = nil },
		"Go":         func(h *Host) { h.Go = nil },
		"ExportBusy": func(h *Host) { h.ExportBusy = nil },
		"Emit":       func(h *Host) { h.Emit = nil },
		"Changed":    func(h *Host) { h.Changed = nil },
	} {
		func() {
			defer func() {
				if r := recover(); r == nil || r.(string) != "facefit.New: не задано поле Host."+name {
					t.Errorf("%s: паника %v", name, r)
				}
			}()
			bad := h
			drop(&bad)
			New(bad)
		}()
	}
	New(h) // Rare не обязателен
}

// TestFitRefusals: отказы до поиска — режим, экспорт, модель, нет листа; правки нет, занятость
// снята (следующий вызов не получает ErrBusy).
func TestFitRefusals(t *testing.T) {
	h, changed := fakeHost(t, false)
	f := New(h)
	if _, _, err := f.Fit("s", "", "всё"); !errors.Is(err, ErrMode) {
		t.Fatalf("режим: %v", err)
	}
	if _, _, err := f.Fit("нет-такого", "", ""); !errors.Is(err, ErrSheet) {
		t.Fatalf("нет листа: %v", err)
	}
	f.Ready = func() error { return errors.New("модель сломана") }
	if _, _, err := f.Fit("s", "", ""); err == nil || errors.Is(err, ErrSheet) || f.Busy() {
		t.Fatalf("сломанная модель: %v, busy=%v", err, f.Busy())
	}
	exp, _ := fakeHost(t, true)
	if _, _, err := New(exp).Fit("s", "", ""); !errors.Is(err, ErrExport) {
		t.Fatalf("экспорт: %v", err)
	}
	if *changed != 0 {
		t.Fatalf("отказы вызвали Changed %d раз", *changed)
	}
}

// TestFitBusy: поиск идёт — второй вызов получает ErrBusy, а не встаёт в очередь.
func TestFitBusy(t *testing.T) {
	f := New(func() Host { h, _ := fakeHost(t, false); return h }())
	f.busy.Store(true)
	if _, _, err := f.Fit("s", "", ""); !errors.Is(err, ErrBusy) {
		t.Fatalf("занято: %v", err)
	}
	if !f.Busy() {
		t.Fatal("чужая занятость снята")
	}
}

// TestMemoryOwner: память принадлежит альбому — у другого редактора она начинается заново, а
// NormOf чужого не трогает.
func TestMemoryOwner(t *testing.T) {
	h, _ := fakeHost(t, false)
	f := New(h)
	a, b := h.Editor(), func() *album.Editor { e, _ := album.NewEditor(model.New(), ""); return e }()
	f.remember(a, "s", map[string]model.Frame{"p": {}}, true, geom.FaceNorm{Size: 1})
	if n, ok := f.SavedNorm("s"); !ok || n.Size != 1 || f.FittedLen() != 1 {
		t.Fatalf("память: %+v %v, кадров %d", n, ok, f.FittedLen())
	}
	if f.NormOf(b, "s") != nil {
		t.Fatal("норма чужого альбома")
	}
	if f.NormOf(a, "s") == nil {
		t.Fatal("нормы своего альбома нет")
	}
	if norm, fitted := f.Memory(b, "s"); norm != nil || len(fitted) != 0 {
		t.Fatalf("новый альбом начал не с пустой памяти: %v %v", norm, fitted)
	}
}
