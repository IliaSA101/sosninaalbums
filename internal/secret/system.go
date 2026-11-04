package secret

import (
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

// service — служба в хранилище ключей; учётная запись — origin сервиса.
const service = "SosninaAlbums"

// osTimeout — предел обращения к ОС: Secret Service ждёт разблокировки связки без предела
// (разбор №9), а окно не должно висеть.
const osTimeout = 15 * time.Second

// coolDown — после тайм-аута ОС не спрашиваем минуту: иначе каждое открытие окна ждало бы 15 с.
const coolDown = time.Minute

var (
	// ErrUnavailable — токен из программы убран, но хранилище ОС недоступно (не ответило в срок
	// или отказало и на проверку): если токен сохранялся туда раньше, он мог остаться. Мягкая
	// ошибка: окно говорит «Из программы токен убран; системное хранилище недоступно».
	ErrUnavailable = errors.New("системное хранилище ключей недоступно: если токен сохранялся в нём раньше, он мог остаться")
	// ErrKept — хранилище ОС отказало в удалении, и токен в нём остался.
	ErrKept = errors.New("системное хранилище ключей не удалило токен")
)

// errNotFound — в хранилище ОС этого токена нет.
var errNotFound = errors.New("not found")

// errTimeout — хранилище ОС не ответило в срок (подробности ОС не нужны).
var errTimeout = errors.New("хранилище ключей системы не ответило")

// backend — хранилище ОС; подменяется в тестах (настоящую связку ключей они не трогают).
type backend interface {
	get(account string) (string, error)
	set(account, token string) error
	del(account string) error
}

// System — токены в хранилище ключей ОС с запасным путём в память. Один Store на процесс: app
// создаёт его при старте и передаёт службе через Host (память запасного пути и пауза после
// тайм-аута живут в нём, два Store разошлись бы). memOnly решает app (стенд — osx.StandOn()):
// пакет сам SA_STAND не читает, иначе в выпуске его поведение разошлось бы с osx. В тестах без
// SA_KEYRING_TEST=1 тоже память: тесты не пишут в связку ключей владельца и CI (разбор С9).
func System(memOnly bool) Store {
	if memOnly || testing.Testing() && os.Getenv("SA_KEYRING_TEST") != "1" {
		return NewMemory()
	}
	return newSystem(keyringBackend{}, osTimeout, time.Now)
}

type system struct {
	be      backend
	mem     *Memory // запасной путь: хранилище недоступно или не ответило
	timeout time.Duration
	now     func() time.Time
	logged  atomic.Bool // отказ ОС уже записан в журнал: дальше молча (не чаще раза за процесс)

	mu   sync.Mutex
	down time.Time // до этого момента ОС не спрашиваем (после тайм-аута)
}

func newSystem(be backend, timeout time.Duration, now func() time.Time) *system {
	return &system{be: be, mem: NewMemory(), timeout: timeout, now: now}
}

// Get: память (токен, не попавший в ОС) главнее — он новее; затем ОС. Токен из ОС проверяется как
// при записи: подложенное другой программой значение с пробелом или переводом строки не годится
// для заголовка — «токена нет».
func (s *system) Get(origin string) (string, Place) {
	o, err := Origin(origin)
	if err != nil {
		return "", InSession
	}
	mem := s.mem.get(o)
	if mem != "" {
		return mem, InSession
	}
	if s.isDown() {
		return "", InSession
	}
	t, err := s.call("", func() (string, error) { return s.be.get(o) })
	switch {
	case err == nil && checkToken(t) == nil:
		return t, InSystem
	case err == nil || errors.Is(err, errNotFound):
		return "", InSystem
	}
	return "", InSession
}

// Set: в ОС; не вышло (нет хранилища, отказ, тайм-аут) — в память.
func (s *system) Set(origin, token string) (Place, error) {
	o, err := check(origin, token)
	if err != nil {
		return InSession, err
	}
	if !s.isDown() {
		_, err = s.call(token, func() (string, error) { return "", s.be.set(o, token) })
		if err == nil {
			s.mem.put(o, "") // старый токен из памяти не должен заслонять новый
			return InSystem, nil
		}
	}
	s.mem.put(o, token)
	return InSession, nil
}

// Delete убирает токен из памяти и из ОС. ОС спрашиваем и в «паузе» после тайм-аута: токен надо
// убрать. Нет токена в ОС — nil. Отказ ОС при удалении проверяется чтением: токена там нет —
// nil (Linux без демона ключей, куда токен и не клали); токен на месте — ErrKept; чтение тоже
// не вышло или тайм-аут — ErrUnavailable. Из памяти токен уходит при любом исходе.
func (s *system) Delete(origin string) error {
	o, err := Origin(origin)
	if err != nil {
		return err
	}
	memTok := s.mem.get(o)
	s.mem.put(o, "")
	_, err = s.call(memTok, func() (string, error) { return "", s.be.del(o) })
	switch {
	case err == nil, errors.Is(err, errNotFound):
		return nil
	case errors.Is(err, errTimeout):
		return ErrUnavailable
	}
	t, gerr := s.call(memTok, func() (string, error) { return s.be.get(o) })
	switch {
	case errors.Is(gerr, errNotFound):
		return nil
	case gerr != nil:
		return ErrUnavailable
	case t != "":
		return ErrKept
	}
	return nil
}

func (s *system) isDown() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.now().Before(s.down)
}

type result struct {
	val string
	err error
}

// call — обращение к ОС с пределом времени. Зависший вызов остаётся в своей горутине и
// отпустит её, когда ОС ответит; результат уже никому не нужен (канал с буфером). Токен
// token, если он известен вызывающему, вырезается из текста ошибки ОС до журнала и возврата.
// Отказ пишется в журнал один раз за процесс.
func (s *system) call(token string, fn func() (string, error)) (string, error) {
	ch := make(chan result, 1)
	go func() {
		v, err := fn()
		ch <- result{v, err}
	}()
	timer := time.NewTimer(s.timeout)
	defer timer.Stop()
	select {
	case r := <-ch:
		if r.err == nil || errors.Is(r.err, errNotFound) {
			return r.val, r.err
		}
		err := scrub(r.err, token)
		if s.logged.CompareAndSwap(false, true) {
			log.Printf("secret: хранилище ключей ОС отказало (дальше без повторов): %v", err)
		}
		return r.val, err
	case <-timer.C:
		s.mu.Lock()
		s.down = s.now().Add(coolDown)
		s.mu.Unlock()
		if s.logged.CompareAndSwap(false, true) {
			log.Printf("secret: хранилище ключей ОС не ответило за %s, токен — в памяти (дальше без повторов)", s.timeout)
		}
		return "", errTimeout
	}
}

// scrub — ошибка ОС без токена: ни в журнал, ни наверх он не уходит. Исходная ошибка не
// оборачивается (её текст остался бы доступен через Unwrap).
func scrub(err error, token string) error {
	msg := err.Error()
	if token != "" {
		msg = strings.ReplaceAll(msg, token, "***")
	}
	msg = strings.Map(func(r rune) rune { // в одну строку журнала
		if r < ' ' {
			return ' '
		}
		return r
	}, msg)
	if r := []rune(msg); len(r) > 200 {
		msg = string(r[:200]) + "…"
	}
	return errors.New(msg)
}

// keyringBackend — хранилище go-keyring: на Windows диспетчер учётных данных, на macOS связка ключей, на Linux служба секретов по D-Bus.
type keyringBackend struct{}

func (keyringBackend) get(account string) (string, error) {
	t, err := keyring.Get(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", errNotFound
	}
	return t, err
}

func (keyringBackend) set(account, token string) error {
	return keyring.Set(service, account, token)
}

func (keyringBackend) del(account string) error {
	err := keyring.Delete(service, account)
	if errors.Is(err, keyring.ErrNotFound) {
		return errNotFound
	}
	return err
}
