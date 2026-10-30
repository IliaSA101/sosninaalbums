package osx

// Разбор тома по таблице монтирования Linux (153, design §4.2). Чистые функции с каталогами в
// параметрах: тест подставляет поддельные by-uuid и by-label, а volid_linux.go — настоящие.

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// volDirs — каталоги устройств: dev (источники монтирования), by-uuid и by-label (симлинки на них).
type volDirs struct{ dev, uuid, label string }

// volFromMounts — том абсолютного пути по таблице: самая длинная точка-префикс, источник
// «/dev/…» → настоящее устройство → UUID. Нет UUID (tmpfs, overlay, ZFS, сеть) — errNoVolID.
func volFromMounts(ms []mount, p string, d volDirs) (Vol, error) {
	p = path.Clean(p)
	m, ok := mountOf(ms, p)
	if !ok {
		return Vol{}, errNoVolID
	}
	dev, ok := d.device(m)
	if !ok {
		return Vol{}, errNoVolID
	}
	uuid := d.index(d.uuid)[dev]
	if !validUUID(uuid) {
		return Vol{}, errNoVolID
	}
	rest := p
	if m.point != "/" {
		rest = strings.TrimPrefix(p, m.point)
	}
	return Vol{
		Root:  m.point,
		ID:    "uuid:" + uuid,
		Label: cleanLabel(unescapeLabel(d.index(d.label)[dev])),
		Rel:   strings.TrimPrefix(path.Join("/", m.root, rest), "/"),
	}, nil
}

// device — настоящее блочное устройство источника «/dev/…» (симлинки раскрыты: /dev/mapper/x →
// /dev/dm-0). Сетевые и виртуальные источники («srv:/export», «tmpfs», «pool/ds») — не годны.
// Источник пишет тот, кто монтирует: fuse подставит «/dev/sdb1» в fsname, поэтому тип ФС — только
// локальный (localFS, без fuse.*), а петля /dev/loop* — отказ: образ с UUID жертвы монтирует любой
// пользователь через udisks (М1, 0.35-155).
func (d volDirs) device(m mount) (string, bool) {
	if !slices.Contains(localFS, m.fstype) || !strings.HasPrefix(m.source, d.dev+"/") {
		return "", false
	}
	dev, err := filepath.EvalSymlinks(m.source)
	if err != nil || isLoopDev(dev) {
		return "", false
	}
	return dev, true
}

// isLoopDev — «loopN» и раздел петли «loopNpM» (loop-control и loopback — не блочные петли).
func isLoopDev(dev string) bool {
	n, ok := strings.CutPrefix(path.Base(filepath.ToSlash(dev)), "loop")
	if !ok {
		return false
	}
	num, part, hasPart := strings.Cut(n, "p")
	return allDigits(num) && (!hasPart || allDigits(part))
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// index — настоящее устройство → имя записи каталога (UUID или метка). Нет каталога — пусто.
func (d volDirs) index(dir string) map[string]string {
	out := map[string]string{}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for i, e := range ents {
		if i >= maxVolEntry {
			break
		}
		if dev, err := filepath.EvalSymlinks(filepath.Join(dir, e.Name())); err == nil {
			out[dev] = e.Name()
		}
	}
	return out
}

// pathFromMounts — путь rel на томе id: строка таблицы того же тома, чей корень поддерева
// охватывает rel (самый глубокий корень — самый точный). Нет такой — ErrVolGone.
func pathFromMounts(ms []mount, id, rel string, d volDirs) (p, label string, err error) {
	uuids, labels := d.index(d.uuid), d.index(d.label)
	bestRoot := -1
	for _, m := range ms {
		dev, ok := d.device(m)
		if !ok || !validUUID(uuids[dev]) || "uuid:"+uuids[dev] != id {
			continue
		}
		root := strings.Trim(path.Clean("/"+m.root), "/")
		var tail string
		switch {
		case root == "":
			tail = rel
		case rel == root:
			tail = ""
		case strings.HasPrefix(rel, root+"/"):
			tail = rel[len(root)+1:]
		default:
			continue
		}
		if len(root) > bestRoot {
			bestRoot, p = len(root), path.Join(m.point, tail)
			label = cleanLabel(unescapeLabel(labels[dev]))
		}
	}
	if bestRoot < 0 {
		return "", "", ErrVolGone
	}
	return p, label, nil
}

// validUUID — UUID ФС: буквы, цифры, «-» и «_» (ext4 — 36 знаков, FAT — «1A2B-3C4D»).
func validUUID(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for _, r := range s {
		ok := r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-' || r == '_'
		if !ok {
			return false
		}
	}
	return true
}

// unescapeLabel — udev пишет в by-label «\x20» вместо пробела и прочих знаков.
func unescapeLabel(s string) string {
	if !strings.Contains(s, `\x`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) && s[i+1] == 'x' {
			if n, err := strconv.ParseUint(s[i+2:i+4], 16, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
