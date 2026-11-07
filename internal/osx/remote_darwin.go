package osx

// remoteOS (macOS): statfs пути — MNT_LOCAL и тип из списка (remote.go, darwinRemote).

import "syscall"

func remoteOS(path string) (bool, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return true, err
	}
	return darwinRemote(st.Flags, cString(st.Fstypename[:])), nil
}
