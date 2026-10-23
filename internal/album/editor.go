package album

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/backdrop"
	"github.com/IliaSA101/sosninaalbums/internal/album/cover"
	"github.com/IliaSA101/sosninaalbums/internal/album/design"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/album/view"
	"github.com/IliaSA101/sosninaalbums/internal/album/vigsheet"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Editor — открытый проект: все правки идут через него, каждая
// правка запоминает прошлое состояние для отмены.
type Editor struct {
	mu sync.Mutex
	// warmMu — прогревы раскладок (Warm) по одному: ход каждого монотонный, а второй берёт
	// готовое из кэша. Порядок: warmMu → mu.
	warmMu    sync.Mutex
	p         *model.Project
	size      int // длина JSON e.p; известна после первой правки (до неё undo пуст)
	path      string
	dirty     bool
	undo      []Snap
	undoBytes int
	undoAlbum int // шагов undo со снимком альбома (p != nil); остальные — только отметки
	redo      []Snap
	rev       int // растёт при каждой реальной правке, отмене и возврате
	// projRev — правки самого проекта: растёт, когда сменились его байты (правка, Undo/Redo шага
	// со снимком); отметки его не трогают. По нему живёт пересборка (auto, 078 §8).
	projRev int
	// auto — последняя автосборка для «Пересобрать» (ops_auto.go).
	auto autoChain
	// lastMerge — ключ слияния последней правки (layout:<разворот>, text:<сессия>):
	// подряд идущие правки с тем же ключом — один шаг отмены.
	lastMerge string
	// origin, from, backup: файл старой версии перед первой записью в тот же
	// путь копируется в «<имя> (для 0.22).salbum» (salbum.BackupName); backupMade — имя
	// сделанной копии до TakeBackup (тост App).
	origin     string
	from       int
	backup     bool
	backupMade string
	// marks — свои отметки (ID фото → отметка), не nil; marksRev — из общего счётчика процесса.
	marks    map[string]model.Mark
	marksRev int
	// bins — подборки фото (083) в порядке создания; членство — в marks[].Bins. Рядом с
	// отметками, а не в проекте: правка подборки не трогает снимок альбома и projRev.
	// Срез не правится на месте: его делят шаги отмены (BinsSwap).
	bins []model.Bin
	// guides — что прочитано о файлах-разметках обложки и разворотов для вида (guideFor, 066):
	// не данные проекта; у каждой разметки свой слот.
	guides [markupKinds]guideState
	// id — номер Editor в процессе (087-3), не меняется (и при Restore): по нему App узнаёт
	// поздний вид прежнего альбома.
	id uint64
	// closed — альбом сменился (Close, 087-5): правки отклоняются, запись и чтения работают.
	closed bool
}

// editorSeq — счётчик номеров Editor на процесс: у двух Editor номера разные, 0 не бывает.
var editorSeq atomic.Uint64

// ID — номер Editor; его же несёт каждый вид (View.Src).
func (e *Editor) ID() uint64 { return e.id }

// NewEditor открывает проект p (из salbum.Load или model.New). Editor держит свою копию, p
// остаётся у вызывающего как был; отметки из файла (p.loaded) дальше живут в Editor.
// Проект проверяется заново (own): собранный не через Load не протащит в альбом чужой путь.
// Проверяются только пути (salbum.Check) и NaN (Marshal); остальные инварианты (лимиты,
// зажимы, цвета) — только Load/Decode, единственный источник внешних проектов.
func NewEditor(p *model.Project, path string) (*Editor, error) {
	q, side, err := own(p)
	if err != nil {
		return nil, err
	}
	return &Editor{p: q, path: path, origin: path, from: q.From(),
		backup: path != "" && q.From() != 0 && q.From() < model.FormatVersion,
		marks:  side.Marks, bins: side.Bins, marksRev: nextMarksRev(), id: editorSeq.Add(1)}, nil
}

// own — своя копия внешнего проекта (NewEditor, Restore) и его отметки и подборки из файла.
// Пути — по правилам Load (salbum.Check): иначе UNC или относительный путь ушёл бы в rescan и
// экспорт. Копия идёт через JSON: from и loaded в него не попадают и переносятся явно; NaN/Inf —
// ошибка, а не паника Clone.
func own(p *model.Project) (*model.Project, model.Side, error) {
	if err := salbum.Check(p); err != nil {
		return nil, model.Side{}, fmt.Errorf("проект не прошёл проверку: %w", err)
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil, model.Side{}, fmt.Errorf("проект не прошёл проверку: %w", err)
	}
	q := model.Unmarshal(data)
	q.SetFrom(p.From())
	// Глубокая копия: указатели Rating/Label/Flag иначе общие с p; marks у Editor не nil.
	side := p.Loaded().Clone()
	if side.Marks == nil {
		side.Marks = map[string]model.Mark{}
	}
	return q, side, nil
}

// side — отметки и подборки для записи. Под e.mu: запись идёт под тем же замком, копия не нужна.
func (e *Editor) side() model.Side { return model.Side{Marks: e.marks, Bins: e.bins} }

// change выполняет правку с сохранением состояния для отмены.
// Правка, ничего не изменившая в проекте, в отмену не попадает.
func (e *Editor) change(fn func(p *model.Project)) model.View { return e.apply("", fn) }

// apply — тело change. merge непуст у правок, которые сливаются в один шаг
// отмены: если прошлая правка была с тем же ключом, новый снимок не делается.
func (e *Editor) apply(merge string, fn func(p *model.Project)) model.View {
	v, _ := e.try(merge, func(p *model.Project) error { fn(p); return nil })
	return v
}

// try — apply, где fn может отказать: проект откатывается к состоянию до fn,
// в отмену ничего не попадает, возвращается текущий View и ошибка. Лимиты
// проверяются внутри fn, под тем же замком, что и правка.
func (e *Editor) try(merge string, fn func(p *model.Project) error) (model.View, error) {
	return e.edit(merge, false, fn, nil)
}

// edit — тело try. whole — fn заменяет проект целиком (Restore): поправки относительно
// прежнего состояния (normalize, keepLayouts, rescale*) к нему не относятся — они исказили бы
// восстановленное (тексты пересчитались бы под прежний формат страницы).
// marks (может быть nil) — дельта своих отметок той же правки (перепривязка, Restore): зовётся
// после удачной правки проекта и идёт в тот же шаг отмены; слияние с прошлым шагом она рвёт.
func (e *Editor) edit(merge string, whole bool, fn func(p *model.Project) error, marks func() []MarkChange) (model.View, error) {
	return e.editWith(merge, whole, fn, marks, editOpts{})
}

// editOpts — необязательное у правки (пересборка, 078 §8). rel — от какого проекта считать
// поправки settle (nil — от проекта до fn): пересборка ставит альбом до прошлой сборки и
// нормализуется от него, а не от прошлого варианта. done — после удачной правки, сменившей
// проект, под замком; pushed — снимок, ушедший в отмену этой правкой (nil — слита с прошлой).
// bins — смена списка подборок той же правки (только Restore, 083 §4.4; ответ nil — список
// прежний): идёт в шаг, как дельта marks.
type editOpts struct {
	rel  *model.Project
	done func(pushed *model.Project)
	bins func() *BinsSwap
}

// editWith — тело edit с необязательным o.
func (e *Editor) editWith(merge string, whole bool, fn func(p *model.Project) error, marks func() []MarkChange, o editOpts) (model.View, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.shut("правка") {
		return e.view(), nil
	}
	t0 := time.Now()
	defer func() { slowAt(t0, slowEdit, "правка: %d разворотов", len(e.p.Spreads)) }()
	data := model.Marshal(e.p) // паника, а не пустой снимок: e.p остаётся как был
	e.size = len(data)
	before := model.Unmarshal(data)
	err := fn(e.p)
	if err == nil {
		err = settle(e.p, o.relOr(before), whole)
	}
	if err != nil {
		e.p = before
		return e.view(), err
	}
	after, err := json.Marshal(e.p)
	if err != nil {
		// В проект попало недопустимое число (NaN/Inf): правка откатывается.
		e.p = before
		return e.view(), fmt.Errorf("правка не применена: %w", err)
	}
	var delta []MarkChange
	if marks != nil {
		delta = marks()
	}
	var bins *BinsSwap
	if o.bins != nil {
		bins = o.bins()
	}
	same := bytes.Equal(data, after)
	if same && len(delta) == 0 && bins == nil {
		return e.view(), nil
	}
	if len(delta) > 0 || bins != nil {
		merge = ""
		e.setSide(delta, bins, false)
	}
	top := len(e.undo) - 1
	merged := merge != "" && merge == e.lastMerge && top >= 0
	var pushed *model.Project
	switch {
	case same:
		e.pushUndo(Snap{Marks: delta, Bins: bins}) // проект не изменился: снимок альбома не нужен
	case !merged:
		e.pushUndo(Snap{P: before, N: len(data), Marks: delta, Bins: bins})
		pushed = before
	case e.undo[top].P != nil && e.undo[top].N == len(after) && bytes.Equal(model.Marshal(e.undo[top].P), after):
		// Слитая цепочка вернула проект к началу (блок добавили и стёрли):
		// пустой шаг отмены не нужен.
		e.dropUndo(top)
		merge = ""
	}
	e.size = len(after)
	e.lastMerge = merge
	e.redo = nil
	e.dirty = true
	e.stepped(same, pushed, o)
	return e.view(), nil
}

// relOr — от чего считать поправки settle: o.rel, а без него — проект до fn.
func (o editOpts) relOr(before *model.Project) *model.Project {
	if o.rel != nil {
		return o.rel
	}
	return before
}

// stepped — счётчики после удачной правки: rev всегда, projRev и o.done — если сменился проект
// (same — нет, правка только отметок).
func (e *Editor) stepped(same bool, pushed *model.Project, o editOpts) {
	e.rev++
	if same {
		return
	}
	e.projRev++
	if o.done != nil {
		o.done(pushed)
	}
}

// settle — хвост конвейера правки после fn: p приводится к инвариантам относительно before.
// Общий для edit и сухого прогона (PreviewVignette): предпросмотр по построению равен итогу
// правки. Ошибка — правку не применять (SyncGrid без пустого места, сверх запаса знаков).
func settle(p, before *model.Project, whole bool) error {
	if err := p.SyncGrid(false); err != nil { // страховка инвариантов сетки
		return err
	}
	p.PruneRefs()
	if !whole {
		p.Normalize(before)
		p.KeepLayouts(before)
		// Смена формата страницы: тексты, места и фоны остаются на своих местах страницы.
		design.Rescale(p, before)
		grid.Rescale(p, before)
		backdrop.Rescale(p, before)
		// Обложка: зоны сменились (разворотов, Spec, страница) — содержимое едет по зонам (032 §3).
		cover.Reflow(p, before)
		// Виньетки (031): инварианты вида и ролей, затем пересборка нетронутых при смене
		// настроек или стиля подписей — после всех пересчётов формата, иначе места
		// масштабировались бы второй раз (critic §2).
		p.FixVignettes()
		vigsheet.Relay(p, before, vigPlan)
	}
	p.FitTexts()
	p.SyncFonts()
	p.FitStickers()
	p.SyncOrder()
	return p.CheckTotals(before)
}

// Save сохраняет проект; пустой path — в текущий файл.
func (e *Editor) Save(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if path == "" {
		path = e.path
	}
	// Первая запись поверх файла старой версии: сначала копия исходника.
	// «Сохранить как» в другой путь копию не делает — исходник и так цел.
	first := e.backup && paths.SamePath(path, e.origin)
	if first {
		bak, err := salbum.BackupOld(path, e.from)
		if err != nil {
			return fmt.Errorf("копия файла старой версии: %w", err)
		}
		e.keepBackup(bak)
	}
	if err := salbum.SaveWith(e.p, path, e.side()); err != nil {
		return err
	}
	if first {
		e.backup = false
	}
	e.path = path
	e.dirty = false
	return nil
}

// Close — альбом уходит (смена альбома, 087-5): ждёт идущую правку и очередь к замку перед собой,
// дальше правки не меняют альбом (вид прежний, без ошибки, строка в log.txt). Save, WriteDraft и
// чтения работают: App пишет альбом уже после Close, и поздняя правка не останется лишь в памяти.
// Повторный — холостой.
func (e *Editor) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closed = true
}

// shut — под e.mu: Editor закрыт, правку op не делать. Строка в журнале — для разбора жалоб
// «правка пропала» (087-8): тоста нет, пользователь уже смотрит другой альбом.
func (e *Editor) shut(op string) bool {
	if e.closed {
		log.Printf("правка прежнего альбома после смены отклонена (%s)", op)
	}
	return e.closed
}

func (e *Editor) Path() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.path
}

// read — чтение проекта под замком: операции областей получают p, а не e.mu.
func (e *Editor) read(fn func(p *model.Project)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	fn(e.p)
}

func (e *Editor) Folders() (dirs []string) {
	e.read(func(p *model.Project) { dirs = slices.Clone(p.Folders) })
	return dirs
}

// Snapshot — копия проекта и вид печати для экспорта в фоне: поля подставлены, пустые —
// пусто, заглушек нет (030; строки переложены по реальному тексту).
func (e *Editor) Snapshot() (*model.Project, model.View) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.p.Clone(), e.printView()
}

// WriteCopy пишет проект в файл, не делая его текущим (черновик). Пути —
// абсолютные: черновик лежит в папке данных, а не рядом с фото.
func (e *Editor) WriteCopy(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return salbum.Write(e.p, path, "", e.side())
}

// Ref — сохранённые в проекте сведения о фото; копия без общих указателей (PhotoRef.Copy).
func (e *Editor) Ref(id string) (r model.PhotoRef, ok bool) {
	e.read(func(p *model.Project) {
		r, ok = p.Photos[id]
		r = r.Copy()
	})
	return r, ok
}

// PhotoUses — где стоит фото id и строка «Использовано N раз» (084). Только чтение: ни шага
// отмены, ни rev. uses не nil.
func (e *Editor) PhotoUses(id string) (uses []model.PhotoUse, used string) {
	e.read(func(p *model.Project) { uses = view.Uses(p, id) })
	return uses, view.UsedText(len(uses))
}

func (e *Editor) View() model.View {
	e.mu.Lock()
	defer e.mu.Unlock()
	t0 := time.Now()
	v := e.view()
	slow(t0, "View: %d разворотов", len(v.Spreads))
	return v
}

// Warm заранее считает раскладки разворотов «Авто» с фото — ту же работу, что первый View
// (073): App зовёт его до своего замка, и окно открытия большого альбома видит ход, а не
// зависание. progress(done, total) — после каждого разворота, done растёт на 1 до total, зовётся
// по одному; total 0 — ни одного вызова. Это не правка: rev, dirty и отмена не меняются.
// Под e.mu — только снимок входа (пропорции и Params, 073-11), счёт — без замка: Folders, Ref
// для /media и правки не ждут секунды холодного прогрева. Правка во время счёта безопасна:
// снимок взят целиком под e.mu (проект уже после normalize прошлой правки), дальше прогрев
// проект не трогает и только кладёт в кэши чистые функции своего входа; View считает по
// текущему проекту под e.mu и берёт из кэша лишь совпавшие ключи. Худшее — лишняя работа.
func (e *Editor) Warm(progress func(done, total int)) {
	e.warmMu.Lock()
	defer e.warmMu.Unlock()
	var jobs []model.WarmJob
	var prm layout.Params
	e.read(func(p *model.Project) {
		var todo []*model.Spread
		for _, s := range p.Spreads {
			if len(s.Grid) == 0 && len(s.Photos) > 0 { // как layAll: остальное View не перебирает
				todo = append(todo, s)
			}
		}
		jobs, prm = p.WarmJobs(todo), p.Params()
	})
	t0 := time.Now()
	var done func(int)
	if progress != nil && len(jobs) > 0 {
		done = func(n int) { progress(n, len(jobs)) }
	}
	model.WarmAspects(jobs, prm, done)
	slow(t0, "прогрев раскладок: %d разворотов", len(jobs))
}

// Медленные операции в log.txt (спека 020).

// slowOp — порог «медленной» операции: дольше него в log.txt уходит строка с временем.
// slowEdit — то же для правки (075-6): цель 50 мс, порог ниже slowOp, чтобы log.txt медленных
// ноутбуков фотографов показал, сколько правка идёт у них. Переменные, а не const, только ради теста.
var (
	slowOp   = 300 * time.Millisecond
	slowEdit = 100 * time.Millisecond
)

// slow пишет в лог строку, если с t0 прошло не меньше slowOp. В строке только счётчики
// и время — ни путей, ни имён файлов (design 020 §4.3).
func slow(t0 time.Time, format string, args ...any) { slowAt(t0, slowOp, format, args...) }

// slowAt — slow со своим порогом limit.
func slowAt(t0 time.Time, limit time.Duration, format string, args ...any) {
	if d := time.Since(t0); d >= limit {
		log.Printf(format+" за %s", append(args, d.Round(time.Millisecond))...)
	}
}

// Восстановление из резервной копии (спека 019 E).

// LoadBase — Load, только относительные пути считаются от base, а не от папки файла: копия
// лежит в папке копий, а фото — рядом с самим альбомом. base == "" — только абсолютные пути
// (черновик; относительный путь берёт запасной abs).
func LoadBase(file, base string) (*model.Project, error) {
	data, err := salbum.ReadLimited(file)
	if err != nil {
		return nil, err
	}
	return salbum.Decode(data, base)
}

// Restore заменяет альбом проектом q (из LoadBase) одним шагом отмены; совпало — холостая,
// не прошли пределы правки — ошибка, альбом прежний.
// Заменяется всё, что пишет encode (Product, Folders, FolderAlt, Settings, Spreads, Photos):
// это поля Project с JSON-тегами, и свои отметки с подборками (q.loaded; у копии до 004 или 083
// их нет — после замены их нет и в альбоме, Ctrl+Z вернёт). from остаётся текущим — копия старой версии файла
// (Editor.backup) нужна для файла альбома, а не для копии. q проверяется заново, как в
// NewEditor (own): не прошёл — ошибка, альбом прежний.
func (e *Editor) Restore(q *model.Project) (model.View, error) {
	src, side, err := own(q)
	if err != nil {
		return e.View(), err
	}
	return e.editWith("", true, func(p *model.Project) error {
		from := p.From()
		*p = *src
		p.SetFrom(from)
		return nil
	}, func() []MarkChange { return e.replaceMarks(side.Marks) },
		editOpts{bins: func() *BinsSwap { return e.replaceBins(side.Bins) }})
}

// Черновик.

// WriteDraft — WriteCopy для черновика: если проект прочитан из файла старой версии (черновик
// от прежней программы), первая запись поверх него сначала копирует исходник в «черновик (для
// 0.22).salbum», как Save делает для обычного файла: откат на старую программу не теряет черновик.
// Копия уже есть или файл уже новой версии — копировать нечего (salbum.BackupOld).
func (e *Editor) WriteDraft(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.from != 0 && e.from < model.FormatVersion {
		bak, err := salbum.BackupOld(path, e.from)
		if err != nil {
			return fmt.Errorf("копия черновика старой версии: %w", err)
		}
		e.keepBackup(bak)
	}
	return salbum.Write(e.p, path, "", e.side())
}

// keepBackup запоминает имя сделанной копии старого формата для тоста; "" — копии не было,
// прежнее имя (ещё не забранное) остаётся. Под e.mu.
func (e *Editor) keepBackup(bak string) {
	if bak != "" {
		e.backupMade = bak
	}
}

// TakeBackup — имя копии старого формата, сделанной последней записью (Save, WriteDraft), и
// забывает его: App показывает тост «Сохранена копия для версии 0.22» один раз. "" — копии не было.
func (e *Editor) TakeBackup() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	bak := e.backupMade
	e.backupMade = ""
	return bak
}
