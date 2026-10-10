package model

import (
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// FrameOrDefault — действующий кадр фото; nil в PhotoRef — без увеличения и сдвига
// (фото закрывает ячейку по центру).
func (r PhotoRef) FrameOrDefault() Frame {
	if r.Frame == nil {
		return Frame{S: 1}
	}
	return *r.Frame
}

// Copy — копия без общих указателей: Frame и Caption, отданные наружу (Ref, Refs),
// не ведут внутрь проекта — правка копии не обходит Editor.change.
func (r PhotoRef) Copy() PhotoRef {
	if r.Frame != nil {
		f := *r.Frame
		r.Frame = &f
	}
	if r.Caption != nil {
		c := *r.Caption
		r.Caption = &c
	}
	return r
}

// FitTexts держит блоки текста на развороте (design 008 §2.3). Ширина: «по тексту» —
// autoW, при смене стоит точка выравнивания (anchor); закреплённая — 10..2·pageW.
// Положение: блок целиком внутри разворота (высота — по тексту), повёрнутый — по
// описанному прямоугольнику. Идемпотентна: холостая правка остаётся холостой.
func (p *Project) FitTexts() {
	sw, ph := 2*p.Settings.PageW, p.Settings.PageH
	maxW := TextMaxW(p.Settings)
	for _, s := range p.Spreads {
		for _, t := range s.Texts {
			if t.Auto {
				p.SetAutoW(t, AutoW(t, maxW), false)
			} else {
				t.W = geom.ClampF(t.W, MinTextW, sw)
			}
			t.X, t.Y = BoxOf(t).FitIn(sw, ph) // то же у geom.GroupRot
		}
	}
}

// SetAutoW — ширина «по тексту» стала w: блок сдвигается по точке выравнивания
// (повёрнутый — вдоль своей оси). pageFit держит его на странице (pageFitX) и нужен
// только при явной смене (возврат к авто-ширине, стиль, кегль): при вводе текста
// рамку растит редактор от якоря, и зажим у поля заставлял бы её отскакивать.
func (p *Project) SetAutoW(t *TextBlock, w float64, pageFit bool) {
	if w != geom.Round2(t.W) {
		ox := t.X
		dx, dy := geom.Rotate(t.Rot, (t.W-w)*geom.AlignX(t.Style.Align), 0)
		t.X, t.Y = t.X+dx, t.Y+dy
		if pageFit && t.Rot == 0 {
			t.X = p.pageFitX(ox, t.W, t.X, w, geom.AlignX(t.Style.Align))
		}
	}
	t.W = w
}

// pageFitX — у блока «по тексту» сменилась ширина (ox, ow → x, w). Если старый блок
// целиком стоял на одной странице (в её безопасном поле или хотя бы на ней), новый
// тоже держим там: сдвиг по точке выравнивания может вытолкнуть его за край страницы
// или разворота, и ширина, вернувшаяся «по тексту», прыгала бы. Иначе — x как есть.
func (p *Project) pageFitX(ox, ow, x, w, ax float64) float64 {
	pw, m := p.Settings.PageW, SafeMargin
	page := 0.0
	if ox+ow*ax >= pw {
		page = 1
	}
	l, r := page*pw, (page+1)*pw
	for _, in := range []float64{m, 0} { // сначала безопасное поле, потом вся страница
		lo, hi := l+in, r-in-w
		if ox >= lo-1e-6 && ox+ow <= r-in+1e-6 && hi >= lo {
			return geom.ClampF(x, lo, hi)
		}
	}
	return x
}

// Рамка блока текста для geom и ширина «по тексту» (design 008 §3).

// BoxOf — рамка блока t (высота — из кэша переносов).
func BoxOf(t *TextBlock) geom.TextBox {
	return geom.TextBox{X: t.X, Y: t.Y, W: t.W, H: text.Cached(t.Text, t.Style, t.W).H, Rot: t.Rot}
}

// TextMaxW — предел ширины «по тексту»: страница без безопасных полей;
// дальше перенос по словам.
func TextMaxW(s Settings) float64 {
	return math.Max(MinTextW, s.PageW-2*SafeMargin)
}

// AutoW — ширина блока «по тексту»: естественная, округлённая вверх до 0.01
// и ещё +0.01 (Cached округляет ширину, впритык последнее слово ушло бы на
// новую строку), не меньше 1 мм и не больше maxW.
func AutoW(t *TextBlock, maxW float64) float64 {
	n := text.NaturalWidth(t.Text, t.Style)
	return geom.Round2(math.Min(math.Max(math.Ceil(n*100-1e-6)/100+0.01, 1), maxW))
}

const (
	// captionK — доля кегля подписи от startSize: подпись — вспомогательный текст,
	// но 9 пт на странице 300 мм (текст там 34 пт) читалась «мизерной». 0,35 даёт
	// 12 пт на 30×30 и 8 пт на 20×20: заметно крупнее прежних 9 пт, но не спорит с текстом.
	captionK = 0.35
	// CaptionMinSize — меньше в печати не читается, пт.
	CaptionMinSize = 8.0
)

// CaptionSize — кегль новой подписи для страницы высотой pageH мм: captionK от
// startSize, кратно 0,5 пт, не меньше captionMinSize (сверху — ClampStyle).
func CaptionSize(pageH float64) float64 {
	return math.Max(CaptionMinSize, math.Round(StartSize(pageH)*captionK*2)/2)
}

// NewCaptionStyle — стиль новой подписи в альбоме с высотой страницы pageH мм.
func NewCaptionStyle(pageH float64) TextStyle {
	return ClampStyle(TextStyle{Font: text.DefaultFont, Size: CaptionSize(pageH), Color: "#555555", Align: "center"})
}

// DefaultCaptionStyle — стиль новой подписи в альбоме по умолчанию (страница 300 мм).
func DefaultCaptionStyle() TextStyle {
	return NewCaptionStyle(DefaultSettings().PageH)
}

// maxSession — длина ключа сессии ввода от интерфейса.
const maxSession = 32

// ValidSession — ключ сессии от интерфейса: 1..maxSession знаков [A-Za-z0-9-].
func ValidSession(session string) bool {
	if session == "" || len(session) > maxSession {
		return false
	}
	for _, c := range session {
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-'
		if !ok {
			return false
		}
	}
	return true
}

// FindText — блок по ID и разворот, где он стоит.
func (p *Project) FindText(id string) (*Spread, int) {
	for _, s := range p.Spreads {
		for i, t := range s.Texts {
			if t.ID == id {
				return s, i
			}
		}
	}
	return nil, -1
}

// DeleteText — тело DeleteText (и DeleteSelection) внутри change.
func (p *Project) DeleteText(textID string) {
	if s, i := p.FindText(textID); s != nil {
		s.Texts = slices.Delete(s.Texts, i, i+1)
	}
}

// plainStyle — стиль нового блока; кегль заменяет startSize (от формата).
var plainStyle = TextStyle{Font: text.DefaultFont, Size: 14, Color: "#222222", Align: "center"}

// PlainStyle — копия plainStyle: снаружи стиль по умолчанию не подменить.
func PlainStyle() TextStyle { return plainStyle }

// StartSize — кегль нового блока, пт: высота кегля ≈ 1/25 высоты страницы
// (300 мм → 34 пт, 200 мм → 23 пт), целый пт; пределы — ClampStyle.
func StartSize(pageH float64) float64 {
	return math.Round(pageH / 25 * 72 / 25.4)
}
