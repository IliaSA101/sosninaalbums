package osx

// Разбор /proc/self/mountinfo (Linux): тип файловой системы пути и точки монтирования. Разбор не
// привязан к ОС, чтобы тесты на образцах строк шли везде; читает файл MountPoints только на Linux.

import (
	"bufio"
	"io"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
)

// Пределы чтения: на машине с тысячами монтирований (snap, контейнеры) файл — сотни КБ; больше
// — чужое или битое, том считаем сетевым.
const (
	maxMountinfo     = 8 << 20
	maxMountinfoLine = 64 << 10
)

// mountinfoFile — путь файла; тест подменяет (нет файла — ошибка).
var mountinfoFile = "/proc/self/mountinfo"

// mount — строка mountinfo: точка монтирования, тип ФС (у FUSE — с подтипом: «fuse.sshfs»), корень
// поддерева внутри ФС (не «/» у bind-монтирования и подтома btrfs) и источник («/dev/sdb1»).
type mount struct{ point, fstype, root, source string }

// localFS — типы, которые считаем локальными (design 150 §4.4). fuseblk — FUSE поверх блочного
// устройства (ntfs-3g, exfat-fuse); прочие fuse.* (sshfs, rclone, gvfs) — сетевые.
var localFS = []string{"ext2", "ext3", "ext4", "xfs", "btrfs", "f2fs", "vfat", "exfat", "ntfs3", "fuseblk", "zfs"}

// parseMountinfo читает строки `ID PARENT MAJ:MIN ROOT POINT OPTS [опции…] - FSTYPE SOURCE …`.
// Негодные строки пропускаются; слишком большой файл или длинная строка — ошибка.
func parseMountinfo(r io.Reader) ([]mount, error) {
	lr := &io.LimitedReader{R: r, N: maxMountinfo + 1}
	sc := bufio.NewScanner(lr)
	sc.Buffer(make([]byte, 0, 4096), maxMountinfoLine)
	var out []mount
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		sep := slices.Index(f, "-")
		if sep < 6 || sep+1 >= len(f) {
			continue
		}
		m := mount{point: unescapeMount(f[4]), fstype: f[sep+1], root: unescapeMount(f[3])}
		if sep+2 < len(f) {
			m.source = unescapeMount(f[sep+2])
		}
		out = append(out, m)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if lr.N <= 0 {
		return nil, io.ErrUnexpectedEOF
	}
	return out, nil
}

// unescapeMount — в mountinfo пробел, табуляция, перевод строки и «\» пишутся как \040, \011,
// \012, \134.
func unescapeMount(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// readMounts — таблица монтирования этой машины; вне Linux её нет (nil, nil).
func readMounts() ([]mount, error) {
	if runtime.GOOS != "linux" {
		return nil, nil
	}
	f, err := os.Open(mountinfoFile) //nolint:gosec // G304: файл ядра, путь не от пользователя
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // чтение
	return parseMountinfo(f)
}

// MountPoints — точки монтирования (Linux; на прочих ОС nil). Ошибка чтения — вызывающий считает
// путь небезопасным, а не «монтирований нет».
func MountPoints() ([]string, error) {
	ms, err := readMounts()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.point)
	}
	return out, nil
}

// fsTypeOf — тип ФС точки монтирования с длиннейшим префиксом path; при равных точках (том
// смонтирован поверх другого) побеждает последняя строка. Нет точки — ok = false.
func fsTypeOf(ms []mount, path string) (fstype string, ok bool) {
	m, ok := mountOf(ms, path)
	return m.fstype, ok
}

// mountOf — строка с длиннейшей точкой-префиксом path; при равных побеждает последняя.
func mountOf(ms []mount, path string) (best mount, ok bool) {
	n := -1
	for _, m := range ms {
		if !hasMountPrefix(path, m.point) || len(m.point) < n {
			continue
		}
		n, best, ok = len(m.point), m, true
	}
	return best, ok
}

// hasMountPrefix — path совпадает с точкой или лежит внутри (граница по «/»: «/a/b» не в «/a/bc»).
func hasMountPrefix(path, point string) bool {
	if point == "/" {
		return strings.HasPrefix(path, "/")
	}
	return path == point || strings.HasPrefix(path, point+"/")
}

// remoteByMounts — путь на нелокальной ФС по таблице; нет точки — «сетевой».
func remoteByMounts(ms []mount, path string) bool {
	t, ok := fsTypeOf(ms, path)
	return !ok || !slices.Contains(localFS, t)
}
