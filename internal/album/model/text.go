package model

import (
	"math"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// FrameOrDefault — действующий кадр фото; nil в PhotoRef — без увеличения и сдвига
// (фото закрывает ячейку по центру).
func (r PhotoRef) FrameOrDefault() Frame {
	if r.Frame == nil {
		return Frame{S: 1}
	}
	return *r.Frame
}

// Copy — копия без общих указателей: Frame, Caption (с Pad), Person и Adj, отданные наружу
// (Ref, Refs), не ведут внутрь проекта — правка копии не обходит Editor.change.
func (r PhotoRef) Copy() PhotoRef {
	if r.Frame != nil {
		f := *r.Frame
		r.Frame = &f
	}
	if r.Caption != nil {
		c := *r.Caption
		if c.Pad != nil {
			v := *c.Pad
			c.Pad = &v
		}
		r.Caption = &c
	}
	if r.Person != nil {
		n := *r.Person
		r.Person = &n
	}
	if r.Adj != nil {
		a := *r.Adj
		r.Adj = &a
	}
	return r
}

// FitTexts держит блоки текста на листах (design 008 §2.3). Ширина: «по тексту» —
// autoW, при смене стоит точка выравнивания (anchor); закреплённая — 10..ширина листа.
// Положение: блок целиком внутри листа (высота — по тексту), повёрнутый — по
// описанному прямоугольнику. Идемпотентна: холостая правка остаётся холостой. Блок своим
// шрифтом, которого нет на этом компьютере, не трогается (082-5).
func (p *Project) FitTexts() {
	for _, s := range p.Sheets() {
		g := p.Geo(s)
		for _, t := range s.Texts {
			if fontMissing(t.Style.Font) {
				continue
			}
			if t.Auto {
				p.SetAutoW(s, t, p.SheetAutoW(s, t), false)
			} else {
				t.W = geom.ClampF(t.W, MinTextW, g.W)
			}
			t.X, t.Y = BoxOf(t).FitIn(g.W, g.H) // то же у geom.GroupRot
		}
	}
}

// SetAutoW — ширина «по тексту» блока листа s стала w: блок сдвигается по точке выравнивания
// (повёрнутый — вдоль своей оси). pageFit держит его на странице (pageFitX) и нужен
// только при явной смене (возврат к авто-ширине, стиль, кегль): при вводе текста
// рамку растит редактор от якоря, и зажим у поля заставлял бы её отскакивать.
func (p *Project) SetAutoW(s *Spread, t *TextBlock, w float64, pageFit bool) {
	if w != geom.Round2(t.W) {
		ox := t.X
		dx, dy := geom.Rotate(t.Rot, (t.W-w)*geom.AlignX(t.Style.Align), 0)
		t.X, t.Y = t.X+dx, t.Y+dy
		if pageFit && t.Rot == 0 {
			t.X = pageFitX(p.Geo(s), ox, t.W, t.X, w, geom.AlignX(t.Style.Align), p.Settings.Safe)
		}
	}
	t.W = w
}

// pageFitX — у блока «по тексту» сменилась ширина (ox, ow → x, w). Если старый блок
// целиком стоял на одной странице листа g (в её безопасном поле m или хотя бы на ней), новый
// тоже держим там: сдвиг по точке выравнивания может вытолкнуть его за край страницы
// или листа, и ширина, вернувшаяся «по тексту», прыгала бы. Иначе — x как есть.
func pageFitX(g Geo, ox, ow, x, w, ax, m float64) float64 {
	pg := g.Page(g.PageOf(ox + ow*ax))
	l, r := pg.X, pg.X+pg.W
	for _, in := range []float64{m, 0} { // сначала безопасное поле, потом вся страница
		lo, hi := l+in, r-in-w
		if ox >= lo-1e-6 && ox+ow <= r-in+1e-6 && hi >= lo {
			return geom.ClampF(x, lo, hi)
		}
	}
	return x
}

// Рамка блока текста для geom и ширина «по тексту» (design 008 §3).

// BoxOf — рамка блока t (высота — из кэша переносов).
func BoxOf(t *TextBlock) geom.TextBox {
	return geom.TextBox{X: t.X, Y: t.Y, W: t.W, H: text.Cached(t.Text, t.Style, t.W).H, Rot: t.Rot}
}

// AutoW — ширина блока «по тексту»: естественная, округлённая вверх до 0.01
// и ещё +0.01 (Cached округляет ширину, впритык последнее слово ушло бы на
// новую строку), не меньше 1 мм и не больше maxW. Меряется текст холста (Shown,
// поля листа v; nil — пустые): в печати рамка та же, короче — только строка.
func AutoW(v *Vars, t *TextBlock, maxW float64) float64 {
	s, _ := Shown(v, t.Text, nil, FieldCanvas)
	n := text.NaturalWidth(s, t.Style)
	return geom.Round2(math.Min(math.Max(math.Ceil(n*100-1e-6)/100+0.01, 1), maxW))
}

const (
	// captionK — доля кегля подписи от startSize: подпись — вспомогательный текст,
	// но 9 пт на странице 300 мм (текст там 34 пт) читалась «мизерной». 0,35 даёт
	// 12 пт на 30×30 и 8 пт на 20×20: заметно крупнее прежних 9 пт, но не спорит с текстом.
	captionK = 0.35
	// CaptionMinSize — меньше в печати не читается, пт.
	CaptionMinSize = 8.0
)

// CaptionSize — кегль новой подписи для страницы высотой pageH мм: captionK от
// startSize, кратно 0,5 пт, не меньше captionMinSize (сверху — ClampStyle).
func CaptionSize(pageH float64) float64 {
	return math.Max(CaptionMinSize, math.Round(StartSize(pageH)*captionK*2)/2)
}

// NewCaptionStyle — стиль новой подписи в альбоме с высотой страницы pageH мм.
func NewCaptionStyle(pageH float64) TextStyle {
	return ClampStyle(TextStyle{Font: text.DefaultFont, Size: CaptionSize(pageH), Color: "#555555", Align: "center"})
}

// DefaultCaptionStyle — стиль новой подписи в альбоме по умолчанию (страница 300 мм).
func DefaultCaptionStyle() TextStyle {
	return NewCaptionStyle(DefaultSettings().PageH)
}

// maxSession — длина ключа сессии ввода от интерфейса.
const maxSession = 32

// ValidSession — ключ сессии от интерфейса: 1..maxSession знаков [A-Za-z0-9-].
func ValidSession(session string) bool {
	if session == "" || len(session) > maxSession {
		return false
	}
	for _, c := range session {
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-'
		if !ok {
			return false
		}
	}
	return true
}

// FindText — блок по ID и лист (с обложкой), где он стоит.
func (p *Project) FindText(id string) (*Spread, int) {
	for _, s := range p.Sheets() {
		for i, t := range s.Texts {
			if t.ID == id {
				return s, i
			}
		}
	}
	return nil, -1
}

// DeleteText — тело DeleteText (и DeleteSelection) внутри change.
func (p *Project) DeleteText(textID string) {
	if s, i := p.FindText(textID); s != nil {
		s.Texts = slices.Delete(s.Texts, i, i+1)
	}
}

// plainStyle — стиль нового блока; кегль заменяет startSize (от формата).
var plainStyle = TextStyle{Font: text.DefaultFont, Size: 14, Color: "#222222", Align: "center"}

// PlainStyle — копия plainStyle: снаружи стиль по умолчанию не подменить.
func PlainStyle() TextStyle { return plainStyle }

// StartSize — кегль нового блока, пт: высота кегля ≈ 1/25 высоты страницы
// (300 мм → 34 пт, 200 мм → 23 пт), целый пт; пределы — ClampStyle.
func StartSize(pageH float64) float64 {
	return math.Round(pageH / 25 * 72 / 25.4)
}

// Имя на фото (030, 043): своё или из имени файла (строгое и вольное правило), вид для карточки.

// Name — имя на фото: своё, иначе из имени файла (fromFile). nil — имени нет. Свой только
// предмет (049) — имя из файла с этим предметом: учителю «Иванова Мария.jpg» вписали
// предмет, имя из файла не пропадает.
func (r PhotoRef) Name() (p *Person, fromFile bool) {
	if r.Person != nil && r.Person.Named() {
		return r.Person, false
	}
	if p = PersonFromFile(r.Path); p != nil {
		if r.Person != nil {
			p.Subject = r.Person.Subject
		}
		return p, true
	}
	return r.Person, false
}

// PersonFromFile — имя из имени файла: «Иванов Петя.jpg» → Иванов / Петя, три слова —
// с отчеством. Слово — буквы (внутри дефис и апостроф), первая заглавная; «_» — пробел;
// номер в начале («01_», «1. ») и отметка копии в конце (« (2)», « 2», «-2») не мешают.
// Иначе nil: «DSC_0012», «001», «Свадьба 2025» именем не считаются, как и слово из
// стоп-списка LooksLikeName («Свадьба Ивановых», 0.14 decisions 32). Правило строгое:
// имя берётся неявно, без предпросмотра (043 Р10).
func PersonFromFile(path string) *Person { return personFromBase(fileBase(path), false, false) }

// PersonFromFileLoose — имя для режима «Имена файлов» (043): вдобавок к PersonFromFile
// «.» — пробел, одно слово через дефис делится («Иванов-Петя»), регистр исправляется
// («ИВАНОВ», «иванов» → «Иванов»); firstFirst — «Имя [Отчество] Фамилия». Вольные правила
// действуют только по явному «Вставить» с предпросмотром.
func PersonFromFileLoose(path string, firstFirst bool) *Person {
	return personFromBase(fileBase(path), true, firstFirst)
}

// fileBase — имя файла без папки и расширения; «\» — разделитель и вне Windows. «й», «ё»
// из двух кодовых точек (macOS пишет имена в NFD) собираются в одну: знак не буква, и
// разбор имени не принял бы слово (decisions 31).
func fileBase(path string) string {
	base := path[strings.LastIndexAny(path, `\/`)+1:]
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return ComposeNames(base)
}

// ComposeNames \u2014 \u0431\u0443\u043a\u0432\u044b \u0438\u0437 \u0434\u0432\u0443\u0445 \u043a\u043e\u0434\u043e\u0432\u044b\u0445 \u0442\u043e\u0447\u0435\u043a (NFD) \u043f\u043e \u0442\u0430\u0431\u043b\u0438\u0446\u0435 composeNames \u2014 \u043e\u0434\u043d\u043e\u0439 (NFC);
// \u043f\u0440\u043e\u0447\u0435\u0435 \u043a\u0430\u043a \u0435\u0441\u0442\u044c. \u0421\u0431\u043e\u0440 \u0432 \u043f\u0430\u043f\u043a\u0443 (069) \u0442\u0430\u043a \u0441\u0440\u0430\u0432\u043d\u0438\u0432\u0430\u0435\u0442 \u0438\u043c\u0435\u043d\u0430 \u0444\u0430\u0439\u043b\u043e\u0432: macOS \u0438\u0445 \u043d\u0435 \u0440\u0430\u0437\u043b\u0438\u0447\u0430\u0435\u0442.
func ComposeNames(s string) string {
	if !strings.ContainsAny(s, "\u0306\u0308") {
		return s
	}
	return composeNames.Replace(s)
}

// composeNames — своя таблица NFC для имён без внешнего пакета (decisions 34): у прочих букв
// кириллицы NFD не раскладывается, латиница с диакритикой фильтр письменности и так не проходит.
var composeNames = strings.NewReplacer(
	"\u0438\u0306", "\u0439", "\u0418\u0306", "\u0419", // й, Й
	"\u0435\u0308", "\u0451", "\u0415\u0308", "\u0401", // ё, Ё
	"\u0456\u0308", "\u0457", "\u0406\u0308", "\u0407", // ї, Ї
	"\u0443\u0306", "\u045e", "\u0423\u0306", "\u040e", // ў, Ў
)

// maxFileBase — длина имени файла до снятия номера и отметки копии; после — 3 слова и 2 пробела.
const maxFileBase = 3*MaxPersonRunes + 12

func personFromBase(base string, loose, firstFirst bool) *Person {
	if !utf8.ValidString(base) || utf8.RuneCountInString(base) > maxFileBase {
		return nil
	}
	sep := "_"
	if loose {
		sep = "_."
	}
	base = strings.Map(func(r rune) rune {
		if strings.ContainsRune(sep, r) {
			return ' '
		}
		return r
	}, base)
	base = strings.TrimSpace(stripCopy(stripLead(strings.TrimSpace(base))))
	if utf8.RuneCountInString(base) > 3*MaxPersonRunes+2 {
		return nil
	}
	words := strings.Fields(base)
	if loose && len(words) == 1 {
		if parts := strings.Split(words[0], "-"); len(parts) == 2 || len(parts) == 3 {
			words = parts
		}
	}
	if len(words) < 2 || len(words) > 3 {
		return nil
	}
	for i, w := range words {
		if loose {
			w = fixCase(w)
			words[i] = w
		}
		if !nameWord(w) || !loose && stopWord(w) {
			return nil
		}
	}
	if firstFirst {
		// «Имя [Отчество] Фамилия», как names.person.
		n := len(words) - 1
		words = append([]string{words[n]}, words[:n]...)
	}
	p := Person{Last: words[0], First: words[1]}
	if len(words) == 3 {
		p.Middle = words[2]
	}
	return ClampPerson(p)
}

// stripLead снимает номер в начале: «01 », «1. », «2) », «3 - » или «1.» / «1-» перед буквой.
func stripLead(s string) string {
	i := 0
	for i < len(s) && i <= 4 && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i > 4 {
		return s
	}
	j := skipSpaces(s, i)
	if j < len(s) && strings.IndexByte(".)-", s[j]) >= 0 {
		k := skipSpaces(s, j+1)
		r, _ := utf8.DecodeRuneInString(s[k:])
		if k > j+1 || j == i && unicode.IsLetter(r) {
			return s[k:]
		}
	}
	if j > i {
		return s[j:]
	}
	return s
}

func skipSpaces(s string, i int) int {
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return i
}

// stripCopy снимает отметку копии в конце: « (2)», « 2», «-2» (1–3 цифры; «2025» — не отметка).
func stripCopy(s string) string {
	t, paren := s, strings.HasSuffix(s, ")")
	if paren {
		t = s[:len(s)-1]
	}
	i := len(t)
	for i > 0 && t[i-1] >= '0' && t[i-1] <= '9' {
		i--
	}
	if n := len(t) - i; n == 0 || n > 3 || i < 2 {
		return s
	}
	switch c := t[i-1]; {
	case paren && c == '(' && t[i-2] == ' ':
		return t[:i-2]
	case !paren && (c == ' ' || c == '-'):
		return t[:i-1]
	}
	return s
}

// fixCase — слово целиком строчными или заглавными → «Иванов», каждая часть через дефис
// отдельно («иванова-петрова» → «Иванова-Петрова»); смешанный регистр («МакКартни») — как есть.
func fixCase(w string) string {
	if strings.ToLower(w) != w && strings.ToUpper(w) != w {
		return w
	}
	parts := strings.Split(strings.ToLower(w), "-")
	for i, p := range parts {
		if r, n := utf8.DecodeRuneInString(p); n > 0 {
			parts[i] = string(unicode.ToUpper(r)) + p[n:]
		}
	}
	return strings.Join(parts, "-")
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

// PersonView — имя на фото для карточки: своё или из имени файла (FromFile — подсказка
// серым; предмет и тогда свой). Teacher — фото на месте учителя виньетки: карточка
// показывает поле «Предмет» (049). Place — ID места, чья табличка видна (071): правка имени
// идёт в табличку, а не в фото (у таблички только с предметом — лишь предмет, 071-16); "" —
// имя фото. SeatNamed — табличка видна и в ней есть имя (не только предмет): по нему фронт
// выбирает подсказку «Имя места…» или «Предмет места…» — имя фото под табличкой только с
// предметом в остальном выглядит так же (Place + имя).
type PersonView struct {
	Last      string `json:"last"`
	First     string `json:"first"`
	Middle    string `json:"middle"`
	Subject   string `json:"subject"`
	FromFile  bool   `json:"fromFile"`
	Teacher   bool   `json:"teacher"`
	Place     string `json:"place"`
	SeatNamed bool   `json:"seatNamed"`
}

// NameView — имя фото r для карточки; имени нет — пустые поля. Teacher ставит тот, кто
// знает проект (TeacherSeat).
func NameView(r PhotoRef) PersonView {
	who, file := r.Name()
	if who == nil {
		return PersonView{}
	}
	return PersonView{Last: who.Last, First: who.First, Middle: who.Middle, Subject: who.Subject, FromFile: file}
}

// NameViewAt — видимое имя фото r на месте pl (NameAt) для карточки: при табличке Place — ID
// места. FromFile — имя из файла под табличкой только с предметом (071-16): правка имени идёт в
// фото, как без таблички, и карточка помечает его «из имени файла»; у таблички с именем — false.
func NameViewAt(r PhotoRef, pl *Place) PersonView {
	who, file, seat := NameAt(r, pl)
	if !seat {
		return NameView(r)
	}
	return PersonView{Last: who.Last, First: who.First, Middle: who.Middle, Subject: who.Subject, FromFile: file, Place: pl.ID, SeatNamed: pl.Person.Named()}
}

// Окно «Имена» (043): предпросмотр и вставка считаются одной функцией names.Plan,
// поэтому то, что показано в окне, и ляжет на фото.

// NamesFiles — NamesOpts.From: имена из названий фото листа, а не из списка.
const NamesFiles = "files"

// NamesOpts — окно «Имена»: откуда имена и как читать список.
type NamesOpts struct {
	From       string `json:"from"`       // "list" | "files"; иное — "list"
	Text       string `json:"text"`       // список (From = "list")
	FirstFirst bool   `json:"firstFirst"` // «Имя первым»
	Caption    bool   `json:"caption"`    // подписать фото {Фамилия} {Имя}, если у обычного листа нет шаблона подписи
}

// NamesRow — место листа, получающее имя, в порядке предпросмотра.
type NamesRow struct {
	Place string  `json:"place"` // ID места (виньетка, ручная сетка); "" — лист «Авто»
	Photo string  `json:"photo"` // "" — пустое место (071: имя ляжет табличкой)
	Seat  bool    `json:"seat"`  // имя ляжет табличкой места, а не в фото (071-4)
	Tag   string  `json:"tag"`   // "учитель" | "ученик 3" | "групповое"; "" — не виньетка
	No    int     `json:"no"`    // номер среди получающих; 0 — групповое (пропуск)
	Line  int     `json:"line"`  // строка поля (1 — первая, считая заголовок); 0 — из имени файла или нет
	Who   *Person `json:"who"`   // что ляжет; nil — имя не меняется
}

// NamesLeft — строка списка без фото.
type NamesLeft struct {
	Line   int    `json:"line"`
	Who    Person `json:"who"`
	Reason string `json:"reason"` // "extra" | "nofile" | "many" | "dup" | "noseat" (учитель без места учителя)
	File   string `json:"file"`   // значение столбца «Файл»; "" — нет
}

// NamesPlan — предпросмотр окна; он же план вставки.
type NamesPlan struct {
	Rows         []NamesRow  `json:"rows"`
	Left         []NamesLeft `json:"left"`         // не больше 50; полное число — Extra
	Extra        int         `json:"extra"`        // строк без фото
	Seats        int         `json:"seats"`        // мест, получающих имя: фото «Авто» или места листа (и пустые), без группового
	Named        int         `json:"named"`        // из них с именем после вставки
	Missing      int         `json:"missing"`      // Seats − Named
	Cols         []string    `json:"cols"`         // по столбцам: "Фамилия" | "Имя" | "Отчество" | "ФИО" | "Предмет" | "Файл" | "" (не используется); nil — без заголовка
	Ignored      []string    `json:"ignored"`      // заголовки неиспользуемых столбцов, как в тексте
	ByFile       bool        `json:"byFile"`       // сопоставление по столбцу «Файл»
	Order        bool        `json:"order"`        // «Имя первым» на что-то влияет (нет отдельных столбцов Фамилия/Имя или заголовок ФИО сам задаёт порядок («Фамилия Имя…», решение №47))
	Teachers     int         `json:"teachers"`     // строк-учителей в списке
	Pupils       int         `json:"pupils"`       // остальных строк
	TeacherSeats int         `json:"teacherSeats"` // мест учителей на виньетке, и пустых; не виньетка — 0
	PupilSeats   int         `json:"pupilSeats"`   // мест учеников на виньетке, и пустых; не виньетка — 0
	NeedCaption  bool        `json:"needCaption"`  // не виньетка, есть строки, нет шаблона подписи: окно покажет флажок «Подписать фото»
}
