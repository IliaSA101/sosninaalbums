package sharer

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/share"
)

const revokeQuestion = "Отозвать ссылку? Клиент больше не откроет альбом."

// target — запись альбома path с отпечатком uid, с которой можно работать по сети: ссылка есть,
// запись этого проекта (T4-9) и того же сервиса, что подключён (С2: по чужой записи запросов нет).
// Вызывать под holdNet.
func (s *Service) target(path, uid string) (origin, token string, rec prefs.ShareRecord, err error) {
	var ok bool
	if rec, ok, _ = s.get(path, uid); !ok || rec.ID == "" {
		return "", "", rec, fail("У этого альбома нет ссылки")
	}
	if origin, token, err = s.need(); err != nil {
		return "", "", rec, err
	}
	if rec.Origin != origin {
		return "", "", rec, fail("Ссылка выгружена на " + hostOf(rec.Origin) + " — подключите этот сервис, чтобы её менять")
	}
	return origin, token, rec, nil
}

// afterNet разбирает ошибку действия по сети: 401 — токен убирается, 404 — запись «нет на сервере».
func (s *Service) afterNet(phase, path, uid, origin string, err error) error {
	logFail(phase, err)
	switch {
	case errors.Is(err, albums.ErrToken):
		s.tokenLost(origin)
	case errors.Is(err, albums.ErrGone):
		s.markGone(path, uid, s.netAlive)
	}
	return fail(errText(err))
}

// Change — «Изменить» пароль и «Продлить» срок (PATCH): срок считается от сегодня, смена пароля
// закрывает прежние входы клиентов (revoke_sessions). Нечего менять — холостой вызов без запроса.
func (s *Service) Change(c Change) (View, error) {
	path, uid := s.h.Ident()
	release, err := s.holdNet(path)
	if err != nil {
		return View{}, err
	}
	defer release()
	origin, token, rec, err := s.target(path, uid)
	if err != nil {
		return View{}, err
	}
	if c.Days != 0 && !validDays[c.Days] {
		return View{}, fail("Срок ссылки: месяц, 3 месяца или год")
	}
	if rec.URL == "" {
		return View{}, fail("Выгрузка не закончена — продолжите её, а потом меняйте срок и пароль")
	}
	var p albums.Patch
	if c.Days > 0 {
		t := s.now().AddDate(0, 0, c.Days).UTC()
		p.ExpiresAt = &t
	}
	if c.SetPassword {
		if c.Password != "" {
			if err := share.CheckPassword(c.Password); err != nil {
				return View{}, err
			}
		}
		pw := c.Password
		p.Password, p.RevokeSessions = &pw, true
	}
	if p.ExpiresAt == nil && p.Password == nil {
		return s.State()
	}
	cl, err := s.client(origin, token)
	if err != nil {
		return View{}, fail(errText(err))
	}
	ctx, cancel := s.netCtx()
	defer cancel()
	al, err := cl.Patch(ctx, rec.ID, &p)
	if err != nil {
		return View{}, s.afterNet("правка ссылки", path, uid, origin, err)
	}
	rec.ExpiresAt, rec.Gone = al.ExpiresAt, false // сервис альбом знает: «нет на сервере» снимается
	if p.Password != nil {
		rec.Password = *p.Password
	}
	wrote, perr := s.putIf(path, rec, s.netAlive)
	if !wrote { // пока шёл PATCH, на путь лёг другой проект: запись прежнего не возвращаем
		log.Printf("выгрузка: альбом сохранён заново во время правки ссылки, запись не возвращена")
		v, _ := s.State() // новый пароль или срок уже на сервере — молчать нельзя
		return v, fail("Изменения приняты сервисом, но альбом за это время сменился — запись о ссылке не сохранена")
	}
	if perr != nil {
		v, _ := s.State() // правка на сервере прошла; запись держится в памяти до выхода
		return v, fail("Изменения приняты сервисом, но записать их в программе не удалось: " + perr.Error())
	}
	return s.State()
}

// Revoke — «Отозвать ссылку»: вопрос («Нет» по умолчанию), затем DELETE; запись удаляется. Сервис
// уже не знает альбома (404) — запись тоже удаляется: клиент ссылку не откроет.
func (s *Service) Revoke() (View, error) {
	path, uid := s.h.Ident()
	release, err := s.holdNet(path) // и на время вопроса: пока диалог открыт, Share не стартует
	if err != nil {
		return View{}, err
	}
	defer release()
	origin, token, rec, err := s.target(path, uid)
	if err != nil {
		return View{}, err
	}
	ok, err := s.h.Ask(revokeQuestion)
	if err != nil || !ok {
		v, _ := s.State()
		return v, err
	}
	cl, err := s.client(origin, token)
	if err != nil {
		return View{}, fail(errText(err))
	}
	ctx, cancel := s.netCtx()
	defer cancel()
	if err := cl.Delete(ctx, rec.ID); err != nil && !errors.Is(err, albums.ErrGone) {
		return View{}, s.afterNet("отзыв ссылки", path, uid, origin, err)
	}
	if err := s.drop(path); err != nil {
		v, _ := s.State()
		return v, fail("Ссылка отозвана, но запись о ней убрать не удалось: " + err.Error())
	}
	return s.State()
}

// netCtx — контекст короткого действия по сети (правка, отзыв): отменяется выходом из программы.
func (s *Service) netCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(s.h.StopCtx(), 60*time.Second)
}
