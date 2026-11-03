package agents

import (
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// journalSvc — служба без сокета: журналу нужны только часы и OnCall.
func journalSvc(now *time.Time, onCall func(Call, int)) *Service {
	s := &Service{o: Options{OnCall: onCall}}
	s.jr.now = func() time.Time { return *now }
	return s
}

func rec(s *Service, title string, ok bool) {
	var res *mcp.CallToolResult
	if !ok {
		res = &mcp.CallToolResult{IsError: true}
	} else {
		res = &mcp.CallToolResult{}
	}
	s.record(toolSpec{title: title}, nil, res, 1500*time.Microsecond)
}

// Порядок (новые первыми), предел 50, счётчик «за сегодня» и его сброс со сменой дня,
// ClearCalls оставляет счётчик, запись переживает панику обратного вызова.
func TestJournal(t *testing.T) {
	now := time.Date(2026, 10, 29, 23, 59, 0, 0, time.Local)
	var got []int
	s := journalSvc(&now, func(c Call, today int) { got = append(got, today) })

	// Пусто: не nil, ноль.
	if calls, today := s.Calls(); calls == nil || len(calls) != 0 || today != 0 {
		t.Fatalf("пустой журнал: %v %d", calls, today)
	}

	// Порядок и поля: новые первыми, OK по IsError, миллисекунды целые.
	rec(s, "первый", true)
	now = now.Add(10 * time.Second)
	rec(s, "второй", false)
	calls, today := s.Calls()
	if today != 2 || len(calls) != 2 || calls[0].Tool != "второй" || calls[1].Tool != "первый" {
		t.Fatalf("порядок: %+v today %d", calls, today)
	}
	if calls[0].OK || !calls[1].OK || calls[0].Ms != 1 || !calls[0].At.Equal(now) {
		t.Errorf("поля записи: %+v", calls[0])
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("OnCall получил today %v, ждали [1 2]", got)
	}

	// Копия: правка ответа журнал не меняет.
	calls[0].Tool = "испорчено"
	if c2, _ := s.Calls(); c2[0].Tool != "второй" {
		t.Error("Calls отдал внутренний срез")
	}

	// Предел 50: остаются самые новые; счётчик считает все.
	for i := range 60 {
		rec(s, "вызов-"+string(rune('A'+i%26)), true)
		if n := len(s.jr.calls); n > journalMax {
			t.Fatalf("после вызова %d в журнале %d записей, предел %d", i, n, journalMax)
		}
	}
	calls, today = s.Calls()
	if len(calls) != journalMax || today != 62 {
		t.Fatalf("предел: %d записей (ждали %d), today %d (ждали 62)", len(calls), journalMax, today)
	}
	if calls[0].Tool != "вызов-"+string(rune('A'+59%26)) {
		t.Errorf("первой должна быть самая новая запись: %q", calls[0].Tool)
	}

	// ClearCalls: список пуст, счётчик дня остаётся; запись после очистки идёт дальше.
	s.ClearCalls()
	if calls, today = s.Calls(); len(calls) != 0 || today != 62 {
		t.Fatalf("после ClearCalls: %d записей, today %d (ждали 0 и 62)", len(calls), today)
	}
	s.ClearCalls() // холостая очистка пустого журнала
	rec(s, "после очистки", true)
	if calls, today = s.Calls(); len(calls) != 1 || today != 63 {
		t.Fatalf("после очистки и записи: %d, today %d", len(calls), today)
	}

	// Смена дня: Calls видит новый день без записи (окно открыто в полночь), запись — тоже.
	now = now.Add(2 * time.Minute) // 00:01 следующего дня
	if calls, today = s.Calls(); today != 0 || len(calls) != 1 {
		t.Fatalf("новый день: today %d (ждали 0), записей %d (журнал остаётся)", today, len(calls))
	}
	rec(s, "утром", true)
	if _, today = s.Calls(); today != 1 {
		t.Errorf("первый вызов нового дня: today %d", today)
	}
	if got[len(got)-1] != 1 {
		t.Errorf("OnCall в новом дне получил today %d", got[len(got)-1])
	}
	now = now.Add(-24 * time.Hour) // часы назад (перевод стрелок): день снова иной, счётчик с нуля, без паники
	if _, today = s.Calls(); today != 0 {
		t.Errorf("часы назад: today %d", today)
	}
}

// Запись не роняет вызов: паника в OnCall (окно, события) поймана, запись в журнале осталась, в
// логе — место и тип, без значения.
func TestJournalPanic(t *testing.T) {
	buf := captureLog(t)
	now := time.Now()
	s := journalSvc(&now, func(Call, int) { panic("секрет /home/user/Свадьба.salbum") })
	rec(s, "падает", true) // паники наружу нет
	if calls, today := s.Calls(); len(calls) != 1 || today != 1 {
		t.Fatalf("запись при панике: %d, today %d", len(calls), today)
	}
	log := buf.String()
	if !strings.Contains(log, "ПАНИКА в agents journal (string)") || strings.Contains(log, "Свадьба") || strings.Contains(log, "/home/user") {
		t.Errorf("лог паники: %q", log)
	}
	// Без обратного вызова (служба без App) — то же без паники.
	s2 := journalSvc(&now, nil)
	rec(s2, "тихо", false)
	if calls, _ := s2.Calls(); len(calls) != 1 || calls[0].OK {
		t.Errorf("без OnCall: %+v", calls)
	}
}
