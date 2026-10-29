//go:build windows

package mcpbridge

import (
	"errors"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestStartProgramInstanceMutex: замок экземпляра Wails — именованный мьютекс; платформенная проверка
// видит его, пока он открыт, и не видит после закрытия.
func TestStartProgramInstanceMutex(t *testing.T) {
	key := "sa-test-" + t.Name()
	if platformInstanceAlive(key) {
		t.Fatal("мьютекса нет, а проверка видит экземпляр")
	}
	if platformInstanceAlive("") {
		t.Error("пустой ключ принят за живой экземпляр")
	}
	name, err := windows.UTF16PtrFromString("wails-app-" + key + "sim")
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		t.Fatal(err)
	}
	if !platformInstanceAlive(key) {
		t.Error("мьютекс открыт, а проверка не видит экземпляр")
	}
	_ = windows.CloseHandle(h)
	if platformInstanceAlive(key) {
		t.Error("мьютекс закрыт, а проверка видит экземпляр")
	}
}

// TestStartProgramPlatform: обычный тестовый процесс — не в пакете MSIX и сам себе токен.
func TestStartProgramPlatform(t *testing.T) {
	if platformPackaged() {
		t.Skip("тест запущен в пакете MSIX")
	}
	if got, want := osx.Elevated(), windows.GetCurrentProcessToken().IsElevated(); got != want {
		t.Errorf("osx.Elevated = %v, токен: %v", got, want)
	}
}

// TestStartProgramNoExe: нет файла — обычная ошибка запуска, не паника. Если среда запрещает выход из
// задания (задание раннера CI, клиент в задании без права breakaway), ОС отвечает «доступ запрещён»
// раньше «файла нет»: errBreakaway тогда верен, а тест доказывает, что причина во флаге, — тот же
// запуск без него даёт обычную ошибку.
func TestStartProgramNoExe(t *testing.T) {
	missing := filepath.Join(testkit.TempDir(t), "нет-такого.exe")
	plain := spawnWith(missing, []string{}, spawnFlags)
	if plain == nil || errors.Is(plain, errBreakaway) || errors.Is(plain, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("запуск без breakaway: %v, ждали обычную ошибку «нет файла»", plain)
	}
	err := spawnProgram(missing, []string{})
	switch {
	case err == nil:
		t.Fatal("spawnProgram: запуск несуществующего файла удался")
	case errors.Is(err, errBreakaway):
		t.Logf("среда запрещает breakaway из задания; без флага: %v", plain)
	case err.Error() != plain.Error():
		t.Errorf("spawnProgram: %v, без breakaway: %v", err, plain)
	}
}
