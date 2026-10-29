package grid

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// «Вернуть авто» со своим кадром (098 design §3.2, decisions 098-19).

// BackToAuto — «Вернуть авто» ручного разворота одной правкой. Сначала свой кадр фото: keep —
// пропорции окон мест (KeepCrops), иначе снимается (как 0.22); потом раскладка — уже по новым
// пропорциям (Aspects берёт Asp): ключ из галереи, при key "" — сохранённый у листа. Ключ не
// строится или не проходит сгиб — лучшая: узкий свой кадр не ляжет на корешок по ключу, снятому
// с широкого фото. Виньетка — без своего кадра, как SetGrid(false). У «Авто» свой кадр не
// трогается, ставится только ключ (как spreads.SetLayout).
func BackToAuto(p *model.Project, spreadID, key string, keep bool) {
	_, s := p.Find(spreadID)
	switch {
	case s == nil:
		return
	case len(s.Grid) == 0:
		if key != "" && fits(p, s, key, true) {
			p.SetLayout(s, key)
		}
		return
	}
	was := p.Aspects(s.Photos)
	if !s.IsVignette() {
		ownCrops(p, s, keep)
	}
	if key == "" {
		key = s.Layout
	}
	ok := key != "" && fits(p, s, key, slices.Equal(was, p.Aspects(s.Photos)))
	p.Unfreeze(s)
	if !ok {
		key = ""
	}
	p.SetLayout(s, key)
}

// fits — ключ строится по действующим пропорциям листа и проходит сгиб. stand — пропорции
// прежние: ключ, сохранённый у листа, принимается и через сгиб (как spreads.foldAllowed — ключ
// старого альбома не отвергается; так «Вернуть авто» без своего кадра остаётся как в 0.22).
func fits(p *model.Project, s *model.Spread, key string, stand bool) bool {
	asp, prm := p.LayoutIn(s.Photos)
	r, ok := layout.Build(key, asp, prm)
	return ok && (layout.FoldOK(r.Rects, asp, prm) || stand && r.Key == s.Layout)
}

// ownCrops — свой кадр фото листа s перед «Вернуть авто»: keep — по окнам мест, иначе снят.
func ownCrops(p *model.Project, s *model.Spread, keep bool) {
	if keep {
		model.KeepCrops(p, s, true)
		return
	}
	for _, id := range s.Photos {
		if r, ok := p.Photos[id]; ok && r.Asp != 0 {
			r.Asp = 0
			p.Photos[id] = r
		}
	}
}

// CropsToKeep — N для галочки «Сохранить кадр правленых фото: N»: сколько фото ручного листа
// получат свой кадр при «Вернуть авто». «Авто», виньетка и неизвестный лист — 0 (галочки нет).
func CropsToKeep(p *model.Project, spreadID string) int {
	_, s := p.Find(spreadID)
	if s == nil || len(s.Grid) == 0 || s.IsVignette() {
		return 0
	}
	return model.KeepCrops(p, s, false)
}
