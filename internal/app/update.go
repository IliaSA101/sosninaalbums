package app

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/updater"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/update"
)

// Обновление программы (спека 111, разрез 126 Ш1): тело — internal/app/updater, здесь фасад с
// прежними методами для интерфейса, определённый тип UpdateInfo и Host службы.

// UpdateInfo — состояние обновления для блока «О программе» и тостов. Тип определён (не псевдоним)
// от updater.Info: поля и теги те же, а models.ts остаётся в пространстве app (126-2).
type UpdateInfo updater.Info

// UpdateInfo — текущее состояние без сети; Just и Failed отдаются один раз.
func (a *App) UpdateInfo() UpdateInfo { return UpdateInfo(a.upd.Info()) }

// CheckUpdate — «Проверить обновления»: всегда в сеть, итог — в фазе.
func (a *App) CheckUpdate() (UpdateInfo, error) {
	info, err := a.upd.Check()
	return UpdateInfo(info), err
}

// DownloadUpdate — скачать и проверить предложенное; ход — update:progress.
func (a *App) DownloadUpdate() (UpdateInfo, error) {
	info, err := a.upd.Download()
	return UpdateInfo(info), err
}

// CancelUpdate — «Отменить»: идущее скачивание или проверка обрывается.
func (a *App) CancelUpdate() { a.upd.Cancel() }

// InstallUpdate — «Перезапустить и обновить»: только ready и Windows.
func (a *App) InstallUpdate() error { return a.upd.Install() }

// OpenUpdatePage — страница выпуска в браузере.
func (a *App) OpenUpdatePage() error { return a.upd.OpenPage() }

// SetUpdateCheck — флажок «Проверять при запуске».
func (a *App) SetUpdateCheck(atStart bool) UpdateInfo { return UpdateInfo(a.upd.SetCheck(atStart)) }

// updHost — что службе обновления нужно от App. Поля — замыкания над a: тесты подменяют a.emit,
// a.cfgDir, openURL и setStart уже после NewApp.
func (a *App) updHost() updater.Host {
	return updater.Host{
		CfgDir:  func() string { return a.cfgDir },
		Version: func() string { return appVersion(wailsJSON) },
		Emit:    func(name string, data ...any) { a.emit(name, data...) },
		GoBG:    func(fn func()) bool { return a.goBG(fn) },
		StopCtx: func() context.Context { return a.stopCtx },
		Quit:    a.quitWindow,
		OpenURL: func(u string) { openURL(a, u) },
		// Общие с проверкой библиотеки (packs_net.go): тесты ставят их через setStart.
		StartTimes: func() (time.Duration, time.Duration) { return startDelay, startEvery },
		Client: func() (*update.Client, bool) {
			src, auto := update.Default()
			return update.New(src), auto
		},
		GOOS:  func() string { return runtime.GOOS },
		Exe:   os.Executable,
		Dir:   func() string { return update.Dir(a.data) },
		Free:  osx.FreeBytes,
		Start: update.Start,
	}
}
