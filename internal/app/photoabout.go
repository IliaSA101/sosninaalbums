package app

import (
	"errors"
	"io/fs"
	"log"
	"slices"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Карточка фото (084 design §4): данные съёмки и вхождения в альбом. Только чтение.

// PhotoAbout — сведения о фото для карточки и раздела «Фото из ленты».
type PhotoAbout struct {
	ID   string           `json:"id"`
	File string           `json:"file"` // «6000 × 4000 px · 24 Мп · 12,3 МБ»; "" — файл не прочитан
	Shot []string         `json:"shot"` // строки «О снимке»; не nil
	Note string           `json:"note"` // "" | «Нет данных о съёмке» | «Файл не найден»
	Used string           `json:"used"` // «Использовано N раз» | «В альбоме не использовано»
	Uses []model.PhotoUse `json:"uses"` // не nil
}

const (
	noteNoShot  = "Нет данных о съёмке"
	noteMissing = "Файл не найден"
	aboutMax    = 512 // записей кэша; в сессии смотрят десятки фото — сброс целиком не мешает
)

// readAbout — library.ReadAbout; подменяется в тестах (счёт чтений файла).
var readAbout = library.ReadAbout

// aboutKey — файл в том виде, в каком его прочитали: изменился размер или время — читаем заново.
type aboutKey struct {
	path string
	size int64
	mod  int64 // ModTime().UnixNano(): time.Time в ключе map нельзя (== учитывает пояс и монотонные часы)
}

// aboutCache — прочитанные сведения и ID, о чьей ошибке чтения уже написано в log.txt.
type aboutCache struct {
	mu     sync.Mutex
	m      map[aboutKey]library.About
	logged map[string]bool
}

func (c *aboutCache) get(k aboutKey) (library.About, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[k]
	return v, ok
}

func (c *aboutCache) put(k aboutKey, v library.About) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil || len(c.m) >= aboutMax {
		c.m = map[aboutKey]library.About{}
	}
	c.m[k] = v
}

// logOnce пишет ошибку чтения в log.txt раз на ID: карточка перезапрашивает на каждую правку.
func (c *aboutCache) logOnce(id string, err error) {
	if errors.Is(err, fs.ErrNotExist) {
		return // пропавший файл — ожидаемое состояние: карточка скажет «Файл не найден», журнал не засоряем
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.logged[id] {
		return
	}
	if c.logged == nil || len(c.logged) >= aboutMax {
		c.logged = map[string]bool{}
	}
	c.logged[id] = true
	log.Printf("карточка фото: %v", paths.NoPath(err))
}

// PhotoAbout — сведения о фото id: путь берётся из ленты, иначе из проекта (как /media); путь
// от интерфейса не принимается. Недостающее фото и мёртвый том — «Файл не найден» без чтения.
func (a *App) PhotoAbout(id string) (out PhotoAbout, err error) {
	defer guardErr("PhotoAbout", &err)
	a.mu.Lock()
	lp, inLib := a.byID[id]
	_, gone := slices.BinarySearch(a.missing, id) // missing — по порядку
	a.mu.Unlock()
	ed := a.editor()
	path := lp.Path
	if !inLib {
		ref, ok := ed.Ref(id)
		if !ok {
			return PhotoAbout{}, errors.New("фото не найдено") // UI ошибку не показывает: блока просто нет
		}
		path = ref.Path
	}
	out = PhotoAbout{ID: id, Shot: []string{}}
	out.Uses, out.Used = ed.PhotoUses(id)
	// Порядок важен: CheckLocalPath без диска режет UNC до пробы тома (утечка NTLM).
	if gone || paths.CheckLocalPath(path) != nil || !reachable(path) {
		out.Note = noteMissing
		return out, nil
	}
	ab, rerr := a.aboutOf(path)
	if rerr != nil {
		a.about.logOnce(id, rerr)
		out.Note = noteMissing
		return out, nil
	}
	out.File = ab.File
	if len(ab.Shot) == 0 {
		out.Note = noteNoShot
	} else {
		out.Shot = slices.Clone(ab.Shot)
	}
	return out, nil
}

// aboutOf — сведения о файле из кэша по (путь, размер, время записи), иначе чтение. Замок кэша
// не держится на время чтения: два одновременных промаха прочтут файл дважды — безвредно.
func (a *App) aboutOf(path string) (library.About, error) {
	st, err := library.StatLocal(path)
	if err != nil {
		return library.About{}, err
	}
	k := aboutKey{path: path, size: st.Size(), mod: st.ModTime().UnixNano()}
	if v, ok := a.about.get(k); ok {
		return v, nil
	}
	v, err := readAbout(path)
	if err != nil {
		return library.About{}, err
	}
	a.about.put(k, v)
	return v, nil
}
