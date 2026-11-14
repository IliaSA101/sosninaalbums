package printshop

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Plans — планы типографий для окна «Печать» (App.PrintPlans, 123 §1.5): по плану на каждую,
// пункт, с которого окно открывается, отмечен Default. last — последний выбор окна ("" — «Обычная»).
// Лист 031 — пустой список: типографии такие листы не принимают. Проект только читается.
func Plans(p *model.Project, albumPath, last string) []Plan {
	out := []Plan{}
	if p.Settings.Sheet {
		return out
	}
	for _, prof := range Profiles() {
		out = append(out, Check(p, albumPath, prof))
	}
	cover, page := AlbumKinds(p)
	def := DefaultKind(out, last, cover, page)
	for i := range out {
		out[i].Default = def != "" && out[i].Kind == def
	}
	return out
}

// AlbumKinds — типографии альбома по его данным: cover — чей пресет у обложки (у Нетпринта и
// «Своих размеров» выгрузки нет — ""); page — единственная типография с выгрузкой, чья кнопка формата
// совпала со страницей целиком (размер с допуском, зона, обрез: 21×30 Фотосферы и Фабрики
// различаются), иначе "".
func AlbumKinds(p *model.Project) (cover, page string) {
	if p.Cover != nil {
		if pr, ok := model.CoverPresetFor(p.Cover.Spec.Preset, p.Settings); ok {
			for _, prof := range Profiles() {
				if prof.Group == pr.Group {
					cover = prof.Kind
				}
			}
		}
	}
	var fit []string
	for _, prof := range Profiles() {
		if model.PageFitsGroup(p.Settings, prof.Group) {
			fit = append(fit, prof.Kind)
		}
	}
	return cover, soleKind(fit)
}

// soleKind — единственная типография из подошедших; две и более (или ни одной) — "": по странице
// не выбрать. Отдельно от таблиц: сегодня формат страницы двум типографиям сразу не подходит,
// ветку держит тест.
func soleKind(kinds []string) string {
	if len(kinds) == 1 {
		return kinds[0]
	}
	return ""
}

// DefaultKind — вид «Печати», на котором открывается окно: "" — «Обычная выгрузка» (123 §1.5).
// cover и page — из AlbumKinds, last — последний выбор окна. Берётся первый пункт без запрета:
//  1. типография обложки — готовый альбом открывается на своей;
//  2. последний выбор, в том числе «Обычная» (у неё запрета нет): привычка фотографа сильнее догадки по странице;
//  3. типография страницы.
//
// Запрет у всех — последний выбор, иначе типография обложки, иначе страницы, иначе «Обычная»:
// запрет объяснит, что менять. Чистая функция.
func DefaultKind(plans []Plan, last, cover, page string) string {
	byKind := func(kind string) (Plan, bool) {
		for _, pl := range plans {
			if pl.Kind == kind && kind != "" {
				return pl, true
			}
		}
		return Plan{}, false
	}
	free := func(kind string) bool {
		pl, ok := byKind(kind)
		return ok && len(pl.Block) == 0
	}
	if free(cover) {
		return cover
	}
	if last == "" {
		return ""
	}
	if free(last) {
		return last
	}
	if free(page) {
		return page
	}
	for _, kind := range []string{last, cover, page} {
		if _, ok := byKind(kind); ok {
			return kind
		}
	}
	return ""
}
