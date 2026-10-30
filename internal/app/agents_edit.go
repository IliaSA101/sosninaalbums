package app

// Правки альбома по просьбе агента (149, Т3): фасад над Editor со скалярными сигнатурами; тонкий
// переходник к agents.Host (Т3b) только переводит типы и ошибки. Правило одно: ed := a.editor()
// берётся один раз, ключ сверяется по нему, правка идёт методами ed со сверкой rev и срока под
// замком Editor (album.Expect), а не через App.* и edit.Methods — те берут редактор заново, и между
// сверкой и записью человек успел бы нажать Ctrl+Z или открыть другой альбом (0.33-149-17, 23).

import (
	"context"
	"errors"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Ошибки фасада: тексты без путей. Переходник Host (agents_host_edit.go) переводит их в ошибки службы одной таблицей.
var (
	errAgentAlbumChanged = errors.New("открыт другой альбом")               // ключ не совпал
	errAgentAlbumEdited  = errors.New("альбом изменился после чтения")      // rev не совпал
	errAgentScanning     = errors.New("лента фото ещё читается")            // не дождались конца чтения
	errAgentNoFree       = errors.New("свободных фото нет")                 // нечего собирать
	errAgentNoSpread     = errors.New("такого листа нет")                   // show_spread
	errAgentLate         = errors.New("срок вышел — правка не применена")   // album.ErrLate
	errAgentBackup       = errors.New("не удалось сделать резервную копию") // правки нет
	errAgentNoAlbum      = errors.New("альбома нет")                        // редактора нет
	errAgentNotReady     = errors.New("программа ещё запускается")          // startup не закончен
	errAgentOff          = errors.New("правки агентам выключены")           // флажок сняли, пока правка ждала
)

// agentEditOpts — сроки фасада; тесты подменяют (миллисекунды, свои часы), как agents.Options.
type agentEditOpts struct {
	BusyWait time.Duration          // сколько ждать снятия «занят», если у ctx нет срока
	BusyTTL  time.Duration          // флаг «занят» жив столько от последнего AgentsBusy(true)
	Poll     time.Duration          // шаг опроса (занят, чтение ленты)
	Now      func() time.Time       // часы флага «занят»
	Key      func(ed uint64) string // ключ альбома по номеру Editor (agents.AlbumKey); nil — все ключи не годны (по умолчанию agents.AlbumKey)
}

// defaultAgentEditOpts — сроки design §2.6; ключ — единственный: agents.AlbumKey (его же отдаёт get_album).
func defaultAgentEditOpts() agentEditOpts {
	return agentEditOpts{BusyWait: 2 * time.Second, BusyTTL: 30 * time.Second, Poll: 25 * time.Millisecond, Now: time.Now,
		Key: agents.AlbumKey}
}

// agentEdit — состояние правок агента одного App: «человек занят» и ключи альбомов, у которых уже
// есть копия за этот запуск.
type agentEdit struct {
	opts agentEditOpts

	mu     sync.Mutex
	busy   bool
	busyAt time.Time

	backMu sync.Mutex // копии идут по одной: две правки подряд не плодят две копии
	backed map[string]bool
}

// agentEdits — состояние по App (App не растёт полем: фасад живёт в новых файлах).
var agentEdits sync.Map // *App → *agentEdit

// agentEd — состояние правок агента этого App.
func (a *App) agentEd() *agentEdit {
	if v, ok := agentEdits.Load(a); ok {
		return v.(*agentEdit)
	}
	v, _ := agentEdits.LoadOrStore(a, &agentEdit{opts: defaultAgentEditOpts(), backed: map[string]bool{}})
	return v.(*agentEdit)
}

// AgentsBusy — интерфейс сообщает, что человек занят (перенос, набор текста, задача, окна
// «Автосборка» и «Настройки альбома»): правка агента подождёт. Сигнал повторяется раз в 10 с,
// флаг жив BusyTTL от последнего true — упавший интерфейс агента не блокирует навсегда.
func (a *App) AgentsBusy(on bool) {
	defer guard("AgentsBusy")
	s := a.agentEd()
	s.mu.Lock()
	s.busy, s.busyAt = on, s.opts.Now()
	s.mu.Unlock()
}

// busyNow — флаг поднят и ещё жив.
func (s *agentEdit) busyNow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.busy && s.opts.Now().Sub(s.busyAt) < s.opts.BusyTTL
}

// agentBusy ждёт снятия «занят» до конца ctx (нет срока — до BusyWait); true — всё ещё занят.
func (a *App) agentBusy(ctx context.Context) bool {
	s := a.agentEd()
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.opts.BusyWait)
		defer cancel()
	}
	for s.busyNow() {
		select {
		case <-ctx.Done():
			return true
		case <-time.After(s.opts.Poll):
		}
	}
	return false
}

// agentEditState — правки ещё возможны: не все обратимые правки, папка и альбом «Запретить» (150), и
// причина, по которой нельзя: процесс от администратора правок не принимает. Решение по каждому
// инструменту — agentPerm; здесь общий вход для вопросов и «между вопросом и записью».
func (a *App) agentEditState() (allowed bool, blocked string) {
	if b := a.editBlocked(); b != "" {
		return false, b
	}
	return a.editGroupOn(), ""
}

// editGroupOn — группа правок не вся «Запретить» (AgentsInfo.Edit, design §6): не зеркало
// agents.edit, которое true только при всех включённых правках, — иначе окно согласия
// повторялось бы на каждой строке таблицы.
func (a *App) editGroupOn() bool {
	p := prefs.LoadAgentPerms(a.cfgDir)
	for _, tool := range prefs.AgentToolNames() {
		switch prefs.AgentToolClass(tool) {
		case prefs.ClassEdit, prefs.ClassFolder, prefs.ClassAlbum:
			if p.Tools[tool] != prefs.PermDeny {
				return true
			}
		}
	}
	return false
}

// agentPerm — Host.Perm: разрешение инструмента из config.json на этот вызов (читается каждый раз) и
// причина «blocked». Инструмента нет в записи — умолчание для нового (AgentPerms.NewMode).
func (a *App) agentPerm(tool string) (mode, blocked string) {
	mode, blocked, _ = a.agentPermLevel(tool)
	return mode, blocked
}

// agentPermLevel — Host.PermLevel: agentPerm и уровень доступа из одного чтения config.json.
func (a *App) agentPermLevel(tool string) (mode, blocked string, level int) {
	cfg := prefs.LoadConfig(a.cfgDir)
	level = prefs.AgentsLevelOf(cfg)
	p := prefs.AgentPermsOf(cfg)
	mode = p.Mode(tool)
	if mode == "" {
		mode = p.NewMode(prefs.AgentToolClass(tool) == prefs.ClassRead)
	}
	return mode, a.editBlocked(), level
}

// agentCheckTool — разрешение самого инструмента перечитывается перед записью: пока правка ждала
// ленту, человек мог поставить ему «Запретить» (спека 150). Вызов без вопроса («Всегда») при смене
// на «Спрашивать» тоже отказ: asked — вопрос по этому вызову уже задан и «да» получено.
func (a *App) agentCheckTool(tool string, asked bool) error {
	mode, b := a.agentPerm(tool)
	if b != "" || mode == prefs.PermDeny || mode == prefs.PermAsk && !asked {
		return errAgentOff
	}
	return nil
}

// agentBlockedElevated — AgentsInfo.EditBlocked: программа запущена от администратора.
const agentBlockedElevated = "elevated"

// agentEditor — редактор, чей ключ назвал агент; один раз на вызов (см. шапку файла).
func (a *App) agentEditor(key string) (*album.Editor, error) {
	select {
	case <-a.ready:
	default:
		return nil, errAgentNotReady
	}
	ed := a.editor()
	if ed == nil {
		return nil, errAgentNoAlbum
	}
	if kf := a.agentEd().opts.Key; kf == nil || key == "" || key != kf(ed.ID()) {
		return nil, errAgentAlbumChanged
	}
	return ed, nil
}

// agentErr переводит ошибки album в ошибки фасада; прочие — как есть.
func agentErr(err error) error {
	switch {
	case errors.Is(err, album.ErrRevChanged):
		return errAgentAlbumEdited
	case errors.Is(err, album.ErrLate):
		return errAgentLate
	}
	return err
}

// agentBackup — копия альбома перед первой правкой агента (§2.4): несохранённое пишется в файл тем
// же путём, что автосохранение, но с ошибкой; затем копия без «раз в 10 минут», закреплённая до
// выхода. Одна копия на ключ за запуск программы (переподключения не плодят). Не вышло — errAgentBackup,
// правки нет. Копировать нечего (файла нет, пустой черновик) — nil.
func (a *App) agentBackup(key string) error {
	ed, err := a.agentEditor(key)
	if err != nil {
		return err
	}
	s := a.agentEd()
	s.backMu.Lock()
	defer s.backMu.Unlock()
	if s.backed[key] {
		return nil
	}
	name, err := a.agentPinned(ed)
	if err != nil {
		return err
	}
	if name != "" {
		s.backed[key] = true
	}
	return nil
}

// agentPinned пишет альбом на диск и кладёт его копию, закреплённую до выхода (ForcePin: не уйдёт
// в чистку, в том числе от нового черновика). Имя "" без ошибки — копировать нечего; любая
// причина, по которой копии нет (нет хранилища, том, запись), — errAgentBackup.
func (a *App) agentPinned(ed *album.Editor) (string, error) {
	file, bkey, err := a.agentSaved(ed)
	if err != nil {
		log.Printf("копия перед правкой агента: %v", paths.NoPath(err))
		return "", errAgentBackup
	}
	if file == "" { // копировать нечего
		return "", nil
	}
	if a.backups == nil || !reachable(file) {
		return "", errAgentBackup
	}
	name, err := a.backups.ForcePin(file, bkey)
	if err != nil {
		log.Printf("копия перед правкой агента %s: %v", bkey, paths.NoPath(err))
		return "", errAgentBackup
	}
	return name, nil
}

// agentSaved пишет альбом на диск (файл проекта или черновик) и возвращает файл для копии и ключ
// папки копий; file "" — копировать нечего. Всё под saveMu, как автосохранение.
func (a *App) agentSaved(ed *album.Editor) (file, bkey string, err error) {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	// Версия на диске до записи — в копию тем же путём, что в save(): иначе Save затёрла бы её.
	if path := ed.Path(); path != "" {
		a.backupBefore(path, backupKey(path), false)
		if err := ed.Save(""); err != nil {
			return "", "", err
		}
		a.noticeBackup(ed)
		v := ed.View()
		a.emit("saved", Saved{Dirty: v.Dirty, Path: v.Path})
		return path, backupKey(path), nil
	}
	v := ed.View()
	if len(v.Spreads) == 0 && len(v.Folders) == 0 && v.Cover == nil { // пустой альбом защищать нечем
		return "", "", nil
	}
	if a.draftLocked.Load() || a.draftNewer.Load() || a.draftPath() == "" {
		return "", "", errors.New("черновик недоступен для записи")
	}
	if backup.HasContent(a.draftPath()) {
		a.backupBefore(a.draftPath(), backupKey(""), false)
	}
	if err := ed.WriteDraft(a.draftPath()); err != nil {
		return "", "", err
	}
	a.noticeBackup(ed)
	if !backup.HasContent(a.draftPath()) {
		return "", "", nil
	}
	return a.draftPath(), backup.DraftKey, nil
}

// agentVary — «Разнообразить раскладки» для листов spreads (ID): variety 1…2, families — фильтр
// (пусто — все). Один шаг отмены; Changed — у скольких листов сменился ключ раскладки (0 — менять
// нечего: rev прежний, шага нет). Before и After — семейство каждого листа до и после (154): ревизия
// берётся под тем же замком, что и чтение; чужая правка между чтением и вызовом — ErrAlbumEdited, а
// сразу после вызова — пара пустая.
func (a *App) agentVary(ctx context.Context, key string, rev int, spreads []string, variety int, families []string) (res agents.VaryRes, err error) {
	defer guardErr("agentVary", &err)
	ed, err := a.agentEditor(key)
	if err != nil {
		return res, err
	}
	if err := a.agentCheckTool(toolVary, true); err != nil {
		return res, err
	}
	before := ed.View()
	if before.Rev != rev {
		return res, errAgentAlbumEdited
	}
	was, r0 := ed.Families(spreads)
	if r0 != rev {
		return res, errAgentAlbumEdited
	}
	old := make(map[string]string, len(before.Spreads))
	for _, sv := range before.Spreads {
		old[sv.ID] = sv.Layout
	}
	// Оценки из XMP берутся до замка Editor: замки не вкладываются.
	infos := a.infosOrRefs(ed.PhotosOf(spreads))
	v, err := ed.VaryLayoutsIf(spreads, model.Variety(max(variety, 1)).Clamp(), model.ClampFamilies(families), infos,
		album.Expect{Rev: rev, Ctx: ctx})
	if err != nil {
		return res, agentErr(err)
	}
	for _, sv := range v.Spreads {
		if l, ok := old[sv.ID]; ok && l != sv.Layout && slices.Contains(spreads, sv.ID) {
			res.Changed++
		}
	}
	res.Album = agents.AlbumRef{Key: key, Rev: v.Rev}
	if now, r1 := ed.Families(spreads); r1 == v.Rev { // иначе человек успел поправить: пары нет
		res.Spreads, res.Before, res.After = spreads, was, now
	}
	a.changed(v)
	return res, nil
}

// agentShow — проверка ключа и листа, затем agents:show: холст переходит, альбом не меняется.
// spreadID — ID листа (model.CoverID — обложка); ответ — номер разворота с 1, у обложки 0.
func (a *App) agentShow(key, spreadID string) (n int, err error) {
	defer guardErr("agentShow", &err)
	ed, err := a.agentEditor(key)
	if err != nil {
		return 0, err
	}
	v := ed.View()
	if spreadID == model.CoverID && v.Cover != nil && !v.Settings.Sheet {
		a.agentShown(AgentShow{Spread: model.CoverID})
		return 0, nil
	}
	for i, sv := range v.Spreads {
		if sv.ID == spreadID && spreadID != model.CoverID {
			a.agentShown(AgentShow{Spread: sv.ID})
			return i + 1, nil
		}
	}
	return 0, errAgentNoSpread
}

// agentUndo — отмена своей правки: Undo под замком Editor при совпавшем rev (иначе ничего не
// отменено); изменились источники — лента перечитывается, как у App.Undo.
func (a *App) agentUndo(ctx context.Context, key string, rev int) (rev2 int, err error) {
	defer guardErr("agentUndo", &err)
	ed, err := a.agentEditor(key)
	if err != nil {
		return 0, err
	}
	if err := a.agentCheckTool(toolUndo, true); err != nil {
		return 0, err
	}
	before := sourcesOf(ed)
	v, err := ed.UndoIfRev(album.Expect{Rev: rev, Ctx: ctx})
	if err != nil {
		return 0, agentErr(err)
	}
	a.changed(v)
	if before.differs(sourcesOf(ed)) {
		a.rescan()
	}
	return v.Rev, nil
}
