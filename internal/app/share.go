package app

import (
	"context"
	"path/filepath"
	"runtime"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/app/sharer"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Отправка альбома по ссылке (спека 146, design §3, §4): тело — internal/app/sharer, здесь фасад с
// методами для интерфейса, DTO (вложенные типы написаны в app заново: пары в TestDTOPairs),
// Host службы и то, что она берёт у App: снимок альбома (sharejob.go), диалоги, браузер.
// События: share:progress {phase, done, total, bytes, totalBytes}, share:done {err, stopped} —
// без ссылки и id (разбор С3).

// ShareOptions — настройки окна «Выгрузить»; определённый тип от sharer.Options (плоский, как ExportOptions).
type ShareOptions sharer.Options

// ShareChange — правка выгруженной ссылки (срок, пароль); определённый тип от sharer.Change.
type ShareChange sharer.Change

// ShareView — содержимое окна «Отправить по ссылке».
type ShareView struct {
	Service ShareService `json:"service"`
	Album   ShareAlbum   `json:"album"`
	Link    *ShareLink   `json:"link"`      // nil — альбом не выгружался (state none)
	Busy    bool         `json:"busy"`      // выгрузка этого альбома идёт (окно показывает ход)
	Other   bool         `json:"busyOther"` // идёт выгрузка другого альбома (В1): хода и чужой ссылки нет
}

// ShareService — подключение к сервису.
type ShareService struct {
	Host  string `json:"host"`  // "" — не подключён; иначе имя хоста для подписи
	URL   string `json:"url"`   // адрес из config.json (поле формы)
	Token bool   `json:"token"` // токен есть (в хранилище или в памяти)
	Store string `json:"store"` // "system" | "session" — где токен
	Days  int    `json:"days"`  // limits.max_days сервиса; 0 — неизвестно
}

// ShareAlbum — что можно сказать о текущем альбоме.
type ShareAlbum struct {
	Block string `json:"block"` // причина, по которой выгрузить нельзя ("" — можно)
	Save  bool   `json:"save"`  // причина — не сохранён: окно даёт «Сохранить…»
	Title string `json:"title"` // название по умолчанию: из записи или имя файла альбома
	Pages int    `json:"pages"` // страниц книги
	Bytes int64  `json:"bytes"` // оценка размера страниц
}

// ShareLink — ссылка альбома; строки для окна готовы в Go.
type ShareLink struct {
	State     string `json:"state"` // none | incomplete | ok | changed | expired | gone | other
	URL       string `json:"url"`
	Title     string `json:"title"`
	Password  string `json:"password"`  // "" — без пароля
	ExpiresAt string `json:"expiresAt"` // RFC 3339
	Until     string `json:"until"`     // «до 29 ноября 2026»
	Host      string `json:"host"`      // сервис записи (для state other)
	Published string `json:"published"` // «Выгружено 29 октября 2026 как «…»» (С1)
}

func shareView(v sharer.View) ShareView {
	out := ShareView{
		Service: ShareService(v.Service), Album: ShareAlbum(v.Album), Busy: v.Busy, Other: v.Other,
	}
	if v.Link != nil {
		l := ShareLink(*v.Link)
		out.Link = &l
	}
	return out
}

// ShareState — состояние окна без сети: подключение, альбом, ссылка.
func (a *App) ShareState() (res ShareView, err error) {
	defer guardErr("ShareState", &err)
	v, err := a.shr.State()
	return shareView(v), err
}

// ShareConnect — «Подключить»: адрес и токен проверяются у сервиса, затем сохраняются (токен — в
// хранилище паролей системы или, если его нет, в памяти).
func (a *App) ShareConnect(url, token string) (res ShareView, err error) {
	defer guardErr("ShareConnect", &err)
	v, err := a.shr.Connect(url, token)
	return shareView(v), err
}

// ShareForget — «Отключить»: токен убирается, адрес остаётся в поле.
func (a *App) ShareForget() (res ShareView, err error) {
	defer guardErr("ShareForget", &err)
	v, err := a.shr.Forget()
	return shareView(v), err
}

// Share — «Выгрузить» / «Обновить по ссылке» / «Продолжить»: проверки и вопросы здесь, остальное в
// фоне; ход — событиями share:progress и share:done. false, nil — ответ «Нет» на вопрос.
func (a *App) Share(o ShareOptions) (started bool, err error) {
	defer guardErr("Share", &err)
	return a.shr.Share(sharer.Options(o))
}

// CancelShare — «Остановить»: опубликованная прежняя версия остаётся.
func (a *App) CancelShare() { a.shr.Cancel() }

// ShareChange — срок от сегодня и пароль выгруженной ссылки (PATCH).
func (a *App) ShareChange(c ShareChange) (res ShareView, err error) {
	defer guardErr("ShareChange", &err)
	v, err := a.shr.Change(sharer.Change(c))
	return shareView(v), err
}

// ShareRevoke — «Отозвать ссылку»: вопрос («Нет» по умолчанию), затем удаление на сервисе и запись.
func (a *App) ShareRevoke() (res ShareView, err error) {
	defer guardErr("ShareRevoke", &err)
	v, err := a.shr.Revoke()
	return shareView(v), err
}

// SharePassword — «Придумать»: слово и четыре цифры.
func (a *App) SharePassword() string { return a.shr.Password() }

// ShareText — текст «Скопировать для клиента»: ссылка, пароль, срок. Копирует интерфейс.
func (a *App) ShareText() (text string, err error) {
	defer guardErr("ShareText", &err)
	return a.shr.Text()
}

// OpenShare открывает ссылку альбома в браузере; адрес — только проверенный (printprof.CheckURL).
func (a *App) OpenShare() (err error) {
	defer guardErr("OpenShare", &err)
	return a.shr.Open()
}

// shareCacheDir — папка кэша программы, где лежат временные страницы (<кэш>/<папка программы>/share).
func (a *App) shareCacheDir() string {
	c := osx.CacheDir()
	if c == "" {
		return ""
	}
	return filepath.Join(c, a.data)
}

// shareHost — что службе выгрузки нужно от App. Поля — замыкания над a (позднее связывание: cfgDir
// ставит startup, диалоги и браузер подменяют тесты после NewApp).
func (a *App) shareHost() sharer.Host {
	return sharer.Host{
		Plan:          a.shareJob,
		Ident:         func() (string, string) { return a.editor().ShareIdent() },
		Pin:           a.pinShare,
		Emit:          func(name string, data ...any) { a.emit(name, data...) },
		GoBG:          func(fn func()) bool { return a.goBG(fn) },
		StopCtx:       func() context.Context { return a.stopCtx },
		Ask:           func(msg string) (bool, error) { return askShare(a, msg) },
		ExportBusy:    func() bool { running, _ := a.exp.Busy(); return running },
		LiftMemory:    liftMemoryLimit,
		CheckURL:      printprof.CheckURL,
		OpenURL:       func(u string) { openURL(a, u) },
		Fold:          paths.Fold,
		CfgDir:        func() string { return a.cfgDir },
		CacheDir:      a.shareCacheDir,
		Store:         a.secrets,
		AllowLoopback: osx.DevBuild,
		Version:       appVersion(wailsJSON),
	}
}

// pinShare — Host.Pin: отпечаток альбома path в его файл, если его там нет (альбом до 0.32 с
// открытия не сохранялся, Restore или Ctrl+Z сменили отпечаток), а печать не этого пути — новый
// отпечаток с печатью path (album.Editor.SaveUID). Запись — как Ctrl+S: копия файла до неё,
// событие saved. Возвращает отпечаток для записи о ссылке. Альбом сменился — album.ErrUIDChanged.
func (a *App) pinShare(path, uid string) (string, error) {
	ed := a.editor()
	need, err := ed.NeedUID(path, uid)
	if err != nil || !need {
		return uid, err
	}
	a.backupBefore(path, backupKey(path), false)
	uid, err = ed.SaveUID(path, uid)
	if err != nil {
		return "", err
	}
	a.noticeBackup(ed)
	v := ed.View()
	a.emit("saved", Saved{Dirty: v.Dirty, Path: v.Path})
	return uid, nil
}

// askShare — вопрос службы выгрузки; окно называется «Отправить по ссылке» (подмена в тестах).
var askShare = func(a *App, msg string) (bool, error) {
	opts := askOptions(runtime.GOOS, msg)
	opts.Title = "Отправить по ссылке"
	ans, err := wr.MessageDialog(a.ctx, opts)
	return err == nil && ans == opts.Buttons[0], err
}
