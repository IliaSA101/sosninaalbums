package peercred

import (
	"fmt"
	"sync"
)

// verdict — решение «ОС не умеет назвать собеседника» (Windows без SIO_AF_UNIX_GETPEERPID). Его
// принимает проба на своей паре сокетов один раз за запуск, а не ответ на очередном подключении
// (0.32-148-11): иначе чужое подключение, получившее ту же ошибку запроса, открывало бы путь
// (программа принимает при ErrUnsupported). После пробы «умеет» любая ошибка запроса — отказ.
type verdict struct {
	once        sync.Once
	unsupported bool
}

// decide — результат пробы; probe зовётся не больше одного раза. Паника пробы — «умеет»
// (once считает вызов выполненным): закрыто по умолчанию.
func (v *verdict) decide(probe func() bool) bool {
	v.once.Do(func() { v.unsupported = probe() })
	return v.unsupported
}

// classify превращает ошибку запроса в итог: nil — как есть; не похожа на «не умею» — как есть;
// похожа (notSupported) — ErrUnsupported, только если проба на своей паре тоже не удалась,
// иначе обычная ошибка (отказ).
func (v *verdict) classify(err error, notSupported bool, probe func() bool) error {
	if err == nil || !notSupported {
		return err
	}
	if v.decide(probe) {
		return ErrUnsupported
	}
	return fmt.Errorf("запрос собеседника отклонён, хотя на своей паре работает: %w", err)
}

// Probe решает заранее (Start программы), умеет ли ОС назвать собеседника: ErrUnsupported — нет,
// nil — да. Повторные вызовы не пробуют заново. Linux и macOS — всегда nil.
func Probe() error {
	if osUnsupported() {
		return ErrUnsupported
	}
	return nil
}
