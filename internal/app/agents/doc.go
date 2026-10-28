// Package agents — служба MCP (чтение альбома агентом, спека 0.31/143): сервер на сокете ОС
// поверх go-sdk. Здесь пока только якорь зависимости (Т0); код служб — задачи Г1…Г5.
package agents

import (
	// Пустой импорт держит go-sdk в go.mod (go mod tidy убирает неиспользуемые модули) и в составе
	// THIRD_PARTY_NOTICES.txt (генератор берёт go list ./...). Уйдёт, когда код службы импортирует mcp.
	_ "github.com/modelcontextprotocol/go-sdk/mcp"
)
