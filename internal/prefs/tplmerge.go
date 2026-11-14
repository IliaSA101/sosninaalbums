package prefs

import (
	"encoding/json"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// MergeResult — итог загрузки шаблонов из файла (086): добавлено, уже были, не вошли по пределу.
type MergeResult struct{ Added, Same, Over int }

// MergeTemplates — cur + add: шаблон с тем же содержимым (без ID и имени) пропускается (Same),
// совпавший только ID получает newID(), сверх MaxTemplates — Over. Порядок add сохраняется;
// повтор внутри add — тоже Same: повторная загрузка того же файла не удваивает список. cur не меняется.
func MergeTemplates(cur, add []model.Template, newID func() string) ([]model.Template, MergeResult) {
	var res MergeResult
	out := slices.Clone(cur)
	seen := make(map[string]bool, len(cur)+len(add))
	ids := make(map[string]bool, len(cur)+len(add))
	for _, t := range cur {
		seen[tplKey(t)], ids[t.ID] = true, true
	}
	for _, t := range add {
		k := tplKey(t)
		if k != "" && seen[k] {
			res.Same++
			continue
		}
		if len(out) >= model.MaxTemplates {
			res.Over++
			continue
		}
		for t.ID == "" || ids[t.ID] {
			t.ID = newID()
		}
		if k != "" {
			seen[k] = true
		}
		ids[t.ID] = true
		out = append(out, t)
		res.Added++
	}
	return out, res
}

// tplKey — содержимое шаблона без ID и имени в том виде, как его пишет файл: шаблон из памяти
// (округлён при записи) и тот же из чужого файла (только зажат) дают один ключ. "" — не
// сравнивается ни с чем (не маршалится: после ClampTemplates NaN не бывает).
func tplKey(t model.Template) string {
	t.ID, t.Name = "", ""
	b, err := json.Marshal(model.RoundTemplates([]model.Template{t})[0])
	if err != nil {
		return ""
	}
	return string(b)
}
