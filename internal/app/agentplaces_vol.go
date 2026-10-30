package app

// Привязка места к диску (0.35/153, design §4.3–4.4): тихая проверка узнаёт место по номеру тома, а
// не по букве. Том читается каждый раз, без кэша (смена карты в слоте — секунды); кэш есть только у
// показа (agentplaces_live.go). Том место получает только от действия человека (pushPlaces,
// ChooseAgentPlace, BindAgentPlace); засев, подъём и тихие проверки его не пишут.

import (
	"errors"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// errVolUnknown — номер диска места узнать не удалось (ошибка, таймаут, у тома нет номера, macOS).
var errVolUnknown = errors.New("не удалось узнать диск")

// volFn — чтение тома: подмена placeEnv.vol или настоящее.
func (e *placeEnv) volFn() func(string) (osx.Vol, error) {
	if e != nil && e.vol != nil {
		return e.vol
	}
	return osx.VolumeOf
}

// volPathFn — поиск тома по номеру: подмена placeEnv.volPath или настоящий.
func (e *placeEnv) volPathFn() func(id, rel string) (string, string, error) {
	if e != nil && e.volPath != nil {
		return e.volPath
	}
	return osx.VolumePath
}

// volOfDir — диск настоящего пути dir под предельным временем; nil — узнать не удалось (место
// остаётся по пути, как в 0.34).
func (e *placeEnv) volOfDir(dir string) *prefs.PlaceVol {
	if e == nil || e.rules == nil {
		return nil
	}
	fn := e.volFn() // до within: зависшая проба переживёт предел, подмену позже читать нельзя
	type res struct {
		v   osx.Vol
		err error
	}
	got, ok := within(e.rules.timeout, func() res {
		v, err := fn(dir)
		return res{v, err}
	})
	if !ok || got.err != nil {
		return nil
	}
	return &prefs.PlaceVol{ID: got.v.ID, Label: got.v.Label, Rel: got.v.Rel}
}

// bindInside — места без привязки в cur, внутри которых (вместе с которыми) или снаружи которых лежит
// одно из add с диском: данные диска, если место и правда на этом диске (проба тома по его пути; не
// удалась — место не трогаем). Ключ — prefs.PlaceKey. Иначе новое место с диском осталось бы рядом
// со старым без него (разные виды «внутри» не сравниваются): место 0.34 не получило бы диск, а
// поддерево внутри нового принимало бы чужой диск под той же буквой.
func (e *placeEnv) bindInside(cur, add []prefs.Place) map[string]prefs.PlaceVol {
	var binds map[string]prefs.PlaceVol
	for _, u := range cur {
		if u.Vol != nil {
			continue
		}
		for _, n := range add {
			nk, uk := paths.DirKey(n.Dir), paths.DirKey(u.Dir)
			if n.Vol == nil || !strings.HasPrefix(nk, uk) && !strings.HasPrefix(uk, nk) {
				continue
			}
			if v := e.volOfDir(u.Dir); v != nil && v.ID == n.Vol.ID {
				if binds == nil {
					binds = map[string]prefs.PlaceVol{}
				}
				binds[prefs.PlaceKey(u)] = *v
			}
			break
		}
	}
	return binds
}

// volSegs — число элементов пути внутри тома («a/b» — 2, корень — 0).
func volSegs(rel string) int {
	if rel == "" {
		return 0
	}
	return strings.Count(rel, "/") + 1
}

// relBelow — rel строго внутри base (пути внутри тома через «/»; регистр — по правилам ОС).
func (r *folderRules) relBelow(rel, base string) bool {
	rel, base = r.fold(rel), r.fold(base)
	if base == "" {
		return rel != ""
	}
	return strings.HasPrefix(rel, base+"/")
}

// placeRes — итог сверки с местами: ok — внутри; иначе причина и строка для вопроса.
type placeRes struct {
	ok        bool
	why, note string
}

// placeWhy — настоящий путь строго внутри места, а место на диске разрешается в себя же (подменённая
// ссылка-место не расширяет доступ): why = "". Иначе причина: whyDisk (путь внутри места другого
// диска; note — строка для вопроса), whyVol (диск узнать не удалось), whyPlace. Место без диска
// сверяется по пути (0.34); с диском — по номеру тома настоящего пути: один VolumeOf на проверку,
// без кэша. Диск — под пределом.
func (a *App) placeWhy(r *folderRules, resolved string) (why, note string) {
	places := a.usablePlaces()
	env := a.placeEnv()
	if len(places) == 0 || env == nil {
		return whyPlace, ""
	}
	disk, volOf := r.disk, env.volFn()
	hasVol := false
	for _, pl := range places {
		hasVol = hasVol || pl.Vol != nil
	}
	res, done := within(r.timeout, func() placeRes {
		var v osx.Vol
		var verr error
		if hasVol {
			v, verr = volOf(resolved)
		}
		return scanPlaces(env, disk, resolved, places, v, verr)
	})
	switch {
	case !done && hasVol:
		return whyVol, "" // диск не ответил за предел: тихо нельзя, спросить
	case !done:
		return whyPlace, ""
	case res.ok:
		return "", ""
	case res.why != "":
		return res.why, res.note
	}
	return whyPlace, ""
}

// volPlaceSelf — место с диском, каким оно лежит сейчас (resolved без хвоста из depth элементов),
// проходит строковые правила своего способа и разрешается в себя же.
func volPlaceSelf(env *placeEnv, disk func(string) (string, error), resolved string, pl prefs.Place, depth int) bool {
	r := env.rules
	cur := resolved
	for range depth {
		cur = r.parent(cur)
	}
	if env.static(cur, pl.How) != nil {
		return false
	}
	self, err := disk(cur)
	return err == nil && r.key(self) == r.key(cur)
}

// diskNote — строка вопроса «другой диск»: что за место и на каком диске оно было.
func diskNote(pl prefs.Place) string {
	if pl.Vol.Label == "" {
		return "Место «" + pl.Dir + "» — на другом диске"
	}
	return "Место «" + pl.Dir + "» — на диске «" + pl.Vol.Label + "»"
}

// scanPlaces — сверка пути со всеми местами (под within): v, verr — том пути, прочитанный один раз.
func scanPlaces(env *placeEnv, disk func(string) (string, error), resolved string, places []prefs.Place, v osx.Vol, verr error) placeRes {
	r := env.rules
	var out placeRes
	for _, pl := range places {
		strictly := r.inside(resolved, pl.Dir) && r.key(resolved) != r.key(pl.Dir)
		switch {
		case pl.Vol == nil:
			if !strictly {
				continue
			}
			if self, err := disk(pl.Dir); err == nil && r.key(self) == r.key(pl.Dir) {
				return placeRes{ok: true}
			}
		case verr == nil && v.ID == pl.Vol.ID:
			if r.relBelow(v.Rel, pl.Vol.Rel) && volPlaceSelf(env, disk, resolved, pl, volSegs(v.Rel)-volSegs(pl.Vol.Rel)) {
				return placeRes{ok: true}
			}
		case out.why == "" && strictly:
			// Не тот диск или диск не узнан: причину подсказывает путь по строке.
			if verr != nil {
				out.why = whyVol
			} else {
				out.why, out.note = whyDisk, diskNote(pl)
			}
		}
	}
	return out
}
