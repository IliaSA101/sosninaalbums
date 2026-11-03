package app

// TestToolsReadOnly (0.31-143-57): ни один инструмент службы агентов не пишет — ни проект, ни
// config.json и templates.json, ни другие файлы папки данных, ни события окну. Стенд — настоящий App.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// agCall — один вызов инструмента.
type agCall struct {
	name string
	args map[string]any
}

// agReadCalls — все инструменты по всем веткам входа: kind, filter, order, страницы, неверный ID.
func agReadCalls(r *agRig) []agCall {
	m := map[string]any{}
	id := library.IDFor(r.photos[0])
	out := []agCall{{"get_album", m}, {"list_spreads", m}, {"list_spreads", map[string]any{"offset": 1, "limit": 1}},
		{"list_spreads", map[string]any{"offset": 99}},
		{"get_spread", map[string]any{"spread": 1}}, {"get_spread", map[string]any{"spread": "2"}}, {"get_spread", map[string]any{"spread": "cover"}},
		{"get_spread", map[string]any{"spread": 99}}, {"get_spread", map[string]any{"spread": "нет-такого"}},
		{"list_photos", m}, {"get_photo", map[string]any{"id": id}}, {"get_photo", map[string]any{"id": "нет-такого"}},
		{"get_photo", map[string]any{"id": library.IDFor(r.photos[3])}}, {"get_photo", map[string]any{"id": library.IDFor(r.photos[4])}},
		{"list_packs", m}, {"list_print_profiles", m}, {"list_albums", m},
		{"get_photo_preview", map[string]any{"id": id}}, {"get_photo_preview", map[string]any{"id": id, "size": 512}},
		{"get_photo_preview", map[string]any{"id": library.IDFor(r.photos[3])}}, {"get_photo_preview", map[string]any{"id": library.IDFor(r.photos[4])}},
		{"get_photo_preview", map[string]any{"id": "нет-такого"}},
		{"get_spread_preview", map[string]any{"spread": 1}}, {"get_spread_preview", map[string]any{"spread": "cover", "size": 800}},
		{"get_spread_preview", map[string]any{"spread": 2, "size": 1600}}, {"get_spread_preview", map[string]any{"spread": 99}}}
	for _, f := range []string{"all", "used", "unused", "missing", "unreadable", "marked"} {
		out = append(out, agCall{"list_photos", map[string]any{"filter": f}})
	}
	out = append(out, agCall{"list_photos", map[string]any{"order": "name", "folder": 1, "bin": "нет"}})
	for _, k := range []string{"bg", "sticker", "frame", "spread", "look", "template"} {
		out = append(out, agCall{"list_decor", map[string]any{"kind": k}})
	}
	for _, k := range []string{"bg", "sticker", "frame", "look"} {
		out = append(out, agCall{"list_decor", map[string]any{"kind": k, "inAlbum": true}})
	}
	return append(out, agCall{"list_decor", map[string]any{"kind": "sticker", "query": "зв", "theme": "basic"}})
}

func TestToolsReadOnly(t *testing.T) {
	r := newAgRig(t)
	if _, err := os.Stat(filepath.Join(r.cfg, "templates.json")); err != nil {
		t.Fatalf("стенд без templates.json: проверка шаблонов пуста: %v", err)
	}
	r.start(t)
	n := 0
	for level := 1; level <= 3; level++ {
		r.a.SetAgentsLevel(level) // config.json меняет только фасад; службе читать
		n += r.readOnlyPass(t, level)
	}
	// Пустой альбом: те же вызовы, те же гарантии (разворотов нет, фото в ленте остались).
	r.a.NewProject(model.ProductAlbum)
	r.settle()
	n += r.readOnlyPass(t, 3)
	// Журнал вызовов — единственное, что службе вольно менять; он в памяти, на диске его нет.
	if info := r.a.AgentsInfo(); info.Today != n || len(info.Calls) != 50 {
		t.Errorf("журнал: today %d (ждали %d), записей %d", info.Today, n, len(info.Calls))
	}
}

// readOnlyPass вызывает всё на уровне level и сверяет проект, папку данных и события до и после;
// возвращает число вызовов.
func (r *agRig) readOnlyPass(t *testing.T, level int) int {
	t.Helper()
	v := r.a.editor().View()
	// Файлы вне папки данных тоже не трогаются: фото (папка съёмки) и альбом (.salbum).
	dirs := []string{r.cfg, filepath.Dir(r.photos[0]), filepath.Dir(r.album)}
	disks := make([]map[string]string, len(dirs))
	for i, d := range dirs {
		disks[i] = diskState(t, d)
	}
	evs := r.eventNames()
	calls := agReadCalls(r)
	for _, c := range calls {
		res := r.call(t, c.name, c.args)
		if res.IsError && strings.Contains(agText(res), "внутренняя ошибка") {
			t.Errorf("уровень %d, %s %v: %s", level, c.name, c.args, agText(res))
		}
		if level == 1 { // уровень 1: устройство альбома, но ни имён, ни путей
			r.noLabels(t, c, res)
		}
	}
	extra := 0
	if level == 3 && len(v.Spreads) > 0 { // тест не пустой: картинки пришли, недоступное названо словами
		extra = 3
		if agImage(r.call(t, "get_photo_preview", map[string]any{"id": library.IDFor(r.photos[0])})) == nil ||
			agImage(r.call(t, "get_spread_preview", map[string]any{"spread": 1})) == nil {
			t.Error("уровень 3: превью не получено")
		}
		if res := r.call(t, "get_photo_preview", map[string]any{"id": library.IDFor(r.photos[3])}); !res.IsError || agImage(res) != nil {
			t.Errorf("пропавшее фото: %v %q", res.IsError, agText(res))
		}
	}
	w := r.a.editor().View()
	if w.Rev != v.Rev || w.Dirty != v.Dirty || w.CanUndo != v.CanUndo || w.CanRedo != v.CanRedo {
		t.Errorf("уровень %d: проект изменился: rev %d→%d dirty %v→%v", level, v.Rev, w.Rev, v.Dirty, w.Dirty)
	}
	for i, d := range dirs {
		if now := diskState(t, d); diskDiff(disks[i], now) != "" {
			t.Errorf("уровень %d: папка %d из %d (данные, фото, альбом): %s", level, i+1, len(dirs), diskDiff(disks[i], now))
		}
	}
	for _, e := range r.eventNames()[len(evs):] {
		if !strings.HasPrefix(e, "agents:") {
			t.Errorf("уровень %d: лишнее событие %q", level, e)
		}
	}
	return len(calls) + extra
}

// noLabels (0.31-143-57, аудит п. 6): ответ уровня 1 не содержит меток стенда — имён фото, папки
// съёмки, названия альбома, путей и имени своего шаблона. Ищется во всём ответе, включая
// структурированную часть и ошибки.
func (r *agRig) noLabels(t *testing.T, c agCall, res *mcp.CallToolResult) {
	t.Helper()
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	// «Свадьба» не метка: так называется встроенная тема каталога оформления; альбом стенда
	// назван иначе (albumName) и ищется голым именем: проекция «название альбома» тоже утечка.
	labels := []string{"Иванов", "Петрова", "Сидоров", "Пропало", "Битое", "Съёмка", "Козлов", albumName,
		".jpg", ".jpeg", ".salbum", filepath.Dir(r.photos[0]), r.album, filepath.Dir(r.album), r.cfg, prefs.ConfigPath(r.cfg)}
	for _, l := range labels {
		if strings.Contains(got, l) { // json.Marshal пишет кириллицу как есть, слэши не экранирует
			t.Errorf("уровень 1, %s %v: в ответе метка %q: %.200s", c.name, c.args, l, got)
		}
	}
}
