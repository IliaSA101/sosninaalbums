package gen

import "github.com/IliaSA101/sosninaalbums/internal/decor/gen/art"

// Набор узоров (research §2) в порядке каталога. Ключ и рисование выпущенного генератора не
// меняются (эталоны testdata/golden.json); новый вид — новый ключ в конце списка.
var patternGens = []*gen{
	{meta: Meta{Kind: "grad-h", Name: "Градиент", Inks: []string{"#f6e7d8", "#e3c4ad"}}, draw: gradH},
	{meta: Meta{Kind: "grad-d", Name: "Диагональный градиент", Inks: []string{"#e6eef6", "#c3d3e6"}}, draw: gradD},
	{meta: Meta{Kind: "glow", Name: "Свечение", Inks: []string{"#fff8ec", "#efd6b0"}}, draw: glow},
	{meta: Meta{Kind: "dots", Name: "Горох", Inks: []string{"#d9b779"}}, draw: dots},
	{meta: Meta{Kind: "dots-scatter", Name: "Россыпь точек", Inks: []string{"#d9b779", "#e8c8bc"}, Random: true}, draw: dotsScatter},
	{meta: Meta{Kind: "stripes", Name: "Полосы", Inks: []string{"#eadccb"}}, draw: stripes},
	{meta: Meta{Kind: "bands", Name: "Широкие полосы", Inks: []string{"#e5d5c1", "#c9a96e"}}, draw: bands},
	{meta: Meta{Kind: "grid-5", Name: "Клетка", Inks: []string{"#b9cbe0"}}, draw: grid5},
	{meta: Meta{Kind: "lines", Name: "Линейка с полем", Inks: []string{"#a9c4e2", "#e07a7a"}}, draw: lines},
	{meta: Meta{Kind: "confetti", Name: "Конфетти", Inks: []string{"#f2b134", "#e85d75", "#4fb0c6", "#7cc47f"}, Random: true}, draw: confetti},
	{meta: Meta{Kind: "stars", Name: "Звёзды", Inks: []string{"#e8c35a", "#f3dfa2"}, Random: true}, draw: stars},
	{meta: Meta{Kind: "hearts", Name: "Сердца", Inks: []string{"#e88aa0", "#f4c2cf"}, Random: true}, draw: hearts},
	{meta: Meta{Kind: "brush", Name: "Мазки", Inks: []string{"#f2c6b4", "#d8a7b1", "#b5c9c3", "#e9d8a6"}, Random: true}, draw: brush},
	{meta: Meta{Kind: "watercolor", Name: "Акварель", Inks: []string{"#efb3ad", "#d9bde0", "#a9c8e0"}, Random: true}, draw: watercolor},
	{meta: Meta{Kind: "waves", Name: "Волны снизу", Inks: []string{"#d3e6f1", "#a9cce1", "#7fb2d1"}}, draw: waves},
	{meta: Meta{Kind: "doodle-school", Name: "Школьные дудлы", Inks: []string{"#5a7fa8"}, Random: true}, draw: doodle(schoolIcons)},
	{meta: Meta{Kind: "doodle-kids", Name: "Детские дудлы", Inks: []string{"#e08a5a"}, Random: true}, draw: doodle(kidsIcons)},
	paperGen("paper-1", "Бумага", "paper-1.jpg"),
	paperGen("paper-2", "Крафт", "paper-2.jpg"),
	paperGen("paper-3", "Лён", "paper-3.jpg"),
}

func paperGen(kind, name, file string) *gen {
	t := &texture{file: file}
	return &gen{meta: Meta{Kind: kind, Name: name, Paper: true}, draw: paper(t), ready: t.ok}
}

// artGens — стикеры программы (пакет art) в порядке каталога.
var artGens = func() []*gen {
	defs := art.All()
	out := make([]*gen, len(defs))
	for i, d := range defs {
		out[i] = artGen(d)
	}
	return out
}()

// artGen — генератор стикера: Draw рисует в пространстве (0..a)×(0..1), a — пропорция рамки.
func artGen(d art.Def) *gen {
	meta := Meta{Kind: d.Kind, Name: d.Name, Inks: d.Inks, Aspect: d.Aspect, Random: d.Random, Stretch: d.Stretch}
	return &gen{meta: meta, art: true, draw: func(c *canvas, s Spec) {
		a := meta.Aspect
		if meta.Stretch {
			a = s.Aspect
		}
		if !(a > 0) {
			a = 1
		}
		// Рамка (0..1)² растянута до a:1 — перевод дизайн-координат (u, v) в доли: x = u/a.
		c.M[0], c.M[3] = c.M[0]/a, c.M[3]/a
		c.W, c.H = a, 1
		d.Draw(c, art.Spec{Kind: s.Kind, Seed: s.Seed}, inks(s), a)
	}}
}
