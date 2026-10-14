package model

import (
	"math"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// Поля в подписях и надписях (спека 030): поля шапки листа, имя на фото, общий
// стиль подписей листа и подстановка {Фамилия}…{Руководитель}.

// Vars — поля листа для {Заголовок} {Подзаголовок} {Год} {Руководитель} (031 dialog §4):
// свойство листа (Spread.Vars), а не альбома — у каждого класса свои (решение 123).
type Vars struct {
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
	Year     string `json:"year,omitempty"`
	Lead     string `json:"lead,omitempty"`
}

// Person — имя человека на фото; nil в PhotoRef — имя из имени файла (лениво, в файл не пишется).
type Person struct {
	Last   string `json:"last,omitempty"`
	First  string `json:"first,omitempty"`
	Middle string `json:"middle,omitempty"`
}

// SheetCaps — общий стиль подписей разворота: перекрывает стиль своих подписей фото листа.
type SheetCaps struct {
	Style TextStyle `json:"style"`
	Pad   float64   `json:"pad"` // мм, 0..MaxPad
	Over  bool      `json:"over,omitempty"`
	Wrap  bool      `json:"wrap,omitempty"` // true — переносить, false — ужимать (по умолчанию)
	Text  string    `json:"text,omitempty"` // шаблон подписи фото без своей; "" — нет
}

// Пределы полей (design 030 §3).
const (
	// DefaultPad — отступ подписи от фото, мм: так было до 030 (Caption.Pad = nil).
	DefaultPad = 1.0
	MaxPad     = 20.0
	// MaxVarRunes — длина поля шапки листа; MaxPersonRunes — одной части имени.
	MaxVarRunes    = 120
	MaxPersonRunes = 60
	// maxFieldName — длина имени поля в скобках: длиннее — не поле, копируется как есть.
	maxFieldName = 20
)

// ClampVars — поля шапки из файла или интерфейса: одна строка, ≤ MaxVarRunes; все пусты — nil.
func ClampVars(v Vars) *Vars {
	c := Vars{
		Title:    cleanLine(v.Title, MaxVarRunes),
		Subtitle: cleanLine(v.Subtitle, MaxVarRunes),
		Year:     cleanLine(v.Year, MaxVarRunes),
		Lead:     cleanLine(v.Lead, MaxVarRunes),
	}
	if c == (Vars{}) {
		return nil
	}
	return &c
}

// ClampPerson — имя из файла или интерфейса: три одностроечных поля ≤ MaxPersonRunes; пусто — nil.
func ClampPerson(p Person) *Person {
	c := Person{
		Last:   cleanLine(p.Last, MaxPersonRunes),
		First:  cleanLine(p.First, MaxPersonRunes),
		Middle: cleanLine(p.Middle, MaxPersonRunes),
	}
	if c == (Person{}) {
		return nil
	}
	return &c
}

// ClampPad — отступ подписи: NaN/Inf — DefaultPad, 0..MaxPad, шаг 0.1.
func ClampPad(v float64) float64 {
	return Round1(geom.ClampF(Finite(v, DefaultPad), 0, MaxPad))
}

// ClampCaptionPad — отступ своей подписи: nil, NaN/Inf и DefaultPad — nil (прежние
// файлы и сброс не оставляют следа, выбор того же отступа — холостая правка).
func ClampCaptionPad(v *float64) *float64 {
	if v == nil || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return nil
	}
	c := ClampPad(*v)
	if c == DefaultPad {
		return nil
	}
	return &c
}

// Зажим при Load (ClampDesign): новые строки списываются из общего запаса знаков
// проекта; CheckTotals считает их тем же обходом spendTexts, иначе после Save+Load лишнее пропало бы.

func runes(ss ...string) int {
	n := 0
	for _, s := range ss {
		n += utf8.RuneCountInString(s)
	}
	return n
}

func (v Vars) runes() int   { return runes(v.Title, v.Subtitle, v.Year, v.Lead) }
func (p Person) runes() int { return runes(p.Last, p.First, p.Middle) }

func clampVarsIn(v *Vars, b *textBudget) *Vars {
	if v == nil {
		return nil
	}
	if c := ClampVars(*v); c != nil && b.takeRunes(c.runes()) {
		return c
	}
	return nil
}

func clampPersonIn(p *Person, b *textBudget) *Person {
	if p == nil {
		return nil
	}
	if c := ClampPerson(*p); c != nil && b.takeRunes(c.runes()) {
		return c
	}
	return nil
}

// clampCapsIn — стиль листа остаётся и без запаса знаков: теряется только шаблон. Шаблон
// списывается по cost — на все его показы (tplCost).
func clampCapsIn(c *SheetCaps, cost func(string) int, b *textBudget) *SheetCaps {
	if c == nil {
		return nil
	}
	cc := ClampSheetCaps(*c)
	if !b.takeRunes(cost(cc.Text)) {
		cc.Text = ""
	}
	return &cc
}

// PadOf — действующий отступ подписи, мм.
func (c Caption) PadOf() float64 {
	if c.Pad == nil {
		return DefaultPad
	}
	return *c.Pad
}

// ClampSheetCaps — стиль листа из файла или интерфейса: стиль — ClampStyle, отступ —
// ClampPad, шаблон — CleanText (≤ MaxCaptionRunes, пробельный — "").
func ClampSheetCaps(c SheetCaps) SheetCaps {
	c.Style = ClampStyle(c.Style)
	c.Pad = ClampPad(c.Pad)
	c.Text = CleanText(c.Text, MaxCaptionRunes)
	if strings.TrimSpace(c.Text) == "" {
		c.Text = ""
	}
	return c
}

// Name — имя на фото: своё, иначе из имени файла (fromFile). nil — имени нет.
func (r PhotoRef) Name() (p *Person, fromFile bool) {
	if r.Person != nil {
		return r.Person, false
	}
	if p = PersonFromFile(r.Path); p != nil {
		return p, true
	}
	return nil, false
}

// PersonFromFile — имя из имени файла: «Иванов Петя.jpg» → Иванов / Петя, три слова —
// с отчеством. Слово — буквы (внутри дефис и апостроф), первая заглавная; «_» — пробел.
// Иначе nil: «DSC_0012», «001», «Свадьба 2025» именем не считаются.
func PersonFromFile(path string) *Person {
	base := filepath.Base(strings.ReplaceAll(path, `\`, "/"))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if utf8.RuneCountInString(base) > 3*MaxPersonRunes+2 || !utf8.ValidString(base) {
		return nil
	}
	words := strings.Fields(strings.ReplaceAll(base, "_", " "))
	if len(words) < 2 || len(words) > 3 {
		return nil
	}
	for _, w := range words {
		if !nameWord(w) {
			return nil
		}
	}
	p := Person{Last: words[0], First: words[1]}
	if len(words) == 3 {
		p.Middle = words[2]
	}
	return ClampPerson(p)
}

// nameWord — слово имени: заглавная буква, дальше буквы, дефис или апостроф
// (не два знака подряд и не в конце): «Иванова-Петрова», «Д'Артаньян».
func nameWord(w string) bool {
	prevSep := false
	for i, r := range w {
		switch {
		case i == 0:
			if !unicode.IsUpper(r) {
				return false
			}
		case unicode.IsLetter(r):
			prevSep = false
		case r == '-' || r == '\'' || r == '’':
			if prevSep {
				return false
			}
			prevSep = true
		default:
			return false
		}
	}
	return !prevSep
}

// FieldMode — как показывать пустое поле.
type FieldMode int

const (
	// FieldCanvas — холст: пустое поле видно как «[Заголовок]».
	FieldCanvas FieldMode = iota
	// FieldPrint — вид печати (экспорт): пустое поле — пусто.
	FieldPrint
	// FieldStub — заглушка пустого места: имя всегда «[Фамилия]», переменные — как на холсте.
	FieldStub
)

// FieldCtx — откуда брать значения: поля шапки листа и (у подписи) имя фото.
type FieldCtx struct {
	Vars   Vars
	Person *Person
	Mode   FieldMode
}

// fieldNames — поля в порядке спеки: имя для «[…]» и предупреждения экспорта. Первые
// nameFields — имя на фото, остальные — поля шапки листа.
var fieldNames = [...]string{"Фамилия", "Имя", "Отчество", "Заголовок", "Подзаголовок", "Год", "Руководитель"}

const nameFields = 3

// fieldIndex — индекс поля по имени без регистра, «ё» = «е»; −1 — не поле. Прежние имена
// полей шапки (030) — синонимы: набранные {Школа}, {Класс}, {Учитель} работают и дальше
// (решение 124), в «[…]» — каноническое имя.
func fieldIndex(name string) int {
	key := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(name)), "ё", "е")
	for i, n := range fieldNames {
		if strings.ToLower(n) == key {
			return i
		}
	}
	switch key {
	case "школа":
		return 3
	case "класс":
		return 4
	case "учитель":
		return 6
	}
	return -1
}

// value — значение поля i; Stub всегда без имени.
func (c FieldCtx) value(i int) string {
	var pr Person
	if c.Person != nil && c.Mode != FieldStub {
		pr = *c.Person
	}
	return [...]string{pr.Last, pr.First, pr.Middle, c.Vars.Title, c.Vars.Subtitle, c.Vars.Year, c.Vars.Lead}[i]
}

// HasHeadFields — в тексте s есть поле шапки листа (или прежнее имя): по нему Load решает,
// куда перенести поля альбома из файла dev-сборки (решение 125).
func HasHeadFields(s string) bool {
	for {
		i := strings.IndexByte(s, '{')
		if i < 0 {
			return false
		}
		s = s[i+1:]
		if name, ok := fieldAt(s); ok && fieldIndex(name) >= nameFields {
			return true
		}
	}
}

// HasFields — в тексте может быть поле: без «{» Expand возвращает текст как есть.
func HasFields(s string) bool { return strings.IndexByte(s, '{') >= 0 }

// Expand подставляет поля в текст s за один проход слева направо: значения повторно не
// раскрываются, неизвестное «{…}» остаётся как есть. Пустое поле — по c.Mode; missing —
// имена пустых полей по порядку первого появления. Пустые строки в конце убираются
// (у фото без отчества нет пустой третьей строки), результат ≤ MaxTextRunes знаков.
func Expand(s string, c FieldCtx) (out string, missing []string) {
	if !HasFields(s) {
		return s, nil
	}
	s = strings.ToValidUTF8(s, string(utf8.RuneError))
	var b strings.Builder
	n, seen := 0, 0 // seen — биты уже названных пустых полей
	put := func(v string) bool {
		k := utf8.RuneCountInString(v)
		if n+k > MaxTextRunes {
			v, k = string([]rune(v)[:MaxTextRunes-n]), MaxTextRunes-n
		}
		b.WriteString(v)
		n += k
		return n < MaxTextRunes
	}
	for i := 0; i < len(s); {
		if s[i] != '{' {
			j := strings.IndexByte(s[i:], '{')
			if j < 0 {
				j = len(s) - i
			}
			if !put(s[i : i+j]) {
				break
			}
			i += j
			continue
		}
		name, ok := fieldAt(s[i+1:])
		f := -1
		if ok {
			f = fieldIndex(name)
		}
		if f < 0 {
			if !put("{") {
				break
			}
			i++
			continue
		}
		v := c.value(f)
		if strings.TrimSpace(v) == "" {
			if seen&(1<<f) == 0 {
				seen |= 1 << f
				missing = append(missing, fieldNames[f])
			}
			v = ""
			if c.Mode != FieldPrint {
				v = "[" + fieldNames[f] + "]"
			}
		}
		i += len(name) + 2
		if !put(v) {
			break
		}
	}
	return trimBlankTail(b.String()), missing
}

// fieldAt — имя поля в начале s (после «{») до «}»: буквы и пробелы, ≤ maxFieldName знаков.
func fieldAt(s string) (string, bool) {
	k := 0
	for i, r := range s {
		switch {
		case r == '}':
			return s[:i], i > 0
		case k >= maxFieldName || !unicode.IsLetter(r) && r != ' ':
			return "", false
		}
		k++
	}
	return "", false
}

// trimBlankTail убирает пустые (из пробелов) строки в конце текста.
func trimBlankTail(s string) string {
	for {
		i := strings.LastIndexByte(s, '\n')
		if i < 0 || strings.TrimSpace(s[i+1:]) != "" {
			return s
		}
		s = s[:i]
	}
}

// Shown — текст s для показа: поля подставлены из полей шапки листа v (p.VarsOf; nil —
// все пусты) и имени who (nil — у надписи имени нет). Одна точка для вида всех листов,
// подписей, заглушек и ширины «по тексту» (AutoW): холст и печать не расходятся в подстановке.
func Shown(v *Vars, s string, who *Person, mode FieldMode) (string, []string) {
	if !HasFields(s) {
		return s, nil
	}
	c := FieldCtx{Person: who, Mode: mode}
	if v != nil {
		c.Vars = *v
	}
	return Expand(s, c)
}

// ShownCaption — Shown для подписи фото и заглушки: не длиннее MaxCaptionRunes знаков.
// Шаблон листа показывается у каждого места, а поля раскрываются до MaxTextRunes: без
// обрезки «{Год}» ×60 дало бы у каждого места по тысяче знаков (decisions 79).
func ShownCaption(v *Vars, s string, who *Person, mode FieldMode) (string, []string) {
	out, miss := Shown(v, s, who, mode)
	if utf8.RuneCountInString(out) > MaxCaptionRunes {
		out = trimBlankTail(string([]rune(out)[:MaxCaptionRunes]))
	}
	return out, miss
}

// IsNameField — поле имени («Фамилия», «Имя», «Отчество»), а не поле шапки листа.
func IsNameField(name string) bool {
	i := fieldIndex(name)
	return i >= 0 && i < nameFields
}

// nameKeys — ключи полей имени для UI (порядок fieldNames).
var nameKeys = [nameFields]string{"last", "first", "middle"}

// NameFieldsOf — поля имени шаблона s в порядке появления, без повторов: "last", "first",
// "middle" (синонимы и регистр — как в Expand). Нет полей имени — nil. По ним окошко
// «Имя» на холсте (037) знает, какие поля показать.
func NameFieldsOf(s string) []string {
	var out []string
	for seen := 0; ; {
		i := strings.IndexByte(s, '{')
		if i < 0 {
			return out
		}
		s = s[i+1:]
		if name, ok := fieldAt(s); ok {
			if f := fieldIndex(name); f >= 0 && f < nameFields && seen&(1<<f) == 0 {
				seen |= 1 << f
				out = append(out, nameKeys[f])
			}
		}
	}
}

// VarsOf — поля шапки листа s: свои; у обложки и обычного разворота без своих — первой по
// порядку виньетки альбома, у которой они есть (школьная обложка показывает школу, свадебный
// альбом без виньеток полей не видит); у виньетки — только свои. nil — все пусты.
// Виньетка — по Kind, а не IsVignette: при Load (ClampDesign до FixVignettes) Vig ещё может
// не быть, а запас знаков (tplCost) обязан считаться так же, как потом в CheckTotals.
func (p *Project) VarsOf(s *Spread) *Vars {
	switch {
	case s == nil:
		return nil
	case s.Vars != nil || s.Kind == KindVignette:
		return s.Vars
	}
	return p.firstVars()
}

// firstVars — поля первой по порядку виньетки, у которой они есть; nil — нет такой.
func (p *Project) firstVars() *Vars {
	for _, v := range p.Spreads {
		if v.Kind == KindVignette && v.Vars != nil {
			return v.Vars
		}
	}
	return nil
}

// SheetVars — VarsOf для обхода всех листов (вид, запас знаков): поля первой виньетки
// ищутся один раз, а не у каждого листа — на 1000 разворотах без виньеток это миллион шагов.
// Годится, пока виньетки и их поля не меняются.
func (p *Project) SheetVars() func(*Spread) *Vars {
	first := p.firstVars()
	return func(s *Spread) *Vars {
		if s == nil || s.Vars != nil || s.Kind == KindVignette {
			return p.VarsOf(s)
		}
		return first
	}
}

// FieldsOf — имена для меню «Вставить поле» на листе s с полями шапки v (VarsOf): имя на
// фото всегда, поля шапки — у виньетки и у листа, которому есть что подставить.
func FieldsOf(s *Spread, v *Vars) []string {
	n := nameFields
	if s != nil && (s.IsVignette() || v != nil) {
		n = len(fieldNames)
	}
	return slices.Clone(fieldNames[:n])
}

// PersonView — имя на фото для карточки: своё или из имени файла (FromFile — подсказка серым).
type PersonView struct {
	Last     string `json:"last"`
	First    string `json:"first"`
	Middle   string `json:"middle"`
	FromFile bool   `json:"fromFile"`
}

// NameView — имя фото r для карточки; имени нет — пустые поля.
func NameView(r PhotoRef) PersonView {
	who, file := r.Name()
	if who == nil {
		return PersonView{}
	}
	return PersonView{Last: who.Last, First: who.First, Middle: who.Middle, FromFile: file}
}
