// Package edit — методы App, которые только зовут Editor и рассылают вид: правки раскладки,
// оформления, сетки, обложки и виньетки. Вынесены из internal/app, чтобы в его папке было
// меньше файлов; App встраивает *Methods, и Wails видит эти методы у App (набор методов и
// типы в биндингах прежние — сторож TestBoundMethodsMatchContract в internal/app).
//
// ВАЖНО: каждый ЭКСПОРТИРУЕМЫЙ метод Methods — метод App для JS. Помощники — только
// неэкспортируемые методы или функции пакета; типы в сигнатурах — из model и других пакетов,
// а не из этого (пространство имён в models.ts равно имени пакета типа).
package edit

import (
	"errors"
	"log"
	"reflect"
	"runtime/debug"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Host — что правкам нужно от App; поля задаёт App.NewApp (все обязательны).
type Host struct {
	Editor      func() *album.Editor
	Changed     func(model.View) model.View          // после правки: автосохранение, событие view, недостающие
	Notice      func(text string)                    // тост пользователю
	Infos       func(ids []string) []model.PhotoInfo // сведения о фото ленты
	InfosOrRefs func(ids []string) []model.PhotoInfo // из ленты, а для фото альбома — из проекта
	Templates   func() []model.Template              // копия списка своих шаблонов
}

// Methods — группа методов App; создаётся New, встраивается в App.
type Methods struct{ h Host }

// New — группа методов поверх h. Пустое поле Host — паника сразу и по имени: иначе nil-вызов
// случился бы посреди правки пользователя.
func New(h Host) *Methods {
	v := reflect.ValueOf(h)
	for i := range v.NumField() {
		if v.Field(i).IsNil() {
			panic("edit.New: не задано поле Host." + v.Type().Field(i).Name)
		}
	}
	return &Methods{h: h}
}

var (
	// ErrInternal — правка упала с паникой: проект не изменён, подробности в log.txt.
	ErrInternal = errors.New("внутренняя ошибка: правка не применена, подробности в журнале")
	// ErrNoPhoto — фото нет ни в ленте, ни в альбоме.
	ErrNoPhoto = errors.New("фото не найдено")
	// ErrBackdropScope — область фона не "spread" | "left" | "right" | "spine" (у узоров — без "spine").
	ErrBackdropScope = errors.New("неизвестная область фона")
)

func (m *Methods) editor() *album.Editor                      { return m.h.Editor() }
func (m *Methods) changed(v model.View) model.View            { return m.h.Changed(v) }
func (m *Methods) infos(ids []string) []model.PhotoInfo       { return m.h.Infos(ids) }
func (m *Methods) templateList() []model.Template             { return m.h.Templates() }
func (m *Methods) infosOrRefs(ids []string) []model.PhotoInfo { return m.h.InfosOrRefs(ids) }

// changedOrErr рассылает вид, только если правка прошла.
func (m *Methods) changedOrErr(v model.View, err error) (model.View, error) {
	if err != nil {
		return v, err
	}
	return m.changed(v), nil
}

// guardEdit — guard для методов правок: паника уходит в лог, интерфейс получает
// текущий вид (а не пустой — иначе он нарисовал бы пустой альбом) и ошибку:
// через err, если метод её возвращает, иначе тостом.
func (m *Methods) guardEdit(name string, v *model.View, err *error) {
	if r := recover(); r != nil {
		Fail(m.h, r, name, v, err)
	}
}

// Fail — отчёт о панике r в методе правки name (recover зовёт сам метод: вызванный отсюда
// он ничего бы не вернул). v получает текущий вид, err — ErrInternal, а без err — тост.
func Fail(h Host, r any, name string, v *model.View, err *error) {
	log.Printf("ПАНИКА в %s: %v\n%s", name, r, debug.Stack())
	func() {
		defer Guard(name + ": вид после паники")
		*v = h.Editor().View()
	}()
	if err != nil {
		*err = ErrInternal
		return
	}
	h.Notice(ErrInternal.Error())
}

// Guard — для `defer`: паника метода уходит в лог, а не роняет программу.
func Guard(name string) {
	if r := recover(); r != nil {
		log.Printf("ПАНИКА в %s: %v\n%s", name, r, debug.Stack())
	}
}

// GuardErr — Guard для методов с ошибкой: вызывающий получает ErrInternal, а не nil
// (иначе интерфейс счёл бы операцию успешной).
func GuardErr(name string, err *error) {
	if r := recover(); r != nil {
		log.Printf("ПАНИКА в %s: %v\n%s", name, r, debug.Stack())
		*err = ErrInternal
	}
}
