package app

// Вопрос агента о правке при «Спрашивать» (150, design §5.2, 0.34-150-4, 0.34-150-37): текст из
// шаблонов Go по входу, окна ОС нет. Вопрос ждёт тот же вызов службы: «да» отдаёт ему итог
// WhyAnswered, а ключ, ревизию и разрешение служба сверяет заново и правку делает сама.

import (
	"crypto/subtle"
	"errors"
	"slices"
	"sort"
	"strconv"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// askKindEdit — вид вопроса о правке (AgentAsk.Kind).
const askKindEdit = "edit"

// askToolOf — инструмент MCP вопроса: у правки его назвала служба, у папки и альбома он по виду.
func askToolOf(req agents.AskReq) string {
	if req.Kind == agents.AskEdit {
		return req.Tool
	}
	return askTool(req.Kind)
}

// askAllowedTool — инструмент не «Запретить» и процесс не от администратора. Разрешение читается заново:
// человек мог сменить его, пока вопрос висел.
func (a *App) askAllowedTool(tool string) bool {
	mode, blocked := a.agentPerm(tool)
	return blocked == "" && mode != prefs.PermDeny
}

// editAskText — заголовок, абзацы и кнопка вопроса о правке по запросу службы. Название фона или
// типографии, которых нет в каталоге, — ошибка до окна.
func (a *App) editAskText(req agents.AskReq, ed *album.Editor) (askBody, error) {
	v := ed.View()
	e := editAsk{Client: req.Client, Tool: req.Tool, Count: req.Count, Sheet: v.Settings.Sheet}
	e.Places = askPlaces(v, req.Spreads)
	if req.Tool == toolAutobuild {
		// Число из входа агента (дубли, уже уложенные фото) вопрос не называет: считаем свободные сами,
		// и когда фото не названы (все свободные ленты) тоже.
		e.Count = len(a.agentFree(ed, v, req.Photos))
		e.Build = max(req.Build, 0)
	}
	for _, f := range req.Families {
		if n, ok := familyNames[f]; ok && !slices.Contains(e.Families, n) {
			e.Families = append(e.Families, n)
		}
	}
	switch req.Tool {
	case toolBg:
		name, err := a.askBgName(req.Item)
		if err != nil {
			return askBody{}, err
		}
		e.Item = name
	case toolPrint:
		pr, ok := printprof.Current().ByID(req.Item)
		if !ok || len(pr.Formats) == 0 {
			return askBody{}, agents.ErrNoProfile
		}
		fi := 0
		if req.Format != "" {
			if fi = slices.IndexFunc(pr.Formats, func(f printprof.PageFormat) bool { return f.Name == req.Format }); fi < 0 {
				return askBody{}, agents.ErrNoProfile
			}
		}
		e.Item, e.Format = catalogName(pr.Name), catalogName(pr.Formats[fi].Name)
	}
	body, ok := editAskBody(e)
	if !ok {
		return askBody{}, errors.New("нет текста вопроса")
	}
	return body, nil
}

// askPlaces — номера разворотов с 1 по ID вида (как видит человек), по возрастанию; обложка — askCoverWord.
// ID, которого уже нет, пропускается: служба сверит ревизию и без нас.
func askPlaces(v model.View, ids []string) []string {
	var nums []int
	cover := false
	for _, id := range ids {
		if id == model.CoverID {
			cover = true
			continue
		}
		if i := slices.IndexFunc(v.Spreads, func(s model.SpreadView) bool { return s.ID == id }); i >= 0 && !slices.Contains(nums, i+1) {
			nums = append(nums, i+1)
		}
	}
	sort.Ints(nums)
	var out []string
	if cover {
		out = append(out, askCoverWord)
	}
	for _, n := range nums {
		out = append(out, strconv.Itoa(n))
	}
	return out
}

// askBgName — название фона по каталогу; нет такого — agents.ErrNoDecor. Название с плохими знаками —
// "" (текст скажет «выбранный фон»).
func (a *App) askBgName(id string) (string, error) {
	cat, err := a.agentsDecor(decor.KindBg)
	if err != nil {
		return "", agentHostErr(err)
	}
	i := slices.IndexFunc(cat.Items, func(it agents.DecorItemLite) bool { return it.ID == id })
	if i < 0 {
		return "", agents.ErrNoDecor
	}
	return catalogName(cat.Items[i].Name), nil
}

// askKindOf — вид открытого вопроса с этим id; false — вопроса нет, он закрыт или id чужой.
func (a *App) askKindOf(id string) (string, bool) {
	m := a.askM()
	m.mu.Lock()
	defer m.mu.Unlock()
	rec := m.cur
	if rec == nil || rec.state != askAsked || subtle.ConstantTimeCompare([]byte(id), []byte(rec.id)) != 1 {
		return "", false
	}
	return rec.kind, true
}

// answerEdit — «да» на вопрос о правке: итог WhyAnswered ждущему вызову (запретили инструмент, пока
// вопрос висел, — WhyOff). Закрыт или чужой — errAskClosed. Правку делает служба, не фасад.
func (a *App) answerEdit(id string) (model.View, error) {
	m := a.askM()
	m.mu.Lock()
	rec := m.cur
	if rec == nil || rec.kind != askKindEdit || rec.state != askAsked || subtle.ConstantTimeCompare([]byte(id), []byte(rec.id)) != 1 {
		m.mu.Unlock()
		return a.askView(), errAskClosed
	}
	tool := rec.req.Tool
	m.mu.Unlock()
	res := agents.AskRes{Why: agents.WhyAnswered}
	if !a.askAllowedTool(tool) {
		res.Why = agents.WhyOff
	}
	if !a.askFinishIf(rec, askAsked, askOutcome{res: res}, res.Why) {
		return a.askView(), errAskClosed
	}
	return a.askView(), nil
}
