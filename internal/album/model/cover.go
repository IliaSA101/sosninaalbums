package model

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
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

// CoverPresets — пресеты типографий из профилей (design 032 §1; 123: таблицы лежат в
// printprof/builtin/*.json — Фотосфера сверена с fsfera.ru 2026-10-13, заворот 20 по PSD и JSON
// калькулятора, 047 решение 48; заворот и расстав Нетпринта Премиум/Royal/мягкой — гипотеза) и
// «Свои размеры» последним. У шаблонных групп сторонку по формату страницы даёт CoverPresetFor.
// Функция, а не переменная: снаружи таблицу не подменить (отдаются копии). id после выпуска не меняются.
func CoverPresets() []CoverPreset {
	src := tables().covers
	out := make([]CoverPreset, 0, len(src)+1)
	for _, c := range src {
		out = append(out, cloneCover(c))
	}
	return append(out, customPreset())
}

// CoverPresetOf — пресет по id; ok=false — неизвестный. Для новой обложки и правок — CoverPresetFor.
func CoverPresetOf(id string) (CoverPreset, bool) { return tables().coverPresetOf(id) }

// coverPresetOf — CoverPresetOf по таблицам t.
func (t *printTables) coverPresetOf(id string) (CoverPreset, bool) {
	if id == CoverCustom {
		return customPreset(), true
	}
	if i := slices.IndexFunc(t.covers, func(c CoverPreset) bool { return c.ID == id }); i >= 0 {
		return cloneCover(t.covers[i]), true
	}
	return CoverPreset{}, false
}

// Фотосфера Pro (047): таблица форматов фотокниги, шаблон обложки по формату и пресеты страницы.

// Имена групп пресетов обложки и рядов страницы — Name встроенных профилей (страж
// TestGroupNamesMatchProfiles). Строки нужны коду, который называет типографию без реестра.
const (
	CoverGroupFsfera   = "Фотосфера"
	CoverGroupNetprint = "Нетпринт"
	// CoverGroupFabrika — Фабрика Фотокниги (081).
	CoverGroupFabrika = "Фабрика Фотокниги"
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
	// Px — цель в пикселях файла разворота (PrintDPI) из профиля; {0,0} — нет, формат задан мм.
	// Есть — совпадение формата и подгонка холста идут по ней (FormatOf, FitPx, 123 §5). Вне
	// JSON: эталон таблиц встроенных (TestBuiltinTablesSnapshot) от неё не меняется, у встроенных её нет.
	Px [2]int `json:"-"`
	// Guide — направляющие экрана по шаблону (123 Б5, PSD Фабрики); нулевая — прежние линии. Вне JSON,
	// как Px: числа закреплены отдельной секцией эталона (TestBuiltinTablesSnapshot), а таблицы для
	// интерфейса не меняются.
	Guide PrintGuide `json:"-"`
}

// PrintGuide — направляющие формата по осям, мм. Нули — «нет»: BleedX/BleedY — полоса обреза разворота
// у бока и сверху-снизу; FoldX/FoldY — линия загиба обложки от края листа; FoldSafe — безопасная линия
// от загиба внутрь, SpineSafe — от края корешка (0 — Settings.Safe, как у обложки без направляющих).
// Лист, файл и .salbum от них не зависят.
type PrintGuide struct {
	BleedX, BleedY      float64
	FoldX, FoldY        float64
	FoldSafe, SpineSafe float64
}

// HasFold — у формата есть линия загиба.
func (g PrintGuide) HasFold() bool { return g.FoldX > 0 && g.FoldY > 0 }

// HasBleed — у формата есть полоса обреза по осям.
func (g PrintGuide) HasBleed() bool { return g.BleedX > 0 || g.BleedY > 0 }

// templateGroup — типография с таблицей форматов: сторонка и заворот обложки её пресетов — числа
// шаблона формата страницы (047, 081). Gen — родительный падеж для «Не по шаблону …».
type templateGroup struct {
	Group, Gen string
	Formats    func() []PrintFormat // копия таблицы
	list       []PrintFormat        // сама таблица, только чтение: FormatOf не копирует её на каждый вызов
}

// templateGroupOf — шаблонная группа по имени; ok=false — у группы шаблона по формату нет.
func templateGroupOf(group string) (templateGroup, bool) { return tables().templateGroupOf(group) }

// templateGroupOf — templateGroupOf по таблицам t.
func (t *printTables) templateGroupOf(group string) (templateGroup, bool) {
	if i := slices.IndexFunc(t.groups, func(g templateGroup) bool { return g.Group == group }); i >= 0 && group != "" {
		return t.groups[i], true
	}
	return templateGroup{}, false
}

// FsferaFormats — форматы фотокниги Фотосферы в порядке калькулятора (research §2; Safe — по PSD,
// решение 50; 30×20 — по PSD, решение 49). Функция, а не переменная, как CoverPresets.
func FsferaFormats() []PrintFormat { return GroupFormats(CoverGroupFsfera) }

// FabrikaFormats — форматы Фабрики в порядке сайта. Страница — половина разворота сайта
// (30×30 → 600×305), сторонка и заворот делят лист обложки «основа + корешок» (20×20 → 446×240
// при корешке 0).
func FabrikaFormats() []PrintFormat { return GroupFormats(CoverGroupFabrika) }

// GroupFormats — форматы шаблонной группы по её названию (копия); у группы без шаблона — nil.
func GroupFormats(group string) []PrintFormat {
	if tg, ok := templateGroupOf(group); ok {
		return tg.Formats()
	}
	return nil
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

// PagePresets — общий ряд (прежние кнопки presets.ts без «Фотосфера 30×30») и ряды типографий из
// профилей (решение 54: один источник — Go; Нетпринт: 058 №16, netprint.ru «Технические размеры
// фотокниг Премиум», 17.10, зона и обрез — гипотеза до шаблона, 066 decisions №5); порядок групп —
// порядок списка «Типография» в окне (058).
func PagePresets() []PagePreset {
	rows := tables().pages
	out := make([]PagePreset, 0, 6+len(rows))
	out = append(out,
		PagePreset{Label: "30×30 (квадрат)", W: 300, H: 300},
		PagePreset{Label: "25×25", W: 250, H: 250},
		PagePreset{Label: "20×20", W: 200, H: 200},
		PagePreset{Label: "30×20 (альбомный)", W: 300, H: 200},
		PagePreset{Label: "20×30 (книжный)", W: 200, H: 300},
		PagePreset{Label: "40×30", W: 400, H: 300},
	)
	return append(out, rows...)
}

// CoverPresetFor — пресет id для страницы s. У шаблонной группы («Фотосфера», «Фабрика
// Фотокниги») сторонка и заворот — всегда числа шаблона: формат страницы из таблицы, на прочей
// странице — ближайший формат (FormatNear, 070 Е, decisions 0.16 №28); W/H 0 («из страницы»)
// пресет не даёт. У Фабрики таблица корешка тоже зависит от формата (spineBy профиля). Остальные
// пресеты — как CoverPresetOf.
func CoverPresetFor(id string, s Settings) (CoverPreset, bool) {
	return tables().coverPresetFor(id, s)
}

// coverPresetFor — CoverPresetFor по таблицам t: пресет, группа, формат и корешок из одного снимка.
func (t *printTables) coverPresetFor(id string, s Settings) (CoverPreset, bool) {
	pr, ok := t.coverPresetOf(id)
	if !ok {
		return pr, false
	}
	tg, tpl := t.templateGroupOf(pr.Group)
	if !tpl {
		return pr, true
	}
	f := tg.near(s)
	pr.Spec.W, pr.Spec.H, pr.Spec.Wrap = f.SideW, f.SideH, f.Wrap
	pr.Spec.Steps = t.stepsFor(pr.ID, f.Name, pr.Spec.Steps)
	return pr, true
}

// CoverOffTemplate — обложка шаблонной группы не совпадает с шаблоном для текущей страницы:
// have/want — размер листа обложки, мм (при текущем числе разворотов). Сравниваются действующая
// сторонка (W/H 0 — страница), Wrap, Hinge, Steps; A, B и безопасные линии — нет. format — формат
// шаблона: страница из таблицы или ближайший к ней (070 Е).
// Пресет не шаблонной группы или обложки нет — off=false.
func (p *Project) CoverOffTemplate() (have, want [2]float64, format PrintFormat, off bool) {
	return p.coverOffTemplate(tables())
}

// coverOffTemplate — CoverOffTemplate по таблицам t.
func (p *Project) coverOffTemplate(t *printTables) (have, want [2]float64, format PrintFormat, off bool) {
	if p.Cover == nil {
		return have, want, format, false
	}
	c := p.Cover.Spec
	pr, ok := t.coverPresetFor(c.Preset, p.Settings)
	tg, isTpl := t.templateGroupOf(pr.Group)
	if !ok || !isTpl {
		return have, want, format, false
	}
	format = tg.near(p.Settings)
	d := pr.Spec
	// Шаблон считает та же CoverGeo: числа строки и «Итог» карточки сходятся до 0,01.
	cg := p.CoverGeo()
	tpl := (&Project{Settings: p.Settings, Spreads: p.Spreads, Cover: &Cover{Spec: d}}).CoverGeo()
	have, want = [2]float64{cg.X[7], cg.Y[3]}, [2]float64{tpl.X[7], tpl.Y[3]}
	// Сторонка в допуске PageTol: обложка из px (SetCoverPx) отстоит от шаблона на ≤ 1 px.
	sideOK := SameMm(cg.X[2]-cg.X[1], tpl.X[2]-tpl.X[1]) && SameMm(cg.Y[2]-cg.Y[1], tpl.Y[2]-tpl.Y[1])
	off = !sideOK || c.Wrap != d.Wrap || c.Hinge != d.Hinge || !slices.Equal(c.Steps, d.Steps)
	return have, want, format, off
}

// CoverGuide — направляющие обложки по шаблону типографии (123 Б5): обложка по шаблону (CoverOffTemplate
// = false), у формата шаблона есть линия загиба, а пользователь не задавал свои безопасные линии
// (CoverSpec.Safe и SpineSafe = 0). Иначе false — прежний расчёт от Settings.Safe. Производные данные:
// лист, файл и .salbum от них не зависят.
func (p *Project) CoverGuide() (PrintGuide, bool) {
	if p.Cover == nil || p.Cover.Spec.Safe != 0 || p.Cover.Spec.SpineSafe != 0 {
		return PrintGuide{}, false
	}
	_, _, f, off := p.coverOffTemplate(tables())
	if off || !f.Guide.HasFold() {
		return PrintGuide{}, false
	}
	return f.Guide, true
}

// CoverFixText — строка «Не по шаблону Фотосферы 30×30: 623 × 331 мм, нужно 626 × 326» для
// карточки обложки и проверки выгрузки; "" — по шаблону или группа не шаблонная. Размер листа совпал
// (другая таблица корешка) — называется заворот, иначе корешок и расстав.
func CoverFixText(p *Project) string {
	t := tables()
	have, want, f, off := p.coverOffTemplate(t)
	if !off {
		return ""
	}
	pr, _ := t.coverPresetFor(p.Cover.Spec.Preset, p.Settings)
	tg, _ := t.templateGroupOf(pr.Group)
	head := "Не по шаблону " + tg.Gen + " " + f.Name // формат есть всегда: точный или ближайший
	if have != want {
		return fmt.Sprintf("%s: %s × %s мм, нужно %s × %s%s", head, MmText(have[0]), MmText(have[1]), MmText(want[0]), MmText(want[1]), coverFixPx(have, want))
	}
	c := p.Cover.Spec
	if c.Wrap != pr.Spec.Wrap {
		return fmt.Sprintf("%s: заворот %s мм, нужно %s", head, MmText(c.Wrap), MmText(pr.Spec.Wrap))
	}
	return head + ": корешок или расстав не по таблице"
}

// CoverFixPxMark — начало хвоста с px в CoverFixText; вопрос обычного экспорта его отрезает.
const CoverFixPxMark = " (файл "

// coverFixPx — хвост « (файл 7583 × 2906 px, нужен 8031 × 2835)» (123 §1.4): размер файла обложки
// для типографии при PrintDPI, чтобы сверить с отказом загрузчика. Пусто, когда px сошлись: сторонка
// на ≤ 0,01 мм не меняет файл, а строка «файл X, нужен X» только путала бы.
func coverFixPx(have, want [2]float64) string {
	hw, hh, ww, wh := Px(have[0], PrintDPI), Px(have[1], PrintDPI), Px(want[0], PrintDPI), Px(want[1], PrintDPI)
	if hw == ww && hh == wh {
		return ""
	}
	return fmt.Sprintf("%s%d × %d px, нужен %d × %d)", CoverFixPxMark, hw, hh, ww, wh)
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
	// Id типографии, которой здесь нет (своя на чужом компьютере, 123 §4), сохраняется: иначе
	// автосохранение стёрло бы пометку изделия. Обложка при этом ведёт себя как «Свои размеры»
	// (CoverMissing). Мусорный id — «Свои размеры».
	if _, ok := CoverPresetOf(out.Preset); !ok && !printprof.ValidID(out.Preset) {
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

// CoverMissing — «Типография не найдена: <id>» для обложки с id изделия, которого нет в реестре
// (своя типография с другого компьютера); "" — пресет известен или это «Свои размеры».
func CoverMissing(preset string) string {
	if _, ok := CoverPresetOf(preset); ok || preset == "" {
		return ""
	}
	return "Типография не найдена: " + preset
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
	// Sides — сторонки с безопасной зоной (CoverSides, 127 G1): Safe строится из их Box, раскладки
	// сторонки (127) рисуют по ним полосу ‹ ▦ ›. Всегда список.
	Sides []CoverSide `json:"sides"`
	// Markup — разметка типографии обложки (066, markup.go); nil — файла нет.
	Markup *GuideView `json:"markup"`
	PxW    int        `json:"pxW"` // 00.jpg при dpi альбома
	PxH    int        `json:"pxH"`
	// Fix — «Не по шаблону Фотосферы 30×30: …» (CoverFixText, 047, 081); "" — по шаблону или не шаблонная группа.
	Fix string `json:"fix"`
	// Missing — «Типография не найдена: own-0a1b2c3d»: id изделия обложки без профиля в реестре;
	// числа Spec остаются, обложка ведёт себя как «Свои размеры». "" — всё известно.
	Missing string `json:"missing"`
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
