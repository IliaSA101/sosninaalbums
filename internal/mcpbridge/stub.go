package mcpbridge

import (
	"encoding/json"
)

// stub — ответ на запрос клиента, пока программы нет (design §4.2). initialize — свой ответ с
// версией не выше 2025-11-25; tools/list — один program_status; его вызов — текст причины
// (b.why: закрыта, небезопасно, много подключений…), вызов любого другого инструмента — тот же текст
// с isError, прочие методы — ошибка JSON-RPC с тем же текстом; ping — пустой ответ;
// server/discover (ревизия 2026-07-28) — ошибка «нет метода»: клиент откатывается на initialize.
func (b *bridge) stub(h *header) {
	switch h.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(h.Params, &p)
		b.negotiated = stubVersion(p.ProtocolVersion)
		b.initDelivered = true
		b.emit(resultLine(h.ID, map[string]any{
			"protocolVersion": b.negotiated,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": true}},
			"serverInfo":      map[string]any{"name": stubName, "version": "bridge"},
		}))
	case "ping":
		b.emit(resultLine(h.ID, map[string]any{}))
	case "tools/list":
		b.emit(resultLine(h.ID, map[string]any{"tools": []any{map[string]any{
			"name":        statusTool,
			"description": statusToolDesc,
			"inputSchema": map[string]any{"type": "object", "additionalProperties": false},
			"annotations": map[string]any{"readOnlyHint": true},
		}}}))
	case "tools/call":
		var p struct {
			Name string `json:"name"`
		}
		// Любой другой инструмент — тоже результат с причиной, а не JSON-RPC-ошибка: клиент мог
		// закешировать прежний список и вызвать get_album, не дождавшись list_changed (0.31-143-85).
		res := map[string]any{"content": []any{map[string]any{"type": "text", "text": b.why}}}
		if json.Unmarshal(h.Params, &p) != nil || p.Name != statusTool {
			res["isError"] = true
		}
		b.emit(resultLine(h.ID, res))
	case "server/discover":
		b.emit(errorLine(h.ID, codeNoMethod, textNoMeth))
	default:
		b.emit(errorLine(h.ID, codeServer, b.why))
	}
}
