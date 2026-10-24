package app

// Окно выбора цвета (спека 109): «Свои цвета» (config.json → myColors, правила — internal/prefs)
// и тонкие обёртки над internal/app/colorpick — пипетка и «С фото». Альбом методы не меняют:
// Host.Changed, событий и rev нет.

import (
	"context"

	"github.com/IliaSA101/sosninaalbums/internal/app/colorpick"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// MyColors — «Свои цвета» из config.json после проверки; никогда не nil.
func (a *App) MyColors() []string { return prefs.MyColors(a.cfgDir) }

// RememberColor ставит hex первым (без повторов, ≤ 10); мусор — список без изменений.
func (a *App) RememberColor(hex string) []string { return prefs.RememberColor(a.cfgDir, hex) }

// ForgetColor убирает hex; нет такого — список без изменений.
func (a *App) ForgetColor(hex string) []string { return prefs.ForgetColor(a.cfgDir, hex) }

// ImportColors — перенос прежних «Последних» из localStorage: пишет, только если в config.json
// пусто, иначе отдаёт что есть (второе окно не задвоит список).
func (a *App) ImportColors(list []string) []string { return prefs.ImportColors(a.cfgDir, list) }

// SpreadPhotoColors — до 8 главных цветов фото листа по убыванию доли (109-3); нет фото — [].
// Тело — colorpick.Picker.PhotoColors.
func (a *App) SpreadPhotoColors(spreadID string) (cols []string, err error) {
	defer guardErr("SpreadPhotoColors", &err)
	return a.colorPick.PhotoColors(spreadID)
}

// PickColorAt — цвет точки (x, y) мм листа spreadID на готовом развороте: "#rrggbb" (пипетка).
// Ошибки: «лист не найден», «точка вне листа». Тело — colorpick.Picker.PickAt.
func (a *App) PickColorAt(spreadID string, x, y float64) (hex string, err error) {
	defer guardErr("PickColorAt", &err)
	return a.colorPick.PickAt(spreadID, x, y)
}

// colorHost — что окну цвета нужно от App.
func (a *App) colorHost() colorpick.Host {
	return colorpick.Host{
		Editor: a.editor,
		Photo:  a.mediaPhoto,
		Cache:  func() *library.Cache { return a.cache },
		Ctx:    func() context.Context { return a.stopCtx },
		// Через переменную пакета: тесты подменяют reachable.
		Reachable: func(path string) bool { return reachable(path) },
		Rare:      logRare,
	}
}
