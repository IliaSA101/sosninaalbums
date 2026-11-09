package app

// Тихое открытие альбома по ключу при «Всегда» (160, design §2): применение плана agentopen_plan.go. Образец —
// quietCreate (agentquiet_apply.go): разрешение и документ перечитываются, прежний альбом закрепляется
// (pinLeaving), смена — retireIf под saveMu: альбом, который человек открыл за время проверки, не уводим.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// quietAllowed — шов тестов: наблюдатель решений allowed (диск трогается ровно там, где он ответил «да»).
var quietAllowed = func(f func(string) bool) func(string) bool { return f }

// openQuiet открывает альбом плана без вопроса. Итог как у quietApply: res/err — ответ службе; why != "" —
// тихо нельзя, задать обычный вопрос с этой причиной (quietAsk — «Спрашивать»: вопрос без причины).
func (a *App) openQuiet(req agents.AskReq, pl *openPlan) (res agents.AskRes, why string, err error) {
	mode, blocked := a.agentPerm(toolOpenAlbum)
	if blocked != "" || mode == prefs.PermDeny {
		return agents.AskRes{Why: agents.WhyOff}, "", nil
	}
	if mode != prefs.PermAllow {
		return res, quietAsk, nil // человек снял «Всегда», пока шла проверка
	}
	if a.openBusy() {
		return res, "", agents.ErrExporting
	}
	cur := pl.cur
	if a.editor() != cur { // пока шла проверка, человек открыл другой альбом
		return res, "", agents.ErrAlbumChanged
	}
	if !a.openDocQuiet(cur) {
		return res, whyDraft, nil
	}
	if w := a.openRecheck(pl); w != "" {
		return res, w, nil
	}
	// Полный разбор — из тех же байтов и только по проверенным папкам (вне них диск отвечает «нет»).
	// Под общим сроком и числом обращений: зависший том (ссылка на NFS) не держит вызов, а сотни тысяч записей — процессор.
	bud := salbum.NewBudget(pl.until, openMaxOps)
	disk := quietDisk(salbum.Bounded(quietAllowed(pl.allowed), bud))
	type decoded struct {
		p   *model.Project
		err error
	}
	dec, done := appkit.Within(max(time.Until(pl.until), time.Millisecond), func() decoded {
		p, err := salbum.DecodeDisk(pl.data, pl.base, disk)
		return decoded{p, err}
	})
	switch {
	case !done || bud.Exceeded() || pl.expired():
		return res, whyBig, nil
	case dec.err != nil:
		return res, whyFile, nil
	}
	full := dec.p
	if w := a.openFullWhy(pl, full); w != "" {
		return res, w, nil
	}
	lv, perr := a.pinLeaving(cur)
	if perr != nil {
		return res, "", agents.ErrBackup
	}
	defer lv.release()
	// Шрифты папки альбома тихо не ставятся (needFolderFonts уже ответил): af пуст, albumFolderFonts не зовём.
	_, changed, oerr := a.openParsed(pl.path, full, albumFonts{}, cur)
	switch {
	case oerr != nil:
		return res, whyFile, nil
	case changed:
		return res, "", agents.ErrAlbumChanged
	}
	prev := lv.prev
	if prev != "" && paths.SamePath(prev, pl.path) {
		prev = ""
	}
	a.setPrev(prev)
	ed := a.editor()
	res = agents.AskRes{Why: agents.WhyQuiet, Album: agents.AlbumRef{Key: a.askKey(ed), Rev: ed.View().Rev}}
	text := "Открыт альбом"
	if name := openName(pl.path); name != "" {
		text += " «" + name + "»"
	}
	// Rev -1: «Вернуть» не предлагается, открытие не отменяется Ctrl+Z; вернуть прежний — «Открыть прежний».
	a.agentEdited(AgentEdited{Client: agentToastClient(req.Client), Text: text, Rev: -1, Prev: prev != "", Quiet: true})
	return res, "", nil
}

// openRecheck — повтор дешёвых проверок перед сменой: файл тот же (Lstat, os.SameFile) и не ссылка, пути
// к проверенным папкам без ссылок (единицы мс). Подмена между планом и применением — file/link.
func (a *App) openRecheck(pl *openPlan) string {
	link := quietLink
	type res struct {
		same, linked bool
		bad          string
	}
	got, done := appkit.Within(pl.left(), func() res {
		st, err := os.Lstat(pl.path)
		out := res{same: err == nil && os.SameFile(st, pl.lst), linked: link(pl.path)}
		for _, d := range pl.dirs {
			if link(d) {
				out.bad = whyLink
				break
			}
		}
		return out
	})
	switch {
	case !done:
		return whyBig
	case !got.same || got.linked:
		return whyFile
	}
	return got.bad
}

// openFullWhy — строки полного разбора ⊆ проверенного: каждая была в сухом перечне, а её папка (или она сама —
// у папки альбома) проверена; свои фоны и стикеры — из библиотеки программы. Иначе place: вопрос задаётся
// тем же путём, что из плана (У101).
func (a *App) openFullWhy(pl *openPlan, full *model.Project) string {
	known := func(s string) bool {
		if !quietPlain(s) {
			return false // «..» после ссылки, хвост «пробел/точка»: строки вне сухого перечня проверяются так же строго (Н1, М5)
		}
		if pl.cfgDir != "" && strings.HasPrefix(s, pl.cfgDir) {
			return true
		}
		return pl.rows[s] && (pl.checked[s] || pl.checked[filepath.Dir(s)])
	}
	why := ""
	_ = salbum.EachPath(full, func(_, p, alt string) error {
		for _, s := range []string{p, alt} {
			if s != "" && !known(s) {
				why = whyPlace
				return errStopRows
			}
		}
		return nil
	})
	return why
}

// errStopRows прерывает обход строк EachPath: причина уже найдена.
var errStopRows = errors.New("стоп")
