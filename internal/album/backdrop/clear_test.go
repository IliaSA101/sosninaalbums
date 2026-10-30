package backdrop

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// ClearBackdrop зовут и мимо обёртки Editor: неизвестная область (Slot == nil) — холостая правка,
// а не паника; известная — снимает фон только своей области.
func TestClearBackdropUnknownScope(t *testing.T) {
	p := model.New()
	s := &model.Spread{ID: "s1", BgPhotoL: &model.Backdrop{Photo: "l"}, BgPhotoR: &model.Backdrop{Photo: "r"}}
	p.Spreads = []*model.Spread{s}

	ClearBackdrop(p, "s1", "top")
	ClearBackdrop(p, "s1", "")
	ClearBackdrop(p, "нет", model.ScopeLeft)
	if s.BgPhotoL == nil || s.BgPhotoR == nil {
		t.Fatalf("холостой вызов снял фон: L=%v R=%v", s.BgPhotoL, s.BgPhotoR)
	}

	ClearBackdrop(p, "s1", model.ScopeLeft)
	if s.BgPhotoL != nil || s.BgPhotoR == nil {
		t.Fatalf("снят не тот фон: L=%v R=%v", s.BgPhotoL, s.BgPhotoR)
	}
}

// SetBackdrop мимо обёртки: неизвестная область — ErrScope, фото не запоминается, фоны целы.
func TestSetBackdropUnknownScope(t *testing.T) {
	p := model.New()
	s := &model.Spread{ID: "s1", BgPhotoL: &model.Backdrop{Photo: "l"}}
	p.Spreads = []*model.Spread{s}
	ph := model.PhotoInfo{ID: "x", Path: "x.jpg", W: 300, H: 200}

	for _, scope := range []string{"top", ""} {
		if err := SetBackdrop(p, "s1", scope, ph); !errors.Is(err, model.ErrScope) {
			t.Fatalf("область %q: ошибка %v, нужна ErrScope", scope, err)
		}
	}
	if _, ok := p.Photos["x"]; ok || s.BgPhotoL == nil || s.BgPhotoL.Photo != "l" || s.BgPhoto != nil || s.BgPhotoR != nil {
		t.Fatalf("холостой вызов изменил проект: photos=%v L=%v R=%v S=%v", p.Photos, s.BgPhotoL, s.BgPhotoR, s.BgPhoto)
	}

	if err := SetBackdrop(p, "s1", model.ScopeRight, ph); err != nil || s.BgPhotoR == nil || s.BgPhotoR.Photo != "x" {
		t.Fatalf("известная область: err=%v R=%v", err, s.BgPhotoR)
	}
}
