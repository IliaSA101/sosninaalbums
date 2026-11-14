//go:build windows

package osx

import (
	"errors"
	"path/filepath"
	"syscall"
)

// longPath — длинное имя вместо 8.3 (%TEMP% бывает «C:\Users\ABCDEF~1\…»). GetLongPathName
// не умеет несуществующие пути, поэтому раскрываем самого глубокого существующего предка, а
// недостающий хвост дописываем как есть: иначе exe в ещё не созданной подпапке остался бы в
// короткой форме и не совпал бы с длинной TEMP. При любой ошибке возвращает p как есть.
func longPath(p string) string { return longPathVia(p, longName) }

// longPathVia — longPath с подменяемым GetLongPathName (тесты). К предку поднимаемся только
// при «файл/путь не найден»; прочие ошибки (недоступная сеть, нет прав) — путь как есть, иначе
// на мёртвом томе был бы вызов на каждом уровне.
func longPathVia(p string, lookup func(string) (string, error)) string {
	if p == "" {
		return p
	}
	// Dir не снимает завершающий разделитель: «C:\PROGRA~1\нет\» дал бы «…\нет\нет».
	p = filepath.Clean(p)
	l, err := lookup(p)
	if err == nil {
		return l
	}
	if !errors.Is(err, syscall.ERROR_FILE_NOT_FOUND) && !errors.Is(err, syscall.ERROR_PATH_NOT_FOUND) {
		return p
	}
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	l = longPathVia(parent, lookup)
	if l == parent { // предок не изменился (нет такого пути или уже длинный)
		return p
	}
	return filepath.Join(l, filepath.Base(p))
}

// longName — GetLongPathName для существующего пути; err — ошибка Windows (в т. ч. «не найден»).
func longName(p string) (string, error) {
	in, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return p, err
	}
	size := uint32(syscall.MAX_PATH)
	for range 2 { // второй проход — если буфер был мал: тогда n — нужный размер
		buf := make([]uint16, size)
		n, err := syscall.GetLongPathName(in, &buf[0], size)
		if err != nil {
			return p, err
		}
		if n == 0 {
			return p, syscall.EINVAL
		}
		if n <= size {
			return syscall.UTF16ToString(buf[:n]), nil
		}
		size = n
	}
	return p, syscall.EINVAL
}
