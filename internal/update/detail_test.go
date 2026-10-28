package update

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Detail: пользователю — слова обновления, в лог — причина; Is видит обе; чужая ошибка — как есть.
func TestDetail(t *testing.T) {
	err := humanize(fetch.ErrSignature)
	if err.Error() != ErrVerify.Error() || !errors.Is(err, ErrVerify) || !errors.Is(err, fetch.ErrSignature) {
		t.Fatalf("humanize: %v", err)
	}
	if d := Detail(err); !strings.HasPrefix(d, ErrVerify.Error()+" (") || !strings.HasSuffix(d, " (подпись не сошлась)") {
		t.Errorf("Detail: %q", d)
	}
	// Обёртка поверх тоже находит причину.
	if d := Detail(errors.Join(errors.New("ещё"), err)); !strings.Contains(d, "подпись не сошлась") {
		t.Errorf("Detail обёртки: %q", d)
	}
	if Detail(fetch.ErrOffline) != fetch.ErrOffline.Error() {
		t.Error("Detail чужой ошибки")
	}
	// В скобке — нейтральная причина, слов про библиотеку и наборы нет; все причины разные.
	seen := map[string]bool{}
	for _, src := range []error{fetch.ErrNotFound, fetch.ErrSignature, fetch.ErrCatalog, fetch.ErrNewer,
		fetch.ErrBroken, fetch.ErrAddress, fetch.ErrCancelled, fetch.ErrDisk} {
		d := Detail(humanize(src))
		if strings.Contains(d, "библиотек") || strings.Contains(d, "набор") || strings.Contains(d, src.Error()) {
			t.Errorf("Detail %v: %q", src, d)
		}
		if seen[d] {
			t.Errorf("Detail повторяет причину: %q", d)
		}
		seen[d] = true
	}
	if d := Detail(humanize(fetch.ErrNotFound)); !strings.HasSuffix(d, " (ответ 404)") {
		t.Errorf("404: %q", d)
	}
	// Причина проверки файла (не fetch) остаётся как есть.
	if d := Detail(&humanError{human: ErrVerify, cause: errors.New("sha256 не совпал")}); !strings.HasSuffix(d, " (sha256 не совпал)") {
		t.Errorf("своя причина: %q", d)
	}
	// Все сопоставления сохраняют исходную причину.
	for src, want := range map[error]error{fetch.ErrCatalog: ErrVerify, fetch.ErrBroken: ErrVerify, fetch.ErrNewer: ErrVerify,
		fetch.ErrAddress: ErrAddress, fetch.ErrNotFound: ErrNoFile, fetch.ErrCancelled: ErrCancelled, fetch.ErrDisk: ErrDiskWrite} {
		if got := humanize(src); !errors.Is(got, want) || !errors.Is(got, src) || got.Error() != want.Error() {
			t.Errorf("%v → %v", src, got)
		}
	}
}

// Verify называет, что не сошлось: размер, sha256, не файл.
func TestVerifyReason(t *testing.T) {
	dir := testkit.TempDir(t)
	data, file := setup("0.25.1", 5000)
	p := filepath.Join(dir, "a.exe")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	bad := file
	bad.Size++
	hash := file
	hash.SHA256 = "0" + file.SHA256[1:]
	for name, c := range map[string]struct {
		path string
		f    File
		want string
	}{"размер": {p, bad, "длина"}, "хеш": {p, hash, "sha256"}, "папка": {dir, file, "не обычный файл"},
		"нет файла": {filepath.Join(dir, "нет"), file, "не обычный файл"}} {
		err := Verify(c.path, c.f)
		if !errors.Is(err, ErrVerify) || err.Error() != ErrVerify.Error() || !strings.Contains(Detail(err), c.want) {
			t.Errorf("%s: %v / %q", name, err, Detail(err))
		}
	}
}

// Client.Stand: только источник стенда (loopback http) — настоящий и тестовый TLS нет.
func TestClientStand(t *testing.T) {
	if New(GitHub()).Stand() {
		t.Error("GitHub назван стендом")
	}
	src, ok := Stand("http://127.0.0.1:34200/feed/", "")
	if !ok || !New(src).Stand() {
		t.Error("стенд не назван стендом")
	}
	if New(fetch.Source{}).Stand() {
		t.Error("пустой источник назван стендом")
	}
}
