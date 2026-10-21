// Package printshop — выгрузка для типографии (047 Фотосфера Pro, 081 Фабрика Фотокниги):
// готовность альбома (проверки и памятка для сайта), имя и создание папки книги, посторонние
// файлы в ней. Типографии различает Profile (081 №7). Отдельный пакет: вид собирает Editor в
// дереве album, которому export не виден, поэтому окно берёт планы методом App.PrintPlans
// (решение 61). Проект только читается.
package printshop

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// DefaultFolder — папка книги несохранённого альбома или с негодным именем файла.
const DefaultFolder = "Альбом"

// Plan — готовность к выгрузке для типографии. Format "" — страница не из её таблицы или
// лист 031: флажка в окне нет, Block объясняет причину.
type Plan struct {
	Kind       string   `json:"kind"`       // вид типографии, как в профиле
	Title      string   `json:"title"`      // «Для Фотосферы Pro» — подпись флажка
	Format     string   `json:"format"`     // "30×30"
	Pages      string   `json:"pages"`      // бумага изделия обложки («Тонкие», «Картон»); "" — нет обложки типографии
	Spreads    int      `json:"spreads"`    // разворотов
	Spine      float64  `json:"spine"`      // корешок, мм; 0 — нет обложки типографии
	Min        int      `json:"min"`        // нижний предел разворотов (085); 0 — нет
	Max        int      `json:"max"`        // верхний предел: изделие обложки, без неё — изделие по умолчанию
	Folder     string   `json:"folder"`     // имя папки книги
	CoverName  string   `json:"coverName"`  // «00.jpg» | «1-00.jpg»
	Quality    int      `json:"quality"`    // качество JPEG типографии; 0 — как выбрано в окне
	Block      []string `json:"block"`      // мешает выгрузке; не nil
	CoverFix   bool     `json:"coverFix"`   // в Block есть «не по шаблону»: окно показывает «Исправить»
	FixAt      int      `json:"fixAt"`      // номер строки «не по шаблону» в Block, −1 — нет (0 — годный номер: не omitempty)
	Warn       []string `json:"warn"`       // не мешает; не nil
	Memo       []string `json:"memo"`       // строки памятки для сайта; не nil
	Done       string   `json:"done"`       // заголовок окна «Готово»
	Site       string   `json:"site"`       // «На сайте: …» окна «Готово»
	SiteButton string   `json:"siteButton"` // кнопка сайта окна «Готово»
}

// Check — готовность проекта p к выгрузке для типографии prof (albumPath — файл альбома,
// "" — несохранённый). Порядок строк Block — порядок правил (047 design §3.3, 081 design §2).
func Check(p *model.Project, albumPath string, prof Profile) Plan {
	n := len(p.Spreads)
	pl := Plan{Kind: prof.Kind, Title: prof.Title, Spreads: n, Folder: FolderName(albumPath), CoverName: prof.CoverName(),
		Quality: prof.Quality, Block: []string{}, FixAt: -1, Warn: []string{}, Memo: []string{}, Done: prof.done, SiteButton: prof.siteButton}
	if p.Settings.Sheet {
		pl.Block = append(pl.Block, "Лист виньетки в "+prof.Acc+" не выгружается")
		return pl
	}
	// Предел и текст — общие со «Сведениями об альбоме» (085), но для этой типографии: под
	// обложкой другой — её изделие по умолчанию без названия (081 С5).
	lim, hasLim := model.LimitIn(p, prof.Group)
	if hasLim {
		pl.Min, pl.Max = lim.Min, lim.Max
	}
	f, ok := model.FormatOf(prof.Group, p.Settings)
	if !ok {
		pg := p.Geo(nil).Page(0)
		pl.Block = append(pl.Block, fmt.Sprintf("Страница %s × %s мм — не формат %s. В «Страница» выберите пресет %s",
			model.MmText(pg.W), model.MmText(pg.H), prof.Gen(), prof.Gen()))
		return pl
	}
	pl.Format = f.Name
	if n == 0 {
		pl.Block = append(pl.Block, "В альбоме нет разворотов")
	}
	pr, own := coverPreset(p, prof)
	if !pl.coverRules(p, prof, pr, own, f) && hasLim && lim.Warn != "" {
		pl.Block = append(pl.Block, lim.Warn) // и «больше», и «меньше»: сайт такую книгу не примет (081 №14)
	}
	if p.Cover == nil {
		pl.Warn = append(pl.Warn, "Обложки нет: выгрузятся только развороты (для "+prof.coverWord+" нужен "+pl.CoverName+")")
	}
	if s := p.Settings.Safe; s < f.Safe {
		pl.Warn = append(pl.Warn, fmt.Sprintf("Безопасная зона %s мм — у %s %s мм: проверьте края (Вид ▸ Безопасная зона)",
			model.MmText(s), prof.Gen(), model.MmText(f.Safe)))
	}
	var item product
	if own {
		item, _ = prof.productOf(p)
	}
	pl.Memo = memo(pl, prof, item, p.Cover != nil)
	pl.Site = "На сайте: " + prof.site(item.name) + " → файлы папки «" + pl.Folder + "»."
	return pl
}

// coverRules — запреты по обложке (правила 4–6): свои размеры, чужая группа, не по шаблону,
// изделия нет у формата. missing — последнее: строка предела тогда не пишется.
func (pl *Plan) coverRules(p *model.Project, prof Profile, pr model.CoverPreset, own bool, f model.PrintFormat) (missing bool) {
	c := p.Cover
	switch {
	case c != nil && c.Spec.Preset == model.CoverCustom:
		pl.Block = append(pl.Block, "Обложка со своими размерами — выберите «"+prof.Group+"» в карточке обложки")
	case c != nil && !own:
		pl.Block = append(pl.Block, fmt.Sprintf("Обложка размечена для «%s · %s» — выберите «%s» в карточке обложки", pr.Group, pr.Name, prof.Group))
	case own:
		pl.Pages, pl.Spine = prof.products[pr.ID].pages, p.CoverGeo().Spine
		if fix := model.CoverFixText(p); fix != "" {
			pl.FixAt, pl.CoverFix = len(pl.Block), true
			pl.Block = append(pl.Block, fix)
		}
		if _, ok := model.LimitFor(pr.ID, f.Name); !ok {
			pl.Block = append(pl.Block, fmt.Sprintf("«%s» не бывает %s — выберите другую обложку", pr.Name, f.Name))
			return true
		}
	}
	return false
}

// coverPreset — пресет обложки p для её страницы; own — обложка группы типографии prof.
func coverPreset(p *model.Project, prof Profile) (model.CoverPreset, bool) {
	if p.Cover == nil {
		return model.CoverPreset{}, false
	}
	pr, ok := model.CoverPresetFor(p.Cover.Spec.Preset, p.Settings)
	return pr, ok && pr.Group == prof.Group
}

// memo — строки памятки (047 сц. 8, 081 сц. 10): изделие, бумага, развороты с корешком, папка
// и файлы. item — изделие обложки типографии (пустое — её обложки нет).
func memo(pl Plan, prof Profile, item product, cover bool) []string {
	head := prof.plain
	if item.name != "" {
		head = item.name
	}
	head += " " + pl.Format
	if cover {
		head += prof.withCover
	}
	out := []string{head}
	if pl.Pages != "" {
		out = append(out, prof.pagesCap+pl.Pages)
	}
	spreads := fmt.Sprintf("Разворотов: %d", pl.Spreads)
	if pl.Spine > 0 {
		spreads += fmt.Sprintf(" (корешок %s мм)", model.MmText(pl.Spine))
	}
	out = append(out, spreads)
	var files []string
	if cover {
		files = append(files, pl.CoverName)
	}
	switch n := pl.Spreads; {
	case n == 1:
		files = append(files, prof.Prefix+export.FileName(0, n))
	case n > 1:
		files = append(files, prof.Prefix+strings.TrimSuffix(export.FileName(0, n), ".jpg")+"…"+prof.Prefix+export.FileName(n-1, n))
	}
	folder := "Папка: " + pl.Folder
	if len(files) > 0 {
		folder += " — " + strings.Join(files, " и ")
	}
	return append(out, folder)
}

// FolderName — имя папки книги по файлу альбома: имя без расширения через paths.FolderName;
// несохранённый альбом и негодное имя — DefaultFolder.
func FolderName(albumPath string) string {
	if albumPath == "" {
		return DefaultFolder
	}
	base := filepath.Base(albumPath)
	return paths.FolderName(strings.TrimSuffix(base, filepath.Ext(base)), DefaultFolder)
}

// CoverFixQuestion — вопрос обычного экспорта по строке CoverView.Fix: «Обложка не по шаблону
// Фотосферы 30×30 (623 × 331 мм, нужно 626 × 326). Выгрузить как есть?»
func CoverFixQuestion(fix string) string {
	head, tail, ok := strings.Cut(fix, ": ")
	q := "Обложка " + strings.Replace(head, "Не по", "не по", 1)
	if ok {
		q += " (" + tail + ")"
	}
	return q + ". Выгрузить как есть?"
}
