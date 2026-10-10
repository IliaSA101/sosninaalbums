// Package inside — доступ тестов к живому состоянию Editor и к полям модели, которые нельзя
// экспортировать: экспортное поле попало бы в JSON снимков отмены и в проверку холостой
// правки (023). Значения ставит ядро в init (internal/album/inside.go); импортировать
// могут только пакеты под internal/album, типизированные обёртки — edtest.
//
// Тип any: inside не импортирует ядро (ядро импортирует inside). e — *album.Editor.
package inside

import (
	"sync"
	"time"
)

// Чтение полей Editor.
var (
	P         func(e any) any // *Project
	Undo      func(e any) any // []Snap
	Redo      func(e any) any // []Snap
	Marks     func(e any) any // map[string]Mark
	Mu        func(e any) *sync.Mutex
	Rev       func(e any) int
	Dirty     func(e any) bool
	Backup    func(e any) bool
	LastMerge func(e any) string
	UndoBytes func(e any) int
)

// Запись полей Editor (тесты пределов отмены и слияния).
var (
	SetUndo      func(e, v any) // []Snap
	SetDirty     func(e any, v bool)
	SetLastMerge func(e any, v string)
	SetUndoAlbum func(e any, v int)
	SetUndoBytes func(e any, v int)
)

// Неэкспортные методы Editor: fn и результат — типы ядра.
var (
	Change      func(e, fn any) any                            // fn func(*Project) → View
	Try         func(e any, merge string, fn any) (any, error) // fn func(*Project) error → View
	RelinkMarks func(e, files, dirs any) any                   // []PathPair → []MarkChange
)

// Поля модели без экспорта (JSON): Project.from, Project.loaded, Mark.alt.
var (
	From   func(p any) int // p — *Project
	Loaded func(p any) any // map[string]Mark
	Alt    func(m any) string
)

// Крючок ядра: порог медленной операции (slowOp в editor.go) — тест лога ставит 0 и час.
var (
	SlowOp    func() time.Duration
	SetSlowOp func(d time.Duration)
)
