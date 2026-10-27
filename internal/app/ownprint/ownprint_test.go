package ownprint

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Сквозные тесты типографий (окно, файл, реестр, события в App) остаются в internal/app (0.26-126-15):
// они стоят на опоре App. Здесь — пустые поля Host, круг «сохранить → перезапуск», края файла,
// события и вызовы Host службой без Wails.

// rig — служба с фальшивым Host: хранилище во временной папке, записанные события и вызовы.
type rig struct {
	t       *testing.T
	dir     string
	store   *decor.Store
	mu      sync.Mutex
	events  []string
	views   int // сколько раз вид ушёл в Changed (правка альбома)
	outs    int // сколько раз — в Out («без правки»)
	applied []string
	open    string // что вернёт диалог открытия
	save    string // что вернёт диалог сохранения
	dirs    []string
}

func (r *rig) host() Host {
	return Host{
		Store:    func() (*decor.Store, error) { return r.store, nil },
		Emit:     func(name string, _ ...any) { r.mu.Lock(); r.events = append(r.events, name); r.mu.Unlock() },
		Version:  func() string { return "0.27.0" },
		Snapshot: func() *model.Project { return model.New() },
		View:     func() model.View { return model.View{} },
		Changed:  func(v model.View) model.View { r.views++; return v },
		Out:      func(v model.View) model.View { r.outs++; return v },
		SetSettings: func(model.Settings) model.View {
			r.applied = append(r.applied, "settings")
			return model.View{}
		},
		ApplyPrint: func(_ model.Settings, preset string) (model.View, error) {
			r.applied = append(r.applied, preset)
			return model.View{}, nil
		},
		PickOpen:    func() (string, error) { return r.open, nil },
		PickSave:    func(string) (string, error) { return r.save, nil },
		RememberDir: func(d string) { r.dirs = append(r.dirs, d) },
	}
}

func newRig(t *testing.T) (*rig, *Service) {
	t.Helper()
	printprof.UseForTest(t, nil) // реестр общий на процесс: к концу теста прежний
	dir := testkit.TempDir(t)
	st := decor.Open(dir)
	st.SetPrintCheck(CheckData)
	r := &rig{t: t, dir: dir, store: st}
	s := New(r.host())
	s.Reload()
	return r, s
}

func (r *rig) count(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e == name {
			n++
		}
	}
	return n
}

func ownIDs() []string {
	var ids []string
	for _, in := range printprof.Current().Infos() {
		if in.Source == printprof.SourceOwn {
			ids = append(ids, in.ID)
		}
	}
	return ids
}

// draft — заготовка на основе Фабрики с названием name.
func draft(t *testing.T, s *Service, name string) Edit {
	t.Helper()
	e, err := s.Draft("fabrika")
	if err != nil {
		t.Fatal(err)
	}
	e.Name = name
	return e
}

// Пустое поле Host — паника New с именем поля (шаблон служб: testkit.NewPanics).
func TestOwnprintNewPanics(t *testing.T) {
	r := &rig{}
	testkit.NewPanics(t, r.host(), func(h Host) { New(h) }, "ownprint.New")
}

// Круг: Save → файл в хранилище → «перезапуск» (новая служба, Reload) видит ту же типографию.
// Несколько объектов сразу; удаление одной из трёх не трогает остальные.
func TestSaveReloadCycleMany(t *testing.T) {
	r, s := newRig(t)
	if len(ownIDs()) != 0 {
		t.Fatalf("в пустой библиотеке свои типографии: %v", ownIDs())
	}
	var ids []string
	for _, name := range []string{"Альфа", "Бета", "Гамма"} {
		res, err := s.Save(draft(t, s, name), false)
		if err != nil || res.ID == "" {
			t.Fatalf("сохранение %q: %v", name, err)
		}
		ids = append(ids, res.ID)
	}
	if got := r.count("print:changed"); got != 3 {
		t.Errorf("событий print:changed %d, ждали 3", got)
	}
	if len(r.applied) != 0 {
		t.Errorf("Save без apply правил альбом: %v", r.applied)
	}
	before := printprof.Current().Infos()

	// Перезапуск: реестр сброшен, служба новая, хранилище то же.
	printprof.Install(printprof.Builtin())
	s2 := New(r.host())
	s2.Reload()
	if got := ownIDs(); len(got) != 3 {
		t.Fatalf("после перезапуска свои %v, ждали 3", got)
	}
	if after := printprof.Current().Infos(); len(after) != len(before) {
		t.Errorf("список типографий: %d → %d", len(before), len(after))
	}
	if s2.OwnKey(ids[1]) == "" || !s2.Taken(ids[0]) {
		t.Error("ключ или занятость своей не восстановлены")
	}

	outs := r.outs
	if _, err := s2.Delete(ids[1]); err != nil {
		t.Fatal(err)
	}
	if got := ownIDs(); len(got) != 2 {
		t.Errorf("после удаления средней своих %d, ждали 2", len(got))
	}
	if _, err := s2.Delete(ids[1]); !errors.Is(err, ErrNotOwn) {
		t.Errorf("повторное удаление: %v", err)
	}
	if r.outs != outs+1 {
		t.Errorf("вид после удаления ушёл в Out %d раз, ждали 1", r.outs-outs)
	}
}

// Save с apply ставит альбому первое изделие одной правкой; отказ применения не стирает запись и
// всё равно сообщает окну (print:changed).
func TestSaveApplyAndFailure(t *testing.T) {
	r, s := newRig(t)
	res, err := s.Save(draft(t, s, "С применением"), true)
	if err != nil || len(r.applied) != 1 || !strings.HasPrefix(r.applied[0], res.ID+"-") || r.views != 1 {
		t.Fatalf("apply: %v, правки %v, видов %d", err, r.applied, r.views)
	}
	h := r.host()
	fail := errors.New("не применилось")
	h.ApplyPrint = func(model.Settings, string) (model.View, error) { return model.View{}, fail }
	s = New(h)
	s.Reload()
	before := r.count("print:changed")
	if _, err := s.Save(draft(t, s, "Отказ применения"), true); !errors.Is(err, fail) {
		t.Fatalf("отказ применения: %v", err)
	}
	if len(ownIDs()) != 2 || r.count("print:changed") != before+1 {
		t.Errorf("запись должна остаться, окно — получить событие: своих %d, событий %d→%d", len(ownIDs()), before, r.count("print:changed"))
	}
}

// Apply: неизвестная типография — отказ без правки альбома; встроенная — одна правка.
func TestApplyUnknownAndBuiltin(t *testing.T) {
	r, s := newRig(t)
	for _, id := range []string{"", "нет-такой"} {
		if _, err := s.Apply(id); !errors.Is(err, ErrRejected) {
			t.Errorf("Apply(%q): %v", id, err)
		}
	}
	if len(r.applied) != 0 || r.views != 0 {
		t.Errorf("отказ тронул альбом: %v, видов %d", r.applied, r.views)
	}
	if _, err := s.Apply("fabrika"); err != nil || len(r.applied) != 1 || r.views != 1 {
		t.Errorf("встроенная: %v %v %d", err, r.applied, r.views)
	}
}

// Заготовка: пустая основа — срез форматов не nil; неизвестная — ошибка; название в ошибке урезано.
func TestDraftEdges(t *testing.T) {
	_, s := newRig(t)
	e, err := s.Draft("")
	if err != nil || e.Formats == nil || len(e.Formats) != 0 || e.DPI != model.PrintDPI || e.Spreads != 1 {
		t.Errorf("пустая заготовка: %+v %v", e, err)
	}
	long := strings.Repeat("я", 200)
	if _, err := s.Draft(long); err == nil || len([]rune(err.Error())) > 80 {
		t.Errorf("неизвестная основа: %v", err)
	}
	c := s.Check(Edit{})
	if c.OK || c.Errs["name"] == "" || c.Formats == nil || c.Lines == nil {
		t.Errorf("расчёт пустых полей: %+v", c)
	}
}

// Диалог файла: отказ — Canceled без события и без запоминания папки; ошибка диалога идёт наверх.
func TestImportFileDialog(t *testing.T) {
	r, s := newRig(t)
	res, err := s.ImportFile()
	if err != nil || !res.Canceled || res.Lines == nil || len(r.dirs) != 0 || r.count("print:changed") != 0 {
		t.Errorf("отказ от диалога: %+v %v %v", res, err, r.dirs)
	}
	bad := filepath.Join(r.dir, "x.txt")
	r.open = bad
	if _, err := s.ImportFile(); !errors.Is(err, ErrFile) || len(r.dirs) != 0 {
		t.Errorf("не .saprint: %v, папки %v", err, r.dirs)
	}
}

// Битые, пустые, усечённые и огромные файлы — ошибка, а не паника; реестр и события не тронуты.
func TestImportPathHostileFiles(t *testing.T) {
	r, s := newRig(t)
	good, err := s.Save(draft(t, s, "Образец"), false)
	if err != nil {
		t.Fatal(err)
	}
	r.save = filepath.Join(r.dir, "образец.saprint")
	if _, err := s.ShareFile(good.ID); err != nil {
		t.Fatal(err)
	}
	enc, err := os.ReadFile(r.save)
	if err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) string {
		p := filepath.Join(r.dir, name)
		if err := os.WriteFile(p, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	cases := map[string]string{
		"пустой":     write("empty.saprint", nil),
		"мусор":      write("junk.saprint", []byte("не json")),
		"усечённый":  write("cut.saprint", enc[:len(enc)/2]),
		"огромный":   write("big.saprint", make([]byte, printprof.MaxSize+1)),
		"нет файла":  filepath.Join(r.dir, "нет.saprint"),
		"папка":      r.dir,
		"расширение": write("a.txt", enc),
	}
	if err := os.Mkdir(filepath.Join(r.dir, "dir.saprint"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases["папка .saprint"] = filepath.Join(r.dir, "dir.saprint")
	before, events := len(ownIDs()), r.count("print:changed")
	for name, p := range cases {
		if _, err := s.ImportPath(p); err == nil {
			t.Errorf("%s: принят", name)
		}
	}
	if len(ownIDs()) != before || r.count("print:changed") != events {
		t.Errorf("отказы тронули реестр: %d→%d, события %d→%d", before, len(ownIDs()), events, r.count("print:changed"))
	}
	// Тот же файл целиком: «такая уже есть» без записи и без события.
	if res, err := s.ImportPath(r.save); err != nil || res.Ask != "same" || r.count("print:changed") != events {
		t.Errorf("повтор: %+v %v", res, err)
	}
}

// Файл «Поделиться»: круг Share → Import на другой службе; отказ от диалога и чужая типография.
func TestShareRoundTrip(t *testing.T) {
	r, s := newRig(t)
	res, err := s.Save(draft(t, s, "Для друга"), false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ShareFile("fabrika"); !errors.Is(err, ErrNotOwn) {
		t.Errorf("встроенная: %v", err)
	}
	sh, err := s.ShareFile(res.ID)
	if err != nil || !sh.Canceled || sh.Name != "" {
		t.Errorf("отказ от диалога: %+v %v", sh, err)
	}
	r.save = filepath.Join(r.dir, "Для друга") // без расширения: служба допишет
	sh, err = s.ShareFile(res.ID)
	if err != nil || sh.Name != "Для друга"+Ext || len(r.dirs) != 1 {
		t.Fatalf("запись: %+v %v %v", sh, err, r.dirs)
	}
	if _, err := s.ShareFile(res.ID); !errors.Is(err, ErrExists) {
		t.Errorf("файл с дописанным расширением уже есть: %v", err)
	}

	// Друг: пустая библиотека, тот же файл.
	r2, s2 := newRig(t)
	out, err := s2.ImportPath(filepath.Join(r.dir, sh.Name))
	if err != nil || out.Ask != "" || out.Info.ID != res.ID || out.Copied || r2.count("print:changed") != 1 {
		t.Fatalf("импорт у друга: %+v %v", out, err)
	}
	if len(out.Lines) == 0 || !strings.HasPrefix(out.Lines[0], "Форматы: ") {
		t.Errorf("сводка: %v", out.Lines)
	}
}

// Вопрос «Заменить?»: без вопроса, чужой токен, просрочка и снятие — ErrStale; верный токен — замена.
func TestConfirmImportStale(t *testing.T) {
	r, s := newRig(t)
	if _, err := s.ConfirmImport("токен"); !errors.Is(err, ErrStale) {
		t.Errorf("без вопроса: %v", err)
	}
	res, err := s.Save(draft(t, s, "Исходная"), false)
	if err != nil {
		t.Fatal(err)
	}
	r.save = filepath.Join(r.dir, "i.saprint")
	if _, err := s.ShareFile(res.ID); err != nil {
		t.Fatal(err)
	}
	p, _ := printprof.Current().ByID(res.ID)
	p.Version++
	data, err := printprof.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(r.save, data, 0o600); err != nil {
		t.Fatal(err)
	}
	ask := func() string {
		t.Helper()
		q, err := s.ImportPath(r.save)
		if err != nil || q.Ask != "replace" || q.Token == "" || q.Question == "" {
			t.Fatalf("вопрос: %+v %v", q, err)
		}
		return q.Token
	}
	tok := ask()
	if _, err := s.ConfirmImport(tok + "0"); !errors.Is(err, ErrStale) {
		t.Errorf("чужой токен: %v", err)
	}
	tok = ask()
	s.ExpirePending()
	if _, err := s.ConfirmImport(tok); !errors.Is(err, ErrStale) {
		t.Errorf("просрочен: %v", err)
	}
	tok = ask()
	s.DropPending()
	if _, err := s.ConfirmImport(tok); !errors.Is(err, ErrStale) {
		t.Errorf("снят: %v", err)
	}
	before := r.count("print:changed")
	tok = ask()
	if _, err := s.ConfirmImport(tok); err != nil || r.count("print:changed") != before+1 {
		t.Errorf("замена: %v", err)
	}
	if cur, _ := printprof.Current().ByID(res.ID); cur.Version != p.Version {
		t.Errorf("версия после замены %d, ждали %d", cur.Version, p.Version)
	}
}

// Библиотечный слой: LibraryChanged шлёт событие, только если список типографий сменился.
func TestLibraryChangedOnlyOnDiff(t *testing.T) {
	r, s := newRig(t)
	s.LibraryChanged()
	if r.count("print:changed") != 0 || r.outs != 0 {
		t.Errorf("список не менялся, а событие ушло: %d, Out %d", r.count("print:changed"), r.outs)
	}
	if _, err := s.Save(draft(t, s, "Своя"), false); err != nil {
		t.Fatal(err)
	}
	// Файл в хранилище появился мимо службы (как после установки набора): Reload его подхватит.
	p, errs := Build(draft(t, s, "Со стороны"), "own-abcd0001", printprof.Current(), 1)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	data, err := printprof.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.store.SavePrint("", p.Name, data); err != nil {
		t.Fatal(err)
	}
	before := r.count("print:changed")
	s.LibraryChanged()
	if r.count("print:changed") != before+1 || len(ownIDs()) != 2 {
		t.Errorf("новый список: событий %d→%d, своих %d", before, r.count("print:changed"), len(ownIDs()))
	}
}

// Адрес окна: края входа — пустой, не https, длиннее предела, не UTF-8 — без паники, как есть.
func TestIDNURLEdges(t *testing.T) {
	for name, in := range map[string]string{
		"пустой":     "",
		"пробелы":    "   ",
		"не https":   "http://фабрика.рф/",
		"без хоста":  "https://",
		"не UTF-8":   "https://\xff\xfe.рф/",
		"длинный":    "https://" + strings.Repeat("ж", 400) + ".рф/",
		"IP-литерал": "https://[::1]/",
		"с именем":   "https://u@фабрика.рф/",
	} {
		out, why := IDNURL(in)
		if strings.TrimSpace(in) != out && why == "" {
			t.Errorf("%s: %q → %q без пояснения", name, in, out)
		}
		_ = IDNShow(in)
	}
	if out, why := IDNURL("https://фабрика.рф/a"); why != "" || out != "https://xn--80aac2ak8aza.xn--p1ai/a" {
		t.Errorf("перевод: %q %q", out, why)
	}
}
