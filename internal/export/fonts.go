package export

import (
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// unnamedFont — гарнитура без названия (id из шаблона): так же её зовёт карточка текста.
const unnamedFont = "Свой шрифт"

// PlanFonts — названия своих гарнитур, которых нет на этом компьютере, в непустых текстах (не заготовках) и
// подписях листов плана (082): в выгрузке они будут шрифтом Montserrat. По алфавиту, без повторов.
func PlanFonts(v model.View, pl *Plan) []string {
	names := map[string]string{}
	for _, f := range v.AlbumFonts {
		names[f.ID] = f.Name
	}
	var out []string
	use := func(t model.TextView) {
		id := t.Style.Font
		// заготовка (131) не печатается — шрифт её текста не нужен
		if t.Draft || strings.TrimSpace(t.Text) == "" || !text.IsCustomID(id) || text.Available(id) {
			return
		}
		name := names[id]
		if name == "" {
			name = unnamedFont
		}
		out = append(out, name)
	}
	add := func(s model.SpreadView) {
		for _, t := range s.Texts {
			use(t)
		}
		for _, c := range s.Captions {
			use(c.TextView)
		}
	}
	if pl.Cover != nil && v.Cover != nil {
		add(v.Cover.Sheet)
	}
	for i, s := range v.Spreads {
		if pl.mask[i+1] != 0 {
			add(s)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// FontsWarning — вопрос перед выгрузкой о шрифтах, которых нет на этом компьютере (082 п. 8);
// "" — все на месте.
func FontsWarning(names []string) string {
	if len(names) == 0 {
		return ""
	}
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = "„" + n + "“"
	}
	return "В выгрузке нет шрифтов: " + strings.Join(q, ", ") + ". Текст будет шрифтом Montserrat. Выгрузить так?"
}
