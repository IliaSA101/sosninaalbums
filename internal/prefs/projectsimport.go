package prefs

import (
	"cmp"
	"strings"
)

// Одноразовый импорт shares.json (0.37) в projects.json (167-4): запись остаётся за тем же
// файлом (ключ-путь → path), ключом становится её UID. shares.json читается как чужой файл —
// без перекладывания в .bad и без записи: откат на 0.37 должен работать.

// importShares — записи для нового projects.json из shares.json папки dir. Записи без годного
// UID пропускаются. Дубли UID (копия альбома выгружена с двух путей): победитель по
// PublishedAt (позднее), затем ExpiresAt (позднее), затем ключ-путь (меньший), затем id
// (меньший) — итог не зависит от порядка чтения. Файл не открылся (замок антивируса) — ошибка:
// projects.json не создаётся, импорт повторится при следующем чтении. sh — один читатель
// (readonly) на Projects: журнал о битом файле не пишется при каждом чтении. Вызывающий
// держит sharesMu.
func importShares(sh *Shares) (map[string]ProjectRecord, error) {
	recs, st := sh.load()
	if st.unread != nil {
		return nil, st.unread
	}
	type cand struct {
		key string
		rec ShareRecord
	}
	won := map[string]cand{}
	for key, rec := range recs {
		if rec.UID == "" {
			continue
		}
		c := cand{key, rec}
		if cur, ok := won[rec.UID]; !ok || beatsV1(c.key, c.rec, cur.key, cur.rec) {
			won[rec.UID] = c
		}
	}
	out := make(map[string]ProjectRecord, len(won))
	for uid, c := range won {
		out[uid] = ProjectOf(c.rec, cleanProjectPath(c.key))
	}
	return out, nil
}

// beatsV1 — запись a побеждает запись b с тем же UID: полный порядок, без равенства.
func beatsV1(keyA string, a ShareRecord, keyB string, b ShareRecord) bool {
	if c := a.PublishedAt.Compare(b.PublishedAt); c != 0 {
		return c > 0
	}
	if c := a.ExpiresAt.Compare(b.ExpiresAt); c != 0 {
		return c > 0
	}
	if c := strings.Compare(keyA, keyB); c != 0 {
		return c < 0
	}
	return cmp.Compare(a.ID, b.ID) < 0
}
