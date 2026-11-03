package model

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSpreadTplV1KeysStable — страж ключей (131 G1, design §1): списки borders/drafts не
// сдвигают ключ шаблона без них. Эталон — 12 встроенных и 4 шаблона Т7 (testdata/tplv1; ключи
// посчитаны кодом до G1). Ключ ушёл — программа 0.28 продублировала бы в библиотеке каждый
// шаблон, а набор и 0.27 перестали бы узнавать записи друг друга.
func TestSpreadTplV1KeysStable(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "tplv1", "keys.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		name, want, ok := strings.Cut(sc.Text(), " ")
		if !ok {
			t.Fatalf("строка эталона %q", sc.Text())
		}
		data, err := os.ReadFile(filepath.Join("testdata", "tplv1", name))
		if err != nil {
			t.Fatal(err)
		}
		tp, err := ParseSpreadTpl(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := TplKey(tp); got != want {
			t.Errorf("%s: ключ %s, в эталоне %s", name, got, want)
		}
		if len(tp.Borders)+len(tp.Drafts) != 0 {
			t.Errorf("%s: из файла v1 появились рамочки или заготовки", name)
		}
		n++
	}
	if n != 16 {
		t.Errorf("в эталоне %d шаблонов, нужно 16 (12 встроенных и 4 Т7)", n)
	}
}
