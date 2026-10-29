package svgcut

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

// Box — прямоугольник в координатах листа (viewBox).
type Box struct{ X0, Y0, X1, Y1 float64 }

// W, H — размеры рамки.
func (b Box) W() float64 { return b.X1 - b.X0 }
func (b Box) H() float64 { return b.Y1 - b.Y0 }

// Center — центр рамки.
func (b Box) Center() (x, y float64) { return (b.X0 + b.X1) / 2, (b.Y0 + b.Y1) / 2 }

// Union — наименьшая рамка, охватывающая обе.
func (b Box) Union(o Box) Box {
	return Box{math.Min(b.X0, o.X0), math.Min(b.Y0, o.Y0), math.Max(b.X1, o.X1), math.Max(b.Y1, o.Y1)}
}

// Near — рамки пересекаются или ближе друг к другу, чем gap по каждой оси.
func (b Box) Near(o Box, gap float64) bool {
	return b.X0 <= o.X1+gap && o.X0 <= b.X1+gap && b.Y0 <= o.Y1+gap && o.Y0 <= b.Y1+gap
}

// Contains — точка внутри рамки (границы включены).
func (b Box) Contains(x, y float64) bool { return x >= b.X0 && x <= b.X1 && y >= b.Y0 && y <= b.Y1 }

// Arr — рамка массивом x0, y0, x1, y1 (для отчёта).
func (b Box) Arr() [4]float64 { return [4]float64{b.X0, b.Y0, b.X1, b.Y1} }

// maxCoord — предел модуля числа в d: больше — разбор не доверяет файлу (и float64 не переполнится).
const maxCoord = 1e7

// pathInfo — то, что нужно знать о контуре: рамка и «прямоугольность» (лист ищется по форме).
type pathInfo struct {
	box  Box
	rect bool // один замкнутый осевой прямоугольник из M и L
}

// pathState — состояние разбора d: текущая команда и точка, рамка, точки единственного подпути.
type pathState struct {
	s          pathScanner
	info       pathInfo
	cmd        byte
	cur, start [2]float64
	have, open bool // был M; подпуть открыт (после z — закрыт)
	started    bool // рамка уже имеет хотя бы одну точку
	curved     bool // были кривые
	pts        [][2]float64
	subpaths   int
}

func (st *pathState) extend(x, y float64) {
	b := &st.info.box
	if !st.started {
		*b, st.started = Box{x, y, x, y}, true
		return
	}
	b.X0, b.X1 = math.Min(b.X0, x), math.Max(b.X1, x)
	b.Y0, b.Y1 = math.Min(b.Y0, y), math.Max(b.Y1, y)
}

// command принимает букву команды.
func (st *pathState) command(c byte) error {
	switch c {
	case 'M', 'L', 'C':
		if c != 'M' && !st.open {
			return fmt.Errorf("команда %c вне подпути (после z нужна M)", c)
		}
		st.cmd = c
	case 'z', 'Z':
		if !st.open {
			return errors.New("z без подпути")
		}
		st.cur, st.open, st.cmd = st.start, false, 0
	default:
		return fmt.Errorf("команда %q не поддерживается (нужны абсолютные M L C и z)", string(c))
	}
	return nil
}

// args читает аргументы текущей команды (повтор без буквы допустим) и применяет их.
func (st *pathState) args() error {
	if st.cmd == 0 {
		return errors.New("число без команды")
	}
	n := 2
	if st.cmd == 'C' {
		n = 6
	}
	var a [6]float64
	for k := 0; k < n; k++ {
		st.s.skip()
		v, err := st.s.number()
		if err != nil {
			return err
		}
		a[k] = v
	}
	switch st.cmd {
	case 'M':
		st.cur, st.start, st.have, st.open = [2]float64{a[0], a[1]}, [2]float64{a[0], a[1]}, true, true
		st.subpaths++
		st.pts = append(st.pts[:0], st.cur)
		st.extend(a[0], a[1])
		st.cmd = 'L' // следующие пары после M — неявные L
	case 'L':
		st.cur = [2]float64{a[0], a[1]}
		st.extend(a[0], a[1])
		if len(st.pts) < 8 {
			st.pts = append(st.pts, st.cur)
		}
	case 'C':
		st.curved = true
		p3 := [2]float64{a[4], a[5]}
		st.extend(a[4], a[5])
		cubicExtremes(st.cur, [2]float64{a[0], a[1]}, [2]float64{a[2], a[3]}, p3, st.extend)
		st.cur = p3
	}
	return nil
}

// scanPath разбирает d и считает точную рамку по кривым Безье. Грамматика Recraft: абсолютные M L C
// и z/Z (строчная z — у всех путей школы); относительные команды, дуги и прочее — отказ, а не
// молчаливо неверная геометрия.
func scanPath(d string) (pathInfo, error) {
	st := &pathState{s: pathScanner{s: d}}
	for {
		st.s.skip()
		if st.s.eof() {
			break
		}
		var err error
		if c := st.s.s[st.s.i]; isLetter(c) {
			st.s.i++
			err = st.command(c)
		} else {
			err = st.args()
		}
		if err != nil {
			return st.info, err
		}
	}
	if !st.have {
		return st.info, errors.New("пустой путь")
	}
	st.info.rect = st.subpaths == 1 && !st.curved && isRect(st.pts)
	return st.info, nil
}

// isRect — точки образуют осевой прямоугольник (4 вершины, пятая может замыкать).
func isRect(p [][2]float64) bool {
	if len(p) == 5 && p[4] == p[0] {
		p = p[:4]
	}
	if len(p) != 4 {
		return false
	}
	for i := range p {
		a, b := p[i], p[(i+1)%4]
		if a[0] != b[0] && a[1] != b[1] {
			return false
		}
	}
	return true
}

// cubicExtremes добавляет экстремумы кубической кривой по каждой оси (корни производной).
func cubicExtremes(p0, p1, p2, p3 [2]float64, extend func(x, y float64)) {
	for axis := 0; axis < 2; axis++ {
		a0, b0, c0 := p1[axis]-p0[axis], p2[axis]-p1[axis], p3[axis]-p2[axis]
		// B'(t)/3 = a·t² + b·t + c
		a, b, c := a0-2*b0+c0, 2*(b0-a0), a0
		var roots []float64
		if math.Abs(a) < 1e-12 {
			if math.Abs(b) > 1e-12 {
				roots = append(roots, -c/b)
			}
		} else if disc := b*b - 4*a*c; disc >= 0 {
			sq := math.Sqrt(disc)
			roots = append(roots, (-b+sq)/(2*a), (-b-sq)/(2*a))
		}
		for _, t := range roots {
			if t <= 0 || t >= 1 {
				continue
			}
			u := 1 - t
			var pt [2]float64
			for k := 0; k < 2; k++ {
				pt[k] = u*u*u*p0[k] + 3*u*u*t*p1[k] + 3*u*t*t*p2[k] + t*t*t*p3[k]
			}
			extend(pt[0], pt[1])
		}
	}
}

// pathScanner — разбор чисел и команд d.
type pathScanner struct {
	s string
	i int
}

func (p *pathScanner) eof() bool { return p.i >= len(p.s) }

func (p *pathScanner) skip() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\n', '\r', ',':
			p.i++
		default:
			return
		}
	}
}

func isLetter(c byte) bool { return (c|0x20) >= 'a' && (c|0x20) <= 'z' }

func (p *pathScanner) digits() int {
	n := 0
	for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
		p.i++
		n++
	}
	return n
}

// number читает число SVG (знак, дробь, порядок); бесконечность, NaN и |v| > maxCoord — отказ.
func (p *pathScanner) number() (float64, error) {
	start := p.i
	if p.i < len(p.s) && (p.s[p.i] == '-' || p.s[p.i] == '+') {
		p.i++
	}
	n := p.digits()
	if p.i < len(p.s) && p.s[p.i] == '.' {
		p.i++
		n += p.digits()
	}
	if n == 0 {
		return 0, fmt.Errorf("ожидалось число в позиции %d", start)
	}
	if p.i < len(p.s) && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
		p.i++
		if p.i < len(p.s) && (p.s[p.i] == '-' || p.s[p.i] == '+') {
			p.i++
		}
		if p.digits() == 0 {
			return 0, fmt.Errorf("порядок числа без цифр в позиции %d", start)
		}
	}
	v, err := strconv.ParseFloat(p.s[start:p.i], 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > maxCoord {
		return 0, fmt.Errorf("число %q вне предела", p.s[start:p.i])
	}
	return v, nil
}
