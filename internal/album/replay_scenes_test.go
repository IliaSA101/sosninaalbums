package album_test

// Сцены голден-реплея (replay_test.go): сетка, обложка, виньетка и хвост сценария.

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// replayGrid — сетка: места, группа, выравнивание, шаблоны, копия и размножение.
func replayGrid(r *replay, ph []model.PhotoInfo) {
	e, id := r.e, func(i int) string { return ph[i].ID }
	sb := r.spreadOf(id(1))
	r.do("SetGrid", e.SetGrid(sb, true))
	r.ve("AddPlace")(e.AddPlace(sb, 1, ""))
	r.ve("AddPlace:левая")(e.AddPlace(sb, 0, ""))
	r.do("SetPlaces", e.SetPlaces(sb, []model.PlaceBox{{ID: id(1), X: 20, Y: 30, W: 200, H: 150}}, "g1"))
	r.do("SetPlaces:слияние", e.SetPlaces(sb, []model.PlaceBox{{ID: id(1), X: 25, Y: 30, W: 200, H: 150}}, "g1"))
	r.do("SetPlaceAngle", e.SetPlaceAngle(sb, id(1), 12))
	r.do("SetPlaceFit", e.SetPlaceFit(sb, []string{id(1)}, model.FitContain))
	r.ve("ArrangePlaces")(e.ArrangePlaces(sb, "splitH", []string{r.emptyPlace(sb)}))
	r.do("ArrangeLayers", e.ArrangeLayers(sb, "front", []string{id(1)}))
	r.ve("AddText:сетка")(e.AddText(sb, 1, "t2"))
	t2 := r.lastText(sb).ID
	r.vnne("PlacePhotos")(e.PlacePhotos(sb, r.emptyPlace(sb), ph[15:17]))
	r.vnn("FillPlaces")(e.FillPlaces(ph[16:18], sb))

	group := []string{id(1), id(15), t2}
	r.read("AlignOverlap", e.AlignOverlap(sb, group, "distH", "spread"))
	r.do("AlignItems", e.AlignItems(sb, group, "top", "page"))
	r.do("TransformGroup", e.TransformGroup(sb, group, 0.9, 20, 20, 3, 2, ""))
	pw, phh := r.v.Settings.PageW, r.v.Settings.PageH
	r.do("RotateGroup", e.RotateGroup(sb, group, 10, pw, phh/2))
	r.do("ScaleGroupAxes", e.ScaleGroupAxes(sb, group, 1.1, 0.9, 0, 20, 20))
	r.do("MirrorGrid", e.MirrorGrid(sb, model.ScopeSpread))
	r.do("DeletePlaces", e.DeletePlaces(sb, []string{id(16)}))

	s0 := r.spreadOf(id(2))
	boxes, mode, err := e.PlaceBoxes(s0, model.ScopeLeft)
	r.read("PlaceBoxes", []any{boxes, mode, fmt.Sprint(err)})
	tpls := []model.Template{{ID: "tpl", Name: "Два", Scope: model.TplPage, Boxes: []model.Box{{X: 0, Y: 0, W: 1, H: 0.5}, {X: 0, Y: 0.5, W: 1, H: 0.5}}}}
	r.read("GridChoices", e.GridChoices(s0, 0, model.ScopeSpread, tpls))

	sc := r.spreadOf(id(6))
	r.vne("ApplyGrid")(e.ApplyGrid(sc, model.ScopeLeft, model.TplPage, tpls[0].Boxes))
	r.vne("ApplyGrid:отказ")(e.ApplyGrid(sc, model.ScopeLeft, model.TplSpread, tpls[0].Boxes))
	targets := []model.GridTarget{{Spread: sc, Scope: model.ScopeRight}}
	r.read("PlanCopyGrid", e.PlanCopyGrid(sb, model.ScopeLeft, targets, true, true))
	r.vne("CopyGrid")(e.CopyGrid(sb, model.ScopeLeft, targets, true, true))
	r.ve("DuplicateGrid")(e.DuplicateGrid(sb, 2, true))
	r.do("DeleteSelection", e.DeleteSelection(sb, []string{id(15)}, []string{r.spread(sb).Places[1].ID}, []string{t2}))
	r.do("SetGrid:авто", e.SetGrid(sb, false))
	r.do("Undo:авто", e.Undo())
}

// replayCover — обложка (032): создание с фото на лицевой, пресет, свои числа, раскладки,
// надпись на корешке, разметка, место с фото. Обложка остаётся до конца: хвост (перепривязка,
// удаление разворотов — Reflow, восстановление) идёт с ней.
func replayCover(r *replay, ph []model.PhotoInfo) {
	e := r.e
	r.ve("CreateCover")(e.CreateCover("", &ph[16]))
	r.ve("CreateCover:повтор")(e.CreateCover("", nil))
	r.ve("ApplyCoverPreset")(e.ApplyCoverPreset("netprint-premium"))
	r.ve("ApplyCoverPreset:неизвестный")(e.ApplyCoverPreset("nope"))
	spec := r.v.Cover.Spec
	spec.W, spec.Hinge = 290, 6
	r.ve("SetCoverSpec")(e.SetCoverSpec(spec))
	r.ve("SetCoverLayout")(e.SetCoverLayout("wrap"))
	r.ve("SetCoverLayout:обе")(e.SetCoverLayout("both"))
	r.ve("SetBackdrop:корешок")(e.SetBackdrop(model.CoverID, model.ScopeSpine, ph[15])) // 061: надпись ниже — без плашки
	r.ve("SetBackdrop:задняя")(e.SetBackdrop(model.CoverID, model.ScopeLeft, ph[17]))
	r.ve("CoverSpineText")(e.CoverSpineText())
	r.ve("CoverSpineText:повтор")(e.CoverSpineText())
	r.do("SetCoverSpineBg", e.SetCoverSpineBg("#203040"))
	r.ve("SetCoverGuide")(e.SetCoverGuide(qPath("Разметка", "cover.png")))
	r.read("CoverGuidePath", e.CoverGuidePath())
	r.ve("SetCoverGuide:UNC")(e.SetCoverGuide(`\\server\share\cover.png`))
	r.do("ClearCoverGuide", e.ClearCoverGuide())
	r.ve("SetSpreadGuide")(e.SetSpreadGuide(qPath("Разметка", "spread_30x30.psd"))) // 066: разметка разворотов
	r.read("SpreadGuidePath", e.SpreadGuidePath())
	r.ve("SetSpreadGuide:PSB")(e.SetSpreadGuide(qPath("Разметка", "big.psb")))
	r.do("ClearSpreadGuide", e.ClearSpreadGuide())
	r.do("ClearSpreadGuide:повтор", e.ClearSpreadGuide())
	r.ve("AddPlace:обложка")(e.AddPlace(model.CoverID, 1, ""))
	r.vnne("PlacePhotos:обложка")(e.PlacePhotos(model.CoverID, r.v.Cover.Sheet.Places[0].ID, ph[18:19]))
	r.do("DeleteCover", e.DeleteCover())
	r.do("Undo:обложка", e.Undo())
}

// replayVignette — виньетка (031): новая по умолчанию, превью, «Применить» с фото и сверх
// предела, сортировка, «Обычный разворот» и его отмена (виньетка идёт в хвост).
func replayVignette(r *replay, ph []model.PhotoInfo) {
	e := r.e
	r.ve("AddVignette")(e.AddVignette(-1))
	sid := r.v.Spreads[len(r.v.Spreads)-1].ID
	r.read("VignetteChoices", e.VignetteChoices(sid, 2, 10, "4:5", nil))
	r.vne("SetVignette")(e.SetVignette(sid, model.Vignette{Tpl: "g-top", Teachers: 2, Pupils: 10, Ratio: "4:5"}, nil))
	r.vnn("FillPlaces:виньетка")(e.FillPlaces(ph[17:19], sid))
	r.vne("SetVignette:предел")(e.SetVignette(sid, model.Vignette{Tpl: "grid", Teachers: 18, Pupils: 64}, nil))
	// Окно «Настроить виньетку»: сухой прогон (чтение) и «Применить» с полями шапки.
	setup := model.VigSetup{Vig: model.Vignette{Tpl: "head-g-top", Teachers: 1, Pupils: 12, Ratio: "3:4"},
		Vars: model.Vars{Title: "Гимназия № 1", Year: "2026"}}
	pr, err := e.PreviewVignette(sid, setup, nil)
	if err != nil {
		r.t.Fatalf("PreviewVignette: %v", err)
	}
	r.read("PreviewVignette", pr)
	r.vne("SetupVignette")(e.SetupVignette(sid, setup, nil))
	// 049: «Руководитель…» — подпись предметника, шапка в ряду — кегль по ячейкам.
	r.vne("SetVignette:руководитель")(e.SetVignette(sid, model.Vignette{Tpl: "head-lead-left", Teachers: 3, Pupils: 12, Ratio: "3:4"}, nil))
	r.vne("SetVignette:шапка в ряду")(e.SetVignette(sid, model.Vignette{Tpl: "t-left-cell", Teachers: 1, Pupils: 12, Ratio: "3:4"}, nil))
	// Свой шаблон (038): лист → шаблон → «Применить» своего, «Вернуть авто».
	tpl, err := e.VigTemplate(sid)
	if err != nil {
		r.t.Fatalf("VigTemplate: %v", err)
	}
	tpl.ID, tpl.Name = "tpl001", "Мой"
	r.read("VigTemplate", tpl)
	r.vne("SetVignette:свой")(e.SetVignette(sid, model.Vignette{Tpl: "my:tpl001", Teachers: 1, Pupils: 14}, []model.Template{tpl}))
	r.vne("ResetVignette")(e.ResetVignette(sid))
	r.do("SortVignette", e.SortVignette(sid))
	r.do("ClearVignette", e.ClearVignette(sid))
	r.do("Undo:виньетка", e.Undo())
}

// replayTail — развороты, папки, перепривязка, восстановление, удаление, запись копий.
func replayTail(r *replay, ph []model.PhotoInfo, q *model.Project) {
	e, id := r.e, func(i int) string { return ph[i].ID }
	s0 := r.spreadOf(id(2))
	r.ve("SplitSpread")(e.SplitSpread(s0, 2))
	r.do("MergeSpreads", e.MergeSpreads(s0))
	r.do("MoveSpread", e.MoveSpread(r.v.Spreads[len(r.v.Spreads)-1].ID, 0))
	r.do("RemovePhotos", e.RemovePhotos([]string{id(12)}))
	r.do("DeleteText", e.DeleteText(r.lastText(s0).ID))

	r.read("Refs", e.Refs())
	ref, ok := e.Ref(id(2))
	r.read("Ref", []any{ref, ok})
	r.read("Folders", e.Folders())
	r.read("Path", strings.Replace(e.Path(), r.file, "<album>", 1))
	r.do("Relink", e.Relink(map[string]string{id(7): qPath("Перенос", "07.jpg")}, qPath("Перенос")))
	// Сменить букву по-настоящему можно только с файлами на двух дисках
	// (editor_relink_test.go): здесь на R: ничего нет — холостая.
	r.do("RelinkDrive", e.RelinkDrive("Q", "R"))
	// Своих из библиотеки в сценарии нет — холостая (настоящая — TestRelinkLib в salbum).
	r.do("RelinkLib", e.RelinkLib(func(string) (string, bool) { return "", false }))
	// Своих шрифтов в сценарии нет — холостая (настоящая — TestRefitFontsStep).
	r.do("RefitFonts", e.RefitFonts())
	r.do("RemoveFolder", e.RemoveFolder(qPath("Банкет")))
	r.ve("Restore")(e.Restore(q))
	r.do("Undo:восстановление", e.Undo())

	r.do("DeleteSpread", e.DeleteSpread(r.spreadOf(id(9))))
	r.do("DeleteSpreads", e.DeleteSpreads([]string{r.v.Spreads[0].ID, r.v.Spreads[1].ID}))
	r.do("Undo:удаление", e.Undo())
	r.do("Undo:ещё", e.Undo())
	r.do("Redo:удаление", e.Redo())
	r.do("Redo:второй", e.Redo())
	r.do("Redo:холостой", e.Redo())

	cp := filepath.Join(r.dir, "copy.salbum")
	r.write("WriteCopy", e.WriteCopy(cp), cp)
	dr := filepath.Join(r.dir, "draft.salbum")
	r.write("WriteDraft", e.WriteDraft(dr), dr)
	r.read("PhotosOf", e.PhotosOf([]string{s0, "нет"}))
	// «Разнообразить» — на развороте «Авто» с оценкой (развороты сценария — сетки, там шаг холостой):
	// 5★ у третьего фото (XMP) делает его главным, раскладка должна смениться.
	r.ve("NewSpread:авто")(e.NewSpread(-1, ph[6:10]))
	last := func() model.SpreadView { return r.v.Spreads[len(r.v.Spreads)-1] }
	sid, was := last().ID, last().Layout
	rated := slices.Clone(ph[6:10])
	rated[2].Xmp.Rating = 5
	r.do("VaryLayouts", e.VaryLayouts([]string{sid}, model.VarietyNormal, nil, rated))
	if last().ID != sid || last().Layout == was {
		r.t.Fatalf("VaryLayouts: раскладка %q не сменилась", was)
	}
	// Бросок 042 (в конце: прежние шаги эталона не сдвигаются): у закреплённого разворота одно
	// пустое место, три новых фото — вопрос без шага, ответ «add» — место и плитка одним шагом.
	// ID новых мест случайны: в out — только числа.
	r.ve("AddPlace:042")(e.AddPlace(sid, 1, ""))
	more := []model.PhotoInfo{replayPhoto(19, 'h', model.XmpMarks{}), replayPhoto(20, 'v', model.XmpMarks{}), replayPhoto(21, 'h', model.XmpMarks{})}
	for _, rest := range []string{"", "add"} {
		v, o, err := e.DropPhotos(sid, "", more, 0.75, rest)
		r.edit("DropPhotos:"+rest, v, fmt.Sprintf("ask=%v fit=%d total=%d fresh=%d grown=%d out=%v left=%d small=%v",
			o.Ask, o.Fit, o.Total, len(o.Fresh), o.Grown, o.Out, o.Left, o.Small), err)
	}
	// Имена своих шаблонов (QA 038): своих шаблонов на листах нет — холостая, без шага и rev.
	v, changed := e.SyncOwnNames([]model.Template{{ID: "0123456789ab", Name: "Свой", Kind: model.TplVignette}})
	r.edit("SyncOwnNames", v, changed, nil)
	// Наклон 045 (в конце, как 042): выпрямить фото и фон, ровная линия — холостая; кадр и
	// зеркало наклон фона оставляют (зеркало меняет знак), «По области» сбрасывает кадр, не наклон.
	r.do("StraightenPhoto", e.StraightenPhoto(sid, ph[6].ID, 40, 3))
	r.do("StraightenPhoto:ровная", e.StraightenPhoto(sid, ph[6].ID, 40, 0))
	r.ve("SetBackdrop:045")(e.SetBackdrop(sid, model.ScopeSpread, ph[13]))
	r.do("SetBackdropAngle", e.SetBackdropAngle(sid, model.ScopeSpread, 4.5, ""))
	r.do("StraightenBackdrop", e.StraightenBackdrop(sid, model.ScopeSpread, 100, 3))
	r.do("SetBackdropFrame:045", e.SetBackdropFrame(sid, model.ScopeSpread, 1.4, 0.3, 0))
	r.do("FlipBackdrop:045", e.FlipBackdrop(sid, model.ScopeSpread, "h"))
	r.do("FitBackdrop:045", e.FitBackdrop(sid, model.ScopeSpread))
	// Предпросмотр окна «Имена» (043): чтение, без шага и rev.
	plan, err := e.PreviewNames(sid, model.NamesOpts{Text: "Фамилия\tИмя\tПредмет\nСидорова\tОльга\tМатематика\nКозлов\tИван"})
	r.read("PreviewNames", []any{plan, fmt.Sprint(err)})
	r.do("SetPlaceRound", e.SetPlaceRound(sid, []string{ph[6].ID}, 20)) // форма 044 — в конце, как 042
	r.ve("AddPlace:круглое")(e.AddPlace(sid, 0, model.ShapeOval))
	r.do("SetPlaceShape", e.SetPlaceShape(sid, []string{ph[7].ID}, model.ShapeOval))
	// Узор и своё из библиотеки (048, в конце, как 042): узор листа, узор страницы (лист — другой
	// странице), серия ползунка, отказ по ключу, своё на правую (узор снят), «Убрать».
	r.ve("ApplyPattern")(e.ApplyPattern(sid, model.ScopeSpread, model.Pattern{Kind: "stripes"}, "#f5efe6"))
	r.ve("ApplyPattern:левая")(e.ApplyPattern(sid, model.ScopeLeft, model.Pattern{Kind: "dots", Inks: []string{"#AABBCC"}}, ""))
	r.do("SetPattern", e.SetPattern(sid, model.ScopeLeft, model.Pattern{Inks: []string{"#aabbcc"}, Scale: 1.5, Fade: 0.2}, "p1"))
	r.do("SetPattern:слияние", e.SetPattern(sid, model.ScopeLeft, model.Pattern{Inks: []string{"#aabbcc"}, Scale: 1.5, Fade: 0.4}, "p1"))
	const key = "my:0123456789abcdef"
	own := model.PhotoInfo{ID: model.LibPhotoID(key), Path: qPath("Библиотека", "Фоны", "0123456789abcdef.jpg"), Lib: key, W: 3000, H: 2000, Size: 99}
	r.ve("ApplyBackdrop:ключ")(e.ApplyBackdrop(sid, model.ScopeRight, model.PhotoInfo{ID: "x", Path: own.Path, Lib: "my:x"}))
	r.ve("ApplyBackdrop")(e.ApplyBackdrop(sid, model.ScopeRight, own))
	r.do("ClearPattern", e.ClearPattern(sid, model.ScopeLeft))
	// Стикеры (048 Ф2, в конце, как 042): плашка и своё, отказ по рисунку, рамка серией, зеркало,
	// прозрачность, цвета, замена, слой, удаление. ID стикеров случайны — в шагах их нет.
	const key2 = "my:fedcba9876543210"
	mine := model.StickerSrc{Photo: model.PhotoInfo{ID: model.LibPhotoID(key2), Path: qPath("Библиотека", "Стикеры", "fedcba9876543210.png"), Lib: key2, W: 800, H: 400}}
	v, k1, err := e.AddSticker(sid, 0, model.StickerSrc{Art: &model.Art{Kind: "plate"}})
	r.edit("AddSticker", v, nil, err)
	v, k2, err := e.AddSticker(sid, 1, mine)
	r.edit("AddSticker:своё", v, nil, err)
	v, _, err = e.AddSticker(sid, 0, model.StickerSrc{Art: &model.Art{Kind: "dots"}})
	r.edit("AddSticker:узор", v, nil, err)
	r.do("SetStickerBox", e.SetStickerBox(sid, k1, 30, 40, 120, 20, 15, "k1"))
	r.do("SetStickerBox:слияние", e.SetStickerBox(sid, k1, 32, 40, 120, 20, 15, "k1"))
	r.do("FlipSticker", e.FlipSticker(sid, k2, "h"))
	r.do("SetStickerFade", e.SetStickerFade(sid, k2, 0.3, "k2"))
	r.do("SetStickerInks", e.SetStickerInks(sid, k1, []string{"#336699"}, "k3"))
	v, _, err = e.ReplaceSticker(sid, k2, model.StickerSrc{Art: &model.Art{Kind: "plate"}})
	r.edit("ReplaceSticker", v, nil, err)
	r.do("ArrangeLayers:стикер", e.ArrangeLayers(sid, "back", []string{k1}))
	r.do("DeleteSelection:стикер", e.DeleteSelection(sid, nil, nil, []string{k2}))
	// «Копировать оформление» (049 Ф2): левая на правую зеркально с надписями, план, повтор холостой.
	right := []model.GridTarget{{Spread: sid, Scope: model.ScopeRight}}
	r.read("PlanCopyDecor", e.PlanCopyDecor(sid, model.ScopeLeft, right, true, true))
	r.ve("CopyDecor")(e.CopyDecor(sid, model.ScopeLeft, right, true, true))
	// Повтор — только rev и ошибка: полный вид второй раз раздул бы эталон к пределу 2 МБ.
	again, err := e.CopyDecor(sid, model.ScopeLeft, right, true, true)
	r.read("CopyDecor:повтор", [2]any{again.Rev, err})
	// Автоимя при броске (060, в конце, как 042): лист с шаблоном имени, файл-«человек» и файл со
	// стоп-словом — имя получает только первый, тем же шагом.
	r.do("SetSheetCaps:имя", e.SetSheetCaps(sid, true, model.SheetCaps{Style: model.NewCaptionStyle(r.v.Settings.PageH), Text: "{Фамилия} {Имя}"}))
	who := []model.PhotoInfo{edtest.Lib(qPath("Свадьба", "иванов_петя.jpg"), model.XmpMarks{}), edtest.Lib(qPath("Свадьба", "Свадьба Ивановых.jpg"), model.XmpMarks{})}
	for i := range who {
		who[i].W, who[i].H = 2000, 3000
	}
	v, o, err := e.DropPhotos(sid, "", who, -1, "add")
	r.edit("DropPhotos:имя", v, fmt.Sprintf("ask=%v total=%d named=%v", o.Ask, o.Total, o.Named), err)
	// Ряд окна автосборки (065, в конце, как 042): чтение на копии — rev тот же, шага нет.
	pv, err := e.AutoPreview(ph[1:7], -1, model.AutoOpts{Spreads: 2, MaxPer: 64, Families: []string{"dense", "hero"}})
	if e.View().Rev != r.v.Rev {
		r.t.Fatalf("AutoPreview сменил rev: %d → %d", r.v.Rev, e.View().Rev)
	}
	r.read("AutoPreview", []any{pv, fmt.Sprint(err)})
	// Похожие кадры (078): чтение; у фото реплея подписей нет — отброшенных нет.
	sd, err := e.AutoSimilar(ph[1:7], model.AutoOpts{Spreads: 2, MaxPer: 64, Similar: model.MaxAutoLevel})
	r.read("AutoSimilar", []any{sd, fmt.Sprint(err)})
	// Готовый разворот (067, в конце, как 042): снимок правой страницы с местами и надписями
	// (своё фото фона — ключом), план, постановка на весь лист (правая — зеркально), повтор
	// холостой — только rev и ошибка.
	tp, err := e.SpreadTplOf(sid, model.ScopeRight, true, true)
	r.read("SpreadTplOf", []any{tp, fmt.Sprint(err)})
	libs := map[string]model.PhotoInfo{key: own}
	tplan, err := e.PlanSpreadTpl(sid, model.ScopeSpread, tp, libs)
	r.read("PlanSpreadTpl", []any{tplan, fmt.Sprint(err)})
	v, tplan, missing, err := e.ApplySpreadTpl(sid, model.ScopeSpread, tp, libs)
	r.edit("ApplySpreadTpl", v, []any{tplan, missing}, err)
	v, _, _, err = e.ApplySpreadTpl(sid, model.ScopeSpread, tp, libs)
	r.read("ApplySpreadTpl:повтор", [2]any{v.Rev, err})
	// Ключи библиотеки для удаления набора (ревью 067, в конце, как 042): фон в проекте, удалённый
	// стикер — только в истории; чтение без шага.
	r.read("LibKeys", e.LibKeys())
	// Табличка места (071, в конце, как 042): набор одной сессией — один шаг, чтение для карточки.
	seat := r.emptyPlace(sid)
	r.do("SetPlacePerson", e.SetPlacePerson(sid, seat, model.Person{Last: "Петров"}, "s071"))
	r.do("SetPlacePerson:слияние", e.SetPlacePerson(sid, seat, model.Person{Last: "Петров", First: "Ваня"}, "s071"))
	spv, ok := e.PlacePerson(sid, seat)
	r.read("PlacePerson", []any{spv, ok})
	// «Убрать из альбома» в полосе «Не найдено» (075, в конце, как 042): фото из мест, фон-фото и
	// своё фото фона — одним шагом; неизвестный ID пропускается.
	r.do("PurgePhotos", e.PurgePhotos([]string{ph[6].ID, ph[13].ID, own.ID, "нет"}))
	// Фон на набор листов (077, в конце, как 042): план, узор на весь лист (обложка — не цель),
	// повтор холостой — только rev, счёт и ошибка.
	bgTo := []model.GridTarget{{Spread: sid, Scope: model.ScopeSpread}, {Spread: model.CoverID, Scope: model.ScopeSpread}}
	dots := model.BgSource{Pattern: &model.Pattern{Kind: "dots", Preset: "sa:dots"}, Color: "#f5efe6"}
	bplan, err := e.PlanBackgroundTo(bgTo, dots)
	r.read("PlanBackgroundTo", []any{bplan, fmt.Sprint(err)})
	r.vne("ApplyBackgroundTo")(e.ApplyBackgroundTo(bgTo, dots))
	bv, bn, err := e.ApplyBackgroundTo(bgTo, dots)
	r.read("ApplyBackgroundTo:повтор", []any{bv.Rev, bn, fmt.Sprint(err)})
	// Рамочки и прозрачность (079, в конце, как 042): встроенная на фото и пустое место, повтор
	// холостой (только rev), серия толщины, своя картинка, отказ по ключу, прозрачность серией.
	at := []string{ph[7].ID, seat}
	r.ve("ApplyBorder")(e.ApplyBorder(sid, at, model.BorderSrc{Kind: "line", Preset: "sa:line"}))
	bv, err = e.ApplyBorder(sid, at, model.BorderSrc{Kind: "line", Preset: "sa:line"})
	r.read("ApplyBorder:повтор", [2]any{bv.Rev, err})
	r.do("SetBorder", e.SetBorder(sid, at, model.BorderPatch{Width: 2.5}, "b079"))
	r.do("SetBorder:слияние", e.SetBorder(sid, at, model.BorderPatch{Inks: []string{"#C8A24A"}}, "b079"))
	const key3 = "my:00112233445566ff"
	fr := model.PhotoInfo{ID: model.LibPhotoID(key3), Path: qPath("Библиотека", "Рамочки", "00112233445566ff.png"), Lib: key3, W: 3000, H: 4000}
	bv, err = e.ApplyBorder(sid, []string{seat}, model.BorderSrc{Photo: model.PhotoInfo{ID: "x", Lib: "my:x"}})
	r.read("ApplyBorder:ключ", [2]any{bv.Rev, fmt.Sprint(err)})
	r.ve("ApplyBorder:своя")(e.ApplyBorder(sid, []string{seat}, model.BorderSrc{Photo: fr, Cut: [4]float64{0.14, 0.14, 0.14, 0.14}}))
	r.do("SetPhotoFade", e.SetPhotoFade([]string{ph[7].ID}, 0.25, "f079"))
	// G6: та же своя повторно — без шага (плитка не в недавние); путь /media/f — только у записи
	// под рамочкой, у фото места — нет.
	sv, changed, err := e.ApplyBorderStep(sid, []string{seat}, model.BorderSrc{Photo: fr, Cut: [4]float64{0.14, 0.14, 0.14, 0.14}})
	r.read("ApplyBorderStep:повтор", []any{sv.Rev, changed, fmt.Sprint(err)})
	frPath, frOK := e.BorderPath(fr.ID)
	phPath, phOK := e.BorderPath(ph[7].ID)
	r.read("BorderPath", []any{frPath, frOK, phPath, phOK})
	r.read("BorderIDs", e.BorderIDs()) // 079-40: /media/f сверяет ID адреса с ними
}
