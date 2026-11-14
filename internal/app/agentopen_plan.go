package app

// План тихого открытия альбома по ключу при «Всегда» (160, design §2): файл из «Последних» читается один раз,
// «сухой» разбор (без диска) называет все пути альбома, каждый путь обязан быть в местах съёмок, а диск
// трогается только в проверенных папках и родителях строк. Само открытие — agentopen_quiet.go.

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/app/agentasktext"
	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Причины вопроса «Открыть альбом» (AgentAsk.Why, 0.36-160): тексты — во фронте, агенту причина не видна.
const (
	whyFile  = "file"  // файл альбома не годится без человека: ссылка, не обычный файл, копия, новее программы, битый
	whyStub  = "stub"  // файл альбома — облачная заглушка
	whyGone  = "gone"  // папки или родителя фото нет на месте
	whyFonts = "fonts" // альбому нужны шрифты из его папки: ставит только человек
)

// Пределы плана: корней (папок и запасных) и родителей строк. Больше — big, до обращения к диску.
const (
	openMaxRoots   = 50
	openMaxParents = 500
	openFileBudget = 3 * time.Second // от проверки файла до решения: медленный диск — big
	// openMaxOps — обращений к диску в полном разборе. Фото в хорошем альбоме — десятки тысяч, запас ×4; больше — big.
	openMaxOps = 200000
)

// Подмены для тестов: срок, открытие файла, сухой разбор и диск полного разбора (стражи считают обращения).
var (
	quietOpenBudget = openFileBudget
	// quietOpenMaxPhotos — сумма фото всех папок открываемого альбома. Свадебный альбом — тысячи фото; 50 000 — с запасом ×5.
	// Сверх — big: лента на сотни тысяч файлов, сканер и миниатюры без человека не нужны (аудит М3).
	quietOpenMaxPhotos = 50000
	quietOpenFile      = openAlbumFile
	quietDry           = salbum.DecodeDry
	quietDisk          = func(d salbum.Disk) salbum.Disk { return d }
	quietTrail         = paths.TrailingDot // подмена в тестах: правило Windows на любой ОС
)

// quietPlain — строка пути годится для тихого открытия: канонична (Clean) и без элемента с точкой или пробелом
// на конце (М5): Win32 срезает их, и текст проверки мест расходится с путём на диске. Единственное место
// правила для allowed, openFullWhy и отказа сухого перечня (agentopen_plan.go).
func quietPlain(s string) bool { return filepath.Clean(s) == s && !quietTrail(s) }

// openPlan — итог проверки: why != "" — тихо нельзя; иначе всё, что нужно openQuiet.
type openPlan struct {
	why     string
	note    string // строка вопроса к причине (why = disk: «Место … — на диске …»); "" — нет
	cur     *album.Editor
	path    string
	base    string
	data    []byte      // файл, прочитанный один раз: полный разбор — из этих же байтов
	lst     fs.FileInfo // Lstat перед чтением: openQuiet сверяет файл os.SameFile
	rules   *agentplaces.Rules
	checked map[string]bool // проверенные папки (корни и родители строк) — по точной строке: регистр свёртывается только при сверке с местами (аудит М2)
	dirs    []string        // они же, в порядке проверки (повтор перед сменой)
	rows    map[string]bool // пути сухого разбора (основные и запасные), точные строки
	cfgDir  string          // папка настроек со слэшем на конце: там библиотека своих фонов и стикеров
	photos  int             // сумма фото в проверенных корнях
	until   time.Time       // срок решения
}

// left — сколько осталось на одну проверку диска: не больше предела правил и общего срока.
func (pl *openPlan) left() time.Duration {
	return max(min(pl.rules.Timeout(), time.Until(pl.until)), time.Millisecond)
}

// expired — общий срок вышел.
func (pl *openPlan) expired() bool { return !time.Now().Before(pl.until) }

// allowed — путь можно трогать на диске при полном разборе: сама проверенная папка, файл прямо в ней или
// библиотека программы. Всё прочее для разбора — «файла нет».
func (pl *openPlan) allowed(path string) bool {
	// Только канонический путь: «..» после ссылки на диске ведёт не туда, куда читается по тексту (аудит Н1);
	// записи без разворота и свёрнутые FoldSources в сухом перечне нет, поэтому проверка и здесь.
	if path == "" || !quietPlain(path) || paths.CheckLocalPath(path) != nil {
		return false
	}
	if pl.checked[path] || pl.checked[filepath.Dir(path)] {
		return true
	}
	return pl.cfgDir != "" && strings.HasPrefix(path, pl.cfgDir)
}

// openPlan проверяет открытие по ключу при «Всегда». path — файл из «Последних» (recentCheck прошёл). Первая
// сработавшая причина — ответ; «rate» — последней.
func (a *App) openPlan(req agents.AskReq, cur *album.Editor, path string) (pl *openPlan) {
	pl = &openPlan{cur: cur, path: path, rules: a.askM().rules()}
	fail := func(why string) *openPlan { pl.why = why; return pl }
	r := pl.rules
	if !a.openDocQuiet(cur) {
		return fail(whyDraft)
	}
	pl.until = time.Now().Add(quietOpenBudget)
	if why := a.openFileWhy(pl); why != "" {
		return fail(why)
	}
	dry, err := quietDry(pl.data, pl.base)
	if err != nil {
		return fail(whyFile)
	}
	// Все строки, основные и запасные, — в местах: до любого обращения к диску.
	places := a.usablePlaces()
	dirs := make([]string, 0, len(places))
	for _, place := range places {
		dirs = append(dirs, place.Dir)
	}
	var parents []string
	pl.rows = map[string]bool{}
	outside := false
	_ = salbum.EachPath(dry, func(what, p, alt string) error {
		for _, s := range []string{p, alt} {
			if s == "" {
				continue
			}
			// Путь не канонический (".." после папки-ссылки) или с хвостом «пробел/точка» (Windows): текст проверки
			// мест и путь на диске разошлись бы (аудит Б1, М5). Такие строки тихо не принимаем — отказ place.
			if !quietPlain(s) {
				outside = true
			}
			pl.rows[s] = true
			if what != salbum.WhatFolder { // родитель файла проверяется; папки — корни
				parents = append(parents, filepath.Dir(s))
			}
			outside = outside || len(dirs) == 0 || !agentplaces.InAny(r, s, dirs)
		}
		return nil
	})
	if outside {
		return fail(whyPlace)
	}
	roots := append(append([]string{}, dry.Folders...), altsOf(dry)...)
	if why := a.openDirsWhy(pl, req, dry, uniqueDirs(roots), uniqueDirs(parents)); why != "" {
		return fail(why)
	}
	if pl.expired() {
		return fail(whyBig)
	}
	if a.needFolderFonts(dry, path) {
		return fail(whyFonts)
	}
	if pl.expired() {
		return fail(whyBig)
	}
	if !req.Always {
		return fail(whyRate)
	}
	return pl
}

// altsOf — запасные папки альбома (FolderAlt: ключ и значение).
func altsOf(p *model.Project) (out []string) {
	for dir, alt := range p.FolderAlt {
		out = append(out, dir, alt)
	}
	return out
}

// uniqueDirs — папки без повторов по точной строке, порядок сохранён. Регистр не сворачивается: на
// регистрозависимом томе «a» и «A» — разные папки, и одна из них может быть ссылкой (аудит М2).
func uniqueDirs(list []string) (out []string) {
	seen := map[string]bool{}
	for _, d := range list {
		if d != "" && !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out
}

// openDocQuiet — открытый альбом можно тихо сменить: сохранён, пуст или нетронутый черновик агента. Правленый
// черновик и файл с несохранёнными правками — вопрос (draft), строже create_album (0.36-160-12).
func (a *App) openDocQuiet(cur *album.Editor) bool {
	switch a.askDocOf(cur) {
	case agentasktext.DocSaved, agentasktext.DocEmpty:
		return true
	case agentasktext.DocDraft:
		return a.draftUntouched(cur)
	}
	return false
}

// openFileWhy — файл альбома: сеть и доступность до Lstat, обычный файл без ссылок, не заглушка; затем одно чтение
// из дескриптора, который прошёл те же проверки (O_NOFOLLOW|O_NONBLOCK, os.SameFile с Lstat). Файл не обязан
// лежать в местах (0.36-160-3). Заполняет pl.base, pl.lst, pl.data.
func (a *App) openFileWhy(pl *openPlan) string {
	path, r := pl.path, pl.rules
	if a.backups.Inside(path) || !strings.HasSuffix(strings.ToLower(path), projectExt) {
		return whyFile
	}
	// Сеть и доступность — до Lstat: мёртвый SMB не держит окно, а удалённый том не читаем без человека.
	reach := reachable
	switch up, done := appkit.Within(pl.left(), func() bool { return reach(path) }); {
	case !done:
		return whyBig
	case !up || quietRemote(path):
		return whyRemote
	}
	type lres struct {
		st       fs.FileInfo
		err      error
		resolved string
		linked   bool
	}
	link := quietLink
	res, done := appkit.Within(pl.left(), func() lres {
		st, err := os.Lstat(path)
		out := lres{st: st, err: err}
		if err == nil {
			out.linked = link(path)
			out.resolved, _ = filepath.EvalSymlinks(path)
		}
		return out
	})
	switch {
	case !done:
		return whyBig
	case res.err != nil || !res.st.Mode().IsRegular() || res.linked:
		return whyFile
	case res.resolved != "" && r.VolumeMoved(path, res.resolved):
		return whyFile
	case quietCloud(fs.FileInfoToDirEntry(res.st)):
		return whyStub
	}
	pl.lst = res.st
	if abs, err := filepath.Abs(filepath.Dir(path)); err == nil {
		pl.base = abs
	} else {
		return whyFile
	}
	data, why := readOpened(path, res.st)
	if why != "" {
		return why
	}
	if pl.expired() {
		return whyBig
	}
	pl.data = data
	return ""
}

// readOpened читает файл альбома один раз: дескриптор обязан быть обычным файлом того же объекта, что Lstat.
// Подмена ссылкой или FIFO между Lstat и открытием — file, без зависания (O_NONBLOCK).
func readOpened(path string, lst fs.FileInfo) ([]byte, string) {
	f, err := quietOpenFile(path)
	if err != nil {
		return nil, whyFile
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	fst, err := f.Stat()
	if err != nil || !fst.Mode().IsRegular() || !os.SameFile(fst, lst) {
		return nil, whyFile
	}
	data, err := salbum.ReadLimitedFrom(f)
	if err != nil {
		return nil, whyFile
	}
	return data, ""
}

// openDirsWhy проверяет корни (папки альбома и запасные) и родителей строк: корни — как папка агента (место, диск,
// сеть, заглушки, ссылки), родитель внутри корня — звенья-ссылки от корня, вне корней — как корень. Срок общий:
// просрочка — big. Заполняет pl.checked/dirs.
func (a *App) openDirsWhy(pl *openPlan, req agents.AskReq, dry *model.Project, roots, parents []string) string {
	r := pl.rules
	if len(roots) > openMaxRoots || len(parents) > openMaxParents {
		return whyBig
	}
	pl.checked = map[string]bool{}
	pl.cfgDir = ""
	if a.cfgDir != "" {
		pl.cfgDir = filepath.Clean(a.cfgDir) + string(filepath.Separator)
	}
	under := func(p string) bool {
		for _, root := range roots {
			if r.Inside(p, root) {
				return true
			}
		}
		return false
	}
	ask := agents.AskReq{Kind: agents.AskOpen, Always: true, Client: req.Client}
	for _, root := range roots {
		if pl.expired() {
			return whyBig
		}
		if why := a.rootWhy(pl, ask, root); why != "" {
			return why
		}
	}
	if pl.photos > quietOpenMaxPhotos {
		return whyBig
	}
	for _, par := range parents {
		if pl.expired() {
			return whyBig
		}
		if under(par) {
			if why := parentWhy(pl, par); why != "" {
				return why
			}
			continue
		}
		if why := a.rootWhy(pl, ask, par); why != "" { // вне корней: проверяется целиком, как корень (строже design §2 п. 8)
			return why
		}
	}
	if pl.photos > quietOpenMaxPhotos {
		return whyBig
	}
	return ""
}

// rootWhy — корень dir: правила места, настоящий путь, ссылки, диск и подсчёт (quietWhy, без предела числа фото).
func (a *App) rootWhy(pl *openPlan, ask agents.AskReq, dir string) string {
	r := pl.rules.WithTimeout(pl.left()) // пробы корня не дольше остатка общего срока (аудит М4)
	if _, err := r.Static(dir); err != nil {
		return whyPlace
	}
	cleaned, resolved, err := r.Resolve(dir)
	if err != nil {
		return whyGone
	}
	if pl.rules.Key(resolved) != pl.rules.Key(cleaned) { // папка альбома проходит через ссылку или junction: тихо нельзя
		return whyLink
	}
	why, note := a.quietWhy(r, ask, cleaned, resolved, pl.cur, &pl.photos)
	if why != "" {
		pl.note = note
		return why
	}
	pl.checked[resolved] = true
	pl.checked[dir] = true
	pl.dirs = append(pl.dirs, resolved)
	return ""
}

// parentWhy — родитель строки внутри проверенного корня: он есть, не ссылка и путь к нему без ссылок
// (agentplaces.CountQuiet в ссылку не заходит, а StatLocal пошёл бы).
func parentWhy(pl *openPlan, par string) string {
	link := quietLink
	type res struct {
		st  fs.FileInfo
		err error
		ln  bool
	}
	got, done := appkit.Within(pl.left(), func() res {
		st, err := os.Lstat(par)
		return res{st, err, err == nil && link(par)}
	})
	switch {
	case !done:
		return whyBig
	case got.err == nil && got.st.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0:
		return whyLink // сам родитель — ссылка
	case got.err != nil || !got.st.IsDir():
		return whyGone
	case got.ln:
		return whyLink
	}
	pl.checked[par] = true
	pl.dirs = append(pl.dirs, par)
	return ""
}
