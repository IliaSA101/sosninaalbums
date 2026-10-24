package app

import (
	"errors"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Свои типографии в App (123 Б1, design §4, §7): реестр printprof собирается из встроенных и
// своих профилей хранилища decor; правка своих — Save/Delete/Import — пересобирает его и шлёт
// `print:changed`. Окно Б3 получает от Go поля, расчёт и ошибки (printprof_edit.go), файл
// .saprint читает и пишет printprof_file.go.

// printState — свои типографии: один писатель за раз, ключи файлов в хранилище и ждущий ответа
// вопрос «Заменить типографию?».
type printState struct {
	mu  sync.Mutex
	own map[string]ownPrint // id профиля → ключ в decor и сам профиль; только принятые реестром
	// stored — id всех разобранных файлов хранилища, и отвергнутых реестром: файл с таким id
	// импортируется копией под новым, иначе в хранилище стало бы два файла одного id.
	stored map[string]bool
	pend   *pendingPrint
}

// ownPrint — своя типография, принятая реестром.
type ownPrint struct {
	key string
	p   printprof.Profile
}

var (
	errPrintDup      = errors.New("Такое название уже есть")                                               //nolint:staticcheck // текст уходит пользователю тостом как есть
	errPrintDupLabel = errors.New("В списках это название выглядит так же, как у другой своей типографии") //nolint:staticcheck // текст уходит пользователю тостом как есть
	errPrintNotOwn   = errors.New("это не своя типография — её нельзя удалить или отправить файлом")
	errPrintRejected = errors.New("реестр не принял типографию — подробности в log.txt")
)

// reloadPrints собирает реестр заново из встроенных, наборов и своих (хранилище decor) и публикует его;
// строки пропущенных — в log.txt. Библиотечный слой — профили установленных наборов (printprof_lib.go).
// Вызов на старте после openDecor — без событий, окна ещё нет.
func (a *App) reloadPrints() {
	a.prints.mu.Lock()
	defer a.prints.mu.Unlock()
	a.reloadPrintsLocked()
}

func (a *App) reloadPrintsLocked() {
	var own, lib []printprof.Profile
	var keys []string // ключ файла own[i]
	a.prints.stored = map[string]bool{}
	s, err := a.decorStore()
	if err == nil {
		for _, f := range s.Prints() {
			p, err := printprof.Parse(f.Data)
			if err != nil {
				log.Printf("типография %q пропущена: %v", f.Key, paths.NoPath(err))
				continue
			}
			own, keys = append(own, p), append(keys, f.Key)
			a.prints.stored[p.ID] = true
		}
		lib = libraryPrints(s, own)
	}
	snap, skipped := printprof.Compose(lib, own)
	for _, line := range skipped {
		log.Print(line)
	}
	printprof.Install(snap)
	a.prints.own = map[string]ownPrint{}
	for i, p := range own {
		// Ключ — только у файла, который реестр принял (аудит 123 №6): отвергнутый первый файл
		// того же id невидим, и «Удалить» не должно стирать его вместо видимого.
		if n, ok := snap.OwnIndex(p.ID); ok && n == i {
			a.prints.own[p.ID] = ownPrint{key: keys[i], p: p}
		}
	}
}

// printsChanged — после смены своих: событие `print:changed` и вид (пределы и запреты считаются
// от нового реестра); вид уходит в цепочку «без правки», событие view — если он сменился.
func (a *App) printsChanged() model.View {
	a.emit("print:changed")
	return a.out(a.editor().View())
}

// PrintProfiles — типографии реестра по порядку: встроенные, библиотечные, свои. Для списка
// «Типография» и окна «Своя типография».
func (a *App) PrintProfiles() []printprof.Info {
	defer guard("PrintProfiles")
	return printprof.Current().Infos()
}

// DeletePrintProfile удаляет свою типографию. Альбомы со ссылкой на неё не меняются: пометка
// изделия обложки остаётся, карточка пишет «Типография не найдена» (123 сц. 11). Шага отмены нет —
// профили вне undo.
func (a *App) DeletePrintProfile(id string) (v model.View, err error) {
	defer guardErr("DeletePrintProfile", &err)
	s, err := a.decorStore()
	if err != nil {
		return v, err
	}
	a.prints.mu.Lock()
	o, ok := a.prints.own[id]
	if !ok {
		a.prints.mu.Unlock()
		return a.editor().View(), errPrintNotOwn
	}
	err = s.DeletePrint(o.key)
	a.reloadPrintsLocked()
	a.prints.mu.Unlock()
	if err != nil && !errors.Is(err, decor.ErrNotFound) {
		return a.editor().View(), err
	}
	return a.printsChanged(), nil
}

// applyPrint — «Сохранить с применением»: альбому ставится первая кнопка формата типографии id
// и изделие её обложки (Default, иначе первое) одним шагом отмены, как нажатие кнопки в окне
// «Настройки». Поля кнопки — как presetFields на фронте: размер, dpi, layflat, зона, обрез;
// поля, промежуток и цвет остаются. Нет изделий — меняется только формат. Тот же формат и то же
// изделие — холостая правка без шага. Сохранённая типография от исхода не зависит.
func (a *App) applyPrint(id string) (model.View, error) {
	pr, ok := printprof.Current().ByID(id)
	if !ok || len(pr.Formats) == 0 {
		return a.editor().View(), errPrintRejected
	}
	f := pr.Formats[0]
	p, _ := a.editor().SnapshotSide()
	s := p.Settings
	s.PageW, s.PageH = f.Page[0], f.Page[1]
	if pr.Page.DPI != 0 {
		s.DPI = pr.Page.DPI
	}
	s.Layflat = s.Layflat || pr.Page.Layflat
	if v := f.SafeIn(pr); v != 0 {
		s.Safe = v
	}
	if v := f.BleedIn(pr); v != 0 {
		s.Bleed = v
	}
	preset := pr.Default
	if preset == "" && len(pr.Products) > 0 {
		preset = pr.Products[0].ID
	}
	if preset == "" {
		return a.changed(a.editor().SetSettings(s)), nil
	}
	return a.changedOrErr(a.editor().ApplyPrint(s, preset))
}

// printTaken — id занят: профилем реестра, своей, ждущей перезагрузки, или файлом хранилища,
// который реестр отверг.
func (a *App) printTaken(id string) bool {
	if _, ok := printprof.Current().ByID(id); ok {
		return true
	}
	_, ok := a.prints.own[id]
	return ok || a.prints.stored[id]
}

// nameClash — чем название своей типографии спорит с реестром (кроме except): errPrintDup —
// двойник названия любого профиля (printprof.SameName: регистр, пробелы, невидимые знаки и
// латинские двойники не в счёт); errPrintDupLabel — подпись в списках («QA тест 1» и «QA тест 1
// (своя)» обе показываются «QA тест 1 (своя)») совпадает с подписью другой своей. У встроенных и
// библиотечных пометки нет, им подпись не проверяется. nil — название свободно.
func nameClash(snap *printprof.Snapshot, name, except string) error {
	ps := snap.Profiles()
	if slices.ContainsFunc(ps, func(p printprof.Profile) bool { return p.ID != except && printprof.SameName(p.Name, name) }) {
		return errPrintDup
	}
	if slices.ContainsFunc(ps, func(p printprof.Profile) bool {
		return p.ID != except && snap.Source(p.ID) == printprof.SourceOwn && printprof.SameOwnLabel(p.Name, name)
	}) {
		return errPrintDupLabel
	}
	return nil
}

// nameTaken — название занято (nameClash != nil).
func nameTaken(snap *printprof.Snapshot, name, except string) bool {
	return nameClash(snap, name, except) != nil
}

// pendingPrint — вопрос «Заменить типографию?»: файл разобран, ждёт ответа packTTL.
type pendingPrint struct {
	token string
	key   string // ключ своей в decor, чью запись заменит файл
	p     printprof.Profile
	until time.Time
}
