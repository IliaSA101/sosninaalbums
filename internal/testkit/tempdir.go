package testkit

import (
	"os"
	"regexp"
	"runtime/debug"
	"sync"
	"testing"
	"time"
)

// removeTimeout — сколько TempDir ждёт, пока Windows отпустит файлы удаляемой папки.
const removeTimeout = 5 * time.Second

// hooks — действия «до удаления папок» одного теста (см. BeforeRemove).
type hooks struct {
	once sync.Once
	fns  []func()
}

// run выполняет действия в обратном порядке регистрации, один раз. Паника действия не валит
// весь бинарник без имени теста и стека (так терялся след редкой паники closeBackground): она
// становится ошибкой теста с именем и полным стеком места паники, остальные действия идут дальше.
func (h *hooks) run(t testing.TB) {
	h.once.Do(func() {
		for i := len(h.fns) - 1; i >= 0; i-- {
			runHook(t, h.fns[i])
		}
	})
}

// runHook вызывает fn; паника — t.Errorf с именем теста и стеком.
func runHook(t testing.TB, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("паника в действии BeforeRemove теста %s: %v\n%s", t.Name(), r, debug.Stack())
		}
	}()
	fn()
}

var (
	hooksMu sync.Mutex
	byTest  = map[testing.TB]*hooks{}
)

// hooksOf возвращает действия теста; первое обращение регистрирует их запуск в Cleanup
// (на случай, если у теста нет ни одной папки TempDir).
func hooksOf(t testing.TB) *hooks {
	hooksMu.Lock()
	defer hooksMu.Unlock()
	h, ok := byTest[t]
	if !ok {
		h = &hooks{}
		byTest[t] = h
		t.Cleanup(func() {
			h.run(t)
			hooksMu.Lock()
			delete(byTest, t)
			hooksMu.Unlock()
		})
	}
	return h
}

// BeforeRemove откладывает fn до очистки теста, но раньше, чем любая папка из TempDir этого
// теста будет удалена: останавливать фоновую работу (сканирование, чистку кэша, автосохранение)
// нужно до удаления папок, а t.Cleanup идёт в обратном порядке и папки, созданные в теле теста
// после NewApp, иначе удалились бы раньше. Несколько fn — в обратном порядке регистрации.
func BeforeRemove(t testing.TB, fn func()) {
	t.Helper()
	h := hooksOf(t)
	hooksMu.Lock()
	h.fns = append(h.fns, fn)
	hooksMu.Unlock()
}

var badName = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

// TempDir — замена t.TempDir(): та же временная папка (os.TempDir, под go test это GOTMPDIR),
// но удаляется с повтором.
//
// Зачем повтор: на Windows удаление файла, который кто-то ещё держит открытым (антивирус,
// индексатор, проверка только что записанного файла), проходит, но файл остаётся в состоянии
// «ожидает удаления» до закрытия чужого дескриптора, и RemoveAll родительской папки падает с
// «The directory is not empty». Это видно и в тесте без единой горутины (TestTemplatesRoundTrip:
// синхронная запись templates.json и удаление, ≈ 1 раз на 300), а повтор через ~5 мс проходит.
// t.TempDir так не умеет и валит тест на очистке. Фоновые горутины программы остановите через
// BeforeRemove — повтором их не лечить: они могут создавать файлы заново.
func TempDir(t testing.TB) string {
	t.Helper()
	name := badName.ReplaceAllString(t.Name(), "_")
	if len(name) > 40 {
		name = name[:40]
	}
	dir, err := os.MkdirTemp("", name+"*")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	h := hooksOf(t)
	t.Cleanup(func() {
		h.run(t)
		if err := removeAll(dir, removeTimeout); err != nil {
			t.Errorf("TempDir RemoveAll cleanup: %v", err)
		}
	})
	return dir
}

// RemoveAll удаляет path внутри теста с повтором до removeTimeout и валит тест при неудаче.
// Повтор по той же причине, что у TempDir: на Windows антивирус или индексатор держат только что
// записанные файлы, и голый os.RemoveAll изредка падает «directory is not empty».
func RemoveAll(t testing.TB, path string) {
	t.Helper()
	if err := removeAll(path, removeTimeout); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
}

// removeAll повторяет os.RemoveAll с паузой, пока не выйдет или не истечёт timeout.
func removeAll(dir string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		err := os.RemoveAll(dir)
		if err == nil || time.Now().After(deadline) {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}
}
