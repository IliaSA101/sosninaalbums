package testkit

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// HostComplete — шаблон стража «Host службы App задан целиком»: host — структура Host, которую
// отдаёт фасад (a.<служба>Host()). Каждое поле — функция и не nil; пустое поле означает nil-вызов
// посреди действия пользователя, а не-функция (string, bool) ломает reflect-проверку New.
// Тест службы: `func Test<Пакет>HostComplete(t) { testkit.HostComplete(t, newApp(t).<x>Host()) }`.
func HostComplete(t testing.TB, host any) {
	t.Helper()
	v := reflect.ValueOf(host)
	if v.Kind() != reflect.Struct || v.NumField() == 0 {
		t.Fatalf("HostComplete ждёт структуру Host с полями, получено %T", host)
	}
	for i := range v.NumField() {
		f := v.Type().Field(i)
		switch {
		case f.Type.Kind() != reflect.Func:
			t.Errorf("поле Host.%s не функция (%s): позднее связывание и проверка New держатся на функциях", f.Name, f.Type)
		case v.Field(i).IsNil():
			t.Errorf("поле Host.%s не задано", f.Name)
		}
	}
}

// NewPanics — шаблон стража «New с пустым полем Host паникует с именем поля»: full — Host со всеми
// заданными полями (годятся заглушки), newFn — конструктор службы, prefix — её имя в тексте
// паники («updater.New»; пусто — не проверяется). Для каждого поля берётся копия full без него.
// Тест службы: `func Test<Пакет>NewPanics(t) { testkit.NewPanics(t, fullHost(), func(h Host) { New(h) }, "<пакет>.New") }`.
func NewPanics[H any](t testing.TB, full H, newFn func(H), prefix string) {
	t.Helper()
	typ := reflect.TypeFor[H]()
	if typ.Kind() != reflect.Struct || typ.NumField() == 0 {
		t.Fatalf("NewPanics ждёт структуру Host с полями, получено %s", typ)
	}
	HostComplete(t, full) // заготовка сама должна быть полной, иначе паника не от того поля
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		h := full
		reflect.ValueOf(&h).Elem().Field(i).SetZero()
		msg, panicked := catchPanic(func() { newFn(h) })
		switch {
		case !panicked:
			t.Errorf("New без поля Host.%s не паникует", name)
		case !strings.Contains(msg, "Host."+name):
			t.Errorf("паника New без поля Host.%s не называет его: %q", name, msg)
		case prefix != "" && !strings.Contains(msg, prefix):
			t.Errorf("паника New без поля Host.%s не начинается с %q: %q", name, prefix, msg)
		}
	}
	if _, panicked := catchPanic(func() { newFn(full) }); panicked {
		t.Errorf("New с полным Host паникует")
	}
}

func catchPanic(fn func()) (msg string, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			msg, panicked = fmt.Sprint(r), true
		}
	}()
	fn()
	return "", false
}
