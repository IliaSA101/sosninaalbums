package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// mcpbApp — App со своей папкой данных и настоящим файлом «программы» (имя с " и \ — путь
// должен пройти манифест без потерь). Возвращает путь программы.
func mcpbApp(t *testing.T, goos string) (*App, string) {
	t.Helper()
	old := mcpbGOOS
	t.Cleanup(func() { mcpbGOOS = old })
	mcpbGOOS = goos
	a := dataApp(t)
	name := `sosnina"al\bums` // на Windows " и \ в имени файла нельзя
	if runtime.GOOS == "windows" {
		name = "sosnina albums.exe"
	}
	exe := filepath.Join(testkit.TempDir(t), name)
	if err := os.WriteFile(exe, []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	tempAside(t) // exe под %TEMP% на Windows — мёртвый путь для ExeHint
	a.agentsExe = func() (string, error) { return exe, nil }
	return a, exe
}

// readMcpb — manifest.json из файла .mcpb и число записей архива.
func readMcpb(t *testing.T, path string) (map[string]any, int) {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	for _, f := range zr.File {
		if f.Name != "manifest.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(rc)
		_ = rc.Close()
		var m map[string]any
		if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		return m, len(zr.File)
	}
	t.Fatal("нет manifest.json")
	return nil, 0
}

// TestSaveMcpb: Linux — ошибка до диалога; на Windows и Mac — отказ в диалоге, файл-архив из 3
// записей, отказы без пути.
func TestSaveMcpb(t *testing.T) {
	pick := ""
	calls, opts := stubSave(t, &pick)

	lin, _ := mcpbApp(t, "linux")
	if _, err := lin.SaveClaudeExtension(); err == nil || *calls != 0 || !strings.Contains(err.Error(), "Windows и Mac") {
		t.Fatalf("linux: %v, диалогов %d", err, *calls)
	}

	for _, goos := range []string{"windows", "darwin"} {
		t.Run(goos, func(t *testing.T) {
			a, exe := mcpbApp(t, goos)
			*calls, pick = 0, ""
			res, err := a.SaveClaudeExtension()
			if err != nil || !res.Canceled || res.Text != "" || *calls != 1 {
				t.Fatalf("отказ: %+v %v", res, err)
			}
			if opts.DefaultFilename != "Sosnina Albums.mcpb" || len(opts.Filters) != 1 || opts.Filters[0].Pattern != "*.mcpb" {
				t.Fatalf("диалог: %+v", *opts)
			}
			out := testkit.TempDir(t)
			pick = filepath.Join(out, "для Claude")
			res, err = a.SaveClaudeExtension()
			if err != nil || res.Canceled || !strings.Contains(res.Text, "для этого компьютера") {
				t.Fatalf("сохранение: %+v %v", res, err)
			}
			m, n := readMcpb(t, pick+".mcpb")
			if def := m["user_config"].(map[string]any)["program"].(map[string]any)["default"]; n != 3 || def != exe {
				t.Fatalf("записей %d, путь %q → %q", n, exe, def)
			}
			if a.dialogDir(dirAgents) != out {
				t.Fatalf("папка диалога %q", a.dialogDir(dirAgents))
			}
			saveMcpbRefusals(t, a, &pick, calls, out)
		})
	}
}

// saveMcpbRefusals: замена с .MCPB проходит, а дописанное имя поверх чужого файла, сетевой путь,
// папка на месте файла и мёртвый путь программы — отказ без пути.
func saveMcpbRefusals(t *testing.T, a *App, pick *string, calls *int, out string) {
	t.Helper()
	*pick = filepath.Join(out, "Второй.MCPB") // замену подтвердил диалог
	if _, err := a.SaveClaudeExtension(); err != nil {
		t.Fatal(err)
	}
	busy := filepath.Join(out, "x.txt.mcpb")
	if err := os.WriteFile(busy, []byte("чужое"), 0o600); err != nil {
		t.Fatal(err)
	}
	*pick = filepath.Join(out, "x.txt")
	_, err := a.SaveClaudeExtension()
	noPathErr(t, "занято", err, out)
	if got, _ := os.ReadFile(busy); string(got) != "чужое" {
		t.Fatalf("файл перезаписан: %q", got)
	}

	*pick = `\\server\share\a.mcpb`
	_, err = a.SaveClaudeExtension()
	noPathErr(t, "сетевой", err, *pick)
	if !errors.Is(err, errMcpbLocal) {
		t.Fatal(err)
	}

	*pick = filepath.Join(out, "папка.mcpb")
	if err := os.Mkdir(*pick, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err = a.SaveClaudeExtension()
	noPathErr(t, "папка", err, *pick)

	*calls = 0
	a.agentsExe = func() (string, error) { return filepath.Join(out, "нет-такого"), nil }
	if _, err = a.SaveClaudeExtension(); !errors.Is(err, errMcpbExe) || *calls != 0 {
		t.Fatalf("мёртвый путь: %v, диалогов %d", err, *calls)
	}
	a.agentsExe = func() (string, error) { return "", errors.New("нет") }
	if _, err = a.SaveClaudeExtension(); !errors.Is(err, errMcpbExe) {
		t.Fatalf("пустой путь: %v", err)
	}
}

// TestWailsCopyright: автор берётся из wails.json; битый JSON и пустое поле — пустая строка.
func TestWailsCopyright(t *testing.T) {
	if got := wailsCopyright([]byte(`{"info":{"copyright":"© 2026 Я"}}`)); got != "© 2026 Я" {
		t.Fatal(got)
	}
	for _, raw := range []string{"", "{", "null", `{"info":{}}`} {
		if got := wailsCopyright([]byte(raw)); got != "" {
			t.Fatalf("%q → %q", raw, got)
		}
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "wails.json"))
	if err != nil {
		t.Fatal(err)
	}
	if wailsCopyright(raw) == "" {
		t.Fatal("в wails.json нет copyright")
	}
}
