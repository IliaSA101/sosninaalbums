package sharer

import (
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/share"
)

// PublishedTitle совпадает с названием в строке Published: с датой, без даты, пустое, чужая запись,
// управляющие знаки (интерфейс не разбирает строку регулярным выражением).
func TestLinkPublishedTitle(t *testing.T) {
	when := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		rec  prefs.ProjectRecord
		own  bool
		want string
	}{
		{"с датой", prefs.ProjectRecord{Title: "Иванов", PublishedAt: when}, true, "Иванов"},
		{"без даты", prefs.ProjectRecord{Title: "Иванов"}, true, "Иванов"},
		{"пустое название с датой", prefs.ProjectRecord{PublishedAt: when}, true, ""},
		{"пустое название без даты", prefs.ProjectRecord{}, true, ""},
		{"чужая запись с датой", prefs.ProjectRecord{Title: "Иванов", PublishedAt: when}, false, "Иванов"},
		{"чужая запись без даты", prefs.ProjectRecord{Title: "Иванов"}, false, "Иванов"},
		{"управляющие знаки", prefs.ProjectRecord{Title: "Ива\u202eнов\n", PublishedAt: when}, true, "Иванов"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l := linkOf(c.rec, share.Kind("moved"), c.own, share.Job{})
			if l.PublishedTitle != c.want {
				t.Fatalf("PublishedTitle = %q, ждали %q", l.PublishedTitle, c.want)
			}
			if c.want != "" && !strings.HasSuffix(l.Published, "как «"+c.want+"»") {
				t.Errorf("Published = %q не заканчивается названием %q", l.Published, c.want)
			}
			if c.want == "" && c.rec.PublishedAt.IsZero() && l.Published != "" {
				t.Errorf("Published без названия и даты: %q", l.Published)
			}
		})
	}
}
