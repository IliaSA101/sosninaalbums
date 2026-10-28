package album

// Обёртки автосборки (027, 041, 065, 078): подсказка окна, предпросмотр, сборка и пересборка,
// отбор похожих кадров. Вход снимается под замком, план и пулы считаются вне его (autoPrep).
// Похожие кадры ищет ядро (album/similar), а spreads получает готовый список (решение 078-15).

import (
	"cmp"
	"errors"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/similar"
	"github.com/IliaSA101/sosninaalbums/internal/album/spreads"
)

// ErrAgainStale — пересобрать нельзя: после сборки альбом менялся (правка, Ctrl+Z, Ctrl+Y) или
// сборки в этом альбоме не было.
var ErrAgainStale = errors.New("после сборки альбом менялся — пересобрать нельзя")

// againGap — крючок теста: зовётся между снимком входа пересборки и замком её правки (nil — нет).
var againGap func()

// autoChain — последняя автосборка (078 §8): «Пересобрать» заменяет её вариант новым.
type autoChain struct {
	key  string         // ключ слияния шага отмены цепочки «Собрать → Пересобрать…»
	base *model.Project // альбом до первой сборки — тот же снимок, что в шаге отмены; nil — сборки нет
	rev  int            // projRev после последней сборки или пересборки
	ids  []string       // ID разворотов последнего варианта по порядку: пересборка берёт их же
	// album — цепочка «Весь альбом заново» (107): пересобрать можно только тем же режимом.
	album bool
}

// againAlive — пересборка жива: после неё проект не менялся (звёзды и метки не в счёт). Под e.mu.
func (e *Editor) againAlive() bool { return e.auto.base != nil && e.projRev == e.auto.rev }

// autoFrom — проект, от которого считает сборка: текущий p или (o.Again) альбом до прошлой
// сборки; пересборка не жива — ErrAgainStale. Под e.mu.
func (e *Editor) autoFrom(p *model.Project, o model.AutoOpts) (*model.Project, error) {
	if !o.Again {
		return p, nil
	}
	if !e.againAlive() || o.Album != e.auto.album {
		return nil, ErrAgainStale
	}
	return e.auto.base, nil
}

// autoJob — вход сборки из b без похожих кадров (o.Similar) и сами отброшенные по порядку ленты.
// Оценки и флажок «беру» — действующие (свои отметки поверх XMP), поэтому под e.mu.
func (e *Editor) autoJob(b *model.Project, photos []model.PhotoInfo, o model.AutoOpts) (spreads.PlanJob, []model.SimilarDrop) {
	rate := e.rating(photos)
	var job spreads.PlanJob
	if o.Album {
		job = spreads.AlbumOf(b, photos, rate) // пул — фото голых разворотов, якоря — стенами (107)
	} else {
		job = spreads.PlanOf(b, photos, rate)
	}
	if o.Similar <= 0 {
		return job, nil
	}
	infos := job.Infos()
	placed := map[string]bool{}
	for _, s := range b.Sheets() {
		for _, id := range s.Photos {
			placed[id] = true
		}
	}
	m := similar.Marks{Rate: job.Rates(), Asp: job.Asp(), Pick: make([]bool, len(infos)), InAlbum: make([]bool, len(infos))}
	for i, ph := range infos {
		m.Pick[i] = model.Effective(e.marks[ph.ID], ph.Xmp).Flag == 1
		m.InAlbum[i] = placed[ph.ID]
	}
	keep := make(map[string]bool, len(o.Keep))
	for _, id := range o.Keep {
		keep[id] = true
	}
	ds := similar.Drops(infos, o.Similar, keep, m)
	out := make([]model.SimilarDrop, len(ds))
	drop := make(map[string]bool, len(ds))
	for i, d := range ds {
		out[i] = model.SimilarDrop{ID: infos[d.I].ID, Like: infos[d.Like].ID, Dist: d.Dist}
		drop[out[i].ID] = true
	}
	return job.Without(drop), out
}

// AutoPlan — подсказка окна автосборки: как фото лягут ровно на o.Spreads разворотов (от o.MinPer
// до o.MaxPer на разворот или «Авто», серии — o.Series; всё зажимается; o.Variety не влияет —
// группы те же). Похожие кадры (o.Similar) в план не идут — их число в AutoPlan.Similar; ч/б
// разворотов — AutoPlan.Gray; o.Again — план от альбома до прошлой сборки (не жива —
// ErrAgainStale). Только чтение: вход снимается под замком, матрица
// оценок и DP считаются вне его (на тысячах фото — секунды при первом вызове).
// Пустой хвост альбома (развороты без фото, названия, цвета, сетки) — свободное место:
// верх ползунка считает его, AutoPlan.Reuse — сколько из него займёт сборка в конец.
// o.Album (107) — план «Весь альбом заново»: пул — фото голых разворотов по порядку альбома (и
// свободные из photos), якоря на местах; AutoPlan.Spreads — сколько новых разворотов, состав —
// AutoPlan.Album; at и пустой хвост не действуют.
// Договор: photos без повторов (DedupePhotos).
func (e *Editor) AutoPlan(photos []model.PhotoInfo, o model.AutoOpts) (model.AutoPlan, error) {
	o = o.Clamp()
	in, err := e.autoInput(model.DedupePhotos(photos), o)
	if err != nil {
		return model.AutoPlan{}, err
	}
	return in.job.Plan(o)
}

// AutoSimilar — кадры, которых автосборка с теми же аргументами не возьмёт как похожие (078),
// по порядку ленты; o.Similar = 0 — пусто. Только чтение.
// Договор: photos без повторов (DedupePhotos).
func (e *Editor) AutoSimilar(photos []model.PhotoInfo, o model.AutoOpts) ([]model.SimilarDrop, error) {
	in, err := e.autoInput(model.DedupePhotos(photos), o.Clamp())
	if err != nil || in.drops == nil {
		return []model.SimilarDrop{}, err
	}
	return in.drops, nil
}

// AutoBuild раскладывает фото ровно на o.Spreads новых разворотов (как AutoPlan с теми же
// аргументами), начиная с позиции at. Фото из сеток и с фонов автосборка не трогает, похожие
// кадры (o.Similar) не берёт — они остаются, где были.
// Сборка в конец (at < 0 или за непустыми) встаёт на место пустого хвоста, а не после него.
// Раскладки — по o.Variety с учётом соседей (041), звёзды — из оценок photos и своих отметок.
// Каждая сборка начинает цепочку «Пересобрать»; o.Again — пересборка (autoAgain).
// o.Album (107) — «Весь альбом заново» (autoAlbum): at не действует, шаг отмены один.
// План и пулы раскладок считаются вне замка (autoPrep): под замком — только выбор и запись.
// Договор: photos без повторов (DedupePhotos).
func (e *Editor) AutoBuild(photos []model.PhotoInfo, at int, o model.AutoOpts) (model.View, error) {
	photos, o = model.DedupePhotos(photos), o.Clamp()
	t0, made := time.Now(), 0
	in, err := e.autoPrep(photos, o)
	if err != nil {
		return e.View(), err
	}
	defer func() {
		slow(t0, "автосборка: %d фото → %d разворотов", in.pre.Photos(), made)
	}()
	if o.Again {
		return e.autoAgain(in, photos, at, &made)
	}
	if o.Album {
		return e.autoAlbum(in, photos, &made)
	}
	key := "auto:" + model.NewID() // уникален: с прошлыми правками не сливается
	var ids []string
	return e.editWith(key, false, func(p *model.Project) (err error) {
		ids, err = spreads.Rebuild(p, in.pre, at, e.rating(photos), nil)
		made = len(ids)
		return err
	}, nil, editOpts{done: func(pushed *model.Project) {
		if pushed != nil {
			e.auto = autoChain{key: key, base: pushed, rev: e.projRev, ids: ids}
		}
	}})
}

// autoAlbum — «Весь альбом заново» (107): голые развороты заменяются новыми с прежними ID, якоря
// и обложка не меняются; цепочка и шаг отмены — как у сборки. Пул пуст — холостая правка.
// «До правки» для settle — копия без ID голых (spreads.WithoutBare), иначе KeepLayouts сбросил бы
// раскладки новых разворотов с прежними ID.
func (e *Editor) autoAlbum(in autoIn, photos []model.PhotoInfo, made *int) (model.View, error) {
	key := "auto:" + model.NewID()
	var ids []string
	rel := new(model.Project)
	return e.editWith(key, false, func(p *model.Project) (err error) {
		*rel = *spreads.WithoutBare(p)
		ids, err = spreads.RebuildAlbum(p, in.pre, e.rating(photos), spreads.BareIDs(p))
		*made = len(ids)
		return err
	}, nil, editOpts{rel: rel, done: func(pushed *model.Project) {
		if pushed != nil {
			e.auto = autoChain{key: key, base: pushed, rev: e.projRev, ids: ids, album: true}
		}
	}})
}

// autoAgain — пересборка (078 §8): альбом до прошлой сборки + сборка с новыми настройками,
// развороты — с ID прошлого варианта; нормализация — от альбома до сборки. После сборки
// шагов не было — тот же шаг отмены (слияние), были отметки — новый шаг. Тот же вариант —
// холостая правка. Живость проверяется и под замком правки: за время счёта могли править.
func (e *Editor) autoAgain(in autoIn, photos []model.PhotoInfo, at int, made *int) (model.View, error) {
	if againGap != nil {
		againGap()
	}
	var ids []string
	return e.editWith(in.key, false, func(p *model.Project) (err error) {
		if !e.againAlive() || e.auto.base != in.base {
			return ErrAgainStale
		}
		from := p.From()
		*p = *in.base.Clone()
		p.SetFrom(from)
		if in.rel != nil { // режим альбома (107)
			ids, err = spreads.RebuildAlbum(p, in.pre, e.rating(photos), e.auto.ids)
		} else {
			ids, err = spreads.Rebuild(p, in.pre, at, e.rating(photos), e.auto.ids)
		}
		*made = len(ids)
		return err
	}, nil, editOpts{rel: cmp.Or(in.rel, in.base), done: func(*model.Project) { e.auto.rev, e.auto.ids = e.projRev, ids }})
}

// autoIn — вход сборки, снятый под замком: job — без похожих, drops — они сами; base и key —
// у пересборки (альбом до прошлой сборки и ключ её цепочки).
type autoIn struct {
	job   spreads.PlanJob
	drops []model.SimilarDrop
	pre   spreads.Prep
	base  *model.Project
	key   string
	rel   *model.Project // у пересборки альбома: base без ID голых разворотов (для settle, см. autoAlbum)
}

// autoInput — снимок входа под замком (без плана): проект — текущий или база пересборки.
func (e *Editor) autoInput(photos []model.PhotoInfo, o model.AutoOpts) (in autoIn, err error) {
	e.read(func(p *model.Project) {
		b, ferr := e.autoFrom(p, o)
		if err = ferr; err != nil {
			return
		}
		in.job, in.drops = e.autoJob(b, photos, o)
		if o.Again {
			in.base, in.key = b, e.auto.key
			if o.Album {
				in.rel = spreads.WithoutBare(b)
			}
		}
	})
	return in, err
}

// autoPrep — autoInput и план с прогревом пулов вне замка (design 041 §7): холодный разворот
// 16 фото — десятки мс, сотня разворотов держала бы замок секунды.
func (e *Editor) autoPrep(photos []model.PhotoInfo, o model.AutoOpts) (autoIn, error) {
	in, err := e.autoInput(photos, o)
	if err == nil {
		in.pre = spreads.Prepare(in.job, photos, o)
	}
	return in, err
}

// AutoPreview — ряд блока «Раскладки» окна автосборки (065): что соберёт AutoBuild с теми же
// аргументами (похожие, ч/б, вариант, пересборка — 078) и варианты групп плана. Только чтение:
// сборка — на копии проекта вне замка (под замком — снимок входа, копия и оценки), rev, undo и
// события не меняются.
// Договор: photos без повторов (DedupePhotos).
func (e *Editor) AutoPreview(photos []model.PhotoInfo, at int, o model.AutoOpts) (model.AutoPreview, error) {
	photos, o = model.DedupePhotos(photos), o.Clamp()
	in, err := e.autoPrep(photos, o)
	if err != nil {
		return model.AutoPreview{Samples: []model.SampleRow{}, Spreads: []model.PreviewSpread{}}, err
	}
	var c *model.Project
	rates := make(map[string]int, len(photos))
	e.read(func(p *model.Project) {
		if in.base != nil {
			p = in.base // неизменяемый снимок: копия та же, что поставит пересборка
		}
		c = p.Clone()
		rate := e.rating(photos) // оценки — как у AutoBuild, по тем же photos
		for _, ph := range photos {
			rates[ph.ID] = rate(ph.ID)
		}
	})
	return spreads.Preview(c, in.pre, at, func(id string) int { return rates[id] })
}
