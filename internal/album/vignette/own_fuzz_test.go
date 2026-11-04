package vignette

import (
	"math"
	"testing"
)

// fuzzFrac — доля из байта: 0…1, а три крайних значения — NaN, +Inf и отрицательная.
func fuzzFrac(b byte) float64 {
	switch b {
	case 255:
		return math.NaN()
	case 254:
		return math.Inf(1)
	case 253:
		return -0.5
	}
	return float64(b) / 250
}

// fuzzShape — шаблон из байтов: по 5 на место (x, y, w, h, роль), не больше 64 мест;
// первый байт — шапка (чётный — есть), второй — страниц (бит 0: лист или разворот) и известен
// ли размер области (бит 1: точный режим по размеру).
func fuzzShape(data []byte) (Shape, int) {
	var sh Shape
	pages := 1
	if len(data) >= 2 {
		if data[0]%2 == 0 {
			sh.Head = &Rect{0.05, 0.02, 0.9, fuzzFrac(data[0]) / 4}
		}
		pages += int(data[1] % 2)
		data = data[2:]
	}
	roles := [...]string{RolePupil, RoleTeacher, RoleGroup, "чужая"}
	for len(data) >= 5 && len(sh.Boxes) < 64 {
		sh.Boxes = append(sh.Boxes, Rect{fuzzFrac(data[0]), fuzzFrac(data[1]), fuzzFrac(data[2]), fuzzFrac(data[3])})
		sh.Roles = append(sh.Roles, roles[data[4]%4])
		data = data[5:]
	}
	return sh, pages
}

// FuzzBuildOwn — BuildOwn на мусорном шаблоне и входе (файл шаблонов недоверенный, зажим
// мог пропустить край): без паники, людей ровно столько, сколько просили, рамки конечные.
func FuzzBuildOwn(f *testing.F) {
	f.Add([]byte{0, 0, 10, 10, 50, 60, 1, 70, 10, 50, 60, 0, 10, 100, 200, 100, 2}, 1, 24, 204.0, 304.0, 0.75, 0.75, 0.67)
	f.Add([]byte{1, 1, 255, 0, 10, 10, 0, 0, 254, 10, 10, 1, 253, 253, 0, 0, 2}, 3, 0, 600.0, 300.0, 0.0, 1.0, 2.0)
	f.Add([]byte{}, 0, 64, 0.0, -1.0, math.NaN(), 0.66, math.Inf(1))
	f.Add([]byte{2, 0, 0, 0, 250, 250, 2, 0, 0, 1, 1, 0}, 18, 64, 1e9, 1e-9, 5.0, 0.2, 0.0)
	f.Add([]byte{0, 2, 10, 10, 50, 60, 1, 10, 10, 50, 60, 0, 255, 0, 10, 10, 0, 10, 100, 200, 100, 2}, 1, 1, 204.0, 304.0, 0.75, 0.75, 0.67)
	f.Fuzz(func(t *testing.T, data []byte, teachers, pupils int, w, h, ratio, shRatio, aspect float64) {
		teachers, pupils = teachers%19, pupils%65 // пределы модели: больше BuildOwn не просят
		sh, pages := fuzzShape(data)
		sh.Ratio, sh.Aspect = shRatio, aspect
		if len(data) >= 2 && data[1]&2 != 0 {
			sh.W, sh.H = w, h
		}
		area := Rect{0, 0, w, h}
		rs := []Rect{{10, 10, w - 20, h - 20}}
		if pages == 2 {
			rs = []Rect{{10, 10, w/2 - 20, h - 20}, {w/2 + 10, 10, w/2 - 20, h - 20}}
		}
		in := Input{Regions: rs, Gap: 3, Pad: 1, Teachers: teachers, Pupils: pupils, Ratio: ratio, HS: 8, HST: 11}
		p := BuildOwn(sh, area, in)
		n := map[string]int{}
		for i, s := range p.Slots {
			b := s.Box
			for _, v := range [...]float64{b.X, b.Y, b.W, b.H} {
				if math.IsNaN(v) || math.IsInf(v, 0) {
					t.Fatalf("слот %d (%s) не конечный: %+v", i, s.Role, b)
				}
			}
			if b.W < 0 || b.H < 0 {
				t.Fatalf("слот %d (%s) с отрицательной стороной: %+v", i, s.Role, b)
			}
			n[s.Role]++
		}
		if n[RoleTeacher] != max(teachers, 0) || n[RolePupil] != max(pupils, 0) || n[RoleGroup] > len(sh.Boxes) {
			t.Fatalf("слотов %v, просили учителей %d, учеников %d (мест шаблона %d)", n, teachers, pupils, len(sh.Boxes))
		}
	})
}
