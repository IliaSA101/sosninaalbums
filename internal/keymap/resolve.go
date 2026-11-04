package keymap

import (
	"slices"
	"sort"
)

// Overrides — свои клавиши: id → полный список сочетаний команды (пустой — «без клавиши»).
// Хранится только разница со стандартными (074-5), в config.json → hotkeys.
type Overrides map[string][]string

// Пределы своего (config.json правят руками; design §4).
const (
	MaxIDs  = 256
	MaxKeys = 3
	MaxID   = 32
)

// Aliases — старый id → новый: переименованная команда не теряет своё сочетание (074-21).
// Каждый id, вышедший в выпуск (testdata/keymap-ids.txt), — в каталоге, здесь или в Retired.
var Aliases = map[string]string{}

// Retired — убранные команды: их своё больше не нужно.
var Retired = map[string]bool{}

func validID(id string) bool {
	if id == "" || len(id) > MaxID {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// known — id каталога или его старое имя.
func known(id string) bool {
	_, ok := lookup(id)
	return ok || Aliases[id] != ""
}

// Sanitize — копия своего в пределах: id [a-zA-Z0-9]{1,32}, не больше MaxIDs id и MaxKeys
// сочетаний, сочетание не длиннее MaxChord. Старые id переводятся по Aliases (если у нового
// своего нет), убранные отбрасываются. Незнакомые id сохраняются: их записала другая версия.
func Sanitize(o Overrides) Overrides {
	out := Overrides{}
	ids := make([]string, 0, len(o))
	for id := range o {
		// nil — строка вместо списка или null в файле: записи нет (стандартная клавиша);
		// пустой срез — своё «без клавиши».
		if validID(id) && !Retired[id] && o[id] != nil {
			ids = append(ids, id)
		}
	}
	// Сначала свои известные, затем по алфавиту: предел режет чужое, а не своё.
	sort.Slice(ids, func(i, j int) bool {
		ki, kj := known(ids[i]), known(ids[j])
		if ki != kj {
			return ki
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		if len(out) >= MaxIDs {
			break
		}
		to := id
		if n, ok := Aliases[id]; ok {
			if o[n] != nil {
				continue
			}
			to = n
		}
		keys := make([]string, 0, MaxKeys)
		for _, k := range o[id] {
			if len(keys) < MaxKeys && len(k) <= MaxChord {
				keys = append(keys, k)
			}
		}
		out[to] = keys
	}
	return out
}

// usable — своё сочетание команды: годное, не служебное, без повторов, не больше MaxKeys.
func usable(list []string) []string {
	out := make([]string, 0, len(list))
	for _, k := range list {
		cc, ok := Canon(k)
		if !ok || ReservedReason(cc) != "" || slices.Contains(out, cc) {
			continue
		}
		if len(out) == MaxKeys {
			break
		}
		out = append(out, cc)
	}
	return out
}

// Resolve — действующие клавиши всех команд каталога (design §4). Своё главнее стандартного:
// стандартное, занятое своим в той же области, не ставится (074-5); своё, уже взятое своим
// команды выше по каталогу в той же области, отбрасывается. Служебные — всегда стандартные.
// Уровень интерфейса не участвует: скрытая команда держит своё сочетание.
func Resolve(o Overrides) map[string][]string {
	o = Sanitize(o)
	eff := make(map[string][]string, len(catalog))
	taken := map[Scope]map[string]bool{}
	for _, s := range scopes {
		taken[s.ID] = map[string]bool{}
	}
	for _, x := range catalog {
		list, own := o[x.ID]
		if x.Fixed || !own {
			continue
		}
		keys := make([]string, 0, MaxKeys)
		for _, k := range usable(list) {
			if !taken[x.Scope][k] {
				taken[x.Scope][k] = true
				keys = append(keys, k)
			}
		}
		eff[x.ID] = keys
	}
	for _, x := range catalog {
		if _, own := o[x.ID]; own && !x.Fixed {
			continue
		}
		keys := make([]string, 0, len(x.Defaults))
		for _, k := range x.Defaults {
			if x.Fixed || !taken[x.Scope][k] {
				keys = append(keys, k)
			}
		}
		eff[x.ID] = keys
	}
	return eff
}

// sameSet — одинаковые наборы сочетаний (порядок не важен).
func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, k := range a {
		if !slices.Contains(b, k) {
			return false
		}
	}
	return true
}

// custom — своё у команды действительно отличается от стандартного.
func custom(o Overrides, x Command) bool {
	list, own := o[x.ID]
	return own && !x.Fixed && !sameSet(usable(list), x.Defaults)
}
