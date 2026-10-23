package model

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Обложка (спека 032): особый лист вне Spreads. Содержимое — обычный *Spread с ID CoverID,
// поэтому места, тексты, фоны и слои правятся теми же операциями. Геометрия — sheet.go.

// CoverID — ID листа обложки: не 12 hex, с NewID не столкнётся. После выпуска не меняется.
const CoverID = "cover"

// Раскладки обложки: какие зоны принимают фото фона. После выпуска — только добавлять.
const (
	CoverFront = ""     // фото на лицевой
	CoverBoth  = "both" // задняя, корешок (061) и лицевая
	CoverWrap  = "wrap" // одно фото через корешок
)

// CoverCustom — id «Своих размеров»: числа Spec заданы пользователем.
const CoverCustom = "custom"

// CoverDefault — пресет новой обложки (decisions 26: Марина печатает в Фотосфере).
const CoverDefault = "fsfera-thin"

type Cover struct {
	Sheet   *Spread   `json:"sheet"`             // содержимое; ID = CoverID; Layout, Variant, Name пусты
	Spec    CoverSpec `json:"spec"`              // размеры и правило корешка
	Layout  string    `json:"layout,omitempty"`  // CoverFront | CoverBoth | CoverWrap
	SpineBg string    `json:"spineBg,omitempty"` // #rrggbb плашки корешка; "" — нет
	Guide   string    `json:"guide,omitempty"`   // файл-разметка; в памяти абсолютный
	// GuideAlt — запасной путь разметки из файла, когда её нет ни там, ни там (как PhotoRef.Alt):
	// пишется обратно в guideAbs, иначе автосохранение на чужом компьютере стёрло бы его.
	GuideAlt string `json:"guideAlt,omitempty"`
}

// CoverSpec — размеры обложки, мм. Числа — истина, Preset — подпись (decisions 51):
// поправка таблицы в новой версии не сдвинет чужую обложку.
type CoverSpec struct {
	Preset string      `json:"preset"`          // id пресета или CoverCustom
	W      float64     `json:"w,omitempty"`     // сторонка; 0 — страница альбома
	H      float64     `json:"h,omitempty"`     // 0 — страница альбома
	Wrap   float64     `json:"wrap"`            // заворот, 0 допустим
	Hinge  float64     `json:"hinge,omitempty"` // расстав с каждой стороны корешка
	Steps  []SpineStep `json:"steps,omitempty"` // таблица корешка; пусто — формула A + B·n
	A      float64     `json:"a,omitempty"`
	B      float64     `json:"b,omitempty"`
	// Safe — безопасная линия сторонки от линии загиба, SpineSafe — от края корешка (у расстава —
	// от его края), мм; 0 — Settings.Safe, как до 047. Направляющие, не геометрия (v8, решение 51).
	Safe      float64 `json:"safe,omitempty"`
	SpineSafe float64 `json:"spineSafe,omitempty"`
}

// SpineStep — строка таблицы: до Upto разворотов включительно корешок Mm.
type SpineStep struct {
	Upto int     `json:"upto"`
	Mm   float64 `json:"mm"`
}

// CoverPreset — пресет типографии для списка «Типография».
type CoverPreset struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	Group string    `json:"group"`
	Spec  CoverSpec `json:"spec"` // без W/H: сторонку задаёт пользователь
}

// steps — таблица корешка из пар (до n разворотов, мм).
func steps(pairs ...float64) []SpineStep {
	out := make([]SpineStep, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, SpineStep{Upto: int(pairs[i]), Mm: pairs[i+1]})
	}
	return out
}

// CoverPresets — пресеты (design 032 §1; таблицы Фотосферы сверены с fsfera.ru 2026-10-13,
// заворот 20 — по PSD и JSON калькулятора, 047 решение 48; заворот и расстав Нетпринта
// Премиум/Royal/мягкой — гипотеза). У Фотосферы сторонку по формату страницы даёт CoverPresetFor. Функция, а не переменная:
// снаружи таблицу не подменить. id после выпуска не меняются.
func CoverPresets() []CoverPreset {
	fs := func(id, name string, st []SpineStep) CoverPreset {
		return CoverPreset{ID: id, Name: name, Group: CoverGroupFsfera, Spec: CoverSpec{Preset: id, Wrap: fsferaWrap, Steps: st, Safe: fsferaCoverSafe, SpineSafe: fsferaSpineSafe}}
	}
	np := func(id, name string, wrap, hinge, a, b float64) CoverPreset {
		return CoverPreset{ID: id, Name: name, Group: CoverGroupNetprint, Spec: CoverSpec{Preset: id, Wrap: wrap, Hinge: hinge, A: a, B: b}}
	}
	out := []CoverPreset{
		fs("fsfera-thin", "Тонкие листы", steps(5, 6, 9, 8, 13, 10, 17, 12, 22, 14, 26, 16, 30, 18, 34, 20, 38, 22, 42, 24, 47, 26, 52, 28, 57, 30)),
		fs("fsfera-card", "Картон", steps(3, 6, 5, 8, 7, 10, 9, 12, 12, 14, 15, 16, 17, 18, 20, 20, 22, 22, 24, 24, 26, 26, 28, 28, 30, 30)),
		fs("fsfera-double", "Двойной картон", steps(1, 6, 2, 8, 4, 10, 6, 12, 7, 14, 9, 16, 10, 18, 12, 20, 13, 22, 15, 24, 16, 26, 18, 28, 20, 30)),
		np("netprint-premium", "Премиум", 15, 9, 5.4, 0.39),
		np("netprint-royal", "Royal", 15, 9, 5.4, 0.8),
		np("netprint-soft", "Мягкая", 10, 0, 0.39, 0.39),
	}
	out = append(out, fabrikaCoverPresets()...)
	return append(out, CoverPreset{ID: CoverCustom, Name: "Свои размеры", Spec: CoverSpec{Preset: CoverCustom, Wrap: 15, A: 10}})
}

// CoverPresetOf — пресет по id; ok=false — неизвестный. Для новой обложки и правок — CoverPresetFor.
func CoverPresetOf(id string) (CoverPreset, bool) {
	all := CoverPresets()
	if i := slices.IndexFunc(all, func(c CoverPreset) bool { return c.ID == id }); i >= 0 {
		return all[i], true
	}
	return CoverPreset{}, false
}

// Фотосфера Pro (047): таблица форматов фотокниги, шаблон обложки по формату и пресеты страницы.

// CoverGroupFsfera — группа пресетов обложки и ряд пресетов страницы Фотосферы.
const CoverGroupFsfera = "Фотосфера"

// CoverGroupNetprint — группа пресетов обложки и ряд пресетов страницы Нетпринта (058).
const CoverGroupNetprint = "Нетпринт"

// Шаблон обложки Фотосферы (PSD): заворот вне таблицы форматов (решение 48) и безопасные
// линии — от линии загиба и от края корешка.
const (
	fsferaWrap                       = 20.0
	fsferaCoverSafe, fsferaSpineSafe = 8.0, 12.0
	// fsferaBleed — обрез разворота: в PSD 20×30, 25×25, 30×30 — 2 мм (066 research §2.1);
	// у прочих форматов — по исследованию 047 («2 мм у всех»).
	fsferaBleed = 2.0
)

// PrintFormat — формат изделия типографии: страница альбома и шаблон обложки, мм.
type PrintFormat struct {
	ID           string  // "fsfera-30x30"; после выпуска не меняется
	Name         string  // "30×30"
	PageW, PageH float64 // страница с вылетом (половина файла разворота)
	Safe         float64 // безопасная зона разворота от края файла
	Bleed        float64 // обрез разворота от края файла (066: розовое поле PSD)
	SideW, SideH float64 // сторонка обложки
	Wrap         float64 // заворот
}

// templateGroup — типография с таблицей форматов: сторонка и заворот обложки её пресетов — числа
// шаблона формата страницы (047, 081). Gen — родительный падеж для «Не по шаблону …».
type templateGroup struct {
	Group, Gen string
	Formats    func() []PrintFormat
}

// templateGroups — шаблонные группы; остальные пресеты сторонку задают сами.
func templateGroups() []templateGroup {
	return []templateGroup{
		{CoverGroupFsfera, "Фотосферы", FsferaFormats},
		{CoverGroupFabrika, "Фабрики Фотокниги", FabrikaFormats},
	}
}

// templateGroupOf — шаблонная группа по имени; ok=false — у группы шаблона по формату нет.
func templateGroupOf(group string) (templateGroup, bool) {
	all := templateGroups()
	if i := slices.IndexFunc(all, func(g templateGroup) bool { return g.Group == group }); i >= 0 && group != "" {
		return all[i], true
	}
	return templateGroup{}, false
}

// FsferaFormats — форматы фотокниги Фотосферы в порядке калькулятора (research §2; Safe — по PSD,
// решение 50; 30×20 — по PSD, решение 49). Функция, а не переменная, как CoverPresets.
func FsferaFormats() []PrintFormat {
	f := func(name string, pw, ph, safe, sw, sh, wrap float64) PrintFormat {
		return PrintFormat{ID: "fsfera-" + strings.ReplaceAll(name, "×", "x"), Name: name,
			PageW: pw, PageH: ph, Safe: safe, Bleed: fsferaBleed, SideW: sw, SideH: sh, Wrap: wrap}
	}
	return []PrintFormat{
		f("20×15", 201, 152, 10, 206, 156, 20),
		f("20×20", 201, 203, 10, 206, 206, 20),
		f("20×30", 201, 273, 10, 206, 276, 20),
		f("30×20", 286, 203, 10, 289, 206, 20),
		f("25×25", 251, 254, 8, 256, 256, 20),
		f("30×30", 281, 284, 10, 286, 286, 20),
		f("21×30", 208, 296, 8, 211, 296, 16),
	}
}

// PagePreset — кнопка размера страницы в «Настройках альбома». 0/false — поле пресет не задаёт.
type PagePreset struct {
	Label   string  `json:"label"` // "30×30" в ряду «Фотосфера», "30×30 (квадрат)" в общем
	Group   string  `json:"group"` // "" — общий ряд; CoverGroupFsfera, CoverGroupNetprint, CoverGroupFabrika
	W       float64 `json:"w"`
	H       float64 `json:"h"`
	DPI     int     `json:"dpi,omitempty"`
	Layflat bool    `json:"layflat,omitempty"` // true — ставит layflat; пресетов с false нет
	Safe    float64 `json:"safe,omitempty"`
	Bleed   float64 `json:"bleed,omitempty"` // обрез (066); 0 — пресет его не задаёт
	Hint    string  `json:"hint,omitempty"`
}

// PagePresets — общий ряд (прежние кнопки presets.ts без «Фотосфера 30×30»), ряд Фотосферы
// из FsferaFormats (решение 54: один источник — Go), ряды Нетпринта и Фабрики Фотокниги (081);
// порядок групп — порядок списка «Типография» в окне (058).
func PagePresets() []PagePreset {
	out := []PagePreset{
		{Label: "30×30 (квадрат)", W: 300, H: 300},
		{Label: "25×25", W: 250, H: 250},
		{Label: "20×20", W: 200, H: 200},
		{Label: "30×20 (альбомный)", W: 300, H: 200},
		{Label: "20×30 (книжный)", W: 200, H: 300},
		{Label: "40×30", W: 400, H: 300},
	}
	for _, f := range FsferaFormats() {
		out = append(out, PagePreset{Label: f.Name, Group: CoverGroupFsfera, W: f.PageW, H: f.PageH, DPI: 300, Layflat: true, Safe: f.Safe, Bleed: f.Bleed,
			Hint: "Фотосфера Pro: JPEG sRGB 300 dpi, 00.jpg — обложка, 01.jpg… — развороты; важное — не ближе " + MmText(f.Safe) + " мм к краю"})
	}
	out = append(out, netprintPresets()...)
	return append(out, fabrikaPagePresets()...)
}

// netprintPresets — Нетпринт «Премиум» твёрдой обложки (058 №16, netprint.ru «Технические размеры
// фотокниг Премиум», 17.10): страница — половина дообрезного разворота; важное — 10 мм от края
// (3 мм обреза + 7 мм зоны возможной обрезки; гипотеза — сверить при QA). Обрез 3 мм — тоже
// гипотеза (066 decisions №5): сверить по шаблону Нетпринта, когда будет.
func netprintPresets() []PagePreset {
	const safe, bleed = 10.0, 3.0
	rows := []struct {
		label string
		w, h  float64
	}{
		{"15×20", 153, 203}, {"20×15", 200, 156}, {"18×13", 182.5, 127}, {"20×20", 200, 203},
		{"20×28", 200, 270}, {"28×20", 285, 203}, {"25×25", 250, 254}, {"30×30", 300, 305},
	}
	out := make([]PagePreset, len(rows))
	for i, r := range rows {
		out[i] = PagePreset{Label: r.label, Group: CoverGroupNetprint, W: r.w, H: r.h, DPI: 300, Layflat: true, Safe: safe, Bleed: bleed,
			Hint: "Нетпринт Премиум: JPEG 300 dpi, разворотами; важное — не ближе " + MmText(safe) + " мм к краю"}
	}
	return out
}

// CoverPresetFor — пресет id для страницы s. У шаблонной группы («Фотосфера», «Фабрика
// Фотокниги») сторонка и заворот — всегда числа шаблона: формат страницы из таблицы, на прочей
// странице — ближайший формат (FormatNear, 070 Е, decisions 0.16 №28); W/H 0 («из страницы»)
// пресет не даёт. Остальные пресеты — как CoverPresetOf.
func CoverPresetFor(id string, s Settings) (CoverPreset, bool) {
	pr, ok := CoverPresetOf(id)
	if !ok {
		return pr, false
	}
	if _, tpl := templateGroupOf(pr.Group); !tpl {
		return pr, true
	}
	f := FormatNear(pr.Group, s)
	pr.Spec.W, pr.Spec.H, pr.Spec.Wrap = f.SideW, f.SideH, f.Wrap
	return pr, true
}

// CoverOffTemplate — обложка шаблонной группы не совпадает с шаблоном для текущей страницы:
// have/want — размер листа обложки, мм (при текущем числе разворотов). Сравниваются действующая
// сторонка (W/H 0 — страница), Wrap, Hinge, Steps; A, B и безопасные линии — нет. format — формат
// шаблона: страница из таблицы или ближайший к ней (070 Е).
// Пресет не шаблонной группы или обложки нет — off=false.
func (p *Project) CoverOffTemplate() (have, want [2]float64, format PrintFormat, off bool) {
	if p.Cover == nil {
		return have, want, format, false
	}
	c := p.Cover.Spec
	pr, ok := CoverPresetFor(c.Preset, p.Settings)
	if _, tpl := templateGroupOf(pr.Group); !ok || !tpl {
		return have, want, format, false
	}
	format = FormatNear(pr.Group, p.Settings)
	d := pr.Spec
	// Шаблон считает та же CoverGeo: числа строки и «Итог» карточки сходятся до 0,01.
	cg := p.CoverGeo()
	tpl := (&Project{Settings: p.Settings, Spreads: p.Spreads, Cover: &Cover{Spec: d}}).CoverGeo()
	have, want = [2]float64{cg.X[7], cg.Y[3]}, [2]float64{tpl.X[7], tpl.Y[3]}
	sideOK := cg.X[2]-cg.X[1] == tpl.X[2]-tpl.X[1] && cg.Y[2]-cg.Y[1] == tpl.Y[2]-tpl.Y[1]
	off = !sideOK || c.Wrap != d.Wrap || c.Hinge != d.Hinge || !slices.Equal(c.Steps, d.Steps)
	return have, want, format, off
}

// CoverFixText — строка «Не по шаблону Фотосферы 30×30: 623 × 331 мм, нужно 626 × 326» для
// карточки обложки и проверки выгрузки; "" — по шаблону или группа не шаблонная. Размер листа совпал
// (другая таблица корешка) — называется заворот, иначе корешок и расстав.
func CoverFixText(p *Project) string {
	have, want, f, off := p.CoverOffTemplate()
	if !off {
		return ""
	}
	pr, _ := CoverPresetFor(p.Cover.Spec.Preset, p.Settings)
	tg, _ := templateGroupOf(pr.Group)
	head := "Не по шаблону " + tg.Gen + " " + f.Name // формат есть всегда: точный или ближайший
	if have != want {
		return fmt.Sprintf("%s: %s × %s мм, нужно %s × %s", head, MmText(have[0]), MmText(have[1]), MmText(want[0]), MmText(want[1]))
	}
	c := p.Cover.Spec
	if c.Wrap != pr.Spec.Wrap {
		return fmt.Sprintf("%s: заворот %s мм, нужно %s", head, MmText(c.Wrap), MmText(pr.Spec.Wrap))
	}
	return head + ": корешок или расстав не по таблице"
}

// MmText — миллиметры для подписи: «14», «13,2», «23,5» (десятые, запятая).
func MmText(v float64) string {
	return strings.Replace(strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64), ".", ",", 1)
}

// Пределы чисел обложки, мм (design 032 §4).
const (
	coverMinSide, coverMaxSide = 50.0, 1000.0
	coverMaxWrap               = 50.0
	coverMaxHinge              = 20.0
	coverMaxA                  = 100.0
	coverMaxB                  = 5.0
	coverMaxMm                 = 100.0
	coverMaxSafe               = 30.0
	// MaxSpineSteps — строк в таблице корешка: больше Load отбрасывает.
	MaxSpineSteps = 64
)

// ClampCoverSpec — числа из файла или интерфейса: NaN и Inf — значения «Своих размеров»,
// остальное зажимается в пределы (сторонка — 0 или 50…1000), шаг 0,01 мм (B — 0,001).
// Таблица: не больше MaxSpineSteps строк, по возрастанию Upto без повторов. Неизвестный
// пресет — CoverCustom. Идемпотентна.
func ClampCoverSpec(c CoverSpec) CoverSpec {
	def, _ := CoverPresetOf(CoverCustom)
	d := def.Spec
	fix := func(v, def, hi float64) float64 { return geom.Round2(geom.ClampF(Finite(v, def), 0, hi)) }
	side := func(v float64) float64 {
		if v = Finite(v, 0); v <= 0 {
			return 0
		}
		return geom.Round2(geom.ClampF(v, coverMinSide, coverMaxSide))
	}
	out := CoverSpec{
		Preset: c.Preset,
		W:      side(c.W),
		H:      side(c.H),
		Wrap:   fix(c.Wrap, d.Wrap, coverMaxWrap),
		Hinge:  fix(c.Hinge, d.Hinge, coverMaxHinge),
		A:      fix(c.A, d.A, coverMaxA),
		B:      math.Round(geom.ClampF(Finite(c.B, d.B), 0, coverMaxB)*1000) / 1000,
		// Линии безопасности: NaN/Inf — 0 (как до 047, Settings.Safe).
		Safe:      fix(c.Safe, 0, coverMaxSafe),
		SpineSafe: fix(c.SpineSafe, 0, coverMaxSafe),
	}
	if _, ok := CoverPresetOf(out.Preset); !ok {
		out.Preset = CoverCustom
	}
	st := slices.Clone(c.Steps[:min(len(c.Steps), MaxSpineSteps)])
	for i := range st {
		st[i].Upto = max(1, min(MaxSpreads, st[i].Upto))
		st[i].Mm = fix(st[i].Mm, d.A, coverMaxMm)
	}
	slices.SortStableFunc(st, func(a, b SpineStep) int { return cmp.Compare(a.Upto, b.Upto) })
	st = slices.CompactFunc(st, func(a, b SpineStep) bool { return a.Upto == b.Upto })
	if len(st) > 0 {
		out.Steps = st
	}
	return out
}

// Spine — корешок при n разворотах, мм (шаг 0,01); over — n больше таблицы (берётся
// последняя строка).
func (c CoverSpec) Spine(n int) (mm float64, over bool) {
	if len(c.Steps) == 0 {
		return geom.Round2(c.A + c.B*float64(max(n, 0))), false
	}
	for _, s := range c.Steps {
		if n <= s.Upto {
			return s.Mm, false
		}
	}
	return c.Steps[len(c.Steps)-1].Mm, true
}

// ValidCoverLayout — известная раскладка обложки.
func ValidCoverLayout(k string) bool { return k == CoverFront || k == CoverBoth || k == CoverWrap }

// IsCover — s — лист обложки этого проекта (по ID: dedupe не даёт развороту ID CoverID).
func (p *Project) IsCover(s *Spread) bool {
	return s != nil && s.ID == CoverID && p.Cover != nil
}

// FixCoverSheet — лист обложки из файла: поля Spread, которых у обложки нет, обнуляются
// (страж — TestCoverSheetFields): «Авто», вариант, название, виньетка (роли мест снимает
// FixVignettes) и скрытие фона альбома (его на обложке нет, 098-10). «Авто» на обложке не бывает, поэтому в Photos — только фото её мест: фото
// без места с обложки уходит (и не отнимает себя у разворота в dedupe).
func FixCoverSheet(s *Spread) {
	s.ID, s.Layout, s.Variant, s.Name = CoverID, "", 0, ""
	s.Kind, s.Vig, s.NoAlbumBg = "", nil, false
	s.Grid = slices.DeleteFunc(s.Grid, func(pl *Place) bool { return pl == nil })
	seen := make(map[string]bool, len(s.Grid))
	s.Photos = nil
	for _, pl := range s.Grid {
		if pl.Photo != "" && !seen[pl.Photo] {
			seen[pl.Photo] = true
			s.Photos = append(s.Photos, pl.Photo)
		}
	}
	if len(s.Grid) == 0 {
		s.Grid = nil
	}
}

// CoverView — обложка для интерфейса (032 §6): лист, числа и направляющие. UI геометрию
// не считает: зоны и размеры — мм листа, проценты — от Sheet.W/H.
type CoverView struct {
	Sheet   SpreadView `json:"sheet"` // ID CoverID; кадры, фоны, LowDPI — как у разворота
	Spec    CoverSpec  `json:"spec"`
	Preset  string     `json:"preset"` // действующий id; CoverCustom — числа свои
	Spine   float64    `json:"spine"`  // корешок при текущем числе разворотов, мм
	N       int        `json:"n"`      // разворотов
	Over    bool       `json:"over"`   // разворотов больше, чем в таблице пресета
	Layout  string     `json:"layout"` // front | both | wrap
	SpineBg string     `json:"spineBg"`
	// SpineText — ID надписи на корешке (Project.SpineTextID), "" — нет: кнопка «Надпись» /
	// «Выделить надпись» (058 №19).
	SpineText string `json:"spineText"`
	// Zones — направляющие (не выделяются, в экспорт не идут); Drop — пустые зоны броска
	// фото фона по раскладке: kind back | front | wrap | spine.
	Zones []CoverZone `json:"zones"`
	Drop  []CoverZone `json:"drop"`
	// Scopes — области фото фона текущей раскладки с подписями по порядку карточки (061):
	// UI берёт их отсюда, а не решает по раскладке сам.
	Scopes []CoverScope `json:"scopes"`
	// Safe — безопасная зона обложки (внутри каждой сторонки), TextMaxW — предел ширины
	// «по тексту» на обложке: у разворотов свои в View.
	Safe     []layout.Rect `json:"safe"`
	TextMaxW float64       `json:"textMaxW"`
	// Markup — разметка типографии обложки (066, markup.go); nil — файла нет.
	Markup *GuideView `json:"markup"`
	PxW    int        `json:"pxW"` // 00.jpg при dpi альбома
	PxH    int        `json:"pxH"`
	// Fix — «Не по шаблону Фотосферы 30×30: …» (CoverFixText, 047, 081); "" — по шаблону или не шаблонная группа.
	Fix string `json:"fix"`
}

// CoverZone — направляющая или зона броска, мм листа обложки.
type CoverZone struct {
	Kind  string      `json:"kind"` // back | front | spine | hinge | wrap | trim | safe
	Rect  layout.Rect `json:"rect"`
	Label string      `json:"label"` // «Задняя», «Корешок 14 мм»…; у trim и safe ""
}

// CoverScope — область фото фона обложки и её подпись («Задняя», «Корешок»…).
type CoverScope struct {
	Scope string `json:"scope"` // spread | left | right | spine
	Label string `json:"label"`
}

// CoverLayoutName — раскладка для интерфейса: CoverFront — "front".
func CoverLayoutName(k string) string {
	if k == CoverFront {
		return "front"
	}
	return k
}
