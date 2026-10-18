package psd

import (
	"bytes"
	"encoding/binary"
)

// block — ресурс изображения для buildPSD.
type block struct {
	id   uint16
	name string
	data []byte
	sig  string // "" — 8BIM
}

// spec описывает синтетический PSD. Нулевые поля — разумные значения: RGB, 8 бит, 3 канала, 300 ppi.
type spec struct {
	w, h, ch, depth, mode, comp int
	version                     int     // 0 — 1
	guides                      []Guide // ресурс 1032
	dpiFixed                    uint32  // 0 — 300; значение Fixed 16.16
	dpiUnit                     uint16  // 0 — 1 (ppi)
	noDPI, noMerged, bitmap     bool    // bitmap: режим 0 (Bitmap), а не RGB по умолчанию
	merged                      *byte   // значение hasRealMergedData; nil — 1
	extra                       []block
	colorData                   int // длина Color mode data
	layerData                   int // длина секции слоёв
	// val даёт значение канала c в пикселе (x, y); для 16 бит — старший байт (младший 0x7f).
	val func(c, x, y int) byte
}

func (s *spec) fill() {
	if s.ch == 0 {
		s.ch = 3
	}
	if s.depth == 0 {
		s.depth = 8
	}
	if s.mode == 0 && !s.bitmap {
		s.mode = modeRGB
	}
	if s.version == 0 {
		s.version = 1
	}
	if s.dpiFixed == 0 {
		s.dpiFixed = 300 << 16
	}
	if s.dpiUnit == 0 {
		s.dpiUnit = 1
	}
	if s.val == nil {
		s.val = func(c, x, y int) byte { return byte(c*50 + x*3 + y*7) }
	}
}

func be16(v int) []byte { return binary.BigEndian.AppendUint16(nil, uint16(v)) }
func be32(v int) []byte { return binary.BigEndian.AppendUint32(nil, uint32(v)) }

// res пишет один блок ресурса: подпись, id, Pascal-имя (выровнено до чётного), размер, данные (выровнены).
func res(b *bytes.Buffer, r block) {
	sig := r.sig
	if sig == "" {
		sig = "8BIM"
	}
	b.WriteString(sig)
	b.Write(be16(int(r.id)))
	b.WriteByte(byte(len(r.name)))
	b.WriteString(r.name)
	if (1+len(r.name))%2 == 1 {
		b.WriteByte(0)
	}
	b.Write(be32(len(r.data)))
	b.Write(r.data)
	if len(r.data)%2 == 1 {
		b.WriteByte(0)
	}
}

// guideBlock — данные ресурса 1032 для n направляющих.
func guideBlock(gs []Guide) []byte {
	b := append(be32(1), be32(0)...)
	b = append(b, be32(0)...)
	b = append(b, be32(len(gs))...)
	for _, g := range gs {
		b = append(b, be32(int(int32(g.Pos*32)))...)
		if g.Horz {
			b = append(b, 1)
		} else {
			b = append(b, 0)
		}
	}
	return b
}

// packBits — простой кодировщик PackBits для тестов: повторы от 2 и литералы до 128.
func packBits(row []byte) []byte {
	var out []byte
	for i := 0; i < len(row); {
		j := i
		for j+1 < len(row) && row[j+1] == row[i] && j-i < 127 {
			j++
		}
		if j > i {
			out = append(out, byte(int8(-(j - i))), row[i])
			i = j + 1
			continue
		}
		k := i
		for k < len(row) && k-i < 128 && (k+1 >= len(row) || row[k+1] != row[k]) {
			k++
		}
		out = append(out, byte(k-i-1))
		out = append(out, row[i:k]...)
		i = k
	}
	return out
}

// build собирает PSD и возвращает его байты и смещение данных изображения (после u16 сжатия).
func build(s spec) ([]byte, int) {
	s.fill()
	var b bytes.Buffer
	b.WriteString("8BPS")
	b.Write(be16(s.version))
	b.Write(make([]byte, 6))
	b.Write(be16(s.ch))
	b.Write(be32(s.h))
	b.Write(be32(s.w))
	b.Write(be16(s.depth))
	b.Write(be16(s.mode))
	b.Write(be32(s.colorData))
	b.Write(make([]byte, s.colorData))

	var r bytes.Buffer
	if !s.noDPI {
		d := append(be32(int(s.dpiFixed)), be16(int(s.dpiUnit))...)
		d = append(d, be16(1)...)
		d = append(d, be32(int(s.dpiFixed))...)
		d = append(d, be16(int(s.dpiUnit))...)
		d = append(d, be16(1)...)
		res(&r, block{id: 1005, data: d})
	}
	if s.guides != nil {
		res(&r, block{id: 1032, name: "xy", data: guideBlock(s.guides)}) // имя нечётной длины: проверка выравнивания
	}
	for _, e := range s.extra {
		res(&r, e)
	}
	if !s.noMerged {
		m := byte(1)
		if s.merged != nil {
			m = *s.merged
		}
		res(&r, block{id: 1057, data: append(be32(1), m)})
	}
	b.Write(be32(r.Len()))
	b.Write(r.Bytes())
	b.Write(be32(s.layerData))
	b.Write(make([]byte, s.layerData))
	b.Write(be16(s.comp))
	off := b.Len()

	bps := s.depth / 8
	if bps < 1 {
		bps = 1
	}
	rowsOf := func(c, y int) []byte {
		row := make([]byte, s.w*bps)
		for x := range s.w {
			v := s.val(c, x, y)
			if bps == 1 {
				row[x] = v
			} else {
				row[x*bps] = v
				for j := 1; j < bps; j++ {
					row[x*bps+j] = 0x7f
				}
			}
		}
		return row
	}
	if s.comp == 0 {
		for c := range s.ch {
			for y := range s.h {
				b.Write(rowsOf(c, y))
			}
		}
		return b.Bytes(), off
	}
	var packed [][]byte
	for c := range s.ch {
		for y := range s.h {
			packed = append(packed, packBits(rowsOf(c, y)))
		}
	}
	for _, p := range packed {
		b.Write(be16(len(p)))
	}
	for _, p := range packed {
		b.Write(p)
	}
	return b.Bytes(), off
}

func mk(s spec) []byte {
	d, _ := build(s)
	return d
}
