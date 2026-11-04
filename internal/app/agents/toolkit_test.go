package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Общее для тестов инструментов: служба на сокете, клиент SDK, вызов и разбор ответа.

// toolsRig — запущенная служба с подставным Host на уровне level и клиент к ней.
func toolsRig(t *testing.T, level int, o Options) (*rig, *mcp.ClientSession) {
	t.Helper()
	r := newRig(t, o, "")
	setLevel(r.cfg, level)
	r.start(t)
	return r, client(t, r.sock, "test-agent")
}

// reply — ответ инструмента: признак ошибки, текст и структура.
type reply struct {
	IsError bool
	Text    string
	Data    map[string]any
	Raw     json.RawMessage // StructuredContent как пришёл
}

func callCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(15*time.Second))
	t.Cleanup(cancel)
	return ctx
}

// tcall вызывает инструмент; ошибка протокола — провал теста.
func tcall(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) reply {
	t.Helper()
	res, err := cs.CallTool(callCtx(t), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: ошибка протокола: %v", name, err)
	}
	r := reply{IsError: res.IsError}
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			r.Text = tc.Text
			break
		}
	}
	if res.StructuredContent != nil {
		b, _ := json.Marshal(res.StructuredContent)
		r.Raw = b
		_ = json.Unmarshal(b, &r.Data)
	}
	return r
}

// outSchemas — схемы выхода инструментов с сервера.
func outSchemas(t *testing.T, cs *mcp.ClientSession) map[string]*jsonschema.Resolved {
	t.Helper()
	res, err := cs.ListTools(callCtx(t), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	out := map[string]*jsonschema.Resolved{}
	for _, tl := range res.Tools {
		b, _ := json.Marshal(tl.OutputSchema)
		var s jsonschema.Schema
		if err := json.Unmarshal(b, &s); err != nil {
			t.Fatalf("%s: схема выхода: %v", tl.Name, err)
		}
		rs, err := s.Resolve(nil)
		if err != nil {
			t.Fatalf("%s: схема выхода: %v", tl.Name, err)
		}
		out[tl.Name] = rs
	}
	return out
}

// mustOK — вызов прошёл, ответ соответствует схеме выхода, Content = StructuredContent.
func mustOK(t *testing.T, cs *mcp.ClientSession, schemas map[string]*jsonschema.Resolved, name string, args map[string]any) reply {
	t.Helper()
	r := tcall(t, cs, name, args)
	if r.IsError {
		t.Fatalf("%s %v: ошибка %q", name, args, r.Text)
	}
	var v, txt any
	_ = json.Unmarshal(r.Raw, &v)
	if err := json.Unmarshal([]byte(r.Text), &txt); err != nil || !reflect.DeepEqual(v, txt) {
		t.Fatalf("%s: Content и StructuredContent разошлись", name)
	}
	if rs := schemas[name]; rs != nil {
		if err := rs.Validate(&v); err != nil {
			t.Fatalf("%s: ответ не по схеме выхода: %v\n%s", name, err, r.Text)
		}
	}
	return r
}

// allCalls — по вызову каждого инструмента по всем веткам входа (кроме превью — они отдельно).
func allCalls() []struct {
	name string
	args map[string]any
} {
	type c = struct {
		name string
		args map[string]any
	}
	m := map[string]any{}
	out := []c{{"get_album", m}, {"list_spreads", m}, {"list_spreads", map[string]any{"offset": 1, "limit": 1}},
		{"get_spread", map[string]any{"spread": 1}}, {"get_spread", map[string]any{"spread": "2"}}, {"get_spread", map[string]any{"spread": "cover"}},
		{"get_spread", map[string]any{"spread": "sp1"}}, {"list_photos", m}, {"get_photo", map[string]any{"id": idA}},
		{"get_photo", map[string]any{"id": idC}}, {"list_packs", m}, {"list_print_profiles", m}, {"list_albums", m}}
	for _, f := range []string{"all", "used", "unused", "missing", "unreadable", "marked"} {
		out = append(out, c{"list_photos", map[string]any{"filter": f}})
	}
	out = append(out, c{"list_photos", map[string]any{"order": "name", "folder": 1, "bin": "b1"}})
	for _, k := range []string{"bg", "sticker", "frame", "spread", "look", "template"} {
		out = append(out, c{"list_decor", map[string]any{"kind": k}})
	}
	for _, k := range []string{"bg", "sticker", "frame", "look"} {
		out = append(out, c{"list_decor", map[string]any{"kind": k, "inAlbum": true}})
	}
	out = append(out, c{"list_decor", map[string]any{"kind": "sticker", "query": "зв", "theme": "basic"}})
	return out
}

// call2 — вызов для горутин теста (t.Fatal из них нельзя): "" — успех, иначе причина.
func call2(cs *mcp.ClientSession, name string, args map[string]any) string {
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(15*time.Second))
	defer cancel()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	switch {
	case err != nil:
		return err.Error()
	case res.IsError:
		return "isError: " + textOf(res)
	}
	return ""
}

func textOf(res *mcp.CallToolResult) string {
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// photoN — n-е фото ленты для проверок страниц (время съёмки возрастает с n).
func photoN(n int) library.Photo {
	id := fmt.Sprintf("%016x", 0xbb00000000+n)
	return library.Photo{ID: id, Path: "/albums/ZQ2dir/p" + strconv.Itoa(n) + ".jpg", Name: "p" + strconv.Itoa(n) + ".jpg", W: 10, H: 10, Shot: int64(1_600_000_000 + n)}
}
