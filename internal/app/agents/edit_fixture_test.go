package agents

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Подставные замыкания правок (149): по умолчанию безобидны, тесты подменяют нужные до старта службы.

// fakeClock — часы службы в тестах: счёт отказов и темп считают по ним, а не по настоящему времени.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *fakeClock { return &fakeClock{t: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// editFake — Host правок над fixture. Счётчики n* — сколько раз служба дошла до замыкания.
type editFake struct {
	f      *fixture
	fake34 // правки 0.34 (edit_run34_test.go)

	mu        sync.Mutex
	allowed   bool
	blocked   string
	perms     map[string]string // явные режимы инструментов (perm)
	scanning  bool
	lastAuto  AutoReq
	lastVary  VaryReq
	lastAsk   AskReq
	lastShow  string
	lastUndo  AlbumRef
	backups   []string
	lastSheet SheetReq

	busy   func(ctx context.Context) bool
	backup func(key string) error
	ask    func(ctx context.Context, q AskReq) (AskRes, error)
	auto   func(ctx context.Context, in AutoReq) (AutoRes, error)
	vary   func(ctx context.Context, in VaryReq) (AlbumRef, int, error)
	show   func(key, id string) (int, error)
	undo   func(ctx context.Context, ref AlbumRef) (AlbumRef, error)
	sheet  func(ctx context.Context, in SheetReq) ([]byte, []string, error)

	nPerm   atomic.Int32                            // вызовов Host.Perm
	permLvl func(tool string) (string, string, int) // Host.PermLevel; nil — не задан
	nSheet  atomic.Int32                            // вызовов Host.Sheet (чтение; в writes не входит)

	nBusy, nBackup, nAsk, nAuto, nVary, nShow, nUndo atomic.Int32
}

func newEditFake(f *fixture) *editFake { return &editFake{f: f, allowed: true} }

// writes — сколько раз служба дошла до замыканий, которые что-то меняют.
func (e *editFake) writes() int32 {
	return e.nBackup.Load() + e.nAsk.Load() + e.nAuto.Load() + e.nVary.Load() + e.nShow.Load() + e.nUndo.Load() + e.writes34()
}

// perm — Host.Perm над подставными настройками: явный режим инструмента (setPerm), иначе чтение, запуск и
// показ разрешены, а правки, папка и альбом — по allowed («флажок» прежних тестов).
func (e *editFake) perm(tool string) (string, string) {
	e.nPerm.Add(1)
	e.mu.Lock()
	defer e.mu.Unlock()
	if m, ok := e.perms[tool]; ok {
		return m, e.blocked
	}
	switch prefs.AgentToolClass(tool) {
	case prefs.ClassEdit, prefs.ClassFolder, prefs.ClassAlbum:
		if !e.allowed {
			return prefs.PermDeny, e.blocked
		}
	}
	return prefs.PermAllow, e.blocked
}

// setPerm ставит режим одному инструменту; потом PermsChanged службы, если вопрос мог висеть.
func (e *editFake) setPerm(tool, mode string) {
	e.mu.Lock()
	if e.perms == nil {
		e.perms = map[string]string{}
	}
	e.perms[tool] = mode
	e.mu.Unlock()
}

func (e *editFake) setFlag(allowed bool, blocked string) {
	e.mu.Lock()
	e.allowed, e.blocked = allowed, blocked
	e.mu.Unlock()
}

// ref — ключ и ревизия открытого альбома сейчас.
func (f *fixture) ref() AlbumRef {
	f.mu.Lock()
	defer f.mu.Unlock()
	return AlbumRef{Key: albumKey(f.ed), Rev: f.view.Rev}
}

// bump — «правка»: ревизия растёт; возвращает новые ключ и ревизию.
func (f *fixture) bump() AlbumRef {
	f.mu.Lock()
	f.view.Rev++
	f.mu.Unlock()
	return f.ref()
}

func (f *fixture) refArgs() map[string]any {
	r := f.ref()
	return map[string]any{"key": r.Key, "rev": r.Rev}
}

// install заполняет поля правок в Host.
func (e *editFake) install(h *Host) {
	h.Perm = e.perm
	h.PermLevel = func(tool string) (string, string, int) { // setup тестов выполняется после install: поле читаем в вызове
		if e.permLvl != nil {
			return e.permLvl(tool)
		}
		m, b := e.perm(tool)
		return m, b, -1 // уровень службе читать самой
	}
	h.Scanning = func() bool {
		e.mu.Lock()
		defer e.mu.Unlock()
		return e.scanning
	}
	h.Busy = func(ctx context.Context) bool {
		e.nBusy.Add(1)
		if e.busy != nil {
			return e.busy(ctx)
		}
		return false
	}
	h.Backup = func(key string) error {
		e.nBackup.Add(1)
		e.mu.Lock()
		e.backups = append(e.backups, key)
		e.mu.Unlock()
		if e.backup != nil {
			return e.backup(key)
		}
		return nil
	}
	h.Ask = func(ctx context.Context, q AskReq) (AskRes, error) {
		e.nAsk.Add(1)
		e.mu.Lock()
		e.lastAsk = q
		e.mu.Unlock()
		if e.ask != nil {
			return e.ask(ctx, q)
		}
		if q.Kind == AskEdit { // «да» на вопрос о правке сам альбом не меняет: правит тело вызова
			return AskRes{Why: WhyAnswered}, nil
		}
		return AskRes{Why: WhyAnswered, Album: e.f.bump(), Folder: 1, Scanning: true}, nil
	}
	h.Auto = func(ctx context.Context, in AutoReq) (AutoRes, error) {
		e.nAuto.Add(1)
		e.mu.Lock()
		e.lastAuto = in
		e.mu.Unlock()
		if e.auto != nil {
			return e.auto(ctx, in)
		}
		return AutoRes{Album: e.f.bump(), Added: 3, Photos: 12, PlanFrom: 2, PlanTo: 4, First: 3}, nil
	}
	h.Vary = func(ctx context.Context, in VaryReq) (AlbumRef, int, error) {
		e.nVary.Add(1)
		e.mu.Lock()
		e.lastVary = in
		e.mu.Unlock()
		if e.vary != nil {
			return e.vary(ctx, in)
		}
		return e.f.bump(), len(in.Spreads), nil
	}
	h.Show = func(key, id string) (int, error) {
		e.nShow.Add(1)
		e.mu.Lock()
		e.lastShow = id
		e.mu.Unlock()
		if e.show != nil {
			return e.show(key, id)
		}
		if id == model.CoverID {
			return 0, nil
		}
		return 1, nil
	}
	h.Undo = func(ctx context.Context, ref AlbumRef) (AlbumRef, error) {
		e.nUndo.Add(1)
		e.mu.Lock()
		e.lastUndo = ref
		e.mu.Unlock()
		if e.undo != nil {
			return e.undo(ctx, ref)
		}
		if cur := e.f.ref(); cur.Rev != ref.Rev {
			return AlbumRef{}, ErrAlbumEdited
		}
		return e.f.bump(), nil
	}
	e.install34(h)
}

// editRig — запущенная служба с Host правок; setup подменяет замыкания до старта.
func editRig(t *testing.T, o Options, setup func(*editFake)) (*rig, *mcp.ClientSession) {
	t.Helper()
	r := newRig(t, o, "")
	setLevel(r.cfg, 1)
	if setup != nil {
		setup(r.fx.edit)
	}
	r.start(t)
	return r, client(t, r.sock, "Claude Code")
}

// failText — ответ-ошибка с точным текстом из списка ошибок службы.
func failText(t *testing.T, r reply, want *agentErr) {
	t.Helper()
	if !r.IsError || r.Text != want.Error() {
		t.Fatalf("ждали ошибку %q, получили isError=%v %q", want.Error(), r.IsError, r.Text)
	}
}

// okReply — вызов прошёл без ошибки.
func okReply(t *testing.T, r reply) reply {
	t.Helper()
	if r.IsError {
		t.Fatalf("неожиданная ошибка: %q", r.Text)
	}
	return r
}

// Чтение записанного под замком: служба пишет из своих горутин (-race).
func (e *editFake) gotAuto() AutoReq  { e.mu.Lock(); defer e.mu.Unlock(); return e.lastAuto }
func (e *editFake) gotVary() VaryReq  { e.mu.Lock(); defer e.mu.Unlock(); return e.lastVary }
func (e *editFake) gotAsk() AskReq    { e.mu.Lock(); defer e.mu.Unlock(); return e.lastAsk }
func (e *editFake) gotShow() string   { e.mu.Lock(); defer e.mu.Unlock(); return e.lastShow }
func (e *editFake) gotUndo() AlbumRef { e.mu.Lock(); defer e.mu.Unlock(); return e.lastUndo }
func (e *editFake) gotBackups() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.backups...)
}
