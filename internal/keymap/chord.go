package keymap

import (
	"slices"
	"strconv"
	"strings"
)

// Сочетание — «[Mod+][Alt+][Shift+]<Код>» (design §3). Mod — основной модификатор платформы:
// Ctrl на Windows, ⌘ на Mac (074-20). Код — физическая клавиша (KeyboardEvent.code), буквы латиницей.

// MaxChord — предел длины записи сочетания (config.json правят руками).
const MaxChord = 32

// Порядок модификаторов в каноническом виде.
var mods = []string{"Mod", "Alt", "Shift"}

// codes — белый список кодов клавиш; цифровой блок интерфейс приводит к верхнему ряду.
var codes = func() []string {
	var out []string
	for ch := 'A'; ch <= 'Z'; ch++ {
		out = append(out, "Key"+string(ch))
	}
	for ch := '0'; ch <= '9'; ch++ {
		out = append(out, "Digit"+string(ch))
	}
	for i := 1; i <= 12; i++ {
		out = append(out, "F"+strconv.Itoa(i))
	}
	return append(out,
		"Minus", "Equal", "BracketLeft", "BracketRight", "Backslash", "Semicolon", "Quote",
		"Comma", "Period", "Slash", "Backquote", "IntlBackslash",
		"Space", "Enter", "Tab", "Escape", "Backspace", "Delete", "Insert", "Home", "End",
		"PageUp", "PageDown", "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown")
}()

var codeSet = func() map[string]bool {
	m := make(map[string]bool, len(codes))
	for _, x := range codes {
		m[x] = true
	}
	return m
}()

// Codes — белый список кодов клавиш (для карты клавиатуры и тестов интерфейса).
func Codes() []string { return slices.Clone(codes) }

// Canon — сочетание в каноническом виде: модификаторы по порядку Mod, Alt, Shift, затем код.
// Ложь — не сочетание: пусто, незнакомый код или модификатор, повтор модификатора, длина > MaxChord.
func Canon(s string) (string, bool) {
	if s == "" || len(s) > MaxChord {
		return "", false
	}
	parts := strings.Split(s, "+")
	code := parts[len(parts)-1]
	if !codeSet[code] {
		return "", false
	}
	var have [3]bool
	for _, p := range parts[:len(parts)-1] {
		i := slices.Index(mods, p)
		if i < 0 || have[i] {
			return "", false
		}
		have[i] = true
	}
	var b strings.Builder
	for i, m := range mods {
		if have[i] {
			b.WriteString(m)
			b.WriteByte('+')
		}
	}
	b.WriteString(code)
	return b.String(), true
}

// codeOf — код клавиши канонического сочетания (после модификаторов).
func codeOf(chord string) string { return chord[strings.LastIndexByte(chord, '+')+1:] }

// Reserved — служебная клавиша для замка на карте: Chord с любыми модификаторами (AnyMods),
// любое сочетание, начинающееся с Chord+ (Prefix), или ровно Chord.
type Reserved struct {
	Chord   string `json:"chord"`
	AnyMods bool   `json:"anyMods,omitempty"`
	Prefix  bool   `json:"prefix,omitempty"`
	Reason  string `json:"reason"`
}

const (
	whyArrows = "стрелки листают развороты и двигают выделенное"
	whyModAlt = "Ctrl+Alt на части раскладок Windows — это AltGr (буквы), выберите другое"
	whyWin    = "системная клавиша Windows"
	whyMac    = "системная клавиша macOS"
)

// Служебные клавиши (design §5, 074-6, 074-23). Список общий для обеих систем. Порядок —
// от частного к общему: у Alt+Space причина «системная», а не «Пробел».
var reserved = []Reserved{
	{Chord: "Alt+F4", Reason: whyWin},
	{Chord: "Alt+Tab", Reason: whyWin},
	{Chord: "Alt+Space", Reason: whyWin},
	{Chord: "Alt+Escape", Reason: whyWin},
	{Chord: "Mod+Escape", Reason: whyWin},
	{Chord: "Mod+Alt+Delete", Reason: whyWin},
	{Chord: "Mod+Shift+Escape", Reason: whyWin},
	{Chord: "Mod+Tab", Reason: whyWin},
	{Chord: "Mod+Shift+Tab", Reason: whyWin},
	{Chord: "Mod+KeyQ", Reason: whyMac},
	{Chord: "Mod+KeyH", Reason: whyMac},
	{Chord: "Mod+KeyM", Reason: whyMac},
	{Chord: "Mod+KeyW", Reason: whyMac},
	{Chord: "Mod+Space", Reason: whyMac},
	{Chord: "Mod+Alt+Escape", Reason: whyMac},
	{Chord: "Mod+Backquote", Reason: whyMac},
	{Chord: "Escape", AnyMods: true, Reason: "Esc — отмена и закрытие окон"},
	{Chord: "Tab", AnyMods: true, Reason: "Tab — переход между полями"},
	{Chord: "Space", AnyMods: true, Reason: "Пробел — листание раскладок и перемещение разворота"},
	{Chord: "F1", AnyMods: true, Reason: "F1 — справочник функций"},
	{Chord: "Mod+Alt", Prefix: true, Reason: whyModAlt},
	{Chord: "ArrowLeft", Reason: whyArrows},
	{Chord: "ArrowRight", Reason: whyArrows},
	{Chord: "ArrowUp", Reason: whyArrows},
	{Chord: "ArrowDown", Reason: whyArrows},
	{Chord: "Shift+ArrowLeft", Reason: whyArrows},
	{Chord: "Shift+ArrowRight", Reason: whyArrows},
	{Chord: "Shift+ArrowUp", Reason: whyArrows},
	{Chord: "Shift+ArrowDown", Reason: whyArrows},
}

// Locked — служебные клавиши по порядку проверки.
func Locked() []Reserved { return slices.Clone(reserved) }

// ReservedReason — почему сочетание нельзя назначить; "" — можно (или это не сочетание).
func ReservedReason(chord string) string {
	cc, ok := Canon(chord)
	if !ok {
		return ""
	}
	code := codeOf(cc)
	for _, r := range reserved {
		switch {
		case r.AnyMods && code == r.Chord,
			r.Prefix && strings.HasPrefix(cc, r.Chord+"+"),
			!r.AnyMods && !r.Prefix && cc == r.Chord:
			return r.Reason
		}
	}
	return ""
}
