package salbum

import (
	"bytes"
	"encoding/json"
	"io"
	"log"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// refList — поле shareRefs файла (167, model/sharerefs.go): привязки к выгруженным альбомам. Чужие
// данные: поле не списком или длиннее model.RawShareRefs — привязок нет, файл открывается; в списке
// остаются только годные (32 знака hex), не больше model.MaxShareRefs. Массив разбирается по
// одному элементу и останавливается на 65-м: файл с миллионом строк не держит ни время, ни память.
type refList []string

// maxRefRaw — длина записи привязки в файле: 32 знака и кавычки; длиннее — не привязка, элемент
// не разбирается.
const maxRefRaw = 2*16 + 2

func (l *refList) UnmarshalJSON(b []byte) error {
	*l = nil
	if string(bytes.TrimSpace(b)) == "null" {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if t, err := dec.Token(); err != nil || t != json.Delim('[') {
		log.Printf("привязки к ссылкам: поле shareRefs не список — не прочитаны")
		return nil
	}
	var out []string
	for n := 0; dec.More(); n++ {
		if n >= model.RawShareRefs {
			log.Printf("привязки к ссылкам: в shareRefs больше %d записей — не прочитаны", model.RawShareRefs)
			return nil
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if err != io.EOF {
				log.Printf("привязки к ссылкам: shareRefs повреждено — не прочитаны")
			}
			return nil
		}
		var s string
		if len(raw) <= maxRefRaw && json.Unmarshal(raw, &s) == nil {
			out = append(out, s)
		}
	}
	*l = model.CleanLinkRefs(out)
	return nil
}
