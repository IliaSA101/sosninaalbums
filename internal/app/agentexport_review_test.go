package app

// Доправка ревью и аудита 152 (0.35-152-25): настройки без типографии, запоминание корня, подмена папки,
// имя в отказе.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestExportOptionsDefaultPrint: типография по умолчанию — только если агент не задал ничего из того, что она
// игнорирует: диапазон, страницы, только обложку, dpi, качество, обложку (quality:80 без print — обычная выгрузка).
func TestExportOptionsDefaultPrint(t *testing.T) {
	cases := []struct {
		name string
		req  agents.AskReq
		want string
	}{
		{"пусто", agents.AskReq{}, "fabrika"},
		{"качество", agents.AskReq{Quality: 80}, ""},
		{"обложка", agents.AskReq{Cover: true}, ""},
		{"диапазон", agents.AskReq{Range: "1-2"}, ""},
		{"dpi", agents.AskReq{DPI: 72}, ""},
		{"страницы", agents.AskReq{Pages: true}, ""},
		{"только обложка", agents.AskReq{CoverOnly: true}, ""},
		{"явная типография и качество", agents.AskReq{Print: "fabrika", Quality: 80}, "fabrika"},
		{"plain", agents.AskReq{Print: "plain"}, ""},
	}
	for _, c := range cases {
		if got := exportOptions(c.req, "fabrika").Print; got != c.want {
			t.Errorf("%s: Print %q, ждали %q", c.name, got, c.want)
		}
	}
}

// lateFolder — прежняя выгрузка «Тест» переезжает в «Петровы» между вопросом и щелчком: набор файлов неизвестен
// вопросу, и ReplaceQuestion спросит заново (окно после выбора папки). Возвращает вопрос и вызов.
func (r *exportRig) lateFolder(t *testing.T) (*askCall, AgentAsk) {
	t.Helper()
	r.run(t, r.req("Тест"))
	c, ev := r.question(t, r.req("Петровы"))
	if !ev.Create {
		t.Fatalf("Петровы должна быть новой: %+v", ev)
	}
	if err := os.Rename(filepath.Join(r.root, "Тест"), filepath.Join(r.root, "Петровы")); err != nil {
		t.Fatal(err)
	}
	return c, ev
}

// TestExportRememberAfterRefusal: «нет» в последнем окне (замена файлов) не запоминает папку выгрузки; «да» — запоминает
// после записи.
func TestExportRememberAfterRefusal(t *testing.T) {
	r := newExportRig(t, 2)
	c, ev := r.lateFolder(t)
	before := r.a.loadConfig().LastDirs
	r.yes = false
	st, err := r.a.AgentAnswerExport(ev.ID, false)
	c.wait(t)
	if err != nil || st.Started || len(r.warns) != 1 || c.res.Why != agents.WhyStopped {
		t.Fatalf("отказ в окне: %+v, %v, окон %q, итог %+v", st, err, r.warns, c.res)
	}
	if got := r.a.loadConfig().LastDirs; got[prefs.DirExport] != before[prefs.DirExport] || got[prefs.DirExportRoot] != before[prefs.DirExportRoot] {
		t.Errorf("после отказа запомнено %v, было %v", got, before)
	}
	// Тот же вопрос, «да»: папка запоминается.
	r.yes = true
	c, ev = r.question(t, r.req("Петровы"))
	if _, err := r.a.AgentAnswerExport(ev.ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	r.finish(t)
	if got := r.a.loadConfig().LastDirs[prefs.DirExport]; got != filepath.Join(r.root, "Петровы") {
		t.Errorf("после выгрузки запомнено %q", got)
	}
}

// TestExportFolderSwappedBeforeWrite: между последним окном и записью показанную папку подменили ссылкой на другую —
// повторная сверка отказывает: ничего не записано и не создано, агенту «папка не подошла» без пути.
func TestExportFolderSwappedBeforeWrite(t *testing.T) {
	r := newExportRig(t, 2)
	other := filepath.Join(filepath.Dir(r.root), "Другая")
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	c, ev := r.lateFolder(t)
	dir := filepath.Join(r.root, "Петровы")
	ask := askYes
	askYes = func(a *App, msg string) (bool, error) {
		ok, err := ask(a, msg)
		if e := os.Rename(dir, dir+" старая"); e != nil {
			t.Error(e)
		} else if e := os.Symlink(other, dir); e != nil {
			t.Skipf("ссылки недоступны: %v", e)
		}
		return ok, err
	}
	before := r.a.loadConfig().LastDirs[prefs.DirExport]
	st, err := r.a.AgentAnswerExport(ev.ID, false)
	c.wait(t)
	if err != nil || st.Started || c.res.Why != agents.WhyTooWide || c.err != nil {
		t.Fatalf("подмена: %+v, %v, итог %+v, %v", st, err, c.res, c.err)
	}
	if n := len(jpgs(t, other)); n != 0 {
		t.Errorf("в подменённую папку записано %d файлов", n)
	}
	if got := r.a.loadConfig().LastDirs[prefs.DirExport]; got != before {
		t.Errorf("запомнено %q, было %q", got, before)
	}
	if n := len(jpgs(t, dir+" старая")); n != 2 {
		t.Errorf("прежние файлы тронуты: %d", n)
	}
	if busy, _ := r.a.exp.Busy(); busy {
		t.Error("флаг выгрузки не снят")
	}
}

// TestWriteDirNewNameValid: имя новой папки из подсказки-пути годно, как и подсказка-имя: точка и пробел на краю,
// знаки `<>:"|?*`, имя устройства, управляющие знаки — отказ; годное имя — «создать».
func TestWriteDirNewNameValid(t *testing.T) {
	base := testkit.TempDir(t)
	r := writeRules(false)
	r.disk = diskProbe
	for _, bad := range []string{"Петровы.", "Петровы ", "a<b", "a|b", "CON", "a\u202eb", "a?b"} {
		if w, err := r.writeDir(filepath.Join(base, bad)); err == nil {
			t.Errorf("негодное имя %q принято: %+v", bad, w)
		}
	}
	for _, ok := range []string{"Петровы", "Петровы 2027", "a.b"} {
		if w, err := r.writeDir(filepath.Join(base, ok)); err != nil || !w.create {
			t.Errorf("годное имя %q: %+v, %v", ok, w, err)
		}
	}
	// Существующую папку с «плохим» именем (чужая, уже есть на диске) правило имени не трогает: она не создаётся.
	odd := filepath.Join(base, "Старая.")
	if err := os.Mkdir(odd, 0o755); err != nil {
		t.Skipf("имя не создаётся на этой ОС: %v", err)
	}
	if w, err := r.writeDir(odd); err != nil || w.create {
		t.Errorf("существующая папка: %+v, %v", w, err)
	}
}
