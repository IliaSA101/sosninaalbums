package libpacks

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Сохранённый список «Можно скачать» (спека 137, design §2, безопасность Д1–Д2, З1–З4): конверт
// каталога последней проверки {sig, body} байт в байт лежит в папке настроек. Файл недоверенный
// (его мог подменить любой процесс пользователя): читается как обычный файл не больше
// fetch.MaxEnvelope и проходит ту же проверку подписи и seq, что свежий каталог. «Скачать» берёт
// адрес архива не отсюда, а из свежего каталога, поэтому худшее при подмене — пустой раздел.

// catalogFile — файл списка в папке настроек.
const catalogFile = "library-catalog.json"

var (
	errCacheKind = errors.New("не обычный файл или больше предела")
	errCacheRead = errors.New("файл не читается")
)

// catPath — "" при пустой папке настроек: список тогда живёт только в памяти (140 Н4).
func (s *Service) catPath() string {
	if s.h.CfgDir() == "" {
		return ""
	}
	return filepath.Join(s.h.CfgDir(), catalogFile)
}

// cached — сохранённый список: из памяти, а при первом обращении — с диска (неудача тоже
// запоминается: окно опрашивает список раз в 2 с и не должно перечитывать файл с проверкой
// подписи каждый раз). bad — файл есть, но не читается. Каталог только читать: он общий.
func (s *Service) cached() (cat *fetch.Catalog, at time.Time, bad bool) {
	s.catMu.Lock()
	defer s.catMu.Unlock()
	if !s.loaded {
		s.loadLocked()
	}
	return s.cat, s.catAt, s.catBad
}

// loadLocked — первое чтение файла, под catMu. Нет файла — молча (первый запуск); есть, но не
// читается — одна строка в log.txt за сеанс, раздел пуст до следующей проверки.
func (s *Service) loadLocked() {
	s.loaded = true
	parse := func(env []byte) (fetch.Catalog, error) {
		c := s.h.Fetcher()
		if c == nil {
			return fetch.Catalog{}, fetch.ErrCatalog
		}
		// Чтение без побочных действий: seq файла больше принятого CatalogSeq не записывается.
		return c.Parse(env, prefs.LibraryNetOf(s.h.CfgDir()).CatalogSeq)
	}
	path := s.catPath()
	if path == "" {
		return // файла нет и быть не может
	}
	cat, at, err := readCatalog(path, parse)
	switch {
	case err == nil:
		s.cat, s.catAt = &cat, at
	case errors.Is(err, fs.ErrNotExist):
	default:
		s.catBad = true
		log.Printf("библиотека: сохранённый список наборов не читается — %v", err)
	}
}

// readCatalog — файл списка: обычный файл (не ссылка, не канал, не папка) не больше MaxEnvelope
// до чтения, открытый тот же, что проверенный (З1), затем parse (подпись и seq). Ошибки — без
// пути: текст уходит в log.txt.
func readCatalog(path string, parse func(env []byte) (fetch.Catalog, error)) (fetch.Catalog, time.Time, error) {
	if paths.CheckLocalPath(path) != nil {
		return fetch.Catalog{}, time.Time{}, errCacheKind
	}
	lst, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fetch.Catalog{}, time.Time{}, fs.ErrNotExist
	case err != nil:
		return fetch.Catalog{}, time.Time{}, fmt.Errorf("%w (%T)", errCacheRead, err)
	case !lst.Mode().IsRegular() || lst.Size() > fetch.MaxEnvelope:
		return fetch.Catalog{}, time.Time{}, errCacheKind
	}
	// На Windows номер файла читается лениво по пути: сверка открытого с проверенным без этого
	// сравнила бы подменённый файл с самим собой (как library.OpenLocal).
	_ = os.SameFile(lst, lst)
	f, got, err := library.OpenLocal(path)
	if err != nil {
		return fetch.Catalog{}, time.Time{}, fmt.Errorf("%w (%T)", errCacheRead, err)
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	if !os.SameFile(lst, got) || !got.Mode().IsRegular() || got.Size() > fetch.MaxEnvelope {
		return fetch.Catalog{}, time.Time{}, errCacheKind
	}
	env, err := io.ReadAll(io.LimitReader(f, fetch.MaxEnvelope+1))
	if err != nil {
		return fetch.Catalog{}, time.Time{}, fmt.Errorf("%w (%T)", errCacheRead, err)
	}
	if len(env) > fetch.MaxEnvelope {
		return fetch.Catalog{}, time.Time{}, errCacheKind
	}
	cat, err := parse(env)
	return dropDevices(cat), got.ModTime(), err
}

// dropDevices — строки с id-именем устройства Windows (con, nul…) уходят в Skipped: fetch
// пропускает их по грамматике, а decor отвергает (критика Д1); и на пути кэша, и на свежем.
func dropDevices(cat fetch.Catalog) fetch.Catalog {
	keep := make([]fetch.Entry, 0, len(cat.Packs))
	for _, e := range cat.Packs {
		if paths.Device(e.ID) {
			cat.Skipped = append(cat.Skipped, fetch.Skip{ID: e.ID, Why: "недопустимый id"})
			continue
		}
		keep = append(keep, e)
	}
	cat.Packs = keep
	return cat
}

// remember — свежий каталог становится сохранённым: файл (ошибка записи только в log.txt,
// операция не падает) и память под одним catMu, чтобы запоздалое ленивое чтение не затёрло его
// старым (Д1). Env после записи обнуляется — лишний мегабайт не держится весь сеанс (З4).
// Каталог с меньшим seq, чем у запомненного, не берётся ни в память, ни в файл (140 Н3): «Обновить»
// и каталог для роликов идут параллельно, и ответ сырого CDN сразу после выпуска мог прийти
// позже нового. Итог — каталог, который действует после вызова.
func (s *Service) remember(cat fetch.Catalog) fetch.Catalog {
	s.catMu.Lock()
	defer s.catMu.Unlock()
	if s.cat != nil && cat.Seq < s.cat.Seq {
		return *s.cat
	}
	grew := s.cat == nil || cat.Seq > s.cat.Seq
	path, at := s.catPath(), time.Now()
	if len(cat.Env) > 0 && path != "" {
		if err := writeCatalog(path, cat.Env); err != nil {
			log.Printf("библиотека: список наборов не сохранён (%T)", err)
		} else if fi, err := library.StatLocal(path); err == nil {
			at = fi.ModTime()
		}
	}
	cat.Env = nil
	s.cat, s.catAt, s.catBad, s.loaded = &cat, at, false, true
	if grew {
		s.cells.clearBad() // плохие листы клеток — до нового списка; тот же seq — список тот же (144 Н8)
	}
	return cat
}

// Cached — сохранённый каталог без сети (файл проверяется при первом чтении, дальше — память):
// ok == false — списка нет или файл не читается. Каталог только читать: он общий. Ролики
// справочника (guidevideo) берут отсюда список и адреса не берут — адрес из свежего (Fresh).
func (s *Service) Cached() (cat fetch.Catalog, ok bool) {
	c, _, _ := s.cached()
	if c == nil {
		return fetch.Catalog{}, false
	}
	return *c, true
}

// Fresh — свежий каталог с сети: подпись и seq проверяет fetch, seq записывается в config.json,
// затем каталог становится сохранённым (remember) — порядок тот же, что у «Обновить» (137 Д1).
// Согласие на сеть сверяет вызывающий (Update — run, ролики — guidevideo). Замка наборов не берёт:
// каталог параллелен установке, меньший seq remember отбросит. Итог — действующий каталог.
func (s *Service) Fresh(ctx context.Context) (fetch.Catalog, error) {
	c := s.h.Fetcher()
	n := prefs.LibraryNetOf(s.h.CfgDir())
	cat, err := c.Catalog(ctx, n.CatalogSeq)
	if err != nil {
		return fetch.Catalog{}, err
	}
	cat = dropDevices(cat)
	s.baseline(cat) // до remember: иначе новый список сойдёт за уже объявленный
	prefs.UpdateLibraryNet(s.h.CfgDir(), func(n *prefs.LibraryNet) { n.CatalogSeq = max(n.CatalogSeq, cat.Seq) })
	for _, k := range cat.Skipped {
		log.Printf("библиотека: строка каталога %s: %s", k.ID, k.Why) // нейтрально: негодное превью строку не убирает (144)
	}
	eff := s.remember(cat) // список «Можно скачать» — после seq: меньший потом не примется
	s.gcPreviews(eff)      // по действующему: меньший seq remember отверг, и уборка идёт по прежнему (144 Н9)
	return eff, nil
}

// baseline — в config.json нет «объявленных» наборов (старый файл или первое обращение): до замены
// сохранённого списка записывается то, что он содержит сейчас. Список не читался (файл есть, но
// битый) — объявленным считается свежий: все наборы «новыми» не объявляем. Поле уже есть — как есть.
func (s *Service) baseline(fresh fetch.Catalog) {
	if prefs.LibraryNetOf(s.h.CfgDir()).Announced != nil {
		return
	}
	old, _, bad := s.cached()
	src := old
	if old == nil && bad {
		src = &fresh
	}
	ids := []string{}
	if src != nil {
		for _, e := range src.Packs {
			ids = append(ids, e.ID)
		}
	}
	prefs.UpdateLibraryNet(s.h.CfgDir(), func(n *prefs.LibraryNet) {
		if n.Announced == nil { // параллельный ход мог записать раньше
			n.Announced = ids
		}
	})
}

// writeCatalog — конверт во временный файл и Rename (library.WriteFileAtomicMode: 0600, Sync,
// при ошибке .tmp удаляется). Путь списка на запись не открывается: Rename заменяет саму запись,
// и подложенная ссылка не перенаправит запись в чужой файл (Д2). Остатки прежних записей
// (процесс убит между созданием и Rename) убираются заранее: в папке настроек их больше никто
// не уберёт.
func writeCatalog(path string, env []byte) error {
	dir, base := filepath.Dir(path), filepath.Base(path)
	if list, err := os.ReadDir(dir); err == nil {
		for _, e := range list {
			if n := e.Name(); e.Type().IsRegular() && strings.HasPrefix(n, base+".") && strings.HasSuffix(n, ".tmp") {
				_ = os.Remove(filepath.Join(dir, n)) // не вышло — остаток доберёт следующая запись
			}
		}
	}
	return library.WriteFileAtomicMode(path, env, 0o600)
}
