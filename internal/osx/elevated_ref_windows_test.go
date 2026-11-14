package osx

import (
	"syscall"
	"testing"
	"unsafe"
)

const (
	tokenIntegrityLevel = 25     // TokenIntegrityLevel: TOKEN_MANDATORY_LABEL
	mandatoryHighRID    = 0x3000 // SECURITY_MANDATORY_HIGH_RID: S-1-16-12288
)

// refElevated — независимый признак: уровень целостности токена (TokenIntegrityLevel), а не
// TokenElevation, как в Elevated. Высокий (S-1-16-12288) и системный — повышенный процесс.
func refElevated(t *testing.T) bool {
	t.Helper()
	h, err := syscall.GetCurrentProcess()
	if err != nil {
		t.Fatalf("GetCurrentProcess: %v", err)
	}
	var tok syscall.Token
	if err := syscall.OpenProcessToken(h, syscall.TOKEN_QUERY, &tok); err != nil {
		t.Fatalf("OpenProcessToken: %v", err)
	}
	defer tok.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	var n uint32
	_ = syscall.GetTokenInformation(tok, tokenIntegrityLevel, nil, 0, &n) // узнать размер
	if n == 0 {
		t.Fatal("GetTokenInformation(TokenIntegrityLevel): размер не вернулся")
	}
	// Буфер из uint64: указатель в начале структуры должен лежать выровненно.
	buf := make([]uint64, (n+7)/8)
	if err := syscall.GetTokenInformation(tok, tokenIntegrityLevel, (*byte)(unsafe.Pointer(&buf[0])), n, &n); err != nil { //nolint:gosec // G103: буфер WinAPI нужного размера
		t.Fatalf("GetTokenInformation(TokenIntegrityLevel): %v", err)
	}
	// TOKEN_MANDATORY_LABEL начинается с указателя на SID: ревизия, число подуровней, 6 байт
	// органа, затем подуровни по 4 байта; последний — уровень целостности.
	sid := *(*unsafe.Pointer)(unsafe.Pointer(&buf[0])) //nolint:gosec // G103: указатель на SID внутри того же буфера
	count := *(*uint8)(unsafe.Add(sid, 1))
	if count == 0 {
		t.Fatal("SID уровня целостности без подуровней")
	}
	rid := *(*uint32)(unsafe.Add(sid, 8+4*(int(count)-1)))
	return rid >= mandatoryHighRID
}
