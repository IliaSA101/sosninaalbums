package looktpl

import (
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Оформление шага-виньетки макета (164 design §1): гарнитура и цвет шапки и подписей, подпись
// учителя, текст шапки. Кегль программа считает сама по формату листа, поэтому у стиля из файла
// он 0 («не задан»), а Align, Bold и Italic не читаются: стиль встаёт поверх стиля по умолчанию.

// MaxLookHeadLines — строк в htext: шапка листа-виньетки — две-три строки.
const MaxLookHeadLines = 3

// clampLookVigStyle — Style шага из файла: у Caps и Head остаются Font и Color, форма мест
// (shape, round) и поля подписей (pad, over, wrap, text) обнуляются, TText — как у своего шаблона
// 038. Пусто после зажима — nil (поведение 0.35).
func clampLookVigStyle(in *model.VigStyle) *model.VigStyle {
	if in == nil {
		return nil
	}
	var out model.VigStyle
	if in.Caps != nil {
		if c := lookFontColor(in.Caps.Style); c != (model.TextStyle{}) {
			out.Caps = &model.SheetCaps{Style: c}
		}
	}
	if in.Head != nil {
		if h := lookFontColor(*in.Head); h != (model.TextStyle{}) {
			out.Head = &h
		}
	}
	if tt := model.CleanText(in.TText, model.MaxCaptionRunes); strings.TrimSpace(tt) != "" {
		out.TText = tt
	}
	if out.Caps == nil && out.Head == nil && out.TText == "" {
		return nil
	}
	return &out
}

// lookFontColor — стиль только с гарнитурой и цветом. Пустое поле — «не менять»; неизвестная
// гарнитура и своя «my:…» (на чужом ПК её нет) — умолчание; плохой цвет — «не менять».
func lookFontColor(st model.TextStyle) model.TextStyle {
	var out model.TextStyle
	if st.Font != "" {
		out.Font = model.ClampStyle(model.TextStyle{Font: st.Font}).Font
		if text.IsCustomID(out.Font) {
			out.Font = text.DefaultFont
		}
	}
	out.Color = model.BgColor(st.Color)
	return out
}

// cleanLookHead — htext: CleanText до MaxCaptionRunes знаков и MaxLookHeadLines строк; из одних
// пробелов — "".
func cleanLookHead(s string) string {
	s = model.CleanText(s, model.MaxCaptionRunes)
	if lines := strings.SplitN(s, "\n", MaxLookHeadLines+1); len(lines) > MaxLookHeadLines {
		s = strings.Join(lines[:MaxLookHeadLines], "\n")
	}
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return s
}
