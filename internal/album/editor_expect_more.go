package album

// Остальные правки агента «если всё ещё то же» (151, design §3): отметки, обложка, фон, стикеры,
// формат. Тела прежних методов — приватные с x *Expect (nil — без сверки); публичные зовут их с
// nil, …If — с &x. Сверка rev и срока — под e.mu, в одном месте с записью (editor_expect.go).

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/backdrop"
	"github.com/IliaSA101/sosninaalbums/internal/album/layers"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/spreads"
)

// ErrNoSheet — лист стикера не найден (или это обложка: стикеры на обложку агенту — 0.35).
var ErrNoSheet = layers.ErrNoSheet

// ErrAnchor — неизвестное место или размер стикера (AddStickersIf).
var ErrAnchor = layers.ErrAnchor

// SetMarksIf — SetMarks со сверкой x под замком: не совпал rev или вышел срок — ошибка, отметки и
// отмена прежние. changed — длина дельты (сколько фото получили другую отметку).
func (e *Editor) SetMarksIf(photos []model.PhotoInfo, patch model.MarkPatch, x Expect) (v model.View, changed int, err error) {
	return e.setMarks(photos, patch, &x)
}

// CreateCoverLookIf — CreateCoverLook со сверкой x. place — куда встало фото (CoverPlace…).
func (e *Editor) CreateCoverLookIf(preset string, ph *model.PhotoInfo, libs map[string]model.PhotoInfo, want *model.LookRef, x Expect) (v model.View, look *model.LookRef, place string, err error) {
	return e.createCoverLook(preset, ph, libs, want, &x)
}

// CoverPhotoIf — фото ph на лицевую обложки, которая уже есть (151, 0.34-151-16): пустое место
// лицевой, иначе кадр заменяется в наибольшем месте (копией, прежний кадр — в ленту), мест нет —
// фоном (в wrap — через корешок). Развороты не трогаются. Тот же файл уже на лицевой (по BaseID,
// в любом месте или на фоне) — холостая: changed false, place — где он стоит. Обложки нет — cover.ErrNone.
func (e *Editor) CoverPhotoIf(ph model.PhotoInfo, x Expect) (v model.View, place string, changed bool, err error) {
	v, err = e.editWith("", false, func(p *model.Project) (err error) {
		place, err = coverPhotoOnce(p, &ph)
		return err
	}, nil, editOpts{expect: &x})
	if err != nil {
		return v, "", false, err
	}
	return v, place, v.Rev != x.Rev, nil
}

// BgOutcome — итог ApplyBackgroundToIf (описание — backdrop.Outcome).
type BgOutcome = backdrop.Outcome

// ApplyBackgroundToIf — ApplyBackgroundTo со сверкой x и защитой фото клиента (0.34-151-19): без
// replace цели, которые сняли бы фото фона клиента, пропускаются до правки; остальные — одним
// шагом. Все пропущены — холостая. В «Недавние» не пишется (решает вызывающий). Ошибка цели,
// области или источника — как у ApplyBackgroundTo: ничего не применено.
func (e *Editor) ApplyBackgroundToIf(targets []model.GridTarget, src model.BgSource, replace bool, x Expect) (model.View, BgOutcome, error) {
	var out BgOutcome
	v, err := e.editWith("", false, func(p *model.Project) (err error) {
		out, err = backdrop.ApplyToAgent(p, targets, src, replace)
		return err
	}, nil, editOpts{expect: &x})
	if err != nil {
		out = BgOutcome{}
	}
	return v, out, err
}

// StickerPut — один стикер пачки AddStickersIf (описание — layers.StickerPut).
type StickerPut = layers.StickerPut

// AddStickersIf — пачка стикеров агента одним шагом со сверкой x (design §1.5, тело —
// layers.AddStickers): пачка целиком или никак, ошибки — ErrNoSheet, layers.ErrAnchor,
// model.ErrStickers, model.ErrStickersTotal, model.ErrSticker. Пустая пачка — холостая.
// ids — новые стикеры в порядке puts.
func (e *Editor) AddStickersIf(puts []StickerPut, x Expect) (v model.View, ids []string, err error) {
	v, err = e.editWith("", false, func(p *model.Project) (err error) {
		ids, err = layers.AddStickers(p, puts)
		return err
	}, nil, editOpts{expect: &x})
	if err != nil {
		ids = nil
	}
	return v, ids, err
}

// ApplyPrintIf — ApplyPrint со сверкой x (формат и изделие обложки одним шагом).
func (e *Editor) ApplyPrintIf(s model.Settings, preset string, x Expect) (model.View, error) {
	return e.applyPrint(s, preset, &x)
}

// SetSettingsIf — SetSettings со сверкой x: изделие обложки остаётся, шаблон идёт за форматом.
func (e *Editor) SetSettingsIf(s model.Settings, x Expect) (model.View, error) {
	return e.setSettings(s, &x)
}

// Families — семейство раскладки (spreads.FamilyOf) каждого листа из ids, в их порядке;
// неизвестный ID, сетка, виньетка и лист без фото — "". Чтение под замком, проект не меняется.
func (e *Editor) Families(ids []string) []string {
	out := make([]string, len(ids))
	e.read(func(p *model.Project) {
		for i, id := range ids {
			_, s := p.Find(id)
			out[i] = spreads.FamilyOf(p, s)
		}
	})
	return out
}
