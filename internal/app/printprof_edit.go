package app

import (
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/export"
)

// Окно «Своя типография» (123 Б1, design §4, §7): поля, живой расчёт и запись. Числа и ошибки
// считает Go, интерфейс только показывает: страница из px — model.PageFromPx, сторонка обложки
// из px — model.SideFromPx (тем же корешком, что у встроенных), изделия — копия основы.

// PrintEdit — поля окна. Числа — как ввёл пользователь; id у своей всегда новый (SavePrintProfile).
type PrintEdit struct {
	Name    string            `json:"name"`
	Base    string            `json:"base"` // id типографии-основы (изделия, корешки, пределы); "" — без обложки
	DPI     int               `json:"dpi"`
	Layflat bool              `json:"layflat"`
	Safe    float64           `json:"safe"`  // безопасная зона страницы, мм
	Bleed   float64           `json:"bleed"` // обрез, мм
	Formats []PrintEditFormat `json:"formats"`
	Spreads int               `json:"spreads"` // число разворотов, при котором введены px обложки (корешок); 0 — как в альбоме
	Prefix  string            `json:"prefix"`  // начало имён файлов выгрузки: «1-»
	Quality int               `json:"quality"` // JPEG при выгрузке: 100 | 95 | 0 («как выбрано в окне»)
	URL     string            `json:"url"`     // сайт типографии, https; "" — без кнопки
}

// PrintEditFormat — строка «подпись · разворот px · обложка px».
type PrintEditFormat struct {
	Name   string `json:"name"`
	PxW    int    `json:"pxW"` // файл разворота
	PxH    int    `json:"pxH"`
	CoverW int    `json:"coverW"` // файл обложки; 0 и 0 — формат без обложки по шаблону
	CoverH int    `json:"coverH"`
}

// PrintEditCheck — расчёт окна: чистая функция полей, проект и файлы не трогаются.
type PrintEditCheck struct {
	OK      bool              `json:"ok"`      // можно сохранять
	Errs    map[string]string `json:"errs"`    // поле → текст под полем: name, base, dpi, safe, bleed, formats, spreads, prefix, quality, url, form
	Formats []PrintCheckRow   `json:"formats"` // по строке формата
	Spine   float64           `json:"spine"`   // корешок основы при Spreads, мм; 0 — нет
	Lines   []string          `json:"lines"`   // «Сейчас альбом даёт …»
	Names   string            `json:"names"`   // пример имён: «1-00.jpg — обложка, 1-01.jpg…1-12.jpg»
}

// PrintCheckRow — итог строки формата: страница и сторонка в мм из px либо ошибка под строкой.
// Признаки для окна даёт Go, а не разбор текста ошибки: Field — какое поле строки неверно, Hit —
// строка совпала с тем, что даёт альбом сейчас (тогда Lines[0] кончается « — это формат «…»»).
type PrintCheckRow struct {
	PageW float64 `json:"pageW"`
	PageH float64 `json:"pageH"`
	SideW float64 `json:"sideW"` // 0 — обложки нет
	SideH float64 `json:"sideH"`
	Err   string  `json:"err"`
	Field string  `json:"field"` // при Err: name — подпись, px — разворот px, cover — обложка px; "" — ошибки нет
	Hit   bool    `json:"hit"`   // разворот px строки = разворот альбома при dpi окна (первая такая строка)
}

// Поля строки формата с ошибкой (PrintCheckRow.Field).
const (
	rowName  = "name"
	rowPx    = "px"
	rowCover = "cover"
)

// PrintSaved — итог «Сохранить»: id новой типографии и вид альбома после применения.
type PrintSaved struct {
	ID   string     `json:"id"`
	View model.View `json:"view"`
}

const (
	printNewID   = "own-00000000" // id-заглушка для расчёта: настоящий выдаёт Save
	maxPrintRows = 32
)

// PrintProfileEdit — заготовка окна на основе типографии base (встроенной, библиотечной или
// своей): её форматы в px, зона, обрез, имена файлов, сайт. base "" — пустая заготовка без
// изделий. Название — «<основа> (своя)», свободное в реестре.
func (a *App) PrintProfileEdit(base string) (e PrintEdit, err error) {
	defer guardErr("PrintProfileEdit", &err)
	snap := printprof.Current()
	p, _ := a.editor().SnapshotSide()
	e = PrintEdit{DPI: model.PrintDPI, Safe: 10, Bleed: 3, Quality: 100, Spreads: max(1, len(p.Spreads)), Formats: []PrintEditFormat{}}
	if base == "" {
		return e, nil
	}
	bp, ok := snap.ByID(base)
	if !ok {
		return e, fmt.Errorf("типографии %q нет", clipText(base))
	}
	e.Base, e.Name = base, freeName(snap, bp.Name)
	e.DPI, e.Layflat, e.Safe, e.Bleed = bp.Page.DPI, bp.Page.Layflat, bp.Page.Safe, bp.Page.Bleed
	if ex := bp.Export; ex != nil {
		e.Prefix, e.URL = ex.Prefix, idnShow(ex.URL) // хост — Юникодом, если безопасен (0.26-123-52)
		if ex.Quality != 0 {
			e.Quality = ex.Quality
		}
	}
	for _, f := range bp.Formats {
		e.Formats = append(e.Formats, editFormat(bp, f, e.DPI, e.Spreads))
	}
	return e, nil
}

// editFormat — строка окна из формата профиля: px цели или рассчитанные из мм, обложка — лист
// при n разворотах.
func editFormat(bp printprof.Profile, f printprof.PageFormat, dpi, n int) PrintEditFormat {
	ef := PrintEditFormat{Name: f.Name, PxW: model.Px(2*f.Page[0], dpi), PxH: model.Px(f.Page[1], dpi)}
	if f.Px != nil {
		ef.PxW, ef.PxH = f.Px[0], f.Px[1]
	}
	if spec, ok := coverSpecOf(bp, f.Name); ok && f.Side != nil {
		ef.CoverW, ef.CoverH = coverPx(spec, f.Side[0], f.Side[1], n, dpi)
	}
	return ef
}

// ownTail — хвост названия своей: « (своя)» или « (своя N)».
var ownTail = regexp.MustCompile(` \(своя(?: [0-9]+)?\)$`)

// freeName — «<имя> (своя)», при занятом — «(своя 2)», …; обрезано до MaxName знаков. Основа —
// уже своя («Альфа (своя)»): её хвост срезается, иначе вышло бы «Альфа (своя) (своя)».
func freeName(snap *printprof.Snapshot, base string) string {
	base = ownTail.ReplaceAllString(base, "")
	for i := 1; ; i++ {
		suffix := " (своя)"
		if i > 1 {
			suffix = fmt.Sprintf(" (своя %d)", i)
		}
		n := string([]rune(base)[:min(utf8.RuneCountInString(base), printprof.MaxName-utf8.RuneCountInString(suffix))]) + suffix
		if !nameTaken(snap, n, "") {
			return n
		}
	}
}

// coverSpecOf — числа обложки основы для формата: заворот формата шаблона (иначе изделия или
// общий), расстав, корешок формата; false — у основы нет изделия по умолчанию.
func coverSpecOf(bp printprof.Profile, format string) (model.CoverSpec, bool) {
	i := slices.IndexFunc(bp.Products, func(pr printprof.Product) bool { return pr.ID == bp.Default })
	if i < 0 {
		return model.CoverSpec{}, false
	}
	pr := bp.Products[i]
	spec := model.CoverSpec{Wrap: pr.Wrap, Hinge: pr.Hinge, A: pr.A, B: pr.B}
	if c := bp.Cover; c != nil && spec.Wrap == 0 {
		spec.Wrap = c.Wrap
	}
	if bf, ok := bp.FormatByName(format); ok && bf.Template() {
		spec.Wrap = bf.Wrap
	}
	for _, s := range printprof.Steps(pr.SpineFor(format)) {
		spec.Steps = append(spec.Steps, model.SpineStep{Upto: s.Upto, Mm: s.Mm})
	}
	return spec, true
}

// coverPx — размер листа обложки в px при n разворотах: та же цепочка, что у выгрузки (CoverGeo).
func coverPx(spec model.CoverSpec, sw, sh float64, n, dpi int) (w, h int) {
	spec.W, spec.H = sw, sh
	p := model.New()
	p.Spreads = make([]*model.Spread, n)
	p.Cover = &model.Cover{Spec: spec}
	g := p.CoverGeo()
	return model.Px(g.X[7], dpi), model.Px(g.Y[3], dpi)
}

// CheckPrintProfile — живой расчёт окна: ошибки полей, мм на строку формата (страница из px,
// сторонка обложки из px), корешок, строки «Сейчас альбом даёт…» и пример имён файлов.
func (a *App) CheckPrintProfile(e PrintEdit) (c PrintEditCheck) {
	defer guard("CheckPrintProfile")
	p, _ := a.editor().SnapshotSide()
	b := buildOwn(e, printNewID, printprof.Current(), len(p.Spreads))
	c = PrintEditCheck{OK: len(b.errs) == 0, Errs: b.errs, Spine: b.spine, Lines: []string{}}
	c.Lines = albumLines(p, &b, e.Formats)
	c.Formats = b.rows
	c.Names = nameSample(e.Prefix, max(1, len(p.Spreads)))
	return c
}

// nameSample — «1-00.jpg — обложка, 1-01.jpg…1-12.jpg»: имена файлов по префиксу.
func nameSample(prefix string, n int) string {
	if !validPrefix(prefix) {
		return ""
	}
	first, last := export.FileName(0, n), export.FileName(n-1, n)
	if n == 1 {
		return prefix + "00.jpg — обложка, " + prefix + first
	}
	return prefix + "00.jpg — обложка, " + prefix + first + "…" + prefix + last
}

// validPrefix — префикс имён по грамматике профиля (до 8 знаков: латиница, цифры, «-», «_»).
func validPrefix(s string) bool {
	if len(s) > 8 {
		return false
	}
	for _, r := range s {
		if r > unicode.MaxASCII || (r != '-' && r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

// albumLines — что даёт текущий альбом при dpi и нужных формата: разворот, обложка, совпавший
// формат — первая годная строка fs с теми же px разворота; ей ставится Hit.
func albumLines(p *model.Project, b *ownBuild, fs []PrintEditFormat) []string {
	dpi := b.dpi
	if dpi <= 0 || p == nil {
		return []string{}
	}
	page := p.Geo(nil).Pages[0] // страницу читают через Geo (TestPageWReaders)
	nw, nh := model.Px(2*page.W, dpi), model.Px(page.H, dpi)
	line := fmt.Sprintf("Сейчас альбом даёт: разворот %d × %d px при %d dpi", nw, nh, dpi)
	for i, f := range fs {
		if i < len(b.rows) && b.rows[i].Err == "" && f.PxW == nw && f.PxH == nh {
			b.rows[i].Hit = true
			line += " — это формат «" + strings.TrimSpace(f.Name) + "»"
			break
		}
	}
	out := []string{line}
	if p.Cover != nil {
		g := p.CoverGeo()
		out = append(out, fmt.Sprintf("Обложка альбома: %d × %d px при %d разворотах (корешок %s мм)",
			model.Px(g.X[7], dpi), model.Px(g.Y[3], dpi), g.N, model.MmText(g.Spine)))
	}
	return out
}

// ownBuild — результат сборки своей типографии из полей: профиль, строки формата и ошибки.
type ownBuild struct {
	p     printprof.Profile
	rows  []PrintCheckRow
	errs  map[string]string
	spine float64
	dpi   int // 0 — плотность негодна: «Сейчас альбом даёт» не считается
}

// buildOwn собирает профиль своей типографии id из полей e на основе e.Base реестра snap.
// Ошибки — по полям в errs; профиль годен только при пустых errs.
func buildOwn(e PrintEdit, id string, snap *printprof.Snapshot, albumSpreads int) ownBuild {
	b := ownBuild{errs: map[string]string{}, rows: []PrintCheckRow{}}
	name := strings.TrimSpace(e.Name)
	switch {
	case name == "":
		b.errs["name"] = "Введите название"
	case utf8.RuneCountInString(name) > printprof.MaxName:
		b.errs["name"] = fmt.Sprintf("Не длиннее %d знаков", printprof.MaxName)
	default:
		if err := nameClash(snap, name, ""); err != nil {
			b.errs["name"] = err.Error()
		}
	}
	var base printprof.Profile
	if e.Base != "" {
		var ok bool
		if base, ok = snap.ByID(e.Base); !ok {
			b.errs["base"] = "Типографии-основы нет"
		}
	}
	if u, why := idnURL(e.URL); why != "" {
		b.errs["url"] = why
	} else {
		e.URL = u // хост в xn--…: дальше адрес идёт в Validate и файл только так
	}
	b.checkPage(e)
	n := e.Spreads
	if n <= 0 {
		n = max(1, albumSpreads)
	}
	if n > model.MaxSpreads {
		b.errs["spreads"] = fmt.Sprintf("Не больше %d разворотов", model.MaxSpreads)
	}
	formats := b.formats(e, base, n)
	if len(b.errs) > 0 {
		return b
	}
	b.p = assemble(e, id, name, base, formats)
	if err := b.p.Validate(); err != nil {
		field, text := fieldOfErr(err)
		b.errs[field] = text
	}
	return b
}

// checkPage — плотность, зона, обрез, имена, качество и сайт; dpi запоминается для строк альбома.
func (b *ownBuild) checkPage(e PrintEdit) {
	if e.DPI >= 72 && e.DPI <= 1200 {
		b.dpi = e.DPI
	} else {
		b.errs["dpi"] = "Плотность от 72 до 1200 dpi"
	}
	if !(e.Safe >= 0 && e.Safe <= 30) {
		b.errs["safe"] = "Зона от 0 до 30 мм"
	}
	if !(e.Bleed >= 0 && e.Bleed <= 10) {
		b.errs["bleed"] = "Обрез от 0 до 10 мм"
	}
	if !validPrefix(e.Prefix) {
		b.errs["prefix"] = "До 8 знаков: латиница, цифры, «-» и «_»"
	}
	if e.Quality != 0 && e.Quality != 95 && e.Quality != 100 {
		b.errs["quality"] = "Качество 100 или 95"
	}
}

// formats — формат за форматом: страница из px файла разворота, сторонка из px обложки.
func (b *ownBuild) formats(e PrintEdit, base printprof.Profile, n int) []printprof.PageFormat {
	if len(e.Formats) == 0 || len(e.Formats) > maxPrintRows {
		b.errs["formats"] = fmt.Sprintf("Нужно от 1 до %d форматов", maxPrintRows)
		return nil
	}
	var out []printprof.PageFormat
	seen := map[string]bool{}
	for _, ef := range e.Formats {
		row := PrintCheckRow{}
		f, field, err := b.format(ef, base, n, seen, &row)
		if err != "" {
			row.Err, row.Field = err, field
			b.errs["formats"] = "Исправьте строки форматов"
		}
		b.rows = append(b.rows, row)
		out = append(out, f)
	}
	if spec, ok := coverSpecOf(base, ""); ok {
		b.spine, _ = spec.Spine(n)
	}
	return out
}

// format — один формат; err — текст под строкой ("" — годна), field — поле строки с ошибкой
// (rowName, rowPx, rowCover). Плотность негодна — строка без чисел и без своей ошибки: ошибка одна,
// у поля dpi (QA-2), а не «Сначала плотность» под каждой строкой.
func (b *ownBuild) format(ef PrintEditFormat, base printprof.Profile, n int, seen map[string]bool, row *PrintCheckRow) (printprof.PageFormat, string, string) {
	name := strings.TrimSpace(ef.Name)
	f := printprof.PageFormat{Name: name}
	switch {
	case name == "":
		return f, rowName, "Введите подпись"
	case utf8.RuneCountInString(name) > printprof.MaxName:
		return f, rowName, fmt.Sprintf("Подпись не длиннее %d знаков", printprof.MaxName)
	case seen[name]:
		return f, rowName, "Подпись повторена"
	}
	seen[name] = true
	if b.dpi == 0 {
		return f, "", ""
	}
	pw, ph, err := model.PageFromPx(ef.PxW, ef.PxH, b.dpi)
	if err != nil {
		return f, rowPx, upperFirst(err.Error())
	}
	f.Page, f.Px = printprof.Pair{pw, ph}, &printprof.PxPair{ef.PxW, ef.PxH}
	row.PageW, row.PageH = pw, ph
	if ef.CoverW == 0 && ef.CoverH == 0 {
		return f, "", ""
	}
	spec, ok := coverSpecOf(base, name)
	switch {
	case !ok:
		return f, rowCover, "Для обложки нужна основа с изделиями"
	case ef.CoverW <= 0 || ef.CoverH <= 0:
		return f, rowCover, "Укажите обе стороны обложки или ни одной"
	}
	spine, _ := spec.Spine(n)
	sw, sh, err := model.SideFromPx(ef.CoverW, ef.CoverH, b.dpi, spec, spine)
	if err != nil {
		return f, rowCover, upperFirst(err.Error())
	}
	f.Side, f.Wrap = &printprof.Pair{sw, sh}, spec.Wrap
	row.SideW, row.SideH = sw, sh
	return f, "", ""
}

// assemble — профиль: страница и форматы из полей, изделия, корешки и пределы — копия основы
// (SpineBy и Limits только для оставшихся подписей форматов), тексты выгрузки — из названия.
func assemble(e PrintEdit, id, name string, base printprof.Profile, formats []printprof.PageFormat) printprof.Profile {
	safe := ""
	if e.Safe > 0 {
		safe = "; важное — не ближе {safe} мм к краю"
	}
	p := printprof.Profile{
		Format: printprof.FormatVersion, ID: id, Version: 1, Name: name, Gen: name,
		Page: printprof.Page{DPI: e.DPI, Layflat: e.Layflat, Safe: e.Safe, Bleed: e.Bleed,
			Hint: fmt.Sprintf("%s: JPEG sRGB %d dpi; %s00.jpg — обложка, %s01.jpg… — развороты%s", name, e.DPI, e.Prefix, e.Prefix, safe)},
		Formats:     formats,
		NameDefault: base.NameDefault,
		Export:      exportOf(e, name),
	}
	if base.Cover != nil {
		c := *base.Cover
		p.Cover = &c
	}
	names := make([]string, len(formats))
	for i, f := range formats {
		names[i] = f.Name
	}
	var m map[string]string
	p.Products, m = copyProducts(base, id, names)
	p.Default = m[base.Default]
	return p
}

// copyProducts — изделия основы под id своей: own-<id>-<n>, адрес сайта изделия не переносится
// (сайт — поле окна). Таблицы и пределы форматов, которых нет среди formats, отбрасываются.
func copyProducts(base printprof.Profile, id string, formats []string) ([]printprof.Product, map[string]string) {
	var out []printprof.Product
	renamed := map[string]string{}
	for i, pr := range base.Products {
		np := pr
		np.ID, np.URL = printprof.OwnProductID(id, i+1), ""
		np.Spine = slices.Clone(pr.Spine)
		np.SpineBy = nil
		for f, t := range pr.SpineBy {
			if slices.Contains(formats, f) {
				if np.SpineBy == nil {
					np.SpineBy = map[string][]float64{}
				}
				np.SpineBy[f] = slices.Clone(t)
			}
		}
		np.Limits = slices.DeleteFunc(slices.Clone(pr.Limits), func(l printprof.Limit) bool {
			return l.Format != "" && !slices.Contains(formats, l.Format)
		})
		if len(np.Limits) == 0 {
			np.Limits = nil
		}
		renamed[pr.ID] = np.ID
		out = append(out, np)
	}
	return out, renamed
}

// exportOf — выгрузка своей: имена файлов и качество — из полей, слова памятки — из названия.
func exportOf(e PrintEdit, name string) *printprof.Export {
	ex := &printprof.Export{
		Acc: "«" + name + "»", Title: "Для «" + name + "»", Prefix: e.Prefix, Quality: e.Quality,
		CoverWord: "обложки", PagesCap: "Разворот: ", Plain: "Фотокнига",
		Done: "Готово для «" + name + "»", Site: "Сайт типографии → {item} → эти параметры",
		SiteNoItem: "изделие", SiteButton: "Открыть сайт типографии", URL: strings.TrimSpace(e.URL),
	}
	if u, err := url.Parse(ex.URL); err == nil && ex.URL != "" && u.Hostname() != "" {
		ex.SiteButton = "Открыть " + printprof.DisplayHost(u.Hostname()) // кнопка называет хост: куда она ведёт, видно
	}
	return ex
}

// fieldOfErr — поле окна и текст под ним по ошибке printprof.Validate («export.prefix: …»).
func fieldOfErr(err error) (field, text string) {
	head, tail, ok := strings.Cut(err.Error(), ":")
	if !ok {
		return "form", upperFirst(err.Error())
	}
	field = "form"
	switch strings.TrimSpace(head) {
	case "name", "gen":
		field = "name"
	case "page.dpi":
		field = "dpi"
	case "export.prefix":
		field = "prefix"
	case "export.quality":
		field = "quality"
	case "export.url":
		field = "url"
	}
	return field, upperFirst(strings.TrimSpace(tail))
}

func upperFirst(s string) string {
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[n:]
}

// clipText — чужая строка в тексте ошибки: не длиннее 40 знаков.
func clipText(s string) string {
	if utf8.RuneCountInString(s) <= 40 {
		return s
	}
	return string([]rune(s)[:40]) + "…"
}

// firstErr — одна строка из ошибок полей для ответа Save: название — первым, дальше по алфавиту.
func firstErr(errs map[string]string) string {
	if v, ok := errs["name"]; ok {
		return v
	}
	keys := slices.Sorted(maps.Keys(errs))
	if len(keys) == 0 {
		return ""
	}
	return errs[keys[0]]
}

// SavePrintProfile записывает новую свою типографию (всегда новый id), пересобирает реестр и
// шлёт `print:changed`. apply — ещё и поставить её альбому одним шагом отмены (applyPrint).
func (a *App) SavePrintProfile(e PrintEdit, apply bool) (res PrintSaved, err error) {
	defer guardErr("SavePrintProfile", &err)
	s, err := a.decorStore()
	if err != nil {
		return res, err
	}
	a.prints.mu.Lock()
	p, _ := a.editor().SnapshotSide()
	id := printprof.NewOwnID(a.printTaken)
	b := buildOwn(e, id, printprof.Current(), len(p.Spreads))
	if len(b.errs) > 0 {
		a.prints.mu.Unlock()
		return res, fmt.Errorf("%s", firstErr(b.errs))
	}
	err = a.storeOwn(s, "", b.p, nil)
	a.prints.mu.Unlock()
	if err != nil {
		return res, err
	}
	res.ID = id
	if apply {
		if _, err = a.applyPrint(id); err != nil {
			a.emit("print:changed") // запись есть: список в окне обновится и при отказе применения
			return res, err
		}
	}
	res.View = a.printsChanged()
	return res, nil
}

// storeOwn пишет профиль p в хранилище (key "" — новый файл) и пересобирает реестр. Если реестр
// профиль не принял (спор имён, лимит), запись откатывается: новый файл удаляется, замена
// возвращает прежнее содержимое prev. Под a.prints.mu.
func (a *App) storeOwn(s *decor.Store, key string, p printprof.Profile, prev *printprof.Profile) error {
	data, err := printprof.Encode(p)
	if err != nil {
		return err
	}
	k, err := s.SavePrint(key, p.Name, data)
	if err != nil {
		return err
	}
	a.reloadPrintsLocked()
	if _, ok := a.prints.own[p.ID]; !ok {
		if key == "" {
			_ = s.DeletePrint(k)
		} else if prev != nil {
			if old, err := printprof.Encode(*prev); err == nil {
				_, _ = s.SavePrint(key, prev.Name, old)
			}
		}
		a.reloadPrintsLocked()
		return errPrintRejected
	}
	return nil
}
