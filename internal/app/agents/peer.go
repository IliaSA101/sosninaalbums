package agents

import (
	"errors"
	"log"
	"net"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/peercred"
)

// foreignEvery — не чаще одной строки в log.txt об отклонённых подключениях (0.32-148-2): чужой
// цикл подключений не должен раздувать журнал.
const foreignEvery = 10 * time.Second

// throttle — предел частоты строки журнала: первая сразу, остальные копятся и выходят одной
// строкой с числом пропущенных. Без часов и журнала внутри — проверяется подстановкой времени.
type throttle struct {
	last    time.Time
	skipped int
}

// hit — событие в момент now: писать ли строку и сколько событий пропущено с прошлой.
func (t *throttle) hit(now time.Time) (write bool, skipped int) {
	if !t.last.IsZero() && now.Sub(t.last) < foreignEvery {
		t.skipped++
		return false, 0
	}
	skipped, t.skipped, t.last = t.skipped, 0, now
	return true, skipped
}

// admitted — сверка собеседника до места в conns и до первого байта (148). Свой — true. Чужой и
// ошибка ОС — отказ, закрыто по умолчанию; единственное исключение — peercred.ErrUnsupported
// (Windows без запроса PID сокета): принять, строка один раз за запуск, защищает DACL папки
// (0.32-148-4). Паника проверки уходит в recover admit и закрывает подключение.
func (r *run) admitted(c net.Conn) bool {
	ok, err := r.o.Peer(c)
	switch {
	case err == nil && ok:
		return true
	case errors.Is(err, peercred.ErrUnsupported):
		r.unsupported.Do(func() {
			log.Printf("agents: проверка собеседника недоступна в этой Windows — доступ защищают права папки")
		})
		return true
	case err != nil:
		r.noteForeign("(проверка собеседника не удалась: " + bare(err).Error() + ")")
	default:
		r.noteForeign("другой учётной записи")
	}
	return false
}

// noteForeign — строка об отклонённом подключении без uid, SID, PID и путей (в тексте ОС их нет).
func (r *run) noteForeign(why string) {
	r.mu.Lock()
	write, skipped := r.foreign.hit(time.Now())
	r.mu.Unlock()
	switch {
	case !write:
	case skipped > 0:
		log.Printf("agents: отклонено подключение %s (ещё %d за %d с)", why, skipped, int(foreignEvery.Seconds()))
	default:
		log.Printf("agents: отклонено подключение %s", why)
	}
}
