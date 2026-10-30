package app

// Числа вопроса о фоне (154, design §1.4): фасад считает их сам по виду и плану редактора, а не со слов
// агента (как свободные фото у autobuild). Ошибка плана — строки нет: «ничего не заменится» вопрос не скажет,
// а саму правку сверит служба.

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
)

// askBgFacts дополняет вопрос set_background: для фона альбома — где свои фоны каталога, где фото клиента,
// где согласие; для фона на набор с replace — сколько фото фона заменится (PlanBackgroundTo, как bgAskText человека).
func (a *App) askBgFacts(e *editAsk, req agents.AskReq, ed *album.Editor, v model.View) {
	if req.AlbumBg {
		e.AlbumBg = true
		for i, sv := range v.Spreads {
			own, client := ownBgOf(sv)
			switch {
			case own == 0:
			case client == 0:
				e.OwnCat = append(e.OwnCat, i+1)
			case req.Replace && slices.Contains(req.Spreads, sv.ID):
				e.OwnCat, e.TakeClient = append(e.OwnCat, i+1), append(e.TakeClient, i+1)
				e.Photos += client
			default:
				e.KeepClient = append(e.KeepClient, i+1)
			}
		}
		return
	}
	if !req.Replace || len(req.Spreads) == 0 {
		return
	}
	s, err := a.decorStore()
	if err != nil {
		return
	}
	bg, err := s.Background(req.Item)
	if err != nil {
		return
	}
	scope := req.Scope
	if scope == "" {
		scope = model.ScopeSpread
	}
	targets := make([]model.GridTarget, len(req.Spreads))
	for i, id := range req.Spreads {
		targets[i] = model.GridTarget{Spread: id, Scope: scope}
	}
	if plan, err := ed.PlanBackgroundTo(targets, model.BgSource{Pattern: bg.Pattern, Photo: bg.Photo}); err == nil {
		e.Photos = plan.Photos
	}
}

// ownBgOf — слои листа, не принадлежащие фону альбома: все (узоры и фото фона) и среди них фото клиента
// (фото фона без ключа библиотеки; фон каталога и наборов ключ имеет).
func ownBgOf(sv model.SpreadView) (own, client int) {
	for _, b := range sv.Backdrops {
		if b.Album {
			continue
		}
		own++
		if b.Lib == "" {
			client++
		}
	}
	for _, p := range sv.Patterns {
		if !p.Album {
			own++
		}
	}
	return own, client
}
