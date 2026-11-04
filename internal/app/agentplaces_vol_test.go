package app

// Привязка места к диску (0.35/153, design §4.3–4.5): сверка по номеру тома в тихой проверке, диск
// пишет только действие человека, показ с пробами по раскрытию. Том подменён моделью дисков
// (fakeVols): корни — настоящие папки, «смена буквы» и «смена карты» — правка модели.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

type fakeVol struct{ root, id, label string }

// fakeVols — «диски машины»: корень (настоящая папка), номер и метка. calls/pathCalls считают пробы.
type fakeVols struct {
	mu               sync.Mutex
	vols             []fakeVol
	err              error
	hang             chan struct{}
	calls, pathCalls atomic.Int32
}

func (f *fakeVols) set(v ...fakeVol) { f.mu.Lock(); f.vols = v; f.mu.Unlock() }

func (f *fakeVols) of(p string) (osx.Vol, error) {
	f.calls.Add(1)
	if f.hang != nil {
		<-f.hang
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return osx.Vol{}, f.err
	}
	sep, best := string(filepath.Separator), -1
	for i, v := range f.vols {
		if (p == v.root || strings.HasPrefix(p, v.root+sep)) && (best < 0 || len(v.root) > len(f.vols[best].root)) {
			best = i
		}
	}
	if best < 0 {
		return osx.Vol{}, errors.New("у тома нет номера")
	}
	v := f.vols[best]
	return osx.Vol{Root: v.root, ID: v.id, Label: v.label, Rel: filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(p, v.root), sep))}, nil
}

func (f *fakeVols) path(id, rel string) (string, string, error) {
	f.pathCalls.Add(1)
	if f.hang != nil {
		<-f.hang
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, v := range f.vols {
		if v.id == id {
			return filepath.Join(v.root, filepath.FromSlash(rel)), v.label, nil
		}
	}
	return "", "", osx.ErrVolGone
}

// useFakeVols подставляет модель дисков вместо настоящих osx.VolumeOf / osx.VolumePath.
func useFakeVols(t *testing.T, f *fakeVols) {
	t.Helper()
	oldOf, oldPath := placeVolOf, placeVolPath
	placeVolOf, placeVolPath = f.of, f.path
	t.Cleanup(func() { placeVolOf, placeVolPath = oldOf, oldPath })
}

// volStand — App с правилами без запретов и двумя «слотами» E и F; на E лежит карта «card1».
type volStand struct {
	a      *App
	states func() int
	e      *agentplaces.Env
	f      *fakeVols
	E, F   string
}

func newVolStand(t *testing.T) *volStand {
	t.Helper()
	s := &volStand{f: &fakeVols{}}
	useFakeVols(t, s.f)
	s.a, s.states, s.e = placeApp(t)
	s.E, s.F = realDir(t, "E"), realDir(t, "F")
	for _, root := range []string{s.E, s.F} {
		if err := os.MkdirAll(filepath.Join(root, "Клиент", "Свадьба"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	s.f.set(fakeVol{s.E, "card1", "Фото-архив"})
	return s
}

func (s *volStand) places() []prefs.Place { return prefs.LoadAgentPerms(s.a.cfgDir).Places }

// why — что скажет тихая проверка о пути (путь настоящий).
func (s *volStand) why(p string) string {
	why, _ := s.a.placeWhy(s.e.Rules, p)
	return why
}

// Добавление папки человеком даёт месту диск; место 0.34 без диска, на котором лежит новая папка,
// тоже получает диск и поглощает новое как внутреннее; засев и пополнение без тома — без диска.
func TestPlaceBindByHuman(t *testing.T) {
	s := newVolStand(t)
	parent := filepath.Join(s.E, "Клиент")
	s.a.pushPlaces(prefs.PlaceParent, parent)
	got := s.places()
	if len(got) != 1 || got[0].Vol == nil || got[0].Vol.ID != "card1" || got[0].Vol.Rel != "Клиент" || got[0].Vol.Label != "Фото-архив" {
		t.Fatalf("новое место без диска: %+v", got)
	}

	// Место 0.34 (без диска) и добавленная глубже папка: старое получает диск, новое — внутри него.
	s2 := newVolStand(t)
	prefs.AddPlace(s2.a.cfgDir, prefs.Place{Dir: filepath.Join(s2.E, "Клиент"), How: prefs.PlaceParent})
	s2.a.pushPlaces(prefs.PlaceParent, filepath.Join(s2.E, "Клиент", "Свадьба"))
	got = s2.places()
	if len(got) != 1 || got[0].Dir != filepath.Join(s2.E, "Клиент") || got[0].Vol == nil || got[0].Vol.Rel != "Клиент" {
		t.Fatalf("старое место не получило диск: %+v", got)
	}

	// Старое место внутри нового (поддерево принимало бы чужой диск под той же буквой): остаётся одно
	// место нового, с диском; соседнее старое без отношения к новому не трогается.
	s4 := newVolStand(t)
	prefs.UpdateAgentPerms(s4.a.cfgDir, func(p *prefs.AgentPerms) {
		p.Places = []prefs.Place{{Dir: filepath.Join(s4.E, "Клиент", "Свадьба"), How: prefs.PlaceParent}, {Dir: filepath.Join(s4.E, "Другое"), How: prefs.PlaceParent}}
	})
	s4.a.pushPlaces(prefs.PlaceParent, filepath.Join(s4.E, "Клиент"))
	got = s4.places()
	if len(got) != 2 || got[0].Dir != filepath.Join(s4.E, "Клиент") || got[0].Vol == nil || got[0].Vol.ID != "card1" || got[1].Vol != nil {
		t.Fatalf("старое внутри нового: %+v", got)
	}

	// Старое внутри нового, но на другом диске (вложенный том): не трогаем, остаётся рядом.
	s5 := newVolStand(t)
	inner := filepath.Join(s5.E, "Клиент", "Свадьба")
	s5.f.set(fakeVol{s5.E, "card1", "Фото-архив"}, fakeVol{inner, "card2", "Вложенный"})
	prefs.AddPlace(s5.a.cfgDir, prefs.Place{Dir: inner, How: prefs.PlaceParent})
	s5.a.pushPlaces(prefs.PlaceParent, filepath.Join(s5.E, "Клиент"))
	got = s5.places()
	if len(got) != 2 || got[1].Dir != inner || got[1].Vol != nil {
		t.Fatalf("старое на другом диске: %+v", got)
	}

	// Тома узнать нельзя — место по пути, как в 0.34.
	s3 := newVolStand(t)
	s3.f.err = errors.New("нет")
	s3.a.pushPlaces(prefs.PlaceParent, filepath.Join(s3.E, "Клиент"))
	if got = s3.places(); len(got) != 1 || got[0].Vol != nil {
		t.Fatalf("при ошибке тома: %+v", got)
	}
}

// Несколько мест сразу: бросок двух папок с разных дисков — два места с разными дисками, одна запись.
func TestPlaceBindManyAtOnce(t *testing.T) {
	s := newVolStand(t)
	s.f.set(fakeVol{s.E, "card1", "Первая"}, fakeVol{s.F, "disk2", ""})
	n := s.states()
	s.a.pushPlaces(prefs.PlaceParent, filepath.Join(s.E, "Клиент"), filepath.Join(s.F, "Клиент"))
	got := s.places()
	if len(got) != 2 || got[0].Vol == nil || got[1].Vol == nil || got[0].Vol.ID != "card1" || got[1].Vol.ID != "disk2" {
		t.Fatalf("два диска: %+v", got)
	}
	if s.states() != n+1 {
		t.Errorf("событий agents:state +%d, ждали одно", s.states()-n)
	}
	// Повтор того же — холостой: ни записи, ни события.
	s.a.pushPlaces(prefs.PlaceParent, filepath.Join(s.E, "Клиент"), filepath.Join(s.F, "Клиент"))
	if s.states() != n+1 {
		t.Errorf("повтор поднял событие: +%d", s.states()-n-1)
	}
}

// «Папка со съёмками…» тоже пишет диск; другой диск по тому же пути — новое место, не перепривязка.
func TestPlaceBindChoose(t *testing.T) {
	s := newVolStand(t)
	pick := ""
	old := openDirDialog
	openDirDialog = func(context.Context, wr.OpenDialogOptions) (string, error) { return pick, nil }
	t.Cleanup(func() { openDirDialog = old })
	pick = filepath.Join(s.E, "Клиент")
	if _, err := s.a.ChooseAgentPlace(); err != nil {
		t.Fatal(err)
	}
	if got := s.places(); len(got) != 1 || got[0].Vol == nil || got[0].Vol.ID != "card1" || got[0].How != prefs.PlaceChosen {
		t.Fatalf("выбор: %+v", got)
	}
	s.f.set(fakeVol{s.E, "card2", "Вторая"})
	if _, err := s.a.ChooseAgentPlace(); err != nil {
		t.Fatal(err)
	}
	got := s.places()
	if len(got) != 2 || got[0].Vol.ID != "card2" || got[1].Vol.ID != "card1" {
		t.Fatalf("другой диск по тому же пути: %+v", got)
	}
}

// Засев и проход по местам 0.34 диск не пишут (да и не читают); тихая проверка 0.34-места — по пути.
func TestPlaceLegacyNoBind(t *testing.T) {
	s := newVolStand(t)
	old := filepath.Join(s.E, "Клиент")
	prefs.AddPlace(s.a.cfgDir, prefs.Place{Dir: filepath.Join(s.E, "2026"), How: prefs.PlaceParent})
	if err := os.Mkdir(filepath.Join(s.E, "2026"), 0o755); err != nil {
		t.Fatal(err)
	}
	prefs.AddPlace(s.a.cfgDir, prefs.Place{Dir: old, How: prefs.PlaceParent})
	prefs.UpdateAgentPerms(s.a.cfgDir, func(p *prefs.AgentPerms) { p.LiftDone = false }) // файл 0.34
	s.a.liftLegacyPlaces()
	s.a.seedPlaces()
	for _, p := range s.places() {
		if p.Vol != nil {
			t.Fatalf("проход записал диск: %+v", p)
		}
	}
	if why := s.why(filepath.Join(old, "Свадьба")); why != "" {
		t.Errorf("место 0.34 по пути: why = %q", why)
	}
	if n := s.f.calls.Load(); n != 0 {
		t.Errorf("тома читались %d раз при местах без диска", n)
	}

	// Засев при переходе с 0.33: место из lastDirs.photos тоже без диска.
	s2 := newVolStand(t)
	prefs.RememberDir(s2.a.cfgDir, prefs.DirPhotos, filepath.Join(s2.E, "Клиент", "Свадьба")) // мимо rememberDir: он сеет сам
	s2.a.seedPlaces()
	if got := s2.places(); len(got) == 0 || got[0].Vol != nil {
		t.Fatalf("засев: %+v", got)
	}
	if n := s2.f.calls.Load(); n != 0 {
		t.Errorf("засев читал том %d раз", n)
	}
}

// Привязанное место: буква сменилась — свой диск узнан (тихо); другой диск на старой букве — whyDisk;
// карта между двумя проверками сменилась — вторая проверка видит это (кэша тома в проверке нет).
func TestPlaceVolLetterMoved(t *testing.T) {
	s := newVolStand(t)
	s.a.pushPlaces(prefs.PlaceParent, filepath.Join(s.E, "Клиент"))
	inE, inF := filepath.Join(s.E, "Клиент", "Свадьба"), filepath.Join(s.F, "Клиент", "Свадьба")
	if why := s.why(inE); why != "" {
		t.Fatalf("свой диск на своей букве: %q", why)
	}
	s.f.set(fakeVol{s.F, "card1", "Фото-архив"}) // карту пересадили под F
	if why := s.why(inF); why != "" {
		t.Errorf("буква сменилась, диск тот же: %q", why)
	}
	// Место само и путь вне места — по-прежнему не тихо.
	if why := s.why(filepath.Join(s.F, "Клиент")); why != whyPlace {
		t.Errorf("само место: %q", why)
	}
	if why := s.why(filepath.Join(s.F, "Другое")); why != whyPlace {
		t.Errorf("вне места: %q", why)
	}
}

func TestPlaceVolForeignDisk(t *testing.T) {
	s := newVolStand(t)
	s.a.pushPlaces(prefs.PlaceParent, filepath.Join(s.E, "Клиент"))
	s.f.set(fakeVol{s.E, "stranger", "Чужая"})
	why, note := s.a.placeWhy(s.e.Rules, filepath.Join(s.E, "Клиент", "Свадьба"))
	if why != whyDisk || !strings.Contains(note, filepath.Join(s.E, "Клиент")) || !strings.Contains(note, "«Фото-архив»") || strings.Contains(note, "Чужая") {
		t.Fatalf("чужой диск: %q, %q", why, note)
	}
}

func TestPlaceVolNoCacheInCheck(t *testing.T) {
	s := newVolStand(t)
	s.a.pushPlaces(prefs.PlaceParent, filepath.Join(s.E, "Клиент"))
	p := filepath.Join(s.E, "Клиент", "Свадьба")
	start := time.Now()
	first := s.why(p)
	s.f.set(fakeVol{s.E, "stranger", ""}) // карту в слоте сменили за секунду
	second := s.why(p)
	if first != "" || second != whyDisk {
		t.Fatalf("первая %q, вторая %q", first, second)
	}
	if time.Since(start) > testkit.Limit(time.Second) {
		t.Fatal("две проверки заняли больше секунды — тест не про это")
	}
}

// Две карты одной камеры под одной буквой: у каждой своё место, без перебросов; на каждой тихо.
func TestPlaceTwoCardsSamePath(t *testing.T) {
	s := newVolStand(t)
	parent, shoot := filepath.Join(s.E, "Клиент"), filepath.Join(s.E, "Клиент", "Свадьба")
	s.a.pushPlaces(prefs.PlaceParent, parent) // карта 1
	s.f.set(fakeVol{s.E, "card2", "Карта 2"})
	if why := s.why(shoot); why != whyDisk {
		t.Fatalf("карта 2 до привязки: %q", why)
	}
	s.a.pushPlaces(prefs.PlaceParent, parent) // человек добавил с карты 2
	got := s.places()
	if len(got) != 2 || got[0].Vol.ID != "card2" || got[1].Vol.ID != "card1" {
		t.Fatalf("два места: %+v", got)
	}
	if why := s.why(shoot); why != "" {
		t.Errorf("карта 2: %q", why)
	}
	s.f.set(fakeVol{s.E, "card1", "Фото-архив"})
	s.a.pushPlaces(prefs.PlaceParent, parent) // снова карта 1: повтор, места те же
	if got = s.places(); len(got) != 2 || got[0].Vol.ID != "card1" || got[1].Vol.ID != "card2" {
		t.Fatalf("вернулась карта 1: %+v", got)
	}
	if why := s.why(shoot); why != "" {
		t.Errorf("карта 1: %q", why)
	}
	s.f.set(fakeVol{s.E, "card3", ""})
	if why := s.why(shoot); why != whyDisk {
		t.Errorf("третья карта: %q", why)
	}
}

// Ошибка тома и зависший том — вопрос «не удалось узнать диск»; место без диска рядом по пути тихо.
func TestPlaceVolUnknownAsks(t *testing.T) {
	s := newVolStand(t)
	s.a.pushPlaces(prefs.PlaceParent, filepath.Join(s.E, "Клиент"))
	p := filepath.Join(s.E, "Клиент", "Свадьба")
	s.f.err = errors.New("не прочитали")
	if why := s.why(p); why != whyVol {
		t.Errorf("ошибка тома: %q", why)
	}
	s.f.err = nil
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	s.f.hang = release
	s.e.Rules = realRules(func(o *agentplaces.Opts) { o.Timeout = 50 * time.Millisecond })
	start := time.Now()
	if why := s.why(p); why != whyVol {
		t.Errorf("зависший том: %q", why)
	}
	if d := time.Since(start); d > testkit.Limit(2*time.Second) {
		t.Errorf("зависший том держал проверку %v", d)
	}
}

// Вопрос агента с причиной disk: Why и строка «Место … — на диске …» в абзацах окна.
func TestPlaceWhyOtherDisk(t *testing.T) {
	r := newQuietRig(t)
	root := filepath.Dir(r.place)
	f := &fakeVols{}
	f.set(fakeVol{root, "card1", "Фото-архив"})
	useFakeVols(t, f)
	prefs.UpdateAgentPerms(r.a.cfgDir, func(p *prefs.AgentPerms) {
		p.Places = []prefs.Place{{Dir: r.place, How: prefs.PlaceChosen, Vol: &prefs.PlaceVol{ID: "card1", Label: "Фото-архив", Rel: "Архив"}}}
	})
	if c := r.run(t, r.addQuiet(r.shoot)); c.err != nil || c.res.Why != agents.WhyQuiet {
		t.Fatalf("свой диск: %+v, %v", c.res, c.err)
	}
	f.set(fakeVol{root, "stranger", "Чужая"})
	c := r.ask(r.addQuiet(r.shoot))
	ev := r.refuse(t, c, 1)
	if ev.Why != whyDisk {
		t.Fatalf("причина: %q", ev.Why)
	}
	if !strings.Contains(strings.Join(ev.Lines, "\n"), "Место «"+r.place+"» — на диске «Фото-архив»") {
		t.Errorf("строки вопроса: %q", ev.Lines)
	}
	f.err = errors.New("нет")
	c = r.ask(r.addQuiet(r.shoot))
	if ev = r.refuse(t, c, 2); ev.Why != whyVol {
		t.Errorf("ошибка тома: %q", ev.Why)
	}
}

// ForgetAgentPlace: по ключу, после смены буквы; чужой ключ и повтор — без записи и события.
func TestForgetAgentPlaceByKey(t *testing.T) {
	s := newVolStand(t)
	s.a.pushPlaces(prefs.PlaceParent, filepath.Join(s.E, "Клиент"))
	s.a.pushPlaces(prefs.PlaceParent, filepath.Join(s.F, "Свои"))
	if err := os.Mkdir(filepath.Join(s.F, "Свои"), 0o755); err != nil {
		t.Fatal(err)
	}
	s.a.pushPlaces(prefs.PlaceParent, filepath.Join(s.F, "Свои"))
	got := s.places()
	if len(got) != 2 {
		t.Fatalf("мест %d: %+v", len(got), got)
	}
	n := s.states()
	if info := s.a.ForgetAgentPlace("000000000000"); len(info.Places) != 2 || s.states() != n {
		t.Fatalf("чужой ключ: %+v, событий +%d", info.Places, s.states()-n)
	}
	s.f.set(fakeVol{s.F, "card1", "Фото-архив"}) // буква сменилась — ключ тот же
	key := prefs.PlaceKey(got[len(got)-1])
	if got[len(got)-1].Vol == nil || got[len(got)-1].Vol.ID != "card1" {
		t.Fatalf("не то место в хвосте: %+v", got)
	}
	info := s.a.ForgetAgentPlace(key)
	if len(info.Places) != 1 || info.Places[0].Key == key || s.states() != n+1 {
		t.Fatalf("после ✕: %+v, событий +%d", info.Places, s.states()-n)
	}
	if info = s.a.ForgetAgentPlace(key); len(info.Places) != 1 || s.states() != n+1 {
		t.Fatalf("повтор: %+v, событий +%d", info.Places, s.states()-n)
	}
}

// BindAgentPlace: привязывает место по ключу; ошибка тома — место прежнее и ошибка; привязанное,
// чужой ключ, ссылка вместо папки — холостой вызов.
func TestPlaceBindManual(t *testing.T) {
	s := newVolStand(t)
	dir := filepath.Join(s.E, "Клиент")
	prefs.AddPlace(s.a.cfgDir, prefs.Place{Dir: dir, How: prefs.PlaceParent})
	key := prefs.PlaceKey(prefs.Place{Dir: dir})
	n := s.states()

	s.f.err = errors.New("нет номера")
	info, err := s.a.BindAgentPlace(key)
	if !errors.Is(err, agentplaces.ErrVolUnknown) || len(info.Places) != 1 || info.Places[0].Bound || s.places()[0].Vol != nil || s.states() != n {
		t.Fatalf("ошибка тома: %v, %+v", err, info.Places)
	}
	s.f.err = nil
	info, err = s.a.BindAgentPlace(key)
	if err != nil || len(info.Places) != 1 || !info.Places[0].Bound || info.Places[0].Label != "Фото-архив" || s.states() != n+1 {
		t.Fatalf("привязка: %v, %+v, событий +%d", err, info.Places, s.states()-n)
	}
	if v := s.places()[0].Vol; v == nil || v.ID != "card1" || v.Rel != "Клиент" {
		t.Fatalf("диск места: %+v", v)
	}
	// Повтор по прежнему ключу (место уже с диском, ключ другой) и по новому — холостые.
	s.f.set(fakeVol{s.E, "card2", "Чужая"})
	for _, k := range []string{key, info.Places[0].Key, "000000000000"} {
		if info, err = s.a.BindAgentPlace(k); err != nil || s.states() != n+1 || s.places()[0].Vol.ID != "card1" {
			t.Fatalf("холостой вызов %q: %v, событий +%d, %+v", k, err, s.states()-n, s.places())
		}
	}
	// Папки нет на диске — привязать нельзя, место прежнее.
	gone := filepath.Join(s.F, "Пропала")
	prefs.AddPlace(s.a.cfgDir, prefs.Place{Dir: gone, How: prefs.PlaceChosen})
	if _, err = s.a.BindAgentPlace(prefs.PlaceKey(prefs.Place{Dir: gone})); !errors.Is(err, agentplaces.ErrPlaceBad) {
		t.Errorf("нет папки: %v", err)
	}
}

// AgentsInfo диск не трогает ни при показе, ни при смене разрешений.
func TestAgentsInfoNoVolumeScan(t *testing.T) {
	s := newVolStand(t)
	s.a.pushPlaces(prefs.PlaceParent, filepath.Join(s.E, "Клиент"))
	prefs.AddPlace(s.a.cfgDir, prefs.Place{Dir: filepath.Join(s.F, "Клиент"), How: prefs.PlaceChosen})
	s.f.calls.Store(0)
	for range 5 {
		if info := s.a.AgentsInfo(); len(info.Places) != 2 {
			t.Fatalf("места: %+v", info.Places)
		}
	}
	s.a.SetAgentPerm("add_folder", prefs.PermAsk)
	s.a.ForgetAgentPlace("000000000000")
	if c, p := s.f.calls.Load(), s.f.pathCalls.Load(); c != 0 || p != 0 {
		t.Errorf("пробы дисков при показе: VolumeOf %d, VolumePath %d", c, p)
	}
}

// AgentPlacesLive: путь по номеру диска, nodisk, nofolder, here (метка и без неё), таймаут, кэш 15 с.
func TestAgentPlacesLive(t *testing.T) {
	s := newVolStand(t)
	G := realDir(t, "G")
	if err := os.MkdirAll(filepath.Join(G, "Свои"), 0o755); err != nil {
		t.Fatal(err)
	}
	s.f.set(fakeVol{s.E, "card1", "Фото-архив"}, fakeVol{G, "disk3", ""})
	bound := func(id, rel, dir string) prefs.Place {
		return prefs.Place{Dir: dir, How: prefs.PlaceParent, Vol: &prefs.PlaceVol{ID: id, Label: "старая", Rel: rel}}
	}
	places := []prefs.Place{
		bound("card1", "Клиент", filepath.Join(s.F, "Клиент")), // буква была F, теперь E
		bound("ушла", "Архив", filepath.Join(s.F, "Архив")),    // диска нет
		bound("card1", "Нет", filepath.Join(s.E, "Нет")),       // диск есть, папки нет
		{Dir: filepath.Join(G, "Свои"), How: prefs.PlaceChosen},
		{Dir: filepath.Join(s.F, "Исчезла"), How: prefs.PlaceChosen},
		{Dir: filepath.Join(s.F, "Клиент", "Свадьба"), How: prefs.PlaceChosen}, // без диска, есть папка, диск F: «here»
	}
	prefs.UpdateAgentPerms(s.a.cfgDir, func(p *prefs.AgentPerms) { p.Places = places })
	s.f.set(fakeVol{s.E, "card1", "Фото-архив"}, fakeVol{G, "disk3", ""}, fakeVol{s.F, "disk4", "Запасной"})
	byDir := func(list []AgentPlace) map[string]AgentPlace {
		m := map[string]AgentPlace{}
		for _, p := range list {
			m[p.Key] = p
		}
		return m
	}
	live := byDir(s.a.AgentPlacesLive())
	get := func(p prefs.Place) AgentPlace {
		t.Helper()
		v, ok := live[prefs.PlaceKey(p)]
		if !ok {
			t.Fatalf("места %v нет в ответе: %+v", p, live)
		}
		return v
	}
	if v := get(places[0]); v.Dir != filepath.Join(s.E, "Клиент") || v.Label != "Фото-архив" || v.State != "" || !v.Bound {
		t.Errorf("буква сменилась: %+v", v)
	}
	if v := get(places[1]); v.State != placeNoDisk || v.Dir != places[1].Dir || v.Label != "старая" {
		t.Errorf("диска нет: %+v", v)
	}
	if v := get(places[2]); v.State != placeNoFolder || v.Dir != filepath.Join(s.E, "Нет") {
		t.Errorf("папки нет: %+v", v)
	}
	if v := get(places[3]); v.State != placeHere || v.Here != "" || v.Bound {
		t.Errorf("here без метки: %+v", v)
	}
	if v := get(places[4]); v.State != placeNoFolder {
		t.Errorf("непривязанное без папки: %+v", v)
	}
	if v := get(places[5]); v.State != placeHere || v.Here != "Запасной" {
		t.Errorf("here с меткой: %+v", v)
	}
	if got := prefs.LoadAgentPerms(s.a.cfgDir).Places; got[0].Dir != places[0].Dir {
		t.Errorf("показ переписал config: %+v", got[0])
	}

	// Кэш 15 с: повтор без проб; через 16 с — пробы снова; изменился список мест — сразу снова.
	now := time.Now()
	old := placeLiveNow
	placeLiveNow = func() time.Time { return now }
	t.Cleanup(func() { placeLiveNow = old })
	s.a.AgentPlacesLive()
	calls := s.f.calls.Load() + s.f.pathCalls.Load()
	s.a.AgentPlacesLive()
	if s.f.calls.Load()+s.f.pathCalls.Load() != calls {
		t.Error("кэш не сработал: повторный вызов пробует диски")
	}
	now = now.Add(16 * time.Second)
	s.a.AgentPlacesLive()
	if s.f.calls.Load()+s.f.pathCalls.Load() == calls {
		t.Error("кэш не истёк через 16 с")
	}
	calls = s.f.calls.Load() + s.f.pathCalls.Load()
	s.a.ForgetAgentPlace(prefs.PlaceKey(places[4]))
	if list := s.a.AgentPlacesLive(); len(list) != 5 || s.f.calls.Load()+s.f.pathCalls.Load() == calls {
		t.Errorf("после ✕: %d мест, проб +%d", len(list), s.f.calls.Load()+s.f.pathCalls.Load()-calls)
	}

	// Таймаут: места как в config, без состояний; не кэшируется.
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	s.f.hang = release
	placeLiveWithin = 50 * time.Millisecond
	t.Cleanup(func() { placeLiveWithin = 1500 * time.Millisecond })
	now = now.Add(time.Minute)
	start := time.Now()
	list := s.a.AgentPlacesLive()
	if d := time.Since(start); d > testkit.Limit(2*time.Second) {
		t.Errorf("зависший диск держал показ %v", d)
	}
	if len(list) != 5 {
		t.Fatalf("при таймауте %d мест", len(list))
	}
	for _, p := range list {
		if p.State != "" || p.Here != "" {
			t.Errorf("при таймауте есть состояние: %+v", p)
		}
	}
}

// Стоимость VolumeOf в тихой проверке: настоящий вызов ОС (на машине без номера — быстрый отказ)
// укладывается в пределы design §4.3.
func TestPlaceVolCost(t *testing.T) {
	testkit.CostTest(t)
	s := newVolStand(t)
	s.a.pushPlaces(prefs.PlaceParent, filepath.Join(s.E, "Клиент"))
	placeVolOf = osx.VolumeOf // настоящий том вместо модели (useFakeVols вернёт подмену в cleanup)
	p := filepath.Join(s.E, "Клиент", "Свадьба")
	d := testkit.Fastest(5, func() {
		for range 20 {
			s.why(p)
		}
	}) / 20
	if d > testkit.Limit(25*time.Millisecond) {
		t.Errorf("проверка места с диском %v на вызов, порог 25 мс", d)
	}
}
