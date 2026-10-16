package gen

import (
	"image/color"
	"math"
)

// Стикеры программы (Ф2, research §2). Рисуются заливками (x/image/vector не умеет обводку) в
// «дизайн-пространстве»: высота рамки — 1, ширина — её пропорция a, поэтому круг остаётся кругом
// при любой рамке. У растяжимых (Stretch) a — пропорция рамки в модели, у остальных — Meta.Aspect.
// Выпущенный стикер не меняется (эталоны golden.json); правка — новый ключ.

// art — генератор стикера: fn рисует в пространстве (0..a)×(0..1), a — пропорция рамки.
func art(meta Meta, fn func(c *canvas, s Spec, ink []color.NRGBA, a float64)) *gen {
	return &gen{meta: meta, art: true, draw: func(c *canvas, s Spec) {
		a := meta.Aspect
		if meta.Stretch {
			a = s.Aspect
		}
		if !(a > 0) {
			a = 1
		}
		// Рамка (0..1)² растянута до a:1 — перевод дизайн-координат (u, v) в доли: x = u/a.
		c.m[0], c.m[3] = c.m[0]/a, c.m[3]/a
		c.w, c.h = a, 1
		fn(c, s, inks(s), a)
	}}
}

var artGens = []*gen{
	art(Meta{Kind: "star", Name: "Звезда", Inks: []string{"#f2c14e"}, Aspect: 1}, drawStar),
	art(Meta{Kind: "burst", Name: "Взрыв", Inks: []string{"#f06a5b", "#ffd166"}, Aspect: 1}, drawBurst),
	art(Meta{Kind: "sparkle", Name: "Искра", Inks: []string{"#f2c14e", "#f9e2a0"}, Aspect: 1}, drawSparkle),
	art(Meta{Kind: "heart", Name: "Сердце", Inks: []string{"#e8607a", "#f7b6c2"}, Aspect: 1}, drawHeart),
	art(Meta{Kind: "hearts-pair", Name: "Пара сердец", Inks: []string{"#e8607a", "#f2a0b4"}, Aspect: 1.2}, drawHeartsPair),
	art(Meta{Kind: "circle", Name: "Круг", Inks: []string{"#e9c8b8"}, Aspect: 1}, drawCircle),
	art(Meta{Kind: "blob", Name: "Пятно", Inks: []string{"#cfe0d6"}, Aspect: 1.2, Random: true}, drawBlob),
	art(Meta{Kind: "plate", Name: "Плашка", Inks: []string{"#fffaf0", "#d9b779"}, Aspect: 3, Stretch: true}, drawPlate),
	art(Meta{Kind: "banner", Name: "Лента-баннер", Inks: []string{"#e8607a", "#b8435a"}, Aspect: 3}, drawBanner),
	art(Meta{Kind: "wave-band", Name: "Волны", Inks: []string{"#d3e6f1", "#a9cce1", "#7fb2d1"}, Aspect: 4, Stretch: true}, drawWaveBand),
	art(Meta{Kind: "rainbow", Name: "Радуга", Inks: []string{"#ef7b7b", "#f6c177", "#9ccc9c", "#8fb8e0"}, Aspect: 2}, drawRainbow),
	art(Meta{Kind: "arches", Name: "Бохо-арки", Inks: []string{"#e7c9b4", "#d4a98f", "#b98466"}, Aspect: 0.85}, drawArches),
	art(Meta{Kind: "corner", Name: "Уголок для фото", Inks: []string{"#c9b79c"}, Aspect: 1}, drawCorner),
	art(Meta{Kind: "brush-stroke", Name: "Мазок", Inks: []string{"#f2c6b4"}, Aspect: 3, Random: true}, drawStroke),
	art(Meta{Kind: "arrow", Name: "Стрелка", Inks: []string{"#6b6b6b"}, Aspect: 2}, drawArrow),
	art(Meta{Kind: "confetti-pile", Name: "Горсть конфетти", Inks: []string{"#f2b134", "#e85d75", "#4fb0c6", "#7cc47f"}, Aspect: 1, Random: true}, drawConfettiPile),
	art(Meta{Kind: "flower", Name: "Цветок", Inks: []string{"#f09bb0", "#f6d365"}, Aspect: 1}, drawFlower),
	art(Meta{Kind: "daisy", Name: "Ромашка", Inks: []string{"#f8ecd8", "#f6c445"}, Aspect: 1}, drawDaisy),
	art(Meta{Kind: "leaf", Name: "Лист", Inks: []string{"#7fb685", "#4f8f5f"}, Aspect: 0.6}, drawLeaf),
	art(Meta{Kind: "branch", Name: "Ветка", Inks: []string{"#7a8f6b", "#a8c69f"}, Aspect: 0.7}, drawBranch),
	art(Meta{Kind: "sun", Name: "Солнце", Inks: []string{"#f6b93b", "#f9d77e"}, Aspect: 1}, drawSun),
	art(Meta{Kind: "cloud", Name: "Облако", Inks: []string{"#cfe3f2", "#e6f0f8"}, Aspect: 1.6}, drawCloud),
	art(Meta{Kind: "balloon", Name: "Шар", Inks: []string{"#e85d75", "#9a9a9a"}, Aspect: 0.8}, drawBalloon),
	art(Meta{Kind: "balloons", Name: "Связка шаров", Inks: []string{"#e85d75", "#f2b134", "#4fb0c6"}, Aspect: 1}, drawBalloons),
	art(Meta{Kind: "cap", Name: "Шапка выпускника", Inks: []string{"#2f3a56", "#e0a93d"}, Aspect: 1.4}, drawCap),
	art(Meta{Kind: "pencil", Name: "Карандаш", Inks: []string{"#f2b134", "#f0d3a8", "#3d3d3d"}, Aspect: 4}, drawPencil),
	art(Meta{Kind: "book", Name: "Книга", Inks: []string{"#fffdf6", "#6b8fb8"}, Aspect: 1.4}, drawBook),
	art(Meta{Kind: "apple", Name: "Яблоко", Inks: []string{"#e24b4b", "#5f9e5f"}, Aspect: 0.95}, drawApple),
	art(Meta{Kind: "bell", Name: "Колокольчик", Inks: []string{"#e0b04a", "#8a6a2a"}, Aspect: 0.95}, drawBell),
	art(Meta{Kind: "rings", Name: "Кольца", Inks: []string{"#d9b779", "#c9a24e"}, Aspect: 1.5}, drawRings),
}

// --- общие построения ---

// spikes — звезда с n лучами: вершины радиуса ro, впадины ri; первый луч вверх при rot 0.
func spikes(x, y, ro, ri float64, n int, rot float64) []pt {
	ps := make([]pt, 2*n)
	for i := range ps {
		r := ro
		if i%2 == 1 {
			r = ri
		}
		sn, cs := math.Sincos(rot - math.Pi/2 + math.Pi*float64(i)/float64(n))
		ps[i] = pt{x + r*cs, y + r*sn}
	}
	return ps
}

// smooth — замкнутая гладкая кривая через точки (Катмулл—Ром в кубических Безье).
func smooth(c *canvas, ps []pt) {
	n := len(ps)
	if n < 3 {
		return
	}
	c.moveTo(ps[0].x, ps[0].y)
	for i := range n {
		p0, p1, p2, p3 := ps[(i+n-1)%n], ps[i], ps[(i+1)%n], ps[(i+2)%n]
		c.cubeTo(p1.x+(p2.x-p0.x)/6, p1.y+(p2.y-p0.y)/6, p2.x-(p3.x-p1.x)/6, p2.y-(p3.y-p1.y)/6, p2.x, p2.y)
	}
	c.closePath()
}

// orient — многоугольник с нужным направлением обхода: у отверстия оно обратное внешнему.
func orient(ps []pt, positive bool) []pt {
	var a float64
	for i, p := range ps {
		q := ps[(i+1)%len(ps)]
		a += p.x*q.y - q.x*p.y
	}
	if (a >= 0) == positive {
		return ps
	}
	rev := make([]pt, len(ps))
	for i, p := range ps {
		rev[len(ps)-1-i] = p
	}
	return rev
}

// rrect — скруглённый прямоугольник (радиус зажат полу-меньшей стороной), по часовой.
func rrect(x0, y0, x1, y1, r float64) []pt {
	r = max(0, min(r, (x1-x0)/2, (y1-y0)/2))
	var ps []pt
	for _, k := range [4][3]float64{{x0 + r, y0 + r, 180}, {x1 - r, y0 + r, 270}, {x1 - r, y1 - r, 0}, {x0 + r, y1 - r, 90}} {
		ps = append(ps, arc(k[0], k[1], r, k[2], k[2]+90, 8)...)
	}
	return ps
}

// band — часть кольца между радиусами r0 < r1 от угла a0 до a1 (градусы).
func band(c *canvas, cx, cy, r0, r1, a0, a1 float64) {
	n := max(8, int(math.Abs(a1-a0)/3))
	outer := arc(cx, cy, r1, a0, a1, n)
	inner := arc(cx, cy, r0, a1, a0, n)
	c.polyPos(append(outer, inner...))
}

// bezier — n+1 точка кубической кривой.
func bezier(p0, p1, p2, p3 pt, n int) []pt {
	ps := make([]pt, n+1)
	for i := range ps {
		t := float64(i) / float64(n)
		u := 1 - t
		ps[i] = pt{
			u*u*u*p0.x + 3*u*u*t*p1.x + 3*u*t*t*p2.x + t*t*t*p3.x,
			u*u*u*p0.y + 3*u*u*t*p1.y + 3*u*t*t*p2.y + t*t*t*p3.y,
		}
	}
	return ps
}

// leafAt — заострённый лист длины l и ширины w: основание (x, y), направление rot (рад).
func leafAt(c *canvas, x, y, l, w, rot float64) {
	sn, cs := math.Sincos(rot)
	at := func(u, v float64) (float64, float64) { return x + u*cs - v*sn, y + u*sn + v*cs }
	c.moveTo(x, y)
	x1, y1 := at(0.25*l, -w)
	x2, y2 := at(0.7*l, -0.8*w)
	x3, y3 := at(l, 0)
	c.cubeTo(x1, y1, x2, y2, x3, y3)
	x1, y1 = at(0.7*l, 0.8*w)
	x2, y2 = at(0.25*l, w)
	c.cubeTo(x1, y1, x2, y2, x, y)
	c.closePath()
}

// heartAt кладёт сердце шириной ≈ 0,68·size так, что середина его высоты — в (x, y).
func heartAt(c *canvas, x, y, size, rot float64) {
	sn, cs := math.Sincos(rot)
	dy := 0.069 * size
	heart(c, x+dy*sn, y-dy*cs, size, rot)
}

// --- фигуры ---

func drawStar(c *canvas, _ Spec, ink []color.NRGBA, a float64) {
	c.poly(spikes(a/2, 0.54, 0.49, 0.2, 5, 0))
	c.fill(ink[0], 1)
}

func drawBurst(c *canvas, _ Spec, ink []color.NRGBA, a float64) {
	c.poly(spikes(a/2, 0.5, 0.49, 0.37, 14, 0))
	c.fill(ink[0], 1)
	c.circle(a/2, 0.5, 0.27)
	c.fill(ink[1], 1)
}

func drawSparkle(c *canvas, _ Spec, ink []color.NRGBA, a float64) {
	four := func(x, y, r float64) {
		c.moveTo(x, y-r)
		k := 0.24 * r
		for i := range 4 {
			// Дуга к следующему лучу прогибается к центру: контрольные точки у центра.
			sn, cs := math.Sincos(math.Pi/4 + math.Pi/2*float64(i))
			nx, ny := x+r*math.Sin(math.Pi/2*float64(i+1)), y-r*math.Cos(math.Pi/2*float64(i+1))
			c.cubeTo(x+k*sn, y-k*cs, x+k*sn, y-k*cs, nx, ny)
		}
		c.closePath()
	}
	four(a*0.45, 0.55, 0.42)
	c.fill(ink[0], 1)
	four(a*0.8, 0.2, 0.17)
	c.fill(ink[1], 1)
}

func drawHeart(c *canvas, _ Spec, ink []color.NRGBA, a float64) {
	heartAt(c, a/2, 0.5, 1.42, 0)
	c.fill(ink[0], 1)
	c.moveTo(0.3, 0.36) // блик
	c.cubeTo(0.28, 0.3, 0.34, 0.27, 0.4, 0.3)
	c.cubeTo(0.37, 0.32, 0.34, 0.36, 0.3, 0.36)
	c.closePath()
	c.fill(ink[1], 1)
}

func drawHeartsPair(c *canvas, _ Spec, ink []color.NRGBA, a float64) {
	heartAt(c, a*0.4, 0.58, 1.15, -0.2)
	c.fill(ink[0], 1)
	heartAt(c, a*0.74, 0.34, 0.8, 0.28)
	c.fill(ink[1], 1)
}

func drawCircle(c *canvas, _ Spec, ink []color.NRGBA, a float64) {
	c.circle(a/2, 0.5, 0.47)
	c.fill(ink[0], 1)
}

func drawBlob(c *canvas, s Spec, ink []color.NRGBA, a float64) {
	r := cellRand(s.Seed, s.Kind, 0, 0)
	const n = 9
	ps := make([]pt, n)
	for i := range ps {
		ang := 2*math.Pi*float64(i)/n + r.rangeF(-0.2, 0.2)
		k := r.rangeF(0.74, 0.97)
		sn, cs := math.Sincos(ang)
		ps[i] = pt{a/2 + 0.5*a*0.96*k*cs, 0.5 + 0.5*0.96*k*sn}
	}
	smooth(c, ps)
	c.fill(ink[0], 1)
}

// drawPlate — плашка под текст: скругление от короткой стороны, не растягивается (решение №136).
func drawPlate(c *canvas, _ Spec, ink []color.NRGBA, a float64) {
	m := min(a, 1)
	pad := 0.02 * m
	c.polyPos(rrect(pad, pad, a-pad, 1-pad, 0.22*m))
	c.fill(ink[0], 1)
	in, w := 0.07*m, 0.018*m
	c.poly(orient(rrect(in, in, a-in, 1-in, 0.17*m), true))
	c.poly(orient(rrect(in+w, in+w, a-in-w, 1-in-w, 0.17*m-w), false))
	c.fill(ink[1], 1)
}

func drawBanner(c *canvas, _ Spec, ink []color.NRGBA, a float64) {
	for _, mir := range [2]float64{0, 1} {
		x := func(u float64) float64 { return math.Abs(mir*a - u) }
		c.poly([]pt{{x(0.03), 0.3}, {x(0.5), 0.3}, {x(0.5), 0.95}, {x(0.03), 0.95}, {x(0.19), 0.625}})
		c.fill(ink[1], 1)
	}
	c.poly([]pt{{0.3, 0.1}, {a - 0.3, 0.1}, {a - 0.3, 0.8}, {0.3, 0.8}})
	c.fill(ink[0], 1)
	// Сгибы: тёмные треугольники под лентой у краёв.
	c.poly([]pt{{0.3, 0.8}, {0.5, 0.8}, {0.5, 0.95}})
	c.poly([]pt{{a - 0.3, 0.8}, {a - 0.5, 0.8}, {a - 0.5, 0.95}})
	c.fill(ink[1], 1)
}

// drawWaveBand — 3 волны от заднего слоя к переднему; длина и размах — от короткой стороны.
func drawWaveBand(c *canvas, _ Spec, ink []color.NRGBA, a float64) {
	m := min(a, 1)
	length, amp := 1.1*m, 0.1*m
	step := length / 32
	for i := range 3 {
		base := 0.34 + 0.2*float64(i)
		ph := float64(i) * length * 0.31
		c.moveTo(0, 1)
		for x := 0.0; ; x += step {
			x = min(x, a)
			c.lineTo(x, base+amp*math.Sin(2*math.Pi*(x+ph)/length))
			if x >= a {
				break
			}
		}
		c.lineTo(a, 1)
		c.closePath()
		c.fill(ink[i], 1)
	}
}

func drawRainbow(c *canvas, _ Spec, ink []color.NRGBA, a float64) {
	for i := range 4 {
		r1 := 0.93 - 0.12*float64(i)
		band(c, a/2, 0.96, r1-0.12, r1, 180, 360)
		c.fill(ink[i], 1)
	}
}

func drawArches(c *canvas, _ Spec, ink []color.NRGBA, a float64) {
	for i := range 3 {
		w, h := 0.8-0.24*float64(i), 0.96-0.24*float64(i)
		x0, x1, bot := a/2-w/2, a/2+w/2, 0.98
		ps := []pt{{x0, bot}}
		ps = append(ps, arc(a/2, bot-h+w/2, w/2, 180, 360, 24)...)
		ps = append(ps, pt{x1, bot})
		c.polyPos(ps)
		c.fill(ink[i], 1)
	}
}

func drawCorner(c *canvas, _ Spec, ink []color.NRGBA, _ float64) {
	c.poly([]pt{{0.03, 0.03}, {0.97, 0.03}, {0.03, 0.97}})
	c.poly(orient([]pt{{0.2, 0.2}, {0.55, 0.2}, {0.2, 0.55}}, false))
	c.fill(ink[0], 1)
}

func drawStroke(c *canvas, s Spec, ink []color.NRGBA, a float64) {
	r := cellRand(s.Seed, s.Kind, 0, 0)
	const n = 7
	var top, bot []pt
	for i := range n {
		x := 0.12 + (a-0.24)*float64(i)/(n-1)
		taper := 0.1 + 0.24*math.Sin(math.Pi*(float64(i)+0.5)/n)
		mid := 0.5 + r.rangeF(-0.08, 0.08)
		top = append(top, pt{x, mid - taper - r.rangeF(0, 0.06)})
		bot = append(bot, pt{x, mid + taper + r.rangeF(0, 0.06)})
	}
	ps := append([]pt{{0.02, 0.5}}, top...)
	ps = append(ps, pt{a - 0.02, 0.5})
	for i := n - 1; i >= 0; i-- {
		ps = append(ps, bot[i])
	}
	smooth(c, ps)
	c.fill(ink[0], 1)
}

func drawArrow(c *canvas, _ Spec, ink []color.NRGBA, a float64) {
	p := bezier(pt{0.12, 0.78}, pt{0.5, 0.95}, pt{a - 0.9, 0.1}, pt{a - 0.3, 0.3}, 28)
	c.stroke(p, false, 0.07)
	e := p[len(p)-1]
	d := pt{e.x - p[len(p)-4].x, e.y - p[len(p)-4].y}
	l := math.Hypot(d.x, d.y)
	d = pt{d.x / l, d.y / l}
	nx, ny := -d.y, d.x
	c.poly([]pt{{e.x + d.x*0.2, e.y + d.y*0.2}, {e.x + nx*0.14, e.y + ny*0.14}, {e.x - nx*0.14, e.y - ny*0.14}})
	c.fill(ink[0], 1)
}

func drawConfettiPile(c *canvas, s Spec, ink []color.NRGBA, a float64) {
	r := cellRand(s.Seed, s.Kind, 0, 0)
	for range 46 {
		ang, rad := r.rangeF(0, 2*math.Pi), 0.46*math.Sqrt(r.float())
		sn, cs := math.Sincos(ang)
		x, y := a/2+rad*cs, 0.5+rad*sn
		rot, col := r.rangeF(0, math.Pi), ink[r.intn(4)]
		switch r.intn(3) {
		case 0:
			c.rotRect(x, y, 0.03, 0.09, rot)
		case 1:
			c.circle(x, y, 0.03)
		default:
			c.poly(regular(x, y, 0.05, 3, rot))
		}
		c.fill(col, 1)
	}
}
