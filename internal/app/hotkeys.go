package app

// Свои горячие клавиши (спека 074, design §6): своё хранится в config.json → hotkeys (только
// отличия от стандартных), правила — internal/keymap. Undo альбома не участвует: «Вернуть»
// окна — RestoreHotkeys со снимком View.Custom.

import (
	"github.com/IliaSA101/sosninaalbums/internal/keymap"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Keymap — каталог с действующими клавишами (окно «Горячие клавиши»; при запуске — State.Keymap).
func (a *App) Keymap() keymap.View {
	return keymap.Build(a.loadConfig().Hotkeys)
}

// editHotkeys — правка своего под замком config.json. Расчёт идёт от файла, а не от вида
// окна: два экземпляра интерфейса (окно и вкладка браузера) не затирают правки друг друга.
// fn возвращает новое своё и write = false, если записывать нечего (ошибка, вопрос «Заменить?»).
func (a *App) editHotkeys(fn func(o keymap.Overrides) (keymap.Overrides, bool)) keymap.Overrides {
	var out keymap.Overrides
	a.updateConfig(func(c *prefs.Config) {
		o, write := fn(c.Hotkeys)
		out = o
		if write {
			c.Hotkeys = o
		}
	})
	return out
}

// SetHotkey назначает команде id сочетание chord: old == "" — добавить, иначе заменить old.
// Клавиша у команды той же области и !replace — в ответе Conflict, ничего не записано
// (окно спрашивает «Заменить?» и зовёт снова с replace). Ошибка — текст для строки окна.
func (a *App) SetHotkey(id, old, chord string, replace bool) (keymap.SetResult, error) {
	var out keymap.Outcome
	var err error
	o := a.editHotkeys(func(o keymap.Overrides) (keymap.Overrides, bool) {
		o, out, err = keymap.Assign(o, id, old, chord, replace)
		return o, err == nil && out.Conflict == nil
	})
	if err != nil {
		return keymap.SetResult{}, err
	}
	return keymap.Result(o, out), nil
}

// RemoveHotkey убирает у команды сочетание; последнее убранное оставляет её без клавиши.
func (a *App) RemoveHotkey(id, chord string) (keymap.View, error) {
	var err error
	o := a.editHotkeys(func(o keymap.Overrides) (keymap.Overrides, bool) {
		o, err = keymap.Remove(o, id, chord)
		return o, err == nil
	})
	if err != nil {
		return keymap.View{}, err
	}
	return keymap.Build(o), nil
}

// ResetHotkey возвращает команде стандартные клавиши. Стандартное занято своим другой команды
// той же области и !replace — Conflict, как у SetHotkey (074-22).
func (a *App) ResetHotkey(id string, replace bool) (keymap.SetResult, error) {
	var out keymap.Outcome
	var err error
	o := a.editHotkeys(func(o keymap.Overrides) (keymap.Overrides, bool) {
		o, out, err = keymap.Reset(o, id, replace)
		return o, err == nil && out.Conflict == nil
	})
	if err != nil {
		return keymap.SetResult{}, err
	}
	return keymap.Result(o, out), nil
}

// ResetHotkeys — «Сбросить всё как было» без вопроса (074-11): команды каталога — стандартные,
// незнакомые id (от новой версии) остаются. Прежнее своё окно берёт из View.Custom до вызова.
func (a *App) ResetHotkeys() keymap.View {
	return keymap.Build(a.editHotkeys(func(o keymap.Overrides) (keymap.Overrides, bool) {
		return keymap.ResetAll(o), true
	}))
}

// RestoreHotkeys — «Вернуть»: своё команд каталога — из снимка custom (View.Custom до правки),
// чужие id — из файла. Снимок из интерфейса проходит те же пределы, что файл (Sanitize).
func (a *App) RestoreHotkeys(custom map[string][]string) keymap.View {
	return keymap.Build(a.editHotkeys(func(o keymap.Overrides) (keymap.Overrides, bool) {
		return keymap.Restore(o, custom), true
	}))
}
