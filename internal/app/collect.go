package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum/collect"
	"github.com/IliaSA101/sosninaalbums/internal/export/fsfera"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// «Собрать проект в папку» (069): копия альбома и всех его файлов в одну папку с относительными
// путями. Открытый альбом не меняется — собирается копия. Порядок как у Export: всё, что может
// отказать, и все вопросы — до копирования; папка — только из нативного диалога Go.

// collectTitle — заголовок вопросов сбора (у экспорта — «Экспорт»).
const collectTitle = "Собрать проект в папку"

// noUNC — отказ сетевой папке: относительные пути альбома у альбома на UNC не работают (salbum).
const noUNC = "Выберите папку на диске с буквой (C:, E:…): в сетевой папке альбом не найдёт фото по относительным путям"

// Диалоги и проводник сбора — переменные, чтобы тесты подменяли нативные окна.
var (
	askCollect = func(a *App, msg string) (bool, error) {
		opts := askOptions(runtime.GOOS, msg)
		opts.Title = collectTitle
		ans, err := wr.MessageDialog(a.ctx, opts)
		return err == nil && ans == opts.Buttons[0], err
	}
	pickCollectDir = func(a *App) (string, error) {
		return wr.OpenDirectoryDialog(a.ctx, wr.OpenDialogOptions{
			Title:            "Куда собрать — папка альбома будет внутри",
			DefaultDirectory: a.dialogDir(prefs.DirCollect),
		})
	}
	openDir = osx.OpenDir
)

// CollectOptions — настройки окна сбора. Путь папки от интерфейса не берём (design §7).
type CollectOptions struct {
	All bool `json:"all"` // «Все фото из папок альбома»; иначе только фото альбома
}

// collectJob — проверенный сбор: снимок альбома, план для папки и файл собранного альбома.
type collectJob struct {
	p       *model.Project
	marks   map[string]model.Mark
	dir     string // папка альбома внутри выбранной
	album   string // dir/<имя>.salbum
	replace bool   // файл альбома уже есть: перед записью — резервная копия
	pl      collect.Plan
}

// collectExtra — пути фото ленты для режима «Все» и идёт ли ещё чтение папок (числа неполные).
func (a *App) collectExtra(all bool) (extra []string, scanning bool) {
	if !all {
		return nil, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	extra = make([]string, 0, len(a.photos))
	for _, ph := range a.photos {
		extra = append(extra, ph.Path)
	}
	return extra, a.scan.Active
}

// CollectInfo — сводка окна сбора: что войдёт и объём (без папки назначения). Проект не меняется.
func (a *App) CollectInfo(all bool) collect.Info {
	p, _ := a.editor().SnapshotMarks()
	extra, scanning := a.collectExtra(all)
	pl, err := collect.Make(p, extra, "", os.Stat)
	if err != nil {
		log.Printf("сбор: сводка: %v", paths.NoPath(err))
	}
	in := pl.Info()
	in.Scanning = scanning
	in.FolderName = fsfera.FolderName(a.editor().Path())
	return in
}

// Collect собирает проект в папку: вопросы, выбор папки, проверки — здесь; копирование и запись
// альбома — в фоне, ход событиями collect:progress и collect:done. false, nil — отказ пользователя.
func (a *App) Collect(o CollectOptions) (bool, error) {
	// Флаг до диалога: пока он открыт, второй вызов (из браузера) не пройдёт.
	a.mu.Lock()
	if a.collecting {
		a.mu.Unlock()
		return false, errors.New("сбор уже идёт")
	}
	a.collecting = true
	a.mu.Unlock()
	started := false
	defer func() {
		if !started {
			a.endCollect()
		}
	}()

	ed := a.editor()
	p, marks := ed.SnapshotMarks()
	open := ed.Path()
	extra, _ := a.collectExtra(o.All)
	pre, err := collect.Make(p, extra, "", os.Stat)
	if err != nil {
		return false, err
	}
	// Пропавшие — до диалога папки: ответ «нет» не должен стоить выбора папки.
	if text := missingQuestion(len(pre.Missing), pre.MissingGuides); text != "" {
		if ok, err := askCollect(a, text); !ok {
			return false, err
		}
	}
	parent, err := pickCollectDir(a)
	if err != nil || parent == "" {
		return false, err
	}
	a.rememberDir(prefs.DirCollect, parent)
	j, ok, err := a.prepareCollect(p, marks, extra, parent, open)
	if !ok || err != nil {
		return false, err
	}
	ctx, cancel := context.WithCancel(a.stopCtx)
	started = true
	a.mu.Lock()
	a.collectDir, a.collectCancel = j.dir, cancel
	a.mu.Unlock()
	a.bg.Go(func() { a.runCollect(ctx, cancel, j) })
	return true, nil
}

// prepareCollect — папка альбома в выбранной parent, план для неё, вопрос о замене альбома и
// место на диске. ok=false без ошибки — пользователь ответил «нет».
func (a *App) prepareCollect(p *model.Project, marks map[string]model.Mark, extra []string, parent, open string) (collectJob, bool, error) {
	if paths.CheckLocalPath(parent) != nil {
		return collectJob{}, false, errors.New(noUNC) //nolint:staticcheck // текст уходит пользователю тостом как есть
	}
	name := fsfera.FolderName(open) // имя файла альбома без расширения; черновик — «Альбом»
	dir, _, err := paths.BookDir(parent, name, "альбома")
	if err != nil {
		return collectJob{}, false, paths.NoPath(err)
	}
	if inPhotoFolder(dir, p.Folders) {
		return collectJob{}, false, errors.New(inFolders) //nolint:staticcheck // текст уходит пользователю тостом как есть
	}
	j := collectJob{p: p, marks: marks, dir: dir, album: filepath.Join(dir, name+projectExt)}
	if open != "" && (paths.SamePath(j.album, open) || sameFile(j.album, open)) {
		return collectJob{}, false, errors.New("Это файл открытого альбома — выберите другую папку") //nolint:staticcheck // текст уходит пользователю тостом как есть
	}
	if j.pl, err = collect.Make(p, extra, dir, os.Stat); err != nil {
		return collectJob{}, false, paths.NoPath(err)
	}
	text, err := replaceAlbumQuestion(j.album)
	if err != nil {
		return collectJob{}, false, err
	}
	if text != "" {
		if ok, err := askCollect(a, text); !ok {
			return collectJob{}, false, err
		}
		j.replace = true
	}
	if err := collect.CheckSpace(dir, j.pl.Bytes); err != nil {
		return collectJob{}, false, paths.NoPath(err)
	}
	return j, true, nil
}

// inFolders — отказ папке сбора внутри папки с фото: сканер ленты нашёл бы копии как новые фото.
const inFolders = "Папку сбора нельзя выбрать внутри папки с фото альбома — копии попадут в ленту. Выберите другую папку"

// inPhotoFolder — папка сбора dir совпадает с одной из папок альбома или лежит внутри неё.
func inPhotoFolder(dir string, folders []string) bool {
	for _, f := range folders {
		if paths.SamePath(dir, f) || library.Under(dir, f) {
			return true
		}
	}
	return false
}

// sameFile — a и b — один файл под разными именами: строкой не поймать subst, \\?\, 8.3,
// junction в пути и жёсткую ссылку. Файла нет — не один.
func sameFile(a, b string) bool {
	sa, err := os.Stat(a)
	if err != nil {
		return false
	}
	sb, err := os.Stat(b)
	return err == nil && os.SameFile(sa, sb)
}

// replaceAlbumQuestion — вопрос о замене альбома, который уже лежит в папке сбора; "" — его нет.
// На месте файла папка или ссылка — отказ: через ссылку не пишем.
func replaceAlbumQuestion(path string) (string, error) {
	st, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", nil
	case err != nil:
		return "", paths.NoPath(err)
	case !st.Mode().IsRegular():
		return "", fmt.Errorf("на месте «%s» — папка или ссылка: выберите другую папку", filepath.Base(path))
	}
	return fmt.Sprintf("В папке уже есть «%s» (изменён %s). Заменить? Прежний уйдёт в резервные копии.",
		filepath.Base(path), st.ModTime().Format("02.01 15:04")), nil
}

// runCollect — фон сбора: копирование, собранный альбом (резервная копия прежнего), итог.
// Отмена и ошибка копирования — альбом не пишется.
func (a *App) runCollect(ctx context.Context, cancel context.CancelFunc, j collectJob) {
	defer guard("сбор")
	// Флаг снимается один раз — до collect:done; здесь только при панике до него: после события
	// флаг может уже принадлежать новому сбору.
	ended := false
	defer func() {
		if !ended {
			a.endCollect()
		}
	}()
	defer cancel()
	start := time.Now()
	copied, err := collect.Copy(ctx, j.dir, &j.pl, func(df, tf int, db, tb int64) {
		a.emit("collect:progress", df, tf, db, tb)
	})
	copyFailed := err != nil
	if err == nil {
		q, qm := collect.Album(j.p, j.marks, j.dir, j.pl)
		if j.replace {
			err = a.backupReplaced(j.album)
		}
		if err == nil {
			err = collect.Save(q, qm, j.album, j.pl)
		}
	}
	msg, note, stopped := collectOutcome(j.pl, copied, err, copyFailed)
	if err != nil {
		log.Printf("сбор: %v", paths.NoPath(err))
	} else {
		files, size, _ := planTotals(j.pl)
		log.Printf("сбор: %d %s (%s) за %v", files, plural(files, "файл", "файла", "файлов"),
			collect.SizeText(size), time.Since(start).Round(time.Second))
	}
	// Флаг — до события: окно по collect:done снова даёт «Собрать…», и повтор сразу не должен
	// получить «сбор уже идёт».
	a.endCollect()
	ended = true
	a.emit("collect:done", j.dir, msg, note, stopped)
}

// errNoBackup — копия заменяемого альбома не сохранилась: вопрос «Заменить?» её обещал.
var errNoBackup = errors.New("Не удалось сохранить резервную копию прежнего альбома — альбом в папку не записан") //nolint:staticcheck // текст уходит пользователю тостом как есть

// backupReplaced — копия альбома, который заменит сбор. В отличие от backupBefore сбой — отказ
// записи, а не только лог: пользователь согласился на замену при условии копии.
func (a *App) backupReplaced(path string) error {
	if a.backups == nil || a.backups.Root == "" {
		log.Printf("сбор: резервная копия: папки копий нет")
		return errNoBackup
	}
	if err := a.backups.Force(path, backupKey(path)); err != nil {
		log.Printf("сбор: резервная копия: %s", paths.NoPath(err))
		return errNoBackup
	}
	if a.backups.PruneDue() {
		a.pruneBackups()
	}
	return nil
}

// endCollect снимает флаг сбора и его отмену.
func (a *App) endCollect() {
	a.mu.Lock()
	a.collecting, a.collectCancel = false, nil
	a.mu.Unlock()
}

// CancelCollect — «Остановить»: недокопированный файл удаляется, скопированные остаются, альбом
// не пишется. Сбора нет — ничего.
func (a *App) CancelCollect() {
	a.mu.Lock()
	cancel := a.collectCancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// ShowCollected открывает папку последнего сбора. Путь от интерфейса не принимаем — как
// ShowInExplorer.
func (a *App) ShowCollected() {
	a.mu.Lock()
	dir := a.collectDir
	a.mu.Unlock()
	showDir(dir)
}

// showDir открывает папку в проводнике, если она есть: не открылась — не страшно, ошибку
// показать нечем.
func showDir(dir string) {
	if st, err := os.Stat(dir); dir == "" || err != nil || !st.IsDir() {
		return
	}
	_ = openDir(dir)
}

// planTotals — файлов и байт в собранной папке по плану (с уже лежавшими) и сколько уже было.
func planTotals(pl collect.Plan) (files int, size int64, skipped int) {
	for _, it := range pl.Items {
		files++
		size += it.Size
		if it.Skip {
			skipped++
		}
	}
	return files, size, skipped
}

// collectOutcome — итог для collect:done: msg — ошибка (пусто при успехе и остановке), note —
// строка тоста, stopped — остановлен пользователем или выходом из программы.
func collectOutcome(pl collect.Plan, copied int, err error, copyFailed bool) (msg, note string, stopped bool) {
	files, size, skipped := planTotals(pl)
	total := files - skipped
	switch {
	case errors.Is(err, context.Canceled):
		return "", fmt.Sprintf("Сбор остановлен: скопировано %d из %d. Запустите сбор ещё раз — скопированное не копируется повторно.", copied, total), true
	case errors.Is(err, collect.ErrDiskFull):
		return fmt.Sprintf("Диск заполнен: скопировано %d из %d. Альбом в папку не записан.", copied, total), "", false
	case err != nil && copyFailed:
		return fmt.Sprintf("%v. Альбом в папку не записан.", paths.NoPath(err)), "", false
	case err != nil:
		return paths.NoPath(err).Error(), "", false
	}
	note = fmt.Sprintf("Проект собран: %d %s, %s", files, plural(files, "файл", "файла", "файлов"), collect.SizeText(size))
	if skipped > 0 {
		note += fmt.Sprintf(" (%d уже %s в папке)", skipped, plural(skipped, "был", "были", "были"))
	}
	var tail []string
	if n := len(pl.Missing); n > 0 {
		tail = append(tail, fmt.Sprintf("Не найдено фото: %d — в альбоме остались со старым путём.", n))
	}
	if pl.SideBusy > 0 {
		tail = append(tail, fmt.Sprintf("Не скопировано XMP: %d — имя в папке занято другим файлом.", pl.SideBusy))
	}
	if len(tail) > 0 {
		note += ". " + strings.Join(tail, " ")
	}
	return "", note, false
}

// missingQuestion — вопрос до выбора папки о файлах, которых нет; "" — все на месте.
func missingQuestion(photos, guides int) string {
	var what []string
	if photos > 0 {
		what = append(what, fmt.Sprintf("%d фото", photos))
	}
	if guides > 0 {
		what = append(what, fmt.Sprintf("%d %s типографии", guides, plural(guides, "разметка", "разметки", "разметок")))
	}
	switch {
	case len(what) == 0:
		return ""
	case photos == 1 && guides == 0:
		return "1 фото не найдено — в папку оно не попадёт, в собранном альбоме останется со старым путём. Собрать без него?"
	case photos == 0 && guides == 1:
		return "1 разметка типографии не найдена — в папку она не попадёт, в собранном альбоме останется со старым путём. Собрать без неё?"
	}
	return strings.Join(what, " и ") + " не найдены — в папку они не попадут, в собранном альбоме останутся со старым путём. Собрать без них?"
}

// plural — форма слова при числе n: 1 файл, 2 файла, 5 файлов (11–14 — «много»).
func plural(n int, one, few, many string) string {
	n %= 100
	if n < 0 {
		n = -n
	}
	switch {
	case n >= 11 && n <= 14:
		return many
	case n%10 == 1:
		return one
	case n%10 >= 2 && n%10 <= 4:
		return few
	}
	return many
}
