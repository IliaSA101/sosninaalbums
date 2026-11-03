package osx

// remoteOS (macOS): statfs пути — MNT_LOCAL и тип из списка (remote.go, darwinRemote).

import "syscall"

func remoteOS(path string) (bool, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return true, err
	}
	name := make([]byte, 0, len(st.Fstypename))
	for _, c := range st.Fstypename {
		if c == 0 {
			break
		}
		name = append(name, byte(c))
	}
	return darwinRemote(st.Flags, string(name)), nil
}
