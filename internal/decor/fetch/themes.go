package fetch

import "encoding/json"

// MaxThemes — предел объявлений тем в наборе (= pack.MaxThemes: fetch не импортирует pack).
const MaxThemes = 16

// themesCount — сколько объявлений тем несёт поле themes строки каталога (170): клиенту нужно
// только число — имена и базы он из строки не берёт, темы задаёт манифест. Не массив или нет
// поля — 0; больше предела — зажим. Негодное поле набор не роняет (поле — RawMessage, как preview).
func themesCount(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return 0
	}
	return min(len(list), MaxThemes)
}
