package model

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Backdrop — фото на фоне. Область (разворот или страница) его обрезает: рамку
// можно вынести за край.
type Backdrop struct {
	Photo string  `json:"photo"` // ID в Project.Photos
	X     float64 `json:"x"`     // рамка, мм разворота: левый верхний угол до поворота
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	// Rot — градусы по часовой вокруг (X, Y), (−180, 180], шаг 0.1.
	Rot   float64 `json:"rot,omitempty"`
	Frame *Frame  `json:"frame,omitempty"` // кадр фото в рамке; nil — S=1, X=Y=0
	FlipH bool    `json:"flipH,omitempty"` // зеркало фото внутри рамки
	FlipV bool    `json:"flipV,omitempty"`
	// Fade — прозрачность фото 0..1 (0 — непрозрачно, 1 — не видно), шаг 0.01 (спека 017).
	// Хранится прозрачность, а не непрозрачность: ноль по умолчанию не пишется.
	Fade float64 `json:"fade,omitempty"`
}

// bdScopes — области фона по порядку обхода.
var bdScopes = []string{ScopeSpread, ScopeLeft, ScopeRight}

// BdScopes — копия bdScopes: снаружи порядок обхода не подменить.
func BdScopes() []string { return slices.Clone(bdScopes) }

// ValidScope — область фона: spread | left | right.
func ValidScope(scope string) bool {
	return scope == ScopeSpread || scope == ScopeLeft || scope == ScopeRight
}

// Slot — где лежит фон области; nil — область неизвестна.
func (s *Spread) Slot(scope string) **Backdrop {
	switch scope {
	case ScopeSpread:
		return &s.BgPhoto
	case ScopeLeft:
		return &s.BgPhotoL
	case ScopeRight:
		return &s.BgPhotoR
	}
	return nil
}

// Backdrops — фоны разворота по порядку: весь разворот, левая, правая.
func (s *Spread) Backdrops() []*Backdrop {
	var out []*Backdrop
	for _, b := range []*Backdrop{s.BgPhoto, s.BgPhotoL, s.BgPhotoR} {
		if b != nil {
			out = append(out, b)
		}
	}
	return out
}

// PhotoIDs — все фото разворота: в местах и на фонах. Единая точка для pruneRefs,
// rekey и colorReport; «в альбоме» (Photos) фото фона не считается.
func (s *Spread) PhotoIDs() []string {
	ids := slices.Clone(s.Photos)
	for _, b := range s.Backdrops() {
		if !slices.Contains(ids, b.Photo) {
			ids = append(ids, b.Photo)
		}
	}
	return ids
}

// OnBackdrops — ID фото, стоящих на фонах любого листа (с обложкой).
func (p *Project) OnBackdrops() map[string]bool {
	out := map[string]bool{}
	for _, s := range p.Sheets() {
		for _, b := range s.Backdrops() {
			out[b.Photo] = true
		}
	}
	return out
}

func (b *Backdrop) TBox() geom.TextBox {
	return geom.TextBox{X: b.X, Y: b.Y, W: b.W, H: b.H, Rot: b.Rot}
}

// FrameOrDefault — действующий кадр; nil — без увеличения и сдвига.
func (b *Backdrop) FrameOrDefault() Frame {
	if b.Frame == nil {
		return Frame{S: 1}
	}
	return *b.Frame
}

// Clone — копия фона (Frame не общий).
func (b *Backdrop) Clone() *Backdrop {
	if b == nil {
		return nil
	}
	c := *b
	if b.Frame != nil {
		f := *b.Frame
		c.Frame = &f
	}
	return &c
}

// FitArea — рамка по области, без поворота и кадра; зеркало остаётся.
func (b *Backdrop) FitArea(a layout.Rect) {
	b.X, b.Y, b.W, b.H, b.Rot, b.Frame = a.X, a.Y, a.W, a.H, 0, nil
}
