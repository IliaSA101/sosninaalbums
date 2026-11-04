package agents

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// maxToolName — сколько знаков чужого имени инструмента попадает в ответ.
const maxToolName = 64

// unknownTool — приёмный слой: вызов инструмента, которого нет, SDK отвечает ошибкой протокола
// с английским «unknown tool "имя"». Клиент со списком от другой версии программы получает её как сбой
// транспорта; здесь она становится обычным отказом инструмента (isError) с русским текстом и
// подсказкой перечитать список. Остальные ошибки и ответы не трогаются.
func unknownTool(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		res, err := next(ctx, method, req)
		// Error() ошибки протокола SDK — её Message; jsonrpc для depguard закрыт, поэтому по тексту.
		if method != "tools/call" || err == nil || !strings.HasPrefix(err.Error(), "unknown tool ") {
			return res, err
		}
		name := ""
		if cr, ok := req.(*mcp.CallToolRequest); ok && cr.Params != nil {
			name = cleanLine(cr.Params.Name, maxToolName)
		}
		return errResult(errNoTool(name)), nil
	}
}

// errNoTool — отказ на вызов инструмента, которого нет в этой версии программы.
func errNoTool(name string) *agentErr {
	return newAgentErr("инструмента «" + name + "» нет в этой версии Sosnina Albums — список инструментов " +
		"изменился: перечитайте его (tools/list) или переподключите агента")
}
