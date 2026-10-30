package app

// Выгрузка альбома по просьбе агента (152, design §3): вопрос с полным путём (класс Q, «Всегда» нет) и та же
// выгрузка, что «Файл ▸ Экспорт…», только для альбома вопроса и его ревизии. Путь агент не получает и не
// присылает: только имя папки или подсказку; пишем в папку, которую человек увидел целиком или выбрал в окне ОС.

import (
	"errors"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/app/exporter"
	"github.com/IliaSA101/sosninaalbums/internal/export/printshop"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// askKindExport — вид вопроса «Выгрузить альбом» (AgentAsk.Kind).
const askKindExport = "export"

// errExportMoved — папку подменили между вопросом и щелчком (ссылка, другой том): выгрузки нет.
var errExportMoved = errors.New("папка выгрузки изменилась")

// errShareBusy — как у Export: идёт выгрузка по ссылке.
var errShareBusy = errors.New("Сейчас идёт выгрузка по ссылке — дождитесь её конца") //nolint:revive,staticcheck // текст уходит пользователю тостом как есть

// ExportStarted — итог AgentAnswerExport: выгрузка началась, сколько файлов, план типографии для окна
// «Готово для …» (nil — обычная выгрузка).
type ExportStarted struct {
	Started bool            `json:"started"`
	Files   int             `json:"files"`
	Print   *printshop.Plan `json:"print"`
}

// exportAsk — что вопрос выгрузки хранит до щелчка: настройки и имя книги, показанную папку и её файлы.
type exportAsk struct {
	o     exporter.Options
	name  string // Job.Name: имя от агента ("" — по файлу альбома)
	files int
	print string // Kind типографии ("" — обычная)
	plan  exportPlan
	sum   exporter.Summary // для повторной сверки папки перед записью
}

// exportPicked — что вернул выбор показанной папки (exportPickFn): с ним сверяется Recheck. set = false —
// папку выбирало окно ОС (человек видел её только что), сверять нечего.
type exportPicked struct {
	w   writeDir
	set bool
}

// exportOptions — настройки окна «Экспорт» из входа агента. «Как открывается окно» (print пусто) — типография
// обложки, но только если агент не просил то, что типография игнорирует (диапазон, страницы, только обложку, dpi)
// или своё качество и обложку: тогда обычная выгрузка (0.35-152-23).
func exportOptions(req agents.AskReq, def string) exporter.Options {
	kind := req.Print
	switch {
	case kind == "plain":
		kind = ""
	case kind == "" && req.Range == "" && !req.Pages && !req.CoverOnly && req.DPI == 0 &&
		req.Quality == 0 && !req.Cover:
		kind = def
	}
	return exporter.Options{Range: req.Range, Quality: req.Quality, DPI: req.DPI, Pages: req.Pages,
		Cover: req.Cover, CoverOnly: req.CoverOnly, Print: kind}
}

// exportCheckErr — отказ проверок выгрузки словами окна «Экспорт», но без путей и имён файлов: имена пропавших
// фото и чужих файлов с номерами заменены общей фразой (человек видит их в программе).
func exportCheckErr(err error) error {
	switch {
	case errors.Is(err, exporter.ErrAlbumEdited):
		return agents.ErrAlbumEdited
	case errors.Is(err, errAgentAlbumChanged):
		return agents.ErrAlbumChanged
	}
	msg := paths.NoPath(err).Error()
	switch {
	case strings.Contains(msg, "не найдены файлы фото"), strings.Contains(msg, "нет в библиотеке этого компьютера"):
		msg = "часть фото альбома не найдена на этом компьютере — человек видит подробности в программе"
	case strings.Contains(msg, "файлы с номерами разворотов"):
		msg = "в папке чужие файлы с номерами разворотов — человек выберет другую папку"
	case strings.Contains(msg, "смешаются с фото"): // отказ 121 называет папки человека по именам
		msg = "папка внутри фото альбома — человек выберет другую"
	case strings.Contains(msg, "не удалось прочитать папку"):
		msg = "не удалось прочитать папку выгрузки"
	case strings.Contains(msg, "не удалось создать"):
		msg = "не удалось создать папку выгрузки"
	case strings.Contains(msg, "уже идёт"):
		return agents.ErrExporting
	}
	if r := []rune(msg); len(r) > 300 || !utf8.ValidString(msg) || paths.BadRunes(msg) {
		msg = "выгрузка не началась — причину видит человек в программе"
	}
	return agents.ExportFail(msg)
}

// askExportAlbum — вопрос «Выгрузить альбом» (Host.Ask, вид export): проверки без окна, папка, текст.
// rec == nil — вопроса нет, res и err — итог вызывающему.
func (a *App) askExportAlbum(req agents.AskReq) (rec *askRec, res agents.AskRes, err error) {
	if exportRunning(a) {
		return nil, res, agents.ErrExporting
	}
	if a.shr.Busy() {
		return nil, res, agents.ErrBusy
	}
	ed, err := a.askTarget(req)
	if err != nil {
		return nil, res, err
	}
	if v := ed.View(); req.CoverOnly && (v.Cover == nil || v.Settings.Sheet) { // окно человека молча выгрузит развороты; агенту — отказ
		return nil, res, agents.ErrNoCover
	}
	o := exportOptions(req, a.exp.DefaultPrint(ed))
	nm := exportNamesOf(req.Hint, ed)
	sm, err := a.exp.Check(ed, o, nm.job)
	if err != nil {
		return nil, res, exportCheckErr(err)
	}
	if ed.View().Rev != req.Rev { // правили, пока считали
		return nil, res, agents.ErrAlbumEdited
	}
	plan := a.planExport(a.askM().rules(), ed, sm, req.Hint, nm)
	body := exportAskText(exportAskIn{Client: req.Client, Album: a.askAlbumName(ed), Lines: sm.Lines, Plan: plan})
	ex := &exportAsk{o: o, name: nm.job, files: sm.Files, print: sm.Print, plan: plan, sum: sm}
	fp := folderPlan{path: plan.shown}
	if plan.badHint {
		fp.why = whyBad
	}
	return a.askPostWith(req, ed, fp, body, ex)
}

// exportPickFn — Job.Pick для щелчка: показанная папка (после повтора правила) или окно ОС. Повтор ловит подмену
// между вопросом и щелчком: ссылка, другой том, папка стала файлом.
func (a *App) exportPickFn(rec *askRec, other bool, got *exportPicked) func(title string) (string, bool, []string, error) {
	ex := rec.exp
	return func(title string) (string, bool, []string, error) {
		if ex.plan.approved() && !other {
			w, err := a.askM().rules().exportPick(ex.plan.pick)
			if err != nil {
				return "", false, nil, errExportMoved
			}
			hits := ex.plan.ours
			if w.create != ex.plan.pickNew { // папка появилась (или исчезла) с вопроса: набор файлов неизвестен
				hits = nil
			}
			*got = exportPicked{w: w, set: true}
			return w.path, w.create, hits, nil
		}
		dir, err := pickExportDir(a, title+" — просит "+clientName(rec.req.Client))
		return dir, false, nil, err
	}
}

// exportRecheck — Job.Recheck: после последнего окна, прямо перед созданием папок и записью, показанная папка
// судится ещё раз (подмена ссылкой за время вопросов, 0.35-152-25), в существующей не должно быть чужих файлов
// с номерами. Расхождение — errExportMoved. dir — папка выгрузки (книги), got — что дал выбор.
func (a *App) exportRecheck(rec *askRec, got *exportPicked) func(dir string) error {
	ex := rec.exp
	return func(dir string) error {
		if !got.set {
			return nil
		}
		r := a.askM().rules()
		w, err := r.exportPick(ex.plan.pick)
		if err != nil || w.path != got.w.path || w.create != got.w.create {
			return errExportMoved
		}
		if _, foreign, known := hitsIn(r, ex.sum, dir); known && foreign {
			return errExportMoved
		}
		return nil
	}
}

// AgentAnswerExport — «Выгрузить» / «Выбрать папку…» в вопросе «Выгрузить альбом»: показанная папка
// (other = false) или папка из окна ОС (other, «Выбрать другую…»). Дальше всё как у «Файл ▸ Экспорт…»:
// предупреждения, полоса хода, «Готово». Отказ идёт через AnswerAgent(id, false). Альбом сменился или
// изменился после вопроса — выгрузки нет, агенту «перечитайте и спросите снова».
func (a *App) AgentAnswerExport(id string, other bool) (st ExportStarted, err error) {
	defer guardErr("AgentAnswerExport", &err)
	rec, err := a.askClaim(id, askKindExport, true)
	if err != nil || rec == nil {
		return st, err
	}
	defer close(rec.idle)
	if !a.askBegin(rec) {
		return st, nil
	}
	if a.shr.Busy() {
		a.askFail(rec, agents.ErrBusy)
		return st, errShareBusy
	}
	if cur, cerr := a.agentEditor(rec.req.Key); cerr != nil || cur != rec.ed {
		a.notice("Открыт другой альбом — выгрузка не начата. Агент спросит снова.")
		a.askFail(rec, agents.ErrAlbumChanged)
		return st, nil
	}
	ex := rec.exp
	var got exportPicked
	started, err := a.exp.ExportWith(exporter.Job{Ed: rec.ed, Rev: rec.req.Rev, Opt: ex.o, Name: ex.name,
		Pick: a.exportPickFn(rec, other, &got), Recheck: a.exportRecheck(rec, &got)})
	switch {
	case errors.Is(err, errExportMoved): // агенту — как «папка не подошла», без пути; человеку — своя фраза
		a.notice("Папка выгрузки изменилась — выгрузка не начата. Выберите папку снова.")
		a.askFinish(rec, askOutcome{res: agents.AskRes{Why: agents.WhyTooWide}}, agents.WhyTooWide)
		return st, nil
	case errors.Is(err, exporter.ErrAlbumEdited):
		a.notice("Альбом изменился, пока вы отвечали — выгрузка не начата. Агент спросит снова.")
		a.askFail(rec, agents.ErrAlbumEdited)
		return st, nil
	case err != nil:
		log.Printf("выгрузка по просьбе агента не началась: %v", paths.NoPath(err))
		a.askFail(rec, exportCheckErr(err))
		return st, err
	case !started: // «нет» в предупреждении, отмена окна ОС: человек принял просьбу и остановил её — не отказ
		why := agents.WhyStopped
		a.askFinish(rec, askOutcome{res: agents.AskRes{Why: why}}, why)
		return st, nil
	}
	res := agents.AskRes{Why: agents.WhyAnswered, Files: ex.files,
		Album: agents.AlbumRef{Key: a.askKey(rec.ed), Rev: rec.ed.View().Rev}}
	a.askFinish(rec, askOutcome{res: res}, agents.WhyAnswered)
	return ExportStarted{Started: true, Files: ex.files, Print: a.exportPlanOf(rec.ed, ex.print)}, nil
}

// exportPlanOf — план типографии kind для окна «Готово для …»; "" — обычная выгрузка.
func (a *App) exportPlanOf(ed *album.Editor, kind string) *printshop.Plan {
	if kind == "" {
		return nil
	}
	for _, pl := range a.exp.PrintPlansFor(ed, "") {
		if pl.Kind == kind {
			return &pl
		}
	}
	return nil
}

// agentExportHold — Host.ExportHold: выгрузка (окна и фон) держит создание и открытие альбома — сразу «идёт
// выгрузка», без ожидания «человек занят», которое фронт поднимает на время фона (0.35-152-27). Остальные
// инструменты, в том числе правки текущего альбома, выгрузку не держит: она уже взяла снимок.
func (a *App) agentExportHold(tool string) bool {
	return (tool == "create_album" || tool == "open_album") && a.openBusy()
}

// agentExportState — Host.ExportState: ход последней выгрузки альбома edID; чужого альбома — idle.
func (a *App) agentExportState(edID uint64) agents.ExportStatus {
	st := a.exp.Last()
	if st.AlbumID != edID || edID == 0 {
		return agents.ExportStatus{State: exporter.StateIdle}
	}
	return agents.ExportStatus{State: st.State, Done: st.Done, Total: st.Total, Error: st.Error}
}
