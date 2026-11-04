package mcpbridge

import (
	"testing"

	"golang.org/x/sys/windows"
)

// lockDir делает папке сокета то же, что программа (agents/perm_windows.go): владелец — свой SID,
// защищённый DACL «пользователь + SYSTEM». Папка в %TEMP% иначе наследует «Администраторы», а у
// процесса раннера владельцем становится группа, и проверка посредника честно отказывает.
func lockDir(t *testing.T, dir string) {
	t.Helper()
	setDACL(t, dir, true, "D:P(A;OICI;FA;;;"+userSID(t).String()+")(A;OICI;FA;;;SY)")
}

func userSID(t *testing.T) *windows.SID {
	t.Helper()
	tu, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	return tu.User.Sid
}

// setDACL ставит папке защищённый DACL по SDDL; ownMe — владелец свой SID. Ошибка — провал.
func setDACL(t *testing.T, dir string, ownMe bool, sddl string) {
	t.Helper()
	if err := trySetDACL(dir, nil, ownMe, sddl); err != nil {
		t.Fatalf("DACL папки: %v", err)
	}
}

// trySetDACL — как setDACL, но с владельцем owner (nil — свой SID, если ownMe) и ошибкой наружу:
// сменить владельца на чужой может не каждый процесс.
func trySetDACL(dir string, owner *windows.SID, ownMe bool, sddl string) error {
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	info := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION | windows.PROTECTED_DACL_SECURITY_INFORMATION)
	if owner == nil && ownMe {
		tu, err := windows.GetCurrentProcessToken().GetTokenUser()
		if err != nil {
			return err
		}
		owner = tu.User.Sid
	}
	if owner != nil {
		info |= windows.OWNER_SECURITY_INFORMATION
	}
	return windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, info, owner, nil, dacl, nil)
}

// checkOnly — checkSock без удержания папки: для тестов, которым нужен только вердикт.
func checkOnly(sock string) (bool, error) {
	present, release, err := checkSock(sock)
	release()
	return present, err
}
