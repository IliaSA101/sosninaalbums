package looks

import (
	"context"
	"errors"
	"image"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

var errNoStore = errors.New("нет библиотеки")

// fakeHost — Host без App: библиотеки нет, кэша нет, лента из двух фото, вызовы считаются.
func fakeHost(t *testing.T) (Host, *int) {
	t.Helper()
	ed, err := album.NewEditor(model.New(), "")
	if err != nil {
		t.Fatal(err)
	}
	feed := new(int)
	return Host{
		Editor:  func() *album.Editor { return ed },
		Store:   func() (*decor.Store, error) { return nil, errNoStore },
		LibsFor: func(*decor.Store, []string, bool) map[string]model.PhotoInfo { return nil },
		Infos:   func([]string) []model.PhotoInfo { return nil },
		Changed: func(v model.View) model.View { return v },
		CfgDir:  func() string { return "" },
		Photo:   func(string) (library.Photo, bool, bool) { return library.Photo{}, false, false },
		Cache:   func() *library.Cache { return nil },
		Feed: func() []library.Photo {
			*feed++
			return []library.Photo{{ID: "a"}, {ID: "b"}, {ID: "c"}}
		},
		Cancelled: func() {},
		Image: func(model.SpreadTpl, int, float64, map[string]model.PhotoInfo, []model.PhotoInfo) (image.Image, error) {
			return nil, errors.New("не рисуем")
		},
		PNG:  func(image.Image) ([]byte, error) { return nil, errors.New("не кодируем") },
		Keys: func(model.SpreadTpl) []string { return nil },
		FreePick: func(ph []library.Photo, _ map[string]model.MarkView, _, _ []string) ([]string, int) {
			ids := make([]string, len(ph))
			for i, p := range ph {
				ids[i] = p.ID
			}
			return ids, 0
		},
	}, feed
}

// TestNewNeedsHost: пустое поле Host — паника с его именем, а не nil-вызов позже.
func TestNewNeedsHost(t *testing.T) {
	h, _ := fakeHost(t)
	for name, drop := range map[string]func(*Host){
		"Editor":    func(h *Host) { h.Editor = nil },
		"Store":     func(h *Host) { h.Store = nil },
		"LibsFor":   func(h *Host) { h.LibsFor = nil },
		"Infos":     func(h *Host) { h.Infos = nil },
		"Changed":   func(h *Host) { h.Changed = nil },
		"CfgDir":    func(h *Host) { h.CfgDir = nil },
		"Photo":     func(h *Host) { h.Photo = nil },
		"Cache":     func(h *Host) { h.Cache = nil },
		"Feed":      func(h *Host) { h.Feed = nil },
		"Cancelled": func(h *Host) { h.Cancelled = nil },
		"Image":     func(h *Host) { h.Image = nil },
		"PNG":       func(h *Host) { h.PNG = nil },
		"Keys":      func(h *Host) { h.Keys = nil },
		"FreePick":  func(h *Host) { h.FreePick = nil },
	} {
		func() {
			defer func() {
				if r := recover(); r == nil || r.(string) != "looks.New: не задано поле Host."+name {
					t.Errorf("%s: паника %v", name, r)
				}
			}()
			bad := h
			drop(&bad)
			New(bad)
		}()
	}
	if New(h) == nil {
		t.Error("полный Host не принят")
	}
}

// TestNoLibrary: библиотеки нет — карточка, числа, вопросы и сохранение отвечают её ошибкой, а не
// паникой; альбом не трогается.
func TestNoLibrary(t *testing.T) {
	h, _ := fakeHost(t)
	l := New(h)
	if _, err := l.Info("my:0000000000000000"); !errors.Is(err, errNoStore) {
		t.Errorf("Info: %v", err)
	}
	if _, err := l.Count("my:0000000000000000", 3, 10); !errors.Is(err, errNoStore) {
		t.Errorf("Count: %v", err)
	}
	if _, err := l.Plan("my:0000000000000000", model.LookOpts{}); !errors.Is(err, errNoStore) {
		t.Errorf("Plan: %v", err)
	}
	if r, err := l.Apply("my:0000000000000000", model.LookOpts{Mode: "?"}); err == nil || r.Plan.Mode != "" {
		t.Errorf("Apply с неизвестным режимом: %v %+v", err, r)
	}
	if _, err := l.PlanStep("s1", "my:0000000000000000", 1); !errors.Is(err, errNoStore) {
		t.Errorf("PlanStep: %v", err)
	}
	if _, err := l.ApplyStep("s1", "my:0000000000000000", 1); !errors.Is(err, errNoStore) {
		t.Errorf("ApplyStep: %v", err)
	}
	if _, err := l.NewSpread(); !errors.Is(err, album.ErrNoLook) {
		t.Errorf("NewSpread у альбома не по макету: %v", err)
	}
	if lh := l.LookHost(); lh.Find(model.LookRef{}) != nil || lh.Has(model.LookRef{}) || lh.Private("x") {
		t.Error("LookHost без библиотеки что-то нашёл")
	}
	if cur, libs := l.CoverLibs(h.Editor()); cur != nil || libs != nil {
		t.Errorf("CoverLibs без макета: %v %v", cur, libs)
	}
}

// TestServeBounds: чужой путь — не наш (false); без библиотеки, мусорный запрос и не-GET — 404.
func TestServeBounds(t *testing.T) {
	h, _ := fakeHost(t)
	l := New(h)
	w := httptest.NewRecorder()
	if l.Serve(w, httptest.NewRequest(http.MethodGet, "/media/tpl/x", nil)) {
		t.Error("чужой путь принят")
	}
	for _, c := range []struct{ method, url string }{
		{http.MethodGet, "/media/look/0123456789abcdef?i=1&px=256&r=1"},
		{http.MethodGet, "/media/look/0123456789abcdef?i=1"},
		{http.MethodPost, "/media/look/0123456789abcdef?i=1&px=256&r=1"},
	} {
		w := httptest.NewRecorder()
		if !l.Serve(w, httptest.NewRequest(c.method, c.url, nil)) || w.Code != http.StatusNotFound {
			t.Errorf("%s %s: код %d", c.method, c.url, w.Code)
		}
	}
}

// TestCandidatesAndFiles: кандидаты берут ленту ровно один раз и ограничены; без кэша места серые,
// снятый запрос — его ошибка.
func TestCandidatesAndFiles(t *testing.T) {
	h, feed := fakeHost(t)
	l := New(h)
	got := l.Candidates(h.Editor())
	if len(got) != 3 || got[0].ID != "a" || *feed != 1 {
		t.Errorf("кандидаты %v, лента спрошена %d раз", got, *feed)
	}
	if out, err := l.Files(context.Background(), got, 0, 3, false); out != nil || err != nil {
		t.Errorf("без кэша: %v %v", out, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := l.Files(ctx, got, 0, 3, false); !errors.Is(err, context.Canceled) {
		t.Errorf("снятый запрос без кэша: %v", err)
	}
}
