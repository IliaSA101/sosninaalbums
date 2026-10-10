package album

// Обёртки Editor над операциями области spreads (internal/album/spreads): состав альбома,
// папки, настройки, разрез и слияние разворотов, выбор раскладки. Тело — не больше трёх
// операторов; строка «Договор» называет то, что проверено до правки (остальное проверяет операция).

import (
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/spreads"
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

// RemovePhotos убирает фото из альбома. Договор: нет — неизвестные ID пропускаются.
func (e *Editor) RemovePhotos(ids []string) model.View {
	return e.change(func(p *model.Project) { spreads.RemovePhotos(p, ids) })
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

// SetSettings: смена формата страницы сдвигает тексты пропорционально (design.Rescale в Editor.edit).
// Договор: нет — размеры и цвет зажимает операция.
func (e *Editor) SetSettings(s model.Settings) (v model.View) {
	t0 := time.Now()
	defer func() { slow(t0, "настройки: %d разворотов", len(v.Spreads)) }()
	return e.change(func(p *model.Project) { spreads.SetSettings(p, s) })
}

// AddFolder добавляет папку; нелокальный путь (UNC, device, относительный) или сверх MaxFolders —
// ошибка, проект не меняется.
// Договор: путь локальный (paths.CheckLocalPath) — операция ему доверяет.
func (e *Editor) AddFolder(dir string) (model.View, error) {
	if err := paths.CheckLocalPath(dir); err != nil {
		return e.View(), err
	}
	return e.try("", func(p *model.Project) error { return spreads.AddFolder(p, dir) })
}

// RemoveFolder убирает папку из списка. Договор: нет — неизвестная папка — холостая правка.
func (e *Editor) RemoveFolder(dir string) model.View {
	return e.change(func(p *model.Project) { spreads.RemoveFolder(p, dir) })
}

// AutoBuild раскладывает фото по новым разворотам, начиная с позиции at.
// perSpread — сколько фото в среднем на разворот. Фото из сеток автосборка не трогает.
// Договор: photos без повторов (DedupePhotos).
func (e *Editor) AutoBuild(photos []model.PhotoInfo, at, perSpread int) (model.View, error) {
	photos, t0, made := model.DedupePhotos(photos), time.Now(), 0
	defer func() { slow(t0, "автосборка: %d фото → %d разворотов", len(photos), made) }()
	return e.try("", func(p *model.Project) (err error) { made, err = spreads.AutoBuild(p, photos, at, perSpread); return })
}

// SplitSpread делит разворот id на два: фото [0:at] остаются, фото [at:] уходят
// в новый разворот сразу после него. at вне 1..len−1 или неизвестный id — ничего
// не делает, как и сетка. У обоих лучшая раскладка для нового состава. Тексты,
// фон и название остаются у первого: координаты текстов привязаны к нему.
// Договор: нет — границы at проверяет операция.
func (e *Editor) SplitSpread(id string, at int) model.View {
	return e.change(func(p *model.Project) { spreads.SplitSpread(p, id, at) })
}

// MergeSpreads клеит разворот id со следующим: фото следующего встают в конец,
// следующий удаляется, его тексты дописываются (координаты те же), цвет фона каждой
// стороны, фото фона и название — первого, без них — второго. Нет следующего, один из двух — сетка, вместе больше MaxSpreadPhotos
// фото или MaxTexts текстов — ничего не делает (без ошибки: интерфейс отключает кнопку заранее,
// а второй клиент мог поменять проект).
// Договор: нет — пределы проверяет операция.
func (e *Editor) MergeSpreads(id string) model.View {
	return e.change(func(p *model.Project) { spreads.MergeSpreads(p, id) })
}

// SetLayout ставит раскладку по ключу (галерея). Битый ключ или уже стоящая
// раскладка — холостая правка. У сетки — «Вернуть авто» с этой раскладкой,
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

// DeleteSelection — «Del» по группе одним шагом undo: тексты, фото (как
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
