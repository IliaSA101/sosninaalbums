// Package ownprint — свои типографии (123 Б1, Б3, Б4; разрез 126 Ш5): реестр printprof собирается
// из встроенных, библиотечных и своих профилей хранилища decor; правка своих — Save, Delete,
// импорт файла .saprint — пересобирает его и шлёт `print:changed`. Окно «Своя типография» получает
// от Go поля, расчёт и ошибки (edit.go), файл .saprint читает и пишет file.go, типографии
// установленных наборов — lib.go, кириллические адреса — idn.go.
//
// Зовёт её internal/app (printprof.go — фасад с методами для интерфейса). Wails и App службе
// недоступны: диалоги файла, событие, проект открытого альбома и цепочка вида приходят через Host
// (поля-функции, замыкания над App — позднее связывание, подмены тестов действуют и после NewApp).
package ownprint

import (
	"errors"
	"log"
	"reflect"
	"slices"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// TTL — сколько ждёт ответа вопрос «Заменить типографию?» (как у набора библиотеки: 10 минут).
const TTL = 10 * time.Minute

// Host — что службе нужно от App. Все поля — функции и обязательны (New паникует с именем
// пустого). App отдаёт настоящие значения; тест пересобирает службу с подменой до первого вызова.
type Host struct {
	Store   func() (*decor.Store, error)   // библиотека; ошибка — startup до неё не дошёл
	Emit    func(name string, data ...any) // событие окну: print:changed (вид — только Changed и Out)
	Version func() string                  // версия программы: minApp типографий наборов
	// Snapshot — проект открытого альбома (только чтение): число разворотов, страница, обложка.
	Snapshot func() *model.Project
	View     func() model.View // текущий вид, без рассылки
	// Changed — вид после правки альбома (автосохранение, событие view); Out — вид «без правки»:
	// событие view, только если он сменился (пределы и запреты считаются от нового реестра).
	Changed func(model.View) model.View
	Out     func(model.View) model.View
	// SetSettings и ApplyPrint — правка настроек альбома одним шагом отмены (Editor); рассылает
	// вид сама служба через Changed.
	SetSettings func(model.Settings) model.View
	ApplyPrint  func(s model.Settings, preset string) (model.View, error)
	PickOpen    func() (string, error)            // «Файл типографии» *.saprint; "" — отмена; папка — у App
	PickSave    func(name string) (string, error) // «Поделиться типографией»: имя файла по умолчанию; "" — отмена
	RememberDir func(dir string)                  // последняя папка диалогов файла типографии
}

// Service — свои типографии: один писатель за раз, ключи файлов в хранилище и ждущий ответа
// вопрос «Заменить типографию?».
type Service struct {
	h  Host
	mu sync.Mutex
	// own — id профиля → ключ в decor и сам профиль; только принятые реестром.
	own map[string]ownPrint
	// stored — id всех разобранных файлов хранилища, и отвергнутых реестром: файл с таким id
	// импортируется копией под новым, иначе в хранилище стало бы два файла одного id.
	stored map[string]bool
	pend   *pendingPrint
}

// New собирает службу; пустое поле Host — паника с его именем (как edit.New и libpacks.New).
func New(h Host) *Service {
	v := reflect.ValueOf(h)
	for i := range v.NumField() {
		if v.Field(i).IsNil() {
			panic("ownprint.New: не задано поле Host." + v.Type().Field(i).Name)
		}
	}
	return &Service{h: h}
}

// ownPrint — своя типография, принятая реестром.
type ownPrint struct {
	key string
	p   printprof.Profile
}

var (
	ErrDup      = errors.New("Такое название уже есть")                                               //nolint:staticcheck // текст уходит пользователю тостом как есть
	ErrDupLabel = errors.New("В списках это название выглядит так же, как у другой своей типографии") //nolint:staticcheck // текст уходит пользователю тостом как есть
	ErrNotOwn   = errors.New("это не своя типография — её нельзя удалить или отправить файлом")
	ErrRejected = errors.New("реестр не принял типографию — подробности в log.txt")
)

// Reload собирает реестр заново из встроенных, наборов и своих (хранилище decor) и публикует его;
// строки пропущенных — в log.txt. Библиотечный слой — профили установленных наборов (lib.go).
// Вызов на старте после openDecor — без событий, окна ещё нет.
func (s *Service) Reload() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reloadLocked()
}

func (s *Service) reloadLocked() {
	var own, lib []printprof.Profile
	var keys []string // ключ файла own[i]
	s.stored = map[string]bool{}
	st, err := s.h.Store()
	if err == nil {
		for _, f := range st.Prints() {
			p, err := printprof.Parse(f.Data)
			if err != nil {
				log.Printf("типография %q пропущена: %v", f.Key, paths.NoPath(err))
				continue
			}
			own, keys = append(own, p), append(keys, f.Key)
			s.stored[p.ID] = true
		}
		lib = libraryPrints(st, own, s.h.Version())
	}
	snap, skipped := printprof.Compose(lib, own)
	for _, line := range skipped {
		log.Print(line)
	}
	printprof.Install(snap)
	s.own = map[string]ownPrint{}
	for i, p := range own {
		// Ключ — только у файла, который реестр принял (аудит 123 №6): отвергнутый первый файл
		// того же id невидим, и «Удалить» не должно стирать его вместо видимого.
		if n, ok := snap.OwnIndex(p.ID); ok && n == i {
			s.own[p.ID] = ownPrint{key: keys[i], p: p}
		}
	}
}

// changed — после смены своих: событие `print:changed` и вид (пределы и запреты считаются от нового
// реестра); вид уходит в цепочку «без правки», событие view — если он сменился.
func (s *Service) changed() model.View {
	s.h.Emit("print:changed")
	return s.h.Out(s.h.View())
}

// Delete удаляет свою типографию. Альбомы со ссылкой на неё не меняются: пометка изделия обложки
// остаётся, карточка пишет «Типография не найдена» (123 сц. 11). Шага отмены нет — профили вне undo.
func (s *Service) Delete(id string) (v model.View, err error) {
	st, err := s.h.Store()
	if err != nil {
		return v, err
	}
	s.mu.Lock()
	o, ok := s.own[id]
	if !ok {
		s.mu.Unlock()
		return s.h.View(), ErrNotOwn
	}
	err = st.DeletePrint(o.key)
	s.reloadLocked()
	s.mu.Unlock()
	if err != nil && !errors.Is(err, decor.ErrNotFound) {
		return s.h.View(), err
	}
	return s.changed(), nil
}

// Apply — «Сохранить с применением»: альбому ставится первая кнопка формата типографии id и
// изделие её обложки (Default, иначе первое) одним шагом отмены, как нажатие кнопки в окне
// «Настройки». Поля кнопки — как presetFields на фронте: размер, dpi, layflat, зона, обрез;
// поля, промежуток и цвет остаются. Нет изделий — меняется только формат. Тот же формат и то же
// изделие — холостая правка без шага. Сохранённая типография от исхода не зависит.
func (s *Service) Apply(id string) (model.View, error) {
	pr, ok := printprof.Current().ByID(id)
	if !ok || len(pr.Formats) == 0 {
		return s.h.View(), ErrRejected
	}
	st := PrintSettings(pr, pr.Formats[0], s.h.Snapshot().Settings)
	preset := pr.Default
	if preset == "" && len(pr.Products) > 0 {
		preset = pr.Products[0].ID
	}
	if preset == "" {
		return s.h.Changed(s.h.SetSettings(st)), nil
	}
	v, err := s.h.ApplyPrint(st, preset)
	if err != nil {
		return v, err
	}
	return s.h.Changed(v), nil
}

// PrintSettings — настройки альбома cur после кнопки формата f типографии pr (поля — как
// presetFields на фронте): размер, dpi, layflat, зона, обрез; поля, промежуток и цвет остаются.
// Чистая: общая для «Сохранить с применением» и агента (apply_print, 151), без шага отмены.
func PrintSettings(pr printprof.Profile, f printprof.PageFormat, cur model.Settings) model.Settings {
	st := cur
	st.PageW, st.PageH = f.Page[0], f.Page[1]
	if pr.Page.DPI != 0 {
		st.DPI = pr.Page.DPI
	}
	st.Layflat = st.Layflat || pr.Page.Layflat
	if v := f.SafeIn(pr); v != 0 {
		st.Safe = v
	}
	if v := f.BleedIn(pr); v != 0 {
		st.Bleed = v
	}
	return st
}

// taken — id занят: профилем реестра, своей, ждущей перезагрузки, или файлом хранилища,
// который реестр отверг. Под s.mu.
func (s *Service) taken(id string) bool {
	if _, ok := printprof.Current().ByID(id); ok {
		return true
	}
	_, ok := s.own[id]
	return ok || s.stored[id]
}

// nameClash — чем название своей типографии спорит с реестром (кроме except): ErrDup —
// двойник названия любого профиля (printprof.SameName: регистр, пробелы, невидимые знаки и
// латинские двойники не в счёт); ErrDupLabel — подпись в списках («QA тест 1» и «QA тест 1
// (своя)» обе показываются «QA тест 1 (своя)») совпадает с подписью другой своей. У встроенных и
// библиотечных пометки нет, им подпись не проверяется. nil — название свободно.
func nameClash(snap *printprof.Snapshot, name, except string) error {
	ps := snap.Profiles()
	if slices.ContainsFunc(ps, func(p printprof.Profile) bool { return p.ID != except && printprof.SameName(p.Name, name) }) {
		return ErrDup
	}
	if slices.ContainsFunc(ps, func(p printprof.Profile) bool {
		return p.ID != except && snap.Source(p.ID) == printprof.SourceOwn && printprof.SameOwnLabel(p.Name, name)
	}) {
		return ErrDupLabel
	}
	return nil
}

// NameTaken — название занято (nameClash != nil).
func NameTaken(snap *printprof.Snapshot, name, except string) bool {
	return nameClash(snap, name, except) != nil
}

// pendingPrint — вопрос «Заменить типографию?»: файл разобран, ждёт ответа TTL.
type pendingPrint struct {
	token string
	key   string // ключ своей в decor, чью запись заменит файл
	p     printprof.Profile
	until time.Time
}

// OwnKey — ключ файла принятой своей типографии id в хранилище ("" — такой нет). Для тестов:
// «Удалить» стирает именно видимый файл, и это видно только по ключу.
func (s *Service) OwnKey(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.own[id].key
}

// Taken — id занят (реестром, своей или файлом хранилища). Для тестов.
func (s *Service) Taken(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.taken(id)
}

// ExpirePending делает ждущий вопрос просроченным (как если бы прошло TTL). Для тестов.
func (s *Service) ExpirePending() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pend != nil {
		s.pend.until = time.Now().Add(-time.Second)
	}
}

// DropPending снимает ждущий ответа вопрос «Заменить?» (как следующая операция). Для тестов.
func (s *Service) DropPending() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pend = nil
}
