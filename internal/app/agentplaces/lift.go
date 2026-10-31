package agentplaces

// Корень архива (0.35/153, design §2): место из родителя добавленной папки поднимается над
// датированной папкой («2026», «2026-05», «12.05.2026 Петровы»), чтобы новый год не требовал вопроса.
// Только строки и одна проба диска при пополнении (Admit); явно выбранное место («chosen») не
// поднимается (0.35-153-3).

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Датированное имя (design §2.1): год в начале с разделителем или концом имени, либо дата дд.мм.гггг.
// Год в конце («Свадьбы 2026») не считается: так захватывалась бы «Бухгалтерия 2025» (0.35-153-2).
var (
	yearName = regexp.MustCompile(`^(199\d|20\d\d)($|[ \-_.–—])`)
	dateName = regexp.MustCompile(`^(0?[1-9]|[12]\d|3[01])[.\-_](0?[1-9]|1[0-2])[.\-_](199\d|20\d\d)($|[ \-_.–—])`)
)

// maxDatedRunes — имя длиннее не датированное (папка клиента с фамилией и подписью — не год).
const maxDatedRunes = 40

// liftAncestors — сколько предков добавленной папки смотрит подъём (год/клиент/Отбор/JPG).
const liftAncestors = 3

// datedName — имя папки начинается с года или даты. NFC: на macOS имена приходят разложенными.
func datedName(name string) bool {
	if len(name) > 4*maxDatedRunes { // до нормализации: чужое имя не гоняем через NFC зря
		return false
	}
	name = norm.NFC.String(name)
	if utf8.RuneCountInString(name) > maxDatedRunes {
		return false
	}
	return yearName.MatchString(name) || dateName.MatchString(name)
}

// name — последний элемент очищенного пути ("" у корня).
func (r *Rules) name(p string) string {
	return p[strings.LastIndex(p, r.Sep())+1:]
}

// liftTarget — среди p и двух его предков (кандидаты a1…a3 добавленной папки, p = a1) самый верхний с
// датированным именем; возвращается его родитель. Нет такого — found = false.
func (r *Rules) liftTarget(p string) (c string, found bool) {
	top, cur := "", p
	for range liftAncestors {
		if datedName(r.name(cur)) {
			top = cur
		}
		next := r.Parent(cur)
		if next == cur {
			break // корень
		}
		cur = next
	}
	if top == "" {
		return "", false
	}
	return r.Parent(top), true
}

// volumeRoot — c корень тома или место, где лежат тома: узкое место на нём — «диск целиком»
// (E:\2026, /media/u/X/2026), а не просто папка выше года.
func (r *Rules) volumeRoot(c string) bool {
	if r.windows {
		return len(c) == 3
	}
	if c == "/" {
		return true
	}
	userRoot, volume := unixMountShape(c)
	return userRoot || volume
}

// Lift — место из base (родитель добавленной папки, он же a1; для места 0.34 — оно само как родитель
// условной папки): над самым верхним датированным среди base и двух его предков, если родитель
// проходит Admit; иначе base — как в 0.34. narrow — место узкое из-за ширины (правила 150 не пускают
// выше года), narrowRoot — выше года только корень тома. Отказ диска узким место не делает.
func (e *Env) Lift(base string) (dir string, narrow, narrowRoot bool, err error) {
	if !e.ok() {
		return "", false, false, ErrPlaceBad
	}
	p, err := e.base(base, false)
	if err != nil {
		return "", false, false, err
	}
	if c, found := e.Rules.liftTarget(p); found {
		if e.staticParent(c) != nil {
			narrow, narrowRoot = true, e.Rules.volumeRoot(c)
		} else if res, aerr := e.Admit(c, prefs.PlaceParent); aerr == nil {
			return res, false, false, nil
		}
	}
	dir, err = e.Admit(base, prefs.PlaceParent)
	return dir, narrow, narrowRoot, err
}

// NarrowOf — сохранённое место «parent» узкое (подъём выше года не пустили правила): строки без
// диска, годится на каждое agents:state. Явно выбранное место узким не бывает.
func (e *Env) NarrowOf(p prefs.Place) (narrow, narrowRoot bool) {
	if !e.ok() || p.How != prefs.PlaceParent {
		return false, false
	}
	clean, err := e.base(p.Dir, false)
	if err != nil || e.staticParent(clean) != nil {
		return false, false // такого места быть не может
	}
	c, found := e.Rules.liftTarget(clean)
	if !found || e.staticParent(c) == nil {
		return false, false
	}
	return true, e.Rules.volumeRoot(c)
}
