package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum/collect"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// collectEnv — альбом «Иванов» с фото из двух папок (одинаковые имена файлов), подмена
// диалогов сбора и сбор событий.
type collectEnv struct {
	a      *App
	parent string // выбранная в диалоге папка
	file   string // файл открытого альбома
	photos []string

	mu       sync.Mutex
	asked    []string
	answer   bool
	picks    int
	progress int
	onPick   func()
	onStep   func()
	onDone   func() // до отправки в done: ещё внутри runCollect, до его defer
	done     chan []any
}

func newCollectEnv(t *testing.T, n int) *collectEnv {
	t.Helper()
	e := &collectEnv{parent: testkit.TempDir(t), answer: true, done: make(chan []any, 4)}
	e.file = filepath.Join(testkit.TempDir(t), "Иванов"+projectExt)
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 60, 40
	srcs := []string{testkit.TempDir(t), testkit.TempDir(t)}
	p.Folders = srcs
	for i := range n {
		path := testkit.JPEG(t, srcs[i%2], fmt.Sprintf("IMG_%d.jpg", i/2), 60, 40)
		id := library.IDFor(path)
		e.photos = append(e.photos, path)
		p.Photos[id] = model.PhotoRef{Path: path, W: 60, H: 40}
		p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprintf("s%d", i), Photos: []string{id}})
	}
	e.a = newApp(t)
	e.a.cfgDir = testkit.TempDir(t)
	e.a.emit = func(name string, data ...any) {
		switch name {
		case "collect:progress":
			e.mu.Lock()
			e.progress++
			step := e.onStep
			e.mu.Unlock()
			if step != nil {
				step()
			}
		case "collect:done":
			busy := e.busy() // флаг на момент события: повтор по нему не должен получить отказ
			e.mu.Lock()
			f := e.onDone
			e.mu.Unlock()
			if f != nil {
				f()
			}
			e.done <- append(data, busy)
		}
	}
	e.a.ed.Store(mustEditor(t, p, e.file))
	oldAsk, oldPick := askCollect, pickCollectDir
	askCollect = func(_ *App, msg string) (bool, error) {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.asked = append(e.asked, msg)
		return e.answer, nil
	}
	pickCollectDir = func(*App) (string, error) {
		e.mu.Lock()
		e.picks++
		f := e.onPick
		e.mu.Unlock()
		if f != nil {
			f()
		}
		return e.parent, nil
	}
	t.Cleanup(func() { askCollect, pickCollectDir = oldAsk, oldPick })
	return e
}

// run — Collect и ожидание collect:done: dir, err, note, stopped.
func (e *collectEnv) run(t *testing.T, all bool) (dir, msg, note string, stopped bool) {
	t.Helper()
	ok, err := e.a.Collect(CollectOptions{All: all})
	if err != nil || !ok {
		t.Fatalf("Collect = %v, %v", ok, err)
	}
	select {
	case d := <-e.done:
		if d[4].(bool) {
			t.Fatal("collect:done пришло, пока флаг сбора стоит")
		}
		return d[0].(string), d[1].(string), d[2].(string), d[3].(bool)
	case <-time.After(testkit.Limit(30 * time.Second)):
		t.Fatal("сбор не закончился")
	}
	return
}

// busy — флаг сбора под замком App.
func (e *collectEnv) busy() bool {
	e.a.mu.Lock()
	defer e.a.mu.Unlock()
	return e.a.collecting
}

func (e *collectEnv) book() string { return filepath.Join(e.parent, "Иванов") }

func (e *collectEnv) albumFile() string { return filepath.Join(e.book(), "Иванов"+projectExt) }

// Сц. 1–4, 10: сбор открытого альбома — папка «Иванов» с альбомом и фото обеих папок, альбом
// открывается без пропавших, отметки на новых путях; открытый альбом не меняется (ни шага, ни
// rev, ни пути); папка выбора запоминается, ShowCollected открывает папку сбора.
func TestCollectAlbum(t *testing.T) {
	e := newCollectEnv(t, 4)
	r := 3
	if _, err := e.a.editor().SetMarks([]model.PhotoInfo{{ID: library.IDFor(e.photos[0]), Path: e.photos[0]}}, model.MarkPatch{Rating: &r}); err != nil {
		t.Fatal(err)
	}
	v0 := e.a.editor().View()
	var opened []string
	oldOpen := openDir
	openDir = func(d string) error { opened = append(opened, d); return nil }
	t.Cleanup(func() { openDir = oldOpen })
	e.a.ShowCollected() // сбора ещё не было — нечего открывать
	dir, msg, note, stopped := e.run(t, false)
	if msg != "" || stopped || dir != e.book() || !strings.HasPrefix(note, "Проект собран: 4 файла") {
		t.Fatalf("итог: %q %q %q %v", dir, msg, note, stopped)
	}
	if e.progress == 0 {
		t.Error("нет collect:progress")
	}
	lp, err := salbum.Load(e.albumFile())
	if err != nil {
		t.Fatal(err)
	}
	if len(lp.Photos) != 4 || len(lp.Folders) != 1 || lp.Folders[0] != filepath.Join(dir, collect.DirPhotos) {
		t.Fatalf("собранный: %d фото, папки %v", len(lp.Photos), lp.Folders)
	}
	for _, ref := range lp.Photos {
		if rel, err := filepath.Rel(dir, ref.Path); err != nil || !filepath.IsLocal(rel) {
			t.Errorf("фото вне папки сбора: %s", ref.Path)
		} else if _, err := os.Stat(ref.Path); err != nil {
			t.Errorf("фото не скопировано: %v", err)
		}
	}
	if marks := lp.Loaded().Marks; len(marks) != 1 {
		t.Errorf("отметок в собранном %d, ждали 1", len(marks))
	}
	v := e.a.editor().View()
	if v.Rev != v0.Rev || v.CanUndo != v0.CanUndo || e.a.editor().Path() != e.file {
		t.Fatalf("открытый альбом изменён: rev %d→%d, путь %q", v0.Rev, v.Rev, e.a.editor().Path())
	}
	if ref, _ := e.a.editor().Ref(library.IDFor(e.photos[0])); ref.Path != e.photos[0] {
		t.Fatalf("путь фото открытого альбома: %q", ref.Path)
	}
	if got := e.a.dialogDir("collect"); got != e.parent {
		t.Errorf("папка диалога %q, ждали %q", got, e.parent)
	}
	e.a.ShowCollected()
	if len(opened) != 1 || opened[0] != dir {
		t.Fatalf("ShowCollected: %v", opened)
	}
	if e.busy() {
		t.Fatal("флаг сбора не снят")
	}
}

// Сц. 6: повтор в ту же папку — вопрос о замене альбома; «нет» — ничего не тронуто, «да» —
// прежний в резервных копиях, файлы «уже на месте».
func TestCollectReplacesAlbumWithBackup(t *testing.T) {
	e := newCollectEnv(t, 2)
	e.a.backups = backup.New(filepath.Join(testkit.TempDir(t), "backups"))
	if err := os.MkdirAll(e.book(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := salbum.Write(model.New(), e.albumFile(), "", model.Side{}); err != nil {
		t.Fatal(err)
	}
	old, _ := os.ReadFile(e.albumFile())
	e.answer = false
	if ok, err := e.a.Collect(CollectOptions{}); ok || err != nil {
		t.Fatalf("«нет»: %v %v", ok, err)
	}
	if len(e.asked) != 1 || !strings.Contains(e.asked[0], "уже есть «Иванов.salbum»") || !strings.Contains(e.asked[0], "резервные копии") {
		t.Fatalf("вопрос: %q", e.asked)
	}
	if now, _ := os.ReadFile(e.albumFile()); string(now) != string(old) {
		t.Fatal("«нет» изменил альбом")
	}
	if _, err := os.Stat(filepath.Join(e.book(), collect.DirPhotos)); err == nil {
		t.Fatal("«нет» начал копирование")
	}
	e.answer = true
	if _, msg, note, _ := e.run(t, false); msg != "" || strings.Contains(note, "уже") {
		t.Fatalf("первый сбор: %q %q", msg, note)
	}
	list, err := e.a.backups.List(backup.Key(e.albumFile()))
	if err != nil || len(list) != 1 {
		t.Fatalf("резервные копии: %v %v", list, err)
	}
	if _, msg, note, _ := e.run(t, false); msg != "" || !strings.Contains(note, "(2 уже были в папке)") {
		t.Fatalf("повтор: %q %q", msg, note)
	}
}

// «Заменить?» обещал копию прежнего альбома: копия не сохранилась (папка копий — файл; копий
// нет вовсе) — альбом не записан, прежний цел, итог об этом; копии пишутся — замена проходит.
func TestCollectReplaceBackupFails(t *testing.T) {
	e := newCollectEnv(t, 2)
	if err := os.MkdirAll(e.book(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := salbum.Write(model.New(), e.albumFile(), "", model.Side{}); err != nil {
		t.Fatal(err)
	}
	old, _ := os.ReadFile(e.albumFile())
	rootFile := filepath.Join(testkit.TempDir(t), "backups")
	if err := os.WriteFile(rootFile, []byte("не папка"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, st := range map[string]*backup.Store{"папка копий — файл": backup.New(rootFile), "копий нет": nil} {
		e.a.backups = st
		if _, msg, note, stopped := e.run(t, false); msg != errNoBackup.Error() || note != "" || stopped {
			t.Fatalf("%s: %q %q %v", name, msg, note, stopped)
		}
		if now, _ := os.ReadFile(e.albumFile()); string(now) != string(old) {
			t.Fatalf("%s: альбом записан без резервной копии", name)
		}
	}
	e.a.backups = backup.New(filepath.Join(testkit.TempDir(t), "backups"))
	if _, msg, _, _ := e.run(t, false); msg != "" {
		t.Fatalf("копии пишутся: %q", msg)
	}
	if now, _ := os.ReadFile(e.albumFile()); string(now) == string(old) {
		t.Fatal("альбом не заменён")
	}
	if list, err := e.a.backups.List(backup.Key(e.albumFile())); err != nil || len(list) != 1 {
		t.Fatalf("резервные копии: %v %v", list, err)
	}
	if len(e.asked) != 3 {
		t.Fatalf("вопросов о замене %d, ждали 3", len(e.asked))
	}
}

// Сц. 11: сетевая папка — отказ до всего; флаг сбора снят.
func TestCollectRejectsUNC(t *testing.T) {
	e := newCollectEnv(t, 1)
	e.parent = `//server/share/x`
	if runtime.GOOS == "windows" {
		e.parent = `\\server\share\x`
	}
	if ok, err := e.a.Collect(CollectOptions{}); ok || err == nil || err.Error() != noUNC {
		t.Fatalf("UNC: %v %v", ok, err)
	}
	if e.busy() || len(e.asked) != 0 {
		t.Fatalf("флаг %v, вопросы %q", e.busy(), e.asked)
	}
}

// Сц. 11: выбрана папка открытого альбома с тем же именем — отказ, файл цел.
func TestCollectOwnAlbum(t *testing.T) {
	e := newCollectEnv(t, 1)
	e.file = filepath.Join(e.book(), "Иванов"+projectExt)
	p, _ := e.a.editor().SnapshotSide()
	e.a.ed.Store(mustEditor(t, p, e.file))
	e.parent = e.book()
	if ok, err := e.a.Collect(CollectOptions{}); ok || err == nil || !strings.Contains(err.Error(), "открытого альбома") {
		t.Fatalf("свой альбом: %v %v", ok, err)
	}
	if _, err := os.Stat(e.book()); err == nil {
		t.Fatal("папка сбора создана при отказе")
	}
}

// Файл открытого альбома под другим путём (жёсткая ссылка; так же subst, \\?\, 8.3) — тоже
// отказ до вопросов, файл цел, копирования нет.
func TestCollectOwnAlbumLink(t *testing.T) {
	e := newCollectEnv(t, 1)
	if err := salbum.Write(model.New(), e.file, "", model.Side{}); err != nil {
		t.Fatal(err)
	}
	old, _ := os.ReadFile(e.file)
	if err := os.MkdirAll(e.book(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(e.file, e.albumFile()); err != nil {
		t.Skipf("жёсткую ссылку не создать: %v", err)
	}
	if ok, err := e.a.Collect(CollectOptions{}); ok || err == nil || !strings.Contains(err.Error(), "открытого альбома") {
		t.Fatalf("тот же файл по ссылке: %v %v", ok, err)
	}
	if now, _ := os.ReadFile(e.file); string(now) != string(old) || len(e.asked) != 0 {
		t.Fatalf("файл изменён или был вопрос: %q", e.asked)
	}
	if _, err := os.Stat(filepath.Join(e.book(), collect.DirPhotos)); err == nil {
		t.Fatal("копирование началось")
	}
}

// Второй сбор, пока первый в диалоге папки, — отказ; отказ в диалоге снимает флаг.
func TestCollectBusy(t *testing.T) {
	e := newCollectEnv(t, 1)
	var second error
	e.onPick = func() { _, second = e.a.Collect(CollectOptions{}) }
	e.parent = ""
	if ok, err := e.a.Collect(CollectOptions{}); ok || err != nil {
		t.Fatalf("отмена диалога: %v %v", ok, err)
	}
	if second == nil || !strings.Contains(second.Error(), "уже идёт") {
		t.Fatalf("второй сбор: %v", second)
	}
	e.onPick, e.parent = nil, testkit.TempDir(t)
	if _, msg, _, _ := e.run(t, false); msg != "" {
		t.Fatalf("после отмены: %q", msg)
	}
}

// Сц. 2: пропавшее фото — вопрос до диалога папки; «нет» — диалога нет, «да» — сбор без него,
// в итоге строка о пропавших.
func TestCollectMissingAsk(t *testing.T) {
	e := newCollectEnv(t, 2)
	if err := os.Remove(e.photos[1]); err != nil {
		t.Fatal(err)
	}
	e.answer = false
	if ok, err := e.a.Collect(CollectOptions{}); ok || err != nil {
		t.Fatalf("«нет»: %v %v", ok, err)
	}
	if len(e.asked) != 1 || !strings.HasPrefix(e.asked[0], "1 фото не найдено") || e.picks != 0 {
		t.Fatalf("вопрос %q, диалогов %d", e.asked, e.picks)
	}
	e.answer = true
	_, msg, note, _ := e.run(t, false)
	if msg != "" || !strings.Contains(note, "Проект собран: 1 файл,") || !strings.Contains(note, "Не найдено фото: 1") {
		t.Fatalf("итог: %q %q", msg, note)
	}
	lp, err := salbum.Load(e.albumFile())
	if err != nil || len(lp.Photos) != 2 {
		t.Fatalf("собранный: %v %v", lp, err)
	}
}

// Сц. 8: «Остановить» — итог «остановлен», альбом не записан, флаг снят; повтор докопирует.
func TestCollectCancel(t *testing.T) {
	e := newCollectEnv(t, 3)
	e.CancelCollectIdle(t)
	e.onStep = e.a.CancelCollect // первый ход приходит до первого файла
	_, msg, note, stopped := e.run(t, false)
	if msg != "" || !stopped || !strings.HasPrefix(note, "Сбор остановлен: скопировано 0 из 3") {
		t.Fatalf("итог: %q %q %v", msg, note, stopped)
	}
	if _, err := os.Stat(e.albumFile()); err == nil {
		t.Fatal("альбом записан после остановки")
	}
	e.mu.Lock()
	e.onStep = nil
	e.mu.Unlock()
	if _, msg, note, stopped := e.run(t, false); msg != "" || stopped || !strings.HasPrefix(note, "Проект собран: 3 файла") {
		t.Fatalf("повтор: %q %q %v", msg, note, stopped)
	}
}

// CancelCollectIdle — «Остановить» без сбора ничего не делает и не падает.
func (e *collectEnv) CancelCollectIdle(t *testing.T) {
	t.Helper()
	e.a.CancelCollect()
	if e.busy() {
		t.Fatal("CancelCollect без сбора поставил флаг")
	}
}

// Сц. 12: черновик без имени — «Альбом»; пустой альбом — только файл альбома, без папок.
func TestCollectEmptyDraft(t *testing.T) {
	e := newCollectEnv(t, 0)
	e.a.ed.Store(mustEditor(t, model.New(), ""))
	dir, msg, note, _ := e.run(t, false)
	if msg != "" || filepath.Base(dir) != "Альбом" || !strings.HasPrefix(note, "Проект собран: 0 файлов") {
		t.Fatalf("итог: %q %q %q", dir, msg, note)
	}
	lp, err := salbum.Load(filepath.Join(dir, "Альбом"+projectExt))
	if err != nil || len(lp.Photos) != 0 || len(lp.Folders) != 0 {
		t.Fatalf("собранный: %v %v", lp, err)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Fatalf("в папке %d записей, ждали только альбом", len(ents))
	}
}

// CollectInfo: только фото альбома или с лентой папок (чужое вне папок не берётся); Scanning —
// только в режиме «Все»; пропавшие считаются.
func TestCollectInfo(t *testing.T) {
	e := newCollectEnv(t, 2)
	p, _ := e.a.editor().SnapshotSide()
	extra := testkit.JPEG(t, p.Folders[0], "лишнее.jpg", 60, 40)
	outside := testkit.JPEG(t, testkit.TempDir(t), "чужое.jpg", 60, 40)
	e.a.mu.Lock()
	e.a.photos = []library.Photo{{ID: library.IDFor(e.photos[0]), Path: e.photos[0]}, {ID: "x", Path: extra}, {ID: "y", Path: outside}}
	e.a.scan.Active = true
	e.a.mu.Unlock()
	if in := e.a.CollectInfo(false); in.Photos != 2 || in.Extra != 0 || in.Scanning || in.Bytes <= 0 {
		t.Fatalf("только альбом: %+v", in)
	}
	in := e.a.CollectInfo(true)
	if in.Photos != 2 || in.Extra != 1 || !in.Scanning {
		t.Fatalf("все фото: %+v", in)
	}
	if err := os.Remove(e.photos[0]); err != nil {
		t.Fatal(err)
	}
	if in := e.a.CollectInfo(false); in.Photos != 1 || in.Missing != 1 || in.FolderName != "Иванов" {
		t.Fatalf("пропавшее: %+v", in)
	}
	// Имя папки — то же, что создаст сбор: у черновика «Альбом».
	e.a.ed.Store(mustEditor(t, model.New(), ""))
	if in := e.a.CollectInfo(false); in.FolderName != "Альбом" || in.Photos != 0 {
		t.Fatalf("черновик: %+v", in)
	}
}

// Папка сбора — одна из папок альбома или внутри неё: отказ до вопросов и копирования, флаг
// снят; соседняя папка с тем же началом имени — не внутри.
func TestCollectInsidePhotoFolder(t *testing.T) {
	e := newCollectEnv(t, 2)
	p, _ := e.a.editor().SnapshotSide()
	sub := filepath.Join(p.Folders[1], "сбор")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, parent := range []string{p.Folders[0], sub} {
		e.parent = parent
		ok, err := e.a.Collect(CollectOptions{})
		if ok || err == nil || err.Error() != inFolders || strings.Contains(err.Error(), parent) {
			t.Fatalf("%s: %v %v", parent, ok, err)
		}
		if e.busy() || len(e.asked) != 0 {
			t.Fatalf("%s: флаг %v, вопросы %q", parent, e.busy(), e.asked)
		}
		if _, err := os.Stat(filepath.Join(parent, "Иванов")); err == nil {
			t.Fatalf("%s: папка сбора создана при отказе", parent)
		}
	}
	// Сама папка фото — внутри; сосед «…2» и родитель — нет.
	if !inPhotoFolder(p.Folders[0], p.Folders) || inPhotoFolder(p.Folders[0]+"2", p.Folders) || inPhotoFolder(filepath.Dir(p.Folders[0]), p.Folders) {
		t.Fatal("inPhotoFolder: совпадение, сосед с тем же началом, родитель")
	}
	if inPhotoFolder(p.Folders[0], nil) {
		t.Fatal("inPhotoFolder без папок альбома")
	}
}

// Новый сбор, начатый по collect:done (окно снова даёт «Собрать…»), не теряет флаг, когда
// прежний runCollect доходит до своих defer: второй вызов в это время ещё в диалоге папки.
func TestCollectFlagAfterDone(t *testing.T) {
	e := newCollectEnv(t, 1)
	entered, release, second := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	e.onDone = func() {
		e.mu.Lock()
		e.onDone = nil
		e.onPick = func() { close(entered); <-release }
		e.mu.Unlock()
		go func() {
			_, err := e.a.Collect(CollectOptions{})
			second <- err
		}()
		<-entered
	}
	if _, msg, _, _ := e.run(t, false); msg != "" {
		t.Fatalf("первый сбор: %q", msg)
	}
	e.a.bg.Wait() // первый runCollect вышел со всеми defer
	if !e.busy() {
		t.Fatal("флаг второго сбора снят defer первого")
	}
	if _, err := e.a.Collect(CollectOptions{}); err == nil {
		t.Fatal("третий сбор прошёл, пока второй в диалоге")
	}
	e.parent = ""
	close(release)
	if err := <-second; err != nil {
		t.Fatalf("второй сбор: %v", err)
	}
	if e.busy() {
		t.Fatal("флаг не снят после отмены диалога")
	}
}

// Тексты итога: остановка, нехватка места, ошибка копирования и записи, «уже были», XMP.
func TestCollectOutcome(t *testing.T) {
	pl := collect.Plan{Items: []collect.Item{{Size: 1 << 20}, {Size: 1 << 20, Skip: true}, {Size: 1 << 20}}, Missing: []string{"m"}, SideBusy: 2}
	if msg, note, st := collectOutcome(pl, 1, fmt.Errorf("x: %w", context.Canceled), true); msg != "" || !st || !strings.Contains(note, "скопировано 1 из 2") {
		t.Errorf("остановка: %q %q %v", msg, note, st)
	}
	if msg, _, st := collectOutcome(pl, 1, collect.ErrDiskFull, true); st || msg != "Диск заполнен: скопировано 1 из 2. Альбом в папку не записан." {
		t.Errorf("место: %q", msg)
	}
	if msg, _, _ := collectOutcome(pl, 0, errors.New("сбой"), true); msg != "сбой. Альбом в папку не записан." {
		t.Errorf("копирование: %q", msg)
	}
	if msg, _, _ := collectOutcome(pl, 2, errors.New("сбой"), false); msg != "сбой" {
		t.Errorf("запись: %q", msg)
	}
	_, note, _ := collectOutcome(pl, 2, nil, false)
	want := "Проект собран: 3 файла, 3,0 МБ (1 уже был в папке). Не найдено фото: 1 — в альбоме остались со старым путём. Не скопировано XMP: 2 — имя в папке занято другим файлом."
	if note != want {
		t.Errorf("успех:\n%q\n%q", note, want)
	}
}

func TestMissingQuestion(t *testing.T) {
	for _, c := range []struct {
		photos, guides int
		want           string
	}{
		{0, 0, ""},
		{1, 0, "1 фото не найдено"},
		{0, 1, "1 разметка типографии не найдена"},
		{4, 0, "4 фото не найдены"},
		{0, 2, "2 разметки типографии не найдены"},
		{1, 1, "1 фото и 1 разметка типографии не найдены"},
	} {
		got := missingQuestion(c.photos, c.guides)
		if c.want == "" && got != "" || !strings.HasPrefix(got, c.want) {
			t.Errorf("%d, %d: %q", c.photos, c.guides, got)
		}
	}
}

func TestPlural(t *testing.T) {
	for n, want := range map[int]string{0: "файлов", 1: "файл", 2: "файла", 4: "файла", 5: "файлов", 11: "файлов", 14: "файлов", 21: "файл", 22: "файла", 111: "файлов", 312: "файлов", -1: "файл"} {
		if got := plural(n, "файл", "файла", "файлов"); got != want {
			t.Errorf("%d: %q, ждали %q", n, got, want)
		}
	}
}
