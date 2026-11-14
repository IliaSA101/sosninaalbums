package app

// Настройки и «Последние»: config.json, папки диалогов, список последних альбомов.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Настройки и «Последние» — рядом с открытием файлов: OpenRecent идёт через openPath.

// config.json лежит в папке данных App (cfgDir); логика и замок — в internal/prefs.
func (a *App) loadConfig() prefs.Config { return prefs.LoadConfig(a.cfgDir) }

func (a *App) updateConfig(fn func(c *prefs.Config)) { prefs.UpdateConfig(a.cfgDir, fn) }

func (a *App) rememberDir(kind, dir string) {
	if kind == prefs.DirPhotos {
		a.seedPlaces() // засев мест из прежней последней папки — до того, как её перепишут (agentplaces_app.go)
	}
	prefs.RememberDir(a.cfgDir, kind, dir)
}

func (a *App) dialogDir(kind string) string { return prefs.DialogDir(a.cfgDir, kind) }

// rememberRecent записывает альбом в «последние». Черновик (его путь внутренний) не пишем.
func (a *App) rememberRecent(path string) {
	if path == "" || paths.FoldEqual(path, a.draftPath()) {
		return
	}
	a.updateConfig(func(c *prefs.Config) { c.Recent = prefs.PushRecent(c.Recent, path) })
}

// OpenRecent открывает альбом из списка последних. Принимается только путь, который
// есть в config.recent: интерфейс не может открыть произвольный файл. Нет файла —
// ошибка, запись остаётся (её убирает ForgetRecent).
func (a *App) OpenRecent(path string) (State, error) {
	if err := a.recentCheck(path); err != nil {
		return a.GetState(), err
	}
	return a.openPath(path)
}

// Отказы recentCheck: агент по ним различает «нет в последних» и «не альбом» (agentopen.go).
var (
	errNotRecent = errors.New("альбома нет в списке последних")
	errNotAlbum  = fmt.Errorf("это не альбом (%s)", projectExt)
)

// recentCheck — путь можно открыть из «Последних»: он есть в config.recent, локальный и с суффиксом
// альбома. Общая проверка меню (OpenRecent) и open_album агента.
func (a *App) recentCheck(path string) error {
	if prefs.RecentIndex(a.loadConfig().Recent, path) < 0 {
		return errNotRecent
	}
	if err := paths.CheckLocalPath(path); err != nil {
		return err
	}
	if !strings.HasSuffix(strings.ToLower(path), projectExt) {
		return errNotAlbum
	}
	return nil
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
