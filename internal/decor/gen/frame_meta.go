package gen

import "math"

// Встроенные рамочки мест (079 design §3): описание видов. Окно фото (Ins × толщина) считает
// модель без рисования, поэтому таблица — здесь, отдельно от генераторов. Выпущенный вид не
// меняется: толщина по умолчанию и Ins держат геометрию альбомов с Width = 0 (079-28); новый
// вид с окном (Ins ≠ 0) — только с новой версией формата.

// FrameMeta — вид встроенной рамочки для каталога, карточки и окна фото.
type FrameMeta struct {
	Kind, Name string
	Inks       []string   // цвета по умолчанию; len — сколько пикеров (0–2)
	Width      float64    // толщина по умолчанию, мм
	Ins        [4]float64 // отступ окна в толщинах: l, t, r, b (0 — накладка поверх фото)
	Mask       bool       // вид меняет маску фото (мягкий край), а не рисует поверх
}

// Пределы рамочки (design §1).
const (
	MaxFrameInks   = 2
	MinFrameWidth  = 0.2 // мм
	MaxFrameWidth  = 20.0
	frameWidthPrec = 10 // шаг 0,1 мм делителем: без хвоста двоичной дроби
)

var (
	insEven  = [4]float64{1, 1, 1, 1}
	insWide  = [4]float64{1, 1, 1, 3.5} // полароид: низ шире
	insNone  = [4]float64{}
	frameTab = []FrameMeta{
		{Kind: "line", Name: "Линия", Inks: []string{"#ffffff"}, Width: 1.5, Ins: insEven},
		{Kind: "double", Name: "Двойная линия", Inks: []string{"#c8a24a"}, Width: 3, Ins: insEven},
		{Kind: "mat", Name: "Паспарту", Inks: []string{"#fbf8f1", "#c8a24a"}, Width: 8, Ins: insEven},
		{Kind: "polaroid", Name: "Полароид", Inks: []string{"#ffffff"}, Width: 5, Ins: insWide},
		{Kind: "stitch", Name: "Строчка", Inks: []string{"#ffffff"}, Width: 0.6, Ins: insNone},
		{Kind: "corners", Name: "Уголки", Inks: []string{"#1f1f1f"}, Width: 7, Ins: insNone},
		{Kind: "gold", Name: "Золотая рамка", Inks: []string{"#ecd27a", "#8f6b22"}, Width: 4, Ins: insEven},
		{Kind: "scallop", Name: "Фестоны", Inks: []string{"#ffffff"}, Width: 4, Ins: insEven},
		{Kind: "soft", Name: "Мягкий край", Width: 8, Ins: insNone, Mask: true},
	}
)

// Frames — виды встроенных рамочек в порядке каталога (копии: правка снаружи таблицу не трогает).
func Frames() []FrameMeta {
	out := make([]FrameMeta, len(frameTab))
	for i, m := range frameTab {
		m.Inks = append([]string(nil), m.Inks...)
		out[i] = m
	}
	return out
}

// FrameLookup — вид рамочки по ключу (копия); ok = false — вида нет в этой сборке.
func FrameLookup(kind string) (FrameMeta, bool) {
	for _, m := range frameTab {
		if m.Kind == kind {
			m.Inks = append([]string(nil), m.Inks...)
			return m, true
		}
	}
	return FrameMeta{}, false
}

// FrameKnown — вид есть в этой сборке; правильный, но неизвестный ключ (файл из новой версии)
// модель хранит, окно у него — всё место.
func FrameKnown(kind string) bool {
	_, ok := FrameLookup(kind)
	return ok
}

// ClampFrameInks — цвета рамочки: #rrggbb нижним регистром, не больше, чем у вида (неизвестного —
// MaxFrameInks); плохой — цвет вида на его месте (у неизвестного — конец списка). Пусто — nil.
func ClampFrameInks(kind string, inks []string) []string {
	m, known := FrameLookup(kind)
	limit := MaxFrameInks
	if known {
		limit = len(m.Inks)
	}
	var out []string
	for i, v := range inks {
		if i >= limit {
			break
		}
		c, ok := normInk(v)
		if !ok {
			if !known {
				break
			}
			c = m.Inks[i]
		}
		out = append(out, c)
	}
	return out
}

// ClampFrameWidth — толщина, мм: NaN, Inf и ≤ 0 — 0 (по умолчанию вида), иначе
// MinFrameWidth…MaxFrameWidth с шагом 0,1.
func ClampFrameWidth(w float64) float64 {
	if math.IsNaN(w) || math.IsInf(w, 0) || w <= 0 {
		return 0
	}
	return math.Round(min(max(w, MinFrameWidth), MaxFrameWidth)*frameWidthPrec) / frameWidthPrec
}
