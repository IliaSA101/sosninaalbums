package peercred

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func init() {
	// Первый вход нового пользователя с -LoadUserProfile создаёт профиль (десятки секунд на раннере),
	// а «готов» помощника приходит только после этого: 30 с на Windows мало.
	readyWait = 120 * time.Second
	peerPIDOf = func(c net.Conn) (uint32, error) {
		rc, err := c.(*net.UnixConn).SyscallConn()
		if err != nil {
			return 0, err
		}
		var pid uint32
		var qerr error
		if err := rc.Control(func(fd uintptr) { pid, qerr = ioctlPeerPID(fd) }); err != nil {
			return 0, err
		}
		return pid, qerr
	}
}

// skipUnlessForeign: чужая учётная запись — только CI (SA_PEER_FOREIGN=cred, SA_PEER_USER/SA_PEER_PASS).
func skipUnlessForeign(t *testing.T) {
	t.Helper()
	if os.Getenv("SA_PEER_FOREIGN") != "cred" {
		t.Skip("чужая учётная запись — только CI: SA_PEER_FOREIGN=cred")
	}
	if os.Getenv("SA_PEER_USER") == "" || os.Getenv("SA_PEER_PASS") == "" {
		t.Fatal("SA_PEER_USER и SA_PEER_PASS пусты")
	}
}

// foreignDir — папка в C:\Users\Public с правом «Все: полный доступ» (профиль раннера чужому закрыт).
func foreignDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(`C:\Users\Public`, "sa-peer-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { removeDir(t, dir) })
	sd, err := windows.SecurityDescriptorFromString("D:(A;OICI;GA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.UNPROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "s"), 0o777); err != nil { // наследует «Все»
		t.Fatal(err)
	}
	return dir
}

// foreignCmd: Windows PowerShell 5.1 запускает cmd с переменными от временного пользователя и ждёт выхода.
// Шаг CI идёт под pwsh 7, а он кладёт в PSModulePath свои модули: 5.1 их не грузит и не находит
// Microsoft.PowerShell.Security (прогон 40832895417) - поэтому PSModulePath у дочернего убран,
// пароль в SecureString собран без ConvertTo-SecureString, любая ошибка скрипта - сразу выход с кодом 1.
func foreignCmd(_ *testing.T, exe, mode, sock string) *exec.Cmd {
	dir := filepath.Dir(filepath.Dir(sock))
	script := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$p = New-Object System.Security.SecureString
foreach ($ch in $env:SA_PEER_PASS.ToCharArray()) { $p.AppendChar($ch) }
$c = New-Object System.Management.Automation.PSCredential(('.\' + $env:SA_PEER_USER), $p)
Start-Process -FilePath cmd.exe -ArgumentList '/c set %s=%s&& set %s=%s&& "%s"' -Credential $c -LoadUserProfile -WorkingDirectory 'C:\Users\Public' -RedirectStandardError '%s' -RedirectStandardOutput '%s' -Wait`,
		helperEnv, mode, sockEnv, sock, exe, filepath.Join(dir, "helper.err"), filepath.Join(dir, "helper.out"))
	ps := filepath.Join(os.Getenv("SystemRoot"), `System32\WindowsPowerShell\v1.0\powershell.exe`)
	cmd := exec.Command(ps, "-NoProfile", "-NonInteractive", "-Command", script)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(kv), "PSMODULEPATH=") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	return cmd
}
