package agentplaces

// Привязка места к диску (0.35/153, design §4.3–4.4): тихая проверка узнаёт место по номеру тома, а
// не по букве. Том читается каждый раз, без кэша (смена карты в слоте — секунды); кэш есть только у
// показа (internal/app/agentplaces_live.go). Том место получает только от действия человека
// (pushPlaces, ChooseAgentPlace, BindAgentPlace); засев, подъём и тихие проверки его не пишут.

import (
	"errors"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Коды причин, которые знает сверка с местами (в app — whyVol, whyDisk; тексты — во фронте).
const (
	WhyDisk = "disk" // внутри места, но на другом диске, не на том, что у места (153)
	WhyVol  = "vol"  // диск места узнать не удалось (153)
)

// ErrVolUnknown — номер диска места узнать не удалось (ошибка, таймаут, у тома нет номера, macOS).
var ErrVolUnknown = errors.New("не удалось узнать диск")

// VolFn — чтение тома: подмена Env.Vol или настоящее.
func (e *Env) VolFn() func(string) (osx.Vol, error) {
	if e != nil && e.Vol != nil {
		return e.Vol
	}
	return osx.VolumeOf
}

// VolPathFn — поиск тома по номеру: подмена Env.VolPath или настоящий.
func (e *Env) VolPathFn() func(id, rel string) (string, string, error) {
	if e != nil && e.VolPath != nil {
		return e.VolPath
	}
	return osx.VolumePath
}

// VolOfDir — диск настоящего пути dir под предельным временем; nil — узнать не удалось (место
// остаётся по пути, как в 0.34).
func (e *Env) VolOfDir(dir string) *prefs.PlaceVol {
	if !e.ok() {
		return nil
	}
	fn := e.VolFn() // до Within: зависшая проба переживёт предел, подмену позже читать нельзя
	type res struct {
		v   osx.Vol
		err error
	}
	got, ok := appkit.Within(e.Rules.timeout, func() res {
		v, err := fn(dir)
		return res{v, err}
	})
	if !ok || got.err != nil {
		return nil
	}
	return &prefs.PlaceVol{ID: got.v.ID, Label: got.v.Label, Rel: got.v.Rel}
}

// BindInside — места без привязки в cur, внутри которых (вместе с которыми) или снаружи которых лежит
// одно из add с диском: данные диска, если место и правда на этом диске (проба тома по его пути; не
// удалась — место не трогаем). Ключ — prefs.PlaceKey. Иначе новое место с диском осталось бы рядом
// со старым без него (разные виды «внутри» не сравниваются): место 0.34 не получило бы диск, а
// поддерево внутри нового принимало бы чужой диск под той же буквой.
func (e *Env) BindInside(cur, add []prefs.Place) map[string]prefs.PlaceVol {
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
			if v := e.VolOfDir(u.Dir); v != nil && v.ID == n.Vol.ID {
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
func (r *Rules) relBelow(rel, base string) bool {
	rel, base = r.fold(rel), r.fold(base)
	if base == "" {
		return rel != ""
	}
	return strings.HasPrefix(rel, base+"/")
}

// ScanRes — итог сверки с местами: OK — внутри; иначе причина (WhyDisk, WhyVol) и строка для вопроса.
type ScanRes struct {
	OK        bool
	Why, Note string
}

// volPlaceSelf — место с диском, каким оно лежит сейчас (resolved без хвоста из depth элементов),
// проходит строковые правила своего способа и разрешается в себя же.
func volPlaceSelf(env *Env, disk func(string) (string, error), resolved string, pl prefs.Place, depth int) bool {
	r := env.Rules
	cur := resolved
	for range depth {
		cur = r.Parent(cur)
	}
	if env.Static(cur, pl.How) != nil {
		return false
	}
	self, err := disk(cur)
	return err == nil && r.Key(self) == r.Key(cur)
}

// diskNote — строка вопроса «другой диск»: что за место и на каком диске оно было.
func diskNote(pl prefs.Place) string {
	if pl.Vol.Label == "" {
		return "Место «" + pl.Dir + "» — на другом диске"
	}
	return "Место «" + pl.Dir + "» — на диске «" + pl.Vol.Label + "»"
}

// Scan — сверка пути со всеми местами (под Within): v, verr — том пути, прочитанный один раз.
func Scan(env *Env, disk func(string) (string, error), resolved string, places []prefs.Place, v osx.Vol, verr error) ScanRes {
	r := env.Rules
	var out ScanRes
	for _, pl := range places {
		strictly := r.Inside(resolved, pl.Dir) && r.Key(resolved) != r.Key(pl.Dir)
		switch {
		case pl.Vol == nil:
			if !strictly {
				continue
			}
			if self, err := disk(pl.Dir); err == nil && r.Key(self) == r.Key(pl.Dir) {
				return ScanRes{OK: true}
			}
		case verr == nil && v.ID == pl.Vol.ID:
			if r.relBelow(v.Rel, pl.Vol.Rel) && volPlaceSelf(env, disk, resolved, pl, volSegs(v.Rel)-volSegs(pl.Vol.Rel)) {
				return ScanRes{OK: true}
			}
		case out.Why == "" && strictly:
			// Не тот диск или диск не узнан: причину подсказывает путь по строке.
			if verr != nil {
				out.Why = WhyVol
			} else {
				out.Why, out.Note = WhyDisk, diskNote(pl)
			}
		}
	}
	return out
}
