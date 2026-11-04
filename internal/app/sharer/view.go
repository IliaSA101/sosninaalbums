package sharer

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/secret"
	"github.com/IliaSA101/sosninaalbums/internal/share"
)

// View и вложенные типы — ответ окна (design §4). App объявляет те же поля своими типами
// (ShareView …, пары в TestDTOPairs): определённый тип годится только для плоских.
type View struct {
	Service ServiceInfo `json:"service"`
	Album   Album       `json:"album"`
	Link    *Link       `json:"link"`      // nil — альбом не выгружался (state none)
	Busy    bool        `json:"busy"`      // выгрузка этого альбома идёт
	Other   bool        `json:"busyOther"` // идёт выгрузка другого альбома (В1): хода и чужой ссылки нет
}

// ServiceInfo — сервис в ответе окна.
type ServiceInfo struct {
	Host  string `json:"host"`  // "" — не подключён
	URL   string `json:"url"`   // origin из config.json (поле формы)
	Token bool   `json:"token"` // токен есть (в хранилище или в памяти)
	Store string `json:"store"` // "system" | "session" ("" — токена нет)
	Days  int    `json:"days"`  // limits.max_days сервиса; 0 — неизвестно
}

// Album — что можно сказать о текущем альбоме.
type Album struct {
	Block string `json:"block"` // причина, по которой выгрузить нельзя
	Save  bool   `json:"save"`  // причина — не сохранён
	Title string `json:"title"` // название по умолчанию
	Pages int    `json:"pages"`
	Bytes int64  `json:"bytes"`
}

// Link — ссылка альбома; строки для окна готовы здесь, в Go.
type Link struct {
	State     string `json:"state"` // none | incomplete | ok | changed | expired | gone | other
	URL       string `json:"url"`
	Title     string `json:"title"`
	Password  string `json:"password"`
	ExpiresAt string `json:"expiresAt"` // RFC 3339
	Until     string `json:"until"`     // «до 29 ноября 2026»
	Host      string `json:"host"`      // сервис записи
	Published string `json:"published"` // «Выгружено 29 октября 2026 как «…»» (С1)
}

// Options — настройки окна «Выгрузить». Days: 30 | 90 | 365; 0 — «срок не менять» (обновление по
// ссылке, «Продолжить»), у новой ссылки 0 — отказ; пустые Title и Password — оставить прежние
// (убрать пароль — Change).
type Options struct {
	Title    string `json:"title"`
	Password string `json:"password"`
	Days     int    `json:"days"`
}

// Change — правка выгруженной ссылки: срок от сегодня, пароль.
type Change struct {
	Days        int    `json:"days"`        // 0 — срок не менять
	SetPassword bool   `json:"setPassword"` // true — заменить пароль на Password ("" — снять)
	Password    string `json:"password"`
}

// State — состояние для окна, без сети (design §3.4): подключение, что можно сказать об альбоме и
// его ссылка. Пока идёт выгрузка другого альбома, чужой ссылки и хода нет (В1).
func (s *Service) State() (View, error) {
	job := s.h.Plan()
	origin, token, place := s.conn()
	v := View{
		Service: ServiceInfo{URL: origin, Token: token != "", Days: s.limits(origin).MaxDays},
		Album:   Album{Block: job.Block, Save: job.Save, Title: job.Title, Pages: job.Pages, Bytes: job.Bytes},
	}
	if origin != "" {
		v.Service.Host = hostOf(origin)
	}
	if token != "" {
		v.Service.Store = string(place)
	}
	s.mu.Lock()
	run := s.run
	var runPath string
	if run != nil {
		runPath = run.path
	}
	s.mu.Unlock()
	if run != nil {
		if job.Path != "" && s.h.Fold(runPath) == s.h.Fold(job.Path) {
			v.Busy = true
		} else {
			v.Other = true
			return v, nil
		}
	}
	if rec, ok, _ := s.get(job.Path, ownUID(job)); ok { // запись другого проекта (T4-9) — как «нет ссылки»
		v.Album.Title = rec.Title
		if k := share.State(toRecord(rec), origin, job.Scene, s.now()); k != share.StateNone {
			v.Link = linkOf(rec, k) // none — ссылки нет: форма выгрузки (T4-7)
		}
	}
	return v, nil
}

func toRecord(r prefs.ShareRecord) *share.Record {
	return &share.Record{Origin: r.Origin, ID: r.ID, URL: r.URL, Title: r.Title, Password: r.Password,
		ExpiresAt: r.ExpiresAt, PublishedAt: r.PublishedAt, Scene: r.Scene, Gone: r.Gone}
}

func linkOf(r prefs.ShareRecord, k share.Kind) *Link {
	l := &Link{State: string(k), URL: r.URL, Title: r.Title, Password: r.Password, Host: hostOf(r.Origin)}
	if !r.ExpiresAt.IsZero() {
		l.ExpiresAt = r.ExpiresAt.Format(time.RFC3339)
		l.Until = "до " + share.DateRU(r.ExpiresAt)
	}
	if !r.PublishedAt.IsZero() {
		l.Published = "Выгружено " + share.DateRU(r.PublishedAt) + " как «" + r.Title + "»"
	}
	return l
}

const connectRefusedText = "Токен не подошёл — проверьте его у владельца сервиса"

// Connect — «Подключить»: адрес и токен проверяются у сервиса (Health, List), затем сохраняются;
// токен прежнего origin удаляется (С2: токен одного сервиса другому не подставляется). Пустой токен —
// ошибка, прежний к новому адресу не применяется.
func (s *Service) Connect(rawURL, token string) (View, error) {
	if s.Busy() {
		return View{}, fail("Идёт выгрузка — дождитесь её конца")
	}
	origin, err := share.ServiceURL(rawURL, s.h.AllowLoopback)
	if err != nil {
		return View{}, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return View{}, fail("Введите токен")
	}
	if albums.CheckToken(token) != nil {
		return View{}, fail("Токен выглядит неверно: скопируйте его целиком, без пробелов")
	}
	c, err := s.client(origin, token)
	if err != nil {
		return View{}, fail(errText(err))
	}
	ctx, cancel := context.WithTimeout(s.h.StopCtx(), 30*time.Second)
	defer cancel()
	h, err := c.Health(ctx)
	if err == nil {
		_, err = c.List(ctx) // здесь токен и проверяется: health без него
	}
	if err != nil {
		logFail("подключение", err)
		if errors.Is(err, albums.ErrToken) { // «заново» при первом подключении сбивает: токен надо проверить
			return View{}, fail(connectRefusedText)
		}
		return View{}, fail(errText(noService(err)))
	}
	prev := prefs.ShareURLOf(s.h.CfgDir(), s.rules())
	if _, err := prefs.SetShareURL(s.h.CfgDir(), origin, s.rules()); err != nil {
		return View{}, err
	}
	if prev != "" && prev != origin {
		if err := s.h.Store.Delete(prev); err != nil {
			log.Printf("выгрузка: токен прежнего сервиса не удалён из системного хранилища")
		}
	}
	if _, err := s.h.Store.Set(origin, token); err != nil {
		return View{}, fail("Токен не сохранён: " + err.Error())
	}
	s.setLimits(origin, h.Limits.OrDefault())
	return s.State()
}

// Forget — «Отключить»: токен убирается из программы и хранилища, адрес остаётся в поле. Отказ ОС
// показывается честно (secret.ErrKept, ErrUnavailable): токен из программы убран в любом случае.
func (s *Service) Forget() (View, error) {
	if s.Busy() {
		return View{}, fail("Идёт выгрузка — дождитесь её конца")
	}
	var derr error
	if origin := prefs.ShareURLOf(s.h.CfgDir(), s.rules()); origin != "" {
		derr = s.h.Store.Delete(origin)
	}
	v, err := s.State()
	if err != nil {
		return v, err
	}
	switch {
	case errors.Is(derr, secret.ErrKept):
		return v, fail("Из программы токен убран, но системное хранилище его не удалило: удалите запись «SosninaAlbums» в хранилище паролей вручную")
	case derr != nil:
		return v, fail("Из программы токен убран; системное хранилище недоступно — если токен сохранялся в нём раньше, он мог остаться")
	}
	return v, nil
}

// Password — «Придумать»: слово и четыре цифры (crypto/rand).
func (s *Service) Password() string {
	p, err := share.Invent()
	if err != nil {
		log.Printf("выгрузка: пароль не придуман")
		return ""
	}
	return p
}

// Text — «Скопировать для клиента»: ссылка, пароль и срок записи текущего альбома.
func (s *Service) Text() (string, error) {
	rec, err := s.linkRecord()
	if err != nil {
		return "", err
	}
	return share.ClientText(share.Link{Title: rec.Title, URL: rec.URL, Password: rec.Password, ExpiresAt: rec.ExpiresAt}), nil
}

// Open — «Открыть»: ссылка в браузере; в браузер уходит только проверенный адрес.
func (s *Service) Open() error {
	rec, err := s.linkRecord()
	if err != nil {
		return err
	}
	if err := s.checkLink(rec.URL); err != nil {
		return fail("Ссылка не годится для открытия")
	}
	s.h.OpenURL(rec.URL)
	return nil
}

// linkRecord — запись текущего альбома с готовой ссылкой (не при выгрузке другого альбома).
func (s *Service) linkRecord() (prefs.ShareRecord, error) {
	path, uid := s.h.Ident()
	if s.otherAlbum(path) {
		return prefs.ShareRecord{}, fail("Идёт выгрузка другого альбома — дождитесь её конца")
	}
	rec, ok, _ := s.get(path, uid)
	if !ok || rec.URL == "" {
		return prefs.ShareRecord{}, fail("У этого альбома ещё нет ссылки")
	}
	return rec, nil
}
