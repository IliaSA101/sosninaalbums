package decor

import (
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
)

// Встроенные рамочки (079 design §3): вид генератора decor/gen + цвета и толщина. ID —
// sa:fr-<имя>, чтобы не совпасть с пресетом фона и стикера; в альбом идут вид, цвета и толщина
// (плюс отметка плитки Preset), не ID. Цвета и состав — на проверку Марине.

// framePreset — плитка встроенной рамочки.
type framePreset struct {
	id, theme, name, kind string
	inks                  []string
	width                 float64 // мм
}

func fr(id, theme, name, kind string, width float64, inks ...string) framePreset {
	return framePreset{id: "sa:fr-" + id, theme: theme, name: name, kind: kind, inks: inks, width: width}
}

// framePresets — порядок каталога.
var framePresets = []framePreset{
	fr("white-line", "basic", "Белая линия", "line", 1.5, "#ffffff"),
	fr("black-thin", "basic", "Тонкая чёрная", "line", 0.5, "#222222"),
	fr("white-wide", "grad", "Белая широкая", "line", 4, "#ffffff"),
	fr("gold-double", "wedding", "Двойная золотая", "double", 2.5, "#c8a24a"),
	fr("blue-double", "school", "Двойная синяя", "double", 2, "#1f3a6e"),
	fr("mat", "wedding", "Паспарту", "mat", 8, "#fbf8f1", "#c8a24a"),
	fr("polaroid", "holiday", "Полароид", "polaroid", 5, "#ffffff"),
	fr("stitch", "kids", "Строчка", "stitch", 0.6, "#ffffff"),
	fr("corners", "basic", "Уголки", "corners", 7, "#1f1f1f"),
	fr("gold", "grad", "Золотая рамка", "gold", 4, "#ecd27a", "#8f6b22"),
	fr("scallop", "kindergarten", "Фестоны", "scallop", 4, "#ffffff"),
	fr("soft", "wedding", "Мягкий край", "soft", 8),
}

func findFramePreset(id string) (framePreset, bool) {
	for _, p := range framePresets {
		if p.id == id {
			return p, true
		}
	}
	return framePreset{}, false
}

// item — плитка встроенной: квадратная, рамочка вокруг силуэта (gen.TileQuery).
func (p framePreset) item() Item {
	return Item{ID: p.id, Theme: p.theme, Name: p.name, URL: gen.TileQuery(p.kind, p.inks, p.width), Aspect: 1}
}

// frameBuiltins — плитки встроенных рамочек вида в этой сборке (kind != frame — пусто).
func frameBuiltins(kind string) []Item {
	if kind != KindFrame {
		return nil
	}
	out := make([]Item, 0, len(framePresets))
	for _, p := range framePresets {
		if gen.FrameKnown(p.kind) { // вид без рисовальщика в сборке — нет плитки, как у узоров
			out = append(out, p.item())
		}
	}
	return out
}

// knownFrames — встроенных рамочек в этой сборке (строка «Встроенные» окна «Наборы»).
func knownFrames() int { return len(frameBuiltins(KindFrame)) }
