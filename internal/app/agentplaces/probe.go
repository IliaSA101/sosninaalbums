package agentplaces

// Пробы мест по диску для показа (0.35/153, design §4.5): кэш и AgentPlace — в internal/app
// (agentplaces_live.go), здесь только то, что узнаётся от диска. Вызывает App под Within.

import (
	"errors"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Состояния места после пробы (фронт видит те же строки).
const (
	StateNoDisk   = "nodisk"   // диск места сейчас не подключён
	StateNoFolder = "nofolder" // диск есть, папки на нём нет
	StateHere     = "here"     // место без привязки, а по его пути сейчас диск: можно привязать
)

// ProbeRes — что узнали о месте: State пуст — всё как в config.json; HasPath — привязанное место
// нашлось на диске, Dir и Label (может быть пустой) — его текущие путь и метка; Here — метка диска
// по пути непривязанного места (State = StateHere).
type ProbeRes struct {
	State   string
	HasPath bool
	Dir     string
	Label   string
	Here    string
}

// Probe — состояние места p по диску (вызов под Within). Привязанное: текущий путь по номеру тома —
// нет тома (ErrVolGone) → StateNoDisk; не смогли узнать (нет номера, macOS) → без состояния.
// Непривязанное: папка по сохранённому пути, диск по ней — для ссылки «Привязать».
func Probe(p prefs.Place, volOf func(string) (osx.Vol, error), volPath func(id, rel string) (string, string, error)) ProbeRes {
	var res ProbeRes
	if p.Vol != nil {
		path, label, err := volPath(p.Vol.ID, p.Vol.Rel)
		switch {
		case errors.Is(err, osx.ErrVolGone):
			res.State = StateNoDisk
		case err == nil:
			res.HasPath, res.Dir, res.Label = true, path, label
			if !statDir(path) {
				res.State = StateNoFolder
			}
		}
		return res
	}
	if !statDir(p.Dir) {
		res.State = StateNoFolder
		return res
	}
	if v, err := volOf(p.Dir); err == nil {
		res.State, res.Here = StateHere, v.Label
	}
	return res
}

// statDir — локальная папка есть (CheckLocalPath, затем StatLocal); предел держит вызывающий.
func statDir(p string) bool {
	if paths.CheckLocalPath(p) != nil {
		return false
	}
	st, err := library.StatLocal(p)
	return err == nil && st.IsDir()
}
