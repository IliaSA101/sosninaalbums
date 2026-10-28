package app

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

// linuxOptions — опции окна Linux (101). ProgramName — имя папок данных (osx.DataName): иначе
// WebKitGTK хранит localStorage по имени исполняемого файла, и настройки интерфейса теряются
// при смене имени; у стенда QA своё. GPU-политика Never — то, что Wails ставит сам при
// пустых опциях (wails#2977); ненулевая структура дала бы нулевое OnDemand. icon — значок окна
// для запуска из tar.gz без .desktop. На других ОС Wails опции игнорирует.
func linuxOptions(data string, icon []byte) *linux.Options {
	return &linux.Options{
		Icon:             icon,
		WebviewGpuPolicy: linux.WebviewGpuPolicyNever,
		ProgramName:      data,
	}
}

// appMenu — системное меню окна. Только macOS: роли App (Cmd+Q, Cmd+H), Edit (Cmd+C/V/X/A/Z
// в полях ввода) и Window (Cmd+M). На Windows и Linux nil — иначе Wails нарисовал бы родную
// строку меню над нашей. Подписи системные (английские), своих пунктов с селекторами Wails v2
// не даёт.
func appMenu(goos string) *menu.Menu {
	if goos != "darwin" {
		return nil
	}
	return menu.NewMenuFromItems(menu.AppMenu(), menu.EditMenu(), menu.WindowMenu())
}
