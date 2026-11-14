package libpacks

import (
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// installBypass — доноры встают мимо службы (чужая операция, прежний запуск): без Relink и событий.
func installBypass(t *testing.T, e *demoEnv, zips ...[]byte) {
	t.Helper()
	for _, z := range zips {
		sg := stageZip(t, e, z, decor.OriginNet)
		_, err := e.st.InstallPack(sg, nil)
		sg.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

// Пример поставлен, три донора встали мимо службы: PlaceDemo сам убирает пример (уход внутри
// PlaceDemo, а не только в установке), одно library:changed и один Relink, пропуск пуст.
func TestPlaceDemoRetiresInside(t *testing.T) {
	e := newDemoEnv(t, true, nil)
	e.place()
	installBypass(t, e, e.mainZip, e.framesZip, e.layoutsZip)
	e.mustStand(demoPackID, decor.OriginDemo)
	changed, relinks := len(e.ev.of("library:changed")), e.relinks.Load()
	e.svc.PlaceDemo(t.Context())
	e.mustNotStand(demoPackID)
	if got := len(e.ev.of("library:changed")) - changed; got != 1 {
		t.Errorf("library:changed после PlaceDemo: %d, ждали 1", got)
	}
	if got := e.relinks.Load() - relinks; got != 1 {
		t.Errorf("Relink: %d, ждали 1", got)
	}
	if len(e.skip()) != 0 || e.svc.Busy() {
		t.Errorf("пропуск %v, busy %v", e.skip(), e.svc.Busy())
	}
	e.svc.PlaceDemo(t.Context()) // повтор: ни ухода, ни события
	if len(e.ev.of("library:changed")) != changed+1 {
		t.Error("повторный PlaceDemo прислал событие")
	}
}

// Компактная: файла нет, пример стоит (скачан раньше), доноры встали позже — при запуске без
// согласия и сети пример уходит (0.30-142-48). Не покрыт — остаётся, замок не ждётся.
func TestPlaceDemoRetiresWithoutFile(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	e.addFile(e.demoZip, "demo")
	e.mustStand(demoPackID, decor.OriginFile)
	changed := len(e.ev.of("library:changed"))
	release, _ := e.svc.Hold() // не покрыт: PlaceDemo замок не ждёт
	done := make(chan struct{})
	go func() { e.svc.PlaceDemo(t.Context()); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("PlaceDemo с непокрытым примером ждёт замок")
	}
	release()
	e.mustStand(demoPackID, decor.OriginFile)
	installBypass(t, e, e.mainZip, e.framesZip) // двух из трёх мало
	e.svc.PlaceDemo(t.Context())
	e.mustStand(demoPackID, decor.OriginFile)
	installBypass(t, e, e.layoutsZip)
	e.svc.PlaceDemo(t.Context())
	e.mustNotStand(demoPackID)
	if got := len(e.ev.of("library:changed")) - changed; got != 1 {
		t.Errorf("library:changed: %d, ждали 1", got)
	}
	if e.srv.catHits.Load()+e.srv.assetHits.Load() != 0 || len(e.skip()) != 0 {
		t.Errorf("запросов %d, пропуск %v", e.srv.catHits.Load()+e.srv.assetHits.Load(), e.skip())
	}
}

// Паника в чтении файла примера не роняет программу и не оставляет службу занятой: PlaceDemo и
// «Вернуть» (ResetSkip, ErrInternal) отпускают замок, в log.txt — ПАНИКА.
func TestDemoPanicReleasesLock(t *testing.T) {
	e := newDemoEnv(t, true, func(h *Host) {
		inner := h.Demo
		h.Demo = func() Demo {
			d := inner()
			d.Read = func() ([]byte, bool) { panic("сбой чтения") }
			return d
		}
	})
	e.svc.PlaceDemo(t.Context()) // без Guard тест падает паникой
	if e.svc.Busy() || !strings.Contains(e.log.String(), "ПАНИКА в пример") {
		t.Fatalf("PlaceDemo: busy %v, log %q", e.svc.Busy(), e.log.String())
	}
	e.mustNotStand(demoPackID)
	prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.SetSkip(demoPackID, true) })
	if _, err := e.svc.ResetSkip(); err == nil || e.svc.Busy() {
		t.Fatalf("ResetSkip: err %v, busy %v", err, e.svc.Busy())
	}
	release, ok := e.svc.Hold() // замок свободен
	if !ok {
		t.Fatal("замок не снят после паники")
	}
	release()
}
