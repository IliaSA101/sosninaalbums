package printprof

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Кириллические адреса (123 Б3, решение 0.26-123-52): окно «Своя типография» переводит хост в
// xn--… (App, x/net/idna), файл хранит и разбирает только ASCII (asciiHost). Здесь — то, что нужно
// и разбору, и окну без x/net: безопасный показ хоста человеку (белый список, 0.26-123-56), канон
// метки xn--… и проверка метки на двойников.

// cyrLookalike — кириллические буквы, неотличимые от латинских в строчном начертании: метка только
// из них (и цифр с «-») читается как латинское слово («аррӏе» — apple, «рорсогп» — popcorn; список
// шире — по Chromium: г ≈ r, п ≈ n, ъ ≈ b).
const cyrLookalike = "асеорхуіјѕӏԁԛԝһгпъ"

// LabelProblem — почему метка хоста (часть между точками, уже в Юникоде) не годится для адреса:
// смесь алфавитов, буквы вне кириллицы, латиницы и греческого, знак вне букв, цифр и «-» (в том
// числе комбинирующий), метка целиком из кириллических двойников латиницы. "" — метка годна.
// Метка из ASCII годна всегда.
func LabelProblem(label string) string {
	var cyr, lat, grk, other, lookalike int
	for _, r := range label {
		switch {
		case r == '-' || r >= '0' && r <= '9':
		case !unicode.IsLetter(r): // и комбинирующие знаки (Mn, Me): накладки и селекторы прячут подмену
			return "знак, которого не должно быть в адресе (только буквы, цифры и «-»)"
		case unicode.Is(unicode.Cyrillic, r):
			cyr++
			if strings.ContainsRune(cyrLookalike, r) {
				lookalike++
			}
		case unicode.Is(unicode.Latin, r):
			lat++
		case unicode.Is(unicode.Greek, r):
			grk++
		default:
			other++
		}
	}
	switch {
	case other > 0:
		return "буквы только кириллицы, латиницы и греческого"
	case btoi(cyr > 0)+btoi(lat > 0)+btoi(grk > 0) > 1:
		return "в одной части адреса (между точками) буквы разных алфавитов — кириллица и латиница, например, выглядят одинаково"
	case cyr > 0 && lookalike == cyr:
		return "эта часть адреса из кириллических букв, неотличимых от латиницы"
	}
	return ""
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// DisplayHost — хост для человека: метки xn--… раскрываются в Юникод, только если весь хост
// показывается целиком (все метки xn--… канонические — xnLabel, каждая метка проходит showLabel:
// белый список кириллицы, решение 0.26-123-56) и он не двойник сайта встроенной типографии
// (likeBuiltin). Иначе хост остаётся как есть в нижнем регистре: двойник нельзя принять за
// настоящий сайт, а наполовину раскрытый хост — за другой.
func DisplayHost(host string) string {
	host = strings.ToLower(host)
	if !strings.Contains(host, "xn--") || len(host) > 253 {
		return host
	}
	labels := strings.Split(host, ".")
	for i, l := range labels {
		if len(l) > 63 {
			return host
		}
		if rest, ok := strings.CutPrefix(l, "xn--"); ok {
			if l, ok = xnLabel(rest); !ok {
				return host
			}
			labels[i] = l
		}
		if !showLabel(labels[i]) {
			return host
		}
	}
	shown := strings.Join(labels, ".")
	if likeBuiltin(shown) {
		return host
	}
	return shown
}

// xnLabel — Юникод метки xn--… (rest — без префикса, в нижнем регистре), если она каноническая:
// декодируется в буквы, цифры и «-», содержит не-ASCII и кодируется обратно в ту же строку.
// «xn---80aac2ak8aza» (лишний «-») и «xn--google-» (одна латиница) раскрылись бы в «фабрика» и
// «google» — чужой хост под видом настоящего; разбор файла такие метки тоже отвергает (asciiHost).
func xnLabel(rest string) (string, bool) {
	r, ok := punyDecode(rest)
	bad := func(c rune) bool { return c != '-' && !unicode.IsLetter(c) && !unicode.IsDigit(c) }
	if !ok || !slices.ContainsFunc(r, func(c rune) bool { return c >= utf8.RuneSelf }) || slices.ContainsFunc(r, bad) || punyEncode(r) != rest {
		return "", false
	}
	return string(r), true
}

// showRune — белый список показа (SEC-1, 0.26-123-56): строчные латиница, цифры и «-», кириллица
// русского, украинского и белорусского алфавитов. Латиница с диакритикой, греческий, малые
// капительные, «ı» и прочие двойники остаются xn--…: чёрный список их не перечислит.
func showRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-':
		return true
	case r >= 'а' && r <= 'я':
		return true
	}
	return strings.ContainsRune("ёєіїўґ", r)
}

// showLabel — Юникод метки можно показать: все знаки из белого списка, дефисы как в DNS (не в начале
// и конце, не «--» на 3–4 месте) и нет смеси алфавитов и метки из одних двойников (LabelProblem).
func showLabel(u string) bool {
	r := []rune(u)
	if len(r) == 0 || r[0] == '-' || r[len(r)-1] == '-' || len(r) >= 4 && r[2] == '-' && r[3] == '-' {
		return false
	}
	return !slices.ContainsFunc(r, func(c rune) bool { return !showRune(c) }) && LabelProblem(u) == ""
}

// likeBuiltin — показанный хост (без «www.») по скелету NameKey совпадает с сайтом встроенной
// типографии, но им не является: «кот.com» под видом kot.com. Встроенные — проверенные адреса,
// их двойник показывается как xn--….
func likeBuiltin(shown string) bool {
	shown = strings.TrimPrefix(shown, "www.")
	key := NameKey(shown)
	for _, p := range Builtin().Profiles() {
		for _, h := range p.siteHosts() {
			if h != shown && NameKey(h) == key {
				return true
			}
		}
	}
	return false
}

// siteHosts — хосты адресов профиля (export.url и изделий) для likeBuiltin, без «www.».
func (p Profile) siteHosts() []string {
	var out []string
	if p.Export != nil && p.Export.URL != "" {
		out = append(out, siteHost(p.Export.URL))
	}
	for _, pr := range p.Products {
		if pr.URL != "" {
			out = append(out, siteHost(pr.URL))
		}
	}
	return out
}

// punyMax — предел накопителей (int64): сумма, произведения на ≤ 35 и n + i/count не переполняются.
const punyMax = 1 << 40

// punyDecode — декодер Punycode (RFC 3492) метки без «xn--». Вход короче 64 знаков (метка DNS),
// все счётчики проверены на переполнение; ok=false — битая строка.
func punyDecode(in string) ([]rune, bool) {
	const (
		base, tmin, tmax, skew, damp = 36, 1, 26, 38, 700
		maxRunes                     = 63
	)
	if len(in) > 63 {
		return nil, false
	}
	var out []rune
	if i := strings.LastIndexByte(in, '-'); i == 0 {
		return nil, false // разделитель без основных знаков: в каноническом виде его нет (RFC 3492 §6.3)
	} else if i > 0 {
		for j := 0; j < i; j++ {
			if in[j] >= utf8.RuneSelf {
				return nil, false
			}
			out = append(out, rune(in[j]))
		}
		in = in[i+1:]
	}
	var n, i, bias int64 = 128, 0, 72
	for pos := 0; pos < len(in); {
		oldi, w := i, int64(1)
		for k := int64(base); ; k += base {
			if pos >= len(in) {
				return nil, false
			}
			d := int64(punyDigit(in[pos]))
			pos++
			if d < 0 || d > (punyMax-i)/w {
				return nil, false
			}
			i += d * w
			t := min(max(k-bias, tmin), tmax)
			if d < t {
				break
			}
			w *= base - t // до умножения w ≤ i ≤ punyMax: произведение влезает в int64
		}
		count := int64(len(out) + 1)
		bias = punyAdapt(i-oldi, count, oldi == 0, damp, skew)
		n += i / count
		i %= count
		if n > unicode.MaxRune || n >= 0xD800 && n <= 0xDFFF || len(out) >= maxRunes {
			return nil, false
		}
		out = append(out, 0)
		copy(out[i+1:], out[i:])
		out[i] = rune(n & 0x1FFFFF) // n ≤ MaxRune проверен выше; маска — для линтера
		i++
	}
	return out, true
}

// punyEncode — кодировщик Punycode (RFC 3492) для проверки канона в xnLabel: вход — декодированная
// метка (≤ 63 знаков, ≤ MaxRune), так что delta < 64·(MaxRune+1)·64 и int64 не переполняется.
func punyEncode(in []rune) string {
	const base, tmin, tmax, skew, damp = 36, 1, 26, 38, 700
	var b strings.Builder
	for _, r := range in {
		if r < utf8.RuneSelf {
			b.WriteRune(r)
		}
	}
	basic := int64(b.Len())
	if basic > 0 {
		b.WriteByte('-')
	}
	n, delta, bias, h := int64(128), int64(0), int64(72), basic
	for h < int64(len(in)) {
		m := int64(unicode.MaxRune) + 1
		for _, r := range in {
			if c := int64(r); c >= n && c < m {
				m = c
			}
		}
		delta += (m - n) * (h + 1)
		n = m
		for _, r := range in {
			c := int64(r)
			if c < n {
				delta++
			}
			if c != n {
				continue
			}
			q := delta
			for k := int64(base); ; k += base {
				t := min(max(k-bias, tmin), tmax)
				if q < t {
					break
				}
				b.WriteByte(punyChar(t + (q-t)%(base-t)))
				q = (q - t) / (base - t)
			}
			b.WriteByte(punyChar(q))
			bias = punyAdapt(delta, h+1, h == basic, damp, skew)
			delta = 0
			h++
		}
		delta++
		n++
	}
	return b.String()
}

// punyChar — цифра Punycode 0…35 строчной буквой или цифрой (t + (q-t)%(36-t) < 36, последняя q < t ≤ 26).
func punyChar(d int64) byte { return "abcdefghijklmnopqrstuvwxyz0123456789"[d] }

func punyDigit(c byte) int {
	switch {
	case c >= 'a' && c <= 'z':
		return int(c - 'a')
	case c >= 'A' && c <= 'Z':
		return int(c - 'A')
	case c >= '0' && c <= '9':
		return int(c-'0') + 26
	}
	return -1
}

func punyAdapt(delta, count int64, first bool, damp, skew int64) int64 {
	if first {
		delta /= damp
	} else {
		delta /= 2
	}
	delta += delta / count
	var k int64
	for delta > (35*26)/2 {
		delta /= 35
		k += 36
	}
	return k + 36*delta/(delta+skew)
}
