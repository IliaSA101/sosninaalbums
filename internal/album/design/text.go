package design

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Блоки текста: набор, оформление, добавление, рамка, поворот, ширина «по тексту»,
// названия разворотов (спеки 006, 008).

// SetText меняет текст блока; пустой (одни пробелы) — блок удаляется. Новая строка делает
// заготовку (131) обычной надписью; та же строка — холостая, признак стоит.
// txt — уже очищенный (CleanText).
func SetText(p *model.Project, textID, txt string) {
	s, i := p.FindText(textID)
	if s == nil {
		return
	}
	if strings.TrimSpace(txt) == "" {
		s.Texts = slices.Delete(s.Texts, i, i+1)
		return
	}
	if t := s.Texts[i]; t.Text != txt {
		t.Text, t.Draft = txt, false
	}
}

// SetTextDraft ставит или снимает признак заготовки (131 design §6) у надписи textID листа
// spreadID. Снятие («Сделать обычной надписью») убирает скобки, если весь текст — одна пара
// «[…]» с непустым содержимым: «[Имя] и [Имя]» и «[ ]» не трогаются. Тот же признак — холостая.
func SetTextDraft(p *model.Project, spreadID, textID string, draft bool) {
	s := p.Sheet(spreadID)
	if s == nil {
		return
	}
	i := slices.IndexFunc(s.Texts, func(t *model.TextBlock) bool { return t.ID == textID })
	if i < 0 || s.Texts[i].Draft == draft {
		return
	}
	t := s.Texts[i]
	t.Draft = draft
	if inner, ok := bracketPair(t.Text); !draft && ok {
		t.Text = inner
	}
}

// bracketPair — содержимое без пробелов по краям, если весь s — одна пара скобок без вложенных
// (^\[[^\[\]]*\]$, 131 К1-r2-7) и внутри не пусто.
func bracketPair(s string) (string, bool) {
	n := len(s)
	if n < 2 || s[0] != '[' || s[n-1] != ']' || strings.ContainsAny(s[1:n-1], "[]") {
		return "", false
	}
	inner := strings.TrimSpace(s[1 : n-1])
	return inner, inner != ""
}

// SetTextStyle меняет оформление блока: st — уже зажатый (ClampStyle: гарнитура из
// таблицы, кегль 6..200 pt).
func SetTextStyle(p *model.Project, textID string, st model.TextStyle) {
	if s, i := p.FindText(textID); s != nil {
		t := s.Texts[i]
		t.Style = st
		if t.Auto {
			p.SetAutoW(s, t, p.SheetAutoW(s, t), true)
		}
	}
}

func DeleteText(p *model.Project, textID string) { p.DeleteText(textID) }

// AddText добавляет блок «Текст» по центру страницы page листа (0 — левая, 1 — правая;
// уже зажата; у обложки — сторонки), ширина — по тексту. Новый блок — последний в
// SpreadView.Texts. Больше MaxTexts — ошибка.
func AddText(p *model.Project, spreadID string, page int) error {
	s := p.Sheet(spreadID)
	if s == nil {
		return nil
	}
	if len(s.Texts) >= model.MaxTexts {
		return fmt.Errorf("на развороте не может быть больше %d текстов", model.MaxTexts)
	}
	s.Texts = append(s.Texts, newText(p, s, page))
	return nil
}

// newText — блок-заготовка по центру страницы page листа s (Geo.Pages). Кегль — от высоты
// страницы альбома: на обложке текст того же размера, что внутри.
func newText(p *model.Project, s *model.Spread, page int) *model.TextBlock {
	pg := p.Geo(s).Page(page)
	t := &model.TextBlock{ID: model.NewID(), Auto: true, Text: "Текст", Style: model.PlainStyle()}
	t.Style.Size = model.StartSize(p.Settings.PageH)
	t.Style = model.ClampStyle(t.Style)
	t.W = p.SheetAutoW(s, t)
	h := text.Cached(t.Text, t.Style, t.W).H
	t.X = geom.Round2(pg.X + (pg.W-t.W)/2)
	t.Y = geom.Round2(pg.Y + math.Max(0, (pg.H-h)/2))
	return t
}

// SetTextBox ставит рамку блока (мм разворота, угол до поворота); NaN/Inf — прежнее
// значение. Другая ширина закрепляет её (потянули ручку); перенос шлёт прежнюю —
// режим «по тексту» цел. Блок за разворот не выходит (fitTexts в change).
func SetTextBox(p *model.Project, textID string, x, y, w float64) {
	s, i := p.FindText(textID)
	if s == nil {
		return
	}
	t := s.Texts[i]
	x, y, w = geom.Round2(model.Finite(x, t.X)), geom.Round2(model.Finite(y, t.Y)), geom.Round2(model.Finite(w, t.W))
	if w != geom.Round2(t.W) {
		t.Auto = false
	}
	t.X, t.Y, t.W = x, y, w
}

// SetTextAngle поворачивает блок на deg по часовой вокруг его центра (центр стоит,
// угол рамки пересчитывается). deg — уже нормализованный (geom.NormDeg); NaN/Inf
// и тот же угол — холостая правка.
func SetTextAngle(p *model.Project, textID string, deg float64) {
	s, i := p.FindText(textID)
	if s == nil || s.Texts[i].Rot == deg {
		return
	}
	t := s.Texts[i]
	x, y := model.BoxOf(t).RotateAbout(deg)
	t.X, t.Y, t.Rot = geom.Round2(x), geom.Round2(y), deg
}

// SetTextScale — угловая ручка: новый кегль size (пт, зажимается ClampStyle, шаг 0.1) и
// новый левый верхний угол рамки (x, y) до поворота — их считает интерфейс так, чтобы
// противоположный тянутому угол стоял. Закреплённая ширина растёт вместе с кеглем
// (×size/было), «по тексту» пересчитывается; fitTexts держит блок в развороте.
// NaN/Inf в x, y — прежние; NaN/Inf/≤0 в size, тот же кегль и те же x, y — холостая правка.
func SetTextScale(p *model.Project, textID string, size, x, y float64) {
	s, i := p.FindText(textID)
	if s == nil {
		return
	}
	t := s.Texts[i]
	x, y = geom.Round2(model.Finite(x, t.X)), geom.Round2(model.Finite(y, t.Y))
	old := t.Style.Clamped().Size
	st := t.Style
	st.Size = math.Round(size*10) / 10
	st = model.ClampStyle(st)
	if st.Size == old && x == geom.Round2(t.X) && y == geom.Round2(t.Y) {
		return
	}
	t.X, t.Y = x, y
	if st.Size == old {
		return
	}
	t.Style = st
	if t.Auto {
		t.W = p.SheetAutoW(s, t) // та же ширина, что насчитает fitTexts
	} else {
		t.W = geom.Round2(t.W * st.Size / old)
	}
}

// SetTextAuto — ширина блока снова по тексту (её считает fitTexts); уже так — холостая.
func SetTextAuto(p *model.Project, textID string) {
	if s, i := p.FindText(textID); s != nil {
		t := s.Texts[i]
		t.Auto = true
		p.SetAutoW(s, t, p.SheetAutoW(s, t), true)
	}
}

// SetSpreadName — название разворота (name — уже очищенное, CleanName); "" снимает.
// То же — холостая.
func SetSpreadName(p *model.Project, spreadID, name string) {
	if _, s := p.Find(spreadID); s != nil {
		s.Name = name
	}
}

// Rescale — шаг конвейера edit: поменялся формат страницы: блоки остаются на том же месте
// страницы пропорционально (x и w — по ширине, y — по высоте); стикеры (048) — центр так же,
// ширина — по ширине, высота — по высоте (пропорцию вернёт FitStickers), стикер, стоявший
// целиком на листе, остаётся на нём (rescaleSticker, 0.27-129-107). Только развороты:
// обложку пересчитывает свой Reflow (032 §3).
func Rescale(p *model.Project, before *model.Project) {
	og, ng := before.Geo(nil), p.Geo(nil)
	ow, oh, nw, nh := og.W, og.H, ng.W, ng.H
	if ow == nw && oh == nh || !(ow > 0) || !(oh > 0) {
		return
	}
	kx, ky := nw/ow, nh/oh
	for _, s := range p.Spreads {
		for _, t := range s.Texts {
			t.X, t.W, t.Y = t.X*kx, t.W*kx, t.Y*ky
		}
		for _, st := range s.Stickers {
			rescaleSticker(p, s, st, ow, oh, nw, nh)
		}
	}
}
