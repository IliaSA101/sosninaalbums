package libpacks

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// pendingPack — распакованный набор, ждущий «Заменить».
type pendingPack struct {
	token string
	st    *decor.Staged
	until time.Time
	gone  chan struct{} // закрыт, когда вопрос снят: сторож просрочки больше не нужен
}

// lock — начало операции наборов; false — уже идёт другая.
func (s *Service) lock() bool {
	if !s.mu.TryLock() {
		return false
	}
	s.busy.Store(true)
	return true
}

func (s *Service) unlock() {
	s.busy.Store(false)
	s.mu.Unlock()
}

// Busy — идёт операция наборов (кнопки окна неактивны).
func (s *Service) Busy() bool { return s.busy.Load() }

// setPending — новый вопрос вытесняет прежний; через TTL распакованное удаляется само. Сторож —
// фон под Host.GoBG (голый таймер пережил бы App); App закрывается — вопрос снимается сразу.
func (s *Service) setPending(st *decor.Staged) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	pp := &pendingPack{token: hex.EncodeToString(b[:]), st: st, until: s.h.Now().Add(TTL), gone: make(chan struct{})}
	s.pendMu.Lock()
	old := s.pending
	s.pending = pp
	if old != nil {
		close(old.gone)
	}
	s.pendMu.Unlock()
	if old != nil {
		old.st.Close()
	}
	if !s.h.GoBG(func() {
		defer appkit.Guard("libpacksPending")
		t := time.NewTimer(TTL)
		defer t.Stop()
		select {
		case <-t.C:
		case <-pp.gone:
			return
		case <-s.h.StopCtx().Done():
		}
		if s.take(pp.token) != nil {
			pp.st.Close()
		}
	}) {
		if s.take(pp.token) != nil { // App закрывается: ответа уже не будет
			pp.st.Close()
		}
	}
	return pp.token
}

// take — ждущий набор по токену (и снимает его); не тот или просрочен — nil.
func (s *Service) take(token string) *pendingPack {
	s.pendMu.Lock()
	defer s.pendMu.Unlock()
	pp := s.pending
	if pp == nil || token == "" || pp.token != token {
		return nil
	}
	s.pending = nil
	close(pp.gone)
	if s.h.Now().After(pp.until) {
		pp.st.Close()
		return nil
	}
	return pp
}

// dropPending — следующая операция отменяет неотвеченный вопрос (design §4.3).
func (s *Service) dropPending() {
	s.pendMu.Lock()
	pp := s.pending
	s.pending = nil
	if pp != nil {
		close(pp.gone)
	}
	s.pendMu.Unlock()
	if pp != nil {
		pp.st.Close()
	}
}

func newAdd() Add { return Add{Skipped: []Skip{}} }

// List — список наборов: встроенные и поставленные.
func (s *Service) List() (list List, err error) {
	defer appkit.GuardErr("LibraryPacks", &err)
	list = List{Packs: []Info{}, Busy: s.Busy(), Net: netView(prefs.LibraryNetOf(s.h.CfgDir()))}
	st, err := s.h.Store()
	if err != nil {
		return list, err
	}
	for _, p := range st.Packs() {
		list.Packs = append(list.Packs, Info(p))
	}
	list.ReadOnly = st.ReadOnly()
	return list, nil
}

// AddFile — «Добавить из файла…»: диалог *.zip в Go. Отказ от диалога — пустой итог.
func (s *Service) AddFile() (res Add, err error) {
	defer appkit.GuardErr("AddPackFile", &err)
	res = newAdd()
	path, err := s.h.PickFile()
	if err != nil || path == "" {
		return res, err
	}
	res, err = s.AddPath(path)
	if err == nil {
		s.h.RememberDir(filepath.Dir(path))
	}
	return res, err
}

// AddPath — набор по пути (как ImportDecorPaths: для проверки из браузера). Путь — только
// локальный обычный файл .zip. Поставленный набор находит пропавшие своего альбома (Host.Relink —
// шаг, если что-то нашлось) и шлёт library:changed.
func (s *Service) AddPath(path string) (res Add, err error) {
	defer appkit.GuardErr("AddPackPath", &err)
	res = newAdd()
	st, err := s.h.Store()
	if err != nil {
		return res, err
	}
	if paths.CheckLocalPath(path) != nil { // ошибка с полным путём — в тост не идёт (как importOne)
		return res, ErrRemote
	}
	if !strings.EqualFold(filepath.Ext(path), ".zip") {
		return res, ErrFile
	}
	if !s.lock() {
		return res, ErrBusy
	}
	defer s.unlock()
	s.dropPending()
	// Проверенная цель и сверка с открытым (как readPrintFile, аудит 123 №8).
	f, _, err := library.OpenLocal(path)
	if err != nil {
		return res, errors.New("файл набора не найден")
	}
	arc, err := pack.OpenFile(f, s.h.Version())
	if err != nil {
		return res, err
	}
	defer arc.Close() //nolint:errcheck // чтение архива: ошибка закрытия ничего не теряет
	res.Skipped = packSkips(arc.Manifest.Skipped)
	sg, err := st.StagePack(decorManifest(arc.Manifest), arc, decor.OriginFile, arc.SHA256)
	if sg != nil {
		for _, k := range sg.Skipped {
			res.Skipped = append(res.Skipped, Skip(k))
		}
	}
	if err != nil {
		sg.Close()
		return res, packEmptyErr(err, res.Skipped)
	}
	res.Pack, res.Ask, res.FromVersion = Info(sg.Info()), sg.Ask, sg.From
	if sg.Ask == decor.AskUpdate {
		res.Token = s.setPending(sg) // распакованное ждёт «Заменить»
		return res, nil
	}
	if sg.Ask != "" {
		sg.Close()
		return res, nil
	}
	defer sg.Close()
	return s.install(st, sg, res)
}

// Confirm — «Заменить версией N» на вопрос AddPath.
func (s *Service) Confirm(token string) (res Add, err error) {
	defer appkit.GuardErr("ConfirmPack", &err)
	res = newAdd()
	st, err := s.h.Store()
	if err != nil {
		return res, err
	}
	if !s.lock() {
		return res, ErrBusy
	}
	defer s.unlock()
	pp := s.take(token)
	if pp == nil {
		return res, ErrStale
	}
	defer pp.st.Close()
	res.FromVersion = pp.st.From
	return s.install(st, pp.st, res)
}

// install — запись набора; держит файлы, нужные открытому альбому (обновление их не уносит).
func (s *Service) install(st *decor.Store, sg *decor.Staged, res Add) (Add, error) {
	info, err := st.InstallPack(sg, s.h.AlbumLibs())
	if err != nil {
		return res, err
	}
	res.Pack, res.Ask, res.Token = Info(info), "", ""
	// Поставленный из файла снова получает обновления из сети (решение 16).
	if slices.Contains(prefs.LibraryNetOf(s.h.CfgDir()).Skip, info.ID) {
		prefs.UpdateLibraryNet(s.h.CfgDir(), func(n *prefs.LibraryNet) { n.SetSkip(info.ID, false) })
	}
	s.h.Relink(st)
	s.emitChanged(info.Title)
	return res, nil
}

// PlanRemove — числа для вопроса «Удалить набор?».
func (s *Service) PlanRemove(id string) (r Remove, err error) {
	defer appkit.GuardErr("PlanRemovePack", &err)
	st, err := s.h.Store()
	if err != nil {
		return r, err
	}
	plan, err := st.PlanRemovePack(id, s.h.AlbumLibs())
	return Remove(plan), err
}

// Remove — удаление набора (решение 10): файлы, нужные открытому альбому, своим и другим
// наборам, остаются; альбом не меняется — шага отмены нет.
func (s *Service) Remove(id string) (list List, err error) {
	defer appkit.GuardErr("RemovePack", &err)
	st, err := s.h.Store()
	if err != nil {
		return List{Packs: []Info{}}, err
	}
	if !s.lock() {
		list, _ = s.List()
		return list, ErrBusy
	}
	s.dropPending()
	// origin — до удаления: после него набора нет. Пропуск отвечает только на «почему сеть не
	// вернула набор» (0.17 решение №4) и пишется под замком: иначе «Обновить» между снятием
	// замка и записью вернул бы удалённый.
	fromNet := slices.ContainsFunc(st.Packs(), func(p decor.PackInfo) bool { return p.ID == id && p.Origin == decor.OriginNet })
	_, err = st.RemovePack(id, s.h.AlbumLibs())
	if err == nil && fromNet {
		prefs.UpdateLibraryNet(s.h.CfgDir(), func(n *prefs.LibraryNet) { n.SetSkip(id, true) })
	}
	s.unlock()
	if err == nil {
		s.emitChanged() // удаление: названий нет, окно тему не меняет
	}
	list, lerr := s.List()
	if err == nil {
		err = lerr
	}
	return list, err
}

// ShowFolder открывает папку библиотеки в Проводнике; нет папки — сначала создаёт.
// Ошибка без пути: интерфейс показывает её человеку.
func (s *Service) ShowFolder() error {
	st, err := s.h.Store()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(st.Root(), 0o700); err != nil {
		return errors.New("папка библиотеки недоступна")
	}
	if err := s.h.OpenDir(st.Root()); err != nil {
		return errors.New("не удалось открыть папку библиотеки")
	}
	return nil
}

// packEmptyErr — «нечего ставить» с первой причиной: Wails с ошибкой отдаёт только текст, и
// Skipped до тоста не доходит. Имена — предметов или записей архива, путей диска в них нет.
func packEmptyErr(err error, skips []Skip) error {
	if !errors.Is(err, decor.ErrPackEmpty) || len(skips) == 0 {
		return err
	}
	msg := fmt.Sprintf("%s — %s", skips[0].Name, skips[0].Why)
	if n := len(skips) - 1; n > 0 {
		msg += fmt.Sprintf(" (и ещё %d)", n)
	}
	return fmt.Errorf("%w: %s", decor.ErrPackEmpty, msg)
}

// decorManifest — манифест pack в decor (поля как есть: проверены ParseManifest).
func decorManifest(m pack.Manifest) decor.PackManifest {
	out := decor.PackManifest{ID: m.ID, Version: m.Version, MinApp: m.MinApp, Title: m.Title, About: m.About, Author: m.Author,
		License: m.License, Source: m.Source, Items: make([]decor.PackItem, len(m.Items))}
	for i, it := range m.Items {
		out.Items[i] = decor.PackItem{File: it.File, Kind: it.Kind, Theme: it.Theme, Name: it.Name, Size: it.Size}
	}
	return out
}

func packSkips(in []pack.Skip) []Skip {
	out := make([]Skip, 0, len(in))
	for _, k := range in {
		out = append(out, Skip{Name: k.File, Why: k.Why})
	}
	return out
}
