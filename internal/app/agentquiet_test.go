package app

// Тихая папка и новый альбом при «Всегда» (150, design §4.1, §4.5): внутри мест — без вопроса и одним
// шагом отмены; вне мест, сеть, много файлов, заглушки, снимки экрана, ссылки, черновик человека —
// вопрос с причиной Why. Стенд — askRig с настоящим диском для разрешения ссылок.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// quietRig — стенд тихого пути: место «Архив», в нём папка «Петровы» с фото; «Всегда» у папки и альбома.
type quietRig struct {
	*askRig
	place, shoot string
}

func realPath(t *testing.T, p string) string {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func newQuietRig(t *testing.T) *quietRig {
	t.Helper()
	r := &quietRig{askRig: newAskRig(t)}
	oldRemote, oldShots, oldCloud := quietRemote, quietShots, quietCloud
	quietRemote, quietShots = func(string) bool { return false }, func() []string { return nil }
	noHiddenAttrs(t) // Windows: TempDir под скрытой AppData, иначе любая подсказка — «bad»
	t.Cleanup(func() { quietRemote, quietShots, quietCloud = oldRemote, oldShots, oldCloud })
	r.a.askM().rules = func() *folderRules { x := permissiveRules(); x.disk = diskProbe; return x }
	r.place = realPath(t, filepath.Join(testkit.TempDir(t), "Архив"))
	r.shoot = filepath.Join(r.place, "Петровы")
	if err := os.Mkdir(r.shoot, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		testkit.JPEG(t, r.shoot, fmt.Sprintf("Кадр_%d.jpg", i), 600, 400)
	}
	prefs.UpdateAgentPerms(r.a.cfgDir, func(p *prefs.AgentPerms) {
		p.Tools[toolAddFolder], p.Tools[toolCreateAlbum] = prefs.PermAllow, prefs.PermAllow
		p.Places = prefs.PushPlace(p.Places, prefs.Place{Dir: r.place, How: prefs.PlaceChosen})
	})
	return r
}

// setMode — разрешение инструмента, как это делает человек в окне.
func (r *quietRig) setMode(tool, mode string) {
	prefs.UpdateAgentPerms(r.a.cfgDir, func(p *prefs.AgentPerms) { p.Tools[tool] = mode })
}

// mkdir создаёт папку с n фото (по имени: счёт идёт по расширению) под место.
func (r *quietRig) mkdir(t *testing.T, name string, files ...string) string {
	t.Helper()
	dir := filepath.Join(r.place, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func (r *quietRig) addQuiet(hint string) agents.AskReq {
	q := r.addReq()
	q.Hint, q.Always = hint, true
	return q
}

func (r *quietRig) createQuiet(hint string) agents.AskReq {
	q := r.createReq()
	q.Hint, q.Always = hint, true
	return q
}

// run — вызов agentAsk до конца: итог без вопроса человеку.
func (r *quietRig) run(t *testing.T, q agents.AskReq) *askCall {
	t.Helper()
	c := r.ask(q)
	c.wait(t)
	return c
}

// refuse — вопрос показан: возвращает его, закрывает отказом.
func (r *quietRig) refuse(t *testing.T, c *askCall, n int) AgentAsk {
	t.Helper()
	ev := r.asked(t, n)
	if _, err := r.a.AnswerAgent(ev.ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	return ev
}

func (r *quietRig) folders() []string { return r.a.editor().View().Folders }

func edited(r *askRig) []AgentEdited {
	var out []AgentEdited
	for _, d := range r.evOf("agents:edited") {
		out = append(out, d.(AgentEdited))
	}
	return out
}

func TestQuietInPlace(t *testing.T) {
	r := newQuietRig(t)
	places := placeDirs(t, r.a)
	rev := r.rev()
	c := r.run(t, r.addQuiet(r.shoot))
	if c.err != nil || c.res.Why != agents.WhyQuiet || c.res.Folder != 2 || c.res.Merged {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if n := len(r.evOf("agents:ask")); n != 0 || r.fronted() != 0 {
		t.Errorf("вопрос и окно вперёд: %d, %d", n, r.fronted())
	}
	if !contains(r.folders(), r.shoot) {
		t.Fatalf("папка не добавлена: %v", r.folders())
	}
	if got := placeDirs(t, r.a); !strings.EqualFold(fmt.Sprint(got), fmt.Sprint(places)) {
		t.Errorf("тихая папка изменила места: %v → %v", places, got)
	}
	ev := edited(r.askRig)
	if len(ev) != 1 || !ev[0].Quiet || ev[0].Path != r.shoot || ev[0].Prev || ev[0].Rev != c.res.Album.Rev ||
		strings.Contains(ev[0].Text, r.shoot) || ev[0].Text != "Добавлена папка" {
		t.Errorf("agents:edited: %+v", ev)
	}
	// Один шаг отмены: Ctrl+Z возвращает альбом.
	r.a.editor().Undo()
	if r.rev() < rev || contains(r.folders(), r.shoot) || len(r.folders()) != 1 {
		t.Errorf("отмена одним шагом: %v", r.folders())
	}
	// Повтор той же папки — слияние: шага нет, тоста нет.
	r.run(t, r.addQuiet(r.shoot))
	before := len(edited(r.askRig))
	q := r.addQuiet(r.shoot)
	q.Rev = r.rev()
	if c := r.run(t, q); c.res.Why != agents.WhyQuiet || !c.res.Merged || len(edited(r.askRig)) != before {
		t.Errorf("слияние: %+v, тостов %d→%d", c.res, before, len(edited(r.askRig)))
	}
}

func TestQuietOutsidePlace(t *testing.T) {
	r := newQuietRig(t)
	outside := realPath(t, filepath.Join(testkit.TempDir(t), "Чужая"))
	c := r.ask(r.addQuiet(outside))
	ev := r.refuse(t, c, 1)
	if ev.Why != whyPlace || ev.Path != outside || ev.Alt == "" || ev.Tool != "add_folder" {
		t.Errorf("вопрос: %+v", ev)
	}
	if len(r.folders()) != 1 {
		t.Error("папка вне места добавлена")
	}
	// Само место — не «внутри» него: строго внутри.
	c = r.ask(r.addQuiet(r.place))
	if ev := r.refuse(t, c, 2); ev.Why != whyPlace {
		t.Errorf("само место: %+v", ev)
	}
	// Мест нет — тихо нельзя нигде.
	prefs.ForgetPlaces(r.a.cfgDir)
	c = r.ask(r.addQuiet(r.shoot))
	if ev := r.refuse(t, c, 3); ev.Why != whyPlace {
		t.Errorf("без мест: %+v", ev)
	}
	// Подсказки нет вовсе — тоже вопрос «place».
	c = r.ask(r.addQuiet(""))
	if ev := r.refuse(t, c, 4); ev.Why != whyPlace || ev.Path != "" {
		t.Errorf("без подсказки: %+v", ev)
	}
}

func TestQuietSymlinkOut(t *testing.T) {
	r := newQuietRig(t)
	outside := realPath(t, filepath.Join(testkit.TempDir(t), "Наружу"))
	link := filepath.Join(r.place, "Ссылка")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("ссылки недоступны:", err)
	}
	c := r.ask(r.addQuiet(link))
	ev := r.refuse(t, c, 1)
	if ev.Why != whyPlace || ev.Path != outside || !ev.Link {
		t.Errorf("ссылка наружу: %+v", ev)
	}
	// Ссылка внутри места на папку внутри места — настоящий путь внутри: тихо, «по ссылке» в тосте не нужно.
	in := filepath.Join(r.place, "Алиас")
	if err := os.Symlink(r.shoot, in); err != nil {
		t.Fatal(err)
	}
	if c := r.run(t, r.addQuiet(in)); c.res.Why != agents.WhyQuiet {
		t.Errorf("ссылка внутри места: %+v, %v", c.res, c.err)
	}
	if got := edited(r.askRig); len(got) != 1 || got[0].Path != r.shoot {
		t.Errorf("путь в тосте — настоящий: %+v", got)
	}
}

func TestQuietLinkedRoot(t *testing.T) {
	r := newQuietRig(t)
	link := filepath.Join(r.place, "Корень")
	if err := os.Symlink(r.shoot, link); err != nil {
		t.Skip("ссылки недоступны:", err)
	}
	// Диск «не разрешил» ссылку (как junction на Windows, который EvalSymlinks не раскрывает): корень-ссылка — не тихо.
	r.a.askM().rules = func() *folderRules { x := permissiveRules(); x.disk = diskOK; return x }
	c := r.ask(r.addQuiet(link))
	if ev := r.refuse(t, c, 1); ev.Why != whyLink {
		t.Errorf("корень-ссылка: %+v", ev)
	}
	if len(r.folders()) != 1 {
		t.Error("добавлено через ссылку")
	}
}

func TestQuietRemote(t *testing.T) {
	r := newQuietRig(t)
	var probed atomic.Value
	quietRemote = func(p string) bool { probed.Store(p); return true }
	c := r.ask(r.addQuiet(r.shoot))
	if ev := r.refuse(t, c, 1); ev.Why != whyRemote {
		t.Errorf("сетевой том: %+v", ev)
	}
	if probed.Load() != r.shoot {
		t.Errorf("проба тома по настоящему пути: %v", probed.Load())
	}
}

func TestQuietCount(t *testing.T) {
	r := newQuietRig(t)
	many := func(n int, ext string) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("f%05d%s", i, ext)
		}
		return out
	}
	mix := append(append(many(2000, ".jpg"), many(2000, ".xmp")...), many(2000, ".cr2")...)
	d500 := r.mkdir(t, "много_папок")
	for i := range 501 {
		if err := os.Mkdir(filepath.Join(d500, fmt.Sprint(i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name string
		dir  string
		over bool
	}{
		{"5000 фото", r.mkdir(t, "ровно", many(5000, ".jpg")...), false},
		{"5001 фото", r.mkdir(t, "лишнее", many(5001, ".jpg")...), true},
		{"RAW+JPG+XMP 6000 записей при 2000 фото", r.mkdir(t, "смесь", mix...), false},
		{"20001 запись", r.mkdir(t, "записи", many(20001, ".txt")...), true},
		{"501 папка", d500, true},
		{"пусто", r.mkdir(t, "пусто"), false},
	}
	for _, c := range cases {
		if got := countQuiet(c.dir); got.over() != c.over || got.cloud {
			t.Errorf("%s: %+v, ждали over=%v", c.name, got, c.over)
		}
	}
	// Через вопрос: 5001 фото — причина big, папка не добавлена.
	c := r.ask(r.addQuiet(cases[1].dir))
	if ev := r.refuse(t, c, 1); ev.Why != whyBig {
		t.Errorf("много фото: %+v", ev)
	}
	// Зависший диск: порция не возвращается — не дольше срока, итог big; обход потом останавливается.
	release := make(chan struct{})
	var stopped atomic.Bool
	oldPause, oldTO := quietPause, quietTimeout
	quietTimeout = 150 * time.Millisecond
	quietPause = func() {
		<-release
		stopped.Store(true)
	}
	t.Cleanup(func() { quietPause, quietTimeout = oldPause, oldTO })
	start := time.Now()
	got := countQuiet(r.shoot)
	if !got.over() || time.Since(start) > testkit.Limit(3*time.Second) {
		t.Errorf("зависший диск: %+v за %v", got, time.Since(start))
	}
	close(release)
	waitUntil(t, "зависшая порция вышла", stopped.Load)
}

func TestQuietPlaceholder(t *testing.T) {
	r := newQuietRig(t)
	cloud := r.mkdir(t, "Облако", "локально.jpg", "облако.jpg")
	quietCloud = func(e fs.DirEntry) bool { return e.Name() == "облако.jpg" } // атрибуты подменены
	c := r.ask(r.addQuiet(cloud))
	if ev := r.refuse(t, c, 1); ev.Why != whyCloud {
		t.Errorf("заглушка: %+v", ev)
	}
	if got := countQuiet(r.shoot); got.cloud || got.photos != 2 {
		t.Errorf("обычная папка: %+v", got)
	}
}

func TestQuietScreenshots(t *testing.T) {
	r := newQuietRig(t)
	shots := r.mkdir(t, "Снимки экрана", "a.png")
	quietShots = func() []string { return []string{shots} }
	for i, dir := range []string{shots, r.mkdir(t, filepath.Join("Снимки экрана", "2026"), "b.png")} {
		c := r.ask(r.addQuiet(dir))
		if ev := r.refuse(t, c, i+1); ev.Why != whyShots {
			t.Errorf("%s: %+v", dir, ev)
		}
	}
	if c := r.run(t, r.addQuiet(r.shoot)); c.res.Why != agents.WhyQuiet {
		t.Errorf("обычная папка рядом: %+v", c.res)
	}
}
