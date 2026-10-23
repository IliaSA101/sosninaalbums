package salbum

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Версия программы в файле и имя копии старого формата (098 design §6, decisions 098-13, 098-22).

// lastApps — последняя версия программы, писавшая формат v (индекс — номер формата). По git:
// FormatVersion на тегах выпусков; v1–v3 — до тегов, граница по истории коммитов. Строка
// текущего формата добавляется вместе со следующим номером (TestLastAppTable).
var lastApps = [...]string{
	1: "0.3", 2: "0.3", 3: "0.3", 4: "0.6", 5: "0.9", 6: "0.10", 7: "0.11", 8: "0.12",
	9: "0.13", 10: "0.14", 11: "0.15", 12: "0.17", 13: "0.19", 14: "0.20", 15: "0.21", 16: "0.22", 17: "0.23",
}

// LastApp — последняя версия программы (major.minor), писавшая формат v; "" — v ≥ FormatVersion
// или вне таблицы. Файл без номера (0) читается как v1.
func LastApp(v int) string {
	v = max(v, 1)
	if v >= model.FormatVersion || v >= len(lastApps) {
		return ""
	}
	return lastApps[v]
}

// maxApp — предел строки app: версия вида «0.23.1-rc2+abc» короче.
const maxApp = 32

// appVer — версия этой программы для поля app; "" — поле не пишется (тесты без SetAppVersion).
// Атомарно: App задаёт её в startup, а запись идёт из горутин автосохранения.
var appVer atomic.Pointer[string]

// SetAppVersion — версия программы для поля app каждого записанного файла (App в startup, как
// SetLibFinder). Негодная строка (cleanApp) — поле не пишется.
func SetAppVersion(v string) {
	v = cleanApp(v)
	appVer.Store(&v)
}

// curApp — версия для записи; "" — не писать.
func curApp() string {
	if v := appVer.Load(); v != nil {
		return *v
	}
	return ""
}

// cleanApp — строка версии к допустимой: ≤ maxApp знаков [0-9A-Za-z.+-], иначе "". Поле из
// файла — чужие данные: в текст ошибки идёт только такая строка.
func cleanApp(s string) string {
	if len(s) > maxApp {
		return ""
	}
	for _, c := range s {
		ok := c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '.' || c == '+' || c == '-'
		if !ok {
			return ""
		}
	}
	return s
}

// appField — поле app файла: мягкий разбор, не строка или негодная строка — "" без ошибки
// (поле справочное, из-за него файл не «повреждён»).
type appField string

// UnmarshalJSON — строка через cleanApp, прочее — "".
func (a *appField) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) != nil {
		s = ""
	}
	*a = appField(cleanApp(s))
	return nil
}

// NewerError — файл новее программы (ErrNewer) с версией записавшей программы, если она есть
// в файле: App называет её в тексте («Альбом сохранён в версии 0.25»). errors.Is(err, ErrNewer)
// — истина, прежние проверки работают.
type NewerError struct {
	Version int    // номер формата файла
	App     string // версия программы из поля app; "" — неизвестна
}

func (e *NewerError) Error() string { return ErrNewer.Error() }

// Unwrap — ErrNewer.
func (e *NewerError) Unwrap() error { return ErrNewer }

// backupTail — хвост имени копии: « (для 0.22)».
func backupTail(from int) string { return " (для " + LastApp(from) + ")" }

// BackupName — имя копии файла path формата from: «<папка>/<стем> (для 0.22).salbum», k ≥ 2 —
// «… (для 0.22) k.salbum». Стем — имя без «.salbum» (регистр не важен) и без хвоста
// « (для X.Y)» / « (для X.Y) n», если открыли саму копию. Версия — только из таблицы LastApp, не
// из поля app файла: строка из файла в имени файла — путь для «../» (decisions 098-22).
func BackupName(path string, from, k int) string {
	dir, name := filepath.Split(path)
	if ext := filepath.Ext(name); strings.EqualFold(ext, projectExt) {
		name = name[:len(name)-len(ext)]
	}
	name = stripBackupTail(name)
	name += backupTail(from)
	if k >= 2 {
		name += fmt.Sprintf(" %d", k)
	}
	return filepath.Join(dir, name+projectExt)
}

// projectExt — расширение файла альбома.
const projectExt = ".salbum"

// stripBackupTail — стем без хвоста « (для X.Y)» или « (для X.Y) n» (n — число): копию копии
// называем от исходного имени, а не «… (для 0.22) (для 0.22)».
func stripBackupTail(stem string) string {
	i := strings.LastIndex(stem, " (для ")
	if i < 0 {
		return stem
	}
	rest := stem[i+len(" (для "):]
	ver, after, ok := strings.Cut(rest, ")")
	if !ok || !isVer(ver) {
		return stem
	}
	if after != "" && (after[0] != ' ' || !isNum(after[1:])) {
		return stem
	}
	return stem[:i]
}

// isVer — строка вида «0.22»: две группы цифр через точку.
func isVer(s string) bool {
	a, b, ok := strings.Cut(s, ".")
	return ok && isNum(a) && isNum(b)
}

// isNum — непустая строка цифр.
func isNum(s string) bool {
	return s != "" && strings.Trim(s, "0123456789") == ""
}
