package app

// Ход выгрузки по просьбе агента от вопроса до JPEG на диске (152 Т3б): полный круг, повтор в ту же папку,
// подмена альбома и правка между вопросом и щелчком, отказ чужим файлам, окна выгрузки и тихие действия.

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/app/exporter"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// TestAgentExportFullFlow: вопрос с полным путём → «Выгрузить» → JPEG на диске → get_album.export = done;
// повтор того же альбома в ту же папку — «заменится» в вопросе и ни одного вопроса «Да / Нет» после щелчка.
func TestAgentExportFullFlow(t *testing.T) {
	r := newExportRig(t, 3)
	want := filepath.Join(r.root, "Петровы")
	c, ev := r.question(t, r.req("Петровы"))
	if ev.Kind != "export" || ev.Path != want || !ev.Create || ev.Tool != "" || ev.OK != askOKExport || ev.Alt != askAlt || ev.Files < 3 {
		t.Fatalf("вопрос: %+v", ev)
	}
	if _, err := os.Stat(want); err == nil {
		t.Fatal("папка создана до щелчка")
	}
	st, err := r.a.AgentAnswerExport(ev.ID, false)
	if err != nil || !st.Started || st.Files != ev.Files || st.Print != nil {
		t.Fatalf("AgentAnswerExport: %+v, %v", st, err)
	}
	c.wait(t)
	if c.err != nil || c.res.Why != agents.WhyAnswered || c.res.Files != ev.Files || c.res.Album.Key != r.a.askKey(r.ed) {
		t.Fatalf("итог агенту: %+v, %v", c.res, c.err)
	}
	if got := r.finish(t); got.State != exporter.StateDone || got.Done != got.Total || got.Total != ev.Files || got.Error != "" {
		t.Fatalf("get_album.export: %+v", got)
	}
	files := jpgs(t, want)
	if len(files) != ev.Files {
		t.Fatalf("файлов на диске %d, в вопросе %d: %v", len(files), ev.Files, files)
	}
	if len(r.warns) != 0 || len(r.osWin) != 0 {
		t.Errorf("лишние окна: вопросы %q, окна ОС %q", r.warns, r.osWin)
	}
	if root := prefs.ExportRoot(r.a.loadConfig()); root != r.root {
		t.Errorf("корень выгрузок %q, ждали %q (создана папка — корень её родитель)", root, r.root)
	}

	// Повтор: те же файлы заменятся, человек видел это в вопросе — второго вопроса нет.
	c2, ev2 := r.question(t, r.req("Петровы"))
	if ev2.Path != want || ev2.Create || !strings.Contains(strings.Join(ev2.Lines, "\n"), "уже есть выгрузка") ||
		!strings.Contains(strings.Join(ev2.Lines, "\n"), "заменится") {
		t.Fatalf("повторный вопрос: %+v", ev2)
	}
	if _, err := r.a.AgentAnswerExport(ev2.ID, false); err != nil {
		t.Fatal(err)
	}
	c2.wait(t)
	if c2.err != nil || c2.res.Why != agents.WhyAnswered {
		t.Fatalf("итог повтора: %+v, %v", c2.res, c2.err)
	}
	if got := r.finish(t); got.State != exporter.StateDone {
		t.Fatalf("повтор: %+v", got)
	}
	if len(r.warns) != 0 {
		t.Errorf("ReplaceQuestion после показанной строки «заменится»: %q", r.warns)
	}
	if again := jpgs(t, want); !slices.Equal(again, files) {
		t.Errorf("после повтора %v, было %v", again, files)
	}
}

// TestAgentExportReplaceChanged: между вопросом и щелчком в папке появился ещё один свой файл — набор не тот,
// что видел человек, и ReplaceQuestion спрашивает как в меню.
func TestAgentExportReplaceChanged(t *testing.T) {
	r := newExportRig(t, 3)
	c, ev := r.question(t, r.req("Петровы"))
	if _, err := r.a.AgentAnswerExport(ev.ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	r.finish(t)
	c2, ev2 := r.question(t, r.req("Петровы"))
	if !strings.Contains(strings.Join(ev2.Lines, "\n"), "заменится") {
		t.Fatalf("вопрос: %+v", ev2)
	}
	extra := filepath.Join(r.root, "Петровы", "02.jpg") // был в наборе; удалим — набор «своих» станет меньше
	if err := os.Remove(extra); err != nil {
		t.Fatal(err)
	}
	if _, err := r.a.AgentAnswerExport(ev2.ID, false); err != nil {
		t.Fatal(err)
	}
	c2.wait(t)
	r.finish(t)
	if len(r.warns) != 1 || !strings.Contains(r.warns[0], "Заменить?") {
		t.Errorf("набор файлов сменился, а ReplaceQuestion нет или не тот: %q", r.warns)
	}
}

// TestAgentExportAlbumSwitched: между вопросом и щелчком тихий create_album заменил альбом — выгрузки нет, агенту
// «альбом изменился», папка не создана, вопрос закрыт.
func TestAgentExportAlbumSwitched(t *testing.T) {
	r := newExportRig(t, 2)
	c, ev := r.question(t, r.req("Петровы"))
	r.a.startNew(blankEditor("album")) // тихое создание: текущий альбом — другой
	st, err := r.a.AgentAnswerExport(ev.ID, false)
	c.wait(t)
	if err != nil || st.Started || !errors.Is(c.err, agents.ErrAlbumChanged) {
		t.Fatalf("подмена альбома: %+v, %v, агенту %v", st, err, c.err)
	}
	if _, err := os.Stat(filepath.Join(r.root, "Петровы")); err == nil {
		t.Error("папка выгрузки создана, хотя выгрузка отклонена")
	}
	if r.a.exp.Last().State != exporter.StateIdle {
		t.Errorf("выгрузка началась: %+v", r.a.exp.Last())
	}
	if r.state() != askAny {
		t.Error("вопрос остался открытым")
	}
}

// TestAgentExportAlbumEdited: правка агента между вопросом и щелчком поднимает ревизию — выгрузки нет, агенту
// «альбом изменён»; следующий вопрос со свежей ревизией выгружает.
func TestAgentExportAlbumEdited(t *testing.T) {
	r := newExportRig(t, 2)
	c, ev := r.question(t, r.req("Петровы"))
	if _, _, err := r.ed.AddFolder(r.src); err != nil { // правка между вопросом и щелчком
		t.Fatal(err)
	}
	st, err := r.a.AgentAnswerExport(ev.ID, false)
	c.wait(t)
	if err != nil || st.Started || !errors.Is(c.err, agents.ErrAlbumEdited) {
		t.Fatalf("правка альбома: %+v, %v, агенту %v", st, err, c.err)
	}
	if _, err := os.Stat(filepath.Join(r.root, "Петровы")); err == nil {
		t.Error("папка выгрузки создана, хотя выгрузка отклонена")
	}
	if r.a.exp.Last().State != exporter.StateIdle {
		t.Errorf("выгрузка началась: %+v", r.a.exp.Last())
	}
	c, ev = r.question(t, r.req("Петровы"))
	if _, err := r.a.AgentAnswerExport(ev.ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.err != nil || r.finish(t).State != exporter.StateDone {
		t.Errorf("после перечитывания: %v, %+v", c.err, r.a.exp.Last())
	}
}

// TestAgentExportForeignRefused: в папке чужие файлы с номерами разворотов — вопрос без «Выгрузить» («Выбрать
// папку…»), «Сюда выгрузить нельзя»; щелчок открывает окно ОС, чужой файл не тронут.
func TestAgentExportForeignRefused(t *testing.T) {
	r := newExportRig(t, 2)
	dir := filepath.Join(r.root, "Петровы")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(dir, "01.jpg")
	if err := os.WriteFile(foreign, []byte("чужое фото"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, ev := r.question(t, r.req("Петровы"))
	if ev.OK != askOKExportPick || ev.Alt != "" || !strings.Contains(strings.Join(ev.Lines, "\n"), "Сюда выгрузить нельзя") {
		t.Fatalf("вопрос: %+v", ev)
	}
	if ev.Path != dir {
		t.Errorf("путь в вопросе %q: человек должен видеть, о какой папке речь", ev.Path)
	}
	// Окно ОС: человек отменил — «остановлено», чужой файл цел.
	st, err := r.a.AgentAnswerExport(ev.ID, false)
	c.wait(t)
	if err != nil || st.Started || len(r.osWin) != 1 || c.res.Why != agents.WhyStopped {
		t.Fatalf("окно ОС: %+v, %v, окон %d, итог %+v", st, err, len(r.osWin), c.res)
	}
	if b, err := os.ReadFile(foreign); err != nil || string(b) != "чужое фото" {
		t.Errorf("чужой файл тронут: %q %v", b, err)
	}
	if r.a.exp.Last().State != exporter.StateIdle {
		t.Errorf("выгрузка началась: %+v", r.a.exp.Last())
	}
}

// TestAgentExportWindowCancel: «Выбрать другую…» → отмена окна ОС — «человек остановил», не отказ.
func TestAgentExportWindowCancel(t *testing.T) {
	r := newExportRig(t, 2)
	c, ev := r.question(t, r.req("Петровы"))
	st, err := r.a.AgentAnswerExport(ev.ID, true)
	c.wait(t)
	if err != nil || st.Started || c.err != nil || c.res.Why != agents.WhyStopped || len(r.osWin) != 1 ||
		!strings.Contains(r.osWin[0], "Claude Code") {
		t.Fatalf("отмена окна: %+v, %v, %+v, %v, окна %q", st, err, c.res, c.err, r.osWin)
	}
}

// TestAgentExportOtherFolder: «Выбрать другую…» → папка из окна ОС: выгрузка туда (а не в показанную).
func TestAgentExportOtherFolder(t *testing.T) {
	r := newExportRig(t, 2)
	r.osDir = filepath.Join(filepath.Dir(r.root), "Другая")
	c, ev := r.question(t, r.req("Петровы"))
	if st, err := r.a.AgentAnswerExport(ev.ID, true); err != nil || !st.Started {
		t.Fatalf("AgentAnswerExport: %+v, %v", st, err)
	}
	c.wait(t)
	r.finish(t)
	if len(jpgs(t, r.osDir)) != ev.Files {
		t.Errorf("в выбранной папке %v, ждали %d файлов", jpgs(t, r.osDir), ev.Files)
	}
	if _, err := os.Stat(filepath.Join(r.root, "Петровы")); err == nil {
		t.Error("показанная папка создана при «Выбрать другую…»")
	}
}

// TestAgentExportWarningNo: «нет» в предупреждении выгрузки — «человек остановил»: не отказ, файлов нет.
func TestAgentExportWarningNo(t *testing.T) {
	r := newExportRig(t, 2)
	r.yes = false
	r.addBlank(t) // пустой разворот: предупреждение «выгружать?»
	c, ev := r.question(t, r.req("Петровы"))
	st, err := r.a.AgentAnswerExport(ev.ID, false)
	c.wait(t)
	if err != nil || st.Started || c.err != nil || c.res.Why != agents.WhyStopped || len(r.warns) != 1 {
		t.Fatalf("«нет» в предупреждении: %+v, %v, %+v, %v, вопросы %q", st, err, c.res, c.err, r.warns)
	}
	if _, err := os.Stat(filepath.Join(r.root, "Петровы")); err == nil {
		t.Error("папка создана после «нет»")
	}
}

// TestAgentQuietWhileExportDialogsOpen: пока окна выгрузки на экране, ни тихое создание, ни открытие альбома не
// проходят: картина под окном предупреждения не меняется. Настоящая выгрузка (флаг службы, окно «Да / Нет»).
func TestAgentQuietWhileExportDialogsOpen(t *testing.T) {
	r := newExportRig(t, 2)
	inDialog := make(chan struct{})
	release := make(chan struct{})
	askYes = func(*App, string) (bool, error) { close(inDialog); <-release; return false, nil }
	r.addBlank(t)
	c, ev := r.question(t, r.req("Петровы"))
	answered := make(chan struct{})
	go func() { defer close(answered); _, _ = r.a.AgentAnswerExport(ev.ID, false) }()
	<-inDialog
	// Окно открыто, фона нет: создание — «идёт выгрузка», открытие — «занято»; альбом не меняется.
	cc := r.ask(r.createReq())
	cc.wait(t)
	if !errors.Is(cc.err, agents.ErrExporting) {
		t.Errorf("вопрос создания при окне выгрузки: %v", cc.err)
	}
	if _, _, err := r.a.quietApply(agents.AskReq{Kind: agents.AskCreate}, r.ed, folderPlan{path: r.other}); !errors.Is(err, agents.ErrExporting) {
		t.Errorf("тихое создание при окне выгрузки: %v", err)
	}
	co := r.ask(agents.AskReq{Kind: agents.AskOpen, Client: "Claude Code"})
	co.wait(t)
	if !errors.Is(co.err, agents.ErrExporting) {
		t.Errorf("открытие при окне выгрузки: %v", co.err)
	}
	if r.a.editor() != r.ed {
		t.Error("альбом сменился под окном выгрузки")
	}
	close(release)
	<-answered
	c.wait(t)
	// Окно закрыто: создание снова доступно.
	cc = r.ask(r.createReq())
	r.asked(t, 2)
	cc.cancel()
	cc.wait(t)
}

// TestAgentExportCoverOnlyNoCover: «только обложка» у альбома без обложки — отказ агенту «нет обложки» без
// вопроса и окна (окно человека молча выгрузило бы развороты); с обложкой — вопрос как обычно, а выгрузка
// разворотов без обложки не затронута.
func TestAgentExportCoverOnlyNoCover(t *testing.T) {
	r := newExportRig(t, 2)
	req := r.req("Петровы")
	req.CoverOnly = true
	c := r.ask(req)
	c.wait(t)
	if !errors.Is(c.err, agents.ErrNoCover) || len(r.evOf("agents:ask")) != 0 || len(r.warns) != 0 {
		t.Fatalf("без обложки: %v, окон %d", c.err, len(r.evOf("agents:ask")))
	}
	if _, ev := r.question(t, r.req("Петровы")); ev.Kind != "export" { // развороты без обложки — вопрос
		t.Fatalf("развороты: %+v", ev)
	}
	r2 := newExportRig(t, 2)
	if _, err := r2.a.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	req = r2.req("Петровы")
	req.CoverOnly = true
	if _, ev := r2.question(t, req); ev.Kind != "export" || ev.Files != 1 {
		t.Fatalf("с обложкой: %+v", ev)
	}
}
