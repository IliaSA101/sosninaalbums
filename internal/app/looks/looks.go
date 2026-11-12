// Package looks — макеты альбома (спека 131, 163): сведения, числа, вопрос до правки и сама
// постановка, «Сохранить как макет», обложка по макету, плитки /media/look (с фото на местах).
// Вынесен из internal/app: публичные методы App для JS — тонкие обёртки с guardEdit, поэтому набор
// методов и биндинги прежние. Службе нужны только чтение библиотеки украшений, редактор и файлы
// превью; общего замка App.mu у неё нет — ленту фото отдаёт Host.Feed (копией под замком App).
package looks

import (
	"errors"
	"fmt"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/looktpl"
	"github.com/IliaSA101/sosninaalbums/internal/app/edit"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Макеты альбома (спека 131, design §8): окно «Макеты альбома» — вид album библиотеки
// (DecorCatalog, RenameDecor, HideDecor — общие 048), сведения, числа, вопрос до правки и сама
// постановка. Макет берётся только по ключу плитки из индекса decor.Store; пути от интерфейса не идут.
// Плитки /media/look — tile.go.

// Host — что службе нужно от App; поля задаёт App, пустое — паника в New. Всё, что тесты App
// подменяют после NewApp (редактор, папка настроек, кэш превью, лента), читается в момент вызова.
type Host struct {
	Editor func() *album.Editor
	// Store — библиотека украшений; ошибка — её нет (startup не дошёл до неё).
	Store func() (*decor.Store, error)
	// Infos — сведения о фото ленты (а для фото альбома вне ленты — из проекта).
	Infos func(ids []string) []model.PhotoInfo
	// Changed — после правки: автосохранение и рассылка вида.
	Changed func(model.View) model.View
	// CfgDir — папка настроек: «недавние» плитки (prefs.RememberDecor).
	CfgDir func() string
	// Photo — фото ленты для места: ok — известно и читаемо, gone — файла нет или том не отвечает.
	Photo func(id string) (p library.Photo, ok, gone bool)
	// Cache — кэш миниатюр и превью; nil — кэша нет, места остаются серыми.
	Cache func() *library.Cache
	// Feed — копия ленты фото (под замком App), Cancelled — запрос плитки снят клиентом (счётчик
	// медиа-запросов App).
	Feed      func() []library.Photo
	Cancelled func()
	// FreePick — свободные фото ленты (тот же выбор, что у автосборки агента).
	FreePick func(photos []library.Photo, own map[string]model.MarkView, placed, backdrop []string) (ids []string, skipped int)
}

// Service — макеты альбома одного App; своей памяти нет.
type Service struct{ h Host }

// New — служба поверх h. Пустое поле Host — паника сразу и по имени, а не nil-вызов посреди правки.
func New(h Host) *Service {
	for name, set := range map[string]bool{
		"Editor": h.Editor != nil, "Store": h.Store != nil, "Infos": h.Infos != nil,
		"Changed": h.Changed != nil, "CfgDir": h.CfgDir != nil, "Photo": h.Photo != nil, "Cache": h.Cache != nil,
		"Feed": h.Feed != nil, "Cancelled": h.Cancelled != nil, "FreePick": h.FreePick != nil,
	} {
		if !set {
			panic("looks.New: не задано поле Host." + name)
		}
	}
	return &Service{h: h}
}

// ErrSheet — лист для печати (031): макеты альбома ему не нужны.
var ErrSheet = errors.New("Макеты альбома ставятся на альбом, не на лист для печати") //nolint:staticcheck // текст уходит пользователю как есть

// StepInfo — шаг серии в карточке макета.
type StepInfo struct {
	Name   string `json:"name"`
	Role   string `json:"role"`   // "" | "repeat" | "vignette"
	Places int    `json:"places"` // мест в шаге
	Pages  int    `json:"pages"`  // 1 — страница, 2 — разворот
}

// Card — карточка макета (App.LookInfo): шаги, есть ли обложка и в каких пределах число разворотов серии.
type Card struct {
	Steps []StepInfo `json:"steps"`
	Cover bool       `json:"cover"`
	Min   int        `json:"min"`
	Max   int        `json:"max"`
}

// Series — серия из N разворотов (App.LookCount): сколько их на деле, мест в них и сколько
// разворотов нужно под k фото («Подобрать под K»).
type Series struct {
	N      int `json:"n"`
	Places int `json:"places"`
	Fit    int `json:"fit"`
}

// Applied — итог Apply: вид и что вышло.
type Applied struct {
	View model.View     `json:"view"`
	Plan model.LookPlan `json:"plan"`
}

// CoverDone — итог CoverLook: вид, встало ли оформление макета альбома и его имя для тоста
// «Обложка в стиле «…» · Без оформления».
type CoverDone struct {
	View   model.View `json:"view"`
	Styled bool       `json:"styled"`
	Look   string     `json:"look"` // имя макета; пусто, если оформление не встало
}

// byID — макет плитки id и его запись для альбома. Нет в индексе, файл не читается или он
// новее программы — ошибка библиотеки.
func (l *Service) byID(id string) (*decor.Store, model.LookRef, looktpl.AlbumTpl, error) {
	s, err := l.h.Store()
	if err != nil {
		return nil, model.LookRef{}, looktpl.AlbumTpl{}, err
	}
	ref, ok := s.LookRefOf(id)
	if !ok {
		return s, ref, looktpl.AlbumTpl{}, decor.ErrNotFound
	}
	t, err := s.Look(id)
	return s, ref, t, err
}

// LookHost — библиотека макетов для Editor (SetLookHost): поиск по записи альбома и «есть на ПК».
// Хранилище читается при вызове: оно открывается после первого Editor.
func (l *Service) LookHost() album.LookHost {
	if l == nil {
		panic("looks: служба не собрана")
	}
	return album.LookHost{
		Find: func(ref model.LookRef) *looktpl.AlbumTpl {
			s := l.loaded()
			if s == nil {
				return nil
			}
			key, ok := s.LookKeyOf(ref)
			if !ok {
				return nil
			}
			t, err := s.Look(key)
			if err != nil {
				return nil
			}
			return &t
		},
		Has: func(ref model.LookRef) bool {
			s := l.loaded()
			return s != nil && s.HasLook(ref)
		},
		Private: func(lib string) bool {
			s := l.loaded()
			return s != nil && s.IsQR(lib)
		},
	}
}

// libs — свои из библиотеки для макетов ts, чьи файлы есть на этом ПК.
func (l *Service) libs(s *decor.Store, ts ...looktpl.AlbumTpl) map[string]model.PhotoInfo {
	var keys []string
	for _, t := range ts {
		for _, st := range t.Steps {
			keys = append(keys, tplKeys(st.Tpl)...)
		}
		if c := t.Cover; c != nil {
			if c.Bg != nil && c.Bg.Lib != "" {
				keys = append(keys, c.Bg.Lib)
			}
			for _, side := range []*looktpl.SpreadTpl{c.Back, c.Front} {
				if side != nil {
					keys = append(keys, tplKeys(*side)...)
				}
			}
		}
	}
	return l.libsFor(s, keys, false)
}

// LibsPrev — libs макета t и прежнего макета альбома (по ним сверяется «своё»).
func (l *Service) LibsPrev(s *decor.Store, t looktpl.AlbumTpl) map[string]model.PhotoInfo {
	ts := []looktpl.AlbumTpl{t}
	if cur := l.h.Editor().Look(); cur != nil {
		if prev := l.LookHost().Find(*cur); prev != nil {
			ts = append(ts, *prev)
		}
	}
	return l.libs(s, ts...)
}

// clampOpts — зажим параметров: разворотов 1…MaxSpreads, режим известен (пусто — выберет ядро).
func clampOpts(o model.LookOpts) (model.LookOpts, error) {
	switch o.Mode {
	case "", model.LookSeries, model.LookAdd, model.LookBg:
	default:
		return o, fmt.Errorf("неизвестный режим макета %q", o.Mode)
	}
	o.Spreads = max(1, min(o.Spreads, model.MaxSpreads))
	return o, nil
}

// loaded — библиотека украшений или nil, если её нет.
func (l *Service) loaded() *decor.Store {
	s, err := l.h.Store()
	if err != nil {
		return nil
	}
	return s
}

// Info — карточка макета id. Число разворотов серии — от минимума (начало, конец и один повтор)
// до MaxSpreads; без повтора серия фиксирована.
func (l *Service) Info(id string) (info Card, err error) {
	_, _, t, err := l.byID(id)
	if err != nil {
		return info, err
	}
	pre, rep, post := looktpl.LookBlockSizes(t)
	info = Card{Steps: make([]StepInfo, len(t.Steps)), Cover: t.Cover != nil, Min: pre + post + min(1, rep), Max: model.MaxSpreads}
	if rep == 0 {
		info.Min, info.Max = len(t.Steps), len(t.Steps)
	}
	for i, st := range t.Steps {
		pages := 2
		if st.Tpl.Pages == 1 {
			pages = 1
		}
		if st.Role == looktpl.RoleVignette {
			pages = 1 // лист 031 — одна страница
		}
		info.Steps[i] = StepInfo{Name: st.Name, Role: st.Role, Places: looktpl.LookPlaces(t, i), Pages: pages}
	}
	return info, nil
}

// Count — серия макета id из n разворотов (зажим 1…MaxSpreads): их число, мест и n под k фото.
func (l *Service) Count(id string, n, k int) (c Series, err error) {
	_, _, t, err := l.byID(id)
	if err != nil {
		return c, err
	}
	ids := looktpl.Series(t, max(1, min(n, model.MaxSpreads)))
	c.N = len(ids)
	for _, i := range ids {
		c.Places += looktpl.LookPlaces(t, i)
	}
	c.Fit = looktpl.LookFit(t, max(0, k))
	return c, nil
}

// Plan — вопрос до Apply: сухой прогон на копии (ничего не меняет). Лист 031 — ошибка.
func (l *Service) Plan(id string, o model.LookOpts) (plan model.LookPlan, err error) {
	ed := l.h.Editor()
	if ed.IsSheet() {
		return plan, ErrSheet
	}
	if o, err = clampOpts(o); err != nil {
		return plan, err
	}
	s, ref, t, err := l.byID(id)
	if err != nil {
		return plan, err
	}
	return ed.PlanLook(ref, t, l.LookHost().Find, o, l.LibsPrev(s, t))
}

// Apply — «Применить» окна макетов: одна правка, один шаг отмены; плитка — в недавние.
func (l *Service) Apply(id string, o model.LookOpts) (r Applied, err error) {
	ed := l.h.Editor()
	if ed.IsSheet() {
		return Applied{View: ed.View()}, ErrSheet
	}
	if o, err = clampOpts(o); err != nil {
		return Applied{View: ed.View()}, err
	}
	s, ref, t, err := l.byID(id)
	if err != nil {
		return Applied{View: ed.View()}, err
	}
	v, plan, err := ed.ApplyLook(ref, t, l.LookHost().Find, o, l.LibsPrev(s, t))
	if r.View, err = l.changedOrErr(v, err); err != nil {
		return r, err
	}
	r.Plan = plan
	prefs.RememberDecor(l.h.CfgDir(), decor.KindAlbum, id)
	return r, nil
}

// PlanStep — вопрос до ApplyStep: сколько фото уйдёт в ленту, что заменится. Обложка
// ('cover') и шаг 0 — оформление обложки (ApplyLookCover; макет без обложки — ErrLookStep).
func (l *Service) PlanStep(spreadID, id string, step int) (plan model.GridPlan, err error) {
	ed := l.h.Editor()
	if ed.IsSheet() {
		return plan, ErrSheet
	}
	s, ref, t, err := l.byID(id)
	if err != nil {
		return plan, err
	}
	return ed.PlanLookStep(spreadID, step, ref, t, l.libs(s, t))
}

// ApplyStep — «Поставить на разворот N»: шаг step макета id на лист spreadID, как плитка 067.
func (l *Service) ApplyStep(spreadID, id string, step int) (r StepApplied, err error) {
	ed := l.h.Editor()
	if ed.IsSheet() {
		return StepApplied{View: ed.View()}, ErrSheet
	}
	s, ref, t, err := l.byID(id)
	if err != nil {
		return StepApplied{View: ed.View()}, err
	}
	v, plan, missing, err := ed.ApplyLookStep(spreadID, step, ref, t, l.libs(s, t))
	if r.View, err = l.changedOrErr(v, err); err != nil {
		return r, err
	}
	r.Left, r.Missing = plan.Left, missing
	prefs.RememberDecor(l.h.CfgDir(), decor.KindAlbum, id)
	return r, nil
}

// NewSpread — «Разворот макета» в меню «+»: следующий повторяемый шаг макета альбома.
// Альбом не по макету — ErrNoLook; макета нет на этом ПК — отдельная ошибка с именем.
func (l *Service) NewSpread() (v model.View, err error) {
	ed := l.h.Editor()
	if ed.IsSheet() {
		return ed.View(), ErrSheet
	}
	cur := ed.Look()
	if cur == nil {
		return ed.View(), album.ErrNoLook
	}
	s, err := l.h.Store()
	if err != nil {
		return ed.View(), err
	}
	key, ok := s.LookKeyOf(*cur)
	if !ok {
		return ed.View(), fmt.Errorf("Макета «%s» нет в библиотеке этого компьютера", cur.Name) //nolint:staticcheck // текст уходит пользователю как есть
	}
	t, err := s.Look(key)
	if err != nil {
		return ed.View(), err
	}
	return l.changedOrErr(ed.NewLookSpread(t, l.libs(s, t)))
}

// Save — «Сохранить альбом как макет…»: серия из разворотов o.Ids (повторяемые — позиции
// RepFrom…RepTo, одинаковые сливаются), обложка при o.Cover, места при o.Places, надписи
// заготовками при o.Drafts — в свои макеты темы o.Theme под именем o.Name. Альбом не меняется (чтение
// под замком); фото и фон из съёмки в макет не идут. Лист 031 — ErrSheet. Больше 24 разных
// шагов — ошибка для тоста окна.
func (l *Service) Save(o model.SaveLookOpts) (it decor.Item, err error) {
	t, err := l.saveTpl(o)
	if err != nil {
		return it, err
	}
	s, err := l.h.Store()
	if err != nil {
		return it, err
	}
	got, err := s.SaveLook(o.Theme, o.Name, t)
	if err != nil {
		return it, err
	}
	return got, nil
}

// saveTpl — макет из разворотов o.Ids до записи: общая часть SaveLook и PlanSaveLook (лист 031,
// пустой выбор, чужие id, больше MaxLookSteps шагов, пустой или слишком большой файл). Чтение под замком.
func (l *Service) saveTpl(o model.SaveLookOpts) (looktpl.AlbumTpl, error) {
	ed := l.h.Editor()
	if ed.IsSheet() {
		return looktpl.AlbumTpl{}, ErrSheet
	}
	t, err := ed.AlbumTplOf(o)
	if err != nil {
		return t, err
	}
	return decor.CheckLook(t)
}

// SaveCheck — сухой прогон Save для окна «Сохранить альбом как макет…».
type SaveCheck struct {
	Ok     bool   `json:"ok"`     // сохранить можно (имя и тема не проверяются: это поля окна)
	Steps  int    `json:"steps"`  // шагов в макете после слияния повторяемых; 0, если Ok = false
	Max    int    `json:"max"`    // предел шагов (MaxLookSteps)
	Reason string `json:"reason"` // почему нельзя — текст ошибки SaveLook как есть; пусто при Ok
}

// PlanSave — сухой прогон Save: тот же путь до записи (saveTpl), но без библиотеки и
// без правки альбома. Звать при смене выбора, границ «Повторяемые» и галочек окна. Ошибка Go — только
// сбой самого вызова; отказ сохранения лежит в Reason.
func (l *Service) PlanSave(o model.SaveLookOpts) (r SaveCheck, err error) {
	r.Max = model.MaxLookSteps
	t, terr := l.saveTpl(o)
	if terr != nil {
		r.Reason = terr.Error()
		return r, nil
	}
	r.Ok, r.Steps = true, len(t.Steps)
	return r, nil
}

// CoverLibs — запись макета альбома и файлы фонов его обложки; хранилище недоступно — оформление
// встанет без «своих» файлов. Общая для CreateCoverLook и set_cover_photo агента.
func (l *Service) CoverLibs(ed *album.Editor) (cur *model.LookRef, libs map[string]model.PhotoInfo) {
	cur = ed.Look()
	if cur != nil {
		if s, serr := l.h.Store(); serr == nil {
			if t := l.LookHost().Find(*cur); t != nil {
				libs = l.libs(s, *t)
			}
		}
	}
	return cur, libs
}

// CoverLook — «Создать обложку» (плитка, бросок фото): как CreateCover, но у альбома по макету
// оформление макета встаёт той же правкой и тем же шагом отмены (131 G5). Макета нет у альбома, нет
// на этом ПК или у него нет обложки — обычная обложка, Styled = false. «Без оформления» — Undo и
// CreateCover (она оформления не ставит).
func (l *Service) CoverLook(preset, photoID string) (r CoverDone, err error) {
	ed := l.h.Editor()
	var ph *model.PhotoInfo
	if photoID != "" {
		infos := l.h.Infos([]string{photoID})
		if len(infos) == 0 {
			return CoverDone{View: ed.View()}, edit.ErrNoPhoto
		}
		ph = &infos[0]
	}
	cur, libs := l.CoverLibs(ed)
	// cur — запись, по которой собраны libs: сменился макет между чтением и правкой — ErrLookChanged.
	v, rec, err := ed.CreateCoverLook(preset, ph, libs, cur)
	if r.View, err = l.changedOrErr(v, err); err != nil {
		return r, err
	}
	if rec != nil {
		r.Styled, r.Look = true, rec.Name
	}
	return r, nil
}

// changedOrErr рассылает вид, только если правка прошла.
func (l *Service) changedOrErr(v model.View, err error) (model.View, error) {
	if err != nil {
		return v, err
	}
	return l.h.Changed(v), nil
}

// StepApplied — итог ApplyStep: вид, сколько фото ушло в ленту и сколько ключей библиотеки макета
// нет на этом ПК (их слои пропущены).
type StepApplied struct {
	View    model.View
	Left    int
	Missing int
}
