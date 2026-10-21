package printshop

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
)

// Profile — типография с выгрузкой «для печати» (047, 081 №7): чем она отличается от соседей —
// имена, тексты, качество и таблица форматов. Третья типография — ещё один профиль.
type Profile struct {
	Kind    string // вид типографии: Plan.Kind и ExportOptions.Print
	Group   string // model.CoverGroup…: группа пресетов обложки и предела 085; она же — имя в текстах
	Acc     string // винительный падеж: «Лист виньетки в Фотосферу Pro не выгружается»
	Title   string // подпись флажка окна экспорта
	Prefix  string // перед именем файла: "" — 00.jpg, 01.jpg; «1-» — 1-00.jpg, 1-01.jpg (081 №9)
	Quality int    // качество JPEG; 0 — как выбрано в окне (Фабрика — 100, 081 №10)
	Formats func() []model.PrintFormat

	coverWord  string                   // «фотообложки» | «обложки» — в строке «Обложки нет…»
	pagesCap   string                   // подпись бумаги в памятке
	products   map[string]product       // id пресета обложки → изделие
	plain      string                   // изделие в памятке и на сайте без обложки группы
	withCover  string                   // хвост заголовка памятки, когда обложка есть (Фотосфера)
	done       string                   // заголовок окна «Готово»
	site       func(name string) string // путь на сайте до файлов; name — изделие обложки, "" — нет
	siteButton string
	url        string // адрес, когда у изделия обложки своего нет
}

// product — изделие типографии по пресету обложки: бумага для памятки, название изделия
// (заголовок памятки и путь на сайте) и его страница на сайте.
type product struct{ pages, name, url string }

// Адреса — только отсюда: интерфейс адрес не передаёт (047).
const (
	fabrikaURL         = "https://fabrika-fotoknigi.com/photo-books"
	fabrikaLayflatURL  = "https://fabrika-fotoknigi.com/layflat"
	fabrikaPhotoURL    = "https://fabrika-fotoknigi.com/photobook-photopaper"
	fabrikaLayflatName = "Лайфлат"
	fabrikaPhotoName   = "Фотокнига на фотобумаге"
)

// Fsfera — Фотосфера Pro (047): тексты прежние, байт в байт.
var Fsfera = Profile{
	Kind: "fsfera", Group: model.CoverGroupFsfera, Acc: "Фотосферу Pro", Title: "Для Фотосферы Pro",
	Formats: model.FsferaFormats, coverWord: "фотообложки", pagesCap: "Страницы: ",
	products: map[string]product{
		"fsfera-thin": {pages: "Тонкие"}, "fsfera-card": {pages: "Картон"}, "fsfera-double": {pages: "Двойной картон"},
	},
	plain: "Фотокнига", withCover: ", обложка «Фото»", done: "Готово для Фотосферы",
	site: func(string) string {
		return "калькулятор Pro → эти параметры → «Загрузить»"
	}, siteButton: "Открыть Фотосферу",
}

// Fabrika — Фабрика Фотокниги (081): имена 1-NN.jpg для их загрузчика, качество 100.
var Fabrika = Profile{
	Kind: "fabrika", Group: model.CoverGroupFabrika, Acc: "Фабрику Фотокниги", Title: "Для Фабрики Фотокниги",
	Prefix: "1-", Quality: 100, Formats: model.FabrikaFormats, coverWord: "обложки", pagesCap: "Разворот: ",
	products: map[string]product{
		"fabrika-photo":          {"Фотобумага", fabrikaPhotoName, fabrikaPhotoURL},
		"fabrika-photo-1":        {"Фотобумага, прослойка 1 мм", fabrikaPhotoName, fabrikaPhotoURL},
		"fabrika-photo-2":        {"Фотобумага, прослойка 2 мм", fabrikaPhotoName, fabrikaPhotoURL},
		"fabrika-layflat-paper":  {"Бумага", fabrikaLayflatName, fabrikaLayflatURL},
		"fabrika-layflat-card":   {"Картон", fabrikaLayflatName, fabrikaLayflatURL},
		"fabrika-layflat-card-1": {"Картон, прослойка 1 мм", fabrikaLayflatName, fabrikaLayflatURL},
	},
	plain: "Фотокнига", done: "Готово для Фабрики Фотокниги",
	site: fabrikaSite, siteButton: "Открыть Фабрику Фотокниги", url: fabrikaURL,
}

// fabrikaSite — путь до загрузки на сайте Фабрики: изделие в кавычках, без обложки — «изделие».
func fabrikaSite(name string) string {
	if name == "" {
		name = "изделие"
	} else {
		name = "«" + name + "»"
	}
	return "Фотокниги → " + name + " → эти параметры → «Создать заказ»"
}

// Profiles — типографии в порядке флажков окна экспорта.
func Profiles() []Profile { return []Profile{Fsfera, Fabrika} }

// ProfileOf — профиль по Kind; ok=false — такой типографии нет.
func ProfileOf(kind string) (Profile, bool) {
	for _, pf := range Profiles() {
		if pf.Kind == kind && kind != "" {
			return pf, true
		}
	}
	return Profile{}, false
}

// Gen — родительный падеж из таблицы пределов 085: один источник со «Сведениями».
func (pf Profile) Gen() string {
	g, _ := model.PrintGroupOf(pf.Group)
	return g.Gen
}

// CoverName — имя файла обложки этой типографии: 00.jpg, 1-00.jpg.
func (pf Profile) CoverName() string { return pf.Prefix + export.CoverName }

// NoCoverQuestion — вопрос перед папкой, когда обложки нет (047 сц. 7): «…для фотообложки нужен 00.jpg…».
func (pf Profile) NoCoverQuestion() string {
	return "Обложки нет: в папке будут только развороты, а для " + pf.coverWord + " нужен " + pf.CoverName() + ". Выгрузить без обложки?"
}

// URL — страница сайта для кнопки окна «Готово»: изделие обложки этой группы, иначе общая.
func (pf Profile) URL(p *model.Project) string {
	if pr, ok := pf.productOf(p); ok && pr.url != "" {
		return pr.url
	}
	return pf.url
}

// productOf — изделие по обложке p; ok=false — обложки нет или она не этой группы.
func (pf Profile) productOf(p *model.Project) (product, bool) {
	if p == nil || p.Cover == nil {
		return product{}, false
	}
	pr, ok := pf.products[p.Cover.Spec.Preset]
	return pr, ok
}
