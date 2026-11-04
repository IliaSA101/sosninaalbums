package gen

import (
	"net/url"
	"strings"
	"testing"
)

// inkParam — значение i= адреса и есть ли оно; параметр один.
func inkParam(t *testing.T, addr string) (string, bool) {
	t.Helper()
	u, err := url.Parse(addr)
	if err != nil {
		t.Fatal(err)
	}
	vals := u.Query()["i"]
	if len(vals) > 1 {
		t.Fatalf("i= повторён: %s", addr)
	}
	if len(vals) == 0 {
		return "", false
	}
	return vals[0], true
}

// Формат i= держит живой показ цвета во фронте (inkUrl, 109 §6): HEX без «#», через запятую,
// порядок как в Inks — в узоре, стикере, рамочке и плитке.
func TestQueryInksFormat(t *testing.T) {
	two := []string{"#c8201e", "#203040"}
	cases := []struct {
		name, addr, want string
	}{
		{"узор", Query(Spec{Kind: "bands", Inks: two}, 100, 80), "c8201e,203040"},
		{"стикер", Query(Spec{Kind: "heart", Inks: two[:1]}, 0, 0), "c8201e"},
		{"рамочка", FrameQuery(FrameSpec{Kind: "line", Inks: two[:1], Width: 1.5, W: 60, H: 80}, false), "c8201e"},
		{"плитка", TileQuery("line", two[:1], 2), "c8201e"},
	}
	for _, c := range cases {
		got, ok := inkParam(t, c.addr)
		if !ok || got != c.want || strings.Contains(c.addr, "%23") || strings.Contains(c.addr, "#") {
			t.Errorf("%s: i=%q (есть %v), ждали %q, адрес %s", c.name, got, ok, c.want, c.addr)
		}
	}
	// Краски в узоре: порядок сохраняется.
	if got, _ := inkParam(t, Query(Spec{Kind: "bands", Inks: []string{"#203040", "#c8201e"}}, 100, 80)); got != "203040,c8201e" {
		t.Errorf("порядок красок: %q", got)
	}
	// Краски не заданы: узор по умолчанию i= не пишет (inkUrl такой адрес не трогает), у рамочки и
	// плитки Clamp ставит белый, и он в адресе есть.
	if got, ok := inkParam(t, Query(Spec{Kind: "bands"}, 100, 80)); ok {
		t.Errorf("узор без красок: i=%q", got)
	}
	for name, addr := range map[string]string{
		"рамочка": FrameQuery(FrameSpec{Kind: "line", W: 60, H: 80}, false),
		"плитка":  TileQuery("line", nil, 0),
	} {
		if got, ok := inkParam(t, addr); !ok || got != "ffffff" {
			t.Errorf("%s без красок: i=%q (есть %v)", name, got, ok)
		}
	}
}
