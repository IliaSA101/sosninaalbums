package paths

import (
	"path/filepath"
	"runtime"
	"strings"
)

// MaxLinkHops — длиннее цепочка ссылок не бывает у своих файлов; дальше — отказ.
const MaxLinkHops = 8

// LinkTargetLocal — цель ссылки лежит на локальном диске: абсолютная (и всё с корнем или двоеточием
// на Windows) проходит CheckLocalPath; относительная — без устройств в элементах (DeviceElem) и только
// из папки dir на локальном диске.
func LinkTargetLocal(dir, target string) bool {
	if target == "" || strings.ContainsRune(target, 0) {
		return false
	}
	if filepath.IsAbs(target) || runtime.GOOS == "windows" && (IsSep(target[0]) || strings.ContainsRune(target, ':')) {
		return CheckLocalPath(target) == nil
	}
	for _, el := range strings.FieldsFunc(target, IsSepRune) {
		if DeviceElem(el) {
			return false
		}
	}
	return runtime.GOOS != "windows" || CheckLocalPath(dir) == nil
}

// LinkChainLocal — у ссылки p каждая цель цепочки (до MaxLinkHops) локальна (LinkTargetLocal). Цель проверяется
// по записи в ссылке (readLink — os.Readlink или подмена теста), на диск по ней не ходим, пока она не проверена.
// Не ссылка (readLink с ошибкой) — конец цепочки, true: дальше решают EvalSymlinks и Stat.
func LinkChainLocal(p string, readLink func(string) (string, error)) bool {
	for range MaxLinkHops {
		raw, err := readLink(p)
		if err != nil {
			return true
		}
		if !LinkTargetLocal(filepath.Dir(p), raw) {
			return false
		}
		if !filepath.IsAbs(raw) {
			raw = filepath.Join(filepath.Dir(p), raw)
		}
		p = raw
	}
	return false
}
