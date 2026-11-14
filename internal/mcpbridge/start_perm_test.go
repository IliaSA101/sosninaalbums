package mcpbridge

import (
	"os"
	"testing"
)

// TestStartProgramDenied: agentPerms.tools.start_program не «Всегда» — посредник не показывает инструмент,
// вызов отвечает как чужому, запуска и отметки нет (150, 0.34-150-2); незнакомое значение — запрет.
// Нет ключа, «allow» и запреты других инструментов start_program не трогают.
func TestStartProgramDenied(t *testing.T) {
	const on = `"agents":{"on":true,"level":1}`
	hidden := map[string]string{
		"deny":              `{` + on + `,"agentPerms":{"tools":{"start_program":"deny"}}}`,
		"ask (его нет)":     `{` + on + `,"agentPerms":{"tools":{"start_program":"ask"}}}`,
		"чужое слово":       `{` + on + `,"agentPerms":{"tools":{"start_program":"Allow"}}}`,
		"пустая строка":     `{` + on + `,"agentPerms":{"tools":{"start_program":""}}}`,
		"число":             `{` + on + `,"agentPerms":{"tools":{"start_program":1}}}`,
		"null":              `{` + on + `,"agentPerms":{"tools":{"start_program":null}}}`,
		"tools — массив":    `{` + on + `,"agentPerms":{"tools":["allow"]}}`,
		"agentPerms — лист": `{` + on + `,"agentPerms":[]}`,
		"усечённый":         `{` + on + `,"agentPerms":{"tools":{"start_prog`,
		"доступ выключен":   `{"agents":{"on":false},"agentPerms":{"tools":{"start_program":"allow"}}}`,
	}
	for name, text := range hidden {
		t.Run("скрыт/"+name, func(t *testing.T) {
			fx := newStartFx(t)
			fx.setConfig(text)
			h := fx.bridge()
			if got := toolNames(t, h); len(got) != 1 || got[0] != "program_status" {
				t.Errorf("tools/list: %v", got)
			}
			if txt, isErr := callStart(t, h, `{}`); !isErr || txt != textClosed {
				t.Errorf("вызов: %q, ошибка %v", txt, isErr)
			}
			if len(fx.spawns) != 0 {
				t.Error("запуск при запрете")
			}
			if _, err := os.Stat(fx.markPath()); err == nil {
				t.Error("отметка записана при запрете")
			}
		})
	}
	shown := map[string]string{
		"нет agentPerms":          `{` + on + `}`,
		"allow":                   `{` + on + `,"agentPerms":{"tools":{"start_program":"allow"}}}`,
		"tools null":              `{` + on + `,"agentPerms":{"tools":null}}`,
		"нет start_program":       `{` + on + `,"agentPerms":{"tools":{"autobuild":"deny","create_album":"ask"}}}`,
		"agentPerms null":         `{` + on + `,"agentPerms":null}`,
		"запрещены все правки":    `{` + on + `,"agentPerms":{"tools":{"autobuild":"deny","add_folder":"deny","show_spread":"deny"},"edit":false}}`,
		"чужой ключ в agentPerms": `{` + on + `,"agentPerms":{"future":1}}`,
	}
	for name, text := range shown {
		t.Run("виден/"+name, func(t *testing.T) {
			fx := newStartFx(t)
			fx.setConfig(text)
			if got := toolNames(t, fx.bridge()); len(got) != 2 {
				t.Errorf("tools/list: %v", got)
			}
		})
	}
	t.Run("смена на лету", func(t *testing.T) { // посредник читает config.json на каждый запрос
		fx := newStartFx(t)
		h := fx.bridge()
		if len(toolNames(t, h)) != 2 {
			t.Fatal("start_program нет до запрета")
		}
		fx.setConfig(hidden["deny"])
		if got := toolNames(t, h); len(got) != 1 {
			t.Errorf("после запрета: %v", got)
		}
		fx.setConfig(shown["allow"])
		if got := toolNames(t, h); len(got) != 2 {
			t.Errorf("после возврата: %v", got)
		}
	})
}
