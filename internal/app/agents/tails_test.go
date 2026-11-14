package agents

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// TestUnknownToolRussian: вызов инструмента, которого нет (список клиента от другой версии),
// — результат isError с русским текстом и подсказкой перечитать список, а не ошибка протокола с
// английским «unknown tool»; сессия жива, настоящие инструменты отвечают. Чужое имя — без
// переводов строк и не длиннее предела; пустое имя — тоже отказ.
func TestUnknownToolRussian(t *testing.T) {
	_, cs := toolsRig(t, 2, Options{})
	long := strings.Repeat("я", 500)
	for _, name := range []string{"get_album_v2", "", "a\nb\x00c", long, "GET_ALBUM"} {
		res, err := cs.CallTool(callCtx(t), &mcp.CallToolParams{Name: name, Arguments: map[string]any{}})
		if err != nil {
			t.Fatalf("%q: ошибка протокола вместо результата: %v", name, err)
		}
		var text string
		for _, c := range res.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				text = tc.Text
			}
		}
		if !res.IsError || !strings.Contains(text, "нет в этой версии Sosnina Albums") || !strings.Contains(text, "tools/list") {
			t.Errorf("%q: %+v %q", name, res.IsError, text)
		}
		if name == "get_album_v2" && !strings.Contains(text, "«get_album_v2»") {
			t.Errorf("имя не попало в текст: %q", text)
		}
		if strings.Contains(text, "unknown") || strings.Contains(text, "\n") || len([]rune(text)) > 300 {
			t.Errorf("%q: текст %q", name, text)
		}
	}
	if r := tcall(t, cs, "get_album", nil); r.IsError {
		t.Errorf("get_album после отказов: %+v", r)
	}
}

// TestUnknownToolOtherErrors: слой трогает только «unknown tool»; ошибка схемы входа остаётся
// отказом инструмента с прежним текстом (не подменяется на «нет инструмента»).
func TestUnknownToolOtherErrors(t *testing.T) {
	_, cs := toolsRig(t, 3, Options{})
	r := tcall(t, cs, "get_photo_preview", map[string]any{"id": idA, "size": 9999})
	if !r.IsError || !strings.HasPrefix(r.Text, "неверные аргументы вызова: ") || strings.Contains(r.Text, "нет в этой версии") {
		t.Errorf("%+v", r)
	}
}

// TestUnnamedAlbumOneWord: безымянный альбом (нет файла) зовётся одним словом на всех уровнях —
// как в интерфейсе («Альбом»); альбом с файлом на уровне 2 — по файлу. Список «Последних» с
// пустым путём и именем — тоже «Альбом».
func TestUnnamedAlbumOneWord(t *testing.T) {
	for _, level := range []int{1, 2, 3} {
		r := newRig(t, Options{}, "")
		r.fx.view.Path = ""
		setLevel(r.cfg, level)
		r.start(t)
		cs := client(t, r.sock, "test-agent")
		if got := tcall(t, cs, "get_album", nil).Data["name"]; got != "Альбом" {
			t.Errorf("уровень %d: %v", level, got)
		}
	}
	if got := recentName(prefs.RecentProject{}); got != "Альбом" {
		t.Errorf("recentName без имени и пути: %q", got)
	}
	if albumName("") != "Альбом" || albumName("/x/y.salbum") != "y" || albumName("y") != "y" {
		t.Errorf("albumName: %q %q", albumName(""), albumName("/x/y.salbum"))
	}
}
