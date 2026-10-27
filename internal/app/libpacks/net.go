package libpacks

import (
	"context"
	"errors"
	"log"
	"os"
	"slices"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// «Обновить библиотеку» (спека 067 Ф3, design §5): каталог и архивы — decor/fetch (единственный
// сетевой клиент), разбор архива — pack, установка — тот же путь, что «Добавить из файла»
// (StagePack → InstallPack, origin net). Без согласия (libraryNet.allowed) fetch не вызывается.

// emitChanged шлёт library:changed с названиями; nil превращается в пустой список.
func (s *Service) emitChanged(titles ...string) {
	if titles == nil {
		titles = []string{}
	}
	s.h.PrintsChanged() // набор мог принести или унести типографию: реестр — до события окна
	s.h.Emit("library:changed", Changed{Installed: titles})
}

// whyFromFile — набор стоит из файла (или встроенный): сеть его не заменяет (0.17 решение №2).
const whyFromFile = "стоит из файла — обновите из файла или удалите"

// whyBuiltin — id строки каталога занят встроенным: «из файла» вводило бы в заблуждение.
const whyBuiltin = "совпадает со встроенным набором"

// begin — контекст операции от stop (выход программы); Cancel отменяет его.
func (s *Service) begin(stop context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(stop)
	s.netMu.Lock()
	s.cancel = cancel
	s.netMu.Unlock()
	return ctx, func() {
		s.netMu.Lock()
		s.cancel = nil
		s.netMu.Unlock()
		cancel()
	}
}

func (s *Service) stop() {
	s.netMu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	s.netMu.Unlock()
}

func netView(n prefs.LibraryNet) Net {
	return Net{Allowed: n.Allowed, AtStart: n.AtStart, Checked: n.Checked, Skipped: len(n.Skip)}
}

// ResetSkip — забыть удалённые наборы: следующее «Обновить библиотеку» вернёт их. В сеть
// не ходит, согласия не требует; пусто — config.json не переписывается.
func (s *Service) ResetSkip() (n Net, err error) {
	defer appkit.GuardErr("ResetLibrarySkip", &err)
	if cur := prefs.LibraryNetOf(s.h.CfgDir()); len(cur.Skip) == 0 {
		return netView(cur), nil
	}
	return netView(prefs.UpdateLibraryNet(s.h.CfgDir(), func(c *prefs.LibraryNet) { c.Skip = nil })), nil
}

// SetNet — согласие и «Проверять при запуске»; отзыв (allowed = false) снимает и флажок
// и отменяет идущее скачивание. Порядок: сначала запись, потом stop — операция, не успевшая
// встать в begin, перечитает согласие после него и не пойдёт в сеть (0.17 решение №2).
func (s *Service) SetNet(allowed, atStart bool) (n Net, err error) {
	defer appkit.GuardErr("SetLibraryNet", &err)
	got := prefs.UpdateLibraryNet(s.h.CfgDir(), func(c *prefs.LibraryNet) {
		c.Allowed, c.AtStart = allowed, atStart && allowed
	})
	s.h.AfterNetWrite()
	if !allowed {
		s.stop()
	}
	return netView(got), nil
}

// beginNet — операция в begin и повторная сверка согласия уже после регистрации: отзыв между
// первой проверкой и begin иначе не отменил бы ничего. Без согласия — ErrNoConsent, ни одного
// запроса.
func (s *Service) beginNet() (context.Context, func(), error) {
	s.h.BeforeBegin()
	ctx, done := s.begin(s.h.StopCtx())
	if !prefs.LibraryNetOf(s.h.CfgDir()).Allowed {
		done()
		return nil, nil, ErrNoConsent
	}
	return ctx, done, nil
}

// Cancel — «Отменить»: идущее обновление обрывается, поставленные до него наборы остаются.
func (s *Service) Cancel() { s.stop() }

// Update — «Обновить библиотеку»: каталог, затем новые наборы и новые версии стоящих по
// одному. Ошибка сети — библиотека прежняя, поставленные до ошибки наборы остаются.
func (s *Service) Update() (res Update, err error) {
	defer appkit.GuardErr("UpdateLibrary", &err)
	res = Update{Skipped: []Skip{}}
	if !prefs.LibraryNetOf(s.h.CfgDir()).Allowed {
		return res, ErrNoConsent
	}
	st, err := s.h.Store()
	if err != nil {
		return res, err
	}
	// Только чтение: InstallPack всё равно откажет — без трафика и до первого запроса.
	if st.ReadOnly() {
		return res, decor.ErrReadOnly
	}
	if !s.lock() {
		return res, ErrBusy
	}
	defer s.unlock()
	s.dropPending()
	ctx, done, err := s.beginNet()
	if err != nil {
		return res, err
	}
	defer done()
	res, _, err = s.update(ctx, st, 0)
	return res, err
}

// ScheduleCheck — проверка при запуске (OnDomReady): только с согласием и флажком, не чаще
// every (часы ушли назад — тоже пора), раз за сеанс (перезагрузка окна не повторяет), через
// delay — окно уже открыто и работа не ждёт сети.
func (s *Service) ScheduleCheck() {
	delay, every, _ := s.h.Start()
	n := prefs.LibraryNetOf(s.h.CfgDir())
	since := time.Now().UnixMilli() - n.Checked
	if !n.Allowed || !n.AtStart || since >= 0 && since < every.Milliseconds() {
		return
	}
	if !s.started.CompareAndSwap(false, true) {
		return
	}
	if !s.h.GoBG(func() {
		defer appkit.Guard("libraryCheck")
		t := time.NewTimer(delay)
		defer t.Stop()
		select {
		case <-t.C:
		case <-s.h.StopCtx().Done():
			return
		}
		s.StartCheck()
	}) {
		s.started.Store(false) // фон закрыт: проверка не запущена, раз за сеанс не потрачен
	}
}

// StartCheck — проверка при запуске без задержки (ScheduleCheck зовёт её после delay, сквозные
// тесты — сразу): молча, как кнопка; новое больше порога — library:offer без скачивания;
// ошибки — только в log.txt (решение 17). Согласие сверяется заново: могли отозвать за 15 с.
func (s *Service) StartCheck() {
	st, err := s.h.Store()
	if err != nil || !prefs.LibraryNetOf(s.h.CfgDir()).Allowed {
		return
	}
	// Только чтение — как неудача: checked не пишется, started не даёт повторить в этом сеансе.
	if st.ReadOnly() {
		log.Printf("библиотека: проверка при запуске пропущена — библиотека только для чтения")
		return
	}
	if !s.lock() {
		log.Printf("библиотека: проверка при запуске пропущена — идёт другая операция")
		return
	}
	defer s.unlock()
	ctx, done, err := s.beginNet()
	if err != nil {
		return
	}
	defer done()
	_, _, offerOver := s.h.Start()
	res, offer, err := s.update(ctx, st, offerOver)
	switch {
	case err != nil:
		return // уже в log.txt
	case offer.Count > 0:
		s.h.Emit("library:offer", offer)
	case res.Added+res.Updated > 0:
		s.h.Emit("library:updated", res) // тост собирает фронт, обычный, не красный (073-2)
	}
}

// update — каталог → план → наборы по одному. offerOver > 0 и новое больше него — только
// предложение (offer), без скачивания. checked пишется после удачной проверки, seq каталога —
// сразу после проверки подписи (меньший потом не примется).
func (s *Service) update(ctx context.Context, st *decor.Store, offerOver int64) (res Update, offer Offer, err error) {
	res = Update{Skipped: []Skip{}}
	prog := s.progress()
	c := s.h.Fetcher()
	prog("catalog", "", 0, 0)
	cat, err := c.Catalog(ctx, prefs.LibraryNetOf(s.h.CfgDir()).CatalogSeq)
	if err != nil {
		logNetErr("проверка не удалась", err)
		return res, offer, err
	}
	prefs.UpdateLibraryNet(s.h.CfgDir(), func(n *prefs.LibraryNet) { n.CatalogSeq = max(n.CatalogSeq, cat.Seq) })
	for _, k := range cat.Skipped {
		log.Printf("библиотека: строка каталога %s пропущена — %s", k.ID, k.Why)
	}
	todo, need := s.netPlan(st, cat, &res)
	if offerOver > 0 && need > offerOver {
		s.markChecked()
		log.Printf("библиотека: проверка, новых %d (%d МБ) — предложено", len(todo), need>>20)
		return res, Offer{Count: len(todo), Bytes: need}, nil
	}
	defer func() {
		if res.Added+res.Updated > 0 {
			s.h.Relink(st)
			s.emitChanged(res.titles...)
		}
	}()
	for _, e := range todo {
		// Донор мог не встать (архив не тот, нечего ставить, поставлен из файла старее): план
		// считал его годным. Без донора шаблоны дополнения ссылаются на несуществующие картинки;
		// причина уже в итоге у самого донора, у дополнения — только log.txt.
		if e.Needs != "" && !hasPack(st.Packs(), e.Needs) {
			log.Printf("библиотека: дополнение %s пропущено — донор %s не поставлен", e.ID, e.Needs)
			continue
		}
		if err = s.netInstall(ctx, st, c, e, &res, prog); err != nil {
			logNetErr("набор "+e.ID+" не поставлен", err)
			return res, offer, err
		}
	}
	s.markChecked()
	log.Printf("библиотека: итог — новых %d, обновлено %d, пропущено %d", res.Added, res.Updated, len(res.Skipped))
	return res, offer, nil
}

// netPlan — что ставить: нет у нас или версия новее; удалённые пользователем (skip) — нет;
// новее стоящего не из сети (из файла) и minApp новее программы — пропуск с причиной. Дополнение
// (needs) без донора — без причины и без тоста: донор в skip, не стоит и не годен к плану
// (иначе каждое «Обновить» качало бы его заново). Порядок строк каталога значения не имеет:
// донор идёт в todo раньше своих дополнений в любом месте списка. need — байты архивов.
func (s *Service) netPlan(st *decor.Store, cat fetch.Catalog, res *Update) (todo []fetch.Entry, need int64) {
	skip := prefs.LibraryNetOf(s.h.CfgDir()).Skip
	have := map[string]decor.PackInfo{}
	for _, p := range st.Packs() {
		have[p.ID] = p
	}
	app := s.h.Version()
	type cand struct {
		e   fetch.Entry
		ok  bool   // годен к плану (до проверки донора)
		why string // причина пропуска; пусто при ok — молча
	}
	var cs []cand
	for _, e := range cat.Packs {
		p, ok := have[e.ID]
		switch {
		case slices.Contains(skip, e.ID), e.Needs != "" && slices.Contains(skip, e.Needs):
			continue
		case ok && p.Origin == decor.OriginBuiltin:
			// До сверки версий: встроенный — версия 1, строка версии 1 иначе пропала бы молча.
			cs = append(cs, cand{e: e, why: whyBuiltin})
		case ok && p.Version >= e.Version:
			cs = append(cs, cand{e: e})
		case ok && p.Origin != decor.OriginNet:
			cs = append(cs, cand{e: e, why: whyFromFile})
		case e.NeedsApp(app):
			cs = append(cs, cand{e: e, why: "нужна версия Sosnina Albums " + e.MinApp + " или новее"})
		default:
			cs = append(cs, cand{e: e, ok: true})
		}
	}
	// Донор: уже стоит или годен к плану. Проходы до неподвижной точки — цепочка дополнений
	// в любом порядке строк; цикл needs (a↔b) не берётся ни одним.
	planned := map[string]bool{}
	placed := make([]bool, len(cs))
	donorOK := func(e fetch.Entry) bool {
		_, stands := have[e.Needs]
		return e.Needs == "" || planned[e.Needs] || stands
	}
	for moved := true; moved; {
		moved = false
		for i, c := range cs {
			if placed[i] || !c.ok || !donorOK(c.e) {
				continue
			}
			placed[i], moved = true, true
			planned[c.e.ID] = true
			todo = append(todo, c.e)
			need += c.e.Size
		}
	}
	// Причины — в порядке каталога; дополнению без донора причина не нужна (молчим, как раньше).
	for _, c := range cs {
		if !c.ok && c.why != "" && donorOK(c.e) {
			res.Skipped = append(res.Skipped, Skip{Name: c.e.Title, Why: c.why})
		}
	}
	return todo, need
}

func hasPack(packs []decor.PackInfo, id string) bool {
	return slices.ContainsFunc(packs, func(p decor.PackInfo) bool { return p.ID == id })
}

// netInstall — один набор: квота по размеру из каталога до скачивания, скачивание в
// Наборы/.dl-<id>.tmp, проверка архива и установка путём «из файла». Набор, который эта
// версия не ставит (манифест, нечего ставить), — пропуск с причиной; сеть и диск — ошибка.
func (s *Service) netInstall(ctx context.Context, st *decor.Store, c *fetch.Client, e fetch.Entry, res *Update, prog progressFn) error {
	// Стоящая версия того же id при обновлении уходит: без вычета набор у предела не обновился
	// бы никогда. Точная сверка (общие ключи) — в InstallPack.
	var old int64
	for _, p := range st.Packs() {
		if p.ID == e.ID {
			old = p.Bytes
		}
	}
	if st.Used()-old+e.Size > decor.MaxLibraryLen {
		return decor.ErrLibFull
	}
	tmp, err := st.DownloadPath(e.ID)
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
	arc, err := pack.Open(tmp, s.h.Version())
	if err != nil {
		res.Skipped = append(res.Skipped, Skip{Name: e.Title, Why: err.Error()})
		return nil
	}
	defer arc.Close() //nolint:errcheck // чтение архива: ошибка закрытия ничего не теряет
	// Архив подлинный (sha256 из подписанного каталога), но строка каталога — о другом наборе.
	if arc.Manifest.ID != e.ID || arc.Manifest.Version != e.Version || arc.SHA256 != e.SHA256 {
		res.Skipped = append(res.Skipped, Skip{Name: e.Title, Why: "архив не совпадает со списком наборов"})
		return nil
	}
	res.Skipped = append(res.Skipped, packSkips(arc.Manifest.Skipped)...)
	sg, err := st.StagePack(decorManifest(arc.Manifest), arc, decor.OriginNet, arc.SHA256)
	if sg != nil {
		defer sg.Close()
		for _, k := range sg.Skipped {
			res.Skipped = append(res.Skipped, Skip(k))
		}
	}
	switch {
	case errors.Is(err, decor.ErrPackEmpty):
		res.Skipped = append(res.Skipped, Skip{Name: e.Title, Why: err.Error()})
		return nil
	case err != nil:
		return diskErr(e, err)
	case sg.Ask == decor.AskSame || sg.Ask == decor.AskOlder:
		return nil // поставили «из файла» за время скачивания
	}
	info, err := st.InstallPack(sg, s.h.AlbumLibs())
	if err != nil {
		return diskErr(e, err)
	}
	res.titles = append(res.titles, info.Title)
	if sg.Ask == decor.AskUpdate {
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

func (s *Service) markChecked() {
	now := time.Now().UnixMilli()
	prefs.UpdateLibraryNet(s.h.CfgDir(), func(n *prefs.LibraryNet) { n.Checked = now })
}

type progressFn func(stage, title string, done, total int64)

// progress — library:progress: смена этапа или набора и конец — сразу, ход байтов — не
// чаще progressGap.
func (s *Service) progress() progressFn {
	var last time.Time
	var key string
	return func(stage, title string, done, total int64) {
		now := time.Now()
		if k := stage + "\x00" + title; k != key || done >= total || now.Sub(last) >= progressGap {
			key, last = k, now
			s.h.Emit("library:progress", Progress{Stage: stage, Title: title, Done: done, Total: total})
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
