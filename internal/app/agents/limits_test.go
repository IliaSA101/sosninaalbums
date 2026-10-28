package agents

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Паника, отсутствие альбома, срок и места вызовов, предел ответа.

// TestToolPanicRecovered: паника в теле инструмента — isError «внутренняя ошибка программы»,
// программа и сессия живы, в log.txt — тип без значения (в нём бывает путь).
func TestToolPanicRecovered(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)
	r := newRig(t, Options{}, "")
	var boom atomic.Bool
	r.fx.hook = func() {
		if boom.Load() {
			panic("секрет /home/user/албом")
		}
	}
	r.start(t)
	cs := client(t, r.sock, "agent")
	boom.Store(true)
	for _, name := range []string{"get_album", "list_spreads", "get_photo_preview"} {
		rep := tcall(t, cs, name, map[string]any{})
		if !rep.IsError || (name != "get_photo_preview" && rep.Text != "внутренняя ошибка программы") {
			t.Fatalf("%s: %+v", name, rep)
		}
	}
	boom.Store(false)
	if rep := tcall(t, cs, "get_album", nil); rep.IsError {
		t.Fatalf("после паники: %+v", rep)
	}
	if out := logs.String(); !strings.Contains(out, "ПАНИКА") || strings.Contains(out, "секрет") || strings.Contains(out, "/home/user") {
		t.Fatalf("журнал: %q", out)
	}
}

// TestToolsNoAlbum: все инструменты в четырёх состояниях — нет альбома, пустой, без обложки, смена
// альбома (вид пуст при живом номере Editor) — отвечают данными или ошибкой из списка, но не паникой и не ошибкой протокола.
func TestToolsNoAlbum(t *testing.T) {
	r, cs := toolsRig(t, 2, Options{})
	schemas := outSchemas(t, cs)
	allowed := []string{"альбом не открыт", "нет такого разворота", "нет такого фото", "inAlbum есть только"}
	check := func(state string) {
		for _, c := range allCalls() {
			rep := tcall(t, cs, c.name, c.args)
			if !rep.IsError {
				continue
			}
			ok := false
			for _, a := range allowed {
				ok = ok || strings.Contains(rep.Text, a)
			}
			if !ok {
				t.Errorf("%s: %s %v: %q", state, c.name, c.args, rep.Text)
			}
		}
	}
	// Нет альбома: сводка и развороты — «альбом не открыт», библиотека отвечает.
	r.fx.mu.Lock()
	r.fx.ed = 0
	r.fx.mu.Unlock()
	check("нет альбома")
	for _, n := range []string{"get_album", "list_spreads", "list_photos"} {
		if rep := tcall(t, cs, n, nil); !rep.IsError || !strings.Contains(rep.Text, "альбом не открыт") {
			t.Errorf("%s без альбома: %+v", n, rep)
		}
	}
	for _, n := range []string{"list_packs", "list_print_profiles", "list_albums"} {
		mustOK(t, cs, schemas, n, nil)
	}
	// Пустой альбом: ни разворотов, ни фото, ни обложки, ни папок.
	r.fx.mu.Lock()
	r.fx.ed = 9
	r.fx.view = model.View{Product: model.ProductAlbum, Settings: model.DefaultSettings()}
	r.fx.photos, r.fx.missing, r.fx.unread, r.fx.marks = nil, nil, nil, nil
	r.fx.mu.Unlock()
	check("пустой альбом")
	for _, n := range []string{"get_album", "list_spreads", "list_photos"} {
		mustOK(t, cs, schemas, n, nil)
	}
	if rep := tcall(t, cs, "get_spread", map[string]any{"spread": "cover"}); !rep.IsError || !strings.Contains(rep.Text, "нет такого разворота") {
		t.Errorf("обложки нет: %+v", rep)
	}
	// Смена альбома: пока Editor меняют, вид может быть пустым при непустом номере (падение Host — TestToolPanicRecovered).
	r.fx.mu.Lock()
	r.fx.view = model.View{}
	r.fx.mu.Unlock()
	check("смена альбома")
	mustOK(t, cs, schemas, "get_album", nil)
}

// TestTimeoutHoldsSlot: вызов застрял — агент получает «время вышло», но место вызова держится до
// конца горутины: следующий вызов (предел 1) — «занято»; когда застрявший кончился, место свободно.
func TestTimeoutHoldsSlot(t *testing.T) {
	r := newRig(t, Options{MaxCalls: 1, CallTimeout: 150 * time.Millisecond}, "")
	release := make(chan struct{})
	var first atomic.Bool
	r.fx.hook = func() {
		if first.CompareAndSwap(false, true) {
			<-release
		}
	}
	r.start(t)
	cs := client(t, r.sock, "agent")
	if rep := tcall(t, cs, "get_album", nil); !rep.IsError || !strings.Contains(rep.Text, "время вышло") {
		t.Fatalf("первый: %+v", rep)
	}
	if rep := tcall(t, cs, "list_packs", nil); !rep.IsError || !strings.Contains(rep.Text, "занято") {
		t.Fatalf("второй при занятом месте: %+v", rep)
	}
	close(release)
	waitFor(t, "место освободилось", func() bool { return !tcall(t, cs, "list_packs", nil).IsError })
}

// rawCall посылает tools/call голым подключением и возвращает строку ответа как она на проводе.
func rawCall(t *testing.T, c net.Conn, br *bufio.Reader, id int, name string, args map[string]any) []byte {
	t.Helper()
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
	_ = c.SetDeadline(time.Now().Add(testkit.Limit(10 * time.Second)))
	if _, err := c.Write(append(req, '\n')); err != nil {
		t.Fatal(err)
	}
	for {
		line, err := br.ReadBytes('\n')
		if err != nil {
			t.Fatalf("ответ: %v", err)
		}
		if bytes.Contains(line, []byte(`"id":`+strconv.Itoa(id)+`,`)) || bytes.Contains(line, []byte(`"id":`+strconv.Itoa(id)+`}`)) {
			return line
		}
	}
}

// TestResponseCap: предел измеряется по строке на проводе (настоящий сокет): крупный, но допустимый
// ответ проходит и укладывается в 1 МБ, слишком большой — «ответ слишком велик», а не обрезок.
func TestResponseCap(t *testing.T) {
	r := newRig(t, Options{}, "")
	setLevel(r.cfg, 2)
	big := strings.Repeat("я", maxText)
	mk := func(n int) []model.TextView {
		ts := make([]model.TextView, n)
		for i := range ts {
			ts[i] = model.TextView{ID: "t" + strconv.Itoa(i), Text: big}
		}
		return ts
	}
	r.fx.view.Spreads = manySpreads(200)
	r.fx.view.Spreads[0].Texts = mk(40) // ≈ 160 КБ текста, дважды в ответе
	r.fx.photos = nil
	for i := range 5000 {
		r.fx.photos = append(r.fx.photos, photoN(i))
	}
	r.start(t)
	c := raw(t, r.sock)
	br := handshake(t, c)
	for i, a := range []struct {
		name string
		args map[string]any
		ok   bool
	}{
		{"get_spread", map[string]any{"spread": 1}, true},
		{"list_spreads", map[string]any{"limit": 100}, true},
		{"list_photos", map[string]any{"limit": 200}, true},
		{"get_album", nil, true},
	} {
		line := rawCall(t, c, br, 10+i, a.name, a.args)
		if len(line) > 1<<20 || bytes.Contains(line, []byte(`"isError":true`)) {
			t.Fatalf("%s: строка %d байт, ошибка: %v", a.name, len(line), bytes.Contains(line, []byte(`"isError":true`)))
		}
	}
	r.fx.mu.Lock()
	r.fx.view.Spreads[0].Texts = mk(200) // ≈ 800 КБ текста — вдвое больше предела
	r.fx.mu.Unlock()
	line := rawCall(t, c, br, 99, "get_spread", map[string]any{"spread": 1})
	if bytes.Contains(line, []byte("limit")) { // у get_spread limit нет: подсказка про него увела бы агента по кругу
		t.Fatalf("подсказка про limit у инструмента без limit: %.200s", line)
	}
	if len(line) > 1<<20 || !bytes.Contains(line, []byte(`"isError":true`)) || !bytes.Contains(line, []byte("ответ слишком велик")) {
		t.Fatalf("слишком большой ответ: %d байт: %.200s", len(line), line)
	}
}
