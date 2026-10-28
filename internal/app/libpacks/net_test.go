package libpacks

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// Какие ошибки рвут весь ход «Обновить», а какие — только набор (0.29-137-7, безопасность Д3).
// Две одинаковые неудачи подряд проверяют сквозные тесты App (packs_net_continue_test.go).
func TestFatalNet(t *testing.T) {
	live := context.Background()
	dead, cancel := context.WithCancel(live)
	cancel()
	cases := []struct {
		name  string
		ctx   context.Context
		err   error
		fatal bool
	}{
		{"отмена", live, fetch.ErrCancelled, true},
		{"контекст отменён", dead, errors.New("любая"), true},
		{"нет связи", live, fetch.ErrOffline, false}, // первая — по набору, вторая подряд рвёт (repeatKind)
		{"предел времени набора", live, fetch.ErrSlow, true},
		{"предел в обёртке", live, &fetch.SlowError{Title: "Т", Limit: time.Minute}, true},
		{"недопустимый id", live, decor.ErrBadPackID, false},
		{"архив не тот", live, packFail{"архив не совпадает со списком наборов"}, false},
		{"TLS", live, fetch.ErrTLS, true},
		{"адрес", live, fetch.ErrAddress, true},
		{"диск", live, fetch.ErrDisk, true},
		{"нет согласия", live, ErrNoConsent, true},
		{"в обёртке", live, fmt.Errorf("набор: %w", fetch.ErrTLS), true},
		{"битый архив", live, fetch.Broken("Т"), false},
		{"ответ 500", live, &fetch.StatusError{Code: 500}, true},
		{"ответ 429", live, &fetch.StatusError{Code: 429}, true},
		{"ответ 403", live, &fetch.StatusError{Code: 403}, true},
		{"ответ 407", live, &fetch.StatusError{Code: 407}, true},
		{"404", live, &fetch.StatusError{Code: 404}, false},
		{"404 как ErrNotFound", live, fetch.ErrNotFound, false},
		{"тайм-аут", live, fetch.ErrTimeout, false},
		{"библиотека заполнена", live, decor.ErrLibFull, false},
		{"нечего ставить", live, decor.ErrPackEmpty, false},
	}
	for _, c := range cases {
		if got := fatalNet(c.ctx, c.err); got != c.fatal {
			t.Errorf("%s: fatalNet = %v, ждали %v", c.name, got, c.fatal)
		}
	}
}

// Причина в «не скачались»: у битого архива — короткий текст без повтора названия.
func TestFailWhy(t *testing.T) {
	if got := failWhy(fetch.Broken("Айвори")); got != "скачался с ошибкой, повторите" {
		t.Errorf("битый: %q", got)
	}
	if got := failWhy(fetch.ErrTimeout); got != fetch.ErrTimeout.Error() {
		t.Errorf("тайм-аут: %q", got)
	}
	if got := failWhy(&fetch.StatusError{Code: 500}); got == "" {
		t.Error("500: пусто")
	}
}

// Вид неудачи, вторая подряд которой рвёт ход: связь (обрыв и застой — одна цепочка) и битый
// архив; остальное считается по одной.
func TestRepeatKind(t *testing.T) {
	conn := repeatKind(fetch.ErrOffline)
	if conn == nil || repeatKind(fetch.ErrTimeout) != conn || repeatKind(fmt.Errorf("x: %w", fetch.ErrOffline)) != conn {
		t.Errorf("обрыв и застой — один вид: %v %v", conn, repeatKind(fetch.ErrTimeout))
	}
	if b := repeatKind(fetch.Broken("Т")); b == nil || b == conn {
		t.Errorf("битый — свой вид: %v", b)
	}
	for _, err := range []error{fetch.ErrNotFound, decor.ErrLibFull, decor.ErrBadPackID, packFail{"x"}, fetch.ErrSlow} {
		if k := repeatKind(err); k != nil {
			t.Errorf("%v: вид %v, ждали nil", err, k)
		}
	}
}
