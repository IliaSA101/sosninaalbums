// Package fsfera — выгрузка для Фотосферы Pro (спека 047): готовность альбома (проверки
// и памятка для калькулятора), имя и создание папки книги, посторонние файлы в ней.
// Отдельный пакет: в internal/export и internal/app по 20 файлов (решение 62); вид собирает
// Editor в дереве album, которому export не виден, поэтому окно берёт план методом
// App.FsferaPlan (решение 61). Проект только читается.
package fsfera

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

// maxNoCover — предел разворотов без обложки Фотосферы: больше всех в таблицах (тонкие листы).
const maxNoCover = 57

// Plan — готовность к выгрузке для Фотосферы Pro (047). Format "" — страница не из таблицы
// или лист 031: флажка в окне нет, Block объясняет причину.
type Plan struct {
	Format   string   `json:"format"`   // "30×30"
	Pages    string   `json:"pages"`    // "Тонкие" | "Картон" | "Двойной картон" | "" — нет обложки Фотосферы
	Spreads  int      `json:"spreads"`  // разворотов
	Spine    float64  `json:"spine"`    // корешок, мм; 0 — нет обложки Фотосферы
	Max      int      `json:"max"`      // предел разворотов: последняя строка таблицы; без обложки — 57
	Folder   string   `json:"folder"`   // имя папки книги
	Block    []string `json:"block"`    // мешает выгрузке; не nil
	CoverFix bool     `json:"coverFix"` // в Block есть «не по шаблону»: окно показывает «Исправить»
	FixAt    int      `json:"fixAt"`    // номер строки «не по шаблону» в Block, −1 — нет (0 — годный номер: не omitempty)
	Warn     []string `json:"warn"`     // не мешает; не nil
	Memo     []string `json:"memo"`     // строки памятки для калькулятора; не nil
}

// pagesName — тип листов в калькуляторе по id пресета обложки.
var pagesName = map[string]string{"fsfera-thin": "Тонкие", "fsfera-card": "Картон", "fsfera-double": "Двойной картон"}

// Check — готовность проекта p к выгрузке (albumPath — файл альбома, "" — несохранённый).
// Порядок строк Block — порядок правил design §3.3.
func Check(p *model.Project, albumPath string) Plan {
	n := len(p.Spreads)
	pl := Plan{Spreads: n, Max: maxNoCover, Folder: FolderName(albumPath), Block: []string{}, FixAt: -1, Warn: []string{}, Memo: []string{}}
	if p.Settings.Sheet {
		pl.Block = append(pl.Block, "Лист виньетки в Фотосферу Pro не выгружается")
		return pl
	}
	f, ok := model.FsferaFormatOf(p.Settings)
	if !ok {
		pg := p.Geo(nil).Page(0)
		pl.Block = append(pl.Block, fmt.Sprintf("Страница %s × %s мм — не формат Фотосферы. В «Страница» выберите пресет Фотосферы",
			model.MmText(pg.W), model.MmText(pg.H)))
		return pl
	}
	pl.Format = f.Name
	if n == 0 {
		pl.Block = append(pl.Block, "В альбоме нет разворотов")
	}
	c := p.Cover
	pr, fs := coverPreset(p)
	switch {
	case c != nil && c.Spec.Preset == model.CoverCustom:
		pl.Block = append(pl.Block, "Обложка со своими размерами — выберите «Фотосфера» в карточке обложки")
	case c != nil && !fs:
		pl.Block = append(pl.Block, fmt.Sprintf("Обложка размечена для «%s · %s» — выберите «Фотосфера» в карточке обложки", pr.Group, pr.Name))
	case fs:
		pl.Pages, pl.Spine = pagesName[pr.ID], p.CoverGeo().Spine
		if st := pr.Spec.Steps; len(st) > 0 {
			pl.Max = st[len(st)-1].Upto
		}
		if fix := model.CoverFixText(p); fix != "" {
			pl.FixAt, pl.CoverFix = len(pl.Block), true
			pl.Block = append(pl.Block, fix)
		}
	}
	if n > pl.Max {
		who := "Фотосферы"
		if fs {
			who = "«" + pr.Name + "»"
		}
		pl.Block = append(pl.Block, fmt.Sprintf("Разворотов %d — у %s не больше %d", n, who, pl.Max))
	}
	if c == nil {
		pl.Warn = append(pl.Warn, "Обложки нет: выгрузятся только развороты (для фотообложки нужен 00.jpg)")
	}
	if s := p.Settings.Safe; s < f.Safe {
		pl.Warn = append(pl.Warn, fmt.Sprintf("Безопасная зона %s мм — у Фотосферы %s мм: проверьте края (Вид ▸ Безопасная зона)",
			model.MmText(s), model.MmText(f.Safe)))
	}
	pl.Memo = memo(pl, c != nil)
	return pl
}

// coverPreset — пресет обложки p для её страницы; fs — обложка «Фотосферы».
func coverPreset(p *model.Project) (model.CoverPreset, bool) {
	if p.Cover == nil {
		return model.CoverPreset{}, false
	}
	pr, ok := model.CoverPresetFor(p.Cover.Spec.Preset, p.Settings)
	return pr, ok && pr.Group == model.CoverGroupFsfera
}

// memo — строки памятки (сц. 8): изделие, листы, развороты с корешком, папка и файлы.
func memo(pl Plan, cover bool) []string {
	head := "Фотокнига " + pl.Format
	if cover {
		head += ", обложка «Фото»"
	}
	out := []string{head}
	if pl.Pages != "" {
		out = append(out, "Страницы: "+pl.Pages)
	}
	spreads := fmt.Sprintf("Разворотов: %d", pl.Spreads)
	if pl.Spine > 0 {
		spreads += fmt.Sprintf(" (корешок %s мм)", model.MmText(pl.Spine))
	}
	out = append(out, spreads)
	var files []string
	if cover {
		files = append(files, export.CoverName)
	}
	switch n := pl.Spreads; {
	case n == 1:
		files = append(files, export.FileName(0, n))
	case n > 1:
		files = append(files, strings.TrimSuffix(export.FileName(0, n), ".jpg")+"…"+export.FileName(n-1, n))
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
