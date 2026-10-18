package psd

import "errors"

var errRLE = errors.New("psd: неверный PackBits")

// unpackBits распаковывает PackBits ровно в dst. Литерал или повтор за край src или dst,
// а также короткая строка — ошибка; -128 — пропуск. Без выхода за буфер на любом входе.
func unpackBits(src, dst []byte) error {
	o := 0
	for i := 0; i < len(src); {
		n := int(src[i]) // заголовок как int8: 128…255 → −128…−1
		if n >= 128 {
			n -= 256
		}
		i++
		switch {
		case n >= 0:
			k := n + 1
			if i+k > len(src) || o+k > len(dst) {
				return errRLE
			}
			copy(dst[o:], src[i:i+k])
			i += k
			o += k
		case n != -128:
			k := 1 - n
			if i >= len(src) || o+k > len(dst) {
				return errRLE
			}
			b := src[i]
			i++
			for j := range k {
				dst[o+j] = b
			}
			o += k
		}
	}
	if o != len(dst) {
		return errRLE
	}
	return nil
}
