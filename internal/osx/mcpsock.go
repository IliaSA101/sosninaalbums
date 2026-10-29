package osx

import "path/filepath"

// roamingDirFn — подмена в тестах: на Windows настоящая известная папка не зависит от среды.
var roamingDirFn = roamingDir

// MCPSock — путь сокета службы агентов (143): <папка настроек>/<name>/mcp/sock, name — DataName
// (у стенда своя папка). "" — папки настроек нет (пустая или не абсолютная): пути от текущей
// папки процесса не бывает, служба тогда не стартует и посредник не ищет сокет.
// Один путь для программы и посредника `--mcp`.
func MCPSock(name string) string {
	base := roamingDirFn()
	if base == "" || name == "" {
		return ""
	}
	return filepath.Join(base, name, "mcp", "sock")
}
