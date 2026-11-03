package osx

// volumeOS (Linux): /proc/self/mountinfo + /dev/disk/by-uuid (volid_mounts.go).

import (
	"errors"
	"strings"
)

var sysVolDirs = volDirs{dev: "/dev", uuid: "/dev/disk/by-uuid", label: "/dev/disk/by-label"}

func isAbsVol(p string) bool { return strings.HasPrefix(p, "/") }

func volumeOS(p string) (Vol, error) {
	ms, err := readMounts()
	if err != nil {
		return Vol{}, err
	}
	if len(ms) == 0 {
		return Vol{}, errors.New("таблица монтирования пуста")
	}
	return volFromMounts(ms, p, sysVolDirs)
}

func volumePathOS(id, rel string) (string, string, error) {
	ms, err := readMounts()
	if err != nil {
		return "", "", err
	}
	return pathFromMounts(ms, id, rel, sysVolDirs)
}
