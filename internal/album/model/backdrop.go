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
	Frame *Frame  `json:"frame,omitempty"` // кадр фото в рамке; nil — S=1, X=Y=0; R — наклон ±44,9 (045, v8)
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

// PhotoIDs — все фото разворота: в местах, на фонах и в стикерах (048). Единая точка для
// rekey и colorReport; «в альбоме» (Photos) фото фона и стикера не считается.
func (s *Spread) PhotoIDs() []string {
	ids := slices.Clone(s.Photos)
	for _, id := range s.decorPhotos() {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// decorPhotos — фото оформления листа: фонов и своих стикеров (с повторами).
func (s *Spread) decorPhotos() []string {
	var ids []string
	for _, b := range s.Backdrops() {
		ids = append(ids, b.Photo)
	}
	for _, st := range s.Stickers {
		if st.Photo != "" {
			ids = append(ids, st.Photo)
		}
	}
	return ids
}

// OnDecor — ID фото, стоящих на фонах и в стикерах любого листа (с обложкой): «в альбоме» они
// не считаются — автосборка и «Заполнить пустые» их не берут.
func (p *Project) OnDecor() map[string]bool {
	out := map[string]bool{}
	for _, s := range p.Sheets() {
		for _, id := range s.decorPhotos() {
			out[id] = true
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

// FitArea — рамка по области, без поворота; кадр сброшен, наклон фото и зеркало остаются
// (045: кнопка ровняет рамку, а не горизонт — как «Сбросить кадр» у фото).
func (b *Backdrop) FitArea(a layout.Rect) {
	b.X, b.Y, b.W, b.H, b.Rot = a.X, a.Y, a.W, a.H, 0
	b.Frame = ClampFrame(&Frame{S: 1, R: b.FrameOrDefault().R})
}
