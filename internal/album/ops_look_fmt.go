package album

import (
	"slices"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/cover"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/spreads"
)

// lookFit — макетный разворот, чьи места вписываются заново при смене формата.
type lookFit struct {
	id   string
	step model.LookStep
}

// SetSettingsLook — SetSettings для альбома с макетом look (Project.Look; nil — макета нет на
// этом ПК, места тянутся как всегда): нетронутые места макетных разворотов (совпадают со шагом на
// прежнем формате) вписываются по LookBoxes нового формата (0.27-131-23), тронутые и «Фоны по
// серии» тянет grid.Rescale. Одна правка, один шаг отмены. Договор: как у SetSettings.
func (e *Editor) SetSettingsLook(s model.Settings, look *model.AlbumTpl) (v model.View) {
	t0 := time.Now()
	defer func() { slow(t0, "настройки: %d разворотов", len(v.Spreads)) }()
	v, _ = e.formatLook(clampLookPtr(look), false, func(p *model.Project) error {
		old := p.Settings
		spreads.SetSettings(p, s)
		cover.FollowFormat(p, old)
		return nil
	})
	return v
}

// formatLook — правка fn, меняющая формат альбома, и вписывание макета в новый формат. c — макет
// явно; nil — его ищет поиск Editor (SetLookHost) по фактическому Project.Look, под замком. Места
// определяются до fn (нетронутость — на прежнем формате), вписываются после settle: Rescale
// растянул бы их по осям.
func (e *Editor) formatLook(c *model.AlbumTpl, auto bool, fn func(p *model.Project) error) (model.View, error) {
	var fit []lookFit
	return e.editWith("", false, func(p *model.Project) error {
		if auto && slices.ContainsFunc(p.Spreads, func(sp *model.Spread) bool { return sp.LookStep > 0 }) {
			c = e.findLook(p.Look) // файл читается, только если есть что вписывать
		}
		fit = lookFits(p, c)
		return fn(p)
	}, nil, editOpts{after: func(p *model.Project) {
		for _, f := range fit {
			if _, sp := p.Find(f.id); sp != nil {
				grid.LookPlacesRefit(p, sp, f.step)
			}
		}
	}})
}

// clampLookPtr — макет, зажатый ClampLook; nil и негодный дают nil.
func clampLookPtr(look *model.AlbumTpl) *model.AlbumTpl {
	if look == nil {
		return nil
	}
	if ct, ok := model.ClampLook(*look); ok {
		return &ct
	}
	return nil
}

// LookHost — что Editor знает о библиотеке макетов (задаёт App): Find — макет по записи альбома
// (читает файл), Has — он есть в библиотеке этого ПК (только индекс: зовётся из каждого View).
// Editor зовёт их под своим замком; обе — не блокирующие правки.
type LookHost struct {
	Find LookFinder
	Has  func(model.LookRef) bool
	// Private — свой стикер с этим ключом не уходит в шаблон разворота и макет (код ссылки клиента).
	Private func(lib string) bool
}

// SetLookHost подключает библиотеку макетов: смена формата сама вписывает макет, View.Look.Found
// знает, есть ли макет на этом ПК. nil-поля — макетов нет.
func (e *Editor) SetLookHost(h LookHost) { e.lookHost.Store(&h) }

// findLook — макет записи r (зажатый); nil — записи, поиска или макета нет.
func (e *Editor) findLook(r *model.LookRef) *model.AlbumTpl {
	h := e.lookHost.Load()
	if r == nil || h == nil || h.Find == nil {
		return nil
	}
	return clampLookPtr(h.Find(*r))
}

// lookFound — макет записи r есть в библиотеке; без поиска — нет.
func (e *Editor) lookFound(r *model.LookRef) bool {
	h := e.lookHost.Load()
	return r != nil && h != nil && h.Has != nil && h.Has(*r)
}

// Look — копия записи макета альбома (Project.Look); nil — альбом не по макету. Чтение под замком.
func (e *Editor) Look() (r *model.LookRef) {
	e.read(func(p *model.Project) {
		if p.Look != nil {
			c := *p.Look
			r = &c
		}
	})
	return r
}

// lookFits — развороты макета c, чьи места ещё те, что поставил шаг на нынешнем формате p.
func lookFits(p *model.Project, c *model.AlbumTpl) (out []lookFit) {
	if c == nil || p.Look == nil {
		return nil
	}
	for _, s := range p.Spreads {
		if s.LookStep > 0 && s.LookStep <= len(c.Steps) && grid.LookPlacesAre(p, s, c.Steps[s.LookStep-1]) {
			out = append(out, lookFit{s.ID, c.Steps[s.LookStep-1]})
		}
	}
	return out
}

// IsSheet — проект — лист для печати (031): макеты альбома ему не нужны. Чтение под замком.
func (e *Editor) IsSheet() (sheet bool) {
	e.read(func(p *model.Project) { sheet = p.Settings.Sheet })
	return sheet
}
