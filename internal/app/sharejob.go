package app

import (
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/share"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// shareEstimateBPP — JPEG страниц листалки (q82), байт на пиксель, занятый фото. Замер 29.10 на
// стоковых фото, страница 300 мм → 1606 px: 0,128…0,167 (QA-альбом 8 разворотов — 0,128), берём 0,14.
// Пустое поле страницы почти не весит, поэтому считаем по площади фото, а не всей страницы (0.32-146-T5-13).
const shareEstimateBPP = 0.14

// shareJob — снимок текущего альбома для службы выгрузки по ссылке (146): отказы (нет файла, лист
// виньетки, нет разворотов, пропавшие фото), страницы, хэш сцены и вопросы до выгрузки. Служба
// sharer не видит проект (depguard) — она берёт только это значение. Проект не меняется.
func (a *App) shareJob() share.Job {
	ed := a.editor()
	gen := text.Gen()                     // до снимка: смена шрифтов между ними не проскочит (export.PinFonts)
	path, p, v, sealed := ed.SnapshotAt() // путь, снимок и печать — под одним замком: от одного альбома
	// Отпечаток — от снимка: страницы и сверка записи от одного проекта.
	job := share.Job{Path: path, UID: p.UID, Unsealed: !sealed, Title: "Альбом"}
	if job.Path != "" {
		job.Title = strings.TrimSuffix(filepath.Base(job.Path), filepath.Ext(job.Path))
	}
	switch {
	case job.Path == "":
		job.Block, job.Save = "Сначала сохраните альбом — ссылка хранится у файла альбома", true
		return job
	case p.Settings.Sheet:
		job.Block = "Лист виньетки нельзя отправить по ссылке"
		return job
	case len(v.Spreads) == 0:
		job.Block = "В альбоме нет разворотов"
		return job
	}
	spec, err := export.NewWebSpec(p.Settings, v)
	if err != nil {
		job.Refuse = err.Error()
		return job
	}
	cover := export.NewCoverJob(p, v, spec.DPI, spec.Quality)
	job.Spec, job.Cover = spec, cover
	job.Pages = len(export.WebPageNames(len(v.Spreads), cover != nil))
	job.Bytes = shareEstimate(spec, v, job.Pages)
	job.Spreads = export.FromView(p, v)
	export.PinFonts(job.Spreads, gen)
	stat := func(path string) (size, mtime int64, ok bool) {
		st, err := library.StatLocal(path)
		if err != nil {
			return 0, 0, false
		}
		return st.Size(), st.ModTime().UnixNano(), true
	}
	if job.Scene, err = share.SceneHash(job.Spreads, cover, spec, stat); err != nil {
		job.Refuse = err.Error()
		return job
	}

	// Те же отказы, что у экспорта: пропавшие фото и рамки.
	pl := export.NewPlan(nil, len(v.Spreads), false)
	if cover != nil {
		pl = pl.WithCover(cover)
	}
	g := p.Geo(nil)
	pageW := g.W / 2
	sel := pl.SelectedPhotos(v, pageW)
	if err := export.MissingPhotos(p, sel, a.exists); err != nil {
		job.Refuse = err.Error()
		return job
	}
	if err := pl.MissingBorderFiles(v, pageW, a.exists); err != nil {
		job.Refuse = err.Error()
		return job
	}
	job.Questions = a.shareQuestions(p, v, cover, pl, sel, pageW)
	return job
}

// shareEstimate — оценка размера страниц без рендера: пиксели страницы × доля, занятая фото
// (площадь Rects в пределах разворота), × shareEstimateBPP. Страницы обложки — по средней доле
// разворотов; доля 0 (разворот без фото) даёт 0 байт — белая страница и фон весят единицы КБ.
func shareEstimate(spec export.WebSpec, v model.View, pages int) int64 {
	tw, th := spec.Trimmed()
	px := tw / 25.4 * float64(spec.DPI) * th / 25.4 * float64(spec.DPI)
	var sum float64
	for _, sp := range v.Spreads {
		if a := sp.W * sp.H; a > 0 {
			var occ float64
			for _, r := range sp.Rects {
				occ += max(r.W, 0) * max(r.H, 0)
			}
			sum += min(occ/a, 1)
		}
	}
	if len(v.Spreads) == 0 || !(px > 0) {
		return 0
	}
	// Каждый разворот — две страницы одной доли; остальные (обложка) — средняя доля.
	spreadPages := min(2*len(v.Spreads), pages)
	avg := sum / float64(len(v.Spreads))
	cover := float64(pages - spreadPages)
	return int64(shareEstimateBPP * px * (2*sum + cover*avg))
}

// shareQuestions — вопросы до выгрузки, как у экспорта, кроме пустых разворотов и папок (design §3.3
// п. 3); считаются лениво, когда фотограф нажал «Выгрузить».
func (a *App) shareQuestions(p *model.Project, v model.View, cover *export.CoverJob, pl export.Plan, sel map[string]bool, pageW float64) func() []string {
	return func() []string {
		var qs []string
		add := func(q string) {
			if q != "" {
				qs = append(qs, q)
			}
		}
		_, other := a.colorReport(export.PickPhotos(p, sel))
		add(export.ColorWarning(other))
		drafts := export.DraftSummaryFor(v, &pl, pageW)
		drafts.TakeBlank(export.BlankSpreads(v, &pl), cover != nil && export.BlankCover(v)) // как у экспорта: пустые места заготовок — в сводку
		add(export.FieldsWarning(export.PlanFields(v, &pl)))
		add(export.FontsWarning(export.PlanFonts(v, &pl)))
		add(export.EmptyDraftWarning(drafts))
		return qs
	}
}
