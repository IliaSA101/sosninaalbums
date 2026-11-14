package testkit

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"math"
	"testing"
)

// PSDGuide — направляющая синтетического PSD: Pos в px документа, Horz — горизонтальная.
type PSDGuide struct {
	Pos  float64
	Horz bool
}

// PSDSpec — синтетический PSD (Photoshop v1, RGB 8 бит, без сжатия): W×H одного цвета C,
// DPI (0 — без ресурса 1005), направляющие (ресурс 1032). NoMerged — ресурс 1057 «сведённого
// нет»; Version 2 — заголовок PSB.
type PSDSpec struct {
	W, H     int
	C        color.NRGBA
	DPI      float64
	Guides   []PSDGuide
	NoMerged bool
	Version  int
}

// PSD пишет в dir файл name по spec (разметка типографии, спека 066).
func PSD(t testing.TB, dir, name string, s PSDSpec) string {
	t.Helper()
	return write(t, dir, name, PSDBytes(s))
}

// PSDBytes — байты PSD по spec.
func PSDBytes(s PSDSpec) []byte {
	be := binary.BigEndian
	var b bytes.Buffer
	b.WriteString("8BPS")
	b.Write(be.AppendUint16(nil, uint16(max(s.Version, 1))))
	b.Write(make([]byte, 6))
	b.Write(be.AppendUint16(nil, 3))
	b.Write(be.AppendUint32(nil, uint32(s.H)))
	b.Write(be.AppendUint32(nil, uint32(s.W)))
	b.Write(be.AppendUint16(nil, 8))
	b.Write(be.AppendUint16(nil, 3)) // RGB
	b.Write(be.AppendUint32(nil, 0)) // color mode data
	var res bytes.Buffer
	if s.DPI > 0 {
		fixed := be.AppendUint32(nil, uint32(math.Round(s.DPI*65536)))
		d := append(append(fixed, 0, 1, 0, 1), fixed...)
		psdBlock(&res, 1005, append(d, 0, 1, 0, 1))
	}
	if len(s.Guides) > 0 {
		d := be.AppendUint32(nil, 1)
		d = be.AppendUint32(d, 0)
		d = be.AppendUint32(d, 0)
		d = be.AppendUint32(d, uint32(len(s.Guides)))
		for _, g := range s.Guides {
			d = be.AppendUint32(d, uint32(int32(math.Round(g.Pos*32))))
			d = append(d, byte(btoi(g.Horz)))
		}
		psdBlock(&res, 1032, d)
	}
	if s.NoMerged {
		psdBlock(&res, 1057, []byte{0, 0, 0, 1, 0})
	}
	b.Write(be.AppendUint32(nil, uint32(res.Len())))
	b.Write(res.Bytes())
	b.Write(be.AppendUint32(nil, 0)) // слои
	b.Write(be.AppendUint16(nil, 0)) // без сжатия
	for _, v := range []byte{s.C.R, s.C.G, s.C.B} {
		b.Write(bytes.Repeat([]byte{v}, s.W*s.H))
	}
	return b.Bytes()
}

// psdBlock — блок ресурса 8BIM с пустым именем; данные выровнены до чётного.
func psdBlock(b *bytes.Buffer, id uint16, data []byte) {
	b.WriteString("8BIM")
	b.Write(binary.BigEndian.AppendUint16(nil, id))
	b.Write([]byte{0, 0})
	b.Write(binary.BigEndian.AppendUint32(nil, uint32(len(data))))
	b.Write(data)
	if len(data)%2 == 1 {
		b.WriteByte(0)
	}
}
