package faces

import (
	"encoding/binary"
	"math"
	"testing"
)

// Кодирование protobuf для сборки крошечных моделей в тестах.
func pbv(num int, v uint64) []byte {
	return binary.AppendUvarint(binary.AppendUvarint(nil, uint64(num)<<3), v)
}

func pbb(num int, b []byte) []byte {
	out := binary.AppendUvarint(nil, uint64(num)<<3|2)
	out = binary.AppendUvarint(out, uint64(len(b)))
	return append(out, b...)
}

func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func nodeBytes(op string, in, out []string, attrs ...[]byte) []byte {
	var b []byte
	for _, s := range in {
		b = append(b, pbb(1, []byte(s))...)
	}
	for _, s := range out {
		b = append(b, pbb(2, []byte(s))...)
	}
	b = append(b, pbb(4, []byte(op))...)
	for _, a := range attrs {
		b = append(b, pbb(5, a)...)
	}
	return b
}

func intsAttr(name string, v ...int64) []byte {
	var packed []byte
	for _, x := range v {
		packed = binary.AppendUvarint(packed, uint64(x))
	}
	return cat(pbb(1, []byte(name)), pbb(8, packed))
}

func floatTensor(name string, dims []int64, v ...float32) []byte {
	var raw []byte
	for _, x := range v {
		raw = binary.LittleEndian.AppendUint32(raw, math.Float32bits(x))
	}
	b := pbv(2, 1)
	for _, d := range dims {
		b = append(b, pbv(1, uint64(d))...)
	}
	return cat(b, pbb(8, []byte(name)), pbb(9, raw))
}

func model(graph ...[]byte) []byte { return pbb(7, cat(graph...)) }

// TestParseBroken: пустая, усечённая и испорченная модель — ошибка, а не паника и не выделение
// памяти по чужому счётчику (design: предел до чтения).
func TestParseBroken(t *testing.T) {
	good := model(pbb(1, nodeBytes("Add", []string{"a", "b"}, []string{"c"})))
	if _, err := parseONNX(good); err != nil {
		t.Fatalf("исправная малая модель: %v", err)
	}
	ten := binary.AppendUvarint(nil, 1<<62)
	bad := map[string][]byte{
		"пусто":                  nil,
		"нет графа":              pbv(1, 8),
		"varint без конца":       {0x08, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80},
		"длина за краем":         {0x3a, 0xff, 0xff, 0xff, 0x7f, 1, 2, 3},
		"длина 2^62":             append([]byte{0x3a}, ten...),
		"тип провода 3":          {0x0b},
		"тип провода 7":          {0x0f, 0},
		"номер поля 0":           {0x00, 0x00},
		"32 бита обрезаны":       {0x0d, 1, 2},
		"64 бита обрезаны":       {0x09, 1, 2, 3},
		"граф: узел обрезан":     model(pbb(1, []byte{0x0a, 0x7f, 'a'})),
		"тензор: dims 2^40":      model(pbb(5, cat(pbv(1, 1<<40), pbv(2, 1), pbb(8, []byte("w"))))),
		"тензор: dims 2×2^21":    model(pbb(5, cat(pbv(1, 1<<21), pbv(1, 1<<21), pbv(2, 1), pbb(8, []byte("w"))))),
		"тензор: raw короче":     model(pbb(5, floatTensor("w", []int64{4}, 1, 2))),
		"тензор: тип 99":         model(pbb(5, cat(pbv(2, 99), pbb(8, []byte("w"))))),
		"тензор: 9 размерностей": model(pbb(5, floatTensor("w", []int64{1, 1, 1, 1, 1, 1, 1, 1, 1}, 1))),
		"имя больше 1 КБ":        model(pbb(1, pbb(4, make([]byte, 2000)))),
		"атрибут без имени":      model(pbb(1, pbb(5, pbv(3, 1)))),
	}
	for name, b := range bad {
		if _, err := parseONNX(b); err == nil {
			t.Errorf("%s: ошибки нет", name)
		}
	}
	if _, err := parseONNX(make([]byte, maxModelBytes+1)); err == nil {
		t.Error("модель больше предела принята")
	}
	many := make([]byte, 0, 4*maxPBFields)
	for range maxPBFields + 10 {
		many = append(many, 0x08, 0x01) // поле 1, varint 1
	}
	if _, err := pbFields(many); err == nil {
		t.Error("число полей не ограничено")
	}
}

// TestParseTruncatedModel: настоящая модель, обрезанная в разных местах и с испорченными байтами,
// не роняет разбор и компиляцию.
func TestParseTruncatedModel(t *testing.T) {
	step := len(modelFile)/211 + 1
	for cut := 0; cut < len(modelFile); cut += step {
		if g, err := parseONNX(modelFile[:cut]); err == nil {
			_, _ = compileNet(g) // обрезка на границе поля может дать цельный, но неполный граф
		}
	}
	for i := 0; i < len(modelFile); i += 997 {
		broken := append([]byte(nil), modelFile...)
		broken[i] ^= 0xff
		if g, err := parseONNX(broken); err == nil {
			_, _ = compileNet(g)
		}
	}
}

// TestCompileRejects: граф, который вывод не умеет, — ошибка при загрузке, а не неверный ответ.
func TestCompileRejects(t *testing.T) {
	conv := func(out string) []byte { return nodeBytes("Conv", []string{"input", "w"}, []string{out}) }
	w1 := pbb(5, floatTensor("w", []int64{1, 3, 1, 1}, 1, 1, 1))
	cases := map[string][]byte{
		"неизвестная операция":  model(pbb(1, nodeBytes("Softmax", []string{"input"}, []string{"o"}))),
		"Relu без свёртки":      model(pbb(1, nodeBytes("Relu", []string{"input"}, []string{"o"}))),
		"Conv без весов":        model(pbb(1, conv("o"))),
		"Conv: веса не 4D":      model(pbb(1, conv("o")), pbb(5, floatTensor("w", []int64{3}, 1, 2, 3))),
		"Conv: dilations 2":     model(pbb(1, nodeBytes("Conv", []string{"input", "w"}, []string{"o"}, intsAttr("dilations", 2, 2))), w1),
		"Conv: шаг 3":           model(pbb(1, nodeBytes("Conv", []string{"input", "w"}, []string{"o"}, intsAttr("strides", 3, 3))), w1),
		"Conv: pads разные":     model(pbb(1, nodeBytes("Conv", []string{"input", "w"}, []string{"o"}, intsAttr("pads", 1, 0, 1, 0))), w1),
		"Conv: group 2 на 1":    model(pbb(1, nodeBytes("Conv", []string{"input", "w"}, []string{"o"}, intsAttr("group", 2))), w1),
		"MaxPool 3×3":           model(pbb(1, nodeBytes("MaxPool", []string{"input"}, []string{"o"}, intsAttr("kernel_shape", 3, 3), intsAttr("strides", 2, 2)))),
		"Resize ×3":             model(pbb(1, nodeBytes("Resize", []string{"input", "r", "s"}, []string{"o"})), pbb(5, floatTensor("s", []int64{4}, 1, 1, 3, 3))),
		"Transpose не 0 2 3 1":  model(pbb(1, nodeBytes("Transpose", []string{"input"}, []string{"o"}, intsAttr("perm", 0, 1, 2, 3)))),
		"Transpose без Reshape": model(pbb(1, nodeBytes("Transpose", []string{"input"}, []string{"o"}, intsAttr("perm", 0, 2, 3, 1)))),
	}
	for name, b := range cases {
		g, err := parseONNX(b)
		if err != nil {
			t.Errorf("%s: разбор: %v", name, err)
			continue
		}
		if _, err := compileNet(g); err == nil {
			t.Errorf("%s: компиляция приняла граф", name)
		}
	}
	// выход графа, которого нет среди голов
	g, err := parseONNX(cat(model(pbb(1, nodeBytes("Add", []string{"a", "b"}, []string{"c"})), pbb(12, pbb(1, []byte("cls_8"))))))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := compileNet(g); err == nil {
		t.Error("выход без головы принят")
	}
}

// TestRunRejectsBadSize: вход не кратен 32 или не того размера — ошибка.
func TestRunRejectsBadSize(t *testing.T) {
	n, err := loadNet()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range [][2]int{{0, 0}, {32, 33}, {31, 32}, {64, 0}, {-32, 32}} {
		if _, err := n.run(make([]float32, 3*32*32), c[0], c[1], 1, &memTrack{}); err == nil {
			t.Errorf("размер %v принят", c)
		}
	}
	if _, err := n.run(make([]float32, 10), 32, 32, 1, &memTrack{}); err == nil {
		t.Error("тензор не той длины принят")
	}
}

// FuzzParseONNX: любой набор байт — не паника; принятый граф компилируется без паники.
func FuzzParseONNX(f *testing.F) {
	f.Add(model(pbb(1, nodeBytes("Add", []string{"a", "b"}, []string{"c"}))))
	f.Add(model(pbb(1, nodeBytes("Conv", []string{"input", "w"}, []string{"o"}, intsAttr("pads", 1, 1, 1, 1))),
		pbb(5, floatTensor("w", []int64{1, 3, 3, 3}, make([]float32, 27)...))))
	f.Add(modelFile[:2048])
	f.Fuzz(func(t *testing.T, b []byte) {
		if g, err := parseONNX(b); err == nil {
			_, _ = compileNet(g)
		}
	})
}
