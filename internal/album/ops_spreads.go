package album

// Обёртки Editor над операциями области spreads (internal/album/spreads): состав альбома,
// папки, настройки, разрез и слияние разворотов, выбор раскладки. Тело — не больше трёх
// операторов; строка «Договор» называет то, что проверено до правки (остальное проверяет операция).

import (
	"math"
	"slices"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/cover"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/names"
	"github.com/IliaSA101/sosninaalbums/internal/album/spreads"
	"github.com/IliaSA101/sosninaalbums/internal/album/vigsheet"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// AddPhotos ставит фото в разворот на позицию index (-1 — в конец).
// Фото, уже стоящие в других разворотах, переезжают.
// Больше MaxPhotos на разворот — ошибка, проект не меняется (View текущий).
// Договор: photos без повторов (DedupePhotos).
func (e *Editor) AddPhotos(spreadID string, photos []model.PhotoInfo, index int) (model.View, error) {
	photos = model.DedupePhotos(photos)
	return e.try("", func(p *model.Project) error { return spreads.AddPhotos(p, spreadID, photos, index) })
}

// DropPhotos — бросок фото на ручной разворот (042): placeID "" — на лист, иначе на место; at —
// доля ширины листа под курсором (< 0 — точки нет; NaN/Inf — как −1); rest "" — спросить,
// spreads.RestAdd / RestSkip — ответ. Ask — правки не было (rev и шаг отмены те же). Всё
// остальное — один шаг отмены; отказ (64 места, мелкая плитка, предел учеников) — откат.
// Имя из названия файла (060) — тем же шагом: out.Named.
// Имена не влезли в запас знаков — тот же бросок без них (overBudget, decisions 31).
// Договор: at — NaN/Inf как −1, остальное зажимает PageAt; прочее проверяет операция.
func (e *Editor) DropPhotos(spreadID, placeID string, photos []model.PhotoInfo, at float64, rest string) (v model.View, out spreads.DropOut, err error) {
	r := spreads.DropReq{Spread: spreadID, Place: placeID, Photos: photos, At: dropAt(at), Rest: rest}
	if v, out, err = e.dropTry(r, true); overBudget(err, out.Named) {
		v, out, err = e.dropTry(r, false)
	}
	return v, out, err
}

// dropTry — одна попытка DropPhotos; auto = false — без автоимён.
func (e *Editor) dropTry(r spreads.DropReq, auto bool) (v model.View, out spreads.DropOut, err error) {
	v, err = e.try("", func(p *model.Project) (err error) { out, err = dropNamed(p, r, auto); return err })
	return v, out, err
}

// dropNamed — бросок и автоимена новых фото (names.WithAuto) внутри одной правки; auto = false —
// только бросок.
func dropNamed(p *model.Project, r spreads.DropReq, auto bool) (out spreads.DropOut, err error) {
	if !auto {
		return spreads.Drop(p, r, dropVig)
	}
	named, err := names.WithAuto(p, r.Photos, func() (err error) { out, err = spreads.Drop(p, r, dropVig); return err })
	out.Named = named
	return out, err
}

// dropAt — доля под курсором: не число — точки нет (−1).
func dropAt(at float64) float64 {
	if math.IsNaN(at) || math.IsInf(at, 0) {
		return -1
	}
	return at
}

// dropVig — виньетка для броска: spreads не импортирует grid (depguard), план знает ядро.
var dropVig = spreads.VigHooks{
	Untouched: func(p *model.Project, s *model.Spread) bool { return vigsheet.Untouched(p, s, vigPlan) },
	Grow:      func(p *model.Project, s *model.Spread, k int) error { return vigsheet.GrowVignette(p, s, k, vigPlan) },
}

// NewSpread создаёт разворот с фото на позиции at (-1 — в конец).
// Договор: photos без повторов (DedupePhotos).
func (e *Editor) NewSpread(at int, photos []model.PhotoInfo) (model.View, error) {
	photos = model.DedupePhotos(photos)
	return e.try("", func(p *model.Project) error { return spreads.NewSpread(p, at, photos) })
}

// MovePhoto переносит фото внутри разворота или в другой разворот.
// Договор: нет — всё проверяет операция (неизвестное фото — холостая правка).
func (e *Editor) MovePhoto(photoID, toSpread string, index int) (model.View, error) {
	return e.try("", func(p *model.Project) error { return spreads.MovePhoto(p, photoID, toSpread, index) })
}

// SwapPhotos меняет два фото местами (могут быть в разных разворотах, в том
// числе в местах сетки). Если одного из них нет в альбоме — холостая правка:
// иначе фото встало бы дважды.
// Договор: нет — проверка «оба в альбоме» внутри операции.
func (e *Editor) SwapPhotos(a, b string) model.View {
	return e.change(func(p *model.Project) { spreads.SwapPhotos(p, a, b) })
}

// RemovePhotos убирает фото из мест, а стоящие только фоном — с фонов. Договор: нет — неизвестные ID пропускаются.
func (e *Editor) RemovePhotos(ids []string) model.View {
	return e.change(func(p *model.Project) { spreads.RemovePhotos(p, ids) })
}

// PurgePhotos — «Убрать из альбома» в полосе «Не найдено» (075-13): фото уходят из мест, с фонов
// всех листов и своими стикерами, одним шагом отмены. Договор: нет — неизвестные ID пропускаются.
func (e *Editor) PurgePhotos(ids []string) model.View {
	return e.change(func(p *model.Project) { spreads.PurgePhotos(p, ids) })
}

// DeleteSpread удаляет разворот. Договор: нет — неизвестный ID — холостая правка.
func (e *Editor) DeleteSpread(id string) model.View {
	return e.change(func(p *model.Project) { spreads.DeleteSpread(p, id) })
}

// DeleteSpreads удаляет развороты одним шагом отмены; неизвестные и повторные ID пропускаются,
// ни одного известного — холостая правка (rev тот же). Больше MaxSpreads ID не разбираем:
// столько разворотов в проекте не бывает, а огромный список от UI не должен строить огромный set.
// Договор: нет — длину списка режет операция.
func (e *Editor) DeleteSpreads(ids []string) model.View {
	return e.change(func(p *model.Project) { spreads.DeleteSpreads(p, ids) })
}

// MoveSpread переставляет разворот на позицию to. Договор: нет — to зажимается операцией.
func (e *Editor) MoveSpread(id string, to int) model.View {
	return e.change(func(p *model.Project) { spreads.MoveSpread(p, id, to) })
}

// SetSettings: смена формата страницы сдвигает тексты пропорционально (design.Rescale в Editor.edit);
// обложка по шаблону Фотосферы переходит на шаблон нового формата тем же шагом (047, cover.FollowFormat).
// Договор: нет — размеры и цвет зажимает операция.
func (e *Editor) SetSettings(s model.Settings) (v model.View) {
	t0 := time.Now()
	defer func() { slow(t0, "настройки: %d разворотов", len(v.Spreads)) }()
	return e.change(func(p *model.Project) { old := p.Settings; spreads.SetSettings(p, s); cover.FollowFormat(p, old) })
}

// SheetSize — мм разворота (у проекта из листов 031 — листа): по ним каталог фонов рисует
// плитку узора в пропорции листа (048). Только чтение, без View.
func (e *Editor) SheetSize() (w, h float64) {
	e.read(func(p *model.Project) { g := p.Geo(nil); w, h = g.W, g.H })
	return w, h
}

// AddFolder добавляет папку (093): уже в списке или внутри папки списка — холостая, Within — та
// папка; папки внутри dir и отдельные фото внутри неё уходят в том же шаге. Нелокальный путь (UNC,
// device, относительный) или сверх MaxFolders — ошибка, проект не меняется.
// Договор: путь локальный (paths.CheckLocalPath) — операция ему доверяет.
func (e *Editor) AddFolder(dir string) (v model.View, r model.FolderAdd, err error) {
	if err := paths.CheckLocalPath(dir); err != nil {
		return e.View(), r, err
	}
	v, err = e.try("", func(p *model.Project) (err error) { r, err = spreads.AddFolder(p, dir); return err })
	return v, r, err
}

// RemoveFolder убирает папку из списка (и вложенные в неё записи). Договор: нет — неизвестная
// папка — холостая правка.
func (e *Editor) RemoveFolder(dir string) model.View {
	return e.change(func(p *model.Project) { spreads.RemoveFolder(p, dir) })
}

// AddLoose — новое добавление отдельных фото (097) со временем at (Unix-секунды; даёт App —
// операция детерминирована в тестах). Ничего нового — холостая: ни шага, ни rev, ни dirty.
// Сверх MaxLooseAdds или нелокальный путь — ошибка, проект не меняется.
// Договор: файлы — обычные JPEG/PNG на локальном диске (проверил App); пути операция проверит сама.
func (e *Editor) AddLoose(files []string, at int64) (v model.View, r model.LooseAdded, err error) {
	v, err = e.try("", func(p *model.Project) (err error) { r, err = spreads.AddLoose(p, files, at); return err })
	return v, r, err
}

// RemoveLoose убирает добавление id (file == "") или один его файл. Договор: нет — неизвестное —
// холостая правка.
func (e *Editor) RemoveLoose(id, file string) model.View {
	return e.change(func(p *model.Project) { spreads.RemoveLoose(p, id, file) })
}

// Sources — копии списков источников ленты (папки и отдельные файлы) для сканера App.
// Договор: только чтение под замком.
func (e *Editor) Sources() (folders, files []string) {
	e.read(func(p *model.Project) { folders, files = slices.Clone(p.Folders), p.LooseFiles() })
	return folders, files
}

// PhotosOf — ID фото разворотов ids по порядку ids (чтение): App по ним берёт оценки из XMP
// вне замка Editor. Неизвестные развороты пропускаются; дальше MaxSpreads ID не разбираем
// (как DeleteSpreads: огромный список от UI не должен раздуть ответ).
func (e *Editor) PhotosOf(ids []string) []string {
	out := []string{}
	e.read(func(p *model.Project) {
		for _, id := range ids[:min(len(ids), model.MaxSpreads)] {
			if _, s := p.Find(id); s != nil {
				out = append(out, s.Photos...)
			}
		}
	})
	return out
}

// VaryLayouts — «Разнообразить раскладки» (041) для разворотов ids с уровнем v («Меньше» —
// как «Обычное») и семействами families (065, пусто — все): раскладки «Авто» с учётом соседей
// и текущего вида, звёзды — из XMP в infos и своих отметок. Сетки, виньетки и обложка не
// трогаются. Один шаг отмены; нечего менять — холостая (rev тот же). Пулы греются вне замка.
// Договор: нет — неизвестные и неподходящие развороты пропускает операция.
func (e *Editor) VaryLayouts(ids []string, v model.Variety, families []string, infos []model.PhotoInfo) model.View {
	e.outside(func(p *model.Project) func() { return spreads.VaryWarm(p, ids, v) })
	return e.change(func(p *model.Project) { spreads.VaryLayouts(p, ids, v, families, e.rating(infos)) })
}

// outside — снимок под замком (snap), его счёт — вне замка: прогрев кэша раскладок перед правкой.
func (e *Editor) outside(snap func(p *model.Project) func()) {
	var run func()
	e.read(func(p *model.Project) { run = snap(p) })
	run()
}

// SplitSpread делит разворот id на два: фото [0:at] остаются, фото [at:] уходят
// в новый разворот сразу после него. at вне 1..len−1 или неизвестный id — ничего
// не делает, как и сетка. У обоих лучшая раскладка для нового состава. Тексты,
// фон и название остаются у первого: координаты текстов привязаны к нему.
// На пределе MaxSpreads — ошибка из CheckTotals, проект прежний, шага нет (075-7: тост, как у NewSpread).
// Договор: нет — границы at проверяет операция.
func (e *Editor) SplitSpread(id string, at int) (model.View, error) {
	return e.try("", func(p *model.Project) error { spreads.SplitSpread(p, id, at); return nil })
}

// MergeSpreads клеит разворот id со следующим: фото следующего встают в конец,
// следующий удаляется, его тексты дописываются (координаты те же), цвет фона каждой
// стороны, фото фона и название — первого, без них — второго. Нет следующего, один из двух — сетка, вместе больше MaxPhotos
// фото или MaxTexts текстов — ничего не делает (без ошибки: интерфейс отключает кнопку заранее,
// а второй клиент мог поменять проект).
// Договор: нет — пределы проверяет операция.
func (e *Editor) MergeSpreads(id string) model.View {
	return e.change(func(p *model.Project) { spreads.MergeSpreads(p, id) })
}

// MergeSpreadsAuto — склейка через ручной разворот (103): ручным из пары «Вернуть авто»
// (keep — свой кадр, 098), затем склейка; одна правка, один шаг отмены. Договор: нельзя
// склеить (виньетка, пределы, последний) — холостая до «Вернуть авто»; ошибка — только отказ settle.
func (e *Editor) MergeSpreadsAuto(id string, keep bool) (model.View, error) {
	return e.try("", func(p *model.Project) error { mergeAuto(p, id, keep); return nil })
}

// SplitSpreadAuto — разрез через ручной разворот (103): «Вернуть авто», затем SplitSpread.
// Договор: at вне 1..len−1, виньетка — холостая; предел MaxSpreads — ошибка CheckTotals, откат всего.
func (e *Editor) SplitSpreadAuto(id string, at int, keep bool) (model.View, error) {
	return e.try("", func(p *model.Project) error { splitAuto(p, id, at, keep); return nil })
}

// mergeAuto — пределы проверяются до «Вернуть авто»: отказ склейки не должен оставить его
// отдельной правкой. Области не импортируют друг друга, поэтому сборка — здесь, в ядре.
func mergeAuto(p *model.Project, id string, keep bool) {
	i, s := p.Find(id)
	if s == nil || i+1 >= len(p.Spreads) || !spreads.Mergeable(s, p.Spreads[i+1], true) {
		return
	}
	for _, x := range []*model.Spread{s, p.Spreads[i+1]} {
		if len(x.Grid) > 0 {
			grid.BackToAuto(p, x.ID, "", keep)
		}
	}
	spreads.MergeSpreads(p, id)
}

// splitAuto — разрез по тем же правилам: границы at и виньетка проверяются до «Вернуть авто».
func splitAuto(p *model.Project, id string, at int, keep bool) {
	_, s := p.Find(id)
	if s == nil || s.Kind == model.KindVignette || at < 1 || at >= len(s.Photos) {
		return
	}
	if len(s.Grid) > 0 {
		grid.BackToAuto(p, id, "", keep)
	}
	spreads.SplitSpread(p, id, at)
}

// SetLayout ставит раскладку по ключу (галерея). Битый ключ, уже стоящая раскладка или
// ключ не из рядов подборки с фото на корешке, которое не пустил бы генератор форм
// (узкое или едва заходящее), — холостая правка. У сетки — «Вернуть авто» с этой раскладкой,
// отдельным шагом отмены (не сливается с листанием).
// Договор: ключ слияния — только у разворота «Авто»; ключ раскладки проверяет операция.
func (e *Editor) SetLayout(spreadID, key string) model.View {
	merge := "layout:" + spreadID
	e.read(func(p *model.Project) {
		if p.GridSpread(spreadID) != nil {
			merge = ""
		}
	})
	return e.apply(merge, func(p *model.Project) { spreads.SetLayout(p, spreadID, key) })
}

// StepLayout листает раскладки по кругу: scope "spread" — весь разворот,
// "left"/"right" — одна страница (вторая не двигается). Листание идёт по тому
// ряду, где стоит текущая раскладка; если её нет ни в одном — с ряда «в порядке
// ленты» (первая при delta > 0, последняя при delta < 0).
// Договор: нет — scope и delta проверяет операция.
func (e *Editor) StepLayout(spreadID, scope string, delta int) model.View {
	return e.apply("layout:"+spreadID, func(p *model.Project) { spreads.StepLayout(p, spreadID, scope, delta) })
}

// LayoutChoices — ряды раскладок для галереи. Только чтение: проект не меняется.
// Договор: под замком; Rev берётся здесь, у операции его нет.
func (e *Editor) LayoutChoices(spreadID string) (c model.Choices) {
	e.read(func(p *model.Project) { c = spreads.LayoutChoices(p, spreadID); c.Rev = e.rev })
	return c
}

// MoreLayouts — то же, что LayoutChoices, но ряд «Авто» (17+ фото) досчитан на порцию
// после have вариантов: плитка «Ещё» галереи. Только чтение.
// Договор: под замком; Rev берётся здесь.
func (e *Editor) MoreLayouts(spreadID string, have int) (c model.Choices) {
	e.read(func(p *model.Project) { c = spreads.MoreLayouts(p, spreadID, have); c.Rev = e.rev })
	return c
}

// DeleteSelection — «Del» по группе одним шагом undo: тексты и стикеры (texts — их ID,
// 048), фото (как
// RemovePhotos: места «Ручной» остаются пустыми) и места по ссылкам (как
// DeletePlaces). Порядок важен: места разбирают после фото, иначе их фото
// ушли бы в ленту вместо удаления из альбома. Неизвестные id пропускаются;
// у «Авто» места не закрепляются (DeletePlaces там холостая). Что убирать
// вместе, решает UI: например, места фото, удалённых этим же вызовом, он
// не передаёт — они остаются выделенными до второго Del.
// Договор: нет — неизвестные ID пропускает операция.
func (e *Editor) DeleteSelection(spreadID string, photos, places, texts []string) model.View {
	return e.change(func(p *model.Project) { spreads.DeleteSelection(p, spreadID, photos, places, texts) })
}
