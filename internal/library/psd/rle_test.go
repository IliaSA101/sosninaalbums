package psd

import (
	"bytes"
	"math/rand/v2"
	"testing"
)

func TestUnpackBits(t *testing.T) {
	for _, c := range []struct {
		name string
		src  []byte
		dst  int
		want []byte
		ok   bool
	}{
		{"литерал", []byte{2, 1, 2, 3}, 3, []byte{1, 2, 3}, true},
		{"повтор", []byte{0xfe, 7}, 3, []byte{7, 7, 7}, true}, // -2 → 3 копии
		{"повтор 128", []byte{0x81, 9}, 128, bytes.Repeat([]byte{9}, 128), true},
		{"пропуск -128", []byte{0x80, 0, 5}, 1, []byte{5}, true},
		{"смесь", []byte{1, 1, 2, 0xff, 3, 0, 0}, 5, []byte{1, 2, 3, 3, 0}, true},
		{"литерал за край src", []byte{3, 1, 2}, 4, nil, false},
		{"повтор без байта", []byte{0xfe}, 3, nil, false},
		{"литерал за край dst", []byte{3, 1, 2, 3, 4}, 3, nil, false},
		{"повтор за край dst", []byte{0xfd, 1}, 3, nil, false},
		{"короткая строка", []byte{1, 1, 2}, 3, nil, false},
		{"пустой src, dst 0", nil, 0, []byte{}, true},
		{"пустой src, dst 1", nil, 1, nil, false},
		{"лишнее после заполнения", []byte{0, 1, 0, 2}, 1, nil, false},
	} {
		dst := make([]byte, c.dst)
		err := unpackBits(c.src, dst)
		if (err == nil) != c.ok {
			t.Fatalf("%s: err = %v, ждали ok = %v", c.name, err, c.ok)
		}
		if c.ok && !bytes.Equal(dst, c.want) {
			t.Fatalf("%s: %v, ждали %v", c.name, dst, c.want)
		}
	}
}

// Круг: кодировщик теста → unpackBits возвращает ту же строку (в том числе на длинных повторах и литералах).
func TestUnpackBitsRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for range 300 {
		row := make([]byte, 1+rng.IntN(600))
		for i := 0; i < len(row); {
			n := min(1+rng.IntN(200), len(row)-i)
			v := byte(rng.IntN(4))
			noisy := rng.IntN(2) == 0
			for j := range n {
				row[i+j] = v
				if noisy {
					row[i+j] = byte(rng.IntN(256))
				}
			}
			i += n
		}
		got := make([]byte, len(row))
		if err := unpackBits(packBits(row), got); err != nil || !bytes.Equal(got, row) {
			t.Fatalf("круг не сошёлся (%d байт): %v", len(row), err)
		}
	}
}

// Любой мусор на входе: без паники и без записи за dst.
func TestUnpackBitsGarbage(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	for range 2000 {
		src := make([]byte, rng.IntN(40))
		for i := range src {
			src[i] = byte(rng.IntN(256))
		}
		_ = unpackBits(src, make([]byte, rng.IntN(60)))
	}
}
