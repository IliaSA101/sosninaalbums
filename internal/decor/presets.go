package decor

import (
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
)

// Встроенные фоны (research §2): генератор decor/gen + начальные цвета + цвет страниц.
// ID sa:<пресет> живёт в каталоге и недавних, в альбом идут параметры генератора (решение №99):
// переименовать пресет можно, но тогда он выпадет из недавних.

const presetPrefix = "sa:"

type preset struct {
	id, theme, name, color string
	spec                   gen.Spec
}

// item — плитка встроенного; имя — пресета, иначе генератора.
func (p preset) item(w, h float64) Item {
	name := p.name
	if name == "" {
		if m, ok := gen.Lookup(p.spec.Kind); ok {
			name = m.Name
		}
	}
	it := Item{ID: p.id, Theme: p.theme, Name: name, URL: gen.Query(p.spec, w, h), Color: p.color}
	if gen.IsArt(p.spec.Kind) { // стикер: плитка в его пропорции; узор заполняет область — 0
		rs, _ := gen.Resolve(p.spec)
		it.Aspect = rs.Aspect
	}
	return it
}

func findPreset(list []preset, id string) (preset, bool) {
	for _, p := range list {
		if p.id == id {
			return p, true
		}
	}
	return preset{}, false
}

func pat(kind string, inks ...string) gen.Spec { return gen.Spec{Kind: kind, Inks: inks} }

// bgPresets — порядок каталога. Текстуры paper-* появятся сами, когда их файлы лягут в сборку.
var bgPresets = []preset{
	{"sa:cream-gradient", "basic", "Кремовый градиент", "#fbf6ef", pat("grad-h")},
	{"sa:sky-gradient", "basic", "Небесный градиент", "#f4f8fc", pat("grad-d")},
	{"sa:soft-glow", "basic", "Мягкое свечение", "#fffaf2", pat("glow")},
	{"sa:thin-stripes", "basic", "Тонкие полосы", "#fbf8f3", pat("stripes")},
	{"sa:wide-bands", "basic", "Широкие полосы", "#faf6f0", pat("bands")},
	{"sa:watercolor", "basic", "Акварель", "#ffffff", pat("watercolor")},
	{"sa:paper-white", "basic", "Бумага", "#f7f4ee", pat("paper-1")},
	{"sa:paper-kraft", "basic", "Крафт", "#d8bf9a", pat("paper-2")},
	{"sa:paper-linen", "basic", "Лён", "#efe9df", pat("paper-3")},

	{"sa:ivory-glow", "wedding", "Айвори", "#fffaf0", pat("glow", "#fffdf6", "#ead9b8")},
	{"sa:gold-dots", "wedding", "Золотой горох", "#fffdf8", pat("dots", "#d4b26a")},
	{"sa:blush-hearts", "wedding", "Нежные сердца", "#fff7f8", pat("hearts")},
	{"sa:rose-watercolor", "wedding", "Розовая акварель", "#fffdfc", pat("watercolor", "#f2c4c4", "#efd9cf", "#e6c6d6")},
	{"sa:pastel-brush", "wedding", "Пастельные мазки", "#fffdfa", pat("brush")},

	{"sa:confetti", "kids", "Конфетти", "#ffffff", pat("confetti")},
	{"sa:kids-doodles", "kids", "Детские дудлы", "#fffaf3", pat("doodle-kids")},
	{"sa:scatter", "kids", "Россыпь точек", "#fffdf8", pat("dots-scatter")},

	{"sa:sea-waves", "kindergarten", "Волны", "#f7fbfe", pat("waves")},
	{"sa:green-doodles", "kindergarten", "Зелёные дудлы", "#f6fbf3", pat("doodle-kids", "#5aa87a")},
	{"sa:big-dots", "kindergarten", "Крупный горох", "#fffaf0", gen.Spec{Kind: "dots", Inks: []string{"#f2b134"}, Scale: 1.6}},

	{"sa:notebook-grid", "school", "Тетрадь в клетку", "#ffffff", pat("grid-5")},
	{"sa:notebook-lines", "school", "Тетрадь в линейку", "#ffffff", pat("lines")},
	{"sa:school-doodles", "school", "Школьные дудлы", "#f7f9fc", pat("doodle-school")},
	{"sa:blue-stripes", "school", "Голубые полосы", "#f7fafd", pat("stripes", "#c9d9ec")},

	{"sa:night-stars", "grad", "Звёздная ночь", "#1f2a4d", pat("stars")}, // цвета звёзд — по умолчанию
	{"sa:gold-bands", "grad", "Золотые полосы", "#fbf8f1", pat("bands", "#e9dcc0", "#c9a24e")},
	{"sa:violet-glow", "grad", "Сиреневое свечение", "#f8f5fc", pat("glow", "#f4effa", "#c9b6e4")},

	{"sa:party-confetti", "holiday", "Праздничное конфетти", "#fffdf7",
		gen.Spec{Kind: "confetti", Inks: []string{"#e85d75", "#f2b134", "#7cc47f", "#4fb0c6"}, Scale: 1.4}},
	{"sa:gold-stars", "holiday", "Золотые звёзды", "#fffdf6", pat("stars")},
	{"sa:festive-dots", "holiday", "Праздничные точки", "#fffaf5", pat("dots-scatter", "#e85d75", "#f2b134")},
}
