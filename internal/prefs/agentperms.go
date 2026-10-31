package prefs

// Разрешения агентов по инструментам (спека 0.34/150, design §2): «Всегда · Спрашивать ·
// Запретить» на каждый инструмент и места съёмок, где папку можно взять без вопроса. Ключ
// верхнего уровня config.json — 0.33 его не знает и при записи сохраняет как есть. Здесь только
// хранение и строение (диск не трогаем: clean() работает на каждом чтении config.json, то есть на
// каждом вызове агента); правила мест — internal/app/agentplaces.go.

import (
	"log"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Значения разрешения.
const (
	PermAllow = "allow" // «Всегда»
	PermAsk   = "ask"   // «Спрашивать»
	PermDeny  = "deny"  // «Запретить»
)

// Классы инструментов (design §1): от класса зависит, что можно поставить и что даёт столбец.
const (
	ClassRead   = "R" // чтение: «Спрашивать» нет
	ClassStart  = "P" // start_program: «Спрашивать» нет, программа закрыта
	ClassShow   = "V" // показ разворота
	ClassEdit   = "E" // обратимая правка
	ClassFolder = "F" // add_folder
	ClassAlbum  = "A" // create_album
	// ClassAskOnly — export_album, open_album: только с вопросом человеку, «Всегда» нет (0.35/152).
	ClassAskOnly = "Q"
)

// Как место попало в список.
const (
	PlaceChosen = "chosen" // «Папка со съёмками…»: место — сама папка
	PlaceParent = "parent" // родитель папки, добавленной человеком
)

// Пределы: 10 мест × 1 КБ держат config.json далеко от 64 КБ, которые читает посредник.
const (
	MaxPlaces       = 10
	maxPlaceBytes   = 1024
	maxToolName     = 64
	maxUnknownTools = 32 // имена новее этой версии, которых не знаем; чужой файл не раздувает карту
)

// Place — место съёмок: «Всегда» у папки берёт без вопроса только внутри мест.
type Place struct {
	Dir string `json:"dir"`
	How string `json:"how"` // PlaceChosen | PlaceParent
	// Vol — диск места (0.35/153): сверяется в тихой проверке по номеру, не по букве; nil — место по
	// пути, как в 0.34 (0.34 поле читает и при записи теряет).
	Vol *PlaceVol `json:"vol,omitempty"`
}

// PlaceLift — место 0.34, поднятое до корня архива при обновлении (0.35/153): заметка в окне
// «ИИ-агенты» живёт, пока человек не нажмёт «Понятно».
type PlaceLift struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// AgentPerms — ключ agentPerms: Edit — значение agents.edit, которое 0.34 записала последним.
type AgentPerms struct {
	Tools  map[string]string `json:"tools,omitempty"`
	Places []Place           `json:"places,omitempty"`
	Lifted []PlaceLift       `json:"lifted,omitempty"` // новые первыми, ≤ MaxPlaces
	// LiftDone — проход подъёма мест 0.34 выполнен (0.35/153): места 0.35 без диска (Mac, тома без
	// номера) подниматься не должны. Ставится первой записью ключа и самим проходом; 0.34 поле теряет.
	LiftDone bool `json:"liftDone,omitempty"`
	Edit     bool `json:"edit"`
}

// toolClass — инструменты этой версии (design §1). Новее — класс не знаем, решает служба.
var toolClass = map[string]string{
	"get_album": ClassRead, "list_spreads": ClassRead, "get_spread": ClassRead, "list_photos": ClassRead,
	"get_photo": ClassRead, "list_decor": ClassRead, "list_packs": ClassRead, "list_print_profiles": ClassRead,
	"list_albums": ClassRead, "get_photo_preview": ClassRead, "get_spread_preview": ClassRead, "get_contact_sheet": ClassRead,
	"start_program": ClassStart, "show_spread": ClassShow,
	"autobuild": ClassEdit, "vary_layouts": ClassEdit, "undo_last_edit": ClassEdit, "set_marks": ClassEdit,
	"set_cover_photo": ClassEdit, "set_background": ClassEdit, "add_sticker": ClassEdit, "apply_print": ClassEdit,
	"add_folder": ClassFolder, "create_album": ClassAlbum,
	"export_album": ClassAskOnly, "open_album": ClassAskOnly,
}

// Пять правок, которыми управлял флажок 0.33 (design §2 «Откат»): остальные 0.33 не знает.
var (
	flag033Allow = []string{"autobuild", "vary_layouts", "undo_last_edit"}
	flag033Ask   = []string{"add_folder", "create_album"}
)

// AgentToolClass — класс инструмента этой версии; "" — не знаем.
func AgentToolClass(tool string) string { return toolClass[tool] }

// AgentToolNames — инструменты этой версии по алфавиту.
func AgentToolNames() []string {
	names := make([]string, 0, len(toolClass))
	for k := range toolClass {
		names = append(names, k)
	}
	slices.Sort(names)
	return names
}

// PermCan — допустимые значения: у чтения и start_program «Спрашивать» нет (0.34-150-3), у
// «только с вопросом» нет «Всегда» (0.35-152-2).
func PermCan(tool string) []string {
	switch toolClass[tool] {
	case ClassRead, ClassStart:
		return []string{PermAllow, PermDeny}
	case ClassAskOnly:
		return []string{PermAsk, PermDeny}
	}
	return []string{PermAllow, PermAsk, PermDeny}
}

func isEditClass(c string) bool { return c == ClassEdit || c == ClassFolder || c == ClassAlbum }

// validToolName — [a-z_]{1,64}.
func validToolName(k string) bool {
	return k != "" && len(k) <= maxToolName && strings.Trim(k, "abcdefghijklmnopqrstuvwxyz_") == ""
}

// cleanMode: значение вне допустимых (PermCan) — «Запретить»; исключение — «Всегда» у «только с
// вопросом» (человек правил config.json и хотел разрешить): это «Спрашивать», а не молчаливый запуск.
func cleanMode(tool, v string) string {
	if slices.Contains(PermCan(tool), v) {
		return v
	}
	if v == PermAllow && toolClass[tool] == ClassAskOnly {
		return PermAsk
	}
	return PermDeny
}

// cleanDir — путь абсолютный (строка, без диска), ≤ 1 КБ, годный UTF-8 без управляющих знаков.
func cleanDir(dir string) (string, bool) {
	if dir == "" || len(dir) > maxPlaceBytes || !utf8.ValidString(dir) || !filepath.IsAbs(dir) {
		return "", false
	}
	for _, r := range dir {
		if r < 0x20 || r == 0x7f {
			return "", false
		}
	}
	dir = filepath.Clean(dir)
	return dir, len(dir) <= maxPlaceBytes
}

// cleanPlace — способ из двух и годный путь (cleanDir); негодная привязка к диску теряется (cleanVol).
func cleanPlace(p Place) (Place, bool) {
	if p.How != PlaceChosen && p.How != PlaceParent {
		return Place{}, false
	}
	dir, ok := cleanDir(p.Dir)
	p.Dir = dir
	p.Vol = cleanVol(p.Vol)
	return p, ok
}

// cleanLifts — годные пары (оба пути — cleanDir, From ≠ To) без повторов по From; первые — новее.
func cleanLifts(list []PlaceLift) []PlaceLift {
	var out []PlaceLift
	seen := map[string]bool{}
	for _, v := range list {
		from, ok1 := cleanDir(v.From)
		to, ok2 := cleanDir(v.To)
		if !ok1 || !ok2 || paths.DirKey(from) == paths.DirKey(to) || seen[paths.DirKey(from)] {
			continue
		}
		seen[paths.DirKey(from)] = true
		if out = append(out, PlaceLift{From: from, To: to}); len(out) == MaxPlaces {
			break
		}
	}
	return out
}

// normalizePlaces — места без повторов (по Place.Key; при повторе «chosen» побеждает «parent», остаётся первая
// позиция) и без внутренних (design 153 §2.3: место внутри другого, в том числе «chosen» внутри
// «parent»); предел limit — после чистки. Негодное строение выбрасывается. Результат — новый срез.
func normalizePlaces(list []Place, limit int) []Place {
	out := make([]Place, 0, min(len(list), 2*MaxPlaces))
	at := make(map[string]int, len(out))
	for _, v := range list {
		v, ok := cleanPlace(v)
		if !ok {
			continue
		}
		k := v.Key()
		if i, dup := at[k]; dup {
			if v.How == PlaceChosen {
				out[i].How = PlaceChosen
			}
			continue
		}
		if len(out) == 2*MaxPlaces {
			break // чужой файл не раздувает работу: после чистки всё равно останется не больше MaxPlaces
		}
		at[k] = len(out)
		out = append(out, v)
	}
	kept := out[:0:0]
	for i, v := range out {
		inner := false
		for j, w := range out {
			if i != j && PlaceInside(v, w) {
				inner = true
				break
			}
		}
		if !inner {
			kept = append(kept, v)
		}
	}
	return kept[:min(len(kept), limit)]
}

// PushPlace ставит место наверх без повторов (по Place.Key), убирает места внутри других, список
// режется до MaxPlaces. Негодное строение — список без изменений. Уже стоявшее «chosen» остаётся
// таким: человек выбрал его явно, а правила «parent» строже. Результат — новый срез.
func PushPlace(list []Place, p Place) []Place {
	if np, ok := cleanPlace(p); ok {
		return normalizePlaces(append([]Place{np}, list...), MaxPlaces)
	}
	return normalizePlaces(list, MaxPlaces)
}

// LiftPlaces переносит места «parent» по moves (ключ — paths.DirKey(From)) в To, сливает повторы,
// убирает внутренние и дописывает пары в Lifted. Возвращает перенесённые пары (нет такого места под
// замком — не в счёте). Нечего менять — ни места, ни Lifted не трогаются.
func (p *AgentPerms) LiftPlaces(moves map[string]PlaceLift) []PlaceLift {
	var done []PlaceLift
	list := slices.Clone(p.Places)
	for i, v := range list {
		if m, ok := moves[paths.DirKey(v.Dir)]; ok && v.How == PlaceParent && v.Vol == nil { // привязанное не поднимаем: путь внутри тома остался бы прежним
			done = append(done, m)
			list[i].Dir = m.To
		}
	}
	p.Places = normalizePlaces(list, MaxPlaces)
	if len(done) > 0 {
		p.Lifted = cleanLifts(append(slices.Clone(done), p.Lifted...))
	}
	return done
}

// DropInnerPlaces — список без мест внутри других (для проверки «писать нечего» до записи).
func DropInnerPlaces(list []Place) []Place { return normalizePlaces(list, MaxPlaces) }

// appendPlaces дописывает годные места из list без повторов, пока не наберётся MaxPlaces.
func appendPlaces(out, list []Place) []Place {
	seen := make(map[string]bool, len(out)+len(list))
	for _, v := range out {
		seen[v.Key()] = true
	}
	for _, v := range list {
		if len(out) == MaxPlaces {
			break
		}
		if v, ok := cleanPlace(v); ok && !seen[v.Key()] {
			seen[v.Key()] = true
			out = append(out, v)
		}
	}
	return out
}

// clean — только строение (design §2): значения из трёх, «Спрашивать» там, где его нет, —
// «Запретить»; чужие имена [a-z_]{1,64} сохраняются (≤ 32 сверх известных), прочие
// выбрасываются; места — PushPlace. Результат не делит карту и срез с p.
func (p AgentPerms) clean() AgentPerms {
	keys := make([]string, 0, len(p.Tools))
	for k := range p.Tools {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var tools map[string]string
	unknown := 0
	for _, k := range keys {
		if !validToolName(k) {
			continue
		}
		if _, known := toolClass[k]; !known {
			if unknown == maxUnknownTools {
				continue
			}
			unknown++
		}
		if tools == nil {
			tools = map[string]string{}
		}
		tools[k] = cleanMode(k, p.Tools[k])
	}
	p.Tools = tools
	if places := appendPlaces(nil, p.Places); len(places) > 0 {
		p.Places = places
	} else {
		p.Places = nil
	}
	p.Lifted = cleanLifts(p.Lifted)
	return p
}

// newToolMode — инструмент, которого нет в tools (новее записи): чтение и показ — «Всегда»,
// запуск тоже; правка и «только с вопросом» — «Спрашивать», а если все записанные правки
// «Запретить» — «Запретить». «Только с вопросом» в счёт правок не входит (0.35-152-11).
func newToolMode(class string, tools map[string]string) string {
	if !isEditClass(class) && class != ClassAskOnly {
		return PermAllow
	}
	for k, v := range tools {
		if isEditClass(toolClass[k]) && v != PermDeny {
			return PermAsk
		}
	}
	return PermDeny
}

// column — столбцы таблицы design §1 для известных инструментов: «0.33, правки вкл.» при flag,
// иначе «Новый» и «0.33, выкл.» (они совпадают).
func column(flag bool) map[string]string {
	t := make(map[string]string, len(toolClass))
	for k, c := range toolClass {
		switch {
		case !isEditClass(c) && c != ClassAskOnly:
			t[k] = PermAllow
		case !flag:
			t[k] = PermDeny
		case c == ClassAskOnly:
			t[k] = PermAsk
		case c == ClassEdit:
			t[k] = PermAllow
		default:
			t[k] = PermAsk
		}
	}
	if flag {
		t["apply_print"] = PermAsk // 0.34-150-14: смена формата и типографии — не молча
	}
	return t
}

// AgentPermsOf — сверенный вид разрешений: после чистки, миграции (нет ключа — столбец по
// agents.edit), умолчаний для недостающих инструментов и отката (agents.edit ≠ agentPerms.edit —
// флажок менял 0.33). Основа любой записи (design §2 «От сверенного вида»); сам файл не меняет.
func AgentPermsOf(c Config) AgentPerms {
	edit := c.Agents != nil && c.Agents.Edit
	if c.AgentPerms == nil {
		return AgentPerms{Tools: column(edit), Edit: edit}
	}
	p := c.AgentPerms.clean()
	if p.Tools == nil {
		p.Tools = map[string]string{}
	}
	stored := maps.Clone(p.Tools)
	for k, class := range toolClass {
		if _, ok := p.Tools[k]; !ok {
			p.Tools[k] = newToolMode(class, stored)
		}
	}
	if edit != p.Edit {
		rollback(p.Tools, edit)
	}
	p.Edit = edit
	return p
}

// rollback — флажок 0.33 сменился: выключен — правки, папка, альбом и «только с вопросом»
// «Запретить»; включён — только пять правок, которыми он управлял (0.34-150-33), «только с
// вопросом» не трогает. Прочее — как записано.
func rollback(tools map[string]string, flag bool) {
	for k, class := range toolClass {
		if !flag && (isEditClass(class) || class == ClassAskOnly) {
			tools[k] = PermDeny
		}
	}
	if flag {
		for _, k := range flag033Allow {
			tools[k] = PermAllow
		}
		for _, k := range flag033Ask {
			tools[k] = PermAsk
		}
	}
}

// mirrorEdit — строгое зеркало agents.edit (аудит С1-б): 0.33 с true правит молча и спрашивает
// папку, поэтому true — только если 0.34 разрешает не меньше.
func (p AgentPerms) mirrorEdit() bool {
	for _, k := range flag033Allow {
		if p.Tools[k] != PermAllow {
			return false
		}
	}
	for _, k := range flag033Ask {
		if p.Tools[k] == PermDeny || p.Tools[k] == "" {
			return false
		}
	}
	return true
}

// Mode — записанное значение; "" — инструмента нет в сверенном виде (новее этой версии:
// решает служба через NewMode).
func (p AgentPerms) Mode(tool string) string { return p.Tools[tool] }

// NewMode — значение для инструмента новее этой версии (design §1, последний пункт).
func (p AgentPerms) NewMode(read bool) string {
	if read {
		return PermAllow
	}
	return newToolMode(ClassEdit, p.Tools)
}

// LoadAgentPerms — сверенный вид из config.json.
func LoadAgentPerms(dir string) AgentPerms { return AgentPermsOf(LoadConfig(dir)) }

// UpdateAgentPerms правит разрешения под замком config.json: fn получает сверенный вид, итог
// чистится, Edit пересчитывается зеркалом, а agentPerms и agents.edit пишутся одним UpdateConfig
// (две записи со сбоем между ними давали бы ложный «флажок включили»). Ошибку записи — в журнал.
func UpdateAgentPerms(dir string, fn func(p *AgentPerms)) AgentPerms {
	out, err := UpdateAgentPermsErr(dir, fn)
	if err != nil {
		log.Printf("config.json: %v", paths.NoPath(err))
	}
	return out
}

// UpdateAgentPermsErr — UpdateAgentPerms с ошибкой записи; при ошибке файл прежний целиком.
func UpdateAgentPermsErr(dir string, fn func(p *AgentPerms)) (AgentPerms, error) {
	var out AgentPerms
	err := UpdateConfigErr(dir, func(c *Config) {
		p := AgentPermsOf(*c)
		if c.AgentPerms == nil {
			p.LiftDone = true // ключа нет — мест 0.34 нет, поднимать будет нечего (fn может снять: тест 0.34)
		}
		fn(&p)
		out = p.clean()
		out.Edit = out.mirrorEdit()
		c.AgentPerms = &out
		a := Agents{Level: AgentsMinLevel}
		if c.Agents != nil {
			a = *c.Agents
		}
		a.Edit = out.Edit
		if a = a.clean(); a == (Agents{Level: AgentsMinLevel}) {
			c.Agents = nil
		} else {
			c.Agents = &a
		}
	})
	return out, err
}

// AddPlace ставит место наверх списка (PushPlace) одной записью; негодное строение — без записи.
func AddPlace(dir string, p Place) AgentPerms {
	if _, ok := cleanPlace(p); !ok {
		return LoadAgentPerms(dir)
	}
	return UpdateAgentPerms(dir, func(v *AgentPerms) { v.Places = PushPlace(v.Places, p) })
}

// ForgetPlaces — «Забыть места»: список пуст (заметка о подъёме тоже), разрешения инструментов целы.
func ForgetPlaces(dir string) AgentPerms {
	return UpdateAgentPerms(dir, func(v *AgentPerms) { v.Places, v.Lifted = nil, nil })
}
