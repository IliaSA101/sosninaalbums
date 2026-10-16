package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"weak"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/text"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Фото альбома на диске: чтение папок, миниатюры, /media, недостающие фото и перепривязка,
// показ фото в Проводнике.

// maxSkippedLog — сколько путей нечитаемых файлов писать в лог.
const maxSkippedLog = 20

// readColor — imgfile.ReadColor; подменяется в тестах (/media не должен читать недостающие).
var readColor = imgfile.ReadColor

// rescan запускает чтение папок проекта и сразу возвращается. Прежнее сканирование и его прогрев
// миниатюр отменяются; из ленты уходят фото, чьих папок больше нет (при смене проекта лента пустеет).
// Дальше интерфейс получает события `scan` (ход) и `library` (список порциями).
func (a *App) rescan() {
	defer guard("rescan")
	ctx, cancel := context.WithCancel(a.stopCtx)

	a.mu.Lock()
	// Список папок и поколение — под одним замком: два rescan подряд (OnFileDrop и Undo)
	// не могут поменяться местами, и новое поколение всегда с новым списком.
	folders := a.editor().Folders()
	if a.scanCancel != nil {
		a.scanCancel()
	}
	stopWarm := a.warm != nil
	if stopWarm {
		a.warm()
		a.warm = nil
	}
	a.scanCancel = cancel
	a.scanGen++
	gen := a.scanGen
	// Новые срезы, а не правка на месте: прежний уже мог уйти в GetState или в событие.
	kept := make([]library.Photo, 0, len(a.photos))
	for _, p := range a.photos {
		if slices.ContainsFunc(folders, func(f string) bool { return library.Under(p.Path, f) }) {
			kept = append(kept, p)
		}
	}
	a.photos, a.byID = kept, byIDMap(kept)
	a.scan = library.ScanProgress{Active: true, Walking: true}
	pr := a.scan
	a.mu.Unlock()

	if stopWarm {
		// Прогрев отменён: полоска «Готовлю миниатюры» сбрасывается (0/0 — прогрева нет).
		a.emitMu.Lock()
		a.emit("thumbs", 0, 0)
		a.emitMu.Unlock()
	}
	a.publish(gen, kept, pr)
	a.refreshMissing()
	a.bg.Go(func() {
		defer guard("сканирование")
		var fin library.ScanProgress
		photos, skipped, err := a.scanner.Scan(ctx, folders, func(ph []library.Photo, cur library.ScanProgress, final bool) {
			if final {
				fin = cur
			}
			a.publish(gen, ph, cur)
		})
		if err != nil {
			return // отменено: уже идёт новое сканирование
		}
		log.Printf("папок %d: %d фото (обход %v, чтение %v, из памяти %d), пропущено %d", len(folders), len(photos),
			fin.WalkTime.Round(time.Millisecond), fin.ReadTime.Round(time.Millisecond), fin.Memo, len(skipped))
		for i, s := range skipped {
			if i >= maxSkippedLog {
				log.Printf("  ... и ещё %d (всего не читается %d)", len(skipped)-i, len(skipped))
				break
			}
			log.Printf("  не читается: %s", filepath.Base(s))
		}
		a.warmThumbs(gen, photos)
	})
}

func byIDMap(photos []library.Photo) map[string]library.Photo {
	m := make(map[string]library.Photo, len(photos))
	for _, p := range photos {
		m[p.ID] = p
	}
	return m
}

// publish принимает отчёт сканирования поколения gen: обновляет реестр фото и ход, шлёт события
// `library` (если список пришёл) и `scan`. Устаревшее поколение молчит. emitMu сериализует отправку,
// чтобы старый список не обогнал новый; a.mu под отправкой не держим — /media не ждёт IPC.
func (a *App) publish(gen int, photos []library.Photo, pr library.ScanProgress) {
	var byID map[string]library.Photo
	if photos != nil {
		byID = byIDMap(photos)
	}
	a.emitMu.Lock()
	defer a.emitMu.Unlock()
	a.mu.Lock()
	if gen != a.scanGen {
		a.mu.Unlock()
		return
	}
	if photos != nil {
		a.photos, a.byID = photos, byID
	}
	a.scan = pr
	a.mu.Unlock()
	if photos != nil {
		a.emit("library", photos)
	}
	a.emit("scan", pr)
}

// warmThumbs запускает прогрев миниатюр после чтения папок, если поколение ещё актуально.
func (a *App) warmThumbs(gen int, photos []library.Photo) {
	if a.cache == nil {
		return
	}
	a.mu.Lock()
	if gen != a.scanGen {
		a.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(a.stopCtx)
	a.warm = cancel
	a.mu.Unlock()
	var lastMu sync.Mutex // колбэк зовут все воркеры прогрева
	last := time.Time{}
	a.cache.Warm(ctx, photos, func(done, total int) {
		lastMu.Lock()
		due := done == total || time.Since(last) > 200*time.Millisecond
		if due {
			last = time.Now()
		}
		lastMu.Unlock()
		if !due {
			return
		}
		// Устаревшее поколение молчит: иначе его хвост перебьёт сброс полоски после rescan.
		a.emitMu.Lock()
		defer a.emitMu.Unlock()
		a.mu.Lock()
		cur := gen == a.scanGen
		a.mu.Unlock()
		if cur {
			a.emit("thumbs", done, total)
		}
	})
}

func (a *App) infos(ids []string) []model.PhotoInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]model.PhotoInfo, 0, len(ids))
	for _, id := range ids {
		if p, ok := a.byID[id]; ok {
			info := model.PhotoInfo{ID: p.ID, Path: p.Path, W: p.W, H: p.H, Size: p.Size}
			if p.Xmp != nil { // отметки — только из сканера: ReadInfo их нарочно не читает
				info.Xmp = *p.Xmp
			}
			out = append(out, info)
		}
	}
	return out
}

// media отдаёт миниатюры: /media/t/<id> и /media/p/<id>; шрифты — /media/font/<файл>;
// библиотека фонов — /media/gen/*, /media/lib/* (serveDecor).
func (a *App) media(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/media/") {
			next.ServeHTTP(w, r)
			return
		}
		defer guard("media")
		if name, ok := strings.CutPrefix(r.URL.Path, "/media/font/"); ok {
			serveFont(w, r, name)
			return
		}
		if r.URL.Path == "/media/cover-guide" {
			a.serveCoverGuide(w, r)
			return
		}
		if a.serveDecor(w, r) {
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/media/"), "/")
		if len(parts) != 2 || a.cache == nil {
			http.NotFound(w, r)
			return
		}
		a.mu.Lock()
		p, ok := a.byID[parts[1]]
		_, gone := slices.BinarySearch(a.missing, parts[1]) // missingOf отдаёт по порядку
		a.mu.Unlock()
		if !ok && gone {
			// Файла нет (или том не отвечает): не читаем, иначе обработчик ждёт таймаута SMB.
			http.NotFound(w, r)
			return
		}
		if !ok {
			// Фото из проекта, чья папка сейчас не открыта.
			if ref, found := a.editor().Ref(parts[1]); found && reachable(ref.Path) {
				col, prof := readColor(ref.Path) // от цвета зависит ключ кэша и пересчёт
				p, ok = library.Photo{ID: parts[1], Path: ref.Path, Name: filepath.Base(ref.Path), Color: col, Profile: prof}, true
			}
		}
		if !ok {
			// Путь запроса в лог не пишем: промахов бывают сотни подряд.
			logRare(a.mediaMiss.Add(1), "media: фото не найдено")
			http.NotFound(w, r)
			return
		}
		file, err := a.cache.File(p, parts[0] == "p")
		if err != nil {
			logRare(a.mediaMiss.Add(1), "превью %s: %v", filepath.Base(p.Path), paths.NoPath(err))
			http.NotFound(w, r)
			return
		}
		// Тип задаём сами: копия бывает PNG (прозрачные картинки), а реестровый тип Windows ненадёжен.
		ct := "image/jpeg"
		if strings.EqualFold(filepath.Ext(file), ".png") {
			ct = "image/png"
		}
		w.Header().Set("Content-Type", ct)
		w.Header().Set("Cache-Control", "max-age=31536000, immutable")
		http.ServeFile(w, r, file) //nolint:gosec // file — путь из кэша миниатюр по ID из реестра, не из запроса
	})
}

// serveFont отдаёт встроенный шрифт. Имя ищется в белом списке text.FontFile:
// путь из запроса в файловую систему не попадает. Не immutable: файлы могут
// смениться с версией, а кэш WebView2 переживает обновление.
func serveFont(w http.ResponseWriter, r *http.Request, name string) {
	data, ok := text.FontFile(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "font/ttf")
	w.Header().Set("Cache-Control", "max-age=86400")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}

// Перенос проекта и перепривязка недостающих фото (спека 006, §8).

// DriveHint — подсказка «Диск E: → F:». From пуст — подсказки нет.
type DriveHint struct {
	From  string `json:"from"`  // буква пропавшего диска, «E»
	To    string `json:"to"`    // буква, на которой фото нашлись
	Found int    `json:"found"` // сколько пропавших фото с диска From лежит там же на To
	Total int    `json:"total"` // сколько пропавших фото было на диске From
}

// RelinkResult — итог поиска в папке: сколько привязано, сколько осталось
// недостающими и сколько нашлось в нескольких равноценных местах (не привязаны).
type RelinkResult struct {
	View      model.View `json:"view"`
	Found     int        `json:"found"`
	Missing   int        `json:"missing"`
	Ambiguous int        `json:"ambiguous"`
}

// missingOf — ID фото фотографа, чьих файлов нет (exists — false), по порядку ID. Свои из
// библиотеки (Lib, 048) не входят: в папке фотографа и на другом диске их не найти —
// они находятся сами, когда тот же файл загружен в библиотеку (RelinkLib).
func missingOf(refs map[string]model.PhotoRef, exists func(string) bool) []string {
	ids := library.MissingOf(refs, func(r model.PhotoRef) string { return r.Path },
		func(p string) bool { return p == "" || exists(p) }) // пустой путь — только у своих
	return slices.DeleteFunc(ids, func(id string) bool { return refs[id].Lib != "" })
}

// reachable — osx.Reachable; подменяется в тестах (мёртвый сетевой том).
var reachable = osx.Reachable

// fileExists — обычный файл на месте; том, который не отвечает (отключённый сетевой диск),
// — «нет файла» без ожидания таймаута SMB.
func fileExists(path string) bool {
	if !reachable(path) {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

// refreshMissing — полный пересчёт недостающих (открытие, rescan) в фоне: os.Stat тысяч фото
// (или мёртвый сетевой том) не держит startup, Undo и rescan. Отчёт прежнего полного пересчёта
// отбрасывается; событие `missing` — под emitMu, как library и scan.
func (a *App) refreshMissing() {
	a.missSyncMu.Lock()
	defer a.missSyncMu.Unlock()
	refs := a.snapRefs(a.editor())
	a.mu.Lock()
	a.missGen++
	gen := a.missGen
	a.missRefs = refs
	a.mu.Unlock()
	a.checkMissing(gen, refs, false)
}

// checkMissing — фоновая проверка фото refs (ID → путь) поколения gen. Итог сливается с уже
// известным списком (mergeMissing). quiet — событие только при смене списка (после правки).
func (a *App) checkMissing(gen int, refs map[string]string, quiet bool) {
	a.missPending.Add(1)
	a.bg.Go(func() {
		defer a.missPending.Add(-1)
		defer guard("missing")
		gone := library.MissingOf(refs, func(p string) string { return p }, a.exists)
		a.emitMu.Lock()
		defer a.emitMu.Unlock()
		a.mu.Lock()
		// Начался полный пересчёт — он проверит и эти фото, его итог свежее.
		if gen != a.missGen || a.stopped.Load() {
			a.mu.Unlock()
			return
		}
		ids := mergeMissing(a.missing, a.missRefs, refs, gone)
		same := slices.Equal(ids, a.missing)
		if !same {
			a.missing = ids // новый срез: прежний уже мог уйти в GetState или в событие
		}
		a.mu.Unlock()
		if !quiet || !same {
			a.emit("missing", ids)
		}
	})
}

// mergeMissing — новый список недостающих по порядку ID: из прежнего cur уходят фото, которых
// нет в альбоме (now), и проверенные заново; из проверенных (checked, пропали gone) входят те,
// чей путь в альбоме тот же, что при проверке. Фото, убранное или переехавшее, пока шла проверка,
// её итогом не возвращается: его путь сверит своя проверка.
func mergeMissing(cur []string, now, checked map[string]string, gone []string) []string {
	out := make([]string, 0, len(cur)+len(gone))
	for _, id := range cur {
		p, ok := now[id]
		if !ok {
			continue
		}
		if q, ok := checked[id]; ok && q == p {
			continue // решает свежая проверка ниже
		}
		out = append(out, id)
	}
	for _, id := range gone {
		if p, ok := now[id]; ok && p == checked[id] {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// editorPaths — ID → путь фото альбома: по нему правка видит, сменился ли набор фото.
func editorPaths(ed *album.Editor) map[string]string {
	refs := ed.Refs()
	m := make(map[string]string, len(refs))
	for id, r := range refs {
		m[id] = r.Path
	}
	return m
}

// syncMissing — после правки (changed): набор фото альбома мог смениться. Убывшие снимаются
// сразу без os.Stat; в фон уходят только новые и переехавшие фото, итог сливается с известными.
// Набор прежний — ничего, и событие `missing` не шлётся. Снимок и запись — под missSyncMu:
// при параллельных правках снимок, прочитанный раньше, раньше и ложится.
func (a *App) syncMissing(rev int) {
	defer guard("syncMissing")
	a.missSyncMu.Lock()
	defer a.missSyncMu.Unlock()
	ed := a.editor()
	wed := weak.Make(ed)
	if a.missEd == wed && a.missRev == rev {
		return // холостая правка или тот же rev: refs не копируем
	}
	a.missEd, a.missRev = wed, rev
	refs := a.snapRefs(ed)
	gen, fresh := a.dropMissing(refs)
	if fresh != nil {
		a.checkMissing(gen, fresh, true)
	}
}

// dropMissing кладёт набор фото refs в missRefs и сразу снимает убывшие из списка; отдаёт
// поколение и новые или переехавшие фото (nil — таких нет). emitMu — снаружи mu, как в
// checkMissing: событие не обгонит отчёт фоновой проверки.
func (a *App) dropMissing(refs map[string]string) (gen int, fresh map[string]string) {
	a.emitMu.Lock()
	defer a.emitMu.Unlock()
	a.mu.Lock()
	for id, p := range refs {
		if q, ok := a.missRefs[id]; !ok || q != p {
			if fresh == nil {
				fresh = map[string]string{}
			}
			fresh[id] = p
		}
	}
	gen = a.missGen
	if fresh == nil && len(refs) == len(a.missRefs) {
		a.mu.Unlock()
		return gen, nil
	}
	a.missRefs = refs
	kept := make([]string, 0, len(a.missing))
	for _, id := range a.missing {
		if _, ok := refs[id]; ok {
			kept = append(kept, id)
		}
	}
	same := len(kept) == len(a.missing)
	if !same {
		a.missing = kept // новый срез: прежний уже мог уйти в GetState или в событие
	}
	a.mu.Unlock()
	if !same {
		a.emit("missing", kept)
	}
	return gen, fresh
}

// MissingNames — имена файлов недостающих фото (ID → имя): пропавшего файла нет в ленте
// сканирования, и интерфейсу имя для списка «Показать» больше взять негде. Без os.Stat. Своего
// (Lib) нет: это плитка библиотеки, у него пустой путь или имя файла-ключа, а не имя фото.
func (a *App) MissingNames() map[string]string {
	refs := a.editor().Refs() // до a.mu: замки редактора и App не вкладываются
	a.mu.Lock()
	defer a.mu.Unlock()
	m := make(map[string]string, len(a.missing))
	for _, id := range a.missing {
		if refs[id].Lib != "" {
			continue
		}
		if p, ok := a.missRefs[id]; ok {
			m[id] = filepath.Base(p)
		}
	}
	return m
}

// SuggestDrive предлагает смену буквы диска: пропавшие фото на самой частой
// пропавшей букве ищутся на остальных существующих дисках. Только чтение.
func (a *App) SuggestDrive() DriveHint {
	defer guard("SuggestDrive")
	refs := a.editor().Refs()
	var gone []string
	for _, id := range missingOf(refs, a.exists) {
		gone = append(gone, refs[id].Path)
	}
	// Буквы дисков тоже через Reachable: отключённая сетевая буква иначе держит подсказку.
	return DriveHint(library.SuggestDrive(gone, func(p string) bool {
		if !reachable(p) {
			return false
		}
		_, err := os.Stat(p)
		return err == nil
	}))
}

// RelinkDrive меняет букву диска у недостающих фото и папок, если файлы нашлись
// на новой. Буквы A–Z (допускается «E:»); ошибка — только для неверных букв.
func (a *App) RelinkDrive(from, to string) (model.View, error) {
	ed := a.editor()
	f, ok1 := paths.DriveLetter(from)
	t, ok2 := paths.DriveLetter(to)
	if !ok1 || !ok2 || f == t {
		return ed.View(), fmt.Errorf("нужны две разные буквы диска")
	}
	v := a.changed(ed.RelinkDrive(f, t))
	a.rescan()
	return v, nil
}

// RelinkFolder спрашивает папку и ищет в ней недостающие фото.
func (a *App) RelinkFolder() (RelinkResult, error) {
	dir, err := wr.OpenDirectoryDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Где лежат недостающие фото",
		DefaultDirectory: a.dialogDir(prefs.DirPhotos),
	})
	if err != nil || dir == "" {
		return RelinkResult{View: a.editor().View()}, err
	}
	res, err := a.RelinkFolderPath(dir)
	if err == nil {
		a.rememberDir(prefs.DirPhotos, filepath.Dir(dir))
	}
	return res, err
}

// RelinkFolderPath ищет недостающие фото в папке dir (локальной) по имени и
// размеру файла и перепривязывает найденные. Не нашлось — проект не меняется.
func (a *App) RelinkFolderPath(dir string) (res RelinkResult, rerr error) {
	defer guardErr("RelinkFolderPath", &rerr)
	ed := a.editor()
	res = RelinkResult{View: ed.View()}
	if err := paths.CheckLocalPath(dir); err != nil {
		return res, err
	}
	dir = filepath.Clean(dir)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return res, fmt.Errorf("не папка: %s", dir)
	}
	refs := ed.Refs()
	ids := missingOf(refs, a.exists) // синхронно: ответ нужен сразу, вызов — от действия пользователя
	if len(ids) == 0 {
		return res, nil
	}
	want := make([]library.Want, len(ids))
	for i, id := range ids {
		r := refs[id]
		want[i] = library.Want{ID: id, Old: r.Path, Size: r.Size, W: r.W, H: r.H}
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	found, err := library.FindByName(ctx, dir, want, 0)
	if err != nil {
		return res, err
	}
	res.Found, res.Ambiguous = len(found.Paths), len(found.Ambiguous)
	res.Missing = len(ids) - res.Found
	if res.Found > 0 {
		res.View = a.changed(ed.Relink(found.Paths, dir))
		a.rescan()
	}
	return res, nil
}

// ShowPhotoInExplorer показывает файл фото альбома в Проводнике. Путь берётся из
// проекта по ID, от интерфейса не принимается (как ShowInExplorer). Файла нет —
// открывается его папка; нет и её — ошибка.
func (a *App) ShowPhotoInExplorer(id string) error {
	r, ok := a.editor().Ref(id)
	if !ok {
		return errors.New("фото нет в альбоме")
	}
	path := filepath.Clean(r.Path)
	if err := paths.CheckLocalPath(path); err != nil {
		return errors.New("путь к фото недоступен")
	}
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		return revealPath(path, true)
	}
	if dir := filepath.Dir(path); isDir(dir) {
		return revealPath(dir, false)
	}
	return errors.New("файл не найден")
}

// revealPath — запуск файлового менеджера; в тестах подменяется, чтобы не стартовать процесс.
// file — выделить файл в его папке, иначе открыть папку. Ждать завершения не нужно.
var revealPath = osx.Reveal

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
