package app

// TestAgentsLogClean (0.31-143-61): что бы ни случилось с фото и разверткой, в log.txt от службы
// агентов нет путей, имён фото, названия альбома и расширений файлов: log.txt уходит в отчёты.

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/library"
)

func TestAgentsLogClean(t *testing.T) {
	buf := captureLog(t)
	r := newAgRig(t)
	r.start(t)
	r.a.SetAgentsLevel(3)
	ctx := context.Background()
	h := r.a.agentsHost()

	// Фото удалено, усечено, чужой ID — по всем инструментам и через Host напрямую.
	for _, c := range agReadCalls(r) {
		r.call(t, c.name, c.args)
	}
	for _, id := range []string{library.IDFor(r.photos[3]), library.IDFor(r.photos[4]), "чужой-ID", ""} {
		if _, err := h.Preview(ctx, id, 512); err == nil {
			t.Errorf("превью %q: ждали ошибку", id)
		}
	}

	// Ошибка render: копия в кэше превью — мусор. Ошибки декода называют файл кэша и фото.
	sp := r.a.editor().View().Spreads[0]
	if len(sp.Photos) == 0 {
		t.Fatal("на первом развороте нет фото")
	}
	for _, ph := range sp.Photos {
		p, ok, _ := r.a.mediaPhoto(ph)
		if !ok {
			continue // пропавшее фото не рисуется
		}
		f, err := r.a.cache.FileCtx(ctx, p, true)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("\xff\xd8\xff\xe0junk"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := h.Render(ctx, sp.ID, 800); !errors.Is(err, errAgentsRender) {
		t.Fatalf("Render с мусором в кэше: %v, ждали errAgentsRender", err)
	}
	if res := r.call(t, "get_spread_preview", map[string]any{"spread": 1}); !res.IsError || agImage(res) != nil {
		t.Errorf("get_spread_preview с мусором в кэше: %v %q", res.IsError, agText(res))
	}

	r.a.stopAgents()
	log := buf.String()
	// Один вызов инструмента — одна запись (защита вызова); прямой Host.Render сам не пишет.
	if n := strings.Count(log, "agents: get_spread_preview: разворот не нарисован"); n != 1 {
		t.Errorf("записей об ошибке render %d, ждали 1:\n%s", n, log)
	}
	if n := strings.Count(log, "разворот не нарисован"); n != 1 {
		t.Errorf("строк «разворот не нарисован» %d, ждали 1:\n%s", n, log)
	}
	banned := []string{"Иванов", "Петрова", "Сидоров", "Пропало", "Битое", "Свадьба", "Съёмка", ".jpg", ".jpeg", ".salbum",
		"_p.jpg", r.cfg, os.TempDir(), r.album, "junk"}
	for _, b := range banned {
		if strings.Contains(log, b) {
			t.Errorf("в log.txt есть %q:\n%s", b, log)
		}
	}
}
