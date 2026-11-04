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

// replayIdent — отпечаток альбома: файл записан шагами сценария (NeedUID false), устаревший
// отпечаток — отказ; восстановление копии без отпечатка без записи — новый отпечаток, его
// записывает SaveUID (без печати пути — выдаёт новый с печатью); Ctrl+Z возвращает прежний;
// «Сохранить как» другим проектом — новый.
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
	share := func(name string) { // отпечаток для записи о ссылке: "" — печать не этого пути
		path, uid := e.ShareIdent()
		own := ""
		if uid != "" {
			own = mark(uid)
		}
		r.read(name, []string{strings.Replace(path, r.dir, "<dir>", 1), own})
	}
	need := func(name, path, uid string) {
		n, err := e.NeedUID(path, uid)
		r.read(name, []any{n, fmt.Sprint(err)})
	}
	path, u1 := ident("Ident")
	share("ShareIdent:записан без выдачи")
	need("NeedUID", path, u1)
	need("NeedUID:чужой", path, strings.Repeat("0", 32))
	need("NeedUID:другой путь", filepath.Join(r.dir, "другой.salbum"), u1)

	q, _ := e.Snapshot()
	q.UID = ""
	if _, err := e.Restore(q); err != nil { // без записи: в файле прежний отпечаток
		r.t.Fatalf("Restore: %v", err)
	}
	_, u2 := ident("Ident:после восстановления")
	need("NeedUID:после восстановления", path, u2)
	u2, err := e.SaveUID(path, u2) // отпечаток без печати пути — выдан новый с печатью
	r.write("SaveUID", err, path)
	ident("Ident:выдан")
	share("ShareIdent:выдан")
	need("NeedUID:записан", path, u2)
	_, err = e.SaveUID(path, u1)
	r.read("SaveUID:устаревший", fmt.Sprint(err))

	r.do("Undo:отпечаток", e.Undo())
	ident("Ident:после отмены")

	other := filepath.Join(r.dir, "новый.salbum")
	r.write("SaveNew", e.SaveNew(other), other)
	_, u3 := ident("Ident:сохранить как")
	share("ShareIdent:сохранить как")
	need("NeedUID:сохранить как", other, u3)

	at, q2, _, sealed := e.SnapshotAt() // путь, отпечаток и печать снимка выгрузки — от одного альбома
	r.read("SnapshotAt", []any{strings.Replace(at, r.dir, "<dir>", 1), mark(q2.UID), sealed})
}
