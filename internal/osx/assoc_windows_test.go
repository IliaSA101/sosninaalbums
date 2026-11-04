//go:build windows

package osx

import (
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// Круг по настоящему реестру в своей ветке HKCU\Software\SosninaAlbumsTest-<pid>-<время>:
// запись → чтение → снятие; кириллица в описании и пути, UserChoice, несколько вызовов подряд.
func TestAssocRegistryRoundTrip(t *testing.T) {
	root := fmt.Sprintf(`Software\SosninaAlbumsTest-%d-%d`, os.Getpid(), time.Now().UnixNano())
	be := winReg{quiet: true}
	r := newReg(root, be)
	t.Cleanup(func() { _ = be.deleteTree(root) })

	if got := r.Read(); got != (AssocState{}) {
		t.Fatalf("новая ветка не пуста: %+v", got)
	}
	exe := `C:\Users\Мария\Альбомы 0.25\SosninaAlbums.exe`
	for _, ver := range []string{"0.25.0", "0.25.1"} { // повтор — перезапись
		if err := r.Write(exe, ver); err != nil {
			t.Fatal(err)
		}
		want := AssocState{ProgID: AssocProgID, Exe: exe, AppVersion: ver}
		if got := r.Read(); got != want {
			t.Fatalf("Read = %+v, ждали %+v", got, want)
		}
	}
	if v, ok := be.get(root+`\Classes\`+AssocProgID, ""); !ok || v != "Альбом Sosnina Albums" {
		t.Errorf("описание = %q, %v", v, ok)
	}
	if v, ok := be.get(root+`\Classes\`+AssocProgID+`\DefaultIcon`, ""); !ok || v != `"`+exe+`",0` {
		t.Errorf("значок = %q", v)
	}

	// Установка 111 и выбор пользователя.
	if err := be.set(root+`\SosninaAlbums`, "InstallDir", `C:\Programs\Sosnina Albums`); err != nil {
		t.Fatal(err)
	}
	if err := be.set(root+`\SosninaAlbums`, "Version", "0.25.0"); err != nil {
		t.Fatal(err)
	}
	choice := root + `\FileExts\.salbum\UserChoice`
	if err := be.set(choice, "ProgId", `Applications\SosninaAlbums-0.24.exe`); err != nil {
		t.Fatal(err)
	}
	got := r.Read()
	if got.InstallDir != `C:\Programs\Sosnina Albums` || got.InstallVersion != "0.25.0" || !got.UserChoiceIsOurs() {
		t.Fatalf("установка/выбор не прочитаны: %+v", got)
	}
	if dropped, err := r.DropUserChoice(); err != nil || !dropped {
		t.Fatalf("DropUserChoice = %v, %v", dropped, err)
	}
	if dropped, err := r.DropUserChoice(); err != nil || dropped {
		t.Errorf("второй DropUserChoice = %v, %v", dropped, err)
	}

	if err := r.Remove(); err != nil {
		t.Fatal(err)
	}
	if err := r.Remove(); err != nil { // повтор — не ошибка
		t.Errorf("повторный Remove: %v", err)
	}
	got = r.Read()
	if got.ProgID != "" || got.Exe != "" || got.AppVersion != "" || got.UserChoice != "" {
		t.Errorf("после Remove: %+v", got)
	}
	if got.InstallDir == "" {
		t.Error("Remove не должен трогать раздел установки")
	}
	for _, k := range []string{`\Classes\.salbum`, `\Classes\.salbum\OpenWithProgids`} {
		if keyExists(root + k) {
			t.Errorf("пустой ключ %s остался после Remove", k)
		}
	}
	// Remove ставит отметку «выключено» (по ней установщик не вернёт связь), Write снимает.
	if !r.AssocOff() {
		t.Error("после Remove нет отметки AssocOff")
	}
	if err := r.Write(exe, "0.25.2"); err != nil {
		t.Fatal(err)
	}
	if r.AssocOff() {
		t.Error("после Write отметка не снята")
	}
}

// keyExists — ключ HKCU\key открывается (есть).
func keyExists(key string) bool {
	k, err := syscall.UTF16PtrFromString(key)
	if err != nil {
		return false
	}
	var h syscall.Handle
	r, _, _ := procRegOpenKeyEx.Call(hkeyCurrentUser, ptr(k), 0, keyCount, uintptr(unsafe.Pointer(&h))) //nolint:gosec // G103: аргументы WinAPI
	if r != 0 {
		return false
	}
	_, _, _ = procRegCloseKey.Call(uintptr(h))
	return true
}

// Чужое значение в OpenWithProgids держит оба ключа, наш ProgID уходит.
func TestAssocRegistryKeepsForeignProgid(t *testing.T) {
	root := fmt.Sprintf(`Software\SosninaAlbumsTest-%d-%d`, os.Getpid(), time.Now().UnixNano())
	be := winReg{quiet: true}
	r := newReg(root, be)
	t.Cleanup(func() { _ = be.deleteTree(root) })
	if err := r.Write(`C:\A\SosninaAlbums.exe`, "0.25.0"); err != nil {
		t.Fatal(err)
	}
	owp := root + `\Classes\.salbum\OpenWithProgids`
	if err := be.setNone(owp, "Other.Album"); err != nil {
		t.Fatal(err)
	}
	if err := r.Remove(); err != nil {
		t.Fatal(err)
	}
	if !keyExists(owp) || !keyExists(root+`\Classes\.salbum`) {
		t.Error("чужое значение не удержало ключи")
	}
}

// Настоящие ветки читаются без паники (запись не делаем).
func TestAssocSystemRead(t *testing.T) {
	if !Supported() {
		t.Fatal("на Windows Supported() должен быть true")
	}
	_ = ReadAssoc()
}
