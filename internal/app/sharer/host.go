// Package sharer — служба «Отправить по ссылке» (спека 146, design §3.3–3.5): проверки и вопросы,
// рендер страниц во временную папку, загрузка в сервис листалок по контракту /v1 (api/albums),
// публикация, правки срока и пароля, отзыв ссылки; записи о выгрузках — prefs.Shares, токен —
// secret.Store. Проект не меняется.
//
// Зовёт её internal/app (share.go — фасад). Wails, App и album службе недоступны (depguard): от
// проекта она берёт снимок share.Job, остальное — через Host (поля-функции над App, позднее связывание).
package sharer

import (
	"context"
	"errors"
	"log"
	"maps"
	"net/url"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/secret"
	"github.com/IliaSA101/sosninaalbums/internal/share"
)

// Host — что службе нужно от App. Функции и Store обязательны (New паникует с именем пустого).
type Host struct {
	Plan func() share.Job // снимок текущего альбома (share.Job): тяжёлый, обходит все фото
	// Ident — путь ("" — не сохранён) и отпечаток текущего альбома: дешёвый, без Plan.
	Ident func() (path, uid string)
	// Pin записывает отпечаток uid в файл альбома path, если его там ещё нет (альбом до 0.32).
	// fresh — «выгрузить как новый» (167): альбому выдаётся новый отпечаток без привязок, прежняя
	// запись остаётся за прежним. Возвращает отпечаток для записи; альбом сменился — ошибка.
	Pin func(path, uid string, fresh bool) (string, error)
	// Bind кладёт привязку к выгруженному альбому id в файл проекта (167-15): сверка path и uid,
	// запись — обычным автосохранением App под замком сохранения, не службой.
	Bind func(path, uid, id string) error
	// LinkRef — привязка проекта uid к альбому id (model.LinkRef; sharer не видит model).
	LinkRef    func(uid, id string) string
	Emit       func(name string, data ...any) // события share:progress и share:done
	GoBG       func(fn func()) bool           // фон под WaitGroup App; false — App закрывается
	StopCtx    func() context.Context         // контекст выхода программы: родитель выгрузки
	Ask        func(msg string) (bool, error) // вопрос «Да / Нет»
	ExportBusy func() bool                    // идёт экспорт: вместе с выгрузкой они не работают
	LiftMemory func() (restore func())        // снять предел памяти на время рендера
	CheckURL   func(raw string) error         // printprof.CheckURL: ссылка клиенту годна
	OpenURL    func(url string)               // адрес в браузере
	Fold       func(path string) string       // paths.Fold: ключ пути без учёта регистра там, где надо
	CfgDir     func() string                  // папка данных: config.json, projects.json; "" — нет
	CacheDir   func() string                  // папка кэша программы (<кэш>/<папка программы>); "" — нет
	// Store — один на процесс (контракт secret.System): память запасного пути и пауза после
	// тайм-аута живут в нём.
	Store secret.Store
	// AllowLoopback — адреса этого компьютера годятся (сборка стенда osx.DevBuild; в тестах true).
	AllowLoopback bool
	Version       string // версия программы: generator манифеста и User-Agent
}

// Service — одна выгрузка за раз, под mu.
type Service struct {
	h   Host
	now func() time.Time
	// wait — пауза между повторами; подменяется в тестах.
	wait func(ctx context.Context, d time.Duration) error
	// gap — не чаще одного share:progress за gap (кроме последнего).
	gap time.Duration
	// after — отложенная отправка хода (задний фронт gap); acked — крючок тестов: страница
	// подтверждена клиентом (done — счёт после неё).
	after func(d time.Duration, f func()) stopper
	acked func(done int)

	mu  sync.Mutex
	run *runState // идущая выгрузка; nil — нет
	// kp — сохранённые страницы оборванной выгрузки (kept.go); closing — Shutdown был: слот не заводится.
	kp      *kept
	closing bool
	net     bool // идёт правка или отзыв ссылки (Change, Revoke): выгрузка не стартует
	// netPath — путь альбома правки или отзыва; netStop — OnSaved положил на него другой проект:
	// поздняя запись Change и пометка «нет на сервере» не пишутся (netAlive).
	netPath string
	netStop bool
	netText string                         // отказ выгрузке, пока флаг net стоит (правка или поиск)
	mem     map[string]prefs.ProjectRecord // записи, которые не удалось записать на диск (ключ — uid)
	// del — записи, которые не удалось стереть на диске (ключ — uid): look их не видит, стереть
	// пробует снова (look, удачный putIf).
	del   map[string]struct{}
	lim   map[string]albums.Limits // пределы сервиса по origin из последнего Health
	fnd   map[string]findState     // итог поиска на сервисе за сессию (ключ — uid), find.go
	pr    *prefs.Projects
	prDir string
}

// runState — идущая выгрузка: снятый на старте путь (разбор В1) и остановка.
type runState struct {
	path   string
	cancel context.CancelFunc // nil, пока фон не запущен (идут вопросы)
	stop   bool               // путь перезаписан другим проектом (OnSaved): запись и старт запрещены
}

// New собирает службу; пустое поле Host — паника с его именем (как exporter.New).
func New(h Host) *Service {
	v := reflect.ValueOf(h)
	for i := range v.NumField() {
		switch f := v.Field(i); f.Kind() {
		case reflect.Func, reflect.Interface:
			if f.IsNil() {
				panic("sharer.New: не задано поле Host." + v.Type().Field(i).Name)
			}
		}
	}
	return &Service{h: h, now: time.Now, wait: sleep, gap: 100 * time.Millisecond, after: realAfter,
		mem: map[string]prefs.ProjectRecord{}, del: map[string]struct{}{}, lim: map[string]albums.Limits{}, fnd: map[string]findState{}}
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Busy — выгрузка идёт (для запрета экспорта).
func (s *Service) Busy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.run != nil
}

// Cancel — «Остановить»: контекст доходит до рендера и запросов; опубликованная прежняя версия
// остаётся, запись с id — тоже. Выгрузки нет — ничего.
func (s *Service) Cancel() {
	s.mu.Lock()
	var cancel context.CancelFunc
	if s.run != nil {
		cancel = s.run.cancel
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// userError — текст для пользователя как есть (заглавная буква в начале допустима).
type userError string

func (e userError) Error() string { return string(e) }

func fail(msg string) error { return userError(msg) }

// rules — проверки адресов для prefs (0.32-146-T3-1).
func (s *Service) rules() prefs.ShareRules {
	allow := s.h.AllowLoopback
	return prefs.ShareRules{
		ServiceURL: func(raw string) (string, error) { return share.ServiceURL(raw, allow) },
		CheckLink:  s.checkLink,
	}
}

// checkLink — ссылка клиенту годна: обычная (printprof.CheckURL) или, на стенде, адрес этого
// компьютера (design §3.3 п. 7). Длиннее albums.MaxURL — нет.
func (s *Service) checkLink(raw string) error {
	if raw == "" { // CheckURL("") пропускает пустое: ссылки нет — не годна
		return errors.New("ссылки нет")
	}
	if len(raw) > albums.MaxURL {
		return errors.New("ссылка длиннее предела")
	}
	if s.h.CheckURL(raw) == nil {
		return nil
	}
	return share.CheckLoopbackLink(raw, s.h.AllowLoopback)
}

// projects — записи выгрузок по uid; пересоздаются, если папка данных сменилась (startup ставит её позже New).
func (s *Service) projects() *prefs.Projects {
	dir := s.h.CfgDir()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pr == nil || s.prDir != dir {
		s.pr, s.prDir = prefs.NewProjects(dir, s.rules()), dir
	}
	return s.pr
}

// look — запись проекта uid как есть: сначала та, что не легла на диск (она новее), затем файл.
// Запись, которую не удалось стереть на диске (drop), не возвращается; стереть её пробуем снова —
// короткий сбой (замок антивируса) так рассасывается.
func (s *Service) look(uid string) (prefs.ProjectRecord, bool) {
	if uid == "" {
		return prefs.ProjectRecord{}, false
	}
	pr := s.projects() // берёт mu сам: до Lock
	s.mu.Lock()
	rec, ok := s.mem[uid]
	_, gone := s.del[uid]
	if gone {
		s.retryDel(pr, uid)
	}
	_, gone = s.del[uid]
	s.mu.Unlock()
	switch {
	case ok:
		return rec, true
	case gone:
		return prefs.ProjectRecord{}, false
	}
	return pr.Get(uid)
}

// get — запись проекта uid и её отношение к файлу path (167-12): own — запись привязана к этому
// пути. Не своя запись (копия, перенос, другой компьютер) не даёт ни пароля, ни названия, ни
// правки и отзыва — только вопрос «обновить или как новый».
func (s *Service) get(path, uid string) (rec prefs.ProjectRecord, had, own bool) {
	rec, had = s.look(uid)
	return rec, had, had && rec.Own(path)
}

// retryDel повторяет стирание записей с меткой del (только uids; пусто — все). Под mu: порядок
// замков тот же, что у putIf и drop (mu, затем замок projects.json).
func (s *Service) retryDel(pr *prefs.Projects, uids ...string) {
	if len(uids) == 0 {
		uids = slices.Collect(maps.Keys(s.del))
	}
	for _, u := range uids {
		if _, ok := s.del[u]; ok && pr.Delete(u) == nil {
			delete(s.del, u)
		}
	}
}

// put пишет запись. Не вышло записать на диск (projects.json новее программы, не прочитан, переполнен,
// не сохранился) — запись остаётся в памяти до выхода: ссылку видно и копируется, а ошибка
// возвращается (0.32-146-T3-7 п. 2, не теряем молча). Негодная запись (ErrShareRecord) в память
// не идёт: это ошибка программы, а не диска.
func (s *Service) put(uid string, rec prefs.ProjectRecord) error {
	_, err := s.putIf(uid, rec, nil)
	return err
}

// netAlive — сверка правки и отзыва для putIf: на путь не лёг другой проект (OnSaved). Под mu.
func (s *Service) netAlive() bool { return !s.netStop }

// own — сверка выгрузки для putIf: run ещё идёт, не остановлена (ctx) и не отменена сохранением
// другого проекта поверх пути (OnSaved). Зовётся под mu.
func (s *Service) own(ctx context.Context, run *runState) func() bool {
	return func() bool { return s.run == run && !run.stop && ctx.Err() == nil }
}

// putIf — put, который под mu сверяется с alive: запись, опоздавшая к смене проекта на пути, не
// пишется (wrote=false), и последний put хода не воскрешает запись прежнего альбома. nil — без сверки.
func (s *Service) putIf(uid string, rec prefs.ProjectRecord, alive func() bool) (wrote bool, err error) {
	pr := s.projects() // берёт mu сам: до Lock
	s.mu.Lock()
	defer s.mu.Unlock()
	if alive != nil && !alive() {
		return false, nil
	}
	err = pr.Put(uid, rec)
	switch {
	case err == nil:
		delete(s.mem, uid)
		delete(s.del, uid)
		s.retryDel(pr) // диск снова пишется: стираем и записи, что не стёрлись раньше
	case !errors.Is(err, prefs.ErrShareRecord) && !errors.Is(err, prefs.ErrProjectUID):
		s.mem[uid] = rec
		delete(s.del, uid)
	}
	return true, err
}

// drop убирает запись проекта (диск и память). Не вышло стереть на диске (файл новее, занят, диск
// полон) — ставится метка «сброшена»: иначе look снова нашёл бы запись (аудит 0.32-146-T4-8).
func (s *Service) drop(uid string) error {
	pr := s.projects()
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.mem, uid)
	err := pr.Delete(uid)
	if err != nil {
		s.del[uid] = struct{}{}
	} else {
		delete(s.del, uid)
	}
	return err
}

// conn — подключение: origin из config.json (негодный даёт "") и токен по нему.
func (s *Service) conn() (origin, token string, place secret.Place) {
	origin = prefs.ShareURLOf(s.h.CfgDir(), s.rules())
	if origin == "" {
		return "", "", ""
	}
	token, place = s.h.Store.Get(origin)
	return origin, token, place
}

// need — подключение для действия по сети: origin и токен или понятный отказ.
func (s *Service) need() (origin, token string, err error) {
	origin, token, _ = s.conn()
	switch {
	case origin == "":
		return "", "", fail("Сервис не подключён — откройте «Отправить по ссылке» и подключите его")
	case token == "":
		return "", "", fail("Токен не найден — подключите сервис заново")
	}
	return origin, token, nil
}

func (s *Service) client(origin, token string) (*albums.Client, error) {
	return albums.New(origin, token, albums.Options{Version: s.h.Version})
}

// limits — пределы сервиса origin из последнего ответа; ещё не спрашивали — нулевые.
func (s *Service) limits(origin string) albums.Limits {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lim[origin]
}

func (s *Service) setLimits(origin string, l albums.Limits) {
	s.mu.Lock()
	s.lim[origin] = l
	s.mu.Unlock()
}

// hostOf — имя хоста для подписи в окне; негодный адрес — как есть.
func hostOf(origin string) string {
	if u, err := url.Parse(origin); err == nil && u.Host != "" {
		return u.Host
	}
	return origin
}

// tokenLost — токен не подошёл: убираем его из хранилища (записи projects.json целы), пишем в журнал.
func (s *Service) tokenLost(origin string) {
	if err := s.h.Store.Delete(origin); err != nil {
		log.Printf("выгрузка: токен убран из программы, но не из системного хранилища")
	}
	log.Printf("выгрузка: сервис ответил 401, токен убран")
}
