package gen

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
