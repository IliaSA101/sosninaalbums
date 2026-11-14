package osx

// CloudRoots — корни облачных папок по настройкам самих клиентов (160, design §4): Dropbox (info.json на всех ОС),
// Яндекс.Диск (config.cfg на Linux, папка по умолчанию на macOS), на Windows ещё реестр — SyncRootManager и закреплённые
// папки Проводника. Папки не угадываются по умолчаниям (человек часто переносит их на другой диск), диски не
// перебираются: виртуальные облачные диски узнаёт VirtualCloud по тому проверяемого пути.
// OneDrive и iCloud здесь нет — их знает agentplaces.CloudRoots по окружению.

import (
	"os"
	"runtime"
)

// CloudRoots — корни облачных папок, найденные в настройках клиентов. local — фильтр путей вызывающего
// (agentplaces передаёт paths.CheckLocalPath: osx ничего из internal не импортирует); он применяется и к путям
// файлов настроек до чтения (%APPDATA% на UNC не читается), и к найденным корням. nil — пусто (закрыто по умолчанию).
// Чтение синхронное, без срока: срок ставит вызывающий (appkit.Within).
func CloudRoots(home string, local func(string) error) []string {
	if local == nil {
		return nil
	}
	var raw []string
	for _, f := range dropboxFiles(runtime.GOOS, home, os.Getenv) {
		if data, ok := readCloudFile(f, local); ok {
			raw = append(raw, parseDropboxInfo(data)...)
		}
	}
	raw = append(raw, cloudRootsOS(home, local)...)
	return cleanCloudRoots(raw, local)
}
