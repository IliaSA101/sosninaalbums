package albums

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestCheckID(t *testing.T) {
	for _, id := range []string{
		strings.Repeat("a", 22), strings.Repeat("A", 64), "k3J_-k3J_-k3J_-k3J_-k3", "_" + strings.Repeat("-", 21), "0" + strings.Repeat("9", 21),
	} {
		if err := CheckID(id); err != nil {
			t.Errorf("%q: %v", id, err)
		}
	}
	for name, id := range map[string]string{
		"пусто": "", "21 знак": strings.Repeat("a", 21), "65 знаков": strings.Repeat("a", 65),
		"первый -": "-" + strings.Repeat("a", 21), "слэш": strings.Repeat("a", 21) + "/", "точки": strings.Repeat("a", 20) + "..",
		"пробел": strings.Repeat("a", 21) + " ", "процент": strings.Repeat("a", 21) + "%", "кириллица": strings.Repeat("я", 22),
		"NUL": strings.Repeat("a", 21) + "\x00", "перевод строки": strings.Repeat("a", 21) + "\n", "огромный": strings.Repeat("a", 1<<20),
	} {
		wantInvalid(t, name, CheckID(id))
	}
}

func TestCheckToken(t *testing.T) {
	for _, tok := range []string{strings.Repeat("a", 16), strings.Repeat("~", 512), "sa_1234567890.-ABC!", "!" + strings.Repeat("x", 15)} {
		if err := CheckToken(tok); err != nil {
			t.Errorf("%.20q: %v", tok, err)
		}
	}
	for name, tok := range map[string]string{
		"пусто": "", "15 знаков": strings.Repeat("a", 15), "513 знаков": strings.Repeat("a", 513),
		"пробел внутри": strings.Repeat("a", 8) + " " + strings.Repeat("a", 8), "перевод строки": strings.Repeat("a", 16) + "\n",
		"CR": strings.Repeat("a", 16) + "\r", "табуляция": strings.Repeat("a", 8) + "\t" + strings.Repeat("a", 8),
		"DEL": strings.Repeat("a", 16) + "\x7f", "не ASCII": strings.Repeat("я", 16), "NUL": strings.Repeat("a", 16) + "\x00",
	} {
		wantInvalid(t, name, CheckToken(tok))
	}
}

func TestParsePageN(t *testing.T) {
	for in, want := range map[string]int{"1": 1, "9": 9, "10": 10, "62": 62, "300": 300} {
		if got, err := ParsePageN(in, 300); err != nil || got != want {
			t.Errorf("%q: %d, %v", in, got, err)
		}
	}
	if _, err := ParsePageN("62", 62); err != nil {
		t.Errorf("ровно m: %v", err)
	}
	for _, in := range []string{
		"", "0", "00", "01", "007", "-1", "+1", " 1", "1 ", "1.0", "1e2", "0x10", "٣", "63", "301", "999999999",
		"1234567890", strconv.Itoa(math.MaxInt), "9223372036854775808", "1\n", "\x00", "١", strings.Repeat("9", 1<<16),
	} {
		m := 62
		if in == "301" {
			m = 300
		}
		if n, err := ParsePageN(in, m); err == nil {
			t.Errorf("%.20q принят как %d", in, n)
		}
	}
	if _, err := ParsePageN("1", 0); err == nil {
		t.Error("m = 0: страниц нет")
	}
}

func TestCheckPassword(t *testing.T) {
	for _, p := range []string{"", "abcd", strings.Repeat("я", 64)} {
		if err := CheckPassword(p); err != nil {
			t.Errorf("%q: %v", p, err)
		}
	}
	for _, p := range []string{"a", "abc", strings.Repeat("a", 65), "ab\x00cd", "ab\u200bcd", "ab\u2029cd", "\xff\xfe\xfd\xfc"} {
		wantInvalid(t, p, CheckPassword(p))
	}
}

func TestValidationErrorNoValue(t *testing.T) {
	secret := "SECRET-ЗНАЧЕНИЕ-" + strings.Repeat("z", 30)
	for name, err := range map[string]error{
		"id":    CheckID(secret + "/"),
		"token": CheckToken(secret + " "),
		"pass":  CheckPassword(secret + "\n"),
		"title": Validate(&Manifest{Schema: 1, Generator: "g", Title: secret + "\n"}, Limits{}),
		"n":     func() error { _, e := ParsePageN(secret, 5); return e }(),
	} {
		if err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
