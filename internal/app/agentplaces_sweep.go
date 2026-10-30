package app

// Проход по местам 0.34 при первом запуске 0.35 (153, design §2.4; решение владельца «Поднять и
// предупредить», 0.35-153-11, -16): места «parent» поднимаются до корня архива, внутренние убираются,
// поднятое пишется в agentPerms.lifted — заметка в «ИИ-агенты» до «Понятно» — и сообщается одним
// событием-указателем. Диск к месту не привязывается (это действие человека, Т3).

import (
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// AgentPlaceLift — «было → стало» для заметки о подъёме мест.
type AgentPlaceLift struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// placesLiftedEvent — для тестов (в a.emit имя литералом: его видит страж TestEventsContract); тост-указатель «Места съёмок для агента расширены · Посмотреть»: один на все места.
const placesLiftedEvent = "agents:placesLifted"

// liftLegacyPlaces — один проход после запуска окна (в фоне: диск под within, окно не ждёт). Повтор ничего
// не меняет: поднятое не датировано, внутренних нет — ни записи, ни события.
func (a *App) liftLegacyPlaces() {
	select {
	case <-a.ready:
	case <-a.stopCtx.Done():
		return
	}
	if a.loadConfig().AgentPerms == nil {
		return // ключа нет — мест 0.34 нет; засев (seedPlaces) сразу поднимает сам и ставит отметку
	}
	env := a.placeEnv()
	if env == nil {
		return
	}
	perms := prefs.LoadAgentPerms(a.cfgDir)
	if perms.LiftDone {
		return // проход 0.35 уже был: места без диска теперь и от 0.35 (Mac), выше не поднимаем
	}
	places := perms.Places
	moves := map[string]prefs.PlaceLift{}
	for _, p := range places {
		if p.How != prefs.PlaceParent || p.Vol != nil {
			continue
		}
		if to, _, _, err := env.lift(p.Dir); err == nil && paths.DirKey(to) != paths.DirKey(p.Dir) {
			moves[paths.DirKey(p.Dir)] = prefs.PlaceLift{From: p.Dir, To: to}
		}
	}
	changed := len(moves) > 0 || !sameOrder(places, prefs.DropInnerPlaces(places))
	var done []prefs.PlaceLift
	// Отметка ставится той же записью, что и подъём (и когда поднимать нечего — одной записью).
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) {
		done = p.LiftPlaces(moves)
		p.LiftDone = true
	})
	if !changed {
		return
	}
	if len(done) > 0 {
		from, to := make([]string, len(done)), make([]string, len(done))
		for i, d := range done {
			from[i], to[i] = d.From, d.To
		}
		a.emit("agents:placesLifted", map[string][]string{"from": from, "to": to})
	}
	a.agents.PermsChanged()
	a.agentsChanged()
}

// DismissPlacesLifted — «Понятно» у заметки о подъёме мест: lifted чистится. Нечего чистить — без записи.
func (a *App) DismissPlacesLifted() AgentsInfo {
	defer guard("DismissPlacesLifted")
	if len(prefs.LoadAgentPerms(a.cfgDir).Lifted) > 0 {
		prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) { p.Lifted = nil })
		a.agentsChanged()
	}
	return a.agentsInfo()
}

// agentLifted — заметка о подъёме для AgentsInfo; не nil.
func (a *App) agentLifted() []AgentPlaceLift {
	out := []AgentPlaceLift{}
	for _, l := range prefs.LoadAgentPerms(a.cfgDir).Lifted {
		out = append(out, AgentPlaceLift(l))
	}
	return out
}
