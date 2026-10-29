package mcpbridge

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// checkSock — проверка перед dial (design §4.1): папка сокета — не ссылка и не точка соединения,
// владелец — свой SID, в списке доступа только разрешающие записи для себя и SYSTEM (то, что
// ставит agents/perm_windows.go), читаются по дескриптору самой папки, а не по пути (подмену
// после проверки ловит сверка собеседника, 148-6); файл — сокет. Владельца файла не сверяем: процесс «от
// администратора» создаёт его от группы «Администраторы», а доступ ограничивает папка.
// present=false — папки или файла нет.
func checkSock(sock string) (present bool, err error) {
	dir := filepath.Dir(sock)
	fi, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil || !fi.IsDir() || fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return false, errUnsafe
	}
	if err := checkDirACL(dir); err != nil {
		return false, errUnsafe
	}
	fi, err = os.Lstat(sock)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil || fi.Mode()&os.ModeSocket == 0 {
		return false, errUnsafe
	}
	return true, nil
}

// openDirNoFollow открывает саму папку, не идя по точке соединения; не каталог или точка
// соединения — errUnsafe. Общий доступ полный: посредник ничьё переименование не держит.
func openDirNoFollow(dir string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	h, err := windows.CreateFile(p, windows.READ_CONTROL,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return 0, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		_ = windows.CloseHandle(h)
		return 0, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(h)
		return 0, errUnsafe
	}
	return h, nil
}

func checkDirACL(dir string) error {
	h, err := openDirNoFollow(dir)
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(h) }()
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	user := tu.User.Sid
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	sd, err := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	if owner == nil || !owner.Equals(user) {
		return errUnsafe
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if dacl == nil { // пустой список доступа = «всем всё»
		return errUnsafe
	}
	for i := range uint32(dacl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return err
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart)) //nolint:gosec // G103: SID лежит в записи ACE со смещения SidStart (WinAPI)
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || (!sid.Equals(user) && !sid.Equals(system)) {
			return errUnsafe
		}
	}
	return nil
}
