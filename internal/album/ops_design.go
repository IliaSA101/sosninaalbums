package album

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/album/design"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/names"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// Обёртки оформления (спека 006, 008, 023): охранная проверка и очистка
// аргументов — здесь, до change; тело правки — design. Строка «Договор» у каждой
// обёртки: что проверено до правки, на что тело может рассчитывать.

// textMerge — ключ слияния шагов отмены для сессии ввода: всё от входа в блок
// до выхода — один шаг. Пустая или чужая по виду сессия — без слияния.
func textMerge(session string) string {
	if !model.ValidSession(session) {
		return ""
	}
	return "text:" + session
}

// Договор: проверок нет; S, X, Y зажимает тело (ClampFrame), чужое фото пропускается.
func (e *Editor) SetFrame(photoID string, s, x, y float64) model.View {
	return e.change(func(p *model.Project) { design.SetFrame(p, photoID, s, x, y) })
}

// Договор: NaN/Inf в наклоне — холостая правка; зажим угла делает тело.
func (e *Editor) SetFrameAngle(photoID string, tilt float64) model.View {
	if math.IsNaN(tilt) || math.IsInf(tilt, 0) {
		return e.View()
	}
	return e.change(func(p *model.Project) { design.SetFrameAngle(p, photoID, tilt) })
}

// Договор: quarters ∈ {−1, 1, 2}, иначе холостая правка; чужие ID пропускает тело.
func (e *Editor) TurnPhotos(ids []string, quarters int) model.View {
	if quarters != -1 && quarters != 1 && quarters != 2 {
		return e.View()
	}
	return e.change(func(p *model.Project) { design.TurnPhotos(p, ids, quarters) })
}

// Договор: проверок нет.
func (e *Editor) ResetFrame(photoID string) model.View {
	return e.change(func(p *model.Project) { design.ResetFrame(p, photoID) })
}

// Договор: axis ∈ {"h", "v"}, иначе холостая правка; чужие ID пропускает тело.
func (e *Editor) SetFlip(ids []string, axis string) model.View {
	if axis != "h" && axis != "v" {
		return e.View()
	}
	return e.change(func(p *model.Project) { design.SetFlip(p, ids, axis) })
}

// Договор: цвет зажат ClampColor (неверный непустой — холостая правка), "" снимает цвет;
// scope проверяет тело.
func (e *Editor) SetBackground(spreadID, scope, color string) model.View {
	c, ok := model.ClampColor(color)
	if !ok && color != "" {
		return e.View()
	}
	return e.change(func(p *model.Project) { design.SetBackground(p, spreadID, scope, c) })
}

// Договор: текст очищен CleanText (MaxTextRunes). session — как у AddText.
func (e *Editor) SetText(textID, txt, session string) model.View {
	txt = model.CleanText(txt, model.MaxTextRunes)
	return e.apply(textMerge(session), func(p *model.Project) { design.SetText(p, textID, txt) })
}

// Договор: стиль зажат ClampStyle (гарнитура из таблицы, кегль 6..200 pt).
func (e *Editor) SetTextStyle(textID string, st model.TextStyle) model.View {
	st = model.ClampStyle(st)
	return e.change(func(p *model.Project) { design.SetTextStyle(p, textID, st) })
}

// Договор: проверок нет.
func (e *Editor) DeleteText(textID string) model.View {
	return e.change(func(p *model.Project) { design.DeleteText(p, textID) })
}

// Договор: текст очищен CleanText (MaxCaptionRunes). session — как у SetText.
func (e *Editor) SetCaption(photoID, txt, session string) model.View {
	txt = model.CleanText(txt, model.MaxCaptionRunes)
	return e.apply(textMerge(session), func(p *model.Project) { design.SetCaption(p, photoID, txt) })
}

// Договор: стиль зажат ClampStyle, отступ — ClampCaptionPad (0..20 мм, шаг 0,1; NaN/Inf и
// 1 мм — nil); подписи нет — холостая правка (тело).
func (e *Editor) SetCaptionStyle(photoID string, st model.TextStyle, over bool, pad float64) model.View {
	st, pp := model.ClampStyle(st), model.ClampCaptionPad(&pad)
	return e.change(func(p *model.Project) { design.SetCaptionStyle(p, photoID, st, over, pp) })
}

// Поля подписей (030, тела — names).

// Договор: имя зажато ClampPerson (все пусты — nil: снова из имени файла); session — как у
// SetText: набор в поле — один шаг отмены.
func (e *Editor) SetPerson(photoID string, who model.Person, session string) model.View {
	pp := model.ClampPerson(who)
	return e.apply(textMerge(session), func(p *model.Project) { names.SetPerson(p, photoID, pp) })
}

// Person — имя фото для карточки (своё или из имени файла); ok = false — фото не в альбоме.
func (e *Editor) Person(photoID string) (pv model.PersonView, ok bool) {
	e.read(func(p *model.Project) {
		var r model.PhotoRef
		if r, ok = p.Photos[photoID]; ok {
			pv = model.NameView(r)
		}
	})
	return pv, ok
}

// PasteNames раскладывает вставленный список имён по фото листа одним шагом отмены.
// extra — лишних строк, missing — фото без имени после вставки.
// Договор: список разобран и ограничен names.Parse (64 КБ, 1000 строк) до правки.
func (e *Editor) PasteNames(spreadID, list string, firstFirst bool) (v model.View, extra, missing int, err error) {
	people, err := names.Parse(list, firstFirst)
	if err != nil {
		return e.View(), 0, 0, err
	}
	v, err = e.try("", func(p *model.Project) (err error) {
		extra, missing, err = names.Paste(p, spreadID, people)
		return err
	})
	return v, extra, missing, err
}

// Договор: стиль листа зажат ClampSheetCaps (стиль, отступ 0..20, NaN — 1 мм; шаблон —
// CleanText); off — c не смотрится.
func (e *Editor) SetSheetCaps(spreadID string, on bool, c model.SheetCaps) model.View {
	c = model.ClampSheetCaps(c)
	return e.change(func(p *model.Project) { names.SetSheetCaps(p, spreadID, on, c) })
}

// AddText добавляет блок «Текст» на страницу page; session — сессия ввода: добавление
// и набор текста отменяются одним шагом.
// Договор: page зажата в 0..1; предел MaxTexts проверяет тело (ошибка).
func (e *Editor) AddText(spreadID string, page int, session string) (model.View, error) {
	page = min(max(page, 0), 1)
	return e.try(textMerge(session), func(p *model.Project) error { return design.AddText(p, spreadID, page) })
}

// Договор: проверок нет; NaN/Inf в x, y, w тело заменяет прежними значениями.
func (e *Editor) SetTextBox(textID string, x, y, w float64) model.View {
	return e.change(func(p *model.Project) { design.SetTextBox(p, textID, x, y, w) })
}

// Договор: NaN/Inf в угле — холостая правка; угол нормализован geom.NormDeg.
func (e *Editor) SetTextAngle(textID string, deg float64) model.View {
	if math.IsNaN(deg) || math.IsInf(deg, 0) {
		return e.View()
	}
	deg = geom.NormDeg(deg)
	return e.change(func(p *model.Project) { design.SetTextAngle(p, textID, deg) })
}

// Договор: size конечный и > 0, иначе холостая правка; кегль зажимает тело.
func (e *Editor) SetTextScale(textID string, size, x, y float64) model.View {
	if math.IsNaN(size) || math.IsInf(size, 0) || size <= 0 {
		return e.View()
	}
	return e.change(func(p *model.Project) { design.SetTextScale(p, textID, size, x, y) })
}

// Договор: проверок нет.
func (e *Editor) SetTextAuto(textID string) model.View {
	return e.change(func(p *model.Project) { design.SetTextAuto(p, textID) })
}

// Договор: название очищено CleanName.
func (e *Editor) SetSpreadName(spreadID, name string) model.View {
	name = model.CleanName(name)
	return e.change(func(p *model.Project) { design.SetSpreadName(p, spreadID, name) })
}
