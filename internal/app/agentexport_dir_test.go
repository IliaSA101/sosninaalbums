package app

// Папка выгрузки по просьбе агента (152 Т3б): предложение без вложенности, имя черновика, путь в папке фото,
// негодная подсказка. Ход выгрузки — agentexport_flow_test.go.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// decline закрывает вопрос ответом «Отмена».
func (r *exportRig) decline(t *testing.T, c *askCall, ev AgentAsk) {
	t.Helper()
	if _, err := r.a.AnswerAgent(ev.ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
}

// run — вопрос и «Выгрузить» до конца записи; возвращает событие вопроса.
func (r *exportRig) run(t *testing.T, req agents.AskReq) AgentAsk {
	t.Helper()
	c, ev := r.question(t, req)
	if st, err := r.a.AgentAnswerExport(ev.ID, false); err != nil || !st.Started {
		t.Fatalf("AgentAnswerExport: %+v, %v", st, err)
	}
	c.wait(t)
	r.finish(t)
	return ev
}

// peek — вопрос без щелчка: путь и строки, затем «Отмена».
func (r *exportRig) peek(t *testing.T, req agents.AskReq) AgentAsk {
	t.Helper()
	c, ev := r.question(t, req)
	r.decline(t, c, ev)
	return ev
}

// TestAgentExportSuggestNoNesting: следующий клиент не ложится внутрь прошлой выгрузки (У84): общая папка,
// папка на клиента (по имени альбома и с другим именем), тот же альбом повторно, Фабрика.
func TestAgentExportSuggestNoNesting(t *testing.T) {
	r := newExportRig(t, 2)
	base := filepath.Dir(r.root)
	// Общая папка: имя альбома внутри.
	if ev := r.peek(t, r.req("Петровы")); ev.Path != filepath.Join(r.root, "Петровы") || !ev.Create {
		t.Fatalf("общая папка: %+v", ev)
	}
	r.run(t, r.req("Петровы"))
	if got := prefs.ExportRoot(r.a.loadConfig()); got != r.root {
		t.Fatalf("корень после выгрузки %q", got)
	}
	// Следующий клиент — соседняя папка, не вложенная; тот же альбом — та же папка и «заменится».
	if ev := r.peek(t, r.req("Ивановы")); ev.Path != filepath.Join(r.root, "Ивановы") {
		t.Errorf("следующий клиент: %q", ev.Path)
	}
	if ev := r.peek(t, r.req("Петровы")); ev.Path != filepath.Join(r.root, "Петровы") || ev.Create || !strings.Contains(strings.Join(ev.Lines, "\n"), "заменится") {
		t.Errorf("тот же альбом: %+v", ev)
	}
	// Папка на клиента по имени альбома (корень = прежняя выгрузка): родитель, а не внутрь.
	r.a.rememberDir(prefs.DirExportRoot, filepath.Join(r.root, "Петровы"))
	if ev := r.peek(t, r.req("Ивановы")); ev.Path != filepath.Join(r.root, "Ивановы") {
		t.Errorf("папка на клиента (имя альбома): %q", ev.Path)
	}
	if ev := r.peek(t, r.req("Петровы")); ev.Path != filepath.Join(r.root, "Петровы") || !strings.Contains(strings.Join(ev.Lines, "\n"), "заменится") {
		t.Errorf("тот же альбом из папки на клиента: %+v", ev)
	}
	// Папка на клиента с другим именем.
	client := filepath.Join(base, "Клиент")
	if err := os.Rename(filepath.Join(r.root, "Петровы"), client); err != nil {
		t.Fatal(err)
	}
	r.a.rememberDir(prefs.DirExportRoot, client)
	if ev := r.peek(t, r.req("Петровы")); ev.Path != filepath.Join(base, "Петровы") || !ev.Create {
		t.Errorf("папка на клиента (другое имя): %q, создать %v", ev.Path, ev.Create)
	}
}

// TestAgentExportSuggestNoNestingPartial: корень — папка прошлого клиента со своими 01.jpg, 02.jpg; выгрузка
// другого клиента только части (диапазон, только обложка) всё равно берёт родителя, а не кладёт внутрь.
func TestAgentExportSuggestNoNestingPartial(t *testing.T) {
	r := newExportRigWith(t, 4, func(p *model.Project) {
		p.Settings.PageW, p.Settings.PageH, p.Settings.Safe = 200, 203, 10
		p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID, BgR: "#203040"}}
	})
	first := r.req("Петровы")
	first.Range = "1-2"
	r.run(t, first)
	client := filepath.Join(r.root, "Петровы")
	if n := len(jpgs(t, client)); n != 2 {
		t.Fatalf("прошлая выгрузка: %d файлов", n)
	}
	r.a.rememberDir(prefs.DirExportRoot, client)
	want := filepath.Join(r.root, "Ивановы")
	for name, tune := range map[string]func(*agents.AskReq){
		"диапазон":       func(q *agents.AskReq) { q.Range = "3" },
		"один из ряда":   func(q *agents.AskReq) { q.Range = "1" },
		"только обложка": func(q *agents.AskReq) { q.CoverOnly = true },
	} {
		q := r.req("Ивановы")
		tune(&q)
		if ev := r.peek(t, q); ev.Path != want || !ev.Create {
			t.Errorf("%s: %+v, ждали %q", name, ev, want)
		}
	}
}

// TestAgentExportSuggestFabrika: Фабрика — папка книги внутри корня, следующий клиент не внутри прошлой книги.
func TestAgentExportSuggestFabrika(t *testing.T) {
	r := newExportRigWith(t, 1, func(p *model.Project) {
		p.Settings.PageW, p.Settings.PageH, p.Settings.Safe = 200, 203, 10
		pr, _ := model.CoverPresetFor("fabrika-photo", p.Settings)
		p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID, BgR: "#203040"}, Spec: pr.Spec}
	})
	req := func(name string) agents.AskReq {
		q := r.req(name)
		q.Print, q.DPI = "fabrika", 0
		return q
	}
	book := filepath.Join(r.root, "Петровы")
	if ev := r.peek(t, req("Петровы")); ev.Path != book || !ev.Create {
		t.Fatalf("книга в корне: %+v", ev)
	}
	r.run(t, req("Петровы"))
	if n := len(jpgsIn(t, book, "1-*.jpg")); n != 2 {
		t.Fatalf("в папке книги %d файлов", n)
	}
	if ev := r.peek(t, req("Ивановы")); ev.Path != filepath.Join(r.root, "Ивановы") {
		t.Errorf("следующая книга: %q", ev.Path)
	}
	r.a.rememberDir(prefs.DirExportRoot, book) // корень — прежняя книга: берётся её родитель
	if ev := r.peek(t, req("Ивановы")); ev.Path != filepath.Join(r.root, "Ивановы") {
		t.Errorf("корень — прежняя книга: %q", ev.Path)
	}
}

// jpgsIn — файлы папки по маске.
func jpgsIn(t *testing.T, dir, pattern string) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestAgentExportDraftName: черновик (файла нет) с подсказкой-именем — корень выгрузок и это имя; без подсказки
// пути нет — «Выбрать папку…» (имени «Альбом» программа не придумывает); негодное имя (CON, точка на краю, знаки,
// управляющие) — подсказка отвергнута, строка «Агент предложил папку, которую программа не берёт без вас».
func TestAgentExportDraftName(t *testing.T) {
	r := newExportRig(t, 2)
	r.ed = mustEditor(t, projectOf(t, r.ed), "") // черновик
	r.a.ed.Store(r.ed)
	if ev := r.peek(t, r.req("Петровы")); ev.Path != filepath.Join(r.root, "Петровы") || !ev.Create {
		t.Errorf("черновик + имя: %+v", ev)
	}
	ev := r.peek(t, r.req(""))
	if ev.Path != "" || ev.OK != askOKExportPick || ev.Alt != "" || ev.Why != "" {
		t.Errorf("черновик без подсказки: %+v", ev)
	}
	for _, bad := range []string{"CON", "Петровы.", "a<b", "a\u202eb", "..\\x", "a\x00b"} {
		ev := r.peek(t, r.req(bad))
		if ev.Path != "" || ev.OK != askOKExportPick || ev.Why != whyBad {
			t.Errorf("негодная подсказка %q: %+v", bad, ev)
		}
	}
	// Файл альбома есть, подсказки нет — имя файла.
	r.ed = mustEditor(t, projectOf(t, r.ed), r.file)
	r.a.ed.Store(r.ed)
	if ev := r.peek(t, r.req("")); ev.Path != filepath.Join(r.root, "Петровы") {
		t.Errorf("имя по файлу: %+v", ev)
	}
}

// projectOf — снимок проекта редактора для нового Editor.
func projectOf(t *testing.T, ed interface {
	Snapshot() (*model.Project, model.View)
}) *model.Project {
	t.Helper()
	p, _ := ed.Snapshot()
	return p
}

// TestAgentExportInPhotoFolder: подсказка-путь внутри папки фото альбома заменяется соседней (121) прямо в вопросе:
// строка «Рядом с папкой фото…», одна кнопка, выгрузка идёт в соседнюю, в папку фото не пишется ни файла.
func TestAgentExportInPhotoFolder(t *testing.T) {
	r := newExportRig(t, 2)
	if _, _, err := r.ed.AddFolder(r.src); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(r.src, "Выгрузка")
	want := filepath.Join(filepath.Dir(r.src), filepath.Base(r.src)+" — выгрузка")
	c, ev := r.question(t, r.req(inside))
	if ev.Path == inside || strings.HasPrefix(ev.Path, r.src+string(filepath.Separator)) || filepath.Dir(ev.Path) != filepath.Dir(r.src) ||
		!ev.Create || ev.OK != askOKExport || !strings.Contains(strings.Join(ev.Lines, "\n"), "Рядом с папкой фото альбома") {
		t.Fatalf("вопрос: %+v, ждали соседнюю вроде %q", ev, want)
	}
	if _, err := r.a.AgentAnswerExport(ev.ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	r.finish(t)
	if len(jpgs(t, ev.Path)) != ev.Files || len(r.warns) != 0 {
		t.Errorf("в соседней %v, вопросов %q", jpgs(t, ev.Path), r.warns)
	}
	if ents, _ := os.ReadDir(r.src); len(ents) != 2 {
		t.Errorf("в папке фото появились файлы: %d записей", len(ents))
	}
}
