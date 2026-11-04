package agents

import (
	"archive/zip"
	"bytes"
	_ "embed" // значок расширения
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

// Файл .mcpb для Claude Desktop (design §4.6, 0.31-143-31): zip из ровно трёх записей. Путь
// программы свой на каждой машине, поэтому архив собирает программа. Здесь только байты: файл
// пишет фасад App (служба агентов файлов не пишет, 0.31-143-34).

//go:embed mcpb_icon.png
var mcpbIcon []byte // 512×512, из build/appicon.png

// Записи архива: других в него не попадает (TestMcpbZip).
const (
	mcpbManifestName = "manifest.json"
	mcpbIconName     = "icon.png"
	mcpbEntryName    = "server/README.txt"
	mcpbReadme       = "Расширение запускает установленную программу Sosnina Albums с ключом --mcp; самой программы в этом файле нет.\n"
	mcpbMaxExe       = 4096 // длиннее путей не бывает; защита от мусора на входе
)

// Ошибки сборки: тексты без путей (идут в тост).
var (
	ErrMcpbPlatform = errors.New("файл для Claude Desktop — только Windows и Mac")
	ErrMcpbExe      = errors.New("путь программы неизвестен")
)

// McpbInfo — что нужно манифесту.
type McpbInfo struct {
	Exe     string // путь программы (os.Executable)
	GOOS    string // "windows" или "darwin"
	Version string // версия программы
	Author  string // автор (wails.json → info.copyright)
}

// mcpbPlatform — имя платформы в compatibility.platforms.
func mcpbPlatform(goos string) (string, bool) {
	switch goos {
	case "windows":
		return "win32", true
	case "darwin":
		return "darwin", true
	}
	return "", false
}

// McpbManifest — manifest.json (схема v0.3). Путь программы лежит только в user_config.program.default;
// собирается encoding/json, поэтому кавычки и обратные слэши пути экранируются.
func McpbManifest(in McpbInfo) ([]byte, error) {
	platform, ok := mcpbPlatform(in.GOOS)
	if !ok {
		return nil, ErrMcpbPlatform
	}
	if in.Exe == "" || len(in.Exe) > mcpbMaxExe || !utf8.ValidString(in.Exe) { // не-UTF-8 JSON подменил бы на U+FFFD
		return nil, ErrMcpbExe
	}
	for _, r := range in.Exe {
		if r < ' ' || r == 0x7f {
			return nil, ErrMcpbExe
		}
	}
	m := map[string]any{
		"manifest_version": "0.3",
		"name":             serverName,
		"display_name":     "Sosnina Albums",
		"version":          in.Version,
		"description":      "Чтение открытого альбома: развороты, места, оформление, наборы",
		"author":           map[string]string{"name": in.Author},
		"homepage":         "https://github.com/IliaSA101/sosninaalbums",
		"license":          "GPL-3.0-only",
		"icon":             mcpbIconName,
		"server": map[string]any{
			"type":        "binary",
			"entry_point": mcpbEntryName,
			"mcp_config":  map[string]any{"command": "${user_config.program}", "args": []string{"--mcp"}},
		},
		"user_config": map[string]any{
			"program": map[string]any{
				"type":        "file",
				"title":       "Программа Sosnina Albums",
				"description": "Файл программы; после обновления переносной версии выберите новый",
				"required":    false,
				"default":     in.Exe,
			},
		},
		"tools_generated": true,
		"compatibility":   map[string]any{"platforms": []string{platform}},
	}
	return json.MarshalIndent(m, "", "  ")
}

// Mcpb — архив .mcpb в памяти: manifest.json, icon.png, server/README.txt.
func Mcpb(in McpbInfo) ([]byte, error) {
	manifest, err := McpbManifest(in)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := []struct {
		name   string
		data   []byte
		method uint16
	}{
		{mcpbManifestName, manifest, zip.Deflate},
		{mcpbIconName, mcpbIcon, zip.Store}, // PNG уже сжат
		{mcpbEntryName, []byte(mcpbReadme), zip.Deflate},
	}
	for _, f := range files {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: f.name, Method: f.method})
		if err == nil {
			_, err = w.Write(f.data)
		}
		if err != nil {
			return nil, fmt.Errorf("сборка архива: %w", err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("сборка архива: %w", err)
	}
	return buf.Bytes(), nil
}
