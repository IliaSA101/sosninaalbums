package app

// Источники и проход мест при корне архива (0.35/153): подъём при пополнении, внутренние места,
// проход по местам 0.34 с заметкой «до Понятно». Правила — agentplaces_lift_test.go.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// placeEvents — все события приложения по именам; для agents:placesLifted — и данные.
type placeEvents struct {
	mu     sync.Mutex
	count  map[string]int
	lifted []map[string][]string
}

func (v *placeEvents) n(name string) int { v.mu.Lock(); defer v.mu.Unlock(); return v.count[name] }

func watchEvents(a *App) *placeEvents {
	v := &placeEvents{count: map[string]int{}}
	a.emit = func(name string, data ...any) {
		v.mu.Lock()
		defer v.mu.Unlock()
		v.count[name]++
		if name == placesLiftedEvent && len(data) == 1 {
			if m, ok := data[0].(map[string][]string); ok {
				v.lifted = append(v.lifted, m)
			}
		}
	}
	return v
}

func mkDirs(t *testing.T, root string, rel ...string) string {
	t.Helper()
	d := filepath.Join(append([]string{root}, rel...)...)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

// Пополнение: «Добавить папку…» из …/2026/Петровы даёт место-архив; другой год и месяц в нём — без
// записи и события; бросок из двух лет — одно место, одна запись.
func TestPlaceLiftOnPush(t *testing.T) {
	a, states, _ := placeApp(t)
	captureLog(t)
	arch := realDir(t, "Архив")
	f1 := mkDirs(t, arch, "2026", "Петровы")
	if _, err := a.addChosenFolder(f1); err != nil {
		t.Fatal(err)
	}
	if got := placeDirs(t, a); len(got) != 1 || got[0] != arch || states() != 1 {
		t.Fatalf("после первой папки: %v (событий %d), ждали [%s]", got, states(), arch)
	}
	// Новый год и месяц-папка: место то же, ни записи, ни события.
	for _, rel := range [][]string{{"2027", "Иванова"}, {"2027", "05", "Сидоровы"}, {"2027", "Май", "Козловы"}} {
		if _, err := a.addChosenFolder(mkDirs(t, arch, rel...)); err != nil {
			t.Fatal(err)
		}
	}
	if got := placeDirs(t, a); len(got) != 1 || states() != 1 {
		t.Fatalf("новые годы: места %v, событий %d", got, states())
	}
	// Бросок: две папки из разных лет одного второго архива — одно место.
	arch2 := realDir(t, "Второй")
	a.dropPaths([]string{mkDirs(t, arch2, "2025", "А"), mkDirs(t, arch2, "2026-05-12 Б", "JPG"), mkDirs(t, arch2, "2024", "В")})
	waitScan(t, a)
	got := placeDirs(t, a)
	if len(got) < 2 || got[0] != arch2 || got[1] != arch {
		t.Fatalf("после броска: %v, ждали [%s %s]", got, arch2, arch)
	}
	for _, d := range got {
		if strings.HasSuffix(d, "2024") || strings.HasSuffix(d, "2025") {
			t.Errorf("год остался местом: %v", got)
		}
	}
}

// Явное место не поднимается: «Папка со съёмками…» из …/2026 остаётся …/2026 (0.35-153-3).
func TestPlaceChosenNotLifted(t *testing.T) {
	a, _, _ := placeApp(t)
	arch := realDir(t, "Архив")
	year := mkDirs(t, arch, "2026")
	old := openDirDialog
	t.Cleanup(func() { openDirDialog = old })
	openDirDialog = func(context.Context, wr.OpenDialogOptions) (string, error) { return year, nil }
	info, err := a.ChooseAgentPlace()
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Places) != 1 || info.Places[0].Dir != year || !info.Places[0].Chosen || info.Places[0].Narrow {
		t.Fatalf("явное место: %+v, ждали %s", info.Places, year)
	}
	// Папка из явного места: родитель поднимается над годом (новое «parent»), и явное место внутри него
	// уходит как внутреннее (design §2.3) — человек видит архив целиком в списке.
	if _, err := a.addChosenFolder(mkDirs(t, year, "05", "Петровы")); err != nil {
		t.Fatal(err)
	}
	if pl := prefs.LoadAgentPerms(a.cfgDir).Places; len(pl) != 1 || pl[0].Dir != arch || pl[0].How != prefs.PlaceParent {
		t.Fatalf("после папки внутри явного места: %+v", pl)
	}
}

// Места внутри других убираются: широкое поглощает узкие (несколько сразу), узкое внутри широкого не
// добавляется и не пишется, «chosen» внутри «parent» не добавляется, «parent» внутри нового «chosen» уходит.
func TestPlaceInnerDropped(t *testing.T) {
	a, states, _ := placeApp(t)
	captureLog(t)
	var picked string
	old := openDirDialog
	t.Cleanup(func() { openDirDialog = old })
	openDirDialog = func(context.Context, wr.OpenDialogOptions) (string, error) { return picked, nil }

	base := realDir(t, "Съёмки")
	for _, sub := range []string{"Свадьбы", "Крестины"} {
		if _, err := a.addChosenFolder(mkDirs(t, base, sub, "Клиент")); err != nil {
			t.Fatal(err)
		}
	}
	if got := placeDirs(t, a); len(got) != 2 {
		t.Fatalf("два узких места: %v", got)
	}
	if _, err := a.addChosenFolder(mkDirs(t, base, "Клиент")); err != nil {
		t.Fatal(err)
	}
	if got := placeDirs(t, a); len(got) != 1 || got[0] != base {
		t.Fatalf("широкое не поглотило два узких: %v", got)
	}
	// Узкое внутри широкого: ни места, ни записи.
	n := states()
	if _, err := a.addChosenFolder(mkDirs(t, base, "Свадьбы", "Новый")); err != nil {
		t.Fatal(err)
	}
	if got := placeDirs(t, a); len(got) != 1 || states() != n {
		t.Fatalf("внутреннее место: %v, событий +%d", got, states()-n)
	}
	// «chosen» внутри «parent» не добавляется (человек видит широкое место в списке).
	picked = mkDirs(t, base, "Свадьбы")
	if _, err := a.ChooseAgentPlace(); err != nil {
		t.Fatal(err)
	}
	if got := placeDirs(t, a); len(got) != 1 || got[0] != base {
		t.Fatalf("chosen внутри parent: %v", got)
	}
	// «chosen» над «parent»: родитель внутри него уходит.
	picked = filepath.Dir(base)
	if _, err := a.ChooseAgentPlace(); err != nil {
		t.Fatal(err)
	}
	pl := prefs.LoadAgentPerms(a.cfgDir).Places
	if len(pl) != 1 || pl[0].Dir != picked || pl[0].How != prefs.PlaceChosen {
		t.Fatalf("parent внутри chosen: %+v", pl)
	}
}

// legacyApp — App с местами 0.34 в config.json (как написала бы 0.34: два года одного архива, явное
// место, место с пропавшей папкой) и перехватом событий.
func legacyApp(t *testing.T) (a *App, ev *placeEvents, arch, chosen, gone string) {
	t.Helper()
	a, _, _ = placeApp(t)
	captureLog(t)
	ev = watchEvents(a)
	arch = realDir(t, "Фото")
	y26, y25 := mkDirs(t, arch, "2026"), mkDirs(t, arch, "2025")
	chosen = realDir(t, "Свои", "2024")
	gone = filepath.Join(realDir(t, "Пропал"), "нет", "2023") // ни папки, ни её родителя: проба диска откажет
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) {
		p.LiftDone = false // файл 0.34: отметки прохода нет
		p.Places = []prefs.Place{{Dir: y26, How: prefs.PlaceParent}, {Dir: chosen, How: prefs.PlaceChosen},
			{Dir: y25, How: prefs.PlaceParent}, {Dir: gone, How: prefs.PlaceParent}}
	})
	return a, ev, arch, chosen, gone
}

// Проход по местам 0.34 (решение «Поднять и предупредить»): места parent поднимаются, повторы
// сливаются, явное и недоступное остаются; одна запись с lifted, одно событие-указатель; «Понятно»
// чистит заметку; повторный проход ничего не пишет и не сообщает.
func TestPlaceLegacyLiftNote(t *testing.T) {
	a, ev, arch, chosen, gone := legacyApp(t)
	y26 := filepath.Join(arch, "2026")
	y25 := filepath.Join(arch, "2025")

	a.liftLegacyPlaces()
	pl := prefs.LoadAgentPerms(a.cfgDir).Places
	if len(pl) != 3 || pl[0] != (prefs.Place{Dir: arch, How: prefs.PlaceParent}) ||
		pl[1] != (prefs.Place{Dir: chosen, How: prefs.PlaceChosen}) || pl[2].Dir != gone {
		t.Fatalf("после прохода: %+v", pl)
	}
	lifted := prefs.LoadAgentPerms(a.cfgDir).Lifted
	if len(lifted) != 2 || lifted[0].To != arch || lifted[1].To != arch {
		t.Fatalf("lifted: %+v", lifted)
	}
	if ev.n(placesLiftedEvent) != 1 || len(ev.lifted) != 1 || len(ev.lifted[0]["from"]) != 2 || ev.lifted[0]["to"][0] != arch {
		t.Fatalf("указатель: %d раз, %+v", ev.n(placesLiftedEvent), ev.lifted)
	}
	froms := ev.lifted[0]["from"]
	if (froms[0] != y26 || froms[1] != y25) && (froms[0] != y25 || froms[1] != y26) {
		t.Errorf("откуда: %v", froms)
	}
	// Диск не привязывается, лишних ключей нет (TestPlaceLegacyNoBind).
	raw, _ := os.ReadFile(prefs.ConfigPath(a.cfgDir))
	if strings.Contains(string(raw), `"vol"`) {
		t.Errorf("проход привязал диск: %s", raw)
	}
	// Окно: заметка в AgentsInfo.
	info := a.AgentsInfo()
	if len(info.Lifted) != 2 || info.Lifted[0].To != arch {
		t.Fatalf("AgentsInfo.Lifted: %+v", info.Lifted)
	}

	// Повтор: ни записи, ни события, файл тот же.
	states, lifts := ev.n("agents:state"), ev.n(placesLiftedEvent)
	a.liftLegacyPlaces()
	after, _ := os.ReadFile(prefs.ConfigPath(a.cfgDir))
	if string(after) != string(raw) || ev.n("agents:state") != states || ev.n(placesLiftedEvent) != lifts {
		t.Fatalf("повторный проход изменил: события +%d/+%d\n%s", ev.n("agents:state")-states, ev.n(placesLiftedEvent)-lifts, after)
	}

	// «Понятно»: заметка уходит, места целы, повтор — без записи и события.
	info = a.DismissPlacesLifted()
	if len(info.Lifted) != 0 || len(prefs.LoadAgentPerms(a.cfgDir).Lifted) != 0 || len(info.Places) != 3 {
		t.Fatalf("после «Понятно»: lifted %+v, мест %d", info.Lifted, len(info.Places))
	}
	if data, _ := os.ReadFile(prefs.ConfigPath(a.cfgDir)); strings.Contains(string(data), `"lifted"`) {
		t.Errorf("lifted остался в файле: %s", data)
	}
	n := ev.n("agents:state")
	if info = a.DismissPlacesLifted(); len(info.Lifted) != 0 || ev.n("agents:state") != n {
		t.Errorf("повторное «Понятно»: событий +%d", ev.n("agents:state")-n)
	}
	// JSON ответа — список, не null.
	b, _ := json.Marshal(info)
	if !strings.Contains(string(b), `"lifted":[]`) {
		t.Errorf("lifted не пустой список: %s", b)
	}
}

// Проход ничего не делает без ключа agentPerms, без мест и с остановкой приложения; места вне
// правил остаются; внутренние места (0.34 их не чистила) убираются без заметки.
func TestPlaceLegacyEdges(t *testing.T) {
	a, _, _ := placeApp(t)
	captureLog(t)
	ev := watchEvents(a)
	a.liftLegacyPlaces() // ключа нет
	if prefs.LoadConfig(a.cfgDir).AgentPerms != nil || ev.n("agents:state") != 0 {
		t.Fatal("проход создал ключ agentPerms")
	}
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) { p.LiftDone = false }) // ключ есть, мест нет
	n := ev.n("agents:state")
	a.liftLegacyPlaces()
	if ev.n("agents:state") != n {
		t.Error("проход без мест сообщил об изменении")
	}
	if !prefs.LoadAgentPerms(a.cfgDir).LiftDone {
		t.Error("проход без мест не поставил отметку")
	}

	// Вложенные места из 0.34 (каждое — само по себе не год): внутренние уходят, заметки нет.
	root := realDir(t, "Корень")
	inner := mkDirs(t, root, "Свои", "Дальше")
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) {
		p.LiftDone = false
		p.Places = []prefs.Place{{Dir: inner, How: prefs.PlaceParent}, {Dir: root, How: prefs.PlaceParent}}
	})
	a.liftLegacyPlaces()
	pl := prefs.LoadAgentPerms(a.cfgDir).Places
	if len(pl) != 1 || pl[0].Dir != root || len(prefs.LoadAgentPerms(a.cfgDir).Lifted) != 0 || ev.n(placesLiftedEvent) != 0 {
		t.Fatalf("вложенные: %+v, lifted %v", pl, prefs.LoadAgentPerms(a.cfgDir).Lifted)
	}

	// Приложение остановлено до готовности: проход выходит, не трогая файл.
	b := NewApp()
	b.cfgDir = a.cfgDir
	b.stopCancel()
	before, _ := os.ReadFile(prefs.ConfigPath(a.cfgDir))
	b.liftLegacyPlaces()
	if after, _ := os.ReadFile(prefs.ConfigPath(a.cfgDir)); string(after) != string(before) {
		t.Error("остановленный проход писал")
	}
}

// Узкое место видно в AgentsInfo без диска: подсказка «Новый год спросит снова» (narrow) и «диск целиком».
func TestAgentsInfoNarrow(t *testing.T) {
	a, _, e := placeApp(t)
	captureLog(t)
	home := realDir(t, "home")
	e.Rules = realRules(func(o *agentplaces.Opts) { o.Homes = []string{home} })
	pics := mkDirs(t, home, "Pictures", "2026")
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) {
		p.Places = []prefs.Place{{Dir: pics, How: prefs.PlaceParent}, {Dir: realDir(t, "Арх"), How: prefs.PlaceParent}}
	})
	info := a.AgentsInfo()
	if len(info.Places) != 2 || !info.Places[0].Narrow || info.Places[0].NarrowRoot || info.Places[1].Narrow {
		t.Fatalf("narrow: %+v", info.Places)
	}
}
