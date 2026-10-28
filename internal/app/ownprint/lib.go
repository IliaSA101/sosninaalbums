package ownprint

import (
	"log"
	"reflect"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Типографии из библиотеки (123 Б4, design §4): профили .saprint установленных наборов —
// слой реестра между встроенными и своими. Ставит их только набор из подписанного каталога
// (decor.stagePrint); здесь они читаются заново при каждой сборке реестра, поэтому удаление
// набора откатывает типографию к встроенной, а правила версий держит printprof.Compose.

// CheckData — проверка профиля набора при установке: тот же строгий разбор, что у файла
// (decor.Store.SetPrintCheck).
func CheckData(data []byte) error {
	_, err := printprof.Parse(data)
	return err
}

// libraryPrints — профили наборов для Compose. Не прошедшие разбор, набор которых требует
// программу новее этой (понизили версию) или чьё название занято своей типографией — пропускаются
// со строкой в log.txt: своя уже есть у фотографа и не должна пропасть из-за чужого набора.
// Название сравнивается по скелету (printprof.NameKey): двойник с латинской «а» — тоже занято.
func libraryPrints(s *decor.Store, own []printprof.Profile, app string) []printprof.Profile {
	var out []printprof.Profile
	for _, pp := range s.PackPrints() {
		if pp.MinApp != "" && (fetch.Entry{MinApp: pp.MinApp}).NeedsApp(app) {
			log.Printf("типография набора %s пропущена: нужна версия Sosnina Albums %s или новее", pp.Pack, pp.MinApp)
			continue
		}
		p, err := printprof.Parse(pp.Data)
		if err != nil {
			log.Printf("типография набора %s пропущена: %v", pp.Pack, paths.NoPath(err))
			continue
		}
		if slices.ContainsFunc(own, func(o printprof.Profile) bool { return printprof.SameName(o.Name, p.Name) }) {
			log.Printf("типография %q набора %s пропущена: название уже занято своей", p.ID, pp.Pack)
			continue
		}
		out = append(out, p)
	}
	return out
}

// LibraryChanged — после установки или удаления набора: реестр собирается заново, и если
// список типографий изменился, уходит `print:changed` и вид (пределы и запреты от нового реестра).
func (s *Service) LibraryChanged() {
	before := printprof.Current().Infos()
	s.Reload()
	if !reflect.DeepEqual(before, printprof.Current().Infos()) {
		s.changed()
	}
}
