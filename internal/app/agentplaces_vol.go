package app

// Сверка пути с местами для тихой проверки (0.35/153, design §4.3–4.4): правила и сверка по тому —
// internal/app/agentplaces; здесь только сбор мест и подмен App. Том читается каждый раз, без кэша.

import (
	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
)

// placeWhy — настоящий путь строго внутри места, а место на диске разрешается в себя же (подменённая
// ссылка-место не расширяет доступ): why = "". Иначе причина: whyDisk (путь внутри места другого
// диска; note — строка для вопроса), whyVol (диск узнать не удалось), whyPlace. Место без диска
// сверяется по пути (0.34); с диском — по номеру тома настоящего пути: один VolumeOf на проверку,
// без кэша. Диск — под пределом.
func (a *App) placeWhy(r *agentplaces.Rules, resolved string) (why, note string) {
	places := a.usablePlaces()
	env := a.placeEnv()
	if len(places) == 0 || env == nil {
		return whyPlace, ""
	}
	disk, volOf := r.Disk(), env.VolFn()
	hasVol := false
	for _, pl := range places {
		hasVol = hasVol || pl.Vol != nil
	}
	res, done := appkit.Within(r.Timeout(), func() agentplaces.ScanRes {
		var v osx.Vol
		var verr error
		if hasVol {
			v, verr = volOf(resolved)
		}
		return agentplaces.Scan(env, disk, resolved, places, v, verr)
	})
	switch {
	case !done && hasVol:
		return whyVol, "" // диск не ответил за предел: тихо нельзя, спросить
	case !done:
		return whyPlace, ""
	case res.OK:
		return "", ""
	case res.Why != "":
		return res.Why, res.Note
	}
	return whyPlace, ""
}
