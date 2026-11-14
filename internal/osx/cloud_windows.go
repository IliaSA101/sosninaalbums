package osx

// Облака на Windows (160, design §4): реестр — SyncRootManager (Cloud Files API: OneDrive, Dropbox, Google Drive и
// другие потоковые клиенты) и закреплённые папки Проводника (Яндекс.Диск 4); виртуальные облачные диски — по метке
// и ФС тома. Реестр и SID — std syscall: x/sys/windows в osx запрещён (depguard strict). Всё только на чтение.

import (
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// Пределы разбора реестра: чужой куст не должен занять поток надолго.
const (
	maxRegKeys  = 2048 // подключей одного раздела
	maxRegValue = 4096 // байт значения: путь в UTF-16 (2048 знаков)
	regNoMore   = syscall.Errno(259)
)

const (
	syncRootsKey  = `SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\SyncRootManager`
	clsidKey      = `Software\Classes\CLSID`
	pinnedValue   = "System.IsPinnedToNameSpaceTree"
	targetValue   = "TargetFolderPath"
	instanceProps = `Instance\InitPropertyBag`
)

// openCloudFile — на Windows ссылки отсекает Lstat, FIFO нет.
func openCloudFile(path string) (*os.File, error) { return os.Open(path) }

func cloudRootsOS(string, func(string) error) []string {
	return append(regSyncRoots(), regPinnedRoots()...)
}

// regOpen открывает раздел на чтение в 64-разрядном представлении; ok = false — нет раздела или доступа.
func regOpen(parent syscall.Handle, path string) (syscall.Handle, bool) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	var h syscall.Handle
	if syscall.RegOpenKeyEx(parent, p, 0, syscall.KEY_READ|syscall.KEY_WOW64_64KEY, &h) != nil {
		return 0, false
	}
	return h, true
}

// regSubkeys — имена подразделов (не больше maxRegKeys).
func regSubkeys(h syscall.Handle) []string {
	var out []string
	for i := uint32(0); i < maxRegKeys; i++ {
		var buf [260]uint16
		n := uint32(len(buf))
		err := syscall.RegEnumKeyEx(h, i, &buf[0], &n, nil, nil, nil, nil)
		if err == regNoMore {
			break
		}
		if err != nil {
			continue // слишком длинное имя и тому подобное: следующий
		}
		out = append(out, syscall.UTF16ToString(buf[:n]))
	}
	return out
}

// regValue — значение: тип и байты (не больше maxRegValue); ok = false — нет значения или оно крупнее предела.
func regValue(h syscall.Handle, name string) (typ uint32, data []byte, ok bool) {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, nil, false
	}
	var size uint32
	if syscall.RegQueryValueEx(h, p, nil, &typ, nil, &size) != nil || size == 0 || size > maxRegValue {
		return 0, nil, false
	}
	data = make([]byte, size)
	if syscall.RegQueryValueEx(h, p, nil, &typ, &data[0], &size) != nil || int(size) > len(data) {
		return 0, nil, false
	}
	return typ, data[:size], true
}

// regString — строковое значение REG_SZ (REG_EXPAND_SZ не раскрываем: путь с «%…%» не абсолютен и отсеется).
func regString(h syscall.Handle, name string) (string, bool) {
	typ, data, ok := regValue(h, name)
	if !ok || (typ != syscall.REG_SZ && typ != syscall.REG_EXPAND_SZ) || len(data) < 2 {
		return "", false
	}
	u := unsafe.Slice((*uint16)(unsafe.Pointer(&data[0])), len(data)/2) //nolint:gosec // G103: байты значения как UTF-16, длина по размеру
	return syscall.UTF16ToString(u), true
}

// regDword — значение REG_DWORD.
func regDword(h syscall.Handle, name string) (uint32, bool) {
	typ, data, ok := regValue(h, name)
	if !ok || typ != syscall.REG_DWORD || len(data) != 4 {
		return 0, false
	}
	return uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24, true
}

// currentSID — SID пользователя процесса в виде строки («S-1-5-21-…»); "" — не вышло.
func currentSID() string {
	tok, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return ""
	}
	defer func() { _ = tok.Close() }()
	u, err := tok.GetTokenUser()
	if err != nil {
		return ""
	}
	s, err := u.User.Sid.String()
	if err != nil {
		return ""
	}
	return s
}

// regSyncRoots — HKLM\…\SyncRootManager\*\UserSyncRoots: значение с именем-SID текущего пользователя — папка клиента.
func regSyncRoots() []string {
	sid := currentSID()
	if sid == "" {
		return nil
	}
	base, ok := regOpen(syscall.Handle(syscall.HKEY_LOCAL_MACHINE), syncRootsKey)
	if !ok {
		return nil
	}
	defer func() { _ = syscall.RegCloseKey(base) }()
	var out []string
	for _, name := range regSubkeys(base) {
		h, ok := regOpen(base, name+`\UserSyncRoots`)
		if !ok {
			continue
		}
		if v, ok := regString(h, sid); ok {
			out = append(out, v)
		}
		_ = syscall.RegCloseKey(h)
	}
	return out
}

// regPinnedRoots — HKCU\Software\Classes\CLSID\{…}: клиент с закреплённой в Проводнике папкой (Яндекс.Диск 4) —
// System.IsPinnedToNameSpaceTree = 1 и TargetFolderPath в Instance\InitPropertyBag.
func regPinnedRoots() []string {
	base, ok := regOpen(syscall.Handle(syscall.HKEY_CURRENT_USER), clsidKey)
	if !ok {
		return nil
	}
	defer func() { _ = syscall.RegCloseKey(base) }()
	var out []string
	for _, name := range regSubkeys(base) {
		if !strings.HasPrefix(name, "{") {
			continue
		}
		h, ok := regOpen(base, name)
		if !ok {
			continue
		}
		pinned, _ := regDword(h, pinnedValue)
		_ = syscall.RegCloseKey(h)
		if pinned != 1 {
			continue
		}
		bag, ok := regOpen(base, name+`\`+instanceProps)
		if !ok {
			continue
		}
		if v, ok := regString(bag, targetValue); ok {
			out = append(out, v)
		}
		_ = syscall.RegCloseKey(bag)
	}
	return out
}

// VirtualCloud — путь на виртуальном облачном диске (Google Drive, Яндекс.Диск, pCloud, Box): метка тома из списка либо
// несъёмный диск с чужой ФС. Вызывать после проверки, что диск не сетевой (osx.Remote), и под сроком: чтение
// сведений о томе мёртвого диска висит. Незнакомое и недоступное — false (недоступное поймают другие проверки).
func VirtualCloud(path string) bool {
	root, err := volRoot(path)
	if err != nil {
		return false
	}
	rp, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return false
	}
	t, _, _ := procVolDriveType.Call(uintptr(unsafe.Pointer(rp))) //nolint:gosec // G103: аргумент WinAPI
	if t != volDriveFixed && t != volDriveRemovable {
		return false
	}
	_, label, fsName, err := volInfo(root)
	if err != nil {
		return false
	}
	return virtualCloudVol(label, fsName, t == volDriveFixed)
}
