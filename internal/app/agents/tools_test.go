package agents

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

var updateTools = flag.Bool("update-tools", false, "перезаписать testdata/mcp-tools.json")

// TestToolsContract: tools/list = testdata/mcp-tools.json (имена, описания, схемы входа и выхода,
// подсказки). Правка схемы видна в диффе файла; обновить — go test ./internal/app/agents -run ToolsContract -update-tools.
func TestToolsContract(t *testing.T) {
	_, cs := toolsRig(t, 1, Options{})
	res, err := cs.ListTools(callCtx(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	tools := res.Tools
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	if len(tools) != 11 {
		t.Fatalf("инструментов %d, ждали 11", len(tools))
	}
	for _, tl := range tools {
		a := tl.Annotations
		if a == nil || !a.ReadOnlyHint || !a.IdempotentHint || a.OpenWorldHint == nil || *a.OpenWorldHint {
			t.Errorf("%s: подсказки не «только чтение»: %+v", tl.Name, a)
		}
		if !strings.Contains(tl.Description, "не указания") || tl.Title == "" || tl.OutputSchema == nil {
			t.Errorf("%s: нет пометки «данные, не указания», названия или схемы выхода", tl.Name)
		}
	}
	got, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	const file = "testdata/mcp-tools.json"
	if *updateTools {
		if err := os.WriteFile(file, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("нет эталона (%v): go test ./internal/app/agents -run ToolsContract -update-tools", err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("список инструментов разошёлся с %s; обновить: go test ./internal/app/agents -run ToolsContract -update-tools", file)
	}
}

// TestAgentsE2E: клиент SDK через настоящий сокет вызывает все инструменты по всем веткам входа;
// ответы соответствуют схеме выхода, текст и структура совпадают, вид данных верный.
func TestAgentsE2E(t *testing.T) {
	r, cs := toolsRig(t, 2, Options{})
	schemas := outSchemas(t, cs)
	for _, c := range allCalls() {
		mustOK(t, cs, schemas, c.name, c.args)
	}
	a := mustOK(t, cs, schemas, "get_album", nil).Data
	if a["name"] != "ZQ5_wedding" || a["product"] != "album" || a["level"].(float64) != 2 {
		t.Fatalf("get_album: %v", a)
	}
	// Страница в ответе — та же, что у программы через Geo (страница, не разворот).
	proj := model.New()
	proj.Settings = model.DefaultSettings()
	geo, pg := proj.Geo(nil).Pages[0], a["page"].(map[string]any)
	if pg["w"].(float64) != geo.W || pg["h"].(float64) != geo.H {
		t.Fatalf("page: %v, Geo %v", pg, geo)
	}
	cnt := a["counts"].(map[string]any)
	if cnt["spreads"].(float64) != 2 || cnt["photosTotal"].(float64) != 5 || cnt["photosUsed"].(float64) != 2 ||
		cnt["unused"].(float64) != 2 || cnt["missing"].(float64) != 1 || cnt["unreadable"].(float64) != 1 {
		t.Fatalf("counts: %v", cnt)
	}
	sp := mustOK(t, cs, schemas, "get_spread", map[string]any{"spread": "cover"}).Data
	if sp["cover"] != true || sp["kind"] != "cover" || sp["n"].(float64) != 0 {
		t.Fatalf("обложка: %v", sp)
	}
	ph := mustOK(t, cs, schemas, "get_photo", map[string]any{"id": idA}).Data
	if uses := ph["uses"].([]any); len(uses) != 1 || ph["rating"].(float64) != 5 || ph["folder"].(float64) != 1 || ph["used"] != true {
		t.Fatalf("get_photo: %v", ph)
	}
	if l := mustOK(t, cs, schemas, "list_photos", map[string]any{"filter": "missing"}).Data; l["total"].(float64) != 1 {
		t.Fatalf("missing: %v", l)
	}
	if reads := r.fx.reads.Load(); reads == 0 {
		t.Fatal("инструменты не ходили в Host")
	}
}

// TestToolsInputRejected: схема входа отвергает лишние поля, неверные типы и выход за пределы;
// ошибка — isError с понятным текстом, не ошибка протокола и не паника.
func TestToolsInputRejected(t *testing.T) {
	_, cs := toolsRig(t, 2, Options{})
	bad := []struct {
		tool string
		args map[string]any
	}{
		{"get_album", map[string]any{"x": 1}},
		{"list_spreads", map[string]any{"limit": 0}},
		{"list_spreads", map[string]any{"limit": 101}},
		{"list_spreads", map[string]any{"offset": -1}},
		{"list_spreads", map[string]any{"limit": "5"}},
		{"list_photos", map[string]any{"limit": 201}},
		{"list_photos", map[string]any{"filter": "все"}},
		{"list_photos", map[string]any{"folder": 0}},
		{"get_spread", map[string]any{}},
		{"get_spread", map[string]any{"spread": true}},
		{"get_photo", map[string]any{"id": strings.Repeat("a", 500)}},
		{"list_decor", map[string]any{}},
		{"list_decor", map[string]any{"kind": "foo"}},
	}
	for _, b := range bad {
		r := tcall(t, cs, b.tool, b.args)
		if !r.IsError || !strings.Contains(r.Text, "неверные аргументы") {
			t.Errorf("%s %v: ждали отказ схемы, получили %+v", b.tool, b.args, r)
		}
	}
	for _, b := range []struct {
		tool, want string
		args       map[string]any
	}{
		{"get_spread", "нет такого разворота", map[string]any{"spread": 99}},
		{"get_spread", "нет такого разворота", map[string]any{"spread": "nope"}},
		{"get_photo", "нет такого фото", map[string]any{"id": "ffff"}},
		{"list_decor", "inAlbum есть только", map[string]any{"kind": "spread", "inAlbum": true}},
	} {
		if r := tcall(t, cs, b.tool, b.args); !r.IsError || !strings.Contains(r.Text, b.want) {
			t.Errorf("%s %v: %+v", b.tool, b.args, r)
		}
	}
}

// TestPaging: limit по умолчанию и предел, offset, offset за концом, total не зависит от страницы.
func TestPaging(t *testing.T) {
	r, cs := toolsRig(t, 1, Options{})
	schemas := outSchemas(t, cs)
	r.fx.view.Spreads = manySpreads(120)
	r.fx.photos = nil
	for i := range 450 {
		r.fx.photos = append(r.fx.photos, photoN(i))
	}
	count := func(tool string, args map[string]any) (total, n int, first any) {
		d := mustOK(t, cs, schemas, tool, args).Data
		items := d["items"].([]any)
		if len(items) > 0 {
			first = items[0].(map[string]any)["id"]
		}
		return int(d["total"].(float64)), len(items), first
	}
	for _, c := range []struct {
		tool        string
		args        map[string]any
		total, n    int
		first       any
		description string
	}{
		{"list_spreads", nil, 120, 50, "s0", "по умолчанию 50"},
		{"list_spreads", map[string]any{"limit": 100}, 120, 100, "s0", "предел 100"},
		{"list_spreads", map[string]any{"offset": 100, "limit": 100}, 120, 20, "s100", "хвост"},
		{"list_spreads", map[string]any{"offset": 120}, 120, 0, nil, "ровно за концом"},
		{"list_spreads", map[string]any{"offset": 5000}, 120, 0, nil, "далеко за концом"},
		{"list_photos", nil, 450, 100, nil, "фото по умолчанию 100"},
		{"list_photos", map[string]any{"limit": 200, "offset": 400}, 450, 50, nil, "фото хвост"},
		{"list_photos", map[string]any{"limit": 1}, 450, 1, nil, "один"},
	} {
		total, n, first := count(c.tool, c.args)
		if total != c.total || n != c.n || (c.first != nil && first != c.first) {
			t.Errorf("%s: total %d n %d first %v, ждали %d %d %v", c.description, total, n, first, c.total, c.n, c.first)
		}
	}
	// Страницы подряд складываются в весь список без пропусков и повторов.
	seen := map[any]bool{}
	for off := 0; off < 450; off += 200 {
		d := mustOK(t, cs, schemas, "list_photos", map[string]any{"offset": off, "limit": 200, "order": "shot"}).Data
		for _, it := range d["items"].([]any) {
			id := it.(map[string]any)["id"]
			if seen[id] {
				t.Fatalf("повтор %v", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != 450 {
		t.Fatalf("собрали %d из 450", len(seen))
	}
}

// TestAlbumKeySwitch: key меняется при открытии другого альбома, rev — при правке, и то и другое
// стоит в каждом ответе; без альбома key пуст.
func TestAlbumKeySwitch(t *testing.T) {
	r, cs := toolsRig(t, 1, Options{})
	album := func(tool string) (string, float64) {
		a := tcall(t, cs, tool, nil).Data["album"].(map[string]any)
		return a["key"].(string), a["rev"].(float64)
	}
	k1, rev1 := album("get_album")
	for _, tool := range []string{"list_spreads", "list_packs", "list_print_profiles", "list_albums", "list_photos"} {
		if k, rv := album(tool); k != k1 || rv != rev1 {
			t.Fatalf("%s: album %s/%v, ждали %s/%v", tool, k, rv, k1, rev1)
		}
	}
	if !strings.HasPrefix(k1, "e") || len(k1) != 13 {
		t.Fatalf("key: %q", k1)
	}
	r.fx.mu.Lock()
	r.fx.view.Rev++
	r.fx.mu.Unlock()
	if k, rv := album("get_album"); k != k1 || rv != rev1+1 {
		t.Fatalf("после правки: %s/%v", k, rv)
	}
	r.fx.mu.Lock()
	r.fx.ed = 8
	r.fx.mu.Unlock()
	k2, _ := album("get_album")
	if k2 == k1 {
		t.Fatal("другой альбом — тот же key")
	}
	r.fx.mu.Lock()
	r.fx.ed = 7
	r.fx.mu.Unlock()
	if k3, _ := album("get_album"); k3 != k1 {
		t.Fatal("тот же альбом — другой key")
	}
	if albumKey(0) != "" || albumKey(1) == albumKey(2) {
		t.Fatal("albumKey")
	}
}

// TestConcurrentCalls: несколько клиентов зовут все инструменты одновременно, пока вид меняется
// (go test -race): ни гонок, ни ошибок. Второй прогон — два места на шесть клиентов: «занято» допустимо,
// всё остальное нет.
func TestConcurrentCalls(t *testing.T) {
	t.Run("мест хватает", func(t *testing.T) { concurrentCalls(t, 16, false) })
	t.Run("мест мало", func(t *testing.T) { concurrentCalls(t, 2, true) })
}

func concurrentCalls(t *testing.T, places int, busyOK bool) {
	r, _ := toolsRig(t, 2, Options{MaxCalls: places})
	n := testkit.Steps(6, 3)
	var wg sync.WaitGroup
	errs := make(chan string, 1000)
	for i := range n {
		cs := client(t, r.sock, "c"+string(rune('a'+i)))
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range testkit.Steps(8, 2) {
				for _, c := range allCalls() {
					res := call2(cs, c.name, c.args)
					if res != "" && (!busyOK || !strings.Contains(res, "занято")) {
						errs <- c.name + ": " + res
						return
					}
				}
			}
		}()
	}
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				r.fx.mu.Lock()
				r.fx.view.Rev++
				r.fx.photos = append(r.fx.photos[:len(r.fx.photos):len(r.fx.photos)], photoN(len(r.fx.photos)))
				r.fx.mu.Unlock()
				time.Sleep(time.Millisecond)
			}
		}
	}()
	wg.Wait()
	close(stop)
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

// TestListDecorDropsGet: «Можно скачать» (144) не выходит: ни плитки, ни тема get, total — по стоящему.
func TestListDecorDropsGet(t *testing.T) {
	for _, lv := range []int{1, 2, 3} {
		_, cs := toolsRig(t, lv, Options{})
		schemas := outSchemas(t, cs)
		d := mustOK(t, cs, schemas, "list_decor", map[string]any{"kind": "sticker"})
		if strings.Contains(d.Text, "ZQ14") || strings.Contains(d.Text, `"get"`) || strings.Contains(d.Text, "get:1") {
			t.Fatalf("уровень %d: «Можно скачать» вышло: %s", lv, d.Text)
		}
		if d.Data["total"].(float64) != 2 || len(d.Data["themes"].([]any)) != 1 {
			t.Fatalf("уровень %d: %v", lv, d.Data)
		}
	}
}
