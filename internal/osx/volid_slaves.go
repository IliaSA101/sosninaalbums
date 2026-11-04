//go:build linux

package osx

// Цепочка подчинённых устройств по sysfs (М1-остаток, 0.37): dm и md стоят поверх других
// блочных устройств, а список нижних лежит в /sys/class/block/<имя>/slaves/. Нужен, чтобы
// увидеть петлю под dm (LUKS-образ через udisksctl loop-setup + unlock).

import (
	"os"
	"path/filepath"
)

// Пределы обхода: sysfs читаем как чужие данные, ни глубина, ни ширина не доверены.
const (
	maxSlaveDepth = 16  // dm поверх dm поверх …; Stratis с кешем и шифрованием — 7 уровней
	maxSlaveNodes = 256 // устройств за один обход
	maxSlaveList  = 64  // подчинённых у одного устройства
)

// onLoop — под устройством name (имя записи в /sys/class/block) где-то лежит петля. Каталога sys
// нет, запись или slaves не читаются — петлю видно только по имени, как до этой проверки (иначе
// отказали бы всем без доступа к /sys). Но то, что прочитано и не сходится — цикл, глубже
// maxSlaveDepth, шире пределов, — отказ: доказать отсутствие петли нельзя.
func (d volDirs) onLoop(name string) bool {
	if d.sys == "" {
		return false
	}
	w := slaveWalk{root: filepath.Join(d.sys, "class", "block"), onPath: map[string]bool{}, done: map[string]bool{}}
	return w.loop(name, 0)
}

type slaveWalk struct {
	root         string
	onPath, done map[string]bool // на текущей ветке (цикл) и уже обойдённые (ромб — не цикл)
}

func (w *slaveWalk) loop(name string, depth int) bool {
	if isLoopDev(name) {
		return true
	}
	if w.onPath[name] || depth >= maxSlaveDepth || len(w.done) >= maxSlaveNodes {
		return true
	}
	if w.done[name] {
		return false
	}
	subs, ok := w.slaves(name)
	if !ok {
		return true
	}
	w.onPath[name] = true
	defer func() { delete(w.onPath, name); w.done[name] = true }()
	for _, s := range subs {
		if w.loop(s, depth+1) {
			return true
		}
	}
	return false
}

// slaves — имена подчинённых (симлинки не раскрываем: нужны только имена) и, у раздела ядра, его
// родитель: свой каталог slaves есть только у целого устройства, а раздел md127p1 стоит на md127
// (udisksctl mount /dev/md127p1 над mdadm-массивом из петли). Нет каталога или он не читается —
// подчинённых нет (ok); больше maxSlaveList или раздел без читаемого родителя — ok=false.
func (w *slaveWalk) slaves(name string) (subs []string, ok bool) {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return nil, true
	}
	dir := filepath.Join(w.root, name)
	if f, err := os.Open(filepath.Join(dir, "slaves")); err == nil {
		ents, _ := f.ReadDir(maxSlaveList + 1)
		_ = f.Close()
		if len(ents) > maxSlaveList {
			return nil, false
		}
		for _, e := range ents {
			subs = append(subs, e.Name())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "partition")); err != nil {
		return subs, true
	}
	// Запись раздела — симлинк …/<родитель>/<раздел>; «..» через Join сократился бы до перехода по ссылке.
	target, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, false
	}
	parent := filepath.Base(filepath.Dir(target))
	if parent == "." || parent == string(filepath.Separator) {
		return nil, false
	}
	return append(subs, parent), true
}
