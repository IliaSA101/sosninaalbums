package app

// Показ мест с дисками (0.35/153, design §4.5): AgentsInfo.Places строится из config.json без проб
// (agents:state идёт на каждое событие окна — спящий диск не должен просыпаться), а текущий путь,
// «диска нет» и «папки нет» узнаёт AgentPlacesLive по раскрытию списка. Кэш 15 с — только здесь:
// показ не решает, тихо агент берёт папку или спрашивает (там том читается каждый раз).

import (
	"strings"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Подмена в тестах: предел одного вызова, срок кэша и часы.
var (
	placeLiveWithin = 1500 * time.Millisecond
	placeLiveTTL    = 15 * time.Second
	placeLiveNow    = time.Now
)

// placesLiveCache — последний удавшийся ответ AgentPlacesLive; sig — ключи мест, под которые он снят:
// список изменился (добавили, убрали, привязали) — ответ устарел сразу.
type placesLiveCache struct {
	mu   sync.Mutex
	at   time.Time
	sig  string
	list []AgentPlace
}

// AgentPlacesLive — места для раскрытого списка: у привязанных текущий путь по номеру диска (буква
// могла смениться), «диска нет» и «папки нет»; у непривязанных — метка диска, который сейчас по их
// пути (State = here: можно привязать). Все пробы одним вызовом под 1,5 с; не успели — места как
// в config.json, без состояния. Config не переписывается.
func (a *App) AgentPlacesLive() []AgentPlace {
	defer guard("AgentPlacesLive")
	places := a.savedPlaces()
	base := a.placeDTOs(places)
	keys := make([]string, len(base))
	for i, d := range base {
		keys[i] = d.Key
	}
	sig := strings.Join(keys, ",")
	c := &a.placesLive
	c.mu.Lock()
	if c.sig == sig && placeLiveNow().Sub(c.at) < placeLiveTTL {
		out := append([]AgentPlace(nil), c.list...)
		c.mu.Unlock()
		return out
	}
	c.mu.Unlock()

	env := a.placeEnv()
	if env == nil {
		return base
	}
	volOf, volPath := env.VolFn(), env.VolPathFn() // до Within: подмену не читаем в пережившей предел горутине
	list, ok := appkit.Within(placeLiveWithin, func() []AgentPlace {
		out := append([]AgentPlace(nil), base...)
		for i, p := range places {
			probePlace(&out[i], p, volOf, volPath)
		}
		return out
	})
	if !ok {
		return base
	}
	c.mu.Lock()
	c.at, c.sig, c.list = placeLiveNow(), sig, append([]AgentPlace(nil), list...)
	c.mu.Unlock()
	return list
}

// probePlace дописывает в d состояние места p по диску (вызов под Within): сами пробы — в
// agentplaces.Probe, здесь только перенос итога в AgentPlace.
func probePlace(d *AgentPlace, p prefs.Place, volOf func(string) (osx.Vol, error), volPath func(id, rel string) (string, string, error)) {
	res := agentplaces.Probe(p, volOf, volPath)
	if res.HasPath {
		d.Dir = res.Dir
		if res.Label != "" {
			d.Label = res.Label
		}
	}
	if res.State != "" {
		d.State, d.Here = res.State, res.Here
	}
}
