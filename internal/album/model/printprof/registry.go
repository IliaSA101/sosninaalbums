package printprof

import (
	"fmt"
	"slices"
	"sync/atomic"
)

// Snapshot — неизменяемый набор профилей: порядок задаёт порядок рядов окна «Типография» и
// пунктов «Печати». Профили внутри не выходят наружу иначе как глубокими копиями (Profile.Clone):
// правка полученного профиля снимок не меняет, новые таблицы — новым снимком.
type Snapshot struct {
	profiles []Profile
	src      map[string]string // id → слой (layers.go); nil — все встроенные
	own      map[string]int    // id своей → номер в списке own у Compose (OwnIndex)
}

// NewSnapshot собирает снимок и проверяет, что профили не спорят: id профиля, имя группы (по
// скелету NameKey) и id изделий уникальны (изделие — пресет обложки на весь реестр, группа — ключ
// рядов и пределов).
func NewSnapshot(profiles []Profile) (*Snapshot, error) {
	ids, names, products := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, p := range profiles {
		if err := p.Validate(); err != nil {
			return nil, fmt.Errorf("профиль %s: %w", q(p.ID), err)
		}
		if ids[p.ID] || names[NameKey(p.Name)] {
			return nil, fmt.Errorf("профиль %s: id или название уже заняты", q(p.ID))
		}
		ids[p.ID], names[NameKey(p.Name)] = true, true
		for _, pr := range p.Products {
			if products[pr.ID] {
				return nil, fmt.Errorf("профиль %s: изделие %s уже есть у другого", q(p.ID), q(pr.ID))
			}
			products[pr.ID] = true
		}
	}
	return &Snapshot{profiles: cloneAll(profiles)}, nil
}

// cloneAll — глубокие копии профилей (Profile.Clone) в том же порядке.
func cloneAll(ps []Profile) []Profile {
	out := make([]Profile, len(ps))
	for i, p := range ps {
		out[i] = p.Clone()
	}
	return out
}

// Profiles — профили по порядку, глубокие копии.
func (s *Snapshot) Profiles() []Profile { return cloneAll(s.profiles) }

// ByID — профиль по id, глубокая копия.
func (s *Snapshot) ByID(id string) (Profile, bool) {
	if i := slices.IndexFunc(s.profiles, func(p Profile) bool { return p.ID == id }); i >= 0 && id != "" {
		return s.profiles[i].Clone(), true
	}
	return Profile{}, false
}

// ByName — профиль по названию группы пресетов (Profile.Name), глубокая копия.
func (s *Snapshot) ByName(name string) (Profile, bool) {
	if i := slices.IndexFunc(s.profiles, func(p Profile) bool { return p.Name == name }); i >= 0 && name != "" {
		return s.profiles[i].Clone(), true
	}
	return Profile{}, false
}

// current — действующий снимок; nil — ещё встроенный. Тяжёлые таблицы (model.tables) кешируются
// по указателю снимка, поэтому подмена всегда новым значением.
var current atomic.Pointer[Snapshot]

// Current — действующий снимок. В (В) это встроенные профили; слои (свои, набор из библиотеки)
// публикуют новый снимок через Install.
func Current() *Snapshot {
	if s := current.Load(); s != nil {
		return s
	}
	return Builtin()
}

// Install делает s действующим снимком; nil возвращает встроенные. Вызывающий отвечает за то,
// чтобы после этого перечитать производные таблицы (model делает это сам по указателю).
func Install(s *Snapshot) { current.Store(s) }

// tb — то, что нужно от *testing.T: пакет не тянет testing в рабочий код.
type tb interface {
	Helper()
	Cleanup(func())
}

// UseForTest ставит s на время теста и возвращает прежний снимок на выходе. Не для t.Parallel:
// реестр общий на процесс.
func UseForTest(t tb, s *Snapshot) {
	t.Helper()
	prev := current.Load()
	current.Store(s)
	t.Cleanup(func() { current.Store(prev) })
}
