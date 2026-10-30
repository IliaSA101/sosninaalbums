package app

// Переходник правок агента к Host службы (149, Т3b2): замыкания Host переводят типы службы в типы
// фасада (agents_edit*.go, agentask*.go) и обратно, а после успешной правки шлют agents:edited.
// Тексты тостов — из Go (design §4, 0.33-149-30); интерфейс дописывает к ним «Клиент · » и «Вернуть».
// Путь папки не уходит никуда: в событии только последний элемент (hintLabel), как в вопросе.

import (
	"context"
	"errors"
	"fmt"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// agentErrTable — ошибки фасада и ошибки службы, которыми агенту скажут то же словами. Порядок
// не важен: ошибки разные, совпадение — errors.Is.
var agentErrTable = []struct{ from, to error }{
	{errAgentNotReady, agents.ErrNotReady},
	{errAgentNoAlbum, agents.ErrNoAlbum},
	{errAgentAlbumChanged, agents.ErrAlbumChanged},
	{errAgentAlbumEdited, agents.ErrAlbumEdited},
	{errAgentScanning, agents.ErrScanning},
	{errAgentNoFree, agents.ErrNoFree},
	{errAgentNoSpread, agents.ErrNoSpread},
	{errAgentLate, agents.ErrLate},
	{errAgentBackup, agents.ErrBackup},
	{errAgentOff, agents.ErrNoEdits},
}

// agentHostErr переводит ошибку фасада в ошибку службы; неизвестная идёт как есть: служба сама
// заменит её на «внутреннюю ошибку» и запишет в log.txt без путей.
func agentHostErr(err error) error {
	for _, e := range agentErrTable {
		if errors.Is(err, e.from) {
			return e.to
		}
	}
	return err
}

// agentToastClient — имя клиента для тоста: службой очищено, пустое (неизвестный или похожий на
// программу клиент) называется «Агент», чтобы тост не начинался с « · ».
func agentToastClient(name string) string {
	if name == "" {
		return "Агент"
	}
	return name
}

// agentsDenied — Options.OnDenied: первый отказ «Запретить» по инструменту за запуск → тост «Настроить».
func (a *App) agentsDenied(client, tool, title string) {
	a.agentDenied(AgentDenied{Client: agentToastClient(client), Tool: tool, Title: title})
}

// agentsScanning — Host.Scanning: лента ещё читает папки.
func (a *App) agentsScanning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.scanning || a.scan.Active
}

// agentsEditHost заполняет замыкания правок Host: пока заполнены не все, служба инструментов
// правки не регистрирует (Host.canEdit).
func (a *App) agentsEditHost(h *agents.Host) {
	h.Perm = a.agentPerm
	h.PermLevel = a.agentPermLevel
	h.Scanning = a.agentsScanning
	h.Busy = a.agentBusy
	h.Backup = func(key string) error { return agentHostErr(a.agentBackup(key)) }
	h.Ask = a.agentsAsk
	h.Auto = a.agentsAuto
	h.Vary = a.agentsVary
	h.Show = func(key, spreadID string) (int, error) {
		n, err := a.agentShow(key, spreadID)
		return n, agentHostErr(err)
	}
	h.Undo = a.agentsUndo
}

// agentsAuto — Host.Auto: автосборка в конец альбома; после неё тост «Собрано разворотов: N» с
// переходом к первому новому развороту.
func (a *App) agentsAuto(ctx context.Context, in agents.AutoReq) (agents.AutoRes, error) {
	r, err := a.agentAuto(ctx, agentAutoReq{Key: in.Key, Rev: in.Rev, Spreads: in.Spreads, PerAuto: in.PerAuto,
		PerMin: in.PerMin, PerMax: in.PerMax, Series: in.Series, Similar: in.Similar, Variant: in.Variant,
		Variety: in.Variety, Families: in.Families, Photos: in.Photos, ScanWaitMs: in.ScanWaitMs, Asked: in.Asked})
	if err != nil {
		return agents.AutoRes{}, agentHostErr(err)
	}
	a.agentDraftStep(in.Key, in.Rev, r.Rev)
	if r.Added > 0 {
		a.agentEdited(AgentEdited{Client: agentToastClient(in.Client), Text: fmt.Sprintf("Собрано разворотов: %d", r.Added),
			Rev: r.Rev, Go: r.Go})
	}
	return agents.AutoRes{Album: agents.AlbumRef{Key: in.Key, Rev: r.Rev}, Added: r.Added, Photos: r.Photos,
		PlanFrom: r.PlanFrom, PlanTo: r.PlanTo, Similar: r.Similar, First: r.First, Go: r.Go}, nil
}

// agentsVary — Host.Vary: раскладки; Changed == 0 — альбом не менялся, тоста нет.
func (a *App) agentsVary(ctx context.Context, in agents.VaryReq) (agents.VaryRes, error) {
	res, err := a.agentVary(ctx, in.Key, in.Rev, in.Spreads, in.Variety, in.Families)
	if err != nil {
		return agents.VaryRes{}, agentHostErr(err)
	}
	a.agentDraftStep(in.Key, in.Rev, res.Album.Rev)
	if res.Changed > 0 {
		a.agentEdited(AgentEdited{Client: agentToastClient(in.Client), Text: fmt.Sprintf("Раскладки изменены: %d", res.Changed), Rev: res.Album.Rev})
	}
	return res, nil
}

// agentsUndo — Host.Undo: отмена своей правки; тоста agents:edited нет (человек видит view).
func (a *App) agentsUndo(ctx context.Context, ref agents.AlbumRef) (agents.AlbumRef, error) {
	rev, err := a.agentUndo(ctx, ref.Key, ref.Rev)
	if err != nil {
		return agents.AlbumRef{}, agentHostErr(err)
	}
	a.agentDraftStep(ref.Key, ref.Rev, rev)
	return agents.AlbumRef{Key: ref.Key, Rev: rev}, nil
}

// agentsAsk — Host.Ask: вопрос человеку; «папка добавлена» (add, answered, не слияние) — тост
// «Добавлена папка «имя»» (create и слияние шага отмены не дают). Событие после ответа: view с
// новой папкой человек уже получил от a.changed в askAddFolder.
func (a *App) agentsAsk(ctx context.Context, q agents.AskReq) (agents.AskRes, error) {
	res, err := a.agentAsk(ctx, q)
	if err == nil && q.Kind == agents.AskAdd && res.Why == agents.WhyAnswered && !res.Merged {
		a.agentFolderEdited(q.Client, res)
	}
	return res, err
}

// agentFolderEdited шлёт тост про добавленную папку; имя — последний элемент пути из вида альбома
// (путь целиком никуда не пишется). Альбом сменился после ответа — тоста нет: «Вернуть» отменило бы чужое.
func (a *App) agentFolderEdited(client string, res agents.AskRes) {
	ed, err := a.agentEditor(res.Album.Key)
	if err != nil {
		return
	}
	folders := ed.View().Folders
	if res.Folder < 1 || res.Folder > len(folders) {
		return
	}
	// Имя папки задаёт не человек: управляющие и двунаправленные знаки (RLO) в тосте подменили бы
	// текст, поэтому с такими знаками вместо имени — слово «папка».
	name := hintLabel(folders[res.Folder-1])
	if paths.BadRunes(name) {
		name = "папка"
	}
	a.agentEdited(AgentEdited{Client: agentToastClient(client), Text: "Добавлена папка «" + name + "»", Rev: res.Album.Rev})
}
