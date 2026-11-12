package album_test

// Сцена голден-реплея (replay_test.go): отпечаток проекта (146 T4-9). Идёт в самом конце
// сценария: прежние строки эталона не сдвигаются. Отпечаток случайный — в эталоне метки u1, u2…
// по порядку появления.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// replayIdent — отпечаток альбома: файл записан шагами сценария (NeedFileUID false), устаревший
// отпечаток — отказ; восстановление копии без отпечатка без записи — новый отпечаток, его
// записывает SaveFresh; Ctrl+Z возвращает прежний (SaveFileUID: запись в файл, если его там нет); «Сохранить как»
// другим проектом (SaveOver) — новый. Печать пути решений не принимает (167): шагов сверки нет.
func replayIdent(r *replay) {
	e := r.e
	marks := map[string]string{}
	mark := func(uid string) string {
		if !model.GoodUID(uid) {
			return "негодный:" + uid
		}
		if _, ok := marks[uid]; !ok {
			marks[uid] = fmt.Sprintf("u%d", len(marks)+1)
		}
		return marks[uid]
	}
	ident := func(name string) (string, string) {
		path, uid := e.Ident()
		r.read(name, []string{strings.Replace(path, r.dir, "<dir>", 1), mark(uid)})
		return path, uid
	}
	need := func(name, path, uid string) {
		n, err := e.NeedFileUID(path, uid)
		r.read(name, []any{n, fmt.Sprint(err)})
	}
	path, u1 := ident("Ident")
	need("NeedFileUID", path, u1)
	need("NeedFileUID:чужой", path, strings.Repeat("0", 32))
	need("NeedFileUID:другой путь", filepath.Join(r.dir, "другой.salbum"), u1)

	q, _ := e.Snapshot()
	q.UID = ""
	if _, err := e.Restore(q); err != nil { // без записи: в файле прежний отпечаток
		r.t.Fatalf("Restore: %v", err)
	}
	_, u2 := ident("Ident:после восстановления")
	need("NeedFileUID:после восстановления", path, u2)
	u2, err := e.SaveFresh(path, u2) // новый отпечаток с печатью пути, файл записан
	r.write("SaveFresh", err, path)
	ident("Ident:выдан")
	need("NeedFileUID:записан", path, u2)
	_, err = e.SaveFresh(path, u1)
	r.read("SaveFresh:устаревший", fmt.Sprint(err))

	r.do("Undo:отпечаток", e.Undo())
	ident("Ident:после отмены")
	need("NeedFileUID:после отмены", path, u1)
	back, err := e.SaveFileUID(path, u1)
	r.read("SaveFileUID", []any{mark(back), fmt.Sprint(err)})
	need("NeedFileUID:после SaveFileUID", path, u1)

	other := filepath.Join(r.dir, "новый.salbum")
	fresh, err := e.SaveOver(other)
	r.write("SaveOver", err, other)
	_, u3 := ident("Ident:сохранить как")
	r.read("SaveOver:новый", fresh)
	need("NeedFileUID:сохранить как", other, u3)

	at, q2, _, refs := e.SnapshotAt() // путь, отпечаток и привязки снимка выгрузки — от одного альбома
	r.read("SnapshotAt", []any{at == other, mark(q2.UID), len(refs)})
}
