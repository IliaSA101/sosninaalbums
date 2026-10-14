//go:build windows

package osx

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// shortPath — имя 8.3 для существующего пути; пусто, если на томе короткие имена выключены.
func shortPath(p string) string {
	in, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return ""
	}
	buf := make([]uint16, syscall.MAX_PATH)
	n, err := syscall.GetShortPathName(in, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 || n > uint32(len(buf)) {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// longTemp — временная папка с длинным именем там, где работают короткие имена: они бывают
// выключены на D: (рабочий диск разработки), поэтому берём настоящий %LOCALAPPDATA%\Temp,
// а если его нет — t.TempDir.
func longTemp(t *testing.T) string {
	t.Helper()
	if base := os.Getenv("LOCALAPPDATA"); base != "" {
		if d, err := os.MkdirTemp(filepath.Join(base, "Temp"), "launch-test-*-длинное-имя"); err == nil {
			t.Cleanup(func() { _ = os.RemoveAll(d) })
			return d
		}
	}
	return t.TempDir()
}

// %TEMP% в виде 8.3 при длинном имени exe (и наоборот) — всё равно «во временной папке».
func TestLaunchPlaceShortNames(t *testing.T) {
	tmp := longTemp(t)
	sub := filepath.Join(tmp, "Temp1_очень длинное имя архива.zip")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(sub, "SosninaAlbums.exe")
	shortTmp, shortSub := shortPath(tmp), shortPath(sub)
	if shortTmp == "" || shortSub == "" || strings.EqualFold(shortSub, sub) {
		t.Skip("н/п: на томе нет коротких имён 8.3")
	}
	if got := LaunchPlace(exe, shortTmp); got != PlaceTemp {
		t.Errorf("temp в 8.3: %q", got)
	}
	if got := LaunchPlace(filepath.Join(shortSub, "SosninaAlbums.exe"), tmp); got != PlaceTemp {
		t.Errorf("exe в 8.3: %q", got)
	}
	if got := LaunchPlace(filepath.Join(shortSub, "x.exe"), shortTmp); got != PlaceTemp {
		t.Errorf("оба в 8.3: %q", got)
	}
	// Подпапок a\b на диске нет: GetLongPathName такой путь целиком не раскроет (сбой CI).
	if got := LaunchPlace(filepath.Join(shortSub, "a", "b", "x.exe"), tmp); got != PlaceTemp {
		t.Errorf("exe в 8.3, подпапок нет, temp длинный: %q", got)
	}
	if got := LaunchPlace(filepath.Join(sub, "a", "b", "x.exe"), shortTmp); got != PlaceTemp {
		t.Errorf("exe длинный, подпапок нет, temp в 8.3: %q", got)
	}
	if got := LaunchPlace(filepath.Join(shortSub, "a", "b", "x.exe"), shortTmp); got != PlaceTemp {
		t.Errorf("оба в 8.3, подпапок нет: %q", got)
	}
	other := t.TempDir()
	if got := LaunchPlace(filepath.Join(shortPath(other), "x.exe"), shortTmp); got != PlaceNormal {
		t.Errorf("чужая папка в 8.3 не должна совпасть: %q", got)
	}
}

// Короткая форма раскрывается, длинная остаётся: t.TempDir на раннере CI лежит в 8.3
// («C:\Users\RUNNER~1\…»), поэтому «путь не меняется» верно только для длинной формы.
func TestLongPathKeepsPath(t *testing.T) {
	dir := t.TempDir()
	long := longPath(dir)
	if got := longPath(long); !strings.EqualFold(got, long) {
		t.Errorf("не идемпотентно: %q → %q", long, got)
	}
	if short := shortPath(dir); short != "" && !strings.EqualFold(longPath(short), long) {
		t.Errorf("короткая форма: %q → %q, ждали %q", short, longPath(short), long)
	}
	missing := filepath.Join(long, "нет", "такого")
	if got := longPath(missing); !strings.EqualFold(got, missing) {
		t.Errorf("несуществующий хвост: %q → %q", missing, got)
	}
	const noDisk = `Q:\нет\диска`
	if got := longPath(noDisk); !strings.EqualFold(got, noDisk) {
		t.Errorf("нет диска: %q", got)
	}
	if got := longPath(""); got != "" {
		t.Errorf("пустой путь: %q", got)
	}
}

// Короткая форма предка + несуществующий хвост раскрывается так же, как у существующего пути.
func TestLongPathMissingTail(t *testing.T) {
	sub := filepath.Join(longTemp(t), "Temp1_очень длинное имя архива.zip")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	short := shortPath(sub)
	if short == "" || strings.EqualFold(short, sub) {
		t.Skip("н/п: на томе нет коротких имён 8.3")
	}
	got := longPath(filepath.Join(short, "a", "b", "x.exe"))
	if want := filepath.Join(sub, "a", "b", "x.exe"); !strings.EqualFold(got, want) {
		t.Errorf("longPath = %q, ждали %q", got, want)
	}
}

// Системная папка с длинным именем («Program Files» → PROGRA~1) есть на любой машине, где 8.3
// включены на C:, — проверка короткой формы и несуществующего хвоста без правок на диске.
func TestLongPathSystemDir(t *testing.T) {
	pf := os.Getenv("ProgramFiles")
	short := shortPath(pf)
	if pf == "" || short == "" || strings.EqualFold(short, pf) {
		t.Skip("н/п: на томе нет коротких имён 8.3")
	}
	if got := longPath(short); !strings.EqualFold(got, pf) {
		t.Errorf("существующая: %q → %q, ждали %q", short, got, pf)
	}
	got := longPath(filepath.Join(short, "нет", "такого", "x.exe"))
	if want := filepath.Join(pf, "нет", "такого", "x.exe"); !strings.EqualFold(got, want) {
		t.Errorf("с хвостом: %q, ждали %q", got, want)
	}
	// Завершающий разделитель: Dir не снимает его, без Clean хвост дублировался («…\нет\нет»).
	got = longPath(short + `\нет\`)
	if want := filepath.Join(pf, "нет"); !strings.EqualFold(got, want) {
		t.Errorf("завершающий разделитель: %q, ждали %q", got, want)
	}
	// «Program Files» как временная папка в 8.3, exe глубже и в длинной форме.
	if got := LaunchPlace(filepath.Join(pf, "Z", "x.exe"), short); got != PlaceTemp {
		t.Errorf("LaunchPlace: %q", got)
	}
}

// К предку поднимаемся только при «не найден»; недоступный том (нет прав, мёртвая сеть) — один
// вызов и путь как есть. Сеть не нужна: GetLongPathName подменён.
func TestLongPathStopsOnOtherErrors(t *testing.T) {
	calls := 0
	deny := func(p string) (string, error) { calls++; return p, syscall.ERROR_ACCESS_DENIED }
	in := `\\сервер\шара\а\б\в.exe`
	if got := longPathVia(in, deny); got != in || calls != 1 {
		t.Errorf("при ошибке доступа: %q, вызовов %d, ждали %q и 1", got, calls, in)
	}
	calls = 0
	miss := func(p string) (string, error) { calls++; return p, syscall.ERROR_PATH_NOT_FOUND }
	longPathVia(`C:\а\б\в`, miss)
	if calls != 4 { // в, б, а, корень
		t.Errorf("при «путь не найден» поднимаемся до корня: вызовов %d, ждали 4", calls)
	}
}
