package testkit

import (
	"fmt"
	"strings"
	"testing"
)

// spyT — testing.TB, который запоминает ошибки вместо провала: проверяем сам страж.
type spyT struct {
	testing.TB
	errs []string
}

func (s *spyT) Helper()                   {}
func (s *spyT) Errorf(f string, a ...any) { s.errs = append(s.errs, fmt.Sprintf(f, a...)) }
func (s *spyT) Fatalf(f string, a ...any) {
	s.errs = append(s.errs, "FATAL "+fmt.Sprintf(f, a...))
	panic("spyT: Fatalf")
}

type probeHost struct {
	Alpha func()
	Beta  func(int) string
}

type badHost struct {
	Alpha func()
	Name  string
}

func probeNew(h probeHost) {
	if h.Alpha == nil {
		panic("probe.New: не задано поле Host.Alpha")
	}
	if h.Beta == nil {
		panic("probe.New: не задано поле Host.Beta")
	}
}

func fullProbe() probeHost { return probeHost{Alpha: func() {}, Beta: func(int) string { return "" }} }

func TestHostCompleteTemplate(t *testing.T) {
	HostComplete(t, fullProbe()) // полный Host — тишина

	s := &spyT{}
	HostComplete(s, probeHost{Alpha: func() {}})
	if len(s.errs) != 1 || !strings.Contains(s.errs[0], "Host.Beta") {
		t.Errorf("пустое поле Beta не названо: %q", s.errs)
	}
	s = &spyT{}
	HostComplete(s, badHost{Alpha: func() {}, Name: "x"})
	if len(s.errs) != 1 || !strings.Contains(s.errs[0], "Host.Name") {
		t.Errorf("не-функция Name не названа: %q", s.errs)
	}
}

func TestNewPanicsTemplate(t *testing.T) {
	NewPanics(t, fullProbe(), probeNew, "probe.New")

	// New, которая молчит про Beta и не называет Alpha, ловится.
	s := &spyT{}
	NewPanics(s, fullProbe(), func(h probeHost) {
		if h.Alpha == nil {
			panic("пусто")
		}
	}, "")
	if len(s.errs) != 2 {
		t.Errorf("ждали две ошибки (Alpha безымянная, Beta не паникует): %q", s.errs)
	}
}
