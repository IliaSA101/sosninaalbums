package prefs

// Отметки «Изучил» справочника F1 (спека 140): один список id статей в config.json, общий для
// всех альбомов. Go статей не знает: id — данные интерфейса, здесь только грамматика и предел.

import "slices"

// maxLearned — предел списка: статей ~130, остальное — запас на рост и чужие id новой версии.
const maxLearned = 500

// maxLearnedID — длина id статьи (грамматика [a-z0-9][a-z0-9-]{0,39}).
const maxLearnedID = 40

// validLearnedID — годный id статьи: строчные латинские буквы, цифры и дефис, не с дефиса.
func validLearnedID(id string) bool {
	if id == "" || len(id) > maxLearnedID {
		return false
	}
	for i := range len(id) {
		c := id[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && (c != '-' || i == 0) {
			return false
		}
	}
	return true
}

// CleanLearned оставляет годные id без повторов, не больше maxLearned; порядок прежний.
// Результат — новый срез, не nil (файл правили руками или писала другая версия).
func CleanLearned(list []string) []string {
	out := make([]string, 0, min(len(list), maxLearned))
	for _, id := range list {
		if len(out) == maxLearned {
			break
		}
		if validLearnedID(id) && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// MarkLearned — копия списка с отметкой ids: on дописывает недостающие в конец, off убирает.
// Негодные id пропускаются; сверх maxLearned новый id не дописывается. Результат — не nil.
func MarkLearned(list, ids []string, on bool) []string {
	out := CleanLearned(list)
	if !on {
		return slices.DeleteFunc(out, func(v string) bool { return slices.Contains(ids, v) })
	}
	for _, id := range ids {
		if len(out) == maxLearned {
			break
		}
		if validLearnedID(id) && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

// GuideLearned — отмеченные id после чистки; никогда не nil.
func GuideLearned(dir string) []string { return CleanLearned(LoadConfig(dir).GuideLearned) }

// MarkGuide ставит (on) или снимает отметку с ids одной записью и возвращает список из файла.
// Ничего не изменилось (мусор, повтор, нет такого) — файл не пишется. Ответ читается заново
// после записи: сбой записи (папка только для чтения, диск полон) не показывает отметку,
// которой в файле нет, — иначе после перезапуска она пропала бы.
func MarkGuide(dir string, ids []string, on bool) []string {
	cur := GuideLearned(dir)
	if slices.Equal(MarkLearned(cur, ids, on), cur) {
		return cur
	}
	UpdateConfig(dir, func(c *Config) {
		c.GuideLearned = MarkLearned(c.GuideLearned, ids, on)
	})
	return GuideLearned(dir)
}
