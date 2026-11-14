//go:build darwin && cgo

package osx

// volumeOS (macOS, 160 Т4, design §5): getattrlist(ATTR_VOL_UUID | ATTR_VOL_NAME) по точке /Volumes/<X>.
// Привязка только к съёмным и подключённым томам; загрузочный и Data — без неё. cgo нужен
// одному вызову: в x/sys и syscall getattrlist нет (0.36-160-7), разбор ответа — на Go (volid_mac.go).

/*
#include <errno.h>
#include <stdlib.h>
#include <string.h>
#include <sys/attr.h>
#include <sys/types.h>
#include <unistd.h>

// sa_volattr кладёт в buf ответ getattrlist по тому path; 0 — успех, иначе errno.
static int sa_volattr(const char *path, void *buf, size_t n) {
	struct attrlist al;
	memset(&al, 0, sizeof al);
	al.bitmapcount = ATTR_BIT_MAP_COUNT;
	al.volattr = ATTR_VOL_INFO | ATTR_VOL_NAME | ATTR_VOL_UUID;
	if (getattrlist(path, &al, buf, n, 0) != 0) {
		return errno;
	}
	return 0;
}
*/
import "C"

import (
	"strings"
	"syscall"
	"unsafe"
)

// volidCgo — номер тома macOS читает cgo-вариант: здесь он есть.
const volidCgo = true

func isAbsVol(p string) bool { return strings.HasPrefix(p, "/") }

func volumeOS(p string) (Vol, error) {
	st, err := macStatUp(p, statMac)
	if err != nil {
		return Vol{}, err
	}
	return macVolume(p, st, readVolAttr)
}

func volumePathOS(id, rel string) (string, string, error) {
	ms, err := listMacMounts()
	if err != nil {
		return "", "", err
	}
	return macPathFromMounts(ms, id, rel, readVolAttr)
}

// readVolAttr — UUID и метка тома с точкой монтирования point.
func readVolAttr(point string) (volAttr, error) {
	if strings.IndexByte(point, 0) >= 0 {
		return volAttr{}, errNoVolID
	}
	cp := C.CString(point)
	defer C.free(unsafe.Pointer(cp))
	buf := make([]byte, maxVolAttrBuf)
	if e := C.sa_volattr(cp, unsafe.Pointer(&buf[0]), C.size_t(len(buf))); e != 0 {
		return volAttr{}, syscall.Errno(e)
	}
	return parseVolAttr(buf)
}
