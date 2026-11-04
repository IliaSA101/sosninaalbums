//go:build !linux && !darwin && !windows

package peercred

import (
	"errors"
	"time"
)

// sameFD: на прочих ОС программа не собирается; ErrUnsupported не годится — сервер пропустил бы.
func sameFD(uintptr, time.Time) (bool, error) {
	return false, errors.New("ОС не поддерживается")
}
