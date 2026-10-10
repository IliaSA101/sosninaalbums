package app

import (
	"fmt"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// config.json лежит в папке данных App (cfgDir); логика и замок — в internal/prefs.
func (a *App) loadConfig() prefs.Config { return prefs.LoadConfig(a.cfgDir) }

func (a *App) updateConfig(fn func(c *prefs.Config)) { prefs.UpdateConfig(a.cfgDir, fn) }

func (a *App) rememberDir(kind, dir string) { prefs.RememberDir(a.cfgDir, kind, dir) }

func (a *App) dialogDir(kind string) string { return prefs.DialogDir(a.cfgDir, kind) }

// rememberRecent записывает альбом в «последние». Черновик (его путь внутренний) не пишем.
func (a *App) rememberRecent(path string) {
	if path == "" || strings.EqualFold(path, a.draftPath()) {
		return
	}
	a.updateConfig(func(c *prefs.Config) { c.Recent = prefs.PushRecent(c.Recent, path) })
}

// prefsAndRecent — для State: настройки и список последних из config.json.
func (a *App) prefsAndRecent() (prefs.Prefs, []prefs.RecentProject) {
	c := a.loadConfig()
	return prefs.Of(c), prefs.RecentView(c.Recent)
}

// prefs — только настройки, без RecentView: его os.Stat на отключённом сетевом диске
// держал бы щелчок по уровню или флажку до таймаута SMB.
func (a *App) prefs() prefs.Prefs {
	return prefs.Of(a.loadConfig())
}

// SetLevel сохраняет только уровень (custom — тоже, чужой — simple); возвращает настройки из файла.
// Поля пишутся раздельно: два экземпляра интерфейса (окно и вкладка браузера) не затирают
// чужой флажок устаревшим состоянием.
func (a *App) SetLevel(level string) prefs.Prefs {
	lvl := prefs.NormLevel(level)
	a.updateConfig(func(c *prefs.Config) { c.Level = lvl })
	return a.prefs()
}

// SetCustom сохраняет только набор «Своего» (очищенный); уровень не меняет — его пишет
// SetLevel. Возвращает настройки из файла.
func (a *App) SetCustom(custom prefs.CustomLevel) prefs.Prefs {
	cl := prefs.SanitizeCustom(custom)
	a.updateConfig(func(c *prefs.Config) { c.Custom = &cl })
	return a.prefs()
}

// SetShowStart сохраняет только флажок стартового окна; возвращает настройки из файла.
func (a *App) SetShowStart(show bool) prefs.Prefs {
	a.updateConfig(func(c *prefs.Config) { c.HideStart = !show })
	return a.prefs()
}

// OpenRecent открывает альбом из списка последних. Принимается только путь, который
// есть в config.recent: интерфейс не может открыть произвольный файл. Нет файла —
// ошибка, запись остаётся (её убирает ForgetRecent).
func (a *App) OpenRecent(path string) (State, error) {
	if prefs.RecentIndex(a.loadConfig().Recent, path) < 0 {
		return a.GetState(), fmt.Errorf("альбома нет в списке последних")
	}
	if err := paths.CheckLocalPath(path); err != nil {
		return a.GetState(), err
	}
	if !strings.HasSuffix(strings.ToLower(path), projectExt) {
		return a.GetState(), fmt.Errorf("это не альбом (%s)", projectExt)
	}
	return a.openPath(path)
}

// ForgetRecent убирает альбом из списка последних (сам файл не трогает).
func (a *App) ForgetRecent(path string) []prefs.RecentProject {
	a.updateConfig(func(c *prefs.Config) { c.Recent = prefs.DropRecent(c.Recent, path) })
	_, recent := a.prefsAndRecent()
	return recent
}

// Recent — список последних без записи config.json: «Сохранить как» кладёт альбом в список
// без события, и UI перечитывает его при открытии меню «Файл» и стартового окна.
func (a *App) Recent() []prefs.RecentProject {
	_, recent := a.prefsAndRecent()
	return recent
}

// ClearRecent очищает список последних одной записью config.json (файлы не трогает).
func (a *App) ClearRecent() []prefs.RecentProject {
	a.updateConfig(func(c *prefs.Config) { c.Recent = nil })
	return []prefs.RecentProject{}
}
