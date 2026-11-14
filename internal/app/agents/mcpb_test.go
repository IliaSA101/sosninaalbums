package agents

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image/png"
	"io"
	"slices"
	"strings"
	"testing"
)

// mcpbManifestOut — манифест, разобранный обратно (нужные поля).
type mcpbManifestOut struct {
	ManifestVersion string `json:"manifest_version"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	Icon            string `json:"icon"`
	Author          struct {
		Name string `json:"name"`
	} `json:"author"`
	Server struct {
		Type       string `json:"type"`
		EntryPoint string `json:"entry_point"`
		McpConfig  struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcp_config"`
	} `json:"server"`
	UserConfig map[string]struct {
		Type     string `json:"type"`
		Required bool   `json:"required"`
		Default  string `json:"default"`
	} `json:"user_config"`
	Compat struct {
		Platforms []string `json:"platforms"`
	} `json:"compatibility"`
}

func mcpbInfo(exe, goos string) McpbInfo {
	return McpbInfo{Exe: exe, GOOS: goos, Version: "0.31.0", Author: "© 2026 Илья Соснин"}
}

func parseManifest(t *testing.T, data []byte) mcpbManifestOut {
	t.Helper()
	var m mcpbManifestOut
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("манифест не разбирается: %v\n%s", err, data)
	}
	return m
}

// TestMcpbManifest: обязательные поля v0.3, команда через user_config, путь программы только в
// default и разбирается обратно (кавычки, обратные слэши, кириллица, пробелы), платформа своей ОС.
func TestMcpbManifest(t *testing.T) {
	exes := map[string]string{
		"windows": `C:\Users\Иван "Ваня"\Sosnina Albums\SosninaAlbums-0.31.0.exe`,
		"darwin":  `/Applications/Sosnina "Albums".app/Contents/MacOS/sosnina\albums`,
	}
	plat := map[string]string{"windows": "win32", "darwin": "darwin"}
	for goos, exe := range exes {
		raw, err := McpbManifest(mcpbInfo(exe, goos))
		if err != nil {
			t.Fatal(goos, err)
		}
		m := parseManifest(t, raw)
		if m.ManifestVersion != "0.3" || m.Name != "sosnina-albums" || m.Version != "0.31.0" || m.Icon != "icon.png" ||
			m.Author.Name != "© 2026 Илья Соснин" {
			t.Fatalf("%s: поля: %+v", goos, m)
		}
		if m.Server.Type != "binary" || m.Server.EntryPoint != "server/README.txt" ||
			m.Server.McpConfig.Command != "${user_config.program}" || !slices.Equal(m.Server.McpConfig.Args, []string{"--mcp"}) {
			t.Fatalf("%s: server: %+v", goos, m.Server)
		}
		p, ok := m.UserConfig["program"]
		if !ok || p.Type != "file" || p.Required || p.Default != exe {
			t.Fatalf("%s: user_config: %+v (путь %q)", goos, m.UserConfig, exe)
		}
		if !slices.Equal(m.Compat.Platforms, []string{plat[goos]}) {
			t.Fatalf("%s: платформы %v", goos, m.Compat.Platforms)
		}
		// Путь программы встречается в манифесте один раз, в default (command — подстановка).
		if n := strings.Count(string(raw), "SosninaAlbums-0.31.0.exe") + strings.Count(string(raw), "sosnina\\\\albums"); n != 1 {
			t.Fatalf("%s: путь в манифесте %d раз:\n%s", goos, n, raw)
		}
	}
}

// TestMcpbManifestBad: пустые, огромные, с управляющими знаками пути и чужая ОС — ошибка без паники и без пути.
func TestMcpbManifestBad(t *testing.T) {
	long := strings.Repeat("a", mcpbMaxExe+1)
	for name, in := range map[string]McpbInfo{
		"пустой путь": mcpbInfo("", "windows"),
		"длинный":     mcpbInfo(long, "windows"),
		"перевод":     mcpbInfo("C:\\a\nb.exe", "windows"),
		"NUL":         mcpbInfo("/a\x00b", "darwin"),
		"не UTF-8":    mcpbInfo("/a\xffb", "darwin"),
		"DEL":         mcpbInfo("/a\x7fb", "darwin"),
		"linux":       mcpbInfo("/usr/bin/sosnina", "linux"),
		"без ОС":      mcpbInfo("/usr/bin/sosnina", ""),
	} {
		raw, err := Mcpb(in)
		if err == nil || raw != nil {
			t.Fatalf("%s: ждали ошибку, получили %d байт", name, len(raw))
		}
		if strings.Contains(err.Error(), in.Exe) && in.Exe != "" {
			t.Fatalf("%s: путь в тексте ошибки: %v", name, err)
		}
	}
	// Предел — ровно mcpbMaxExe байт ещё годится.
	if _, err := Mcpb(mcpbInfo(strings.Repeat("a", mcpbMaxExe), "windows")); err != nil {
		t.Fatal("путь на пределе:", err)
	}
}

// TestMcpbZip: ровно 3 записи, имена относительные и без «..», entry_point есть в архиве,
// значок — PNG 512×512, manifest.json разбирается, архив не бьётся и мал.
func TestMcpbZip(t *testing.T) {
	raw, err := Mcpb(mcpbInfo(`C:\Program Files\Sosnina Albums\SosninaAlbums.exe`, "windows"))
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 3 {
		t.Fatalf("записей %d, ждали 3", len(zr.File))
	}
	body := map[string][]byte{}
	for _, f := range zr.File {
		n := f.Name
		if strings.HasPrefix(n, "/") || strings.HasPrefix(n, `\`) || strings.Contains(n, "..") || strings.Contains(n, `\`) ||
			strings.Contains(n, ":") || f.FileInfo().IsDir() {
			t.Fatalf("плохое имя записи %q", n)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(n, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(n, err)
		}
		body[n] = b
	}
	for _, n := range []string{"manifest.json", "icon.png", "server/README.txt"} {
		if len(body[n]) == 0 {
			t.Fatalf("нет записи %q (есть %v)", n, len(body))
		}
	}
	m := parseManifest(t, body["manifest.json"])
	if len(body[m.Server.EntryPoint]) == 0 || len(body[m.Icon]) == 0 {
		t.Fatalf("entry_point %q или icon %q нет в архиве", m.Server.EntryPoint, m.Icon)
	}
	img, err := png.Decode(bytes.NewReader(body["icon.png"]))
	if err != nil {
		t.Fatal("значок:", err)
	}
	if b := img.Bounds(); b.Dx() != 512 || b.Dy() != 512 {
		t.Fatalf("значок %dx%d, ждали 512x512", b.Dx(), b.Dy())
	}
	if len(raw) > 100<<10 {
		t.Fatalf("архив %d байт — слишком велик", len(raw))
	}
	// Два вызова дают один и тот же архив (без времени и случайности).
	again, _ := Mcpb(mcpbInfo(`C:\Program Files\Sosnina Albums\SosninaAlbums.exe`, "windows"))
	if !bytes.Equal(raw, again) {
		t.Fatal("архив не воспроизводится")
	}
}

// TestAgentsSetups: кнопка файла — только у Claude Desktop и только на Windows и Mac; пустой путь — пусто.
func TestAgentsSetups(t *testing.T) {
	for goos, want := range map[string]bool{"windows": true, "darwin": true, "linux": false} {
		list := Setups("/x/sosnina", goos)
		if len(list) != 4 {
			t.Fatalf("%s: клиентов %d", goos, len(list))
		}
		for _, s := range list {
			if s.File != (want && s.ID == SetupClaudeDesktop) {
				t.Fatalf("%s: %s file=%v", goos, s.ID, s.File)
			}
		}
	}
	if got := Setups("", "windows"); got == nil || len(got) != 0 {
		t.Fatalf("пустой путь: %#v", got)
	}
}

// FuzzMcpbManifest: любой путь — либо ошибка, либо манифест, где default равен исходному пути.
func FuzzMcpbManifest(f *testing.F) {
	for _, s := range []string{`C:\a b\c.exe`, `/x/"y"`, "", "\x00", `\\server\share`, "я/ё\u2028"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, exe string) {
		raw, err := McpbManifest(mcpbInfo(exe, "windows"))
		if err != nil {
			return
		}
		var m mcpbManifestOut
		if json.Unmarshal(raw, &m) != nil || m.UserConfig["program"].Default != exe {
			t.Fatalf("путь %q не вернулся из манифеста:\n%s", exe, raw)
		}
	})
}
