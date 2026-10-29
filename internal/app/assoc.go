package app

import (
	"os"

	"github.com/IliaSA101/sosninaalbums/internal/app/assoc"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
)

// Связь .salbum (спека 112) и «файл новее программы» (098): тело — internal/app/assoc, здесь фасад
// с прежними методами для интерфейса, определённые типы AssocView и NewerInfo и Host службы
// (разрез 126 Ш2).

// AssocView — связь .salbum для интерфейса (State.Assoc, ответы SetAssoc и AssocLater). Тип
// определён (не псевдоним) от assoc.View: поля и теги те же, а models.ts остаётся в пространстве
// app (126-2).
type AssocView assoc.View

// NewerInfo — окно «Альбом из новой версии» (State.Newer, 112 design §5); определён от
// assoc.NewerInfo по той же причине.
type NewerInfo assoc.NewerInfo

// SetAssoc — «Да» стартового окна, галочка меню (on) и «Не спрашивать» (off).
func (a *App) SetAssoc(on bool) AssocView { return AssocView(a.assoc.Set(on)) }

// AssocLater — «Не сейчас»: вопрос не возвращается до следующей версии (major.minor).
func (a *App) AssocLater() AssocView { return AssocView(a.assoc.Later()) }

// OpenInNewer запускает найденную новую версию с файлом из окна «Альбом из новой версии» и
// закрывает эту программу.
func (a *App) OpenInNewer() error { return a.assoc.OpenInNewer() }

// assocView — связь для State.Assoc.
func (a *App) assocView() AssocView { return AssocView(a.assoc.View()) }

// syncAssoc — при запуске: связь приводится к плану в фоне.
func (a *App) syncAssoc(fromFile bool) { a.assoc.Sync(fromFile) }

// setNewer запоминает отказ открыть path (draft — черновик) и возвращает данные окна.
func (a *App) setNewer(path string, draft bool, err error) NewerInfo {
	return NewerInfo(a.assoc.SetNewer(path, draft, err))
}

// startNewer — отказ открыть кандидата запуска: окно покажет первый GetState.
func (a *App) startNewer(path string, draft bool, err error) { a.assoc.StartNewer(path, draft, err) }

// takeStartNewer — отказ при старте, один раз.
func (a *App) takeStartNewer() *NewerInfo { return (*NewerInfo)(a.assoc.TakeStartNewer()) }

// newerText — текст ошибки открытия err для what с версией этой программы (assoc.NewerText).
func newerText(err error, what string) string {
	return assoc.NewerText(err, what, appVersion(wailsJSON))
}

// openErr — ошибка открытия альбома для ответа фронту: файл новее программы — с версией.
func openErr(err error) error { return assoc.OpenErr(err, appVersion(wailsJSON)) }

// useSystemAssoc подключает настоящий реестр; зовёт Run до окна, тесты — никогда.
func (a *App) useSystemAssoc() { a.assocReg = assoc.System{} }

// assocHost — что службе связи нужно от App. Поля — замыкания над a: тесты подменяют a.emit,
// a.cfgDir, a.exists и a.data уже после NewApp.
func (a *App) assocHost() assoc.Host {
	return assoc.Host{
		CfgDir:    func() string { return a.cfgDir },
		Data:      func() string { return a.data },
		Version:   func() string { return appVersion(wailsJSON) },
		Notice:    func(text string) { a.notice(text) },
		GoBG:      func(fn func()) bool { return a.goBG(fn) },
		Exists:    func(path string) bool { return a.exists(path) },
		FlushSave: func() { a.flushSave() },
		Quit:      a.quitWindow,
		Reg:       func() assoc.Registry { return a.assocReg },
		Place:     launchPlace,
		Temp:      os.TempDir,
		Start:     osx.Start,
		DevBuild:  func() bool { return osx.DevBuild },
	}
}
