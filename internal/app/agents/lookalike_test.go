package agents

import (
	"strings"
	"testing"
)

// Подделка под программу (М2): греческая, кириллическая и цифровая подмена, ударение, полная
// ширина, пробелы — всё сводится к «sosnin»; обычные клиенты и пустое имя — нет.
func TestLookalike(t *testing.T) {
	for _, bad := range []string{
		"Sosnina", "Sοsnina" /* греческая ο */, "S0snina", "Sosnína", "Ｓｏｓｎｉｎａ", "Sos nina", "Sosnina Albums",
		"S\u200bosnina" /* невидимый знак */, "Sоѕnіna" /* кириллица */, "Sοsnιna" /* греческая ι */, "Soѕnin",
		"Сосн", "С о с н", "СОСНИНА", "sósnina", /* ударение отдельным знаком */
	} {
		if !Lookalike(bad) {
			t.Errorf("%q: не признано похожим", bad)
		}
	}
	for _, ok := range []string{"Claude Code", "Cursor", "Claude Desktop", "", "Соня", "Sonia", "Sosna"} {
		if Lookalike(ok) {
			t.Errorf("%q: признано похожим", ok)
		}
	}
}

// Огромное и битое имя не ломает сравнение: ответ, а не паника.
func TestLookalikeHuge(t *testing.T) {
	if Lookalike(strings.Repeat("a", 1<<16)) || Lookalike("\xff\xfe") {
		t.Error("мусор признан похожим")
	}
}
