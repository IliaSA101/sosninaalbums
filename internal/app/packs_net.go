package app

import (
	"context"
	"errors"
	"log"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// «Обновить библиотеку» (спека 067 Ф3, design §5): каталог и архивы — decor/fetch (единственный
// сетевой клиент), разбор архива — pack, установка — тот же путь, что «Добавить из файла»
// (StagePack → InstallPack, origin net). Без согласия (libraryNet.allowed) fetch не вызывается.

// LibraryUpdate — итог «Обновить библиотеку»: новых и обновлённых наборов, пропущенное с
// причиной (нужна новая программа, предметы набора), байты скачанного.
type LibraryUpdate struct {
	Added   int         `json:"added"`
	Updated int         `json:"updated"`
	Skipped []DecorSkip `json:"skipped"`
	Bytes   int64       `json:"bytes"`
	titles  []string    // названия поставленных и обновлённых — в library:changed; в JSON не идут
}

// libChanged — данные library:changed: названия (PackInfo.Title) поставленных и обновлённых
// наборов, чтобы окно «Фоны/Стикеры/Развороты» встало на тему нового (решение 073-1). При
// удалении — пустой список (не null).
type libChanged struct {
	Installed []string `json:"installed"`
}

// emitChanged шлёт library:changed с названиями; nil превращается в пустой список.
func (a *App) emitChanged(titles ...string) {
	if titles == nil {
		titles = []string{}
	}
	a.emit("library:changed", libChanged{Installed: titles})
}

// LibraryProgress — событие library:progress: этап catalog | download | install, набор и байты.
type LibraryProgress struct {
	Stage string `json:"stage"`
	Title string `json:"title"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

// LibraryOffer — событие library:offer: при запуске новое больше startOffer — тост «Скачать».
type LibraryOffer struct {
	Count int   `json:"count"`
	Bytes int64 `json:"bytes"`
}

// Проверка при запуске (решение 17); переменные — для тестов.
var (
	startDelay        = 15 * time.Second
	startEvery        = 20 * time.Hour
	startOffer  int64 = 100 << 20
	progressGap       = 250 * time.Millisecond // library:progress не чаще 4 раз в секунду
)

var errNoConsent = errors.New("обращение к сети не разрешено — нажмите «Обновить библиотеку» и разрешите")

// whyFromFile — набор стоит из файла (или встроенный): сеть его не заменяет (0.17 решение №2).
const whyFromFile = "стоит из файла — обновите из файла или удалите"

// whyBuiltin — id строки каталога занят встроенным: «из файла» вводило бы в заблуждение.
const whyBuiltin = "совпадает со встроенным набором"

// Крючки тестов гонки отзыва: beforeBegin — между первой проверкой согласия и begin;
// afterNetWrite — в SetLibraryNet между записью согласия и stop.
var beforeBegin, afterNetWrite = func() {}, func() {}

// libNet — отмена идущего обновления, проверка при запуске раз за сеанс и источник (тесты
// подменяют client своим сервером).
type libNet struct {
	mu      sync.Mutex
	cancel  context.CancelFunc
	started atomic.Bool
	client  func() *fetch.Client
}

func (n *libNet) fetcher() *fetch.Client {
	if n.client != nil {
		return n.client()
	}
	return fetch.New(fetch.Default())
}

// begin — контекст операции от stop (выход программы); CancelLibrary отменяет его.
func (n *libNet) begin(stop context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(stop)
	n.mu.Lock()
	n.cancel = cancel
	n.mu.Unlock()
	return ctx, func() {
		n.mu.Lock()
		n.cancel = nil
		n.mu.Unlock()
		cancel()
	}
}

func (n *libNet) stop() {
	n.mu.Lock()
	if n.cancel != nil {
		n.cancel()
	}
	n.mu.Unlock()
}

func netView(n prefs.LibraryNet) LibraryNet {
	return LibraryNet{Allowed: n.Allowed, AtStart: n.AtStart, Checked: n.Checked, Skipped: len(n.Skip)}
}

// ResetLibrarySkip — забыть удалённые наборы: следующее «Обновить библиотеку» вернёт их. В сеть
// не ходит, согласия не требует; пусто — config.json не переписывается.
func (a *App) ResetLibrarySkip() (n LibraryNet, err error) {
	defer guardErr("ResetLibrarySkip", &err)
	if cur := prefs.LibraryNetOf(a.cfgDir); len(cur.Skip) == 0 {
		return netView(cur), nil
	}
	return netView(prefs.UpdateLibraryNet(a.cfgDir, func(c *prefs.LibraryNet) { c.Skip = nil })), nil
}

// SetLibraryNet — согласие и «Проверять при запуске»; отзыв (allowed = false) снимает и флажок
// и отменяет идущее скачивание. Порядок: сначала запись, потом stop — операция, не успевшая
// встать в begin, перечитает согласие после него и не пойдёт в сеть (0.17 решение №2).
func (a *App) SetLibraryNet(allowed, atStart bool) (n LibraryNet, err error) {
	defer guardErr("SetLibraryNet", &err)
	got := prefs.UpdateLibraryNet(a.cfgDir, func(c *prefs.LibraryNet) {
		c.Allowed, c.AtStart = allowed, atStart && allowed
	})
	afterNetWrite()
	if !allowed {
		a.net.stop()
	}
	return netView(got), nil
}

// beginNet — операция в begin и повторная сверка согласия уже после регистрации: отзыв между
// первой проверкой и begin иначе не отменил бы ничего. Без согласия — errNoConsent, ни одного
// запроса.
func (a *App) beginNet() (context.Context, func(), error) {
	beforeBegin()
	ctx, done := a.net.begin(a.stopCtx)
	if !prefs.LibraryNetOf(a.cfgDir).Allowed {
		done()
		return nil, nil, errNoConsent
	}
	return ctx, done, nil
}

// CancelLibrary — «Отменить»: идущее обновление обрывается, поставленные до него наборы остаются.
func (a *App) CancelLibrary() { a.net.stop() }

// UpdateLibrary — «Обновить библиотеку»: каталог, затем новые наборы и новые версии стоящих по
// одному. Ошибка сети — библиотека прежняя, поставленные до ошибки наборы остаются.
func (a *App) UpdateLibrary() (res LibraryUpdate, err error) {
	defer guardErr("UpdateLibrary", &err)
	res = LibraryUpdate{Skipped: []DecorSkip{}}
	if !prefs.LibraryNetOf(a.cfgDir).Allowed {
		return res, errNoConsent
	}
	s, err := a.decorStore()
	if err != nil {
		return res, err
	}
	// Только чтение: InstallPack всё равно откажет — без трафика и до первого запроса.
	if s.ReadOnly() {
		return res, decor.ErrReadOnly
	}
	if !a.packs.lock() {
		return res, errPackBusy
	}
	defer a.packs.unlock()
	a.packs.dropPending()
	ctx, done, err := a.beginNet()
	if err != nil {
		return res, err
	}
	defer done()
	res, _, err = a.updateLibrary(ctx, s, 0)
	return res, err
}

// scheduleLibraryCheck — проверка при запуске (OnDomReady): только с согласием и флажком, не
// чаще startEvery (часы ушли назад — тоже пора), раз за сеанс (перезагрузка окна не повторяет),
// через startDelay — окно уже открыто и работа не ждёт сети.
func (a *App) scheduleLibraryCheck() {
	n := prefs.LibraryNetOf(a.cfgDir)
	since := time.Now().UnixMilli() - n.Checked
	if !n.Allowed || !n.AtStart || since >= 0 && since < startEvery.Milliseconds() {
		return
	}
	if !a.net.started.CompareAndSwap(false, true) {
		return
	}
	a.bg.Go(func() {
		defer guard("libraryCheck")
		t := time.NewTimer(startDelay)
		defer t.Stop()
		select {
		case <-t.C:
		case <-a.stopCtx.Done():
			return
		}
		a.startCheck()
	})
}

// startCheck — молча, как кнопка; новое больше startOffer — library:offer без скачивания;
// ошибки — только в log.txt (решение 17). Согласие сверяется заново: могли отозвать за 15 с.
func (a *App) startCheck() {
	s, err := a.decorStore()
	if err != nil || !prefs.LibraryNetOf(a.cfgDir).Allowed {
		return
	}
	// Только чтение — как неудача: checked не пишется, started не даёт повторить в этом сеансе.
	if s.ReadOnly() {
		log.Printf("библиотека: проверка при запуске пропущена — библиотека только для чтения")
		return
	}
	if !a.packs.lock() {
		log.Printf("библиотека: проверка при запуске пропущена — идёт другая операция")
		return
	}
	defer a.packs.unlock()
	ctx, done, err := a.beginNet()
	if err != nil {
		return
	}
	defer done()
	res, offer, err := a.updateLibrary(ctx, s, startOffer)
	switch {
	case err != nil:
		return // уже в log.txt
	case offer.Count > 0:
		a.emit("library:offer", offer)
	case res.Added+res.Updated > 0:
		a.emit("library:updated", res) // тост собирает фронт, обычный, не красный (073-2)
	}
}

// updateLibrary — каталог → план → наборы по одному. offerOver > 0 и новое больше него — только
// предложение (offer), без скачивания. checked пишется после удачной проверки, seq каталога —
// сразу после проверки подписи (меньший потом не примется).
func (a *App) updateLibrary(ctx context.Context, s *decor.Store, offerOver int64) (res LibraryUpdate, offer LibraryOffer, err error) {
	res = LibraryUpdate{Skipped: []DecorSkip{}}
	prog := a.libProgress()
	c := a.net.fetcher()
	prog("catalog", "", 0, 0)
	cat, err := c.Catalog(ctx, prefs.LibraryNetOf(a.cfgDir).CatalogSeq)
	if err != nil {
		logNetErr("проверка не удалась", err)
		return res, offer, err
	}
	prefs.UpdateLibraryNet(a.cfgDir, func(n *prefs.LibraryNet) { n.CatalogSeq = max(n.CatalogSeq, cat.Seq) })
	for _, k := range cat.Skipped {
		log.Printf("библиотека: строка каталога %s пропущена — %s", k.ID, k.Why)
	}
	todo, need := a.netPlan(s, cat, &res)
	if offerOver > 0 && need > offerOver {
		a.markChecked()
		log.Printf("библиотека: проверка, новых %d (%d МБ) — предложено", len(todo), need>>20)
		return res, LibraryOffer{Count: len(todo), Bytes: need}, nil
	}
	defer func() {
		if res.Added+res.Updated > 0 {
			a.relinkLib(s)
			a.emitChanged(res.titles...)
		}
	}()
	for _, e := range todo {
		if err = a.netInstall(ctx, s, c, e, &res, prog); err != nil {
			logNetErr("набор "+e.ID+" не поставлен", err)
			return res, offer, err
		}
	}
	a.markChecked()
	log.Printf("библиотека: итог — новых %d, обновлено %d, пропущено %d", res.Added, res.Updated, len(res.Skipped))
	return res, offer, nil
}

// netPlan — что ставить: нет у нас или версия новее; удалённые пользователем (skip) — нет;
// новее стоящего не из сети (из файла) и minApp новее программы — пропуск с причиной. need —
// байты архивов к скачиванию.
func (a *App) netPlan(s *decor.Store, cat fetch.Catalog, res *LibraryUpdate) (todo []fetch.Entry, need int64) {
	skip := prefs.LibraryNetOf(a.cfgDir).Skip
	have := map[string]decor.PackInfo{}
	for _, p := range s.Packs() {
		have[p.ID] = p
	}
	app := appVersion(wailsJSON)
	for _, e := range cat.Packs {
		p, ok := have[e.ID]
		switch {
		case slices.Contains(skip, e.ID):
			continue
		case ok && p.Origin == decor.OriginBuiltin:
			// До сверки версий: встроенный — версия 1, строка версии 1 иначе пропала бы молча.
			res.Skipped = append(res.Skipped, DecorSkip{Name: e.Title, Why: whyBuiltin})
			continue
		case ok && p.Version >= e.Version:
			continue
		case ok && p.Origin != decor.OriginNet:
			res.Skipped = append(res.Skipped, DecorSkip{Name: e.Title, Why: whyFromFile})
			continue
		case e.NeedsApp(app):
			res.Skipped = append(res.Skipped, DecorSkip{Name: e.Title, Why: "нужна версия Sosnina Albums " + e.MinApp + " или новее"})
			continue
		}
		todo = append(todo, e)
		need += e.Size
	}
	return todo, need
}

// netInstall — один набор: квота по размеру из каталога до скачивания, скачивание в
// Наборы/.dl-<id>.tmp, проверка архива и установка путём «из файла». Набор, который эта
// версия не ставит (манифест, нечего ставить), — пропуск с причиной; сеть и диск — ошибка.
func (a *App) netInstall(ctx context.Context, s *decor.Store, c *fetch.Client, e fetch.Entry, res *LibraryUpdate, prog progressFn) error {
	// Стоящая версия того же id при обновлении уходит: без вычета набор у предела не обновился
	// бы никогда. Точная сверка (общие ключи) — в InstallPack.
	var old int64
	for _, p := range s.Packs() {
		if p.ID == e.ID {
			old = p.Bytes
		}
	}
	if s.Used()-old+e.Size > decor.MaxLibraryLen {
		return decor.ErrLibFull
	}
	tmp, err := s.DownloadPath(e.ID)
	if err != nil {
		return diskErr(e, err)
	}
	defer os.Remove(tmp) //nolint:errcheck // временный: нет файла — уже удалён
	prog("download", e.Title, 0, e.Size)
	err = c.Download(ctx, e, tmp, func(done int64) { prog("download", e.Title, done, e.Size) })
	if err != nil {
		return err
	}
	res.Bytes += e.Size
	prog("install", e.Title, 0, 0)
	arc, err := pack.Open(tmp, appVersion(wailsJSON))
	if err != nil {
		res.Skipped = append(res.Skipped, DecorSkip{Name: e.Title, Why: err.Error()})
		return nil
	}
	defer arc.Close() //nolint:errcheck // чтение архива: ошибка закрытия ничего не теряет
	// Архив подлинный (sha256 из подписанного каталога), но строка каталога — о другом наборе.
	if arc.Manifest.ID != e.ID || arc.Manifest.Version != e.Version || arc.SHA256 != e.SHA256 {
		res.Skipped = append(res.Skipped, DecorSkip{Name: e.Title, Why: "архив не совпадает со списком наборов"})
		return nil
	}
	res.Skipped = append(res.Skipped, packSkips(arc.Manifest.Skipped)...)
	st, err := s.StagePack(decorManifest(arc.Manifest), arc, decor.OriginNet, arc.SHA256)
	if st != nil {
		defer st.Close()
		for _, k := range st.Skipped {
			res.Skipped = append(res.Skipped, DecorSkip(k))
		}
	}
	switch {
	case errors.Is(err, decor.ErrPackEmpty):
		res.Skipped = append(res.Skipped, DecorSkip{Name: e.Title, Why: err.Error()})
		return nil
	case err != nil:
		return diskErr(e, err)
	case st.Ask == decor.AskSame || st.Ask == decor.AskOlder:
		return nil // поставили «из файла» за время скачивания
	}
	info, err := s.InstallPack(st, a.albumLibs())
	if err != nil {
		return diskErr(e, err)
	}
	res.titles = append(res.titles, info.Title)
	if st.Ask == decor.AskUpdate {
		res.Updated++
	} else {
		res.Added++
	}
	return nil
}

// diskErr — ошибка установки скачанного набора для тоста: квота как есть, предмет не сходится с
// манифестом (архив подлинный по sha каталога — ошибка издателя, не диска) — «скачался с
// ошибкой», остальное — постоянный текст без пути (fetch.ErrDisk); в log.txt — только тип:
// текст ошибки диска может нести путь.
func diskErr(e fetch.Entry, err error) error {
	if errors.Is(err, decor.ErrLibFull) || errors.Is(err, decor.ErrPackEmpty) {
		return err
	}
	if errors.Is(err, pack.ErrDamaged) {
		log.Printf("библиотека: набор %s — предмет не сходится с манифестом (%T)", e.ID, err)
		return fetch.Broken(e.Title)
	}
	log.Printf("библиотека: набор %s — ошибка установки (%T)", e.ID, err)
	return fetch.ErrDisk
}

func (a *App) markChecked() {
	now := time.Now().UnixMilli()
	prefs.UpdateLibraryNet(a.cfgDir, func(n *prefs.LibraryNet) { n.Checked = now })
}

type progressFn func(stage, title string, done, total int64)

// libProgress — library:progress: смена этапа или набора и конец — сразу, ход байтов — не
// чаще progressGap.
func (a *App) libProgress() progressFn {
	var last time.Time
	var key string
	return func(stage, title string, done, total int64) {
		now := time.Now()
		if k := stage + "\x00" + title; k != key || done >= total || now.Sub(last) >= progressGap {
			key, last = k, now
			a.emit("library:progress", LibraryProgress{Stage: stage, Title: title, Done: done, Total: total})
		}
	}
}

// logNetErr — строка log.txt: что делали и ошибка (без адресов: их в ошибках fetch нет); у
// ответа не 200 — код.
func logNetErr(what string, err error) {
	if errors.Is(err, fetch.ErrCancelled) {
		log.Printf("библиотека: обновление отменено")
		return
	}
	log.Printf("библиотека: %s — %v", what, err)
}
