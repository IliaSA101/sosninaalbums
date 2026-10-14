package model

import (
	"cmp"
	"math"
	"slices"

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
	CoverBoth  = "both" // лицевая и задняя
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
// заворот и расстав Нетпринта Премиум/Royal/мягкой — гипотеза). Функция, а не переменная:
// снаружи таблицу не подменить. id после выпуска не меняются.
func CoverPresets() []CoverPreset {
	fs := func(id, name string, st []SpineStep) CoverPreset {
		return CoverPreset{ID: id, Name: name, Group: "Фотосфера", Spec: CoverSpec{Preset: id, Wrap: 23.5, Steps: st}}
	}
	np := func(id, name string, wrap, hinge, a, b float64) CoverPreset {
		return CoverPreset{ID: id, Name: name, Group: "Нетпринт", Spec: CoverSpec{Preset: id, Wrap: wrap, Hinge: hinge, A: a, B: b}}
	}
	return []CoverPreset{
		fs("fsfera-thin", "Тонкие листы", steps(5, 6, 9, 8, 13, 10, 17, 12, 22, 14, 26, 16, 30, 18, 34, 20, 38, 22, 42, 24, 47, 26, 52, 28, 57, 30)),
		fs("fsfera-card", "Картон", steps(3, 6, 5, 8, 7, 10, 9, 12, 12, 14, 15, 16, 17, 18, 20, 20, 22, 22, 24, 24, 26, 26, 28, 28, 30, 30)),
		fs("fsfera-double", "Двойной картон", steps(1, 6, 2, 8, 4, 10, 6, 12, 7, 14, 9, 16, 10, 18, 12, 20, 13, 22, 15, 24, 16, 26, 18, 28, 20, 30)),
		np("netprint-premium", "Премиум", 15, 9, 5.4, 0.39),
		np("netprint-royal", "Royal", 15, 9, 5.4, 0.8),
		np("netprint-soft", "Мягкая", 10, 0, 0.39, 0.39),
		{ID: CoverCustom, Name: "Свои размеры", Spec: CoverSpec{Preset: CoverCustom, Wrap: 15, A: 10}},
	}
}

// CoverPresetOf — пресет по id; ok=false — неизвестный.
func CoverPresetOf(id string) (CoverPreset, bool) {
	all := CoverPresets()
	if i := slices.IndexFunc(all, func(c CoverPreset) bool { return c.ID == id }); i >= 0 {
		return all[i], true
	}
	return CoverPreset{}, false
}

// Пределы чисел обложки, мм (design 032 §4).
const (
	coverMinSide, coverMaxSide = 50.0, 1000.0
	coverMaxWrap               = 50.0
	coverMaxHinge              = 20.0
	coverMaxA                  = 100.0
	coverMaxB                  = 5.0
	coverMaxMm                 = 100.0
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
// (страж — TestCoverSheetFields): «Авто», вариант, название и виньетка (роли мест снимает
// FixVignettes). «Авто» на обложке не бывает, поэтому в Photos — только фото её мест: фото
// без места с обложки уходит (и не отнимает себя у разворота в dedupe).
func FixCoverSheet(s *Spread) {
	s.ID, s.Layout, s.Variant, s.Name = CoverID, "", 0, ""
	s.Kind, s.Vig = "", nil
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
	// Zones — направляющие (не выделяются, в экспорт не идут); Drop — пустые зоны броска
	// фото фона по раскладке: kind back | front | wrap.
	Zones []CoverZone `json:"zones"`
	Drop  []CoverZone `json:"drop"`
	// Safe — безопасная зона обложки (внутри каждой сторонки), TextMaxW — предел ширины
	// «по тексту» на обложке: у разворотов свои в View.
	Safe     []layout.Rect `json:"safe"`
	TextMaxW float64       `json:"textMaxW"`
	// Guide — имя файла разметки без папки ("" — нет); GuideOK — файл на месте;
	// GuideWarn — пропорции файла отличаются от листа больше 1 %.
	Guide     string `json:"guide"`
	GuideOK   bool   `json:"guideOK"`
	GuideWarn bool   `json:"guideWarn"`
	PxW       int    `json:"pxW"` // 00.jpg при dpi альбома
	PxH       int    `json:"pxH"`
}

// CoverZone — направляющая или зона броска, мм листа обложки.
type CoverZone struct {
	Kind  string      `json:"kind"` // back | front | spine | hinge | wrap | trim | safe
	Rect  layout.Rect `json:"rect"`
	Label string      `json:"label"` // «Задняя», «Корешок 14 мм»…; у trim и safe ""
}

// CoverLayoutName — раскладка для интерфейса: CoverFront — "front".
func CoverLayoutName(k string) string {
	if k == CoverFront {
		return "front"
	}
	return k
}
