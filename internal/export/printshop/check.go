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
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// DefaultFolder — папка книги несохранённого альбома или с негодным именем файла.
const DefaultFolder = "Альбом"

// Plan — готовность к выгрузке для типографии. Format "" — страница не из её таблицы или
// лист 031: Block объясняет причину. Флажок в окне есть и при Format "" (120), кроме листа 031
// (его отсекает App.PrintPlans); у чужой страницы подсказка флажка — Off.
type Plan struct {
	Kind       string   `json:"kind"`       // вид типографии, как в профиле
	Title      string   `json:"title"`      // «Для Фотосферы Pro» — подпись флажка
	Format     string   `json:"format"`     // "30×30"; "" — страница не её формат (или лист 031)
	Off        string   `json:"off"`        // «страница 300 × 200 мм — не формат Фабрики Фотокниги»; "" — формат свой
	Pages      string   `json:"pages"`      // бумага изделия обложки («Тонкие», «Картон»); "" — нет обложки типографии
	Px         string   `json:"px"`         // «7181×2398 px» — файл разворота при выгрузке; "" — страница не формат
	Default    bool     `json:"default"`    // окно открывается на этом пункте (DefaultKind); без omitempty: поле есть всегда, это контракт с UI
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
	// FitW, FitH — размер файла разворота, к которому выгрузка подгоняет холст (render.Options.Fit,
	// 123 §5); 0 — без подгонки. Fit — на сколько px по оси; для памятки. В интерфейс не идут.
	FitW int `json:"-"`
	FitH int `json:"-"`
	Fit  int `json:"-"`
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
	pg := p.Geo(nil).Page(0) // страницу читает только геометрия листа (TestPageWReaders)
	f, ok := model.FormatOf(prof.Group, p.Settings)
	if !ok {
		size := fmt.Sprintf("%s × %s мм — не формат %s", model.MmText(pg.W), model.MmText(pg.H), prof.Gen())
		pl.Off = "страница " + size // подсказка у флажка; запрет ниже — та же фраза с заглавной
		// Путь по интерфейсу: формат выбирают в «Настройках альбома» (типография и кнопка формата) или вводят px файла.
		pl.Block = append(pl.Block, "Страница "+size+": "+notFormatPx(pg.W, pg.H, p.Settings, prof)+
			". В «Настройках альбома» выберите типографию «"+prof.Group+"» и кнопку формата или введите px файла")
		return pl
	}
	pl.Format = f.Name
	pl.Px = pxText(pg.W, pg.H)
	if fw, fh, ok := f.FitPx(pg.W, pg.H); ok { // формат с px: файл строго по цели, а не по мм страницы
		nw, nh := pxFile(pg.W, pg.H)
		pl.FitW, pl.FitH, pl.Fit = fw, fh, max(abs(fw-nw), abs(fh-nh))
		pl.Px = fmt.Sprintf("%d×%d px", fw, fh)
	}
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

// pxFile — размер файла разворота страницы w × h мм при выгрузке для типографии (всегда PrintDPI):
// тот же холст, что рисует экспорт (render.CanvasSize).
func pxFile(w, h float64) (pw, ph int) {
	return render.CanvasSize(render.Options{PageW: w, PageH: h, DPI: model.PrintDPI})
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// pxText — «7181×2398 px» для памятки.
func pxText(w, h float64) string {
	pw, ph := pxFile(w, h)
	return fmt.Sprintf("%d×%d px", pw, ph)
}

// notFormatPx — хвост запрета «не формат» (123 §1.4): сколько px выйдет у страницы и сколько у
// ближайшего формата типографии: «разворот выйдет 7087 × 2362 px, у 30×20 — 7181 × 2398 (304 × 203 мм)».
func notFormatPx(w, h float64, s model.Settings, prof Profile) string {
	near := model.FormatNear(prof.Group, s)
	pw, ph := pxFile(w, h)
	nw, nh := pxFile(near.PageW, near.PageH)
	return fmt.Sprintf("разворот выйдет %d × %d px, у %s — %d × %d (%s × %s мм)", pw, ph, near.Name, nw, nh,
		model.MmText(near.PageW), model.MmText(near.PageH))
}

// coverRules — запреты по обложке (правила 4–6): свои размеры, чужая группа, не по шаблону,
// изделия нет у формата. missing — последнее: строка предела тогда не пишется.
func (pl *Plan) coverRules(p *model.Project, prof Profile, pr model.CoverPreset, own bool, f model.PrintFormat) (missing bool) {
	c := p.Cover
	switch {
	case c != nil && c.Spec.Preset == model.CoverCustom:
		pl.Block = append(pl.Block, "Обложка со своими размерами — выберите «"+prof.Group+"» в карточке обложки")
	case c != nil && model.CoverMissing(c.Spec.Preset) != "":
		// Изделия нет в реестре (своя типография с другого компьютера): без этой ветки вышло бы «для « · »».
		pl.Block = append(pl.Block, "Обложка размечена для типографии, которой нет на этом компьютере ("+c.Spec.Preset+")")
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
	if pl.Px != "" {
		files := "Файлы: " + pl.Px // размер файла разворота: сверить с требованием сайта
		if pl.Fit > 0 {
			files += fmt.Sprintf(" (подогнано на %d px по обрезу)", pl.Fit)
		}
		out = append(out, files)
	}
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
	fix, _, _ = strings.Cut(fix, model.CoverFixPxMark) // px файла — для выгрузки в типографию, в обычной dpi другой
	head, tail, ok := strings.Cut(fix, ": ")
	q := "Обложка " + strings.Replace(head, "Не по", "не по", 1)
	if ok {
		q += " (" + tail + ")"
	}
	return q + ". Выгрузить как есть?"
}
