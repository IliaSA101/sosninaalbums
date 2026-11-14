package app

// Виртуальный облачный диск в тихом пути (160 Т3): Google Drive, Яндекс.Диск, pCloud, Box на Windows — папка на нём
// всегда «облако», а зависшее чтение сведений о томе — «медленно», не ожидание окна.

import (
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func TestQuietVirtualCloud(t *testing.T) {
	r := newQuietRig(t)
	old := quietVirtual
	t.Cleanup(func() { quietVirtual = old })

	var seen string
	quietVirtual = func(p string) bool { seen = p; return true }
	c := r.ask(r.addQuiet(r.shoot))
	if ev := r.refuse(t, c, 1); ev.Why != whyCloud {
		t.Errorf("виртуальный диск: %+v", ev)
	}
	if seen != r.shoot {
		t.Errorf("признак спрошен про %q, ждали %q", seen, r.shoot)
	}

	// Обычный диск: тихо, как раньше.
	quietVirtual = func(string) bool { return false }
	if c := r.run(t, r.addQuiet(r.shoot)); c.res.Why != agents.WhyQuiet {
		t.Errorf("обычный диск: %+v", c.res)
	}
}

// TestQuietVirtualCloudHang: сведения о томе не читаются (мёртвый диск) — вопрос с причиной «медленно», вызов не ждёт.
func TestQuietVirtualCloudHang(t *testing.T) {
	r := newQuietRig(t)
	old := quietVirtual
	t.Cleanup(func() { quietVirtual = old })
	opts := permissiveOpts()
	opts.Disk = nil
	opts.Timeout = 30 * time.Millisecond
	r.a.askM().rules = func() *agentplaces.Rules { return agentplaces.New(opts) }

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	quietVirtual = func(string) bool { <-release; return false }
	start := time.Now()
	c := r.ask(r.addQuiet(r.shoot))
	if ev := r.refuse(t, c, 1); ev.Why != whyBig {
		t.Errorf("повисший диск: %+v", ev)
	}
	if d := time.Since(start); d > testkit.Limit(5*time.Second) {
		t.Errorf("ждали повисшее чтение %v", d)
	}
}
