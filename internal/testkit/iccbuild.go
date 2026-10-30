package testkit

import (
	"encoding/binary"
	"math"
)

// Точные колоранты (s15Fixed16 как целые, /65536) для сверки с LittleCMS: опорные значения
// спеки 119 получены из профилей, собранных именно этими числами. Порядок как у XYZSRGB.
var (
	XYZP3Exact     = s15XYZ(33759, 15807, -69, 19135, 45367, 2745, 10296, 4363, 51385)
	XYZProPhoto    = s15XYZ(52276, 18877, 0, 8860, 46654, 0, 2054, 6, 54061)
	XYZLinearSRGB  = s15XYZ(28576, 14581, 912, 25239, 46983, 6361, 9375, 3972, 46787)
	XYZRec709Exact = s15XYZ(28564, 14574, 912, 25253, 46992, 6366, 9373, 3971, 46782)
)

func s15XYZ(v ...int32) (o [9]float64) {
	for i, x := range v {
		o[i] = float64(x) / 65536
	}
	return o
}

// XiaomiP3Name — имя профиля Xiaomi (скриншот владельца 23.10): Display P3 под другим именем.
const XiaomiP3Name = "sRGB EOTF with DCI-P3 Color Gamut"

// SRGBPara3 — параметры para типа 3 для кривой sRGB: g, a, b, c, d (s15Fixed16 как у LittleCMS).
var SRGBPara3 = []float64{157286.0 / 65536, 62119.0 / 65536, 3417.0 / 65536, 5072.0 / 65536, 2651.0 / 65536}

// ParaTag — кривая para: тип 0…4, параметры s15Fixed16 (число параметров — по типу).
func ParaTag(typ uint16, params ...float64) []byte {
	b := append([]byte("para\x00\x00\x00\x00"), 0, 0, 0, 0)
	binary.BigEndian.PutUint16(b[8:], typ)
	for _, p := range params {
		b = append(b, be32(uint32(int32(math.Round(p*65536))))...) //nolint:gosec // s15Fixed16
	}
	return b
}

// TableTag — кривая curv из n точек f(i/(n-1)) в 16 битах.
func TableTag(n int, f func(float64) float64) []byte {
	b := append([]byte("curv\x00\x00\x00\x00"), be32(uint32(n))...) //nolint:gosec // n — размер таблицы теста
	for i := range n {
		v := math.Max(0, math.Min(1, f(float64(i)/float64(n-1))))
		b = binary.BigEndian.AppendUint16(b, uint16(math.Round(v*65535))) //nolint:gosec // 0…65535
	}
	return b
}

// LinearTag — кривая curv без точек: тождество.
func LinearTag() []byte { return append([]byte("curv\x00\x00\x00\x00"), be32(0)...) }

func trcProfile(xyz [9]float64, c []byte, desc []byte, extra ...ICCTag) []byte {
	tags := []ICCTag{
		{"desc", desc},
		{"rXYZ", XYZTag(xyz[0:3]...)}, {"gXYZ", XYZTag(xyz[3:6]...)}, {"bXYZ", XYZTag(xyz[6:9]...)},
		{"rTRC", c}, {"gTRC", c}, {"bTRC", c},
	}
	return BuildICC("RGB ", append(tags, extra...)...)
}

// ParaProfile — матричный профиль с кривой para (одной на три канала).
func ParaProfile(xyz [9]float64, typ uint16, params []float64, desc []byte) []byte {
	return trcProfile(xyz, ParaTag(typ, params...), desc)
}

// TableProfile — матричный профиль с кривой-таблицей из n точек (одной на три канала).
func TableProfile(xyz [9]float64, n int, f func(float64) float64, desc []byte) []byte {
	return trcProfile(xyz, TableTag(n, f), desc)
}

// LinearProfile — матричный профиль с тождественной кривой (curv n=0).
func LinearProfile(xyz [9]float64, desc []byte) []byte {
	return trcProfile(xyz, LinearTag(), desc)
}

// LUTProfile — матричный профиль с пустым тегом A2B0: LittleCMS в таком случае берёт таблицу,
// поэтому программа его не пересчитывает (спека 119, сценарий 6).
func LUTProfile(xyz [9]float64, desc []byte) []byte {
	return trcProfile(xyz, CurvTag(2.2), desc, ICCTag{"A2B0", []byte("mft2\x00\x00\x00\x00")})
}

// SRGBEOTF — кривая sRGB: кодированное 0…1 → линейное.
func SRGBEOTF(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// Rec709EOTF — кривая BT.709 (обратная к OETF) для таблицы Rec. 709.
func Rec709EOTF(v float64) float64 {
	if v < 0.081 {
		return v / 4.5
	}
	return math.Pow((v+0.099)/1.099, 1/0.45)
}

// ProfileXiaomiP3 — синтетический профиль Xiaomi: примарии P3 D65, кривая sRGB (para 3), имя
// «sRGB EOTF with DCI-P3 Color Gamut» в desc (v2) или mluc (v4). Сходится с LittleCMS до уровня.
func ProfileXiaomiP3(v4 bool) []byte {
	d := DescV2(XiaomiP3Name)
	if v4 {
		d = DescV4(XiaomiP3Name)
	}
	return ParaProfile(XYZP3Exact, 3, SRGBPara3, d)
}

// SRGBProfile — обычный матричный sRGB (колоранты sRGB D50, кривая para 3): класс srgb, значка нет.
// MatrixProfile(XYZSRGB, 2.2, …) для этого не годится: гамма 2.2 — другая кривая, и такой профиль
// программа пересчитывает (спека 119, сценарий 7).
func SRGBProfile(desc []byte) []byte { return ParaProfile(XYZSRGB, 3, SRGBPara3, desc) }
