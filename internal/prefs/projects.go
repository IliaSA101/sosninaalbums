package prefs

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Записи выгрузок по проекту (167-4): <папка данных>/projects.json, ключ — uid проекта. Запись
// не привязана к пути навсегда: поле path говорит, чей это файл на этом компьютере, и «своя»
// запись — только та, чей path равен пути файла (paths.Fold). shares.json 0.38 только читает
// (одноразовый импорт, projectsimport.go) и никогда не пишет: откат на 0.37 работает.
// Пределы и правила чтения — как у shares.json (recstore.go).

var (
	// ErrProjectsNewer — projects.json записан более новой программой: читаем, не пишем.
	ErrProjectsNewer = errors.New("projects.json записан более новой версией программы")
	// ErrProjectsUnread — projects.json есть, но не открылся или не отложен: записи не меняем.
	ErrProjectsUnread = errors.New("не удалось прочитать projects.json")
	// ErrProjectsFull — записей или файла больше предела.
	ErrProjectsFull = errors.New("слишком много записей о проектах")
	// ErrProjectUID — uid негоден в ключ записи (не 32 знака hex нижнего регистра).
	ErrProjectUID = errors.New("uid не годится для записи о проекте")
)

// ProjectsPath — projects.json в папке данных dir; пустая dir — папки нет, путь пуст.
func ProjectsPath(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "projects.json")
}

// ProjectRecord — запись о выгрузке проекта. Поля ShareRecord без UID (он — ключ) и два новых.
type ProjectRecord struct {
	Origin      string    `json:"origin"`
	ID          string    `json:"id"`
	URL         string    `json:"url,omitempty"` // пуста, пока не опубликовано
	Title       string    `json:"title,omitempty"`
	Password    string    `json:"password,omitempty"` // открытым текстом (design 146 §6 п. 6); при PassUnknown пуст
	ExpiresAt   time.Time `json:"expires_at,omitzero"`
	PublishedAt time.Time `json:"published_at,omitzero"`
	Scene       string    `json:"scene,omitempty"` // хэш сцены на момент публикации; пуст — «изменён»
	Gone        bool      `json:"gone,omitempty"`
	// Path — файл, к которому привязана запись (Clean, настоящий регистр); "" — найдена на
	// сервисе и не привязана. Негодный путь (UNC, \\?\, устройство) читается как "".
	Path string `json:"path,omitempty"`
	// PassUnknown — у альбома есть пароль, а этому компьютеру он не известен (167-3).
	PassUnknown bool `json:"pass_unknown,omitempty"`
}

// Own — запись своя для файла path: привязана к нему (167-12). Регистр и разделители — как у
// всех ключей (Clean и paths.Fold). Пустой путь ничьим не бывает.
func (r ProjectRecord) Own(path string) bool {
	if r.Path == "" || path == "" {
		return false
	}
	return paths.Fold(filepath.Clean(r.Path)) == paths.Fold(filepath.Clean(path))
}

// TitleAuto — название записи было именем её файла (167-7): не хранится, выводится из Path.
// clean — очистка названия, как у манифеста (share.CleanTitle; prefs от share не зависит);
// nil — как есть. Записи без пути признак неизвестен — false (тогда сверяют с названием
// задания через TitleIs).
func (r ProjectRecord) TitleAuto(clean func(string) string) bool {
	if r.Path == "" {
		return false
	}
	return r.TitleIs(strings.TrimSuffix(filepath.Base(r.Path), filepath.Ext(r.Path)), clean)
}

// TitleIs — название записи равно name после очистки, без учёта регистра (paths.Fold: запись,
// импортированная по ключу shares.json, помнит путь только в нижнем регистре).
func (r ProjectRecord) TitleIs(name string, clean func(string) string) bool {
	if clean != nil {
		name = clean(name)
	}
	return paths.Fold(r.Title) == paths.Fold(name)
}

// ShareRecord — запись в виде shares.json 0.37 под uid.
func (r ProjectRecord) ShareRecord(uid string) ShareRecord {
	return ShareRecord{Origin: r.Origin, ID: r.ID, URL: r.URL, Title: r.Title, Password: r.Password,
		ExpiresAt: r.ExpiresAt, PublishedAt: r.PublishedAt, Scene: r.Scene, Gone: r.Gone, UID: uid}
}

// ProjectOf — запись проекта из ShareRecord, привязанная к path (UID — ключ, в запись не идёт).
func ProjectOf(rec ShareRecord, path string) ProjectRecord {
	return ProjectRecord{Origin: rec.Origin, ID: rec.ID, URL: rec.URL, Title: rec.Title, Password: rec.Password,
		ExpiresAt: rec.ExpiresAt, PublishedAt: rec.PublishedAt, Scene: rec.Scene, Gone: rec.Gone, Path: path}
}

func sameProject(a, b ProjectRecord) bool {
	return a.Origin == b.Origin && a.ID == b.ID && a.URL == b.URL && a.Title == b.Title &&
		a.Password == b.Password && a.ExpiresAt.Equal(b.ExpiresAt) && a.PublishedAt.Equal(b.PublishedAt) &&
		a.Scene == b.Scene && a.Gone == b.Gone && a.Path == b.Path && a.PassUnknown == b.PassUnknown
}

// cleanProjectPath — путь записи: негодный (не локальный, с управляющими знаками, не UTF-8)
// отдаёт "" — запись остаётся, но ни к какому файлу не привязана. CheckLocalPath — до Clean:
// Clean("//srv/x") иначе сделал бы из UNC локальный путь.
func cleanProjectPath(p string) string {
	if p == "" || paths.CheckLocalPath(p) != nil || !utf8.ValidString(p) {
		return ""
	}
	for _, r := range p {
		if unicode.IsControl(r) {
			return ""
		}
	}
	return filepath.Clean(p)
}

// cleanProject проверяет запись правилами shares.json (адрес, id, ссылка, пароль, название,
// хэш сцены) и чистит путь; ok == false — запись отбросить.
func (r ShareRules) cleanProject(rec ProjectRecord) (ProjectRecord, bool) {
	c, ok := r.clean(rec.ShareRecord(""))
	if !ok {
		return ProjectRecord{}, false
	}
	out := ProjectOf(c, cleanProjectPath(rec.Path))
	if out.PassUnknown = rec.PassUnknown; out.PassUnknown {
		out.Password = "" // пароля этот компьютер не знает: чужой в запись не кладём
	}
	return out, true
}

// projectsFile — содержимое projects.json; version первой.
type projectsFile struct {
	Version  int                        `json:"version"`
	Projects map[string]json.RawMessage `json:"projects"`
}

// Projects — записи выгрузок по uid в папке данных dir. Без папки данных (dir == "") записи
// живут в памяти до выхода. Правила адресов даёт вызывающий (ShareRules).
type Projects struct {
	store[ProjectRecord]
	rules ShareRules
}

// NewProjects — записи выгрузок папки данных dir. Нет projects.json — он создаётся из
// shares.json (по uid), сам shares.json не меняется.
func NewProjects(dir string, rules ShareRules) *Projects {
	p := &Projects{rules: rules}
	imp := NewShares(dir, rules) // читатель shares.json для импорта: один на Projects, только чтение
	imp.readonly = true
	p.store = newStore(dir, storeKind[ProjectRecord]{
		file: "projects.json",
		decode: func(data []byte) (int, map[string]json.RawMessage, error) {
			var f projectsFile
			err := json.Unmarshal(data, &f)
			return f.Version, f.Projects, err
		},
		encode: func(items map[string]json.RawMessage) ([]byte, error) {
			return json.MarshalIndent(projectsFile{Version: sharesVersion, Projects: items}, "", "  ")
		},
		key:      model.GoodUID,
		clean:    rules.cleanProject,
		same:     sameProject,
		expires:  func(r ProjectRecord) time.Time { return r.ExpiresAt },
		seed:     func() (map[string]ProjectRecord, error) { return importShares(imp) },
		errNewer: ErrProjectsNewer, errUnread: ErrProjectsUnread, errFull: ErrProjectsFull,
	})
	return p
}

// Get — запись проекта; нет записи (или uid негоден) — false.
func (p *Projects) Get(uid string) (ProjectRecord, bool) {
	if !model.GoodUID(uid) {
		return ProjectRecord{}, false
	}
	sharesMu.Lock()
	defer sharesMu.Unlock()
	recs, _ := p.load()
	rec, ok := recs[uid]
	return rec, ok
}

// Put пишет запись проекта (заменяет прежнюю). Негодный uid — ErrProjectUID, запись не по
// правилам — ErrShareRecord; файл не тронут.
func (p *Projects) Put(uid string, rec ProjectRecord) error {
	if !model.GoodUID(uid) {
		return ErrProjectUID
	}
	rec, ok := p.rules.cleanProject(rec)
	if !ok {
		return ErrShareRecord
	}
	return p.modify(func(recs map[string]ProjectRecord) error {
		if _, had := recs[uid]; !had && len(recs) >= maxShares {
			return ErrProjectsFull
		}
		recs[uid] = rec
		return nil
	})
}

// PutIfAbsent пишет запись, только если под uid ничего нет: проверка и запись под одним
// замком, запись, появившаяся за время поиска на сервисе, не затирается. Возвращает запись,
// которая теперь под uid (новая или прежняя), и added — легла ли новая.
func (p *Projects) PutIfAbsent(uid string, rec ProjectRecord) (cur ProjectRecord, added bool, err error) {
	if !model.GoodUID(uid) {
		return ProjectRecord{}, false, ErrProjectUID
	}
	rec, ok := p.rules.cleanProject(rec)
	if !ok {
		return ProjectRecord{}, false, ErrShareRecord
	}
	err = p.modify(func(recs map[string]ProjectRecord) error {
		if had, ok := recs[uid]; ok {
			cur = had
			return nil
		}
		if len(recs) >= maxShares {
			return ErrProjectsFull
		}
		recs[uid], cur, added = rec, rec, true
		return nil
	})
	if err != nil {
		return ProjectRecord{}, false, err
	}
	return cur, added, nil
}

// Update меняет запись проекта под замком: fn получает копию, результат проверяется по
// правилам и пишется. Записи нет — ok == false, fn не зовётся, файл не пишется.
func (p *Projects) Update(uid string, fn func(rec *ProjectRecord)) (rec ProjectRecord, ok bool, err error) {
	if !model.GoodUID(uid) {
		return ProjectRecord{}, false, ErrProjectUID
	}
	err = p.modify(func(recs map[string]ProjectRecord) error {
		cur, had := recs[uid]
		if !had {
			return nil
		}
		fn(&cur)
		var good bool
		if cur, good = p.rules.cleanProject(cur); !good {
			return ErrShareRecord
		}
		recs[uid], rec, ok = cur, cur, true
		return nil
	})
	if err != nil {
		return ProjectRecord{}, false, err
	}
	return rec, ok, nil
}

// Delete убирает запись проекта; записи нет — не ошибка, файл не пишется.
func (p *Projects) Delete(uid string) error {
	if !model.GoodUID(uid) {
		return ErrProjectUID
	}
	return p.modify(func(recs map[string]ProjectRecord) error {
		delete(recs, uid)
		return nil
	})
}
