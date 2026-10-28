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
// наследованием на содержимое. Наследование от %APPDATA% не годится: PortableApps уводит папку
// на флешку, у корня диска по умолчанию у вошедших пользователей «Изменение». После записи
// дескриптор читается обратно и сверяется: владелец, защита от наследования, ровно две записи.
// Права файла сокета отдельно не ставятся: он наследует DACL папки (спайк Т0, С2).
func secureDir(dir string) error {
	if fi, err := os.Lstat(dir); err == nil {
		if fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 { // ссылка и точка соединения
			return errNoPerm
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	user := tu.User.Sid
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + user.String() + ")(A;OICI;FA;;;SY)")
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil); err != nil {
		return err
	}
	return checkDACL(dir, user)
}

// checkDACL читает дескриптор папки и сверяет с ожидаемым.
func checkDACL(dir string, user *windows.SID) error {
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	admins, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return err
	}
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	// Владелец: пользователь, а у запущенного «от администратора» процесса — группа
	// «Администраторы» (так ОС записывает владельца новых объектов); другого не бывает.
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	if owner == nil || !owner.Equals(user) && !owner.Equals(admins) {
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
