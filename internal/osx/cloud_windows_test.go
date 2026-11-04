package osx

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// regExe выполняет reg.exe с аргументами; провал — t.Fatalf с выводом (раннер win-qa — администратор, HKLM доступен).
func regExe(t *testing.T, args ...string) {
	t.Helper()
	if out, err := exec.Command("reg", args...).CombinedOutput(); err != nil {
		t.Fatalf("reg %v: %v: %s", args, err, out)
	}
}

// TestCloudRootsWindowsReg: настоящий реестр. Поддельные SyncRootManager\Test…!SID (HKLM, раннер — администратор) и
// закреплённый CLSID (HKCU) плюс info.json Dropbox в %APPDATA% — CloudRoots находит все три корня и не берёт незакреплённый
// CLSID. Пишет в реестр только при SA_REG_WRITE=1 (шаг win-qa; без него — пропуск, в CI он провал через peer-foreign.mjs);
// разделы и значения удаляются при любом исходе.
func TestCloudRootsWindowsReg(t *testing.T) {
	if os.Getenv("SA_REG_WRITE") != "1" {
		t.Skip("запись в реестр разрешена только SA_REG_WRITE=1 (шаг win-qa)")
	}
	sid := currentSID()
	if sid == "" {
		t.Fatal("SID процесса не прочитан")
	}
	syncRoot, pinned, notPinned, dropbox := testkit.AbsPath(`SATest160\Sync`), testkit.AbsPath(`SATest160\Pinned`),
		testkit.AbsPath(`SATest160\NotPinned`), testkit.AbsPath(`SATest160\Dropbox`)

	provider := `HKLM\` + syncRootsKey + `\SATest160!` + sid + `!test`
	regExe(t, "add", provider+`\UserSyncRoots`, "/v", sid, "/t", "REG_SZ", "/d", syncRoot, "/f")
	t.Cleanup(func() { _ = exec.Command("reg", "delete", provider, "/f").Run() })

	clsid := `HKCU\` + clsidKey + `\{A1600000-0000-4000-8000-000000000160}`
	regExe(t, "add", clsid, "/v", pinnedValue, "/t", "REG_DWORD", "/d", "1", "/f")
	regExe(t, "add", clsid+`\`+instanceProps, "/v", targetValue, "/t", "REG_SZ", "/d", pinned, "/f")
	t.Cleanup(func() { _ = exec.Command("reg", "delete", clsid, "/f").Run() })

	free := `HKCU\` + clsidKey + `\{A1600000-0000-4000-8000-000000000161}`
	regExe(t, "add", free, "/v", pinnedValue, "/t", "REG_DWORD", "/d", "0", "/f")
	regExe(t, "add", free+`\`+instanceProps, "/v", targetValue, "/t", "REG_SZ", "/d", notPinned, "/f")
	t.Cleanup(func() { _ = exec.Command("reg", "delete", free, "/f").Run() })

	home := testkit.TempDir(t)
	writeCloudFile(t, dropboxFile(t, home), dropboxJSON(t, dropbox, ""))

	got := CloudRoots(home, allowAll)
	for _, want := range []string{syncRoot, pinned, dropbox} {
		if !slices.Contains(got, filepath.Clean(want)) {
			t.Errorf("корень %q не найден: %q", want, got)
		}
	}
	if slices.Contains(got, filepath.Clean(notPinned)) {
		t.Errorf("незакреплённая папка принята: %q", got)
	}
	if got := CloudRoots(home, refuseAll); len(got) != 0 {
		t.Errorf("фильтр отказал, корни %q", got)
	}
}

// TestVirtualCloudWindowsSystem: настоящий том временной папки — не виртуальный облачный диск; UNC и пустая строка — false
// без паники и без обращения к сети.
func TestVirtualCloudWindowsSystem(t *testing.T) {
	if VirtualCloud(testkit.TempDir(t)) {
		t.Error("диск временной папки признан облачным")
	}
	for _, p := range []string{``, `\\srv\share\x`, `relative`} {
		if VirtualCloud(p) {
			t.Errorf("%q признан облачным диском", p)
		}
	}
}
