// Package appkit — общие помощники App и его служб (internal/app/updater, assoc, exporter,
// edit): защита от паники и адрес репозитория. Только stdlib: службы не импортируют друг друга
// и app, а общее берут отсюда. Класть сюда только то, что нужно нескольким из них.
package appkit

import (
	"errors"
	"log"
	"runtime/debug"
)

// RepoURL — адрес репозитория программы: окно «О программе» и ссылки службы обновления.
const RepoURL = "https://github.com/IliaSA101/sosninaalbums"

// ErrInternal — правка упала с паникой: проект не изменён, подробности в log.txt.
var ErrInternal = errors.New("внутренняя ошибка: правка не применена, подробности в журнале")

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
