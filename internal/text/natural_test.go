package text

import (
	"math"
	"testing"
)

func TestNaturalWidth(t *testing.T) {
	one := "Иван и Мария приглашают вас на свою свадьбу"
	want := Layout(one, testStyle, 1e5).Lines[0].W
	if got := NaturalWidth(one, testStyle); got != want || got <= 40 {
		t.Fatalf("одна строка: %.3f, ждали %.3f", got, want)
	}
	// Без переноса: в ширину natural текст встаёт одной строкой.
	if n := len(Cached(one, testStyle, math.Ceil(want*100)/100+0.01).Lines); n != 1 {
		t.Fatalf("в свою ширину — %d строк", n)
	}
	// Абзацы — самый длинный.
	short := "Иван"
	if got := NaturalWidth(short+"\n"+one+"\n"+short, testStyle); got != want {
		t.Fatalf("абзацы: %.3f, ждали %.3f", got, want)
	}
	if got := NaturalWidth("", testStyle); got != 0 {
		t.Fatalf("пустой: %.3f", got)
	}
}
