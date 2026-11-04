package faces

import (
	"errors"
	"fmt"
)

// tensor — CHW, пакет всегда один.
type tensor struct {
	c, h, w int
	d       []float32
}

// opKind — операции сети; Transpose+Reshape(+Sigmoid) голов слиты в одну (opHead), Relu — в Conv.
type opKind uint8

const (
	opConv opKind = iota
	opMaxPool
	opResize
	opAdd
	opHead
)

// step — один шаг вычисления над значениями (номера в vals).
type step struct {
	op      opKind
	in      [2]int
	out     int
	conv    *convSpec
	headC   int  // opHead: каналов в выходе (последняя размерность)
	sigmoid bool // opHead
	name    string
}

// net — скомпилированный граф: шаги по порядку, число потребителей каждого значения.
// После сборки только читается, поэтому один экземпляр делят все горутины.
type net struct {
	steps []step
	nvals int
	input int
	uses  []int
	heads []string // имена выходов графа
}

// headOut — выход головы: n позиций по c каналов (раскладка [1, n, c] как у onnxruntime).
type headOut struct {
	n, c int
	d    []float32
}

var errNet = errors.New("faces: неподдерживаемый граф модели")

// compileNet строит программу по графу. Любое отклонение от того, что умеет вывод
// (операция, атрибут, форма), — ошибка: модель вшита, новая — новая спека.
func compileNet(g *onnxGraph) (*net, error) {
	b := &builder{g: g, ids: map[string]int{}, skip: map[int]bool{}, cons: map[string][]int{}}
	for i, nd := range g.nodes {
		for _, in := range nd.in {
			b.cons[in] = append(b.cons[in], i)
		}
	}
	n := &net{}
	b.n = n
	n.input = b.id("input")
	for i, nd := range g.nodes {
		if b.skip[i] {
			continue
		}
		if err := b.node(nd); err != nil {
			return nil, err
		}
	}
	n.nvals = b.count
	n.uses = make([]int, n.nvals)
	for _, s := range n.steps {
		n.uses[s.in[0]]++
		if s.op == opAdd {
			n.uses[s.in[1]]++
		}
	}
	for _, o := range g.outputs {
		found := false
		for _, s := range n.steps {
			if s.op == opHead && s.name == o {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: нет выхода %q", errNet, o)
		}
		n.heads = append(n.heads, o)
	}
	return n, nil
}

type builder struct {
	g    *onnxGraph
	n    *net
	ids  map[string]int
	skip map[int]bool     // узлы, слитые в предыдущие
	cons map[string][]int // имя → индексы узлов-потребителей
	// count — число выданных номеров значений: у слитого Relu имя другое, а номер тот же
	count int
}

func (b *builder) id(name string) int {
	if v, ok := b.ids[name]; ok {
		return v
	}
	v := b.count
	b.count++
	b.ids[name] = v
	return v
}

// node добавляет шаг для узла nd.
func (b *builder) node(nd onnxNode) error {
	switch nd.op {
	case "Conv":
		return b.conv(nd)
	case "Relu":
		return b.relu(nd)
	case "MaxPool":
		return b.maxPool(nd)
	case "Resize":
		return b.resize(nd)
	case "Add":
		if len(nd.in) != 2 || len(nd.out) != 1 {
			return fmt.Errorf("%w: Add", errNet)
		}
		b.n.steps = append(b.n.steps, step{op: opAdd, in: [2]int{b.id(nd.in[0]), b.id(nd.in[1])}, out: b.id(nd.out[0])})
		return nil
	case "Transpose":
		return b.head(nd)
	}
	return fmt.Errorf("%w: операция %q", errNet, nd.op)
}

// maxPool — только 2×2 с шагом 2 без отступов.
func (b *builder) maxPool(nd onnxNode) error {
	if len(nd.in) != 1 || len(nd.out) != 1 || !intsAre(nd.ints["kernel_shape"], 2, 2) ||
		!intsAre(nd.ints["strides"], 2, 2) || !allZero(nd.ints["pads"]) {
		return fmt.Errorf("%w: MaxPool", errNet)
	}
	b.n.steps = append(b.n.steps, step{op: opMaxPool, in: [2]int{b.id(nd.in[0])}, out: b.id(nd.out[0])})
	return nil
}

// resize — только ближайший сосед в 2 раза по высоте и ширине: масштабы лежат в весах (третий вход).
func (b *builder) resize(nd onnxNode) error {
	if len(nd.in) != 3 || len(nd.out) != 1 {
		return fmt.Errorf("%w: Resize", errNet)
	}
	sc := b.g.tensors[nd.in[2]]
	if m := nd.strs["mode"]; m != "" && m != "nearest" {
		return fmt.Errorf("%w: Resize mode %q", errNet, m)
	}
	if sc == nil || len(sc.f) != 4 || sc.f[0] != 1 || sc.f[1] != 1 || sc.f[2] != 2 || sc.f[3] != 2 {
		return fmt.Errorf("%w: Resize масштабы", errNet)
	}
	b.n.steps = append(b.n.steps, step{op: opResize, in: [2]int{b.id(nd.in[0])}, out: b.id(nd.out[0])})
	return nil
}

// relu сливается в предыдущую свёртку: её выход нужен только этому Relu, поэтому лишнего прохода
// и второго тензора нет.
func (b *builder) relu(nd onnxNode) error {
	if len(nd.in) != 1 || len(nd.out) != 1 || len(b.cons[nd.in[0]]) != 1 || len(b.n.steps) == 0 {
		return fmt.Errorf("%w: Relu", errNet)
	}
	last := &b.n.steps[len(b.n.steps)-1]
	src, ok := b.ids[nd.in[0]]
	if !ok || last.op != opConv || last.out != src || last.conv.relu {
		return fmt.Errorf("%w: Relu не после свёртки", errNet)
	}
	last.conv.relu = true
	b.ids[nd.out[0]] = src
	return nil
}

// head сливает цепочку Transpose [0 2 3 1] → Reshape [1,-1,C] → (Sigmoid) в один шаг.
func (b *builder) head(nd onnxNode) error {
	if len(nd.in) != 1 || len(nd.out) != 1 || !intsAre(nd.ints["perm"], 0, 2, 3, 1) {
		return fmt.Errorf("%w: Transpose", errNet)
	}
	ri, ok := b.only(nd.out[0], "Reshape")
	if !ok {
		return fmt.Errorf("%w: после Transpose нет Reshape", errNet)
	}
	rs := b.g.nodes[ri]
	if len(rs.in) != 2 || len(rs.out) != 1 {
		return fmt.Errorf("%w: Reshape", errNet)
	}
	shape := b.g.tensors[rs.in[1]]
	if shape == nil || len(shape.i) != 3 || shape.i[0] != 1 || shape.i[2] <= 0 || shape.i[2] > 64 {
		return fmt.Errorf("%w: Reshape форма", errNet)
	}
	st := step{op: opHead, in: [2]int{b.id(nd.in[0])}, headC: int(shape.i[2]), name: rs.out[0]}
	b.skip[ri] = true
	if si, ok := b.only(rs.out[0], "Sigmoid"); ok {
		b.skip[si] = true
		st.sigmoid = true
		st.name = b.g.nodes[si].out[0]
	}
	st.out = b.id(st.name)
	b.n.steps = append(b.n.steps, st)
	return nil
}

// only — единственный потребитель имени, если это узел нужной операции.
func (b *builder) only(name, op string) (int, bool) {
	c := b.cons[name]
	if len(c) != 1 || b.g.nodes[c[0]].op != op || len(b.g.nodes[c[0]].out) != 1 {
		return 0, false
	}
	return c[0], true
}

func intsAre(v []int64, want ...int64) bool {
	if len(v) != len(want) {
		return false
	}
	for i := range v {
		if v[i] != want[i] {
			return false
		}
	}
	return true
}

func allZero(v []int64) bool {
	for _, x := range v {
		if x != 0 {
			return false
		}
	}
	return true
}
