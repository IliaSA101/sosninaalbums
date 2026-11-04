package app

import "github.com/IliaSA101/sosninaalbums/internal/prefs"

// Настройки программы (config.json): уровень, «Свой», стартовое окно, туры обучения.
// Чтение и запись файла — loadConfig/updateConfig в files.go.

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

// SetTour запоминает состояние тура обучения (113) в config.json; проект не меняет (ни rev, ни
// dirty). Негодный id или state — без записи. Возвращает настройки из файла: флаги TourDue и
// TourMore уже пересчитаны.
func (a *App) SetTour(id, state string) prefs.Prefs {
	a.updateConfig(func(c *prefs.Config) { c.Tours = prefs.SetTourState(c.Tours, id, state) })
	return a.prefs()
}
