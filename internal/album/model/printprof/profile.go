// Package printprof — профили типографий данными (123 (В), design §2–§3): форматы страницы,
// изделия обложки, корешки, пределы и тексты выгрузки лежат в JSON, а не в коде. Встроенные —
// builtin/*.json внутри программы; Parse разбирает профиль строго, реестр отдаёт снимок Current().
//
// Пакет ничего не знает о проекте: таблицы из профилей строит model, профиль выгрузки —
// printshop. Логика (совпадение формата, шаблон обложки, проверки) остаётся в Go.
package printprof

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
)

// FormatVersion — версия формата файла профиля. После выпуска 0.26 новые поля появляются только с
// format 2; до выпуска format 1 расширен полями направляющих (guide, foldSafe, guideSpineSafe — 0.26-123-50).
const FormatVersion = 1

// Pair — два числа JSON-массивом «[ширина, высота]»; другая длина — ошибка, а не молчаливое
// обрезание (encoding/json отбросил бы лишнее и дополнил бы нулём недостающее).
type Pair [2]float64

// UnmarshalJSON читает ровно два числа.
func (p *Pair) UnmarshalJSON(b []byte) error {
	var v []float64
	if err := json.Unmarshal(b, &v); err != nil {
		return userError("нужны два числа: " + reason(err))
	}
	if len(v) != 2 {
		return userError(fmt.Sprintf("нужны два числа, а не %d", len(v)))
	}
	*p = Pair{v[0], v[1]}
	return nil
}

// PxPair — размер файла в пикселях «[ширина, высота]».
type PxPair [2]int

// UnmarshalJSON читает ровно два целых.
func (p *PxPair) UnmarshalJSON(b []byte) error {
	var v []int
	if err := json.Unmarshal(b, &v); err != nil {
		return userError("нужны два целых числа: " + reason(err))
	}
	if len(v) != 2 {
		return userError(fmt.Sprintf("нужны два целых, а не %d", len(v)))
	}
	*p = PxPair{v[0], v[1]}
	return nil
}

// Profile — типография (format 1). Нулевые поля с omitempty означают «не задано».
type Profile struct {
	Format  int    `json:"format"`
	ID      string `json:"id"`      // после выпуска не меняется
	Version int    `json:"version"` // растёт с каждой правкой таблиц; старшая версия перекрывает младшую
	Name    string `json:"name"`    // «Фотосфера»: группа пресетов обложки и ряд кнопок страницы
	Gen     string `json:"gen"`     // родительный падеж: «Фотосферы» — в строках предела и «Не по шаблону …»
	Page    Page   `json:"page"`
	// Formats — кнопки страницы. С Side формат шаблонный: сторонку и заворот обложки задаёт он.
	Formats []PageFormat `json:"formats"`
	// Cover — общие числа обложки изделий (заворот, безопасные линии); nil — у каждого изделия свои.
	Cover       *Cover    `json:"cover,omitempty"`
	Products    []Product `json:"products,omitempty"`
	Default     string    `json:"default,omitempty"` // изделие, когда обложки типографии нет
	NameDefault bool      `json:"nameDefault,omitempty"`
	// Export — выгрузка «для печати»; nil — типографии нет в списке «Печать» (Нетпринт).
	Export *Export `json:"export,omitempty"`
}

// Page — общие поля кнопок страницы.
type Page struct {
	DPI     int     `json:"dpi"`
	Layflat bool    `json:"layflat,omitempty"`
	Safe    float64 `json:"safe,omitempty"`  // безопасная зона от края файла, мм; у формата своя перекрывает
	Bleed   float64 `json:"bleed,omitempty"` // обрез от края файла, мм
	Hint    string  `json:"hint,omitempty"`  // подсказка кнопки; подстановка {safe} — зона формата
}

// PageFormat — формат (кнопка) типографии.
type PageFormat struct {
	Name  string  `json:"name"`            // «30×20»
	Page  Pair    `json:"page"`            // страница с вылетом, мм
	Px    *PxPair `json:"px,omitempty"`    // цель в пикселях файла разворота; не используется в (В)
	Safe  float64 `json:"safe,omitempty"`  // 0 — из Page
	Bleed float64 `json:"bleed,omitempty"` // 0 — из Page
	Side  *Pair   `json:"side,omitempty"`  // сторонка обложки по шаблону, мм; есть — формат шаблонный
	Wrap  float64 `json:"wrap,omitempty"`  // заворот шаблона, мм
	// Guide — направляющие экрана по шаблону типографии; nil — прежние линии. Лист и файл от них не зависят.
	Guide *Guide `json:"guide,omitempty"`
}

// Guide — направляющие формата по осям, мм (123 Б5, PSD Фабрики): обрез разворота и загиб обложки
// «[бок, верх-низ]». Хотя бы одно поле задано; Fold — только у формата с Side.
type Guide struct {
	Bleed *Pair `json:"bleed,omitempty"` // полоса обреза разворота: бок и верх-низ
	Fold  *Pair `json:"fold,omitempty"`  // линия загиба обложки от края листа: бок и верх-низ
}

// Cover — общие числа обложки.
type Cover struct {
	Wrap      float64 `json:"wrap,omitempty"`
	Safe      float64 `json:"safe,omitempty"`      // безопасная линия сторонки от загиба; 0 — Settings.Safe
	SpineSafe float64 `json:"spineSafe,omitempty"` // от края корешка
	// FoldSafe и GuideSpineSafe — безопасные линии направляющих формата с guide.fold: от загиба внутрь
	// (линия риска шаблона) и от края корешка. Отдельно от Safe/SpineSafe: те попадают в данные альбома
	// (CoverSpec), а направляющие — производные и действуют, пока пользователь своих не задал.
	FoldSafe       float64 `json:"foldSafe,omitempty"`
	GuideSpineSafe float64 `json:"guideSpineSafe,omitempty"`
}

// Product — изделие: пресет обложки со своей таблицей корешка и пределом.
type Product struct {
	ID    string `json:"id"`              // id пресета обложки; после выпуска не меняется
	Name  string `json:"name"`            // «Фотобумага»
	Pages string `json:"pages,omitempty"` // бумага в памятке
	Item  string `json:"item,omitempty"`  // название изделия: заголовок памятки и путь на сайте
	URL   string `json:"url,omitempty"`   // страница изделия на сайте
	// Spine — таблица корешка плоским списком «до N разворотов, мм»: [8,8, 12,10, …].
	Spine []float64 `json:"spine,omitempty"`
	// SpineBy — таблица корешка для отдельных форматов вместо Spine (подпись формата → таблица):
	// у Фабрики 30×20 прослойка 2 мм идёт до 10 разворотов.
	SpineBy map[string][]float64 `json:"spineBy,omitempty"`
	// Wrap, Hinge, A, B — числа изделия без таблицы форматов и корешка (Нетпринт); Wrap
	// перекрывает Cover.Wrap.
	Wrap   float64 `json:"wrap,omitempty"`
	Hinge  float64 `json:"hinge,omitempty"`
	A      float64 `json:"a,omitempty"`
	B      float64 `json:"b,omitempty"`
	Limits []Limit `json:"limits,omitempty"`
}

// Limit — сколько разворотов принимает изделие; строка с Format точнее строки без него.
type Limit struct {
	Format string `json:"format,omitempty"` // подпись формата; "" — все
	Min    int    `json:"min,omitempty"`
	Max    int    `json:"max,omitempty"` // 0 — верх таблицы корешка (SpineBy формата или Spine)
	Pages  bool   `json:"pages,omitempty"`
}

// Export — как типография выгружается и что сказать фотографу. Подстановки: {item} в Site —
// название изделия в кавычках-ёлочках (нет изделия — SiteNoItem).
type Export struct {
	Acc        string `json:"acc"`                  // винительный падеж: «Фотосферу Pro»
	Title      string `json:"title"`                // подпись пункта «Печати»
	Prefix     string `json:"prefix,omitempty"`     // перед именем файла: «1-»
	Quality    int    `json:"quality,omitempty"`    // качество JPEG; 0 — как выбрано в окне
	CoverWord  string `json:"coverWord"`            // «фотообложки» | «обложки»
	PagesCap   string `json:"pagesCap"`             // подпись бумаги в памятке
	Plain      string `json:"plain"`                // изделие без обложки типографии
	WithCover  string `json:"withCover,omitempty"`  // хвост заголовка памятки с обложкой
	Done       string `json:"done"`                 // заголовок окна «Готово»
	Site       string `json:"site"`                 // путь на сайте
	SiteNoItem string `json:"siteNoItem,omitempty"` // вместо {item}, когда изделия обложки нет
	SiteButton string `json:"siteButton"`
	URL        string `json:"url,omitempty"` // сайт, когда у изделия обложки своего нет
}

// Steps — таблица корешка плоским списком в пары (до N разворотов, мм).
func Steps(flat []float64) []Step {
	out := make([]Step, 0, len(flat)/2)
	for i := 0; i+1 < len(flat); i += 2 {
		out = append(out, Step{Upto: int(flat[i]), Mm: flat[i+1]})
	}
	return out
}

// Step — строка таблицы корешка: до Upto разворотов включительно корешок Mm.
type Step struct {
	Upto int
	Mm   float64
}

// SpineFor — таблица корешка изделия для формата: SpineBy[format], иначе Spine.
func (pr Product) SpineFor(format string) []float64 {
	if t, ok := pr.SpineBy[format]; ok {
		return t
	}
	return pr.Spine
}

// Template — формат шаблонный (у него есть сторонка).
func (f PageFormat) Template() bool { return f.Side != nil }

// SafeIn и BleedIn — зона и обрез формата: своя или общая страницы.
func (f PageFormat) SafeIn(p Profile) float64 {
	if f.Safe > 0 {
		return f.Safe
	}
	return p.Page.Safe
}

// BleedIn — обрез формата: свой или общий страницы.
func (f PageFormat) BleedIn(p Profile) float64 {
	if f.Bleed > 0 {
		return f.Bleed
	}
	return p.Page.Bleed
}

// FormatByName — формат по подписи.
func (p Profile) FormatByName(name string) (PageFormat, bool) {
	for _, f := range p.Formats {
		if f.Name == name {
			return f, true
		}
	}
	return PageFormat{}, false
}

// Clone — глубокая копия: ни один срез, карта и указатель не общие с p. Снимок реестра отдаёт
// только копии (registry.go), иначе правка полученного профиля тихо меняла бы таблицы всех.
// Новое поле-ссылка в Profile добавляется сюда же (TestCloneNoSharing обходит тип через reflect).
func (p Profile) Clone() Profile {
	out := p
	out.Formats = slices.Clone(p.Formats)
	for i := range out.Formats {
		f := &out.Formats[i]
		f.Px = clonePtr(f.Px)
		f.Side = clonePtr(f.Side)
		f.Guide = f.Guide.clone()
	}
	out.Cover = clonePtr(p.Cover)
	out.Products = slices.Clone(p.Products)
	for i := range out.Products {
		pr := &out.Products[i]
		pr.Spine = slices.Clone(pr.Spine)
		pr.SpineBy = maps.Clone(pr.SpineBy)
		for k, t := range pr.SpineBy {
			pr.SpineBy[k] = slices.Clone(t)
		}
		pr.Limits = slices.Clone(pr.Limits)
	}
	out.Export = clonePtr(p.Export)
	return out
}

// clone — копия направляющих; nil остаётся nil.
func (g *Guide) clone() *Guide {
	if g == nil {
		return nil
	}
	return &Guide{Bleed: clonePtr(g.Bleed), Fold: clonePtr(g.Fold)}
}

// clonePtr — копия значения за указателем; nil остаётся nil.
func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
