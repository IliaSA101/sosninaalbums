package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Наборы библиотеки (спека 067 Ф2, design §4, §6): «Добавить из файла», вопрос о замене,
// список и удаление. Разбор архива — decor/pack, содержимое и запись — decor.Store; App
// переносит манифест между ними (decor не импортирует pack — depguard) и держит одну
// операцию наборов за раз.

// PackInfo — строка окна «Наборы библиотеки» (decor.PackInfo).
type PackInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	About     string `json:"about"`
	Author    string `json:"author"`
	License   string `json:"license"`
	Source    string `json:"source"` // текст, не ссылка (решение 14)
	Origin    string `json:"origin"` // builtin | file | net
	Version   int    `json:"version"`
	Bg        int    `json:"bg"`
	Sticker   int    `json:"sticker"`
	Spread    int    `json:"spread"`
	Bytes     int64  `json:"bytes"`
	Removable bool   `json:"removable"`
}

// LibraryNet — согласие на сеть и проверка при запуске (prefs.LibraryNet без списка удалённых:
// наружу — только сколько их, для «Вернуть удалённые»).
type LibraryNet struct {
	Allowed bool  `json:"allowed"`
	AtStart bool  `json:"atStart"`
	Checked int64 `json:"checked"` // unix мс последней удачной проверки
	Skipped int   `json:"skipped"` // удалённых пользователем: «Обновить» их не ставит
}

// PackList — содержимое окна «Наборы»: Busy — идёт операция (кнопки неактивны).
type PackList struct {
	Packs    []PackInfo `json:"packs"`
	Net      LibraryNet `json:"net"`
	Busy     bool       `json:"busy"`
	ReadOnly bool       `json:"readOnly"`
}

// PackAdd — итог «Добавить из файла». Ask: "" — поставлен; same — эта версия уже стоит;
// update — стоит FromVersion, замена — ConfirmPack(Token); older — стоит новее, не ставится.
type PackAdd struct {
	Pack        PackInfo    `json:"pack"`
	Ask         string      `json:"ask"`
	FromVersion int         `json:"fromVersion"`
	Token       string      `json:"token"`
	Skipped     []DecorSkip `json:"skipped"`
}

// PackRemove — числа вопроса «Удалить набор?» (decor.PackRemove).
type PackRemove struct {
	Files int   `json:"files"`
	Kept  int   `json:"kept"`
	Bytes int64 `json:"bytes"`
}

// packTTL — сколько ждёт ответа вопрос «Заменить набор?»: распакованное держит место на диске.
const packTTL = 10 * time.Minute

// dirPack — своя последняя папка у диалога «Добавить из файла».
const dirPack = "pack"

var (
	errPackBusy  = errors.New("библиотека уже обновляется")
	errPackStale = errors.New("вопрос устарел — добавьте набор ещё раз")
	errPackFile  = errors.New("это не набор библиотеки (.zip)")
	// errPackRemote — без пути: ошибка CheckLocalPath несёт его целиком, а тост его не показывает.
	errPackRemote = errors.New("файл набора не на локальном диске")
)

// packState — одна операция наборов за раз и распакованный набор, ждущий «Заменить».
type packState struct {
	mu   sync.Mutex // TryLock: вторая операция — errPackBusy, а не ожидание
	busy atomic.Bool

	pendMu  sync.Mutex
	pending *pendingPack
}

type pendingPack struct {
	token string
	st    *decor.Staged
	until time.Time
}

// lock — начало операции наборов; false — уже идёт другая.
func (p *packState) lock() bool {
	if !p.mu.TryLock() {
		return false
	}
	p.busy.Store(true)
	return true
}

func (p *packState) unlock() {
	p.busy.Store(false)
	p.mu.Unlock()
}

// setPending — новый вопрос вытесняет прежний; через packTTL распакованное удаляется само.
func (p *packState) setPending(st *decor.Staged) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	pp := &pendingPack{token: hex.EncodeToString(b[:]), st: st, until: time.Now().Add(packTTL)}
	p.pendMu.Lock()
	old := p.pending
	p.pending = pp
	p.pendMu.Unlock()
	if old != nil {
		old.st.Close()
	}
	time.AfterFunc(packTTL, func() {
		if p.take(pp.token) != nil {
			pp.st.Close()
		}
	})
	return pp.token
}

// take — ждущий набор по токену (и снимает его); не тот или просрочен — nil.
func (p *packState) take(token string) *pendingPack {
	p.pendMu.Lock()
	defer p.pendMu.Unlock()
	pp := p.pending
	if pp == nil || token == "" || pp.token != token {
		return nil
	}
	p.pending = nil
	if time.Now().After(pp.until) {
		pp.st.Close()
		return nil
	}
	return pp
}

// dropPending — следующая операция отменяет неотвеченный вопрос (design §4.3).
func (p *packState) dropPending() {
	p.pendMu.Lock()
	pp := p.pending
	p.pending = nil
	p.pendMu.Unlock()
	if pp != nil {
		pp.st.Close()
	}
}

func newPackAdd() PackAdd { return PackAdd{Skipped: []DecorSkip{}} }

// LibraryPacks — список наборов: встроенные и поставленные.
func (a *App) LibraryPacks() (list PackList, err error) {
	defer guardErr("LibraryPacks", &err)
	list = PackList{Packs: []PackInfo{}, Busy: a.packs.busy.Load(), Net: netView(prefs.LibraryNetOf(a.cfgDir))}
	s, err := a.decorStore()
	if err != nil {
		return list, err
	}
	for _, p := range s.Packs() {
		list.Packs = append(list.Packs, PackInfo(p))
	}
	list.ReadOnly = s.ReadOnly()
	return list, nil
}

// AddPackFile — «Добавить из файла…»: диалог *.zip в Go. Отказ от диалога — пустой итог.
func (a *App) AddPackFile() (res PackAdd, err error) {
	defer guardErr("AddPackFile", &err)
	res = newPackAdd()
	path, err := openFileDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Набор библиотеки",
		DefaultDirectory: a.dialogDir(dirPack),
		Filters:          []wr.FileFilter{{DisplayName: "Наборы библиотеки (*.zip)", Pattern: "*.zip"}},
	})
	if err != nil || path == "" {
		return res, err
	}
	res, err = a.AddPackPath(path)
	if err == nil {
		a.rememberDir(dirPack, filepath.Dir(path))
	}
	return res, err
}

// AddPackPath — набор по пути (как ImportDecorPaths: для проверки из браузера). Путь — только
// локальный обычный файл .zip. Поставленный набор находит пропавшие своего альбома (RelinkLib —
// шаг, если что-то нашлось) и шлёт library:changed.
func (a *App) AddPackPath(path string) (res PackAdd, err error) {
	defer guardErr("AddPackPath", &err)
	res = newPackAdd()
	s, err := a.decorStore()
	if err != nil {
		return res, err
	}
	if paths.CheckLocalPath(path) != nil { // ошибка с полным путём — в тост не идёт (как importOne)
		return res, errPackRemote
	}
	if !strings.EqualFold(filepath.Ext(path), ".zip") {
		return res, errPackFile
	}
	if fi, err := os.Stat(path); err != nil || !fi.Mode().IsRegular() {
		return res, errors.New("файл набора не найден")
	}
	if !a.packs.lock() {
		return res, errPackBusy
	}
	defer a.packs.unlock()
	a.packs.dropPending()
	arc, err := pack.Open(path, appVersion(wailsJSON))
	if err != nil {
		return res, err
	}
	defer arc.Close() //nolint:errcheck // чтение архива: ошибка закрытия ничего не теряет
	res.Skipped = packSkips(arc.Manifest.Skipped)
	st, err := s.StagePack(decorManifest(arc.Manifest), arc, decor.OriginFile, arc.SHA256)
	if st != nil {
		for _, k := range st.Skipped {
			res.Skipped = append(res.Skipped, DecorSkip(k))
		}
	}
	if err != nil {
		st.Close()
		return res, packEmptyErr(err, res.Skipped)
	}
	res.Pack, res.Ask, res.FromVersion = PackInfo(st.Info()), st.Ask, st.From
	if st.Ask == decor.AskUpdate {
		res.Token = a.packs.setPending(st) // распакованное ждёт «Заменить»
		return res, nil
	}
	if st.Ask != "" {
		st.Close()
		return res, nil
	}
	defer st.Close()
	return a.installPack(s, st, res)
}

// ConfirmPack — «Заменить версией N» на вопрос AddPackPath.
func (a *App) ConfirmPack(token string) (res PackAdd, err error) {
	defer guardErr("ConfirmPack", &err)
	res = newPackAdd()
	s, err := a.decorStore()
	if err != nil {
		return res, err
	}
	if !a.packs.lock() {
		return res, errPackBusy
	}
	defer a.packs.unlock()
	pp := a.packs.take(token)
	if pp == nil {
		return res, errPackStale
	}
	defer pp.st.Close()
	res.FromVersion = pp.st.From
	return a.installPack(s, pp.st, res)
}

// installPack — запись набора; держит файлы, нужные открытому альбому (обновление их не уносит).
func (a *App) installPack(s *decor.Store, st *decor.Staged, res PackAdd) (PackAdd, error) {
	info, err := s.InstallPack(st, a.albumLibs())
	if err != nil {
		return res, err
	}
	res.Pack, res.Ask, res.Token = PackInfo(info), "", ""
	// Поставленный из файла снова получает обновления из сети (решение 16).
	if slices.Contains(prefs.LibraryNetOf(a.cfgDir).Skip, info.ID) {
		prefs.UpdateLibraryNet(a.cfgDir, func(n *prefs.LibraryNet) { n.SetSkip(info.ID, false) })
	}
	a.relinkLib(s)
	a.emitChanged(info.Title)
	return res, nil
}

// PlanRemovePack — числа для вопроса «Удалить набор?».
func (a *App) PlanRemovePack(id string) (r PackRemove, err error) {
	defer guardErr("PlanRemovePack", &err)
	s, err := a.decorStore()
	if err != nil {
		return r, err
	}
	plan, err := s.PlanRemovePack(id, a.albumLibs())
	return PackRemove(plan), err
}

// RemovePack — удаление набора (решение 10): файлы, нужные открытому альбому, своим и другим
// наборам, остаются; альбом не меняется — шага отмены нет.
func (a *App) RemovePack(id string) (list PackList, err error) {
	defer guardErr("RemovePack", &err)
	s, err := a.decorStore()
	if err != nil {
		return PackList{Packs: []PackInfo{}}, err
	}
	if !a.packs.lock() {
		list, _ = a.LibraryPacks()
		return list, errPackBusy
	}
	a.packs.dropPending()
	// origin — до удаления: после него набора нет. Пропуск отвечает только на «почему сеть не
	// вернула набор» (0.17 решение №4) и пишется под замком: иначе «Обновить» между снятием
	// замка и записью вернул бы удалённый.
	fromNet := slices.ContainsFunc(s.Packs(), func(p decor.PackInfo) bool { return p.ID == id && p.Origin == decor.OriginNet })
	_, err = s.RemovePack(id, a.albumLibs())
	if err == nil && fromNet {
		prefs.UpdateLibraryNet(a.cfgDir, func(n *prefs.LibraryNet) { n.SetSkip(id, true) })
	}
	a.packs.unlock()
	if err == nil {
		a.emitChanged() // удаление: названий нет, окно тему не меняет
	}
	list, lerr := a.LibraryPacks()
	if err == nil {
		err = lerr
	}
	return list, err
}

// ShowLibraryFolder открывает папку библиотеки в Проводнике; нет папки — сначала создаёт.
// Ошибка без пути: интерфейс показывает её человеку.
func (a *App) ShowLibraryFolder() error {
	s, err := a.decorStore()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.Root(), 0o700); err != nil {
		return errors.New("папка библиотеки недоступна")
	}
	if err := openDir(s.Root()); err != nil {
		return errors.New("не удалось открыть папку библиотеки")
	}
	return nil
}

// albumLibs — ключи библиотеки открытого альбома, и в шагах отмены и возврата: иначе Ctrl+Y
// после удаления набора вернул бы фон или стикер без файла (ревью 067).
func (a *App) albumLibs() func(key string) bool {
	keys := a.editor().LibKeys()
	return func(key string) bool { return keys[key] }
}

// packEmptyErr — «нечего ставить» с первой причиной: Wails с ошибкой отдаёт только текст, и
// Skipped до тоста не доходит. Имена — предметов или записей архива, путей диска в них нет.
func packEmptyErr(err error, skips []DecorSkip) error {
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
	out := decor.PackManifest{ID: m.ID, Version: m.Version, Title: m.Title, About: m.About, Author: m.Author,
		License: m.License, Source: m.Source, Items: make([]decor.PackItem, len(m.Items))}
	for i, it := range m.Items {
		out.Items[i] = decor.PackItem{File: it.File, Kind: it.Kind, Theme: it.Theme, Name: it.Name, Size: it.Size}
	}
	return out
}

func packSkips(in []pack.Skip) []DecorSkip {
	out := make([]DecorSkip, 0, len(in))
	for _, k := range in {
		out = append(out, DecorSkip{Name: k.File, Why: k.Why})
	}
	return out
}
