package layers

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// Группа объектов: перенос, масштаб, поворот и масштаб по осям рамки (спеки 009, 018).
// Расчёт — чистые geom.GroupXf, GroupRot и GroupAxes (зеркала frontend geom/groupxf.ts,
// grouprot.ts, groupaxes.ts; паритет testdata/parity/groupxf.json), запись в проект — здесь.
// Угол рамки группы — состояние UI, в проекте его нет.

// TransformGroup — тело Editor.TransformGroup над проектом (аргументы уже проверены).
func TransformGroup(p *model.Project, spreadID string, ids []string, k, ax, ay, dx, dy float64) {
	groupEdit(p, spreadID, ids, func(p *model.Project, s *model.Spread) { transformGroup(p, s, ids, k, ax, ay, dx, dy) })
}

// GroupMergeKey — ключ слияния серии стрелок группы: сессия merge; у «Авто» с фото
// в группе слияния нет (закрепление не должно влипнуть в чужую серию). p не меняет.
func GroupMergeKey(p *model.Project, spreadID string, ids []string, merge string) string {
	if autoWithPhotos(p, spreadID, ids) {
		return ""
	}
	return model.GridMerge(merge)
}

// RotateGroup — тело Editor.RotateGroup над проектом (аргументы уже проверены).
func RotateGroup(p *model.Project, spreadID string, ids []string, deg, cx, cy float64) {
	groupEdit(p, spreadID, ids, func(p *model.Project, s *model.Spread) {
		pls, txs := groupMembers(s, ids)
		in := geom.GxRotIn{SW: 2 * p.Settings.PageW, PH: p.Settings.PageH, Deg: deg, CX: cx, CY: cy}
		in.Places, in.Texts = gxMembers(pls, txs)
		out := geom.GroupRot(in)
		for i, pl := range pls {
			np := *pl // копия целиком: поля места, которых здесь нет, не теряются
			r := out.Places[i]
			np.X, np.Y, np.Rot = r.X, r.Y, r.Rot
			p.ClampPlace(&np)
			*pl = np
		}
		for i, t := range txs {
			o := out.Texts[i]
			t.X, t.Y, t.Rot = o.X, o.Y, o.Rot
		}
	})
}

// ScaleGroupAxes — тело Editor.ScaleGroupAxes над проектом (аргументы уже проверены).
func ScaleGroupAxes(p *model.Project, spreadID string, ids []string, kx, ky, phi, ax, ay float64) {
	groupEdit(p, spreadID, ids, func(p *model.Project, s *model.Spread) {
		if kx == ky {
			transformGroup(p, s, ids, kx, ax, ay, 0, 0)
			return
		}
		pls, txs := groupMembers(s, ids)
		in := geom.GxAxesIn{SW: 2 * p.Settings.PageW, PH: p.Settings.PageH, KX: kx, KY: ky, Phi: phi, AX: ax, AY: ay}
		in.Places, in.Texts = gxMembers(pls, txs)
		out := geom.GroupAxes(in)
		for i, pl := range pls {
			np := *pl
			r := out.Places[i]
			np.X, np.Y, np.W, np.H = r.X, r.Y, r.W, r.H
			p.ClampPlace(&np)
			*pl = np
		}
		for i, t := range txs {
			o := out.Texts[i]
			t.X, t.Y, t.W = o.X, o.Y, o.W
			if t.W < model.MinTextW { // fitTexts поднял бы ширину от угла рамки — растим вокруг точки выравнивания
				dx, dy := geom.Rotate(t.Rot, (t.W-model.MinTextW)*o.Al, 0)
				t.X, t.Y, t.W = geom.Round2(t.X+dx), geom.Round2(t.Y+dy), model.MinTextW
			}
			if o.Size != in.Texts[i].Size {
				t.Style.Size = o.Size
				t.Style = model.ClampStyle(t.Style)
			}
			if t.Auto { // ширина по тексту нового кегля, точка выравнивания стоит
				p.SetAutoW(t, model.AutoW(t, model.TextMaxW(p.Settings)), false)
			}
		}
	})
}

// transformGroup — тело TransformGroup над разворотом s (у «Авто» с фото — уже закреплённым).
func transformGroup(p *model.Project, s *model.Spread, ids []string, k, ax, ay, dx, dy float64) {
	pls, txs := groupMembers(s, ids)
	in := geom.GxIn{SW: 2 * p.Settings.PageW, PH: p.Settings.PageH, K: k, AX: ax, AY: ay, DX: dx, DY: dy}
	in.Places, in.Texts = gxMembers(pls, txs)
	out := geom.GroupXf(in)
	if out.K == 1 && out.DX == 0 && out.DY == 0 {
		return
	}
	for i, pl := range pls {
		np := *pl // копия целиком: поля места, которых здесь нет, не теряются
		r := out.Places[i]
		np.X, np.Y, np.W, np.H = r.X, r.Y, r.W, r.H
		p.ClampPlace(&np)
		*pl = np
	}
	for i, t := range txs {
		o := out.Texts[i]
		t.X, t.Y = o.X, o.Y
		if o.Size != in.Texts[i].Size {
			t.Style.Size = o.Size
			t.Style = model.ClampStyle(t.Style)
		}
		if t.Auto {
			t.W = model.AutoW(t, model.TextMaxW(p.Settings)) // та же ширина, что насчитает fitTexts
		} else {
			t.W = o.W
		}
	}
}

// groupEdit — правка группы одним шагом undo: у «Авто» с фото в группе разворот
// закрепляется той же правкой (withGrid), одни тексты не закрепляют.
func groupEdit(p *model.Project, spreadID string, ids []string, run func(p *model.Project, s *model.Spread)) {
	_, s := p.Find(spreadID)
	if s == nil {
		return
	}
	fn := func() error { run(p, s); return nil }
	if len(s.Grid) == 0 && hasPhoto(s, ids) {
		_ = p.WithGrid(s, fn)
		return
	}
	_ = fn()
}

// groupMembers — места и тексты группы по ids (ref мест и ID текстов разворота s),
// без повторов; чужое и неизвестное пропускается.
func groupMembers(s *model.Spread, ids []string) (pls []*model.Place, txs []*model.TextBlock) {
	for _, ref := range ids {
		if pl := s.PlaceRef(ref); pl != nil {
			if !slices.Contains(pls, pl) {
				pls = append(pls, pl)
			}
			continue
		}
		if i := slices.IndexFunc(s.Texts, func(t *model.TextBlock) bool { return t.ID == ref }); i >= 0 && !slices.Contains(txs, s.Texts[i]) {
			txs = append(txs, s.Texts[i])
		}
	}
	return pls, txs
}

// gxMembers — члены группы для geom: высота текста — по переносам, кегль — зажатый.
func gxMembers(pls []*model.Place, txs []*model.TextBlock) ([]geom.RotRect, []geom.GxText) {
	var rs []geom.RotRect
	var ts []geom.GxText
	for _, pl := range pls {
		rs = append(rs, pl.Rect())
	}
	for _, t := range txs {
		ts = append(ts, geom.GxText{X: t.X, Y: t.Y, W: t.W, H: model.BoxOf(t).H, Rot: t.Rot,
			Size: t.Style.Clamped().Size, Al: geom.AlignX(t.Style.Align)})
	}
	return rs, ts
}

// hasPhoto — среди ids есть фото разворота s.
func hasPhoto(s *model.Spread, ids []string) bool {
	return slices.ContainsFunc(ids, func(r string) bool { return slices.Contains(s.Photos, r) })
}

// autoWithPhotos — разворот «Авто» и среди ids есть его фото (группа с местами).
func autoWithPhotos(p *model.Project, spreadID string, ids []string) bool {
	_, s := p.Find(spreadID)
	return s != nil && len(s.Grid) == 0 && hasPhoto(s, ids)
}
