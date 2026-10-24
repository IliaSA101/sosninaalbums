package paths

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
)

// DirKey — ключ папки для «внутри»: Fold, Clean и разделитель на конце (как library.Under:
// регистр не важен на Windows и macOS, на Linux важен).
func DirKey(dir string) string {
	k := Fold(filepath.Clean(dir))
	if !strings.HasSuffix(k, string(filepath.Separator)) {
		k += string(filepath.Separator)
	}
	return k
}

// TopKeys — ключи DirKey папок без повторов и вложенных, по возрастанию: ни один не префикс
// другого, поэтому «внутри» решает ближайший снизу (UnderAny, свёртка источников 093).
func TopKeys(folders []string) []string {
	keys := make([]string, 0, len(folders))
	for _, f := range folders {
		keys = append(keys, DirKey(f))
	}
	slices.Sort(keys)
	top := keys[:0]
	for _, k := range keys {
		if len(top) == 0 || !strings.HasPrefix(k, top[len(top)-1]) {
			top = append(top, k)
		}
	}
	return top
}

// UnderAny — путь внутри любой из папок (как library.Under по каждой) за O(log папок): Under по
// каждой папке на каждый путь открывал чужой файл минутами.
func UnderAny(folders []string) func(path string) bool {
	top := TopKeys(folders)
	return func(path string) bool {
		p := Fold(filepath.Clean(path))
		i, found := slices.BinarySearch(top, p)
		return found || i > 0 && strings.HasPrefix(p, top[i-1])
	}
}

// Linked — папка dir — ссылка или junction (Lstat): сканер в такие подпапки не заходит
// (library.walkEntry), а из корня-ссылки не берёт ничего, поэтому Reaches сквозь них не идёт.
// Не узнать (ошибка Lstat, кроме «нет такого» на живом томе) — как ссылка: запись остаётся
// отдельным корнем. Иначе альбом, открытый при отключённом диске, свернул бы junction-запись
// в корень, а автосохранение убрало бы её навсегда (0.22-sec-1).
func Linked(dir string) bool {
	st, err := lstat(dir)
	if err == nil {
		return st.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return true
	}
	// Нет такого: на живом томе — папки правда нет, она не ссылка; тома нет — не узнать.
	root := filepath.VolumeName(dir) + string(filepath.Separator)
	if f := volumeOracle.Load(); f != nil {
		return !(*f)(root)
	}
	_, err = lstat(root)
	return err != nil
}

// lstat — os.Lstat для Linked; подменяется в тестах (отказ доступа, отключённый том).
var lstat = os.Lstat

// volumeOracle — «том на месте» для Linked; nil — Lstat корня тома.
var volumeOracle atomic.Pointer[func(root string) bool]

// SetVolumeOracle — только для тестов: live(root) — «том с корнем root на месте» вместо Lstat
// (реплей с вымышленным диском Q: не зависит от того, есть ли Q: на машине). Возвращает
// восстановление прежнего.
func SetVolumeOracle(live func(root string) bool) (restore func()) {
	old := volumeOracle.Swap(&live)
	return func() { volumeOracle.Store(old) }
}

// Reaches — обход папки root сканером дойдёт до path (093-4): path внутри root (как
// library.Under) и ни один элемент хвоста от root не начинается с «.» и не устройство
// (DeviceElem; у файла последний элемент — только устройство: точку-файл сканер читает), а
// папки хвоста (у папки — и она сама) — не ссылки (linked; nil — не проверять). Иначе запись —
// отдельный корень: свернуть её в root — потерять её фото. Сама root внутри себя не считается.
// Корень-ссылка не накрывает ничего: сканер из неё не берёт ни файла (library.walk), а свёртка в
// неё опустошила бы ленту (decisions 0.22-sec-1).
func Reaches(root, path string, file bool, linked func(dir string) bool) bool {
	root, path = filepath.Clean(root), filepath.Clean(path)
	if !strings.HasPrefix(Fold(path), DirKey(root)) {
		return false
	}
	// Элементы, а не срез строки: нижний регистр меняет длину в байтах у части букв.
	sep := string(filepath.Separator)
	tail := strings.Split(path, sep)[len(strings.Split(strings.TrimRight(root, sep), sep)):]
	if len(tail) == 0 || tail[0] == "" || linked != nil && linked(root) {
		return false
	}
	dir := root
	for i, el := range tail {
		last := file && i == len(tail)-1
		if DeviceElem(el) || !last && strings.HasPrefix(el, ".") {
			return false
		}
		if last {
			break
		}
		dir = filepath.Join(dir, el)
		if linked != nil && linked(dir) {
			return false
		}
	}
	return true
}

// Cover — Reaches для многих путей сразу: решает ближайшая папка списка выше пути — дальняя
// дошла бы только через неё (её хвост длиннее). Ссылки проверяются только у путей, над которыми
// есть папка списка (их мало), каждая папка — один раз.
type Cover struct {
	roots  map[string]string // DirKey → папка как в списке (первая)
	linked func(dir string) bool
}

// NewCover — Cover папок folders; linked — как в Reaches (nil — не проверять).
func NewCover(folders []string, linked func(dir string) bool) *Cover {
	c := &Cover{roots: make(map[string]string, len(folders))}
	for _, f := range folders {
		if k := DirKey(f); c.roots[k] == "" {
			c.roots[k] = f
		}
	}
	if linked != nil {
		memo := map[string]bool{}
		c.linked = func(dir string) bool {
			k := Fold(dir)
			v, ok := memo[k]
			if !ok {
				v = linked(dir)
				memo[k] = v
			}
			return v
		}
	}
	return c
}

// Root — папка списка, через которую обход дойдёт до path; ok = false — path вне папок списка
// или отдельный корень (Reaches). Папка списка внутри себя не считается.
func (c *Cover) Root(path string, file bool) (root string, ok bool) {
	if len(c.roots) == 0 {
		return "", false
	}
	path = filepath.Clean(path)
	for prev, d := path, filepath.Dir(path); d != prev; prev, d = d, filepath.Dir(d) {
		if r, in := c.roots[DirKey(d)]; in {
			return r, Reaches(r, path, file, c.linked)
		}
	}
	return "", false
}

// HasFile — обход какой-либо папки списка дойдёт до файла path.
func (c *Cover) HasFile(path string) bool {
	_, ok := c.Root(path, true)
	return ok
}
