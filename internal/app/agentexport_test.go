package app

// Выгрузка по просьбе агента (152 Т3): правило места записи, вопрос без ссылки «Не спрашивать», отказы проверок без
// пути, тихие create/open при окнах выгрузки. Служебная часть (схема, итоги) — agents/export_tools_test.go.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/app/exporter"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// writeRules — правила записи для таблицы: Windows или Unix, домашняя, три целые папки, наши, системные деревья.
func writeRules(windows bool) *folderRules {
	r := &folderRules{windows: windows, fold: paths.Fold, timeout: 1e9, disk: func(p string) (string, error) { return p, nil }}
	if windows {
		r.fold = strings.ToLower // регистр Windows не важен, а тест идёт и на Linux
		r.system = "C:"
		r.homes = []string{`C:\Users\u`}
		r.whole = []string{`C:\Users\u\Desktop`, `C:\Users\u\Documents`, `C:\Users\u\Downloads`}
		r.ours = []string{`C:\Users\u\AppData\Roaming\SosninaAlbums`, `C:\Program Files\SosninaAlbums`}
		r.trees, _, r.exact, r.elems = windowsLists(`C:\Users\u`, func(string) string { return "" })
		return r
	}
	r.homes = []string{"/home/u"}
	r.whole = []string{"/home/u/Desktop", "/home/u/Documents", "/home/u/Downloads"}
	r.ours = []string{"/home/u/.config/sosninaalbums"}
	r.trees, r.allow, r.exact, r.elems = osLists("linux", "/home/u", func(string) string { return "" })
	return r
}

// TestAgentExportHintDestination: куда агент может предложить выгрузку (U59, U61): флешка, «Рабочий стол\Петровы» — да;
// системные, корень системного тома, домашняя, скрытое (`.ssh` и на Windows), UNC, папка программы — нет.
func TestAgentExportHintDestination(t *testing.T) {
	win := writeRules(true)
	for _, c := range []struct {
		p  string
		ok bool
	}{
		{`E:\`, true}, {`E:\Выгрузки\Петровы`, true}, {`C:\Users\u\Desktop`, true}, {`C:\Users\u\Desktop\Петровы`, true},
		{`C:\Users\u\Documents\x`, true}, {`C:\Users\u\Петровы`, true},
		{`C:\Windows`, false}, {`C:\Windows\Temp\x`, false}, {`C:\`, false}, {`C:\Users`, false}, {`C:\Users\u`, false},
		{`C:\Users\u\.ssh`, false}, {`D:\a\.hidden\x`, false}, {`\\server\share\x`, false}, {`C:\Program Files\SosninaAlbums\x`, false},
		{`C:\Users\u\AppData\Roaming\SosninaAlbums`, false}, {`relative\x`, false}, {`D:\System Volume Information`, false},
		{"E:\\a\u202eb", false}, {"", false},
	} {
		if _, err := win.judgeWrite(c.p); (err == nil) != c.ok {
			t.Errorf("Windows %q: err=%v, ждали ok=%v", c.p, err, c.ok)
		}
	}
	unix := writeRules(false)
	for _, c := range []struct {
		p  string
		ok bool
	}{
		{"/media/u/USB/Выгрузки", true}, {"/home/u/Desktop/Петровы", true}, {"/home/u/Документы2/x", true},
		{"/etc/x", false}, {"/", false}, {"/home/u", false}, {"/home", false}, {"/home/u/.ssh", false},
		{"/home/u/.config/sosninaalbums/x", false}, {"//server/x", false}, {"/media", false}, {"/proc/self", false},
	} {
		if _, err := unix.judgeWrite(c.p); (err == nil) != c.ok {
			t.Errorf("Unix %q: err=%v, ждали ok=%v", c.p, err, c.ok)
		}
	}
}

// TestAgentExportHintDisk: подсказка на настоящем диске (Unix): папка есть — можно, нет последней — «создать»,
// глубже одного уровня, файл вместо папки, ссылка в системную папку — нет; ссылка в свою папку показывается
// настоящим путём; после вопроса папку подменили ссылкой в чужое — повтор при щелчке отказывает (У60).
func TestAgentExportHintDisk(t *testing.T) {
	base := testkit.TempDir(t)
	r := writeRules(false)
	r.disk = diskProbe
	mk := func(parts ...string) string {
		p := filepath.Join(append([]string{base}, parts...)...)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	own := mk("Выгрузки")
	if w, err := r.writeDir(own); err != nil || w.create || w.path != own {
		t.Errorf("есть папка: %+v %v", w, err)
	}
	if w, err := r.writeDir(filepath.Join(own, "Петровы")); err != nil || !w.create {
		t.Errorf("нет последней: %+v %v", w, err)
	}
	if _, err := r.writeDir(filepath.Join(own, "а", "б")); err == nil {
		t.Error("глубже одного уровня создавать нельзя")
	}
	file := filepath.Join(base, "файл.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := r.writeDir(file); err == nil {
		t.Error("файл вместо папки принят")
	}
	toEtc, toReal := filepath.Join(base, "в_etc"), filepath.Join(base, "в_свою")
	if err := os.Symlink("/etc", toEtc); err != nil {
		t.Skipf("ссылки недоступны: %v", err)
	}
	if err := os.Symlink(own, toReal); err != nil {
		t.Fatal(err)
	}
	if _, err := r.writeDir(toEtc); err == nil {
		t.Error("ссылка в /etc принята")
	}
	if _, err := r.writeDir(filepath.Join(toEtc, "Петровы")); err == nil {
		t.Error("новая папка внутри ссылки в /etc принята")
	}
	if w, err := r.writeDir(toReal); err != nil || w.path != own {
		t.Errorf("ссылка в свою папку: %+v %v (ждали настоящий путь %q)", w, err, own)
	}
	shown := filepath.Join(own, "Новая")
	w, err := r.writeDir(shown)
	if err != nil || !w.create {
		t.Fatalf("показанная: %+v %v", w, err)
	}
	if err := os.Symlink("/etc", shown); err != nil { // между вопросом и щелчком
		t.Fatal(err)
	}
	if _, err := r.exportPick(shown); err == nil {
		t.Error("подмена ссылкой в /etc между вопросом и щелчком не поймана")
	}
}

// TestAgentExportPickCreateRecheck: папку, которой не было, успели подменить — повтор при щелчке её не берёт; обычная
// папка вместо несуществующей — берётся как существующая.
func TestAgentExportPickCreateRecheck(t *testing.T) {
	r := writeRules(false)
	exists := map[string]string{"/media/u/X": "/media/u/X"}
	r.disk = func(p string) (string, error) {
		if v, ok := exists[p]; ok {
			return v, nil
		}
		return "", fs.ErrNotExist
	}
	w, err := r.writeDir("/media/u/X/Петровы")
	if err != nil || !w.create || w.path != "/media/u/X/Петровы" {
		t.Fatalf("новая папка: %+v %v", w, err)
	}
	if _, err := r.writeDir("/media/u/X/Глубже/Петровы"); err == nil {
		t.Error("глубже одного уровня создавать нельзя")
	}
	exists["/media/u/X/Петровы"] = "/media/u/X/Петровы" // появилась обычная папка
	if w, err := r.exportPick("/media/u/X/Петровы"); err != nil || w.create {
		t.Errorf("появилась обычная: %+v %v", w, err)
	}
	exists["/media/u/X/Петровы"] = "/etc" // подменили ссылкой в чужое
	if _, err := r.exportPick("/media/u/X/Петровы"); err == nil {
		t.Error("ссылка в системную папку принята")
	}
	exists["/media/u/X/Петровы"] = "/media/u/Y/Петровы" // ссылка в другое место: ключ не совпал
	if _, err := r.exportPick("/media/u/X/Петровы"); err == nil {
		t.Error("ссылка в другую папку принята")
	}
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

// TestAgentQuietWhileExportDialogs: пока открыты окна выгрузки, тихое и вопросное создание альбома — «идёт выгрузка»;
// фон выгрузки создание не держит (снимок уже взят).
func TestAgentQuietWhileExportDialogs(t *testing.T) {
	r := newAskRig(t)
	old, oldRun := exportDialogs, exportRunning
	t.Cleanup(func() { exportDialogs, exportRunning = old, oldRun })
	exportDialogs = func(*App) bool { return true }
	exportRunning = func(*App) bool { return true } // флаг службы поднят до окон
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
	exportDialogs, exportRunning = func(*App) bool { return false }, func(*App) bool { return false }
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

// TestAgentExportHold: Host.ExportHold — окна выгрузки держат create_album, выгрузка с фоном — open_album; прочие
// инструменты выгрузка не держит; без выгрузки не держит никого.
func TestAgentExportHold(t *testing.T) {
	a := newAskRig(t).a
	old, oldRun := exportDialogs, exportRunning
	t.Cleanup(func() { exportDialogs, exportRunning = old, oldRun })
	exportDialogs, exportRunning = func(*App) bool { return false }, func(*App) bool { return false }
	for _, tool := range []string{"create_album", "open_album", "autobuild", ""} {
		if a.agentExportHold(tool) {
			t.Errorf("%q держится без выгрузки", tool)
		}
	}
	exportRunning = func(*App) bool { return true } // фон: снимок взят, создание можно
	if a.agentExportHold("create_album") || !a.agentExportHold("open_album") || a.agentExportHold("autobuild") {
		t.Error("фон выгрузки")
	}
	exportDialogs = func(*App) bool { return true }
	if !a.agentExportHold("create_album") || !a.agentExportHold("open_album") || a.agentExportHold("add_folder") {
		t.Error("окна выгрузки")
	}
}
