package export

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRange(t *testing.T) {
	got, err := ParseRange("3-5, 7п, 3", 10)
	want := []Unit{{3, 0}, {4, 0}, {5, 0}, {7, 2}}
	if err != nil || len(got) != len(want) {
		t.Fatalf("ParseRange = %v, %v", got, err)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ParseRange = %v, ждали %v", got, want)
		}
	}
	// Подсказки ошибок: что именно не так, чтобы поле в окне могло это показать.
	for in, frag := range map[string]string{
		"п":     "слитно",
		"7 п":   "слитно",
		"3-5п":  "только у одного",
		"7п-9":  "только у одного",
		"12":    "нет (всего 10)",
		"0п":    "с 1",
		"5-3":   "наоборот",
		"":      "укажите",
		"7пп":   "не понимаю",
		"1,2,3": "",
	} {
		_, err := ParseRange(in, 10)
		switch {
		case frag == "" && err != nil:
			t.Errorf("%q: неожиданная ошибка %v", in, err)
		case frag != "" && (err == nil || !strings.Contains(err.Error(), frag)):
			t.Errorf("%q: ждали ошибку с %q, получили %v", in, frag, err)
		}
	}
	if _, err := ParseRange(strings.Repeat("1,", 600), 10); err == nil {
		t.Error("строка длиннее 1000 знаков должна отклоняться")
	}
}

// Общие примеры с export/exportrange.ts: тот же файл читает exportrange.test.ts.
func TestParseRangeMatchesUI(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "parity", "ranges.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		In   string   `json:"in"`
		N    int      `json:"n"`
		Want [][2]int `json:"want"`
		Err  bool     `json:"err"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 30 {
		t.Fatalf("в testdata/parity/ranges.json всего %d примеров", len(cases))
	}
	for _, c := range cases {
		got, err := ParseRange(c.In, c.N)
		if c.Err {
			if err == nil {
				t.Errorf("%q (n=%d): ждали ошибку, получили %v", c.In, c.N, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q (n=%d): %v", c.In, c.N, err)
			continue
		}
		if len(got) != len(c.Want) {
			t.Errorf("%q (n=%d): %v, ждали %v", c.In, c.N, got, c.Want)
			continue
		}
		for i, w := range c.Want {
			if got[i] != (Unit{w[0], w[1]}) {
				t.Errorf("%q (n=%d): %v, ждали %v", c.In, c.N, got, c.Want)
				break
			}
		}
	}
}
