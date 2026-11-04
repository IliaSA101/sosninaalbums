package grid

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Рамочки мест шаблона разворота (131, design §1): TplBorder ↔ Place.Border.

// borders кладёт рамочки шаблона в Box.Border мест t.Places — до tplBoxes и samePlaces: рамочка
// входит в сверку «места те же», иначе повтор не холостой. t — зажатая копия (ClampSpreadTpl
// даёт срезу свой массив). Картинка записывается в Photos здесь же, до ClampPlace (К1-12);
// файла на этом ПК нет — место без рамочки, ключ уходит в tplRes.gone (missing).
func (r *tplRes) borders(t *model.SpreadTpl) {
	for _, tb := range t.Borders {
		if tb.I < 0 || tb.I >= len(t.Places) {
			continue
		}
		t.Places[tb.I].Border = r.border(tb)
	}
}

// border — рамочка шаблона, зажатая как в проекте (ClampBorder: повтор постановки сравнивает
// готовые значения); nil — картинки нет на этом ПК.
func (r *tplRes) border(tb model.TplBorder) *model.Border {
	if tb.Lib != "" {
		id := r.photo(tb.Lib)
		if id == "" {
			return nil
		}
		return model.ClampBorder(r.p, &model.Border{Photo: id, Scale: tb.Scale, Cut: tb.Cut, Open: tb.Open})
	}
	return model.ClampBorder(r.p, &model.Border{Kind: tb.Kind, Inks: slices.Clone(tb.Inks), Width: tb.Width, Preset: tb.Preset})
}

// bordersOf — рамочки мест boxes (долей области, порядок = Places шаблона): встроенная как
// есть, картинка — только своя из библиотеки (ключ Lib; запись без ключа — снимок съёмки, в
// шаблон не идёт, как кадр у фона, решение 6). Отметка плитки Preset едет с рамочкой.
func bordersOf(p *model.Project, boxes []model.Box) []model.TplBorder {
	var out []model.TplBorder
	for i, b := range boxes {
		bd := b.Border
		switch {
		case bd == nil:
		case bd.Kind != "":
			out = append(out, model.TplBorder{I: i, Kind: bd.Kind, Inks: slices.Clone(bd.Inks), Width: bd.Width, Preset: bd.Preset})
		default:
			if key := p.Photos[bd.Photo].Lib; key != "" {
				out = append(out, model.TplBorder{I: i, Lib: key, Scale: bd.Scale, Cut: bd.Cut, Open: bd.Open})
			}
		}
	}
	return out
}
