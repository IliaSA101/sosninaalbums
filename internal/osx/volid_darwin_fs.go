//go:build darwin

package osx

// statfs и getfsstat macOS (160 Т4): точки монтирования и их флаги. Без cgo, только std; решения по
// ответам — volid_mac.go. Файл собирается и без cgo (vet на Linux), но нужен лишь volid_darwin.go.

import "syscall"

// statMac — запись statfs для пути p (путь должен существовать).
func statMac(p string) (macStat, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(p, &st); err != nil {
		return macStat{}, err
	}
	return macStatOf(&st), nil
}

// listMacMounts — смонтированные тома (MNT_NOWAIT: спящие диски не будим), не больше maxMacMounts.
func listMacMounts() ([]macStat, error) {
	n, err := syscall.Getfsstat(nil, mntNoWait)
	if err != nil {
		return nil, err
	}
	buf := make([]syscall.Statfs_t, min(n, maxMacMounts)+8) // запас: тома могли прибавиться между вызовами
	n, err = syscall.Getfsstat(buf, mntNoWait)
	if err != nil {
		return nil, err
	}
	out := make([]macStat, 0, min(n, len(buf)))
	for i := 0; i < n && i < len(buf); i++ {
		out = append(out, macStatOf(&buf[i]))
	}
	return out, nil
}

func macStatOf(st *syscall.Statfs_t) macStat {
	return macStat{Point: cString(st.Mntonname[:]), Fstype: cString(st.Fstypename[:]), Flags: st.Flags}
}

// cString — C-строка из массива знаков: до первого нуля.
func cString(a []int8) string {
	b := make([]byte, 0, len(a))
	for _, c := range a {
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	return string(b)
}
