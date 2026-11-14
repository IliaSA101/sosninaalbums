//go:build linux

package osx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// sysDirs — fakeVolDirs с петлёй loop3 и поддельным /sys/class/block: stack["dm-0"] = {"loop3"}
// кладёт slaves/loop3 (симлинк в виртуальную часть, как в настоящем sysfs). Устройства dm-N
// получают файл в dev и UUID в by-uuid — их источник годен, если под ними нет петли.
func sysDirs(t *testing.T, stack map[string][]string) volDirs {
	t.Helper()
	d := loopDirs(t)
	d.sys = filepath.Join(filepath.Dir(d.dev), "sys")
	for name, subs := range stack {
		dir := filepath.Join(d.sys, "class", "block", name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if subs != nil { // nil — устройство без каталога slaves, как sda1; пустой — каталог без записей
			if err := os.MkdirAll(filepath.Join(dir, "slaves"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		for _, s := range subs {
			// Цель может не существовать: имена читаем, а ссылки не раскрываем.
			if err := os.Symlink(filepath.Join("..", "..", "..", "..", "virtual", "block", s), filepath.Join(dir, "slaves", s)); err != nil {
				t.Fatal(err)
			}
		}
		devNode(t, d, name)
	}
	return d
}

// devNode — файл в dev и UUID в by-uuid: источник name годен, если под ним нет петли.
func devNode(t *testing.T, d volDirs, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(d.dev, name), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", name), filepath.Join(d.uuid, "DM00-"+name)); err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}
}

// sysPart — раздел part целого устройства parent в стиле настоящего sysfs: записи class/block/* —
// симлинки на devices/virtual/block/<parent>[/<part>], файл partition только у раздела, а slaves —
// только у родителя (parentSubs == nil — без каталога slaves, как у nvme0n1).
func sysPart(t *testing.T, d volDirs, parent, part string, parentSubs []string) {
	t.Helper()
	dev := filepath.Join(d.sys, "devices", "virtual", "block", parent)
	cls := filepath.Join(d.sys, "class", "block")
	for _, dir := range []string{filepath.Join(dev, part), cls} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if parentSubs != nil {
		if err := os.MkdirAll(filepath.Join(dev, "slaves"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range parentSubs {
		if err := os.Symlink(filepath.Join("..", "..", "..", "..", "virtual", "block", s), filepath.Join(dev, "slaves", s)); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dev, part, "partition"), []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{parent: parent, part: filepath.Join(parent, part)} {
		if err := os.Symlink(filepath.Join("..", "..", "devices", "virtual", "block", target), filepath.Join(cls, name)); err != nil {
			t.Fatal(err)
		}
	}
	devNode(t, d, part)
}

func devOK(d volDirs, name string) bool {
	_, ok := d.device(mount{source: d.dev + "/" + name, fstype: "ext4"})
	return ok
}

// TestDeviceSlaves — цепочка подчинённых: петля под dm любой глубины — отказ, обычный диск — годен.
func TestDeviceSlaves(t *testing.T) {
	long := map[string][]string{"sda2": nil}
	for i := 0; i <= maxSlaveDepth+1; i++ { // dm-0 → … → dm-N → sda2: глубже предела
		next := "sda2" // петли нет: отказ даёт только глубина
		if i <= maxSlaveDepth {
			next = fmt.Sprintf("dm-%d", i+1)
		}
		long[fmt.Sprintf("dm-%d", i)] = []string{next}
	}
	wide := make([]string, maxSlaveList+1)
	for i := range wide {
		wide[i] = fmt.Sprintf("sdx%d", i)
	}
	// Дерево 5 уровней по 4 подчинённых без петли: глубина и ширина в пределах, узлов больше maxSlaveNodes.
	big, next := map[string][]string{}, 0
	var grow func(name string, level int)
	grow = func(name string, level int) {
		if level == 5 {
			return // лист: записи в sysfs нет
		}
		for i := 0; i < 4; i++ {
			next++
			sub := fmt.Sprintf("dm-%d", next)
			big[name] = append(big[name], sub)
			grow(sub, level+1)
		}
	}
	big["dm-0"] = nil
	grow("dm-0", 0)
	for _, c := range []struct {
		name  string
		stack map[string][]string
		dev   string
		want  bool
	}{
		{"dm → loop", map[string][]string{"dm-0": {"loop3"}}, "dm-0", false},
		{"dm → dm → loop", map[string][]string{"dm-1": {"dm-0"}, "dm-0": {"loop3"}}, "dm-1", false},
		{"dm → раздел петли", map[string][]string{"dm-0": {"loop3p1"}}, "dm-0", false},
		{"LUKS на диске", map[string][]string{"dm-0": {"sda2"}, "sda2": nil}, "dm-0", true},
		{"dm → dm → диск", map[string][]string{"dm-1": {"dm-0"}, "dm-0": {"sda2"}}, "dm-1", true},
		{"md из диска и петли", map[string][]string{"md127": {"sda2", "loop3"}}, "md127", false},
		{"ромб без петли", map[string][]string{"dm-3": {"dm-1", "dm-2"}, "dm-1": {"dm-0"}, "dm-2": {"dm-0"}, "dm-0": {"sda2"}}, "dm-3", true},
		{"цикл: отказ, доказать нельзя", map[string][]string{"dm-0": {"dm-1"}, "dm-1": {"dm-0"}}, "dm-0", false},
		{"цикл на себя", map[string][]string{"dm-0": {"dm-0"}}, "dm-0", false},
		{"глубже предела", long, "dm-0", false},
		{"больше узлов, чем maxSlaveNodes", big, "dm-0", false},
		{"шире предела", map[string][]string{"dm-0": wide}, "dm-0", false},
		{"sda2 без каталога slaves", map[string][]string{"sda2": nil}, "sda2", true},
		{"sda2 без записи в sys", map[string][]string{"sdb1": nil}, "sda2", true},
		{"пустой slaves", map[string][]string{"dm-0": {}}, "dm-0", true},
	} {
		d := sysDirs(t, c.stack)
		if got := devOK(d, c.dev); got != c.want {
			t.Errorf("%s: device = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestDevicePartition — раздел ядра обходит и родителя: у md127p1 своего slaves нет.
func TestDevicePartition(t *testing.T) {
	for _, c := range []struct {
		name, parent, part string
		subs               []string
		want               bool
	}{
		{"md127p1 над петлёй", "md127", "md127p1", []string{"loop0"}, false},
		{"md127p1 над диском и петлёй", "md127", "md127p1", []string{"sda2", "loop0"}, false},
		{"md127p1 над диском", "md127", "md127p1", []string{"sda2"}, true},
		{"sda1, пустой slaves", "sda", "sda1", []string{}, true},
		{"nvme0n1p2, slaves нет", "nvme0n1", "nvme0n1p2", nil, true},
	} {
		d := sysDirs(t, nil)
		sysPart(t, d, c.parent, c.part, c.subs)
		if got := devOK(d, c.part); got != c.want {
			t.Errorf("%s: device = %v, want %v", c.name, got, c.want)
		}
	}
	// Раздел с файлом partition, но без родителя в пути (запись в корне sysfs): доказать нельзя — отказ.
	d := sysDirs(t, nil)
	if err := os.MkdirAll(filepath.Join(d.sys, "class", "block", "md127p1"), 0o755); err == nil {
		_ = os.WriteFile(filepath.Join(d.sys, "class", "block", "md127p1", "partition"), []byte("1\n"), 0o600)
		devNode(t, d, "md127p1")
		if !devOK(d, "md127p1") { // родитель «block»: у него ни slaves, ни partition — петли нет
			t.Error("раздел-каталог: отклонён")
		}
	}
}

// TestDeviceNoSys — нет /sys (или поле пусто): петлю видно только по имени, как до проверки цепочки.
func TestDeviceNoSys(t *testing.T) {
	d := sysDirs(t, map[string][]string{"dm-0": {"loop3"}})
	for _, sys := range []string{"", filepath.Join(d.dev, "нет-sys")} {
		d.sys = sys
		if !devOK(d, "dm-0") {
			t.Errorf("sys=%q: dm-0 отклонён без чтения sysfs", sys)
		}
		if devOK(d, "loop3") {
			t.Errorf("sys=%q: loop3 принят", sys)
		}
	}
	// sys есть, а slaves — не каталог (нечитаем): подчинённых нет, как без sysfs.
	d = sysDirs(t, map[string][]string{"dm-0": nil})
	if err := os.WriteFile(filepath.Join(d.sys, "class", "block", "dm-0", "slaves"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if !devOK(d, "dm-0") {
		t.Error("slaves-файл: dm-0 отклонён")
	}
}

// TestVolDmOverLoop — сквозь volFromMounts и pathFromMounts: LUKS-образ с UUID жертвы не даёт ID
// и не находится по нему, а настоящий LUKS на диске — даёт.
func TestVolDmOverLoop(t *testing.T) {
	d := sysDirs(t, map[string][]string{"dm-0": {"loop3"}, "dm-1": {"dm-2"}, "dm-2": {"sda2"}})
	ms := parseForge(t, d, "50 22 253:0 / /mnt/img rw - ext4 DEV/dm-0 rw\n51 22 253:1 / /mnt/luks rw - ext4 DEV/dm-1 rw\n")
	if v, err := volFromMounts(ms, "/mnt/img/x", d); !errors.Is(err, errNoVolID) {
		t.Errorf("образ: %+v, %v; want errNoVolID", v, err)
	}
	if p, _, err := pathFromMounts(ms, "uuid:DM00-dm-0", "x", d); !errors.Is(err, ErrVolGone) {
		t.Errorf("образ по ID: %q, %v; want ErrVolGone", p, err)
	}
	if v, err := volFromMounts(ms, "/mnt/luks/x", d); err != nil || v.ID != "uuid:DM00-dm-1" {
		t.Errorf("LUKS на диске: %+v, %v", v, err)
	}
}
