package psd

import (
	"bufio"
	"encoding/binary"
	"errors"
	"image"
	"io"
	"math"
)

// Preview читает сведённое изображение и уменьшает его потоком до maxSide px по длинной стороне.
// Если картинку показать нельзя (Info.Image = false), возвращает nil-картинку без ошибки:
// направляющие всё равно нужны. Память — превью, одна строка и одна полоса сумм; полное
// изображение не создаётся.
func Preview(r io.ReadSeeker, size int64, maxSide int) (*image.NRGBA, Info, error) {
	if maxSide < 1 {
		return nil, Info{}, errors.New("psd: maxSide меньше 1")
	}
	d, err := parse(r, size)
	if err != nil {
		return nil, Info{}, err
	}
	if !d.Image {
		return nil, d.Info, nil
	}
	if _, err := r.Seek(d.imgOff, io.SeekStart); err != nil {
		return nil, Info{}, err
	}
	x := &rd{r: bufio.NewReaderSize(r, 1<<16), size: size, pos: d.imgOff} // sk не нужен: skip тут не зовётся
	img, err := d.scale(x, maxSide)
	if err != nil {
		return nil, Info{}, err
	}
	return img, d.Info, nil
}

// readRLETable читает таблицу длин строк всех каналов (каналы сверх нужных только проверяются),
// проверяет каждую длину пределом и сумму нужных — остатком файла и снизу минимумом PackBits:
// строке rowBytes нужно не меньше 2 байт на каждые 128 (повтор), иначе таблица из нулей прошла
// бы до выделения превью по заголовку. Память — до 3,4 МБ.
func (d *doc) readRLETable(x *rd, need, rowBytes, maxPacked int) ([]uint16, error) {
	counts := make([]uint16, need*d.H)
	buf := make([]byte, 2*d.H)
	var sum int64
	for c := range d.Channels {
		if err := x.full(buf); err != nil {
			return nil, err
		}
		for y := range d.H {
			n := binary.BigEndian.Uint16(buf[2*y:])
			if int(n) > maxPacked {
				return nil, ErrCorrupt
			}
			if c < need {
				counts[c*d.H+y] = n
				sum += int64(n)
			}
		}
	}
	if sum > x.size-x.pos || sum < int64(need*d.H)*2*int64((rowBytes+127)/128) {
		return nil, ErrCorrupt
	}
	return counts, nil
}

func (d *doc) scale(x *rd, maxSide int) (*image.NRGBA, error) {
	need := needChannels(d.Mode)
	bps := d.Depth / 8
	rowBytes := d.W * bps
	maxPacked := rowBytes + (rowBytes+127)/128 // худший PackBits: байт заголовка на 128 литералов
	var counts []uint16
	if d.comp == 1 {
		var err error
		if counts, err = d.readRLETable(x, need, rowBytes, maxPacked); err != nil {
			return nil, err
		}
	} else if int64(need)*int64(d.H)*int64(rowBytes) > x.size-x.pos {
		return nil, ErrCorrupt
	}
	k := (max(d.W, d.H) + maxSide - 1) / maxSide
	pw, ph := (d.W+k-1)/k, (d.H+k-1)/k
	img := image.NewNRGBA(image.Rect(0, 0, pw, ph))
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255
	}
	row := make([]byte, rowBytes)
	src := make([]byte, maxPacked)
	sum := make([]int64, pw)
	for c := range need {
		for py := range ph {
			clear(sum)
			y1 := min(py*k+k, d.H)
			for y := py * k; y < y1; y++ {
				if err := d.readRow(x, counts, c, y, row, src); err != nil {
					return nil, err
				}
				addRow(sum, row, bps, k, d.W)
			}
			d.store(img, c, py, sum, k, y1-py*k)
		}
	}
	return img, nil
}

// readRow читает строку y канала c в row (raw или PackBits).
func (d *doc) readRow(x *rd, counts []uint16, c, y int, row, src []byte) error {
	if d.comp == 0 {
		return x.full(row)
	}
	n := int(counts[c*d.H+y])
	if err := x.full(src[:n]); err != nil {
		return err
	}
	if unpackBits(src[:n], row) != nil {
		return ErrCorrupt
	}
	return nil
}

// addRow добавляет в sum[px] значения строки блоками по k пикселей; у 16 бит — старший байт.
func addRow(sum []int64, row []byte, bps, k, w int) {
	for px := range sum {
		var s int64
		for i := px * k; i < min(px*k+k, w); i++ {
			s += int64(row[i*bps])
		}
		sum[px] += s
	}
}

// store пишет среднее по блокам полосы py в канал c превью. Края делятся на фактическое число
// пикселей. Gray — во все три канала; CMYK хранится обращённым (255 — без краски), K' домножает
// C'M'Y' — приближение без профиля, только для подложки на экране.
func (d *doc) store(img *image.NRGBA, c, py int, sum []int64, k, rowsN int) {
	for px, s := range sum {
		cnt := int64((min(px*k+k, d.W) - px*k) * rowsN)
		v := clamp8((s + cnt/2) / cnt)
		p := img.Pix[py*img.Stride+4*px:]
		switch {
		case d.Mode == modeGray:
			p[0], p[1], p[2] = v, v, v
		case c < 3:
			p[c] = v
		default:
			for j := range 3 {
				p[j] = clamp8(int64(p[j]) * int64(v) / 255)
			}
		}
	}
}

// clamp8 — значение в byte; границы нужны линтеру (среднее и произведение и так в 0…255).
func clamp8(v int64) byte {
	if v < 0 {
		return 0
	}
	if v > math.MaxUint8 {
		return math.MaxUint8
	}
	return byte(v)
}
