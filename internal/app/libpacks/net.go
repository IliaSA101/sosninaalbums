package libpacks

import (
	"context"
	"errors"
	"log"
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

// ResetSkip — забыть удалённые наборы: они снова обычные строки «Можно скачать», «Скачать все»
// и «Обновить» их не пропускают. В сеть не ходит, согласия не требует; пусто — config.json не
// переписывается. Пример в пропуске (142): замок как у «Удалить» (занято — ErrBusy, пропуск не
// тронут), под ним пропуск снимается и пример встаёт из файла сразу (DemoBack); файла нет — строка
// «Скачать» снова видна.
func (s *Service) ResetSkip() (n Net, err error) {
	defer appkit.GuardErr("ResetLibrarySkip", &err)
	drop := func(c *prefs.LibraryNet) { c.Skip = nil }
	cur := prefs.LibraryNetOf(s.h.CfgDir())
	if len(cur.Skip) == 0 {
		return s.netFor(cur), nil
	}
	d := s.h.Demo()
	if d.ID == "" || !slices.Contains(cur.Skip, d.ID) {
		return s.netFor(prefs.UpdateLibraryNet(s.h.CfgDir(), drop)), nil
	}
	if !s.lock() {
		return s.netFor(cur), ErrBusy
	}
	n, placed, changed := s.resetSkipLocked(d, drop)
	n.DemoBack = placed
	if changed {
		if st, serr := s.h.Store(); serr == nil {
			s.h.Relink(st)
		}
		s.emitChanged()
	}
	return n, nil
}

// resetSkipLocked — снятие пропуска и пример под замком (он уже взят и снимается здесь отложенно,
// паника не оставит службу занятой). placed — пример встал; changed — состав библиотеки изменился.
func (s *Service) resetSkipLocked(d Demo, drop func(*prefs.LibraryNet)) (n Net, placed, changed bool) {
	defer s.unlock()
	st, serr := s.h.Store()
	got := prefs.UpdateLibraryNet(s.h.CfgDir(), drop)
	var retired bool
	if serr == nil {
		placed = s.placeLocked(st, d)
		retired = s.retireDemo(st)
	}
	return s.netWith(got, st), placed, placed || retired
}

// SetNet — согласие и «Проверять при запуске»; отзыв (allowed = false) снимает и флажок
// и отменяет идущее скачивание. Порядок: сначала запись, потом stop — операция, не успевшая
// встать в begin, перечитает согласие после него и не пойдёт в сеть (0.17 решение №2).
func (s *Service) SetNet(allowed, atStart bool) (n Net, err error) {
	defer appkit.GuardErr("SetLibraryNet", &err)
	got := prefs.UpdateLibraryNet(s.h.CfgDir(), func(c *prefs.LibraryNet) {
		c.Allowed, c.AtStart = allowed, atStart && allowed
	})
	s.setConsent(s.h.CfgDir(), got.Allowed)
	s.h.AfterNetWrite()
	if !allowed {
		s.stop()
		s.stopPreviews() // образцы — своя отмена: «Отменить» окна их не трогает (144 Н4)
	}
	return s.netFor(got), nil
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

// Update — «Обновить библиотеку»: каталог, затем новые версии стоящих наборов и дополнения к
// стоящему донору, которых нет. Новые наборы не ставятся — их объявляет раздел «Можно скачать»
// (137). Ошибка сети — библиотека прежняя, поставленные до ошибки наборы остаются.
func (s *Service) Update() (res Update, err error) {
	defer appkit.GuardErr("UpdateLibrary", &err)
	res, _, err = s.run(op{kind: opUpdate})
	return res, err
}

// Download — «Скачать» в строке раздела: набор id с нестоящими донорами вверх и годными
// дополнениями вниз по свежему каталогу (pick). id не по грамматике или нет в свежем списке —
// ErrGone, до сети в первом случае.
func (s *Service) Download(id string) (res Update, err error) {
	defer appkit.GuardErr("DownloadLibraryPack", &err)
	if !pack.ValidID(id) {
		return newUpdate(), ErrGone
	}
	o := op{kind: opPick, id: id}
	if d := s.h.Demo(); d.ID != "" && id == d.ID {
		o = op{kind: opDemo, id: id} // пример — не строка каталога: без каталога и дополнений
	}
	res, _, err = s.run(o)
	return res, err
}

// DownloadAll — «Скачать все»: ровно строки раздела, которые видел пользователь (сохранённый
// список до операции); обновления стоящих — дело «Обновить».
func (s *Service) DownloadAll() (res Update, err error) {
	defer appkit.GuardErr("DownloadAllLibrary", &err)
	res, _, err = s.run(op{kind: opAll})
	return res, err
}

// run — общий вход трёх кнопок: согласие, библиотека, только чтение, замок, сеть.
func (s *Service) run(o op) (res Update, start Start, err error) {
	res = newUpdate()
	if !prefs.LibraryNetOf(s.h.CfgDir()).Allowed {
		return res, start, ErrNoConsent
	}
	st, err := s.h.Store()
	if err != nil {
		return res, start, err
	}
	// Только чтение: InstallPack всё равно откажет — без трафика и до первого запроса.
	if st.ReadOnly() {
		return res, start, decor.ErrReadOnly
	}
	if !s.lock() {
		return res, start, ErrBusy
	}
	defer s.unlock()
	s.dropPending()
	ctx, done, err := s.beginNet()
	if err != nil {
		return res, start, err
	}
	defer done()
	if o.kind == opDemo {
		res, err = s.downloadDemo(ctx, st)
		return res, start, err
	}
	return s.update(ctx, st, o)
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
// тесты — сразу): обновляет стоящие наборы, новое объявляет событием library:start (одно на
// проверку: тост один, следующий стирает прежний); обновления больше порога — только
// предложение. Ошибки и неудачи отдельных наборов — только в log.txt (решение 17). Согласие
// сверяется заново: могли отозвать за 15 с.
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
	// Ошибка уже в log.txt, но сохранённый список переписан и новое при следующем запуске не
	// «новое»: что успело сосчитаться (обновлено, новых наборов), объявляем и при обрыве хода.
	_, start, uerr := s.update(ctx, st, op{kind: opStart, offerOver: offerOver})
	if start.Updated > 0 || start.Offer.Count > 0 || start.Fresh.Count > 0 {
		s.h.Emit("library:start", start) // тост собирает фронт, обычный, не красный (073-2)
	}
	// Компактная редакция: пример докачивается в этой же проверке (под тем же замком и раз в срок),
	// без тоста; сеть не отвечала — не пробуем (142, 0.30-142-41).
	if uerr == nil {
		s.startDemo(ctx, st)
	}
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
