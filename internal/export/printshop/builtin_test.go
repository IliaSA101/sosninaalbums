package printshop

import "github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"

// builtinProfile — выгрузка встроенного профиля id (без слоёв): тесты называют типографию прямо.
// В рабочем коде типографию берёт ProfileOf по id из действующего снимка (слои 123 Б1).
func builtinProfile(id string) Profile {
	p, ok := printprof.Builtin().ByID(id)
	if !ok || p.Export == nil {
		panic("printshop: встроенного профиля выгрузки нет: " + id)
	}
	return fromProfile(p, true)
}

// Fsfera — Фотосфера Pro (047): тексты прежние, байт в байт.
var Fsfera = builtinProfile("fsfera")

// Fabrika — Фабрика Фотокниги (081): имена 1-NN.jpg для их загрузчика, качество 100.
var Fabrika = builtinProfile("fabrika")
