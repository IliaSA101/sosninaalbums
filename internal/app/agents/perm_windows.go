package agents

import (
	"errors"
	"io/fs"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// fullAccess — FILE_ALL_ACCESS (маска «FA» в SDDL).
const fullAccess = 0x1F01FF

// secureDir готовит папку сокета mcp: явный защищённый DACL «пользователь + SYSTEM» с
// наследованием на содержимое; владелец ставится явно (у процесса «от администратора» ОС
// записала бы группу «Администраторы», а посредник сверяет свой SID). Наследование от %APPDATA% не годится: PortableApps уводит папку
// на флешку, у корня диска по умолчанию у вошедших пользователей «Изменение». Права пишутся по
// дескриптору самой папки, открытому с FILE_FLAG_OPEN_REPARSE_POINT: точка соединения, появившаяся
// после проверки, не уводит запись в чужую папку (0.32-148-6). После записи дескриптор читается
// обратно и сверяется: владелец, защита от наследования, ровно две записи. Дескриптор без
// FILE_SHARE_DELETE и с FILE_LIST_DIRECTORY держится до release (его зовёт open после Listen):
// пока он открыт, mcp не переименовать. Права файла сокета отдельно не ставятся: он наследует DACL папки (спайк Т0, С2).
func secureDir(dir string) (release func(), err error) {
	release = func() {}
	// Ссылку отсекаем сразу (висячая ссылка сорвала бы MkdirAll с другим текстом); от подмены
	// после этой проверки защищает дескриптор ниже.
	if fi, err := os.Lstat(dir); err == nil {
		if fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 { // ссылка и точка соединения
			return release, errNoPerm
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return release, err
	}
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return release, err
	}
	h, err := openDirNoFollow(dir)
	if err != nil {
		return release, err
	}
	release = func() { _ = windows.CloseHandle(h) }
	defer func() {
		if err != nil {
			release()
			release = func() {}
		}
	}()
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return release, err
	}
	user := tu.User.Sid
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + user.String() + ")(A;OICI;FA;;;SY)")
	if err != nil {
		return release, err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return release, err
	}
	if err = windows.SetSecurityInfo(h, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		user, nil, dacl, nil); err != nil {
		return release, err
	}
	got, err := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return release, err
	}
	err = checkDACL(got, user)
	return release, err
}

// openDirNoFollow открывает саму папку (не цель ссылки) на чтение и запись прав и сверяет по
// дескриптору, что это каталог, а не ссылка или точка соединения (иначе errNoPerm). Общий доступ
// без FILE_SHARE_DELETE: пока дескриптор жив, папку не переименовать и не удалить. Режим общего
// доступа ОС сверяет лишь с дескрипторами, открытыми на данные (FILE_READ_DATA = FILE_LIST_DIRECTORY,
// FILE_WRITE_DATA, FILE_APPEND_DATA, FILE_EXECUTE = FILE_TRAVERSE, DELETE; IoCheckShareAccess):
// с одними READ_CONTROL/WRITE_DAC/WRITE_OWNER переименование проходило бы, поэтому просим ещё
// FILE_LIST_DIRECTORY (хозяину своей папки он доступен; bind и удаление сокета внутри не мешает).
func openDirNoFollow(dir string) (windows.Handle, error) {
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	h, err := windows.CreateFile(p, windows.FILE_LIST_DIRECTORY|windows.READ_CONTROL|windows.WRITE_DAC|windows.WRITE_OWNER,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return 0, err
	}
	var fi windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &fi); err != nil {
		_ = windows.CloseHandle(h)
		return 0, err
	}
	if fi.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || fi.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		_ = windows.CloseHandle(h)
		return 0, errNoPerm
	}
	return h, nil
}

// checkDACL сверяет прочитанный дескриптор папки с ожидаемым: владелец — user, защита от
// наследования, ровно две разрешающие записи (user и SYSTEM) с полным доступом и наследованием.
func checkDACL(sd *windows.SECURITY_DESCRIPTOR, user *windows.SID) error {
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	if owner == nil || !owner.Equals(user) {
		return errNoPerm
	}
	ctl, _, err := sd.Control()
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if ctl&windows.SE_DACL_PROTECTED == 0 || dacl == nil || dacl.AceCount != 2 {
		return errNoPerm
	}
	seenUser, seenSystem := false, false
	for i := range uint32(dacl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return err
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart)) //nolint:gosec // G103: SID лежит в записи ACE со смещения SidStart (WinAPI)
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Mask != fullAccess ||
			ace.Header.AceFlags&(windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE) !=
				windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE {
			return errNoPerm
		}
		switch {
		case sid.Equals(user):
			seenUser = true
		case sid.Equals(system):
			seenSystem = true
		default:
			return errNoPerm
		}
	}
	if !seenUser || !seenSystem {
		return errNoPerm
	}
	return nil
}

// secureSock на Windows ничего не меняет: файл сокета наследует DACL папки.
func secureSock(string) error { return nil }
