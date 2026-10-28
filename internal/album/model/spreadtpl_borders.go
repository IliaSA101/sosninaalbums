package model

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
)

// Рамочки мест и заготовки в шаблоне разворота (спека 131, design §1, решение 0.27-131-2):
// списки при V = 1 — 0.27 их не видит (ClampSpreadTpl собирает копию с нуля), ставит шаблон
// без рамочек и без заготовок, ключ шаблонов без них прежний.

// TplBorder — рамочка места I шаблона: встроенная (Kind, Inks, Width, Preset) или картинка
// набора вида frame (Lib, Scale, Cut, Open) — ровно одно из Kind и Lib.
type TplBorder struct {
	I     int      `json:"i"` // индекс в Places
	Kind  string   `json:"kind,omitempty"`
	Inks  []string `json:"inks,omitempty"`
	Width float64  `json:"w,omitempty"`
	// Preset — плитка sa:<…>, поставившая рамочку: свой снимок разворота на тот же разворот
	// иначе теряет отметку и правка не холостая.
	Preset string     `json:"preset,omitempty"`
	Lib    string     `json:"lib,omitempty"`
	Scale  float64    `json:"scale,omitempty"`
	Cut    [4]float64 `json:"cut,omitzero"`
	Open   bool       `json:"open,omitempty"`
}

// tplPlaces — места шаблона до зажима и карта at: индекс во входе → индекс после зажима (−1 —
// выброшено: не число или сверх MaxPlaces). Рамочки привязаны к индексу, он не должен
// съезжать, когда ClampBoxes выбрасывает место.
func tplPlaces(in []Box) (out []Box, at []int) {
	at = make([]int, len(in))
	for i, b := range in {
		at[i] = -1
		if len(out) < MaxPlaces && finiteBox(b) {
			at[i] = len(out)
			out = append(out, b)
		}
	}
	return out, at
}

// clampTplBorders — рамочки к допустимым: I в пределах мест (at — карта tplPlaces, n — мест
// после зажима), на одно место одна запись — первая из файла, даже испорченная; записей не
// больше мест; порядок по I.
func clampTplBorders(in []TplBorder, at []int, n int) []TplBorder {
	var out []TplBorder
	seen := make([]bool, n)
	for _, b := range in {
		if b.I < 0 || b.I >= len(at) || at[b.I] < 0 || at[b.I] >= n || seen[at[b.I]] {
			continue
		}
		i := at[b.I]
		seen[i] = true
		if c, ok := clampTplBorder(b); ok {
			c.I = i
			out = append(out, c)
		}
	}
	slices.SortFunc(out, func(a, b TplBorder) int { return a.I - b.I })
	return out
}

// clampTplBorder — одна рамочка: ровно одно из Kind и Lib; встроенная — вид по грамматике,
// цвета, толщина и Preset как у Border (gen.ClampFrameInks/ClampFrameWidth), картинка — ключ
// библиотеки, размер, срез. Поля чужого вида снимаются. Идемпотентна.
func clampTplBorder(b TplBorder) (TplBorder, bool) {
	if (b.Kind == "") == (b.Lib == "") {
		return TplBorder{}, false
	}
	if b.Lib != "" {
		if !ValidLibKey(b.Lib) {
			return TplBorder{}, false
		}
		return TplBorder{Lib: b.Lib, Scale: clampBorderScale(b.Scale), Cut: gen.ClampCut(b.Cut), Open: b.Open}, true
	}
	if !gen.ValidKind(b.Kind) {
		return TplBorder{}, false
	}
	out := TplBorder{Kind: b.Kind, Inks: gen.ClampFrameInks(b.Kind, b.Inks), Width: gen.ClampFrameWidth(b.Width)}
	if ValidPreset(b.Preset) {
		out.Preset = b.Preset
	}
	return out, true
}

// clampTplTexts — надписи шаблона не больше limit штук, негодные отброшены.
func clampTplTexts(in []TplText, limit int) []TplText {
	var out []TplText
	for _, tx := range in {
		if len(out) >= limit {
			break
		}
		if ct, ok := clampTplText(tx); ok {
			out = append(out, ct)
		}
	}
	return out
}
