package printshop

import (
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/export"
)

// Profile — типография с выгрузкой «для печати» (047, 081 №7): чем она отличается от соседей —
// имена, тексты и качество (форматы — в model.GroupFormats по Group). Третья типография — ещё один профиль.
type Profile struct {
	Kind    string // вид типографии: Plan.Kind и ExportOptions.Print
	Group   string // model.CoverGroup…: группа пресетов обложки и предела 085; она же — имя в текстах
	Acc     string // винительный падеж: «Лист виньетки в Фотосферу Pro не выгружается»
	Title   string // подпись флажка окна экспорта
	Prefix  string // перед именем файла: "" — 00.jpg, 01.jpg; «1-» — 1-00.jpg, 1-01.jpg (081 №9)
	Quality int    // качество JPEG; 0 — как выбрано в окне (Фабрика — 100, 081 №10)

	coverWord  string                   // «фотообложки» | «обложки» — в строке «Обложки нет…»
	pagesCap   string                   // подпись бумаги в памятке
	products   map[string]product       // id пресета обложки → изделие
	plain      string                   // изделие в памятке и на сайте без обложки группы
	withCover  string                   // хвост заголовка памятки, когда обложка есть (Фотосфера)
	done       string                   // заголовок окна «Готово»
	site       func(name string) string // путь на сайте до файлов; name — изделие обложки, "" — нет
	siteButton string                   // подпись кнопки сайта окна «Готово»; у невстроенных — хост (siteButtonOf)
	url        string                   // адрес, когда у изделия обложки своего нет
}

// product — изделие типографии по пресету обложки: бумага для памятки, название изделия
// (заголовок памятки и путь на сайте) и его страница на сайте.
type product struct{ pages, name, url string }

// fromProfile — профиль выгрузки из секции export профиля типографии (123 (В)): тексты, префикс,
// качество и адреса лежат в данных. Профиль без export в «Печати» не участвует (Нетпринт).
// builtin — профиль из встроенных: только им верим подпись кнопки сайта из файла.
func fromProfile(p printprof.Profile, builtin bool) Profile {
	e := p.Export
	pf := Profile{
		Kind: p.ID, Group: p.Name, Acc: e.Acc, Title: e.Title, Prefix: e.Prefix, Quality: e.Quality,
		coverWord: e.CoverWord, pagesCap: e.PagesCap, plain: e.Plain, withCover: e.WithCover, done: e.Done,
		siteButton: siteButtonOf(p, builtin), url: e.URL, products: make(map[string]product, len(p.Products)),
	}
	for _, pr := range p.Products {
		pf.products[pr.ID] = product{pages: pr.Pages, name: pr.Item, url: pr.URL}
	}
	pf.site = func(name string) string {
		if name == "" {
			return strings.ReplaceAll(e.Site, "{item}", e.SiteNoItem)
		}
		return strings.ReplaceAll(e.Site, "{item}", "«"+name+"»")
	}
	return pf
}

// siteButtonOf — подпись кнопки сайта окна «Готово» (повторный аудит 123 №3, решение 0.26-123-51):
// у своей и библиотечной — «Открыть <хост>», как в редакторе своей (App.exportOf), иначе чужой
// файл с хостом evil.example подписал бы кнопку «Открыть Фабрику Фотокниги». Текст из файла —
// только у встроенных.
func siteButtonOf(p printprof.Profile, builtin bool) string {
	switch host := p.SiteHost(); {
	case builtin:
		return p.Export.SiteButton
	case host != "":
		return "Открыть " + host
	}
	return "Открыть сайт типографии"
}

// Profiles — типографии с выгрузкой в порядке пунктов окна «Печать»: профили действующего снимка
// с секцией export. Пересчёт по каждому вызову дёшев (профилей три), зато слои видны сразу.
func Profiles() []Profile {
	var out []Profile
	snap := printprof.Current()
	for _, p := range snap.Profiles() {
		if p.Export != nil {
			out = append(out, fromProfile(p, snap.Source(p.ID) == printprof.SourceBuiltin))
		}
	}
	return out
}

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
