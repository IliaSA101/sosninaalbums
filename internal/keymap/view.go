package keymap

import "slices"

// Entry — команда каталога с действующими клавишами.
type Entry struct {
	Command
	Keys   []string `json:"keys"`
	Custom bool     `json:"custom"` // своё отличается от стандартного: в окне видна «Стандартное»
}

// View — всё, что интерфейсу нужно о клавишах (design §6): State.Keymap и ответы окна.
type View struct {
	Commands []Entry     `json:"commands"` // каталог по порядку
	Groups   []Named     `json:"groups"`
	Scopes   []ScopeInfo `json:"scopes"`
	Locked   []Reserved  `json:"locked"`
	Custom   Overrides   `json:"custom"`  // своё как есть — для «Вернуть»
	Changed  int         `json:"changed"` // команд со своим; 0 — «Сбросить всё» серое
}

// SetResult — ответ окну на правку: вид и итог (design §6).
type SetResult struct {
	View     View  `json:"view"`
	Conflict *Ref  `json:"conflict"` // не записано: спросить «Заменить?»
	Replaced *Ref  `json:"replaced"` // у кого забрали после «Заменить»
	Shadows  []Ref `json:"shadows"`  // перекрытия в других областях
}

// Build — каталог с действующими клавишами для своего o.
func Build(o Overrides) View {
	o = Sanitize(o)
	eff := Resolve(o)
	v := View{
		Commands: make([]Entry, len(catalog)),
		Groups:   Groups(),
		Scopes:   Scopes(),
		Locked:   Locked(),
		Custom:   o,
	}
	for i, x := range catalog {
		x.Defaults = slices.Clone(x.Defaults)
		e := Entry{Command: x, Keys: eff[x.ID], Custom: custom(o, x)}
		if e.Custom {
			v.Changed++
		}
		v.Commands[i] = e
	}
	return v
}

// Result — ответ окну после правки: вид по новому o и итог.
func Result(o Overrides, out Outcome) SetResult {
	r := SetResult{View: Build(o), Conflict: out.Conflict, Replaced: out.Replaced, Shadows: out.Shadows}
	if r.Shadows == nil {
		r.Shadows = []Ref{}
	}
	return r
}
