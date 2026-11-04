package osx

// remoteOS (Linux): точка монтирования с длиннейшим префиксом пути из /proc/self/mountinfo и
// список разрешённых ФС (mountinfo.go). Нет точки или файл не читается — ошибка, то есть «сетевой».

import "errors"

func remoteOS(path string) (bool, error) {
	ms, err := readMounts()
	if err != nil {
		return true, err
	}
	if len(ms) == 0 {
		return true, errors.New("таблица монтирования пуста")
	}
	return remoteByMounts(ms, path), nil
}
