package app

// Правки агента 0.34 (151, design §1.2–1.7): отметки, кадр обложки, фон, стикеры, формат типографии.
// Правило то же, что у agents_edit.go: редактор берётся один раз, rev и срок сверяются под замком
// Editor (album.Expect, методы …If). Один вызов — один шаг отмены; холостая правка — без шага и тоста.
// Тосты — шаблоны Go (agents:edited), слова те же, что у действий человека.

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/app/ownprint"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// agentsEdit34Host заполняет замыкания 0.34: пока заполнены не все семь, служба правящих
// инструментов 0.34 не регистрирует (Host.canEdit34).
func (a *App) agentsEdit34Host(h *agents.Host) {
	h.Families = a.agentFamilies
	h.Mark = a.agentsMark
	h.Cover = a.agentsCover
	h.Bg = a.agentsBg
	h.Stickers = a.agentsStickers
	h.Print = a.agentsPrint
}

// agentFamilies — Host.Families: семейство раскладки каждого листа; альбома ещё нет — пустые.
func (a *App) agentFamilies(ids []string) []string {
	select {
	case <-a.ready:
	default:
		return nil
	}
	ed := a.editor()
	if ed == nil {
		return nil
	}
	return ed.Families(ids)
}

// agentStart — начало правки: редактор по ключу агента и разрешение самого инструмента (не «хоть
// одна правка не Запретить»). Вопрос по вызову задан на входе (asked=true, как у vary_layouts и undo):
// отказ — «Запретить» этому инструменту или причина blocked, пока правка ждала ленту.
func (a *App) agentStart(key, tool string) (*album.Editor, error) {
	ed, err := a.agentEditor(key)
	if err != nil {
		return nil, err
	}
	return ed, a.agentCheckTool(tool, true)
}

// agentInfos — сведения о фото ленты для правки агента: только id из a.byID. Запрос копии «id-new»
// (ветка a.infos для интерфейса) агенту не принадлежит: отметка под таким ключом после переоткрытия
// легла бы на настоящее фото.
func (a *App) agentInfos(ids []string) []model.PhotoInfo {
	return a.infos(slices.DeleteFunc(slices.Clone(ids), model.IsCopyReq))
}

// agentDecorErr: плитки нет в библиотеке — ErrNoDecor; прочее — как есть.
func agentDecorErr(err error) error {
	if errors.Is(err, decor.ErrNotFound) {
		return agents.ErrNoDecor
	}
	return err
}

// agentsMark — Host.Mark: отметки фото ленты, как SetMarks у человека; чужие ID — в Skipped.
func (a *App) agentsMark(ctx context.Context, in agents.MarkReq) (res agents.MarkRes, err error) {
	defer guardErr("agentsMark", &err)
	ed, err := a.agentStart(in.Key, toolMarks)
	if err != nil {
		return res, agentHostErr(err)
	}
	infos := a.agentInfos(in.IDs)
	known := make(map[string]bool, len(infos))
	for _, p := range infos {
		known[p.ID] = true
	}
	for _, id := range in.IDs {
		if !known[id] {
			res.Skipped = append(res.Skipped, id)
		}
	}
	var patch model.MarkPatch // Toggle false: повтор того же цвета его не снимает
	if in.SetRating {
		patch.Rating = &in.Rating
	}
	switch in.Label {
	case "":
	case "none":
		patch.Label = new(string)
	default:
		patch.Label = &in.Label
	}
	if f, ok := map[string]int{"pick": 1, "reject": -1, "none": 0}[in.Flag]; ok {
		patch.Flag = &f
	}
	v, n, err := ed.SetMarksIf(infos, patch, album.Expect{Rev: in.Rev, Ctx: ctx})
	if err != nil {
		return res, agentHostErr(agentErr(err))
	}
	res.Album, res.Changed = agents.AlbumRef{Key: in.Key, Rev: v.Rev}, n
	if n > 0 {
		a.changed(v)
		a.agentEdited(AgentEdited{Client: agentToastClient(in.Client), Text: fmt.Sprintf("Отметки: %d фото", n), Rev: v.Rev})
	}
	a.agentDraftStep(in.Key, in.Rev, res.Album.Rev) // черновик агента остаётся нетронутым
	return res, nil
}

// agentsCover — Host.Cover: обложки нет — создаётся с фото, как бросок на «+ Обложка»; есть — фото
// встаёт на лицевую (пустое место, иначе вместо кадра в наибольшем, иначе фоном).
func (a *App) agentsCover(ctx context.Context, in agents.CoverReq) (res agents.CoverRes, err error) {
	defer guardErr("agentsCover", &err)
	ed, err := a.agentStart(in.Key, toolCover)
	if err != nil {
		return res, agentHostErr(err)
	}
	infos := a.agentInfos([]string{in.Photo})
	if len(infos) == 0 {
		return res, agents.ErrNoPhoto
	}
	before := ed.View()
	if before.Settings.Sheet {
		return res, agents.ErrNoCover
	}
	x := album.Expect{Rev: in.Rev, Ctx: ctx}
	var v model.View
	if before.Cover == nil {
		cur, libs := a.coverLookLibs(ed)
		v, _, res.Place, err = ed.CreateCoverLookIf("", &infos[0], libs, cur, x)
		if err == nil { // по ревизии, а не «нет ошибки»: холостой вызов не создание
			res.Changed = v.Rev != before.Rev
			res.Created = res.Changed
			if res.Changed && v.Cover != nil {
				res.Product = v.Cover.Preset
			}
		}
	} else {
		v, res.Place, res.Changed, err = ed.CoverPhotoIf(infos[0], x)
	}
	if err != nil {
		return agents.CoverRes{}, agentHostErr(agentErr(err))
	}
	res.Album = agents.AlbumRef{Key: in.Key, Rev: v.Rev}
	if res.Changed {
		text := "Фото на обложке"
		if res.Created {
			text = "Обложка создана с фото"
		}
		a.changed(v)
		a.agentEdited(AgentEdited{Client: agentToastClient(in.Client), Text: text, Rev: v.Rev, Go: model.CoverID})
	}
	a.agentDraftStep(in.Key, in.Rev, res.Album.Rev) // черновик агента остаётся нетронутым
	return res, nil
}

// agentsBg — Host.Bg: фон из каталога на набор листов одним шагом. Листы с фото фона клиента без
// Replace пропускаются (Skipped), вопроса агент не получает. В «Недавние» плитка не пишется.
func (a *App) agentsBg(ctx context.Context, in agents.BgReq) (res agents.BgRes, err error) {
	defer guardErr("agentsBg", &err)
	ed, err := a.agentStart(in.Key, toolBg)
	if err != nil {
		return res, agentHostErr(err)
	}
	prev := ed.View()
	targets := make([]model.GridTarget, len(in.Spreads))
	for i, id := range in.Spreads {
		targets[i] = model.GridTarget{Spread: id, Scope: in.Scope}
	}
	src, err := a.bgSource(targets, in.Item)
	if err != nil {
		return res, agentHostErr(agentDecorErr(err))
	}
	v, out, err := ed.ApplyBackgroundToIf(targets, src, in.Replace, album.Expect{Rev: in.Rev, Ctx: ctx})
	if err != nil {
		return res, agentHostErr(agentErr(err))
	}
	res = agents.BgRes{Album: agents.AlbumRef{Key: in.Key, Rev: v.Rev}, Changed: out.Changed, Skipped: out.Skipped, Replaced: out.Replaced > 0}
	if out.Changed > 0 {
		first := "" // первый лист, где вид действительно изменился (пропущенные и холостые не в счёт)
		for _, id := range in.Spreads {
			if !slices.Contains(out.Skipped, id) && spreadChanged(prev, v, id) {
				first = id
				break
			}
		}
		unit := plural(out.Changed, "разворот", "разворота", "разворотов")
		if v.Settings.Sheet {
			unit = plural(out.Changed, "лист", "листа", "листов")
		}
		a.changed(v)
		a.agentEdited(AgentEdited{Client: agentToastClient(in.Client), Text: fmt.Sprintf("Фон поставлен на %d %s", out.Changed, unit),
			Rev: v.Rev, Go: first})
	}
	a.agentDraftStep(in.Key, in.Rev, res.Album.Rev) // черновик агента остаётся нетронутым
	return res, nil
}

// spreadChanged — лист id в виде после правки отличается от вида до неё.
func spreadChanged(before, after model.View, id string) bool {
	i := slices.IndexFunc(before.Spreads, func(s model.SpreadView) bool { return s.ID == id })
	j := slices.IndexFunc(after.Spreads, func(s model.SpreadView) bool { return s.ID == id })
	return i < 0 || j < 0 || !reflect.DeepEqual(before.Spreads[i], after.Spreads[j])
}

// agentsStickers — Host.Stickers: пачка стикеров одним шагом; нет места на листе или в альбоме —
// отказ всей пачки без шага (ErrStickerLimit). Длины параллельных срезов сверяются ещё раз.
func (a *App) agentsStickers(ctx context.Context, in agents.StickerReq) (res agents.StickerRes, err error) {
	defer guardErr("agentsStickers", &err)
	n := len(in.Spreads)
	if n == 0 || n > agents.MaxStickers || len(in.Pages) != n || len(in.Items) != n || len(in.Places) != n || len(in.Sizes) != n {
		return res, errors.New("стикеры: срезы запроса разной длины")
	}
	ed, err := a.agentStart(in.Key, toolSticker)
	if err != nil {
		return res, agentHostErr(err)
	}
	srcs := map[string]model.StickerSrc{}
	puts := make([]album.StickerPut, n)
	for i := range puts {
		src, ok := srcs[in.Items[i]]
		if !ok {
			if src, err = a.stickerSrc(in.Items[i]); err != nil {
				return res, agentHostErr(agentDecorErr(err))
			}
			srcs[in.Items[i]] = src
		}
		page := 1
		if in.Pages[i] == "left" {
			page = 0
		}
		puts[i] = album.StickerPut{Spread: in.Spreads[i], Page: page, Src: src, Anchor: in.Places[i], Size: in.Sizes[i]}
	}
	v, ids, err := ed.AddStickersIf(puts, album.Expect{Rev: in.Rev, Ctx: ctx})
	switch {
	case errors.Is(err, album.ErrNoSheet):
		return res, agents.ErrNoSpread
	case errors.Is(err, model.ErrStickers), errors.Is(err, model.ErrStickersTotal), errors.Is(err, model.ErrSticker):
		return res, agents.ErrStickerLimit
	case err != nil:
		return res, agentHostErr(agentErr(err))
	}
	res = agents.StickerRes{Album: agents.AlbumRef{Key: in.Key, Rev: v.Rev}, Added: len(ids), IDs: ids}
	if len(ids) > 0 {
		a.changed(v)
		a.agentEdited(AgentEdited{Client: agentToastClient(in.Client), Text: fmt.Sprintf("Стикеров добавлено: %d", len(ids)),
			Rev: v.Rev, Go: in.Spreads[0]})
	}
	a.agentDraftStep(in.Key, in.Rev, res.Album.Rev) // черновик агента остаётся нетронутым
	return res, nil
}

// agentsPrint — Host.Print: кнопка формата типографии (поля — ownprint.PrintSettings, как
// presetFields у окна «Настройки альбома»). Изделие обложки: обложки нет, стоит изделие этой
// типографии или «Свои размеры» — остаётся, шаблон идёт за форматом (SetSettings); иначе (другая
// типография, пустое, типографии нет на этом ПК) — изделие по умолчанию этой (ApplyPrint).
func (a *App) agentsPrint(ctx context.Context, in agents.PrintReq) (res agents.PrintRes, err error) {
	defer guardErr("agentsPrint", &err)
	ed, err := a.agentStart(in.Key, toolPrint)
	if err != nil {
		return res, agentHostErr(err)
	}
	pr, ok := printprof.Current().ByID(in.Profile)
	if !ok || len(pr.Formats) == 0 {
		return res, agents.ErrNoProfile
	}
	fi := 0
	if in.Format != "" {
		if fi = slices.IndexFunc(pr.Formats, func(f printprof.PageFormat) bool { return f.Name == in.Format }); fi < 0 {
			return res, agents.ErrNoProfile
		}
	}
	f := pr.Formats[fi]
	before := ed.View()
	if before.Rev != in.Rev {
		return res, agents.ErrAlbumEdited
	}
	st := ownprint.PrintSettings(pr, f, before.Settings)
	x := album.Expect{Rev: in.Rev, Ctx: ctx}
	preset := pr.Default
	if preset == "" && len(pr.Products) > 0 {
		preset = pr.Products[0].ID
	}
	var v model.View
	if cv := before.Cover; preset == "" || cv == nil || cv.Preset == model.CoverCustom ||
		slices.ContainsFunc(pr.Products, func(p printprof.Product) bool { return p.ID == cv.Preset }) {
		v, err = ed.SetSettingsIf(st, x)
	} else {
		v, err = ed.ApplyPrintIf(st, preset, x)
	}
	if err != nil {
		return agents.PrintRes{}, agentHostErr(agentErr(err))
	}
	res = agents.PrintRes{Album: agents.AlbumRef{Key: in.Key, Rev: v.Rev}, PageW: f.Page[0], PageH: f.Page[1],
		Changed: v.Rev != before.Rev}
	if v.Cover != nil {
		res.Cover = v.Cover.Preset
		res.CoverChanged = before.Cover != nil && before.Cover.Preset != v.Cover.Preset
	}
	if pl := v.PageLimit; pl != nil && pl.Max > 0 && pl.Count > pl.Max {
		res.OverLimit = true
	}
	if res.Changed {
		a.changed(v)
		a.agentEdited(AgentEdited{Client: agentToastClient(in.Client), Text: printToast(pr.Name, f.Name), Rev: v.Rev})
	}
	a.agentDraftStep(in.Key, in.Rev, res.Album.Rev) // черновик агента остаётся нетронутым
	return res, nil
}

// printToast — «Формат: Фабрика Фотокниги 30×20». Название типографии и формата задавал человек или
// набор: со знаками, подменяющими текст (RLO, управляющие), вместо них пусто.
func printToast(profile, format string) string {
	if paths.BadRunes(profile) {
		profile = ""
	}
	if paths.BadRunes(format) {
		format = ""
	}
	t := "Формат"
	if profile != "" || format != "" {
		t += ":"
	}
	for _, s := range []string{profile, format} {
		if s != "" {
			t += " " + s
		}
	}
	return t
}
