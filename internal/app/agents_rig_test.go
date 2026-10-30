package app

// Стенд тестов фасада службы агентов (143, Г5): настоящий App с папкой данных, альбомом и фото на
// диске, служба на сокете в короткой папке и клиент SDK. Тесты рядом: agents_readonly_test.go
// (инструменты ничего не пишут), agents_preview_test.go (превью и журнал), agents_clients_test.go.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// shortTemp — короткая папка для сокета: t.TempDir бывает длиннее sun_path (maxSockPath = 100).
func shortTemp(t *testing.T) string {
	t.Helper()
	base := os.TempDir()
	if runtime.GOOS != "windows" {
		base = "/tmp"
	}
	d, err := os.MkdirTemp(base, "sa")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	return d
}

// agRig — App с открытым альбомом и службой агентов.
type agRig struct {
	a      *App
	cfg    string // папка данных: config.json, templates.json, mcp/sock
	album  string // файл .salbum
	photos []string
	sock   string
	cs     *mcp.ClientSession

	mu     sync.Mutex
	events []string
}

// eventNames — имена всех событий с начала теста.
func (r *agRig) eventNames() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

// albumName — имя альбома стенда: уникальное, не совпадает ни с темой каталога («Свадьба»), ни с
// другими метками; noLabels ищет его без пути и расширения.
const albumName = "Альбом Лисицыных"

// newAgRig собирает стенд: 5 JPEG (один усечён), разворотов по автосборке, одно фото убрано с диска
// (пропавшее), альбом сохранён и записан в «Последние». Служба не включена.
func newAgRig(t *testing.T) *agRig {
	t.Helper()
	xdg := shortTemp(t) // до newApp: удаляется после остановки фона
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", xdg)            // Mac: ~/Library/Application Support
	osx.UseRoamingDirForTest(t, xdg) // Windows: известная папка от среды не зависит
	a := newApp(t)
	a.data = "sa"
	sock := osx.MCPSock(a.data)
	if sock == "" || len(sock) > 100 {
		t.Skipf("папка настроек не подменяется средой: %q", sock)
	}
	r := &agRig{a: a, sock: sock, cfg: filepath.Dir(filepath.Dir(sock))}
	if err := os.MkdirAll(r.cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	a.cfgDir = r.cfg
	a.emit = func(name string, _ ...any) {
		r.mu.Lock()
		r.events = append(r.events, name)
		r.mu.Unlock()
	}
	cache, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = cache
	a.openDecor() // как startup: библиотека оформления в папке данных

	shots := filepath.Join(testkit.TempDir(t), "Съёмка")
	if err := os.Mkdir(shots, 0o755); err != nil {
		t.Fatal(err)
	}
	r.photos = []string{
		testkit.JPEG(t, shots, "Иванов_1.jpg", 600, 400),
		testkit.JPEG(t, shots, "Петрова_2.jpg", 400, 600),
		testkit.JPEG(t, shots, "Сидоров_3.jpg", 600, 400),
		testkit.JPEG(t, shots, "Пропало_4.jpg", 600, 400),
		truncated(t, shots, "Битое_5.jpg", 400, 300, 700),
	}
	if _, err := a.AddFolderPath(shots); err != nil {
		t.Fatal(err)
	}
	waitRead(t, a)
	ids := []string{}
	for _, p := range r.photos[:4] {
		ids = append(ids, library.IDFor(p))
	}
	if _, err := a.AutoBuild(ids, -1, model.AutoOpts{Spreads: 2, MaxPer: 2, Variety: model.VarietyNormal}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(r.photos[3]); err != nil { // стоит на развороте, файла нет
		t.Fatal(err)
	}
	a.rescan()
	waitRead(t, a)
	r.album = filepath.Join(testkit.TempDir(t), albumName+".salbum")
	if err := a.editor().Save(r.album); err != nil {
		t.Fatal(err)
	}
	a.rememberProject(r.album)
	if err := a.SaveGridTemplate(a.editor().View().Spreads[0].ID, "spread", "Шаблон Козловых"); err != nil { // templates.json
		t.Fatal(err)
	}
	r.settle()
	return r
}

// settle доводит фон до покоя: прогрев миниатюр кончился, автосохранение записало черновик и
// больше не сработает. После этого любое событие и любая запись — от проверяемого кода.
func (r *agRig) settle() {
	r.a.bg.Wait()
	r.a.mu.Lock()
	if r.a.saveTimer != nil {
		r.a.saveTimer.Stop()
	}
	r.a.mu.Unlock()
	r.a.flushSave()
}

// start включает службу и подключает клиента SDK.
func (r *agRig) start(t *testing.T) {
	t.Helper()
	info, err := r.a.SetAgents(true)
	if err != nil || info.Status != "on" {
		t.Fatalf("SetAgents(true): %+v %v", info, err)
	}
	c, err := net.Dial("unix", r.sock)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(10*time.Second))
	defer cancel()
	r.cs, err = mcp.NewClient(&mcp.Implementation{Name: "тест", Version: "1"}, nil).
		Connect(ctx, &mcp.IOTransport{Reader: c, Writer: c}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.cs.Close() })
}

// call вызывает инструмент; ошибка протокола — провал теста.
func (r *agRig) call(t *testing.T, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(30*time.Second))
	defer cancel()
	res, err := r.cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s %v: ошибка протокола: %v", name, args, err)
	}
	return res
}

// text — первый текст ответа.
func agText(res *mcp.CallToolResult) string {
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// structured — ответ инструмента как объект.
func structured(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(agText(res)), &m); err != nil {
		t.Fatalf("ответ не JSON: %v: %q", err, agText(res))
	}
	return m
}

// agImage — картинка ответа; nil — её нет.
func agImage(res *mcp.CallToolResult) []byte {
	for _, c := range res.Content {
		if ic, ok := c.(*mcp.ImageContent); ok {
			return ic.Data
		}
	}
	return nil
}

// diskState — всё, что лежит в папке dir, кроме сокета (mcp): путь → хеш содержимого и время записи.
func diskState(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if rel == "mcp" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		st, err := d.Info()
		if err != nil {
			return err
		}
		h := sha256.Sum256(b)
		out[rel] = hex.EncodeToString(h[:8]) + "@" + st.ModTime().UTC().Format(time.RFC3339Nano)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// diskDiff — различия двух состояний словами; "" — совпали.
func diskDiff(before, after map[string]string) string {
	var d []string
	for k, v := range before {
		switch w, ok := after[k]; {
		case !ok:
			d = append(d, "исчез "+k)
		case w != v:
			d = append(d, "изменён "+k)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			d = append(d, "появился "+k)
		}
	}
	sort.Strings(d)
	return strings.Join(d, "; ")
}

// agWait ждёт условия cond (без замков App) до 10 с, с поправкой на занятую машину.
func agWait(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(testkit.Limit(10 * time.Second))
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("не дождались: %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}
