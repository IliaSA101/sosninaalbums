package album

import (
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/album/design"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
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

// Договор: стиль зажат ClampStyle; подписи нет — холостая правка (тело).
func (e *Editor) SetCaptionStyle(photoID string, st model.TextStyle, over bool) model.View {
	st = model.ClampStyle(st)
	return e.change(func(p *model.Project) { design.SetCaptionStyle(p, photoID, st, over) })
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
