// Package faces — поиск лиц на фото: детектор YuNet (2023mar, OpenCV Zoo, веса MIT) со своим
// выводом сети на чистом Go, без cgo и сторонних модулей. Модель вшита и неизменна: набор
// операций закрыт, новая модель — новая спека.
package faces

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// Пределы разбора. Модель вшита и проверена по sha256 (TestModelPinned), но разбор всё равно
// не доверяет файлу: счётчики и размеры внутри формата режутся до выделения памяти.
const (
	maxModelBytes = 8 << 20 // файл модели
	maxTensorElem = 1 << 22 // элементов в одном тензоре весов
	maxNodes      = 1 << 12 // узлов графа
	maxNameBytes  = 1 << 10 // имя тензора, узла, атрибута
	maxDims       = 8       // размерностей у тензора
	maxInts       = 1 << 10 // целых в атрибуте
	maxPBFields   = 1 << 14 // полей в одном сообщении (в модели их порядка 250)
)

var errPB = errors.New("faces: повреждённая модель")

// pbField — одно поле protobuf: номер, тип провода, число (varint) или байты (длина-префикс, 32/64 бита).
type pbField struct {
	num, wire int
	v         uint64
	b         []byte
}

// pbVarint читает varint с позиции i: значение и позиция после него.
func pbVarint(b []byte, i int) (uint64, int, error) {
	var r uint64
	for s := uint(0); s < 70; s += 7 {
		if i >= len(b) {
			return 0, 0, errPB
		}
		x := b[i]
		i++
		r |= uint64(x&0x7f) << s
		if x < 0x80 {
			return r, i, nil
		}
	}
	return 0, 0, errPB
}

// pbFields разбирает сообщение на поля; байты полей — срезы b, без копий.
func pbFields(b []byte) ([]pbField, error) {
	var out []pbField
	for i := 0; i < len(b); {
		if len(out) >= maxPBFields {
			return nil, errPB
		}
		k, j, err := pbVarint(b, i)
		if err != nil {
			return nil, err
		}
		i = j
		f := pbField{num: int(k >> 3), wire: int(k & 7)}
		if f.num <= 0 || k>>3 > math.MaxInt32 {
			return nil, errPB
		}
		switch f.wire {
		case 0:
			f.v, i, err = pbVarint(b, i)
		case 1, 5:
			n := 4
			if f.wire == 1 {
				n = 8
			}
			if len(b)-i < n {
				return nil, errPB
			}
			f.b = b[i : i+n]
			i += n
		case 2:
			var ln int
			ln, i, err = pbLen(b, i)
			if err == nil {
				f.b = b[i : i+ln]
				i += ln
			}
		default:
			err = errPB
		}
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// toInt64 — 64 бита как знаковое число (дополнительный код): в формах бывает -1.
func toInt64(v uint64) int64 {
	low := int64(v & math.MaxInt64)
	if v>>63 != 0 {
		return low + math.MinInt64
	}
	return low
}

// pbLen читает длину поля с позиции i и проверяет, что поле целиком лежит в b: длина и позиция его начала.
func pbLen(b []byte, i int) (int, int, error) {
	n, j, err := pbVarint(b, i)
	if err != nil {
		return 0, 0, err
	}
	if n > math.MaxInt32 {
		return 0, 0, errPB
	}
	ln := int(n)
	if ln > len(b)-j {
		return 0, 0, errPB
	}
	return ln, j, nil
}

// pbInts читает повторяющееся целое поле: и одиночное значение, и «упакованное».
func pbInts(f pbField, dst []int64) ([]int64, error) {
	if f.wire == 0 {
		return append(dst, toInt64(f.v)), nil
	}
	if f.wire != 2 {
		return nil, errPB
	}
	for k := 0; k < len(f.b); {
		v, j, err := pbVarint(f.b, k)
		if err != nil {
			return nil, err
		}
		if len(dst) >= maxInts {
			return nil, errPB
		}
		dst = append(dst, toInt64(v))
		k = j
	}
	return dst, nil
}

// onnxNode — узел графа: операция, имена входов и выходов, целые и строковые атрибуты.
type onnxNode struct {
	op      string
	in, out []string
	ints    map[string][]int64
	strs    map[string]string
}

// onnxTensor — тензор весов: размерности и значения (float32 или int64).
type onnxTensor struct {
	dims []int64
	f    []float32
	i    []int64
}

// onnxGraph — разобранный граф модели.
type onnxGraph struct {
	nodes   []onnxNode
	tensors map[string]*onnxTensor
	outputs []string
}

// parseONNX разбирает файл модели: ModelProto.graph (поле 7) → узлы (1), веса (5), выходы (12).
func parseONNX(model []byte) (*onnxGraph, error) {
	if len(model) == 0 || len(model) > maxModelBytes {
		return nil, fmt.Errorf("%w: размер %d", errPB, len(model))
	}
	top, err := pbFields(model)
	if err != nil {
		return nil, err
	}
	var graph []byte
	for _, f := range top {
		if f.num == 7 && f.wire == 2 {
			graph = f.b
		}
	}
	if graph == nil {
		return nil, fmt.Errorf("%w: нет графа", errPB)
	}
	fs, err := pbFields(graph)
	if err != nil {
		return nil, err
	}
	g := &onnxGraph{tensors: map[string]*onnxTensor{}}
	for _, f := range fs {
		if f.wire != 2 {
			continue
		}
		switch f.num {
		case 1:
			if len(g.nodes) >= maxNodes {
				return nil, fmt.Errorf("%w: узлов больше %d", errPB, maxNodes)
			}
			n, err := parseNode(f.b)
			if err != nil {
				return nil, err
			}
			g.nodes = append(g.nodes, n)
		case 5:
			name, t, err := parseTensor(f.b)
			if err != nil {
				return nil, err
			}
			g.tensors[name] = t
		case 12:
			name, err := parseValueName(f.b)
			if err != nil {
				return nil, err
			}
			g.outputs = append(g.outputs, name)
		}
	}
	return g, nil
}

// pbName — строковое поле с пределом длины.
func pbName(f pbField) (string, error) {
	if f.wire != 2 || len(f.b) > maxNameBytes {
		return "", errPB
	}
	return string(f.b), nil
}

// parseValueName — имя из ValueInfoProto (поле 1).
func parseValueName(b []byte) (string, error) {
	fs, err := pbFields(b)
	if err != nil {
		return "", err
	}
	for _, f := range fs {
		if f.num == 1 {
			return pbName(f)
		}
	}
	return "", errPB
}

// parseNode разбирает NodeProto: input 1, output 2, op_type 4, attribute 5.
func parseNode(b []byte) (onnxNode, error) {
	n := onnxNode{ints: map[string][]int64{}, strs: map[string]string{}}
	fs, err := pbFields(b)
	if err != nil {
		return n, err
	}
	for _, f := range fs {
		switch f.num {
		case 1, 2, 4:
			s, err := pbName(f)
			if err != nil {
				return n, err
			}
			switch f.num {
			case 1:
				n.in = append(n.in, s)
			case 2:
				n.out = append(n.out, s)
			default:
				n.op = s
			}
		case 5:
			if f.wire != 2 {
				return n, errPB
			}
			if err := parseAttr(f.b, &n); err != nil {
				return n, err
			}
		}
	}
	return n, nil
}

// parseAttr разбирает AttributeProto: name 1, i 3, s 4, ints 8. Остальное модели не нужно.
func parseAttr(b []byte, n *onnxNode) error {
	fs, err := pbFields(b)
	if err != nil {
		return err
	}
	var name string
	var ints []int64
	var str string
	for _, f := range fs {
		switch f.num {
		case 1:
			if name, err = pbName(f); err != nil {
				return err
			}
		case 3:
			if f.wire != 0 {
				return errPB
			}
			ints = []int64{toInt64(f.v)}
		case 4:
			if str, err = pbName(f); err != nil {
				return err
			}
		case 8:
			if ints, err = pbInts(f, ints); err != nil {
				return err
			}
		}
	}
	if name == "" {
		return errPB
	}
	n.ints[name] = ints
	n.strs[name] = str
	return nil
}

// parseTensor разбирает TensorProto: dims 1, data_type 2, float_data 4, int64_data 7, name 8, raw_data 9.
// Размер значений сверяется с размерностями до выделения памяти.
func parseTensor(b []byte) (string, *onnxTensor, error) {
	fs, err := pbFields(b)
	if err != nil {
		return "", nil, err
	}
	t := &onnxTensor{}
	var name string
	var dt uint64
	var raw []byte
	for _, f := range fs {
		switch f.num {
		case 1:
			if t.dims, err = pbInts(f, t.dims); err != nil || len(t.dims) > maxDims {
				return "", nil, errPB
			}
		case 2:
			dt = f.v
		case 8:
			if name, err = pbName(f); err != nil {
				return "", nil, err
			}
		case 9:
			raw = f.b
		}
	}
	count := int64(1)
	for _, d := range t.dims {
		if d < 0 || d > maxTensorElem {
			return "", nil, errPB
		}
		count *= d
		if count > maxTensorElem {
			return "", nil, errPB
		}
	}
	switch dt {
	case 1: // float32
		if int64(len(raw)) != 4*count {
			return "", nil, fmt.Errorf("%w: тензор %q", errPB, name)
		}
		t.f = make([]float32, count)
		for i := range t.f {
			t.f[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:]))
		}
	case 7: // int64 — формы и масштабы
		if int64(len(raw)) != 8*count {
			return "", nil, fmt.Errorf("%w: тензор %q", errPB, name)
		}
		t.i = make([]int64, count)
		for i := range t.i {
			t.i[i] = toInt64(binary.LittleEndian.Uint64(raw[8*i:]))
		}
	default:
		return "", nil, fmt.Errorf("%w: тип данных %d у %q", errPB, dt, name)
	}
	return name, t, nil
}
