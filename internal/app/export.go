package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/export/printshop"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// colorReport считает фото альбома с не-sRGB профилем: adobe — сколько пересчитается в sRGB
// само, other — сколько фото с «чужим» профилем (имя профиля → число), их не пересчитываем.
// Цвет берётся из реестра ленты, а если фото там нет (папка закрыта или чтение не дошло) — из
// заголовка файла. Нечитаемые файлы пропускаются: экспорт упадёт на них с понятной ошибкой.
// Счёт — по файлам: копии (098) — тот же файл, без двойного счёта и второго чтения заголовка.
func (a *App) colorReport(p *model.Project) (adobe int, other map[string]int) {
	other = map[string]int{}
	files := make(map[string]model.PhotoRef, len(p.Photos)) // ID файла → любая запись семьи
	for id, ref := range p.Photos {
		files[model.BaseID(id)] = ref
	}
	a.mu.Lock()
	known := make(map[string]library.Photo, len(files))
	for id := range files {
		if ph, ok := a.byID[id]; ok {
			known[id] = ph
		}
	}
	a.mu.Unlock()
	for id, ref := range files {
		col, prof := known[id].Color, known[id].Profile
		if _, ok := known[id]; !ok {
			col, prof = imgfile.ReadColor(ref.Path)
		}
		switch col {
		case imgfile.ColorAdobe:
			adobe++
		case imgfile.ColorOther:
			if prof == "" {
				prof = "неизвестный профиль"
			}
			other[prof]++
		}
	}
	return adobe, other
}

// askOptions — окно вопроса «Да / Нет». На Mac Wails рисует кнопки строками Buttons как есть
// (и возвращает нажатую), на Windows — системные кнопки на языке ОС с ответом "Yes"/"No".
// Первая кнопка — «да», вторая (и по умолчанию, и по Esc) — «нет».
func askOptions(goos, msg string) wr.MessageDialogOptions {
	yes, no := "Yes", "No"
	if goos == "darwin" {
		yes, no = "Да", "Нет"
	}
	return wr.MessageDialogOptions{
		Type:          wr.QuestionDialog,
		Title:         "Экспорт",
		Message:       msg,
		Buttons:       []string{yes, no},
		DefaultButton: no,
		CancelButton:  no,
	}
}

// Диалоги экспорта — переменные, чтобы тесты подменяли нативные окна.
var (
	askYes = func(a *App, msg string) (bool, error) {
		opts := askOptions(runtime.GOOS, msg)
		ans, err := wr.MessageDialog(a.ctx, opts)
		return err == nil && ans == opts.Buttons[0], err
	}
	pickExportDir = func(a *App, title string) (string, error) {
		return wr.OpenDirectoryDialog(a.ctx, wr.OpenDialogOptions{
			Title:            title,
			DefaultDirectory: a.dialogDir(prefs.DirExport),
		})
	}
	// openURL — открыть адрес в браузере (подмена в тестах).
	openURL = func(a *App, url string) {
		osx.OpenURL(url, func(u string) { wr.BrowserOpenURL(a.ctx, u) })
	}
)

// fsferaURL — калькулятор Pro Фотосферы для «Открыть Фотосферу» (047): адрес только отсюда.
const fsferaURL = "https://fsfera.ru/order_pro/calculator/"

// OpenFsfera открывает калькулятор Pro в браузере. Адрес от интерфейса не принимаем — как
// ShowInExplorer: иначе страница в окне открывала бы что угодно.
func (a *App) OpenFsfera() { openURL(a, fsferaURL) }

// OpenFabrika открывает страницу изделия Фабрики Фотокниги по обложке альбома (лайфлат,
// фотобумага, иначе каталог). Адрес — константа printshop, от интерфейса не принимаем (081 №11).
func (a *App) OpenFabrika() {
	p, _ := a.editor().SnapshotSide()
	openURL(a, printshop.Fabrika.URL(p))
}

// PrintPlans — готовность альбома к выгрузке для типографий, в таблице которых есть страница
// (Format не пуст; 21×30 — Фотосфера и Фабрика): окно экспорта запрашивает их при открытии и
// при смене rev, флажок на план (081 №13). Проект не меняется; нет таких — пустой список.
func (a *App) PrintPlans() []printshop.Plan {
	p, _ := a.editor().SnapshotSide()
	out := []printshop.Plan{}
	for _, prof := range printshop.Profiles() {
		if pl := printshop.Check(p, a.editor().Path(), prof); pl.Format != "" {
			out = append(out, pl)
		}
	}
	return out
}

// printJob — режим «для типографии» (047, 081): её профиль и план — папка книги и памятка.
type printJob struct {
	prof printshop.Profile
	plan printshop.Plan
}

// printOptions — режим типографии prof (047 §4.1): запрет — ошибка до всех вопросов, иначе
// настройки окна заменяются заданными: все развороты целиком, 300 dpi, с обложкой, качество
// типографии (Фабрика — 100, 081 №10).
func printOptions(p *model.Project, path string, o *ExportOptions, prof printshop.Profile) (*printJob, error) {
	pl := printshop.Check(p, path, prof)
	if len(pl.Block) > 0 {
		return nil, errors.New(strings.Join(pl.Block, ". "))
	}
	o.Range, o.Pages, o.DPI, o.Cover, o.CoverOnly = "", false, 300, true, false
	if prof.Quality > 0 {
		o.Quality = prof.Quality
	}
	return &printJob{prof: prof, plan: pl}, nil
}

// coverQuestion — вопрос об обложке до выбора папки (047): в режиме типографии её нет,
// в обычном — обложка шаблонной группы не по шаблону (CoverView.Fix).
func coverQuestion(pj *printJob, cover *export.CoverJob, v model.View) string {
	switch {
	case pj != nil && cover == nil:
		return pj.prof.NoCoverQuestion()
	case pj == nil && cover != nil && v.Cover != nil && v.Cover.Fix != "":
		return printshop.CoverFixQuestion(v.Cover.Fix)
	}
	return ""
}

// bookDir — папка книги в выбранной dir (режим типографии): mk — путь, если её надо создать
// (os.Mkdir после всех вопросов). Без режима — dir как есть.
func bookDir(pj *printJob, dir string) (book, mk string, err error) {
	if pj == nil {
		return dir, "", nil
	}
	book, exists, err := printshop.BookDir(dir, pj.plan.Folder)
	if err != nil || exists {
		return book, "", err
	}
	return book, book, nil
}

// extraQuestion — вопрос о посторонних файлах в уже существующей папке книги: загрузчик
// типографии возьмёт их вместе с альбомом (решение 59).
func extraQuestion(prof printshop.Profile, dir string, written []string) string {
	n, first := printshop.Extra(dir, written, prof.Prefix)
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("В папке «%s» есть посторонние файлы (%d шт., например %s): %s загрузит их вместе с альбомом. Продолжить?",
		filepath.Base(dir), n, first, prof.Group)
}

// Export собирает развороты по настройкам окна экспорта: спрашивает папку и пишет в фоне.
// Ход работы приходит событиями export:progress и export:done; остановка — CancelExport.
func (a *App) Export(o ExportOptions) (bool, error) {
	// Флаг ставим до диалога: пока он открыт, второй вызов (из браузера) не пройдёт.
	a.mu.Lock()
	if a.exporting {
		a.mu.Unlock()
		return false, fmt.Errorf("экспорт уже идёт")
	}
	a.exporting = true
	a.mu.Unlock()
	started := false
	defer func() {
		if !started {
			a.mu.Lock()
			a.exporting = false
			a.mu.Unlock()
		}
	}()

	// Всё, что может отказать, — до диалога папки и удаления прежних файлов: иначе отказ пришёл бы после потерь.
	gen := fontGen() // до снимка: смена шрифтов между ними не проскочит (export.PinFonts)
	p, v := a.editor().Snapshot()
	var pj *printJob // режим «для типографии» (047, 081): папка книги, имена и памятка
	if o.Print != "" {
		prof, ok := printshop.ProfileOf(o.Print)
		if !ok {
			return false, errors.New("неизвестная типография")
		}
		var err error
		if pj, err = printOptions(p, a.editor().Path(), &o, prof); err != nil {
			return false, err
		}
	}
	o.Quality, o.DPI = export.ClampOptions(o.Quality, o.DPI, p.Settings.DPI)
	n := len(v.Spreads)
	// Обложка (032): нет её или проект-лист — флаги игнорируются; «только обложка» включает обложку.
	cover := export.NewCoverJob(p, v, o.DPI, o.Quality)
	if p.Settings.Sheet || (!o.Cover && !o.CoverOnly) {
		cover = nil
	}
	o.Cover, o.CoverOnly = cover != nil, o.CoverOnly && cover != nil
	if n == 0 && cover == nil {
		return false, fmt.Errorf("в альбоме нечего экспортировать")
	}
	o.Pages = o.Pages && !p.Settings.Sheet // у листа 031 «отдельными страницами» нет: один файл на лист
	g := p.Geo(nil)                        // холст — половина листа (032 §7): у разворота страница, у листа 031 — W/2
	opt := render.Options{PageW: g.W / 2, PageH: g.H, DPI: o.DPI, Quality: o.Quality}
	var pl export.Plan // «только обложка»: пустой план, не Full — хвосты разворотов не удаляются
	if !o.CoverOnly {
		units, err := exportUnits(o.Range, n, p.Settings.Sheet)
		if err != nil {
			return false, err
		}
		pl = export.NewPlan(units, n, o.Pages)
		if err := export.CheckCanvases(opt, o.Pages); err != nil {
			return false, err
		}
	}
	if cover != nil {
		if err := export.CheckCanvas(cover.Opt); err != nil {
			return false, err
		}
		pl = pl.WithCover(cover)
	}
	if pj != nil { // имена типографии (1-01.jpg у Фабрики) — и в записи, и в разборе папки
		pl = pl.WithPrefix(pj.prof.Prefix)
	}
	sel := pl.SelectedPhotos(v, opt.PageW)
	if err := export.MissingPhotos(p, sel, a.exists); err != nil {
		return false, err
	}
	if err := pl.MissingBorderFiles(v, opt.PageW, a.exists); err != nil {
		return false, err
	}
	// Профили проверяем тоже до диалога папки: ответ «нет» не должен стоить выбора папки.
	adobe, other := a.colorReport(export.PickPhotos(p, sel))
	if text := export.ColorWarning(other); text != "" {
		if ok, err := askYes(a, text); !ok {
			return false, err
		}
	}
	// Пустые развороты — тоже до папки, по тому же доводу (спека 013).
	// Лист только с фоном альбома считается пустым, но белым он не выйдет (0.23-g3g4 G4-4);
	// обложку фон альбома не красит (098-10) — одна пустая обложка остаётся «белой».
	blank := export.BlankSpreads(v, &pl)
	white := (p.Settings.Bg == "" || p.Settings.Bg == "#ffffff") && (p.AlbumBg == nil || len(blank) == 0)
	blankCover := cover != nil && export.BlankCover(v)
	if text := export.BlankWarningCover(blank, blankCover, white); text != "" {
		if ok, err := askYes(a, text); !ok {
			return false, err
		}
	}
	// Пустые поля подписей (030) и имена без фото (071) — тоже до папки: в печати они пусты. Только листы плана.
	if text := export.FieldsWarning(export.PlanFields(v, &pl)); text != "" {
		if ok, err := askYes(a, text); !ok {
			return false, err
		}
	}
	// Своих шрифтов нет на этом компьютере (082) — тоже до папки: текст выйдет шрифтом Montserrat.
	if text := export.FontsWarning(export.PlanFonts(v, &pl)); text != "" {
		if ok, err := askYes(a, text); !ok {
			return false, err
		}
	}
	if text := coverQuestion(pj, cover, v); text != "" {
		if ok, err := askYes(a, text); !ok {
			return false, err
		}
	}
	title := "Куда выгрузить"
	if pj != nil {
		title = "Куда выгрузить — папка книги будет внутри"
	}
	dir, err := pickExportDir(a, title)
	if err != nil || dir == "" {
		return false, err
	}
	a.rememberDir(prefs.DirExport, dir)
	dir, mkBook, err := bookDir(pj, dir) // дальше dir — папка книги; mkBook — создать её
	if err != nil {
		return false, err
	}
	// Пустые места после выбора папки, до вопроса о перезаписи: «нет» ничего не удаляет.
	if text := export.EmptyWarning(export.EmptySummaryFor(v, &pl, opt.PageW)); text != "" {
		if ok, err := askYes(a, text); !ok {
			return false, err
		}
	}

	var ours, foreign []export.DirFile
	if mkBook == "" { // новой папки книги ещё нет: читать нечего
		if ours, foreign, err = export.PlanDirPrefix(dir, pl.Prefix); err != nil {
			return false, fmt.Errorf("не удалось прочитать папку экспорта: %w", err)
		}
	}
	ours, foreign = export.AddCoverFile(pl, dir, ours, foreign) // 00.jpg (1-00.jpg): наш заменяется, чужой — отказ
	if foreign = export.HitFiles(pl, foreign); len(foreign) > 0 {
		return false, fmt.Errorf("В папке есть файлы с номерами разворотов (или %s) не из Sosnina Albums (%d шт., например %s). Выберите другую папку или уберите их.", pl.CoverName(), len(foreign), foreign[0].Name) //nolint:revive,staticcheck // текст уходит пользователю тостом как есть
	}
	var stale []string
	if hit := export.HitFiles(pl, ours); len(hit) > 0 {
		if ok, err := askYes(a, export.ReplaceQuestion(pl, hit)); !ok {
			return false, err
		}
		stale = export.StaleNames(hit, pl.Names(n)) // удалятся после записи новых
	}
	if pj != nil && cover == nil && mkBook == "" { // свой старый 00.jpg: загрузчик взял бы прежнюю обложку
		stale = append(stale, printshop.OldCover(dir, pj.prof.CoverName())...)
	}
	if pj != nil && mkBook == "" {
		if text := extraQuestion(pj.prof, dir, pl.Names(n)); text != "" {
			if ok, err := askYes(a, text); !ok {
				return false, err
			}
		}
	}
	if mkBook != "" { // после всех вопросов: «нет» не оставляет пустой папки
		if err := os.Mkdir(mkBook, 0o755); err != nil {
			log.Printf("экспорт: папка книги: %v", paths.NoPath(err))
			return false, fmt.Errorf("не удалось создать папку книги: %w", err)
		}
	}
	var memo []string // памятка окна «Готово» типографии; nil — обычный тост
	if pj != nil {
		memo = pj.plan.Memo
	}

	spreads := export.FromView(p, v)
	export.PinFonts(spreads, gen)
	j := exportJob{spreads: spreads, pl: pl, opt: opt, dir: dir, stale: stale, adobe: adobe, memo: memo}
	ctx, cancel := context.WithCancel(a.stopCtx)
	started = true
	a.mu.Lock()
	a.exportDir, a.exportCancel = dir, cancel
	a.mu.Unlock()
	if !a.goBG(func() { a.runExport(ctx, cancel, j) }) {
		cancel()
		a.endExport() // фон закрыт: выгрузки не будет, флаг не оставляем
		return false, nil
	}
	return true, nil
}

// exportJob — проверенная выгрузка: развороты, план, папка и что сказать по итогу.
type exportJob struct {
	spreads []render.Spread
	pl      export.Plan
	opt     render.Options
	dir     string
	stale   []string // прежние файлы: удаляются только после записи всех новых
	adobe   int      // фото Adobe RGB — для строки о пересчёте цвета
	memo    []string // памятка типографии; nil — обычный тост
}

// runExport — фон экспорта и итог export:done (dir, err, note, memo, stopped). Остановка
// (CancelExport или выход из программы) прерывает текущий разворот (ctx доходит до декодов и ожидания
// бюджета памяти, 080-8), недописанного файла нет: готовые файлы остаются,
// прежние не удаляются.
func (a *App) runExport(ctx context.Context, cancel context.CancelFunc, j exportJob) {
	defer guard("экспорт")
	// Флаг снимается один раз — до export:done; здесь только при панике до него: после события
	// флаг может уже принадлежать новому экспорту.
	ended := false
	defer func() {
		if !ended {
			a.endExport()
		}
	}()
	defer cancel()
	restoreMem := liftMemoryLimit()
	defer restoreMem() // при панике; обычно — до export:done ниже
	start := time.Now()
	done, total := 0, len(j.pl.Jobs)
	if j.pl.Cover != nil {
		total++
	}
	err := export.RunPlan(ctx, j.spreads, j.pl, j.opt, j.dir, j.stale, func(d, t int) {
		done, total = d, t
		a.emit("export:progress", d, t)
	})
	msg, note, memo, stopped := "", "", []string(nil), false
	switch {
	case errors.Is(err, context.Canceled):
		note, stopped = fmt.Sprintf("Экспорт остановлен: готово %d из %d. Уже выгруженные файлы остались в папке.", done, total), true
		log.Printf("экспорт: остановлен, готово %d из %d", done, total)
	case err != nil:
		msg = err.Error()
		log.Printf("экспорт: %v", paths.NoPath(err))
	default:
		note, memo = export.ColorNote(j.adobe), j.memo
		log.Printf("экспорт: %d разворотов за %v", len(j.pl.Jobs), time.Since(start).Round(time.Second))
		album.LogMem("экспорт")
		if note != "" {
			log.Print(note)
		}
	}
	// Предел и флаг — до события: окно по export:done снова даёт «Экспорт», и повтор сразу не должен
	// получить «экспорт уже идёт» или застать снятый предел.
	restoreMem()
	a.endExport()
	ended = true
	a.emit("export:done", j.dir, msg, note, memo, stopped)
}

// endExport снимает флаг экспорта и его отмену.
func (a *App) endExport() {
	a.mu.Lock()
	a.exporting, a.exportCancel = false, nil
	a.mu.Unlock()
}

// CancelExport — «Остановить»: текущий разворот прерывается (не дописывается), следующие не пишутся, прежние
// файлы папки не удаляются. Экспорта нет — ничего.
func (a *App) CancelExport() {
	a.mu.Lock()
	cancel := a.exportCancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// exportUnits — цели диапазона окна экспорта; "" — nil (все развороты). У листа 031 «л»/«п» — ошибка.
func exportUnits(rng string, n int, sheet bool) ([]export.Unit, error) {
	if strings.TrimSpace(rng) == "" {
		return nil, nil
	}
	units, err := export.ParseRange(rng, n)
	switch {
	case err != nil && sheet:
		// ParseRange общий с альбомом и говорит «разворота 7 нет»; у листа 031 единица — лист.
		err = errors.New(strings.Replace(err.Error(), "разворота", "листа", 1))
	case err == nil && sheet:
		err = export.CheckSheet(units)
	}
	return units, err
}

// ShowInExplorer открывает папку последнего экспорта. Путь от интерфейса не принимаем:
// иначе любая страница в окне смогла бы запускать explorer с произвольным аргументом.
func (a *App) ShowInExplorer() {
	a.mu.Lock()
	dir := a.exportDir
	a.mu.Unlock()
	showDir(dir)
}

// ExportOptions — настройки окна экспорта. Живёт в internal/app, а не в internal/export: Wails берёт
// пространство имён models.ts из имени пакета, а `export` — ключевое слово TS.
type ExportOptions struct {
	Range   string `json:"range"`   // "" — все развороты; «3-5, 8, 7п» (export.ParseRange)
	Quality int    `json:"quality"` // 100 | 95 | 80; другое — 95
	DPI     int    `json:"dpi"`     // 72..1200; 0 — из настроек альбома
	Pages   bool   `json:"pages"`   // по страницам: левая и правая отдельными файлами
	// Cover — выгрузить обложку 00.jpg (спека 032); обложки нет или проект-лист — игнорируется.
	// CoverOnly — только обложка, развороты и Range не выгружаются (включает Cover).
	Cover     bool `json:"cover"`
	CoverOnly bool `json:"coverOnly"`
	// Print — выгрузка для типографии (047, 081 С6): "" — обычная, иначе Kind профиля printshop
	// («fsfera», «fabrika»): папка книги, все развороты, 300 dpi, с обложкой, имена и качество
	// типографии; Range, Pages, DPI, Cover и CoverOnly игнорируются (Quality — при своём у
	// типографии), запреты printshop.Check — ошибка, неизвестная типография — тоже.
	Print string `json:"print"`
}
