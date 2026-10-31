package keymap

import (
	"errors"
	"fmt"
	"slices"
)

// Ref — команда и сочетание для строки окна: «Ctrl+D уже у „Дублировать“».
type Ref struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Chord string `json:"chord"`
}

// Outcome — итог правки своих клавиш.
type Outcome struct {
	Conflict *Ref  // не записано: клавиша у команды той же области — спросить «Заменить?»
	Replaced *Ref  // после «Заменить»: у кого забрали
	Shadows  []Ref // та же клавиша в пересекающейся области другого ранга: «там сработает она»
}

// editable — команда, клавиши которой можно менять.
func editable(id string) (Command, error) {
	x, ok := lookup(id)
	if !ok {
		return Command{}, fmt.Errorf("неизвестная команда %q", id)
	}
	if x.Fixed {
		return Command{}, errors.New("служебная команда: её клавиши не меняются")
	}
	return x, nil
}

// holder — другая команда той же области, у которой сочетание действует.
func holder(eff map[string][]string, x Command, chord string) (Command, bool) {
	for _, h := range catalog {
		if h.ID != x.ID && !h.Fixed && h.Scope == x.Scope && slices.Contains(eff[h.ID], chord) {
			return h, true
		}
	}
	return Command{}, false
}

// shadows — перекрытия сочетаний команды в пересекающихся областях другого ранга.
func shadows(eff map[string][]string, x Command, chords []string) []Ref {
	var out []Ref
	for _, k := range chords {
		if !slices.Contains(eff[x.ID], k) {
			continue
		}
		for _, h := range catalog {
			if h.Fixed || h.Scope == x.Scope || !overlaps(h.Scope, x.Scope) || !slices.Contains(eff[h.ID], k) {
				continue
			}
			out = append(out, Ref{h.ID, h.Label, k})
		}
	}
	return out
}

func without(list []string, k string) []string {
	return slices.DeleteFunc(slices.Clone(list), func(s string) bool { return s == k })
}

// settle — своё известных команд приводится к действующему: годные сочетания в каноническом
// виде; совпавшее со стандартным и своё служебных удаляется. Чужие id не трогаются.
func settle(o Overrides) Overrides {
	eff := Resolve(o)
	for id := range o {
		x, ok := lookup(id)
		switch {
		case !ok:
		case x.Fixed || sameSet(eff[id], x.Defaults):
			delete(o, id)
		default:
			o[id] = eff[id]
		}
	}
	return o
}

// Assign — назначить команде id сочетание chord: old == "" — добавить, иначе заменить old.
// Сочетание у команды той же области: без replace — Outcome.Conflict и o без изменений,
// с replace — у той команды клавиша снимается. Ошибка — текст для строки окна.
func Assign(o Overrides, id, old, chord string, replace bool) (Overrides, Outcome, error) {
	o = Sanitize(o)
	x, err := editable(id)
	if err != nil {
		return o, Outcome{}, err
	}
	cc, ok := Canon(chord)
	if !ok {
		return o, Outcome{}, fmt.Errorf("не сочетание клавиш: %q", chord)
	}
	if why := ReservedReason(cc); why != "" {
		return o, Outcome{}, errors.New(why)
	}
	eff := Resolve(o)
	cur := eff[id]
	oc, _ := Canon(old)
	i := slices.Index(cur, oc)
	if slices.Contains(cur, cc) {
		if i < 0 || oc == cc {
			return o, Outcome{}, nil
		}
		// Заменить old на уже имеющееся — значит убрать old.
		o[id] = slices.Delete(slices.Clone(cur), i, i+1)
		return settle(o), Outcome{}, nil
	}
	next := slices.Clone(cur)
	if i >= 0 {
		next[i] = cc
	} else {
		next = append(next, cc)
	}
	if len(next) > MaxKeys {
		return o, Outcome{}, fmt.Errorf("у команды не больше %d сочетаний — сначала уберите одно", MaxKeys)
	}
	h, held := holder(eff, x, cc)
	if held && !replace {
		return o, Outcome{Conflict: &Ref{h.ID, h.Label, cc}}, nil
	}
	o[id] = next
	out := Outcome{}
	if held {
		o[h.ID] = without(eff[h.ID], cc)
		out.Replaced = &Ref{h.ID, h.Label, cc}
	}
	o = settle(o)
	out.Shadows = shadows(Resolve(o), x, []string{cc})
	return o, out, nil
}

// Remove — убрать у команды сочетание; последнее убранное оставляет команду без клавиши.
func Remove(o Overrides, id, chord string) (Overrides, error) {
	o = Sanitize(o)
	if _, err := editable(id); err != nil {
		return o, err
	}
	cc, ok := Canon(chord)
	if !ok {
		return o, fmt.Errorf("не сочетание клавиш: %q", chord)
	}
	cur := Resolve(o)[id]
	if !slices.Contains(cur, cc) {
		return o, nil
	}
	o[id] = without(cur, cc)
	return settle(o), nil
}

// Reset — вернуть команде стандартные клавиши. Стандартное занято своим другой команды той же
// области: без replace — Outcome.Conflict (первое из занятых) и o без изменений (074-22),
// с replace — у тех команд клавиши снимаются.
func Reset(o Overrides, id string, replace bool) (Overrides, Outcome, error) {
	o = Sanitize(o)
	x, err := editable(id)
	if err != nil {
		return o, Outcome{}, err
	}
	if _, own := o[id]; !own {
		return o, Outcome{}, nil
	}
	eff := Resolve(o)
	type take struct {
		h Command
		d string
	}
	var takes []take
	for _, d := range x.Defaults {
		if h, held := holder(eff, x, d); held {
			takes = append(takes, take{h, d})
		}
	}
	if len(takes) > 0 && !replace {
		return o, Outcome{Conflict: &Ref{takes[0].h.ID, takes[0].h.Label, takes[0].d}}, nil
	}
	var out Outcome
	for i, t := range takes {
		eff[t.h.ID] = without(eff[t.h.ID], t.d)
		o[t.h.ID] = eff[t.h.ID]
		if i == 0 {
			out.Replaced = &Ref{t.h.ID, t.h.Label, t.d}
		}
	}
	delete(o, id)
	o = settle(o)
	out.Shadows = shadows(Resolve(o), x, x.Defaults)
	return o, out, nil
}

// ResetAll — все команды каталога — стандартные; чужие id (другой версии) остаются.
func ResetAll(o Overrides) Overrides {
	out := Overrides{}
	for id, list := range Sanitize(o) {
		if !known(id) {
			out[id] = list
		}
	}
	return out
}

// Restore — «Вернуть» окна: своё команд каталога — из снимка prev, чужие id (другой версии) —
// из файла cur, а каких там нет — из снимка. Снимок мог быть снят до правки другим экземпляром
// интерфейса: чужое, записанное за это время, не теряется. Итог — как после правки (settle).
func Restore(cur, prev Overrides) Overrides {
	out := ResetAll(cur)
	for id, list := range Sanitize(prev) {
		if _, has := out[id]; !has || known(id) {
			out[id] = list
		}
	}
	return settle(Sanitize(out))
}
