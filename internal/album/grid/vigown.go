package grid

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Свои шаблоны виньетки (038 design §3): чтение листа в шаблон и разбор «my:<id>» при
// «Применить». Геометрию строит vignette.BuildOwn, тела правок — vignette.go.

// OwnTpl — свой шаблон, выбранный для листа: Shape встаёт в Vignette.Own; Style — оформление,
// которое шаблон приносит при смене шаблона (nil — копия с листа, оформление не приносится).
type OwnTpl struct {
	Shape *model.VigShape
	Style *model.VigStyle
}

// vigError — ошибка с готовым текстом тоста (с заглавной буквы, как model.NoPlacesError).
type vigError string

func (e vigError) Error() string { return string(e) }

const (
	errNotVig    = vigError("Это не виньетка")
	errTplFormat = vigError("Шаблон для другого формата")
)

// VigTemplateOf — шаблон виньетки листа spreadID для «Сохранить как шаблон»: все места листа
// с ролями (без угла: BuildOwn его не знает), полоса шапки, пропорции и размер листа, стиль подписей листа,
// подпись учителя и стиль шапки; фото, имена, поля и текст шапки не берутся. Результат
// проходит тот же зажим, что файл шаблонов. Не виньетка — «Это не виньетка», нет мест учеников
// и учителей — NoPlacesError. Только чтение.
func VigTemplateOf(p *model.Project, spreadID string) (boxes []model.Box, geo model.VigGeo, style model.VigStyle, err error) {
	_, s := p.Find(spreadID)
	if s == nil {
		return nil, geo, style, errVigSpread
	}
	if !s.IsVignette() {
		return nil, geo, style, errNotVig
	}
	g := p.Geo(s)
	geo = model.VigGeo{Pages: len(g.Pages), Aspect: g.W / g.H, Ratio: s.Vig.Ratio, W: g.W, H: g.H}
	for _, pl := range s.Grid {
		boxes = append(boxes, model.Box{X: pl.X / g.W, Y: pl.Y / g.H, W: pl.W / g.W, H: pl.H / g.H})
		geo.Roles = append(geo.Roles, model.RoleOf(pl))
	}
	if h := vigHead(s, s.Vig.Head); h != nil {
		b := model.BoxOf(h)
		geo.Head = &model.Box{X: b.X / g.W, Y: b.Y / g.H, W: b.W / g.W, H: b.H / g.H}
		hs := h.Style
		style.Head = &hs
	}
	if s.Caps != nil {
		c := *s.Caps
		style.Caps = &c
	}
	style.TText = s.Vig.TText
	// Зажим как у файла: NaN, лишние места, чужой портрет; не осталось мест людей — шаблона нет.
	out := model.ClampTemplates([]model.Template{{ID: model.NewID(), Scope: model.TplSpread, Boxes: boxes,
		Kind: model.TplVignette, Vig: &model.VigTpl{VigGeo: geo, Style: style}}})
	if len(out) == 0 {
		return nil, geo, style, model.NoPlacesError("На листе нет мест учеников и учителей")
	}
	return out[0].Boxes, out[0].Vig.VigGeo, out[0].Vig.Style, nil
}

// ResolveOwn — свой шаблон для Vignette.Tpl = tpl листа spreadID. Не «my:…» — nil (каталог).
// Шаблон из списка tpls (templates.json) — копия геометрии с оформлением; нет в списке, но это
// шаблон, уже стоящий на листе, — его Own (шаблон удалён или альбом на другом компьютере),
// оформление не приносится; иначе «Шаблон не найден». Шаблон другого числа страниц — ошибка.
func ResolveOwn(p *model.Project, spreadID, tpl string, tpls []model.Template) (*OwnTpl, error) {
	id, ok := strings.CutPrefix(tpl, model.VigOwnPrefix)
	if !ok {
		return nil, nil
	}
	_, s := p.Find(spreadID)
	if s == nil {
		return nil, errVigSpread
	}
	for _, t := range tpls {
		if t.ID != id || t.Kind != model.TplVignette || t.Vig == nil {
			continue
		}
		if t.Vig.Pages != len(p.Geo(s).Pages) {
			return nil, errTplFormat
		}
		geo := t.Vig.VigGeo
		geo.Roles = slices.Clone(geo.Roles)
		if geo.Head != nil {
			h := *geo.Head
			geo.Head = &h
		}
		st := t.Vig.Style
		return &OwnTpl{Shape: &model.VigShape{Name: t.Name, Boxes: slices.Clone(t.Boxes), VigGeo: geo}, Style: &st}, nil
	}
	if s.IsVignette() && s.Vig.Tpl == tpl && s.Vig.Own != nil {
		return &OwnTpl{Shape: s.Vig.Own}, nil // Own не меняется на месте: указатель можно делить
	}
	return nil, model.ErrTplNotFound // та же ошибка, что у удаления шаблона: тост один
}

// OwnNames — имена виньеточных шаблонов файла по Vig.Tpl листа («my:<id>»).
func OwnNames(tpls []model.Template) map[string]string {
	names := map[string]string{}
	for _, t := range tpls {
		if t.Kind == model.TplVignette {
			names[model.VigOwnPrefix+t.ID] = t.Name
		}
	}
	return names
}

// RenameOwn ставит копиям своих шаблонов на листах p имена names (OwnNames): n — сколько
// листов сменили имя, grow — на сколько байт вырос JSON проекта (вес снимка отмены). Vig и
// Own копируются, а не правятся на месте: копии листа делят указатели (DuplicateGrid).
func RenameOwn(p *model.Project, names map[string]string) (n, grow int) {
	for _, s := range p.Spreads {
		if !s.IsVignette() || s.Vig.Own == nil {
			continue
		}
		name, ok := names[s.Vig.Tpl]
		if !ok || name == s.Vig.Own.Name {
			continue
		}
		grow += jsonLen(name) - jsonLen(s.Vig.Own.Name)
		o, v := *s.Vig.Own, *s.Vig
		o.Name = name
		v.Own = &o
		s.Vig = &v
		n++
	}
	return n, grow
}

// jsonLen — длина строки в JSON снимка (model.Marshal — encoding/json с экранированием).
func jsonLen(s string) int {
	b, _ := json.Marshal(s)
	return len(b)
}
