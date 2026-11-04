package app

// Тихое открытие альбома по ключу при «Всегда» (160 Т2): исход, каждая причина, стражи с контролем, смена
// альбома человеком, срок. Стенд — quietRig с местом «Архив» и двумя сохранёнными альбомами в «Последних».

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

type oqRig struct {
	*quietRig
	dir, prev, target string
}

// newOQRig: открыт сохранённый «Прежний»; «Ивановы» — в «Последних», в нём папка r.shoot (внутри места); «Всегда» у open_album.
func newOQRig(t *testing.T) *oqRig {
	t.Helper()
	r := &oqRig{quietRig: newQuietRig(t), dir: testkit.UserTemp(t)}
	r.setMode(toolOpenAlbum, prefs.PermAllow)
	r.target = r.album(t, "Ивановы", r.shoot)
	r.prev = r.album(t, "Прежний")
	if _, err := r.a.openPath(r.prev); err != nil {
		t.Fatal(err)
	}
	return r
}

// album сохраняет альбом name с папками folders и кладёт его в «Последние».
func (r *oqRig) album(t *testing.T, name string, folders ...string) string {
	t.Helper()
	ed := blankEditor("album")
	for _, f := range folders {
		if _, _, err := ed.AddFolder(f); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(r.dir, name+projectExt)
	if err := ed.Save(path); err != nil {
		t.Fatal(err)
	}
	r.a.rememberProject(path)
	return path
}

func (r *oqRig) req(path string) agents.AskReq {
	return agents.AskReq{Kind: agents.AskOpen, Client: "Claude Code", Key: agents.RecentKey(path), Always: true}
}

// quietly — вызов до конца; итог без вопроса человеку.
func (r *oqRig) quietly(t *testing.T, path string) *askCall {
	t.Helper()
	c := r.ask(r.req(path))
	c.wait(t)
	return c
}

// asks — вызов должен кончиться вопросом с причиной why; вопрос закрывается отказом.
func (r *oqRig) asks(t *testing.T, name, path, why string) {
	t.Helper()
	t.Log("вопрос:", name)
	n := len(r.evOf("agents:ask"))
	c := r.ask(r.req(path))
	ev := r.asked(t, n+1)
	if ev.Why != why || ev.Kind != agents.AskOpen {
		t.Errorf("%s: Why = %q, ждали %q", name, ev.Why, why)
	}
	if _, err := r.a.AnswerAgent(ev.ID, false); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	c.wait(t)
	if r.a.editor().Path() != r.prev {
		t.Errorf("%s: альбом сменился: %q", name, r.a.editor().Path())
	}
}

func TestOpenQuietHappy(t *testing.T) {
	r := newOQRig(t)
	c := r.quietly(t, r.target)
	if c.err != nil || c.res.Why != agents.WhyQuiet || c.res.Album.Key != r.key() || c.res.Album.Rev != r.rev() {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if r.a.editor().Path() != r.target || !contains(r.folders(), r.shoot) {
		t.Errorf("открыт не тот альбом: %q %v", r.a.editor().Path(), r.folders())
	}
	if n := len(r.evOf("agents:ask")); n != 0 || r.fronted() != 0 {
		t.Errorf("вопрос и окно вперёд: %d, %d", n, r.fronted())
	}
	ev := edited(r.askRig)
	if len(ev) != 1 || !ev[0].Quiet || ev[0].Rev != -1 || !ev[0].Prev || !strings.Contains(ev[0].Text, "«Ивановы»") {
		t.Errorf("тост: %+v", ev)
	}
	if got := prefs.LoadAgentPerms(r.a.cfgDir).Places; len(got) != 1 {
		t.Errorf("тихое открытие изменило места: %v", got)
	}
}

// TestOpenQuietPrev: «Открыть прежний» возвращает прежний файл; пустой альбом открывается тихо.
func TestOpenQuietPrev(t *testing.T) {
	r := newOQRig(t)
	empty := r.album(t, "Пустой")
	if c := r.quietly(t, empty); c.err != nil || c.res.Why != agents.WhyQuiet { // TestOpenQuietEmpty
		t.Fatalf("пустой альбом: %+v, %v", c.res, c.err)
	}
	if c := r.quietly(t, r.target); c.err != nil || c.res.Why != agents.WhyQuiet {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	st, err := r.a.AgentOpenPrev()
	if err != nil || st.View.Path != empty {
		t.Fatalf("прежний: %v, %q", err, st.View.Path)
	}
}

func TestOpenQuietWhy(t *testing.T) {
	r := newOQRig(t)
	t.Run("rate", func(t *testing.T) {
		q := r.req(r.target)
		q.Always = false
		n := len(r.evOf("agents:ask"))
		c := r.ask(q)
		ev := r.asked(t, n+1)
		if ev.Why != whyRate {
			t.Errorf("Why = %q", ev.Why)
		}
		_, _ = r.a.AnswerAgent(ev.ID, false)
		c.wait(t)
	})
	t.Run("hint_found_pick_no_args", func(t *testing.T) {
		for _, c := range []struct {
			name string
			q    agents.AskReq
			why  string
		}{
			{"hint_found", func() agents.AskReq { q := r.req(r.target); q.Hint = "Иванов"; return q }(), whyHint},
			{"no_args", agents.AskReq{Kind: agents.AskOpen, Client: "Claude Code", Always: true}, whyPick},
		} {
			n := len(r.evOf("agents:ask"))
			call := r.ask(c.q)
			ev := r.asked(t, n+1)
			if ev.Why != c.why {
				t.Errorf("%s: Why = %q", c.name, ev.Why)
			}
			_, _ = r.a.AnswerAgent(ev.ID, false)
			call.wait(t)
		}
	})
	outside := filepath.Join(testkit.UserTemp(t), "Чужое")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	r.asks(t, "place", r.album(t, "Вне", outside), whyPlace)
	// Запасная папка вне мест — тоже place (запасные пути проверяются как основные).
	r.asks(t, "alt_outside", r.altAlbum(t, "Запас", r.shoot, outside), whyPlace)
	gone := r.mkdir(t, "Исчезнет", "a.jpg")
	goneAlbum := r.album(t, "Исчезла", gone)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	r.asks(t, "gone", goneAlbum, whyGone)
	// Подпапка-ссылка внутри места: фото под ней — link.
	if link := r.linkBelow(t); link != "" {
		r.asks(t, "link_below", link, whyLink)
	}
	// Ссылка вместо файла альбома.
	if ln := filepath.Join(r.dir, "Ссылка"+projectExt); os.Symlink(r.target, ln) == nil {
		r.a.rememberProject(ln)
		r.a.rememberProject(r.prev)
		r.asks(t, "file_link", ln, whyFile)
	}
	// Файл не альбом: JSON битый; новее программы.
	bad := filepath.Join(r.dir, "Битый"+projectExt)
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.a.rememberProject(bad)
	r.a.rememberProject(r.prev)
	r.asks(t, "file_bad", bad, whyFile)
	newer := filepath.Join(r.dir, "Новый"+projectExt)
	if err := os.WriteFile(newer, []byte(`{"version":99999}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r.a.rememberProject(newer)
	r.a.rememberProject(r.prev)
	r.asks(t, "file_newer", newer, whyFile)
	// Много папок: больше 50 корней.
	many := make([]string, 0, openMaxRoots+1)
	for i := range openMaxRoots + 1 {
		many = append(many, r.mkdir(t, "Папка"+string(rune('A'+i%26))+strings.Repeat("x", i/26+1), "a.jpg"))
	}
	r.asks(t, "many_dirs", r.album(t, "Много", many...), whyBig)
	// Правленый черновик человека: draft («Последние» ограничены — вернуть альбом в список).
	r.a.rememberProject(r.target)
	r.a.NewProject("album")
	if _, _, err := r.a.editor().AddFolder(r.shoot); err != nil {
		t.Fatal(err)
	}
	n := len(r.evOf("agents:ask"))
	c := r.ask(r.req(r.target))
	ev := r.asked(t, n+1)
	if ev.Why != whyDraft {
		t.Errorf("draft: Why = %q", ev.Why)
	}
	_, _ = r.a.AnswerAgent(ev.ID, false)
	c.wait(t)
}

// TestOpenQuietWhySwapped: подмены признаков — заглушка файла альбома, сеть, виртуальный облачный диск.
func TestOpenQuietWhySwapped(t *testing.T) {
	r := newOQRig(t)
	oldCloud, oldRemote, oldVirt := quietCloud, quietRemote, quietVirtual
	t.Cleanup(func() { quietCloud, quietRemote, quietVirtual = oldCloud, oldRemote, oldVirt })
	quietCloud = func(e fs.DirEntry) bool { return e.Name() == filepath.Base(r.target) }
	r.asks(t, "stub", r.target, whyStub)
	quietCloud = oldCloud
	quietRemote = func(p string) bool { return strings.HasPrefix(p, r.place) }
	r.asks(t, "remote", r.target, whyRemote)
	quietRemote = oldRemote
	quietVirtual = func(string) bool { return true }
	r.asks(t, "vcloud", r.target, whyCloud)
	quietVirtual = oldVirt
}

// altAlbum — альбом с папкой dir и запасным путём alt к ней (FolderAlt пишется в файл как abs).
func (r *oqRig) altAlbum(t *testing.T, name, dir, alt string) string {
	t.Helper()
	path := r.album(t, name, dir)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Запасной путь у существующей папки: добавляем отдельную запись, которой нет на диске — основной путь
	// «Нет», запасной вне мест.
	gone := filepath.Join(r.place, "НетТакой")
	s := strings.Replace(string(data), `"folders": [`, `"folders": [{"path": `+quote(gone)+`, "abs": `+quote(alt)+`},`, 1)
	if s == string(data) {
		s = strings.Replace(string(data), `"folders":[`, `"folders":[{"path":`+quote(gone)+`,"abs":`+quote(alt)+`},`, 1)
	}
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func quote(s string) string { return `"` + strings.ReplaceAll(filepath.ToSlash(s), `"`, `\"`) + `"` }

// linkBelow — альбом с отдельным фото под папкой-ссылкой внутри места ("" — ссылку создать нельзя).
func (r *oqRig) linkBelow(t *testing.T) string {
	t.Helper()
	target := r.mkdir(t, "Настоящая", "a.jpg")
	ln := filepath.Join(r.shoot, "Ссылка")
	if os.Symlink(target, ln) != nil {
		return ""
	}
	return r.altLoose(t, "Под ссылкой", filepath.Join(ln, "a.jpg"))
}

// altLoose — альбом с одним отдельным фото file.
func (r *oqRig) altLoose(t *testing.T, name, file string) string {
	t.Helper()
	ed := blankEditor("album")
	p := model.New()
	p.Loose = []model.LooseAdd{{ID: "0123456789ab", Files: []model.LooseFile{{Path: file}}}}
	_ = ed
	path := filepath.Join(r.dir, name+projectExt)
	if err := salbum.Save(p, path); err != nil {
		t.Fatal(err)
	}
	r.a.rememberProject(path)
	return path
}

// TestOpenQuietFullSubset: строка полного разбора, которой нет в сухом перечне или чья папка не проверена, — place.
func TestOpenQuietFullSubset(t *testing.T) {
	r := newOQRig(t)
	rules := r.a.askM().rules()
	pl := &openPlan{rules: rules, rows: map[string]bool{}, checked: map[string]bool{}}
	in, other := filepath.Join(r.shoot, "a.jpg"), filepath.Join(r.place, "Чужая", "b.jpg")
	pl.rows[in], pl.checked[r.shoot] = true, true
	full := model.New()
	full.Loose = []model.LooseAdd{{ID: "0123456789ab", Files: []model.LooseFile{{Path: in}}}}
	if why := r.a.openFullWhy(pl, full); why != "" {
		t.Fatalf("проверенная строка: %q", why)
	}
	full.Loose[0].Files = append(full.Loose[0].Files, model.LooseFile{Path: other})
	if why := r.a.openFullWhy(pl, full); why != whyPlace {
		t.Errorf("непроверенная строка: %q", why)
	}
}

// TestOpenQuietNoTouchOutside: диск вне мест не трогается, пока строки не проверены. Контроль: сухой разбор,
// подменённый настоящим (старый шов DecodeDry → Decode), трогает диск по строке вне мест.
func TestOpenQuietNoTouchOutside(t *testing.T) {
	r := newOQRig(t)
	outside := filepath.Join(testkit.UserTemp(t), "Чужое")
	testkit.JPEG(t, outside, "x.jpg", 40, 30)
	alb := r.altAlbum(t, "Вне", r.shoot, outside) // запасная папка вне мест: полный разбор проверит её на диске
	var touched atomic.Int32
	spy := func(op, p string) {
		if !agentplaces.InAny(r.a.askM().rules(), p, []string{r.place, r.dir}) {
			touched.Add(1)
		}
	}
	oldDry, oldDisk := quietDry, quietDisk
	t.Cleanup(func() { quietDry, quietDisk = oldDry, oldDisk })
	quietDisk = func(d salbum.Disk) salbum.Disk { return salbum.Spy(d, spy) }
	r.asks(t, "вне мест", alb, whyPlace)
	if n := touched.Load(); n != 0 {
		t.Fatalf("диск вне мест тронут %d раз", n)
	}
	quietDry = func(data []byte, base string) (*model.Project, error) {
		return salbum.DecodeDisk(data, base, salbum.Spy(salbum.OSDisk(), spy))
	}
	r.asks(t, "контроль", alb, whyPlace)
	if touched.Load() == 0 {
		t.Error("контроль: настоящий разбор до проверки диск вне мест не тронул — страж не ловит")
	}
}

// TestOpenQuietReadOnce: файл читается один раз, а разбор — из тех же байтов: подмена содержимого между планом
// и применением на открытый альбом не влияет. Контроль: openPath читает новый файл.
func TestOpenQuietReadOnce(t *testing.T) {
	r := newOQRig(t)
	var opens atomic.Int32
	oldOpen, oldDisk := quietOpenFile, quietDisk
	t.Cleanup(func() { quietOpenFile, quietDisk = oldOpen, oldDisk })
	quietOpenFile = func(p string) (*os.File, error) { opens.Add(1); return oldOpen(p) }
	swapped := r.album(t, "Другой") // содержимое «Другого» подложим вместо «Ивановых» после плана
	quietDisk = func(d salbum.Disk) salbum.Disk {
		data, err := os.ReadFile(swapped)
		if err != nil {
			t.Error(err)
		}
		if err := os.WriteFile(r.target, data, 0o644); err != nil {
			t.Error(err)
		}
		return d
	}
	if c := r.quietly(t, r.target); c.err != nil || c.res.Why != agents.WhyQuiet {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if opens.Load() != 1 {
		t.Errorf("файл открыт %d раз", opens.Load())
	}
	if !contains(r.folders(), r.shoot) {
		t.Errorf("открыт не прочитанный планом файл: %v", r.folders())
	}
	// Контроль: прежний путь открытия читает файл заново и видит подмену.
	if _, err := r.a.openPath(r.target); err != nil || contains(r.folders(), r.shoot) {
		t.Errorf("контроль: openPath не увидел подмену (%v): %v", err, r.folders())
	}
}

// TestOpenQuietNoFontImport: тихое открытие не ставит шрифты папки альбома: библиотека не меняется, а
// needFolderFonts без шрифтов в альбоме молчит.
func TestOpenQuietNoFontImport(t *testing.T) {
	r := newOQRig(t)
	if r.a.needFolderFonts(model.New(), r.target) {
		t.Error("шрифтов в альбоме нет, а needFolderFonts говорит «нужны»")
	}
	if c := r.quietly(t, r.target); c.err != nil || c.res.Why != agents.WhyQuiet {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if r.a.startInfo != "" {
		t.Errorf("тост шрифтов при тихом открытии: %q", r.a.startInfo)
	}
}

// TestOpenQuietHumanSwitched: человек открыл другой альбом за время проверки — его альбом цел, агенту ErrAlbumChanged.
func TestOpenQuietHumanSwitched(t *testing.T) {
	r := newOQRig(t)
	human := r.album(t, "Человека")
	oldDisk := quietDisk
	t.Cleanup(func() { quietDisk = oldDisk })
	quietDisk = func(d salbum.Disk) salbum.Disk {
		if _, err := r.a.openPath(human); err != nil {
			t.Error(err)
		}
		return d
	}
	c := r.quietly(t, r.target)
	if c.err == nil || !strings.Contains(c.err.Error(), agents.ErrAlbumChanged.Error()) {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if r.a.editor().Path() != human {
		t.Errorf("альбом человека уведён: %q", r.a.editor().Path())
	}
}

// TestOpenQuietDeadline: зависший том не держит вызов дольше срока — big. Срок — шов, без ожидания 3 с.
func TestOpenQuietDeadline(t *testing.T) {
	r := newOQRig(t)
	oldBudget, oldReach := quietOpenBudget, reachable
	hang := make(chan struct{})
	t.Cleanup(func() { close(hang); quietOpenBudget, reachable = oldBudget, oldReach })
	quietOpenBudget = 50 * time.Millisecond
	reachable = func(p string) bool {
		if p == r.target {
			<-hang
		}
		return true
	}
	r.a.askM().rules = func() *agentplaces.Rules { return permissiveOnDisk() }
	start := time.Now()
	r.asks(t, "срок", r.target, whyBig)
	if d := time.Since(start); d > testkit.Limit(2*time.Second) {
		t.Errorf("вызов ждал %v", d)
	}
}
