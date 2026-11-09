package sharer

// Поиск выгруженного альбома на сервисе (167-8, design §4.3): в файле проекта нет ни адреса, ни id,
// только ref = хэш(uid, id). Владелец находит по нему свой альбом через List подключённого
// сервиса — на другом компьютере, после копирования файла, при продлении срока вне этого
// компьютера. Запросы только на адрес из config.json и его токен; адресов из файла нет вовсе.

import (
	"context"
	"errors"
	"log"
	"path/filepath"
	"slices"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/share"
)

// Значения Album.Find.
const (
	findNeed = "need"
	findLost = "lost"
	findFail = "fail"
)

// findState — итог поиска проекта за сессию.
type findState struct {
	kind string // findLost | findFail | "done" (нашёлся или запись обновлена)
	err  string // причина findFail
}

const findTimeout = 30 * time.Second

func (s *Service) setFind(uid string, st findState) {
	s.mu.Lock()
	s.fnd[uid] = st
	s.mu.Unlock()
}

func (s *Service) findOf(uid string) (findState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.fnd[uid]
	return st, ok
}

// findView — Album.Find и FindErr для окна. Поиск нужен проекту без записи и своей записи, которой
// срок вышел или альбом пропал (167-19): продлённый на другом компьютере виден после него.
func (s *Service) findView(uid string, had, own bool, k share.Kind) (find, why string) {
	if had && (!own || k != share.StateExpired && k != share.StateGone) {
		return "", ""
	}
	st, ok := s.findOf(uid)
	switch {
	case !ok:
		return findNeed, ""
	case st.kind == findLost && !had:
		return findLost, ""
	case st.kind == findFail && !had:
		return findFail, st.err
	}
	return "", ""
}

// Find — «ShareFind»: ищет альбом текущего проекта среди альбомов подключённого сервиса по
// привязкам из файла. Нашёлся — запись без пути (moved), своя запись — обновляется. Не нашёлся или
// сбой — итог в Album.Find; ошибкой возвращается только то, что мешает искать (нет токена, идёт
// выгрузка). Альбом сменился за время поиска — запись по uid снимка, у текущего Find не меняется.
func (s *Service) Find() (View, error) {
	snap := s.h.Plan()
	if snap.Path == "" || snap.UID == "" || len(snap.Refs) == 0 {
		return s.State()
	}
	origin, token, err := s.need()
	if err != nil {
		return View{}, err
	}
	release, err := s.holdNet(snap.Path, findBusyText)
	if err != nil {
		return View{}, err
	}
	defer release()
	st, err := s.search(snap, origin, token)
	if err != nil {
		return View{}, err
	}
	s.setFind(snap.UID, st)
	return s.State()
}

// search — один List и сверка привязок; ошибкой возвращает только 401 (токен убран).
func (s *Service) search(snap share.Job, origin, token string) (findState, error) {
	c, err := s.client(origin, token)
	if err != nil {
		return findState{}, fail(errText(err))
	}
	ctx, cancel := context.WithTimeout(s.h.StopCtx(), findTimeout)
	defer cancel()
	list, err := c.List(ctx)
	if err != nil {
		logFail("поиск", err)
		if errors.Is(err, albums.ErrToken) {
			s.tokenLost(origin)
			return findState{}, fail(errText(err))
		}
		return findState{kind: findFail, err: findErrText(err)}, nil
	}
	log.Printf("выгрузка: поиск: альбомов %d", len(list)) // только число (167-20)
	cur, had := s.look(snap.UID)
	switch {
	case had && cur.Own(snap.Path):
		if cur.Origin == origin { // список подключённого сервиса не говорит о записи другого адреса
			s.refresh(snap, cur, list)
		}
		return findState{kind: "done"}, nil
	case had:
		// Чужая запись, которую окно видит (moved, other, expired, gone): вопрос «обновить или как
		// новый» и так будет. Неполная чужая (classify: «записи нет») ничего не говорит — как «не нашёлся».
		if _, seen, _, _ := s.classify(snap, origin); seen {
			return findState{kind: "done"}, nil
		}
		return findState{kind: findLost}, nil
	}
	for _, a := range list {
		if !slices.Contains(snap.Refs, s.h.LinkRef(snap.UID, a.ID)) {
			continue
		}
		if err := s.keepFound(snap.UID, origin, a); err != nil {
			log.Printf("выгрузка: найденный альбом не записан: %v", err)
			if _, kept := s.look(snap.UID); !kept { // записи нет и в памяти: «done» оставил бы Create с тем же uid
				return findState{kind: findLost}, nil
			}
		}
		return findState{kind: "done"}, nil
	}
	return findState{kind: findLost}, nil
}

// findErrText — причина неудачного поиска для окна: без текста сервиса и адресов.
func findErrText(err error) string {
	if errors.Is(err, albums.ErrResponse) || errors.Is(err, albums.ErrTooBig) {
		return "Сервис ответил неверно"
	}
	return errText(noService(err))
}

// foundRecord — запись по альбому списка: не привязана к пути (Path ""), без сцены («изменён») и
// без пароля (167-3). Не опубликованный (черновик) — без ссылки: incomplete.
func (s *Service) foundRecord(origin string, a albums.Album) prefs.ProjectRecord {
	rec := prefs.ProjectRecord{Origin: origin, ID: a.ID, Title: a.Title, ExpiresAt: a.ExpiresAt, PassUnknown: a.HasPassword}
	if a.Published && s.checkLink(a.URL) == nil {
		rec.URL = a.URL
	}
	return rec
}

// keepFound кладёт найденную запись, если за время поиска своей не появилось (PutIfAbsent под одним
// замком). Не легла на диск — остаётся в памяти до выхода, как у put.
func (s *Service) keepFound(uid, origin string, a albums.Album) error {
	rec := s.foundRecord(origin, a)
	pr := s.projects()
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.mem[uid]; ok {
		return nil
	}
	_, _, err := pr.PutIfAbsent(uid, rec)
	if err != nil && !errors.Is(err, prefs.ErrShareRecord) && !errors.Is(err, prefs.ErrProjectUID) {
		s.mem[uid] = rec
	}
	return err
}

// refresh обновляет свою запись по списку сервиса (167-19): срок, «нет на сервере», ссылка,
// признак пароля. Альбома записи в списке нет — запись не трогается. Сервис не отдаёт истёкшие
// альбомы, поэтому «срок вышел» остаётся, пока альбом не нашёлся продлённым.
func (s *Service) refresh(snap share.Job, cur prefs.ProjectRecord, list []albums.Album) {
	i := slices.IndexFunc(list, func(a albums.Album) bool { return a.ID == cur.ID })
	if i < 0 {
		return
	}
	a := list[i]
	rec := cur
	rec.ExpiresAt, rec.Gone = a.ExpiresAt, false
	if rec.URL == "" {
		rec.URL = s.foundRecord(cur.Origin, a).URL
	}
	switch {
	case !a.HasPassword:
		rec.Password, rec.PassUnknown = "", false
	case rec.Password == "":
		rec.PassUnknown = true
	}
	if rec.URL == cur.URL && rec.ExpiresAt.Equal(cur.ExpiresAt) && rec.Gone == cur.Gone &&
		rec.Password == cur.Password && rec.PassUnknown == cur.PassUnknown {
		return
	}
	// alive: пока шёл List, на путь мог лечь другой проект (OnSaved ставит netStop).
	if _, err := s.putIf(snap.UID, rec, s.netAlive); err != nil {
		log.Printf("выгрузка: запись по результатам поиска не сохранена: %v", err)
	}
}

// fixCase переписывает путь своей записи настоящим регистром (ключ сравнивается через Fold, а
// строка в записи могла прийти из shares.json в нижнем регистре). Не удалось — молча: запись цела.
func (s *Service) fixCase(uid, path string, rec prefs.ProjectRecord) {
	clean := filepath.Clean(path)
	if rec.Path == clean || !rec.Own(path) {
		return
	}
	_, _, _ = s.projects().Update(uid, func(r *prefs.ProjectRecord) {
		if r.Own(path) {
			r.Path = clean
		}
	})
}

// Opened — проект открывается по пути path с отпечатком uid (App.openParsed до смены альбома):
// привязка своей записи, чтобы положить её в файл (167-24). Только чтение. Не своя запись или записи
// нет — "".
func (s *Service) Opened(path, uid string) (ref string) {
	rec, had, own := s.get(path, uid)
	if !had || !own || rec.ID == "" {
		return ""
	}
	return s.h.LinkRef(uid, rec.ID)
}

// Settled — проект открыт по пути path (App.openParsed после удачной смены альбома): путь своей
// записи переписывается настоящим регистром. Пишет на диск, поэтому не раньше, чем альбом стал
// текущим, и не из State.
func (s *Service) Settled(path, uid string) {
	if rec, had, own := s.get(path, uid); had && own {
		s.fixCase(uid, path, rec)
	}
}
