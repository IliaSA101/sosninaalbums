package app

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// TestAskToolKinds: вид вопроса -> инструмент разрешений (0.35/152): «Запретить» у строки снимает
// вопрос её вида; незнакомый и пустой вид — add_folder, как было.
func TestAskToolKinds(t *testing.T) {
	for kind, want := range map[string]string{
		agents.AskCreate: "create_album",
		agents.AskExport: "export_album",
		agents.AskOpen:   "open_album",
		agents.AskAdd:    "add_folder",
		agents.AskEdit:   "add_folder",
		"":               "add_folder",
		"новый":          "add_folder",
	} {
		if got := askTool(kind); got != want {
			t.Errorf("askTool(%q) = %q, ждали %q", kind, got, want)
		}
	}
	// Каждый инструмент вида известен разрешениям (иначе строка в окне и вопрос разошлись бы).
	for _, kind := range []string{agents.AskCreate, agents.AskExport, agents.AskOpen} {
		if prefs.AgentToolClass(askTool(kind)) == "" {
			t.Errorf("%s: инструмент %q неизвестен prefs", kind, askTool(kind))
		}
	}
}
