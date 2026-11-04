package app

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/looks"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Позднее связывание Host (126-25): в программе cfgDir ставит startup, assocReg — Run, а тесты
// меняют emit, exists и время старта; все они после NewApp. Host службы читает их при каждом
// вызове — значение, снятое при сборке Host, увидело бы старое (пустой cfgDir, nil-реестр).
// Хосты берём сразу после NewApp и только потом меняем App.
func TestHostsLateBound(t *testing.T) {
	a := newApp(t)
	uh, ah, eh, ph, oh := a.updHost(), a.assocHost(), a.expHost(), a.packsHost(), a.printsHost()
	gh := a.agentsHost()

	// Подмены после сборки Host.
	dir := testkit.TempDir(t)
	reg := &fakeReg{}
	var events []string
	a.cfgDir = dir
	a.data = "late-data"
	a.assocReg = reg
	a.emit = func(name string, _ ...any) { events = append(events, name) }
	asked := map[string]int{}
	a.exists = func(p string) bool { asked[p]++; return p == "/late/yes" }
	defer setStart(7*time.Second, 9*time.Hour)()
	ed := mustEditor(t, model.New(), "")
	a.ed.Store(ed)

	if got := uh.CfgDir(); got != dir {
		t.Errorf("updater CfgDir = %q, ждали %q", got, dir)
	}
	if got := ah.CfgDir(); got != dir {
		t.Errorf("assoc CfgDir = %q, ждали %q", got, dir)
	}
	if got := ah.Data(); got != "late-data" {
		t.Errorf("assoc Data = %q", got)
	}
	if got := ah.Reg(); got != reg {
		t.Errorf("assoc Reg = %v, ждали подменённый реестр", got)
	}
	if d, e := uh.StartTimes(); d != 7*time.Second || e != 9*time.Hour {
		t.Errorf("StartTimes = %v %v, ждали 7s 9h", d, e)
	}
	if got := ph.CfgDir(); got != dir {
		t.Errorf("libpacks CfgDir = %q, ждали %q", got, dir)
	}
	if d, e, o := ph.Start(); d != 7*time.Second || e != 9*time.Hour || o != startOffer {
		t.Errorf("libpacks Start = %v %v %v, ждали 7s 9h %v", d, e, o, startOffer)
	}
	ph.Emit("p")
	oh.Emit("o")

	uh.Emit("u")
	eh.Emit("e")
	ah.Notice("n")
	gh.Changed()
	if want := []string{"p", "o", "u", "e", "notice", "agents:state"}; !slices.Equal(events, want) {
		t.Errorf("события = %v, ждали %v", events, want)
	}

	if !ah.Exists("/late/yes") || ah.Exists("/late/no") {
		t.Error("assoc Exists не зовёт подменённую a.exists")
	}
	if !eh.Exists("/late/yes") || eh.Exists("/late/no") {
		t.Error("exporter Exists не зовёт подменённую a.exists")
	}
	if asked["/late/yes"] != 2 || asked["/late/no"] != 2 {
		t.Errorf("a.exists вызвана %v", asked)
	}

	if sp, v := oh.Snapshot(), oh.View(); sp == nil || len(sp.Spreads) != len(ed.View().Spreads) || len(v.Spreads) != len(ed.View().Spreads) {
		t.Error("ownprint Snapshot/View не читают текущий редактор")
	}
	if eh.Editor() != ed {
		t.Error("exporter Editor не текущий редактор App")
	}
	// Служба агентов: папка настроек, путь сокета, лента и вид читаются при вызове.
	a.photos = []library.Photo{{ID: "late"}}
	if got := gh.CfgDir(); got != dir {
		t.Errorf("agents CfgDir = %q, ждали %q", got, dir)
	}
	if got := gh.Sock(); got != osx.MCPSock("late-data") {
		t.Errorf("agents Sock = %q, ждали путь для late-data", got)
	}
	if ph := gh.Photos(); len(ph) != 1 || ph[0].ID != "late" {
		t.Errorf("agents Photos = %v: лента не текущая", ph)
	}
	if v, id := gh.View(); id != ed.ID() || len(v.Spreads) != len(ed.View().Spreads) {
		t.Errorf("agents View: редактор %d, ждали %d", id, ed.ID())
	}
	// Новый редактор отличаем числом разворотов: Rev у пустых одинаков, раннее связывание не видно.
	p2 := model.New()
	p2.Spreads = []*model.Spread{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}}
	ed2 := mustEditor(t, p2, "")
	a.ed.Store(ed2)
	n2 := len(ed2.View().Spreads)
	if sp, v := oh.Snapshot(), oh.View(); n2 == len(ed.View().Spreads) || sp == nil || len(sp.Spreads) != n2 || len(v.Spreads) != n2 {
		t.Errorf("ownprint Snapshot/View закэшированы при сборке Host (разворотов %d, ждали %d)", len(v.Spreads), n2)
	}
	if eh.Editor() != ed2 {
		t.Error("exporter Editor закэширован при сборке Host")
	}
	if v, id := gh.View(); id != ed2.ID() || len(v.Spreads) != n2 {
		t.Errorf("agents View закэширован при сборке Host: редактор %d (ждали %d), разворотов %d (ждали %d)", id, ed2.ID(), len(v.Spreads), n2)
	}
}

// TestFaceHostLateBound — то же для подгона по лицам: редактор, родитель поиска и событие читаются
// при вызове, а не при сборке Host (отдельно от общего сторожа — предел сложности).
func TestFaceHostLateBound(t *testing.T) {
	a := newApp(t)
	fh := a.faceHost()

	var events []string
	a.emit = func(name string, _ ...any) { events = append(events, name) }
	oldCtx := a.stopCtx
	defer func() { a.stopCtx = oldCtx }()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	a.stopCtx = canceled
	ed := mustEditor(t, model.New(), "")
	a.ed.Store(ed)

	if fh.Editor() != ed {
		t.Error("faces Editor не текущий редактор App")
	}
	if fh.Ctx() != canceled || fh.Ctx().Err() == nil {
		t.Error("faces Ctx не читает подменённый a.stopCtx")
	}
	fh.Emit("f")
	if !slices.Equal(events, []string{"f"}) {
		t.Errorf("faces Emit: события %v, ждали [f] (подменённый a.emit)", events)
	}
	if fh.ExportBusy() {
		t.Error("faces ExportBusy: экспорта нет, а занятость есть")
	}
	// Новый редактор: Host не должен держать прежний.
	p2 := model.New()
	p2.Spreads = []*model.Spread{{ID: "s1"}}
	ed2 := mustEditor(t, p2, "")
	a.ed.Store(ed2)
	if fh.Editor() != ed2 {
		t.Error("faces Editor закэширован при сборке Host")
	}
}

// TestLooksHostLateBound — то же для макетов альбома: редактор, папка настроек, кэш превью и лента
// читаются при вызове, а не при сборке Host (NewApp собирает службу до startup, который их ставит).
func TestLooksHostLateBound(t *testing.T) {
	a := newApp(t)
	lh := a.looksHost()

	dir := testkit.TempDir(t)
	a.cfgDir = dir
	ed := mustEditor(t, model.New(), "")
	a.ed.Store(ed)
	a.photos = []library.Photo{{ID: "late"}}
	cache, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = cache

	if got := lh.CfgDir(); got != dir {
		t.Errorf("looks CfgDir = %q, ждали %q", got, dir)
	}
	if lh.Editor() != ed {
		t.Error("looks Editor не текущий редактор App")
	}
	if lh.Cache() != cache {
		t.Error("looks Cache закэширован при сборке Host (был nil)")
	}
	if f := lh.Feed(); len(f) != 1 || f[0].ID != "late" {
		t.Errorf("looks Feed = %v: лента не текущая", f)
	}
	before := a.mediaSt.cancelled.Load()
	lh.Cancelled()
	if a.mediaSt.cancelled.Load() != before+1 {
		t.Error("looks Cancelled не считает снятый запрос App")
	}
	p2 := model.New()
	p2.Spreads = []*model.Spread{{ID: "s1"}}
	ed2 := mustEditor(t, p2, "")
	a.ed.Store(ed2)
	if lh.Editor() != ed2 {
		t.Error("looks Editor закэширован при сборке Host")
	}
	// Пределы запроса плитки макета те же, что у /media/tpl: подпакет их дублирует.
	if looks.MinPx != tplMinPx || looks.MaxPx != tplMaxPx || looks.MinR != tplMinR || looks.MaxR != tplMaxR {
		t.Error("пределы /media/look разошлись с /media/tpl")
	}
}
