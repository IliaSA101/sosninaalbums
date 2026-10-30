package app

// Выгрузка по просьбе агента (152 Т3): правило места записи, вопрос без ссылки «Не спрашивать», отказы проверок без
// пути, тихие create/open при окнах выгрузки. Служебная часть (схема, итоги) — agents/export_tools_test.go.

import (
	"errors"
	"io/fs"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/app/exporter"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// writeOpts — правила записи для таблицы: Windows или Unix, домашняя, три целые папки, наши, системные деревья.
func writeOpts(windows bool) agentplaces.Opts {
	o := agentplaces.Opts{Windows: windows, Fold: paths.Fold, Timeout: time.Second, Disk: diskOK}
	if windows {
		o.Fold = strings.ToLower // регистр Windows не важен, а тест идёт и на Linux
		o.System = "C:"
		o.Homes = []string{`C:\Users\u`}
		o.Whole = []string{`C:\Users\u\Desktop`, `C:\Users\u\Documents`, `C:\Users\u\Downloads`}
		o.Ours = []string{`C:\Users\u\AppData\Roaming\SosninaAlbums`, `C:\Program Files\SosninaAlbums`}
		o.Lists = "windows"
		return o
	}
	o.Homes = []string{"/home/u"}
	o.Whole = []string{"/home/u/Desktop", "/home/u/Documents", "/home/u/Downloads"}
	o.Ours = []string{"/home/u/.config/sosninaalbums"}
	o.Lists = "linux"
	return o
}

// diskWriteRules — правила записи на настоящем диске: таблица своей ОС (путь Unix на Windows не локален, а
// временная папка Windows лежит под AppData, которую продукт не берёт, — поэтому списки writeRules, а не машины).
func diskWriteRules() *agentplaces.Rules {
	o := writeOpts(runtime.GOOS == "windows")
	o.Disk = nil
	return agentplaces.New(o)
}

// TestExportCheckErrorsNoPath: отказы проверок и выгрузки агенту — без пути и без имён файлов (U67).
func TestExportCheckErrorsNoPath(t *testing.T) {
	const dir = "/home/user/Клиенты/Петровы"
	for _, err := range []error{
		&fs.PathError{Op: "open", Path: dir, Err: fs.ErrPermission},
		errors.New("не найдены файлы фото: 2 (IMG_0001.jpg, IMG_0002.jpg); найдите их: Файл ▸ Найти пропавшие фото… — и повторите выгрузку"),
		errors.New("В папке есть файлы с номерами разворотов (или 00.jpg) не из Sosnina Albums (3 шт., например 01.jpg). Выберите другую папку"),
		errors.New(export.PlaceRefusal("/home/user/Клиенты/Петровы", "/home/user/Клиенты")), // отказ 121 с именами папок
		errors.New("не удалось прочитать папку экспорта: open " + dir + ": denied"),
		errors.New("не удалось создать папку: mkdir " + dir + ": exists"),
		errors.New(strings.Repeat("длинный текст ", 60)),
	} {
		got := exportCheckErr(err)
		var text string
		if got != nil {
			text = got.Error()
		}
		if text == "" || strings.Contains(text, dir) || strings.Contains(text, "IMG_") || strings.Contains(text, "01.jpg") || strings.Contains(text, "Петровы") || strings.Contains(text, "Клиенты") || len([]rune(text)) > 300 {
			t.Errorf("%v -> %q", err, text)
		}
	}
	if !errors.Is(exportCheckErr(exporter.ErrAlbumEdited), agents.ErrAlbumEdited) {
		t.Error("ErrAlbumEdited не переведена")
	}
	if !errors.Is(exportCheckErr(errors.New("экспорт уже идёт")), agents.ErrExporting) {
		t.Error("«экспорт уже идёт» не переведена")
	}
}

// TestExportQuestionLines: заголовок и строки вопроса — путь, создание, соседняя папка, облако, сеть, «заменится»,
// чужие файлы (тогда «Выбрать папку…» и без «Выбрать другую…»); имя папки с плохими знаками в заголовок не идёт.
func TestExportQuestionLines(t *testing.T) {
	sum := []string{"Развороты 1–24 и обложка · 300 dpi · качество 95 · 25 файлов", "Обычная выгрузка"}
	in := func(p exportPlan) askBody {
		return exportAskText(exportAskIn{Client: "Claude Code", Album: "Петровы", Lines: sum, Plan: p})
	}
	b := in(exportPlan{shown: "/v/Петровы", create: true, nextTo: true, net: true, cloud: true, ours: []string{"01.jpg", "02.jpg"}})
	all := strings.Join(b.Lines, "\n")
	for _, want := range []string{"Клиент «Claude Code» просит выгрузить альбом «Петровы» в JPEG:", sum[0], "Папка будет создана.",
		"Рядом с папкой фото альбома", "Сетевой диск.", "синхронизируется с облаком", "уже есть выгрузка (2 файла) — заменится", askWho} {
		if !strings.Contains(all, want) {
			t.Errorf("нет строки %q в %q", want, all)
		}
	}
	if b.Title != "Выгрузить альбом в папку «Петровы»?" || b.OK != askOKExport {
		t.Errorf("заголовок %q, кнопка %q", b.Title, b.OK)
	}
	f := in(exportPlan{shown: "/v/Петровы", foreign: true})
	if f.OK != askOKExportPick || !strings.Contains(strings.Join(f.Lines, "\n"), "Сюда выгрузить нельзя") {
		t.Errorf("чужие файлы: %+v", f)
	}
	n := in(exportPlan{})
	if n.Title != "Выгрузить альбом по просьбе агента?" || n.OK != askOKExportPick || !strings.Contains(strings.Join(n.Lines, "\n"), "Папку выберете вы") {
		t.Errorf("без пути: %+v", n)
	}
	if bad := in(exportPlan{shown: "/v/a\u202eb"}); strings.Contains(bad.Title, "a") {
		t.Errorf("плохое имя в заголовке: %q", bad.Title)
	}
	for n, w := range map[int]string{1: "файл", 2: "файла", 5: "файлов", 11: "файлов", 21: "файл", 112: "файлов"} {
		if filesWord(n) != w {
			t.Errorf("filesWord(%d) = %q", n, filesWord(n))
		}
	}
}

// TestAgentQuietWhileExportRunning: пока выгрузка идёт (окна открыты или уже фон), тихое и вопросное создание и
// открытие альбома — «идёт выгрузка» сразу, без вопроса человеку (0.35-152-27).
func TestAgentQuietWhileExportRunning(t *testing.T) {
	r := newAskRig(t)
	oldRun := exportRunning
	t.Cleanup(func() { exportRunning = oldRun })
	exportRunning = func(*App) bool { return true } // флаг службы поднят: окна или фон
	o := r.ask(agents.AskReq{Kind: agents.AskOpen, Client: "Claude Code"})
	o.wait(t)
	if !errors.Is(o.err, agents.ErrExporting) || len(r.evOf("agents:ask")) != 0 {
		t.Errorf("вопрос открытия при окнах выгрузки: %v, окон %d", o.err, len(r.evOf("agents:ask")))
	}
	c := r.ask(agents.AskReq{Kind: agents.AskCreate, Client: "Claude Code"})
	c.wait(t)
	if !errors.Is(c.err, agents.ErrExporting) {
		t.Errorf("вопрос создания при окнах выгрузки: %v", c.err)
	}
	if _, _, err := r.a.quietApply(agents.AskReq{Kind: agents.AskCreate}, r.a.editor(), folderPlan{path: r.other}); !errors.Is(err, agents.ErrExporting) {
		t.Errorf("тихое создание при окнах выгрузки: %v", err)
	}
	exportRunning = func(*App) bool { return false }
	c = r.ask(agents.AskReq{Kind: agents.AskCreate, Client: "Claude Code"})
	ev := r.asked(t, 1) // выгрузки нет — вопрос создания задаётся
	if _, err := r.a.AnswerAgent(ev.ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
}

// TestExportAskNoNeverLink: у вопроса выгрузки Tool пуст — интерфейс ссылку «Не спрашивать» не рисует (0.35-152-21);
// у вида export нет и «Всегда»: askTool даёт export_album, строка разрешений — «Спрашивать · Запретить».
func TestExportAskNoNeverLink(t *testing.T) {
	// Полный ход: askExportAlbum с альбомом из разворотов — событие без Tool, с Files и «Выбрать другую…».
	full := newExportRig(t, 3)
	c, ev := full.question(t, full.req("Петровы"))
	if ev.Kind != "export" || ev.Tool != "" || ev.Files < 3 || ev.Alt != askAlt || ev.OK != askOKExport {
		t.Errorf("полный ход: %+v", ev)
	}
	full.decline(t, c, ev)
	// Для сравнения: у создания альбома ссылка «Не спрашивать» есть.
	cc := full.ask(full.createReq())
	if cr := full.asked(t, 2); cr.Tool != toolCreateAlbum {
		t.Errorf("create: Tool %q", cr.Tool)
	}
	cc.cancel()
	cc.wait(t)

	r := newAskRig(t)
	rec, _, err := r.a.askPostWith(agents.AskReq{Kind: agents.AskExport, Client: "Claude Code"}, r.a.editor(), folderPlan{},
		askBody{Title: "t", OK: askOKExportPick}, &exportAsk{files: 3})
	if err != nil || rec == nil {
		t.Fatalf("askPostWith: %v", err)
	}
	ev = r.asked(t, 1)
	if ev.Kind != "export" || ev.Tool != "" || ev.Files != 3 || ev.Alt != "" {
		t.Errorf("событие: %+v", ev)
	}
}

// TestExportPrecheckNoWindow: альбом без разворотов — отказ словами окна «Экспорт» без вопроса и без окна;
// чужой ключ и ревизия — свои отказы агента.
func TestExportPrecheckNoWindow(t *testing.T) {
	r := newAskRig(t)
	key := r.a.askKey(r.a.editor())
	rev := r.a.editor().View().Rev
	c := r.ask(agents.AskReq{Kind: agents.AskExport, Client: "Claude Code", Key: key, Rev: rev})
	c.wait(t)
	if c.err == nil || !strings.Contains(c.err.Error(), "нечего экспортировать") {
		t.Errorf("пустой альбом: %v", c.err)
	}
	if len(r.evOf("agents:ask")) != 0 {
		t.Error("окно вопроса показано")
	}
	c = r.ask(agents.AskReq{Kind: agents.AskExport, Client: "Claude Code", Key: key, Rev: rev + 5})
	c.wait(t)
	if !errors.Is(c.err, agents.ErrAlbumEdited) {
		t.Errorf("чужая ревизия: %v", c.err)
	}
	c = r.ask(agents.AskReq{Kind: agents.AskExport, Client: "Claude Code", Key: "ffff", Rev: rev})
	c.wait(t)
	if !errors.Is(c.err, agents.ErrAlbumChanged) {
		t.Errorf("чужой ключ: %v", c.err)
	}
}

// TestAgentExportHold: Host.ExportHold — выгрузка (окна и фон) держит create_album и open_album; прочие
// инструменты, в том числе правки текущего альбома, не держит; без выгрузки не держит никого (0.35-152-27).
func TestAgentExportHold(t *testing.T) {
	a := newAskRig(t).a
	hold := a.agentsHost().ExportHold // подключение в Host, а не только метод App
	if hold == nil {
		t.Fatal("Host.ExportHold не заполнен")
	}
	oldRun := exportRunning
	t.Cleanup(func() { exportRunning = oldRun })
	exportRunning = func(*App) bool { return false }
	for _, tool := range []string{"create_album", "open_album", "autobuild", ""} {
		if hold(tool) {
			t.Errorf("%q держится без выгрузки", tool)
		}
	}
	exportRunning = func(*App) bool { return true } // окна или фон — флаг службы один
	if !hold("create_album") || !hold("open_album") {
		t.Error("выгрузка не держит создание и открытие")
	}
	for _, tool := range []string{"autobuild", "add_folder", "set_background", "undo_last_edit", "export_album", ""} {
		if hold(tool) {
			t.Errorf("%q держится выгрузкой", tool)
		}
	}
}
