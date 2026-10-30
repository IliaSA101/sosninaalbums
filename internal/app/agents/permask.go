package agents

import (
	"context"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Вопрос о правке при «Спрашивать» (150, design §5.2): обратимая правка и показ разворота. Текст
// вопроса — шаблон Go у фасада по числам входа; своего текста агент не вносит. В отличие от папки
// и альбома (вопрос с pending и await_answer) вызов ждёт ответ здесь же: правка идёт после «да»
// тем же вызовом, с проверкой ревизии, а нет ответа за срок вызова — вопрос закрывается.

// editAskReq достаёт из входа правящего инструмента то, что войдёт в вопрос: имя и числа. Разворота
// ссылками агента (как прислал); askEdit переводит их в ID по виду. Item и Format здесь — строки
// агента как пришли: до Host.Ask их заменяет verifyEditItem (настоящий id из каталога), иначе отказ.
func editAskReq(tool string, in any) AskReq {
	r := AskReq{Kind: AskEdit, Tool: tool}
	switch v := in.(type) {
	case autoIn:
		r.Count, r.Families, r.Photos, r.Build = len(v.Photos), v.Families, dedupe(v.Photos), v.Spreads
		r.Variety = 1
		if v.Variety != nil {
			r.Variety = *v.Variety
		}
	case varyIn:
		r.Spreads, r.Families, r.Variety = refStrings(v.Spreads), v.Families, max(v.Variety, 1)
	case showIn:
		r.Spreads = []string{string(v.Spread)}
	case marksIn:
		r.Count = len(v.IDs)
	case setCoverIn:
		r.Count = 1
	case bgIn:
		r.Spreads, r.Item = refStrings(v.Spreads), v.Item
		r.AlbumBg, r.Replace, r.Scope = v.To == "album", v.Replace, v.Scope
	case addStickerIn:
		r.Count = len(v.Stickers)
		for _, st := range v.Stickers {
			r.Spreads = append(r.Spreads, string(st.Spread))
		}
	case applyPrintIn:
		r.Item, r.Format = v.Profile, v.Format
	}
	return r
}

func refStrings(refs []spreadRef) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = string(r)
	}
	return out
}

// askEdit — вопрос о правке: место правки остаётся за вызовом (его отпустит тело или отказ), итог
// «да» возвращает nil, прочее — ошибку для агента. Ключ и ревизию проверяем заново: пока человек
// думал, альбом мог поменяться.
func (c *call) askEdit(ref *AlbumRef, needRev bool) error {
	s := c.s
	if err := s.ed.admit(); err != nil {
		return err
	}
	req := c.editReq
	req.Client = askClient(c.client())
	if err := c.verifyEditItem(&req); err != nil { // строка агента в вопрос не идёт: отказ до окна
		return err
	}
	if ref != nil {
		req.Key, req.Rev = ref.Key, ref.Rev
	}
	if len(req.Spreads) > 0 { // ссылки агента → ID по виду; нет такого разворота — отказ до вопроса
		refs := make([]spreadRef, len(req.Spreads))
		for i, r := range req.Spreads {
			refs[i] = spreadRef(r)
		}
		ids, err := c.spreadIDs(refs, c.spec.view || c.spec.coverOK)
		if err != nil {
			return err
		}
		req.Spreads = ids
	}
	actx, cancel := context.WithCancel(c.env.ctx)
	p := &pend{sess: c.sess, kind: AskEdit, tool: c.spec.name, done: make(chan struct{}), cancel: cancel}
	s.ed.watchAsk(c.sess, p) // подключение закроется — агент ушёл: вопрос «gone»
	c.env.r.wg.Add(1)
	go s.runAsk(actx, askRun{r: c.env.r, runCtx: c.env.ctx, p: p, req: req, release: func() {}})
	select {
	case <-p.done:
	case <-c.ctx.Done():
		p.cancel()
		return errTimeout
	}
	s.ed.mu.Lock()
	err := p.err
	s.ed.mu.Unlock()
	if err != nil {
		return err
	}
	c.verdict = noteYes
	if mode, _ := s.permOf(c.spec); mode == prefs.PermDeny { // запретили, пока вопрос висел и «да» уже пришло
		return errDenied
	}
	if ref != nil {
		v, ed := s.h.View()
		switch {
		case ed == 0:
			return errNoAlbum
		case ref.Key != albumKey(ed):
			return errAlbumChanged
		case needRev && ref.Rev != v.Rev:
			return errAlbumEdited
		}
	}
	return nil
}

// verifyEditItem заменяет строки агента в вопросе настоящими: фон — id из каталога, типография —
// настоящий id профиля (на уровне 1 маска own-N переводится), формат — его настоящее имя. Не
// прошло проверку — отказ до вопроса. У прочих инструментов Item и Format пусты.
func (c *call) verifyEditItem(req *AskReq) error {
	item, format := req.Item, req.Format
	req.Item, req.Format = "", ""
	switch req.Tool {
	case "set_background":
		id, err := c.decorID("bg", item)
		if err != nil {
			return err
		}
		req.Item = id
	case "apply_print":
		id, name, err := c.printTarget(item, format)
		if err != nil {
			return err
		}
		req.Item, req.Format = id, name
	}
	return nil
}
