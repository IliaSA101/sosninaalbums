package album

import (
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/backdrop"
	"github.com/IliaSA101/sosninaalbums/internal/album/layers"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// Обёртки слоёв: порядок, выравнивание, перенос/поворот/масштаб группы (спеки 009, 010,
// 018, 023). Охранная проверка аргументов — здесь, до change; тело правки — layers.
// Строка «Договор» у каждой обёртки: что проверено до правки, на что тело может рассчитывать.

// Допустимые операция и опора выравнивания (UI шлёт строки; чужие — холостая правка).
var (
	alignTo  = []string{geom.AlignSel, geom.AlignPage, geom.AlignSpread}
	alignOps = []string{"left", "hcenter", "right", "top", "vcenter", "bottom", "distH", "distV"}
)

// allFinite — все числа конечны (NaN/Inf из UI — холостая правка).
func allFinite(vs ...float64) bool {
	return !slices.ContainsFunc(vs, func(v float64) bool { return math.IsNaN(v) || math.IsInf(v, 0) })
}

// AlignItems — выравнивание и распределение одной правкой. ids — ref мест и фото,
// ID текстов разворота; op: left | hcenter | right | top | vcenter | bottom | distH |
// distV; to (опора): sel — описанный прямоугольник выделения (2+ объекта, для dist* —
// 3+), page — поле страницы центра каждого объекта (распределение — отдельно по
// каждой странице), spread — поле всего разворота.
// Неизвестное op/to, мало объектов, ничего не сдвинулось — холостая. У «Авто» с
// фото в выборе разворот закрепляется той же правкой (withGrid), одни тексты не закрепляют.
// Договор: op из alignOps и to из alignTo, иначе холостая правка; число объектов проверяет тело.
func (e *Editor) AlignItems(spreadID string, ids []string, op, to string) model.View {
	if !slices.Contains(alignOps, op) || !slices.Contains(alignTo, to) {
		return e.View()
	}
	return e.change(func(p *model.Project) { layers.AlignItems(p, spreadID, ids, op, to) })
}

// AlignOverlap — только чтение: распределение op не помещается без наложения хотя
// бы на одной опоре (подсказка UI). Геометрия та же, что у AlignItems; у «Авто» с
// фото — на копии, закреплённой, как в правке. После распределения ответ тот же, что
// до: размеры не меняются, поля опор и страницы центров — тоже.
// Договор: op ∈ {distH, distV} и to из alignTo, иначе false; проект не меняется.
func (e *Editor) AlignOverlap(spreadID string, ids []string, op, to string) (over bool) {
	if (op != "distH" && op != "distV") || !slices.Contains(alignTo, to) {
		return false
	}
	e.read(func(p *model.Project) { over = layers.AlignOverlap(p, spreadID, ids, op, to) })
	return over
}

// ArrangeLayers — порядок слоёв (спека 009 design §3, 010 design §4): op front | up |
// down | back. ids — ref мест и ID текстов разворота; места и тексты — один стек
// (layerItems), текст может лечь под фото. up/down — через ближайшее перекрывающееся
// невыделенное, а не на одну позицию: шаг на одну позицию у неперекрытых элементов
// менял бы только порядок ленты. У «Авто» мест нет — двигаются только тексты.
// Ничего не изменилось — холостая.
// Договор: проверок нет; неизвестную op отбрасывает RestackLayers (холостая правка).
func (e *Editor) ArrangeLayers(spreadID, op string, ids []string) model.View {
	return e.change(func(p *model.Project) { layers.ArrangeLayers(p, spreadID, op, ids) })
}

// TransformGroup — перенос и пропорциональный масштаб группы одной правкой.
// ids — ref мест (ID места или ID фото) и ID текстов разворота; чужое и
// неизвестное пропускается. a = (ax, ay) — неподвижный угол, мм разворота.
// У «Авто» с фото в группе разворот закрепляется той же правкой и только если
// места изменились (withGrid); одни тексты не закрепляют. k = 1 и d = 0, k ≤ 0 и
// NaN/Inf — холостая. merge — сессия стрелок: серия — один шаг отмены; у «Авто»
// с фото не действует (закрепление не должно влипнуть в чужую серию).
// Договор: числа конечны, k > 0, не тождество (k = 1 при d = 0), иначе холостая правка.
func (e *Editor) TransformGroup(spreadID string, ids []string, k, ax, ay, dx, dy float64, merge string) model.View {
	if !allFinite(k, ax, ay, dx, dy) || k <= 0 || k == 1 && dx == 0 && dy == 0 {
		return e.View()
	}
	return e.apply(e.groupMerge(spreadID, ids, merge), func(p *model.Project) { layers.TransformGroup(p, spreadID, ids, k, ax, ay, dx, dy) })
}

// groupMerge — ключ слияния серии стрелок группы (чтение под замком).
func (e *Editor) groupMerge(spreadID string, ids []string, merge string) (key string) {
	e.read(func(p *model.Project) { key = layers.GroupMergeKey(p, spreadID, ids, merge) })
	return key
}

// RotateGroup — поворот группы на deg (по часовой) вокруг (cx, cy), мм разворота.
// ids — как у TransformGroup. Угол не упирается: группа сдвигается внутрь целиком,
// не помещается — крайние поджимаются по одному (geom.GroupRot). Размеры, кадр
// и кегль стоят. NaN/Inf и угол 0 (в том числе 360) — холостая.
// Договор: числа конечны и угол не кратен 360°, иначе холостая правка.
func (e *Editor) RotateGroup(spreadID string, ids []string, deg, cx, cy float64) model.View {
	if !allFinite(deg, cx, cy) || geom.NormDeg(deg) == 0 {
		return e.View()
	}
	return e.change(func(p *model.Project) { layers.RotateGroup(p, spreadID, ids, deg, cx, cy) })
}

// ScaleGroupAxes — масштаб группы по осям рамки под углом phi (градусы): kx вдоль
// её оси x, ky — вдоль y, (ax, ay) стоит. Место сохраняет угол (стороны — образы
// своих осей), у текста стоит точка выравнивания, кегль — по меньшему множителю
// (geom.GroupAxes). kx = ky — прежний TransformGroup (пропорционально, без сдвига).
// NaN/Inf, kx или ky ≤ 0 и kx = ky = 1 — холостая.
// Договор: числа конечны, kx > 0, ky > 0, не оба 1, иначе холостая правка.
func (e *Editor) ScaleGroupAxes(spreadID string, ids []string, kx, ky, phi, ax, ay float64) model.View {
	if !allFinite(kx, ky, phi, ax, ay) || kx <= 0 || ky <= 0 || kx == 1 && ky == 1 {
		return e.View()
	}
	return e.change(func(p *model.Project) { layers.ScaleGroupAxes(p, spreadID, ids, kx, ky, phi, ax, ay) })
}

// Фон (спека 009): фото на фоне разворота или страницы, слой под местами и текстами
// (internal/album/backdrop). «Договор» — то, что проверено до правки (остальное проверяет операция).

// backdropMerge — ключ слияния серии стрелок рамки в один шаг отмены.
func backdropMerge(session string) string {
	if !model.ValidSession(session) {
		return ""
	}
	return "backdrop:" + session
}

// SetBackdrop ставит фото ph на фон области scope разворота. Фона не было — рамка
// по области; был — меняется только фото (кадр сброшен, рамка, поворот и зеркало
// остаются). Фоны другой формы (весь разворот ↔ страницы) снимаются.
// Неизвестная область — errScope, неизвестный разворот — холостая правка.
// Договор: область известна (ValidScope).
func (e *Editor) SetBackdrop(spreadID, scope string, ph model.PhotoInfo) (model.View, error) {
	if !model.ValidScope(scope) {
		return e.View(), model.ErrScope
	}
	return e.try("", func(p *model.Project) error { return backdrop.SetBackdrop(p, spreadID, scope, ph) })
}

// SetBackdropBox ставит рамку фона: перенос, размер, поворот (мм разворота, угол
// до поворота). Зажим — clampBackdrop. merge — сессия стрелок: серия — один шаг отмены.
// Договор: область известна; ключ слияния — только у допустимой сессии (backdropMerge).
func (e *Editor) SetBackdropBox(spreadID, scope string, x, y, w, h, rot float64, merge string) model.View {
	if !model.ValidScope(scope) {
		return e.View()
	}
	return e.apply(backdropMerge(merge), func(p *model.Project) { backdrop.SetBackdropBox(p, spreadID, scope, x, y, w, h, rot) })
}

// SetBackdropFrame ставит кадр фото в рамке: s 1..4, x/y −1..1 (как SetFrame).
// Договор: область известна; зажим кадра — в операции.
func (e *Editor) SetBackdropFrame(spreadID, scope string, s, x, y float64) model.View {
	if !model.ValidScope(scope) {
		return e.View()
	}
	return e.change(func(p *model.Project) { backdrop.SetBackdropFrame(p, spreadID, scope, s, x, y) })
}

// SetBackdropAngle ставит наклон фото в рамке фона: ±44,9°, шаг 0,1 (045). Рамка стоит,
// фото закрывает её само, наклон держит приближение (057). session — сессия строки «Наклон»:
// один шаг отмены, расчёт от кадра её начала.
// Договор: область известна, tilt — конечное число; ключ слияния — tiltMerge.
func (e *Editor) SetBackdropAngle(spreadID, scope string, tilt float64, session string) model.View {
	if !model.ValidScope(scope) || !allFinite(tilt) {
		return e.View()
	}
	merge := tiltMerge("bdtilt:"+spreadID+":"+scope, session)
	return e.apply(merge, func(p *model.Project) {
		backdrop.SetBackdropAngle(p, spreadID, scope, tilt, backdrop.BaseFrame(e.sessionStart(merge), spreadID, scope))
	})
}

// StraightenBackdrop — «Выпрямить по линии» для фона: (dx, dy) мм листа встаёт вдоль края рамки.
// Договор: область известна, dx, dy — конечные числа.
func (e *Editor) StraightenBackdrop(spreadID, scope string, dx, dy float64) model.View {
	if !model.ValidScope(scope) || !allFinite(dx, dy) {
		return e.View()
	}
	return e.change(func(p *model.Project) { backdrop.StraightenBackdrop(p, spreadID, scope, dx, dy) })
}

// FlipBackdrop переключает зеркало по оси axis ("h" | "v"); видимая часть кадра
// остаётся той же, только отражённой (X → −X / Y → −Y), как у SetFlip.
// Договор: ось "h" или "v", область известна.
func (e *Editor) FlipBackdrop(spreadID, scope, axis string) model.View {
	if axis != "h" && axis != "v" || !model.ValidScope(scope) {
		return e.View()
	}
	return e.change(func(p *model.Project) { backdrop.FlipBackdrop(p, spreadID, scope, axis) })
}

// SetBackdropFade ставит прозрачность фото фона области: fade 0..1 (зажим, шаг 0.01).
// merge — сессия ползунка: серия — один шаг отмены. Не число — холостая.
// Договор: область известна, fade — конечное число; ключ слияния — backdropMerge.
func (e *Editor) SetBackdropFade(spreadID, scope string, fade float64, merge string) model.View {
	if !model.ValidScope(scope) || math.IsNaN(fade) || math.IsInf(fade, 0) {
		return e.View()
	}
	return e.apply(backdropMerge(merge), func(p *model.Project) { backdrop.SetBackdropFade(p, spreadID, scope, fade) })
}

// FitBackdrop — «По области»: рамка = область, поворот рамки и кадр сброшены, наклон фото и зеркало остаются.
// Договор: область известна.
func (e *Editor) FitBackdrop(spreadID, scope string) model.View {
	if !model.ValidScope(scope) {
		return e.View()
	}
	return e.change(func(p *model.Project) { backdrop.FitBackdrop(p, spreadID, scope) })
}

// ClearBackdrop снимает фото фона; цвет страниц остаётся. Фото, нигде больше не
// стоящее, уходит из Photos само (pruneRefs).
// Договор: область известна.
func (e *Editor) ClearBackdrop(spreadID, scope string) model.View {
	if !model.ValidScope(scope) {
		return e.View()
	}
	return e.change(func(p *model.Project) { backdrop.ClearBackdrop(p, spreadID, scope) })
}

// Узор и фон из библиотеки (спека 048): узор — параметры генератора в слотах Pat/PatL/PatR
// под фото фона; своё — фото фона с ключом библиотеки (internal/album/backdrop, pattern.go).

// patternMerge — ключ слияния серии ползунка узора в один шаг отмены.
func patternMerge(session string) string {
	if !model.ValidSession(session) {
		return ""
	}
	return "pattern:" + session
}

// ApplyPattern — плитка программы в галерее фонов (решение №76): узор pt в область scope и
// цвет страниц color ("" — цвет не трогать); прежний фон области — узоры и фото на ней —
// снимается тем же шагом. Узор всего листа под страницей остаётся другой странице. Тот же
// повторно — холостая. Неверный узор или цвет — ErrPattern, неизвестный разворот — холостая.
// Договор: область известна (ValidScope); узор, цвет и страницу листа 031 проверяет тело.
func (e *Editor) ApplyPattern(spreadID, scope string, pt model.Pattern, color string) (model.View, error) {
	if !model.ValidScope(scope) {
		return e.View(), model.ErrScope
	}
	return e.try("", func(p *model.Project) error { return backdrop.ApplyPattern(p, spreadID, scope, pt, color) })
}

// ApplyBackdrop — своё из библиотеки в галерее фонов (048): фото ph на фон области, как
// SetBackdrop (ph.Lib — ключ, ph.ID = LibPhotoID), и узоры области снимаются тем же шагом.
// Бросок фото и «Свой файл…» (SetBackdrop) узор не снимают.
// Договор: область известна (ValidScope); ключ проверяет тело (ErrLibKey).
func (e *Editor) ApplyBackdrop(spreadID, scope string, ph model.PhotoInfo) (model.View, error) {
	if !model.ValidScope(scope) {
		return e.View(), model.ErrScope
	}
	return e.try("", func(p *model.Project) error { return backdrop.ApplyBackdrop(p, spreadID, scope, ph) })
}

// SetPattern ставит цвета, масштаб, seed и прозрачность узора области (ключ генератора
// стоит); зажим — ClampPattern. merge — сессия ползунка: серия — один шаг отмены. Узора нет —
// холостая.
// Договор: область известна, масштаб и прозрачность — конечные числа; ключ слияния — patternMerge.
func (e *Editor) SetPattern(spreadID, scope string, pt model.Pattern, merge string) model.View {
	if !model.ValidScope(scope) || !allFinite(pt.Scale, pt.Fade) {
		return e.View()
	}
	return e.apply(patternMerge(merge), func(p *model.Project) { backdrop.SetPattern(p, spreadID, scope, pt) })
}

// ClearPattern снимает узор области; цвет страниц и фото фона остаются.
// Договор: область известна.
func (e *Editor) ClearPattern(spreadID, scope string) model.View {
	if !model.ValidScope(scope) {
		return e.View()
	}
	return e.change(func(p *model.Project) { backdrop.ClearPattern(p, spreadID, scope) })
}

// Стикеры (спека 048 Ф2): картинка на листе — слой между местами и текстами
// (internal/album/layers, sticker.go). Слои, выравнивание, группа и удаление — прежние методы
// (ID стикера там, где ID текста).

// stickerMerge — ключ слияния серии рамки, ползунка или палитры стикера в один шаг отмены.
func stickerMerge(session string) string {
	if !model.ValidSession(session) {
		return ""
	}
	return "sticker:" + session
}

// AddSticker ставит стикер src по центру страницы page листа (шириной ¼ страницы, не выше ½)
// над местами и стикерами, под текстами; id — новый стикер (UI его выделяет). Один шаг.
// Неверный рисунок — ErrSticker, ключ своего — ErrLibKey, больше MaxStickers — ErrStickers.
// Неизвестный лист — холостая с пустым id: встал ли стикер, видно под тем же замком, что и
// правка (сравнение rev снаружи — гонка с чужой правкой между чтением и правкой).
// Договор: page зажата в 0..1, ID создан здесь; источник проверяет тело.
func (e *Editor) AddSticker(spreadID string, page int, src model.StickerSrc) (v model.View, id string, err error) {
	page = min(max(page, 0), 1)
	id = model.NewID()
	placed := false
	v, err = e.try("", func(p *model.Project) error {
		err := layers.AddSticker(p, spreadID, page, id, src)
		s, _ := p.FindSticker(id)
		placed = s != nil
		return err
	})
	if err != nil || !placed {
		id = ""
	}
	return v, id, err
}

// ReplaceSticker — «Заменить…»: другой рисунок или своё фото; центр, ширина и поворот стоят,
// высота — по новой пропорции. Тот же — холостая. changed — правка встала шагом (App кладёт
// плитку в недавние): rev до правки читается под её замком.
// Договор: источник проверяет тело (ошибки — как у AddSticker).
func (e *Editor) ReplaceSticker(spreadID, id string, src model.StickerSrc) (v model.View, changed bool, err error) {
	rev := -1
	v, err = e.try("", func(p *model.Project) error {
		rev = e.rev
		return layers.ReplaceSticker(p, spreadID, id, src)
	})
	return v, err == nil && v.Rev != rev, err
}

// SetStickerBox ставит рамку стикера: перенос, размер, поворот (мм листа, угол до поворота,
// поворот вокруг центра). Зажим — ClampSticker, пропорция — FitStickers. merge — сессия
// стрелок или жеста: серия — один шаг отмены.
// Договор: числа конечны, иначе холостая правка; ключ слияния — stickerMerge.
func (e *Editor) SetStickerBox(spreadID, id string, x, y, w, h, rot float64, merge string) model.View {
	if !allFinite(x, y, w, h, rot) {
		return e.View()
	}
	return e.apply(stickerMerge(merge), func(p *model.Project) { layers.SetStickerBox(p, spreadID, id, x, y, w, h, rot) })
}

// FlipSticker переключает зеркало стикера по оси axis ("h" | "v").
// Договор: ось "h" или "v", иначе холостая правка.
func (e *Editor) FlipSticker(spreadID, id, axis string) model.View {
	if axis != "h" && axis != "v" {
		return e.View()
	}
	return e.change(func(p *model.Project) { layers.FlipSticker(p, spreadID, id, axis) })
}

// SetStickerFade ставит прозрачность стикера: 0..1 (зажим, шаг 0.01). merge — сессия ползунка.
// Договор: fade — конечное число, иначе холостая правка; ключ слияния — stickerMerge.
func (e *Editor) SetStickerFade(spreadID, id string, fade float64, merge string) model.View {
	if !allFinite(fade) {
		return e.View()
	}
	return e.apply(stickerMerge(merge), func(p *model.Project) { layers.SetStickerFade(p, spreadID, id, fade) })
}

// SetStickerInks ставит цвета процедурного стикера (зажим — ClampArt: плохой цвет — цвет
// генератора). merge — сессия палитры. У своего стикера — холостая.
// Договор: проверок нет; ключ слияния — stickerMerge.
func (e *Editor) SetStickerInks(spreadID, id string, inks []string, merge string) model.View {
	inks = slices.Clone(inks)
	return e.apply(stickerMerge(merge), func(p *model.Project) { layers.SetStickerInks(p, spreadID, id, inks) })
}
