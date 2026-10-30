package agents

import (
	"strings"
	"testing"
)

// TestDeniedText: errDenied называет, где разрешить, и не несёт путей и данных альбома; WhyQuiet —
// не один из прежних итогов вопроса (иначе finishAsk принял бы тихий успех за другой).
func TestDeniedText(t *testing.T) {
	if got := errDenied.Error(); !strings.Contains(got, "Файл ▸ ИИ-агенты (MCP)… ▸ Разрешения") || strings.ContainsAny(got, `/\`) {
		t.Errorf("errDenied: %q", got)
	}
	for _, other := range []string{WhyAnswered, WhyDeclined, WhyTimeout, WhyGone, WhyClosed, WhyOff, WhyTooWide, WhyOurs, WhyDraftLocked} {
		if other == WhyQuiet {
			t.Errorf("WhyQuiet совпал с %q", other)
		}
	}
	if out, orig := toAgent(errDenied); out != errDenied || orig != nil {
		t.Errorf("toAgent(errDenied) = %v, %v: текст агенту должен дойти как есть", out, orig)
	}
}
