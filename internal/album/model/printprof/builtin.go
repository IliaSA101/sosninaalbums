package printprof

import (
	"embed"
	"fmt"
	"io/fs"
	"sync"
	"sync/atomic"
)

//go:embed builtin/*.json
var builtinFS embed.FS

// builtinFiles — встроенные профили в порядке рядов окна «Типография» (058): Фотосфера, Нетпринт,
// Фабрика. Порядок файлов в embed — по алфавиту, поэтому он задан здесь; TestBuiltinParse сверяет
// список с папкой builtin.
var builtinFiles = []string{"fsfera", "netprint", "fabrika"}

var (
	builtinOnce sync.Once
	builtinSnap *Snapshot
	builtinTest atomic.Pointer[Snapshot] // подмена встроенных в тесте (UseBuiltinForTest)
)

// Builtin — снимок из одних встроенных профилей. Разбор встроенных файлов — часть сборки:
// негодный файл — ошибка программиста, а не данных пользователя, поэтому паника с причиной
// (её ловит TestBuiltinParse, до выпуска дело не доходит).
func Builtin() *Snapshot {
	if s := builtinTest.Load(); s != nil {
		return s
	}
	builtinOnce.Do(func() {
		builtinSnap = mustLoadBuiltin(builtinFS, builtinFiles)
	})
	return builtinSnap
}

// mustLoadBuiltin — loadBuiltin с паникой на ошибке.
func mustLoadBuiltin(fsys fs.FS, names []string) *Snapshot {
	s, err := loadBuiltin(fsys, names)
	if err != nil {
		panic("printprof: " + err.Error())
	}
	return s
}

// loadBuiltin разбирает builtin/<имя>.json по списку names в порядке списка.
func loadBuiltin(fsys fs.FS, names []string) (*Snapshot, error) {
	ps := make([]Profile, 0, len(names))
	for _, name := range names {
		data, err := fs.ReadFile(fsys, "builtin/"+name+".json")
		if err != nil {
			return nil, fmt.Errorf("встроенный профиль %s: %w", name, err)
		}
		p, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("встроенный профиль %s: %w", name, err)
		}
		ps = append(ps, p)
	}
	return NewSnapshot(ps)
}

// UseBuiltinForTest подставляет s вместо встроенных профилей на время теста: «новая программа со
// встроенной старшей версией» без правки самих файлов. Не для t.Parallel: снимок общий на процесс.
func UseBuiltinForTest(t tb, s *Snapshot) {
	t.Helper()
	prev := builtinTest.Load()
	builtinTest.Store(s)
	t.Cleanup(func() { builtinTest.Store(prev) })
}
