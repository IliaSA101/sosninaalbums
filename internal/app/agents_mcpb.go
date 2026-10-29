package app

// Файл .mcpb для Claude Desktop (спека 0.31/143, design §4.6, Г7): байты собирает agents.Mcpb,
// диалог и запись — здесь, по образцу ExportTemplates (служба агентов файлов не пишет).

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// dirAgents — своя последняя папка диалога «Сохранить файл для Claude Desktop».
const dirAgents = "agents"

// mcpbGOOS — ОС для файла; тесты подменяют, чтобы проверить Windows и Mac на Linux.
var mcpbGOOS = runtime.GOOS

var mcpbFilter = []wr.FileFilter{{DisplayName: "Расширение Claude Desktop (*.mcpb)", Pattern: "*.mcpb"}}

// Тексты отказов в тост: без путей.
var (
	errMcpbExe   = errors.New("путь программы временный — перенесите программу в постоянную папку и откройте окно снова")
	errMcpbLocal = errors.New("сохраните файл на диске этого компьютера")
	errMcpbSave  = errors.New("не удалось сохранить файл для Claude Desktop")
)

// mcpbToast — Б1: в файле путь программы с именем пользователя ОС.
const mcpbToast = "Файл сохранён — он для этого компьютера (в нём путь к программе), другим не пересылайте. " +
	"Откройте его двойным щелчком, Claude Desktop предложит установить"

// SaveClaudeExtension — диалог сохранения и файл .mcpb для Claude Desktop (только Windows и Mac).
// Отказ в диалоге — Canceled без ошибки.
func (a *App) SaveClaudeExtension() (res AgentsSaved, err error) {
	defer guardErr("SaveClaudeExtension", &err)
	if mcpbGOOS != "windows" && mcpbGOOS != "darwin" {
		return res, agents.ErrMcpbPlatform
	}
	exe, _ := a.agentsExe()
	if osx.ExeHint(exe, mcpbGOOS, os.TempDir(), isRegular) != "" {
		return res, errMcpbExe
	}
	data, err := agents.Mcpb(agents.McpbInfo{Exe: exe, GOOS: mcpbGOOS, Version: appVersion(wailsJSON), Author: wailsCopyright(wailsJSON)})
	if err != nil {
		return res, err
	}
	path, err := saveFileDialog(a.ctx, wr.SaveDialogOptions{
		Title:            "Сохранить файл для Claude Desktop",
		DefaultDirectory: a.dialogDir(dirAgents),
		DefaultFilename:  "Sosnina Albums.mcpb",
		Filters:          mcpbFilter,
	})
	if err != nil {
		return res, err
	}
	if path == "" {
		return AgentsSaved{Canceled: true}, nil
	}
	added := !strings.EqualFold(filepath.Ext(path), ".mcpb")
	if added {
		path += ".mcpb"
	}
	if paths.CheckLocalPath(path) != nil {
		return res, errMcpbLocal
	}
	// Файла с дописанным «.mcpb» диалог не видел: молча перезаписать его нельзя.
	if added {
		if _, err := os.Lstat(path); err == nil {
			return res, fmt.Errorf("файл уже есть: %s", filepath.Base(path))
		}
	}
	if err := library.WriteFileAtomic(path, data); err != nil {
		log.Printf("файл для Claude Desktop: %v", paths.NoPath(err))
		return res, errMcpbSave
	}
	a.rememberDir(dirAgents, filepath.Dir(path))
	return AgentsSaved{Text: mcpbToast}, nil
}

// wailsCopyright — info.copyright из wails.json (автор в манифесте .mcpb).
func wailsCopyright(raw []byte) string {
	var cfg struct {
		Info struct {
			Copyright string `json:"copyright"`
		} `json:"info"`
	}
	if json.Unmarshal(raw, &cfg) != nil {
		return ""
	}
	return cfg.Info.Copyright
}
