package update

import (
	"bytes"
	"encoding/json"
)

// Правила минимальной системы в манифесте (спека 171, design §2): minOS — «GOOS → версия
// системы», lastOn — лестница «GOOS → {порог → последняя версия программы для систем ниже порога}».
const (
	// maxKnownKeys — известных ключей ОС в одном поле; неизвестные не считаются и не разбираются
	// (171-14): их добавит будущий выпуск, а прочитать их 0.4x должны без отказа.
	maxKnownKeys = 8
	maxSteps     = 8
)

// knownOS — ключи ОС, которые программа понимает. Регистр строгий: «Darwin» — неизвестный ключ.
var knownOS = []string{"darwin", "windows", "linux"}

// rules — разобранные minOS и lastOn; skipped — имена пропущенных полей другой ОС.
type rules struct {
	minOS   map[string]string
	lastOn  map[string]map[string]string
	skipped []string
}

// parseRules — разбор полей minOS и lastOn (version — версия выпуска, проверена). Ошибка в ключе
// своей ОС (goos; "" — всех, фикстура и FuzzParse) — ErrManifest; в ключе другой известной ОС —
// ключ пропущен, имя в skipped (171-13): неверное поле одной ОС не отрезает от обновлений другие.
// Поле не объект и не null — ErrManifest для всех: оно общее.
func parseRules(minRaw, lastRaw json.RawMessage, version, goos string, known []string) (rules, error) {
	var r rules
	minMap, err := objectOf(minRaw, known)
	if err != nil {
		return r, err
	}
	lastMap, err := objectOf(lastRaw, known)
	if err != nil {
		return r, err
	}
	// bad — ошибка поля: своей ОС (или любой при goos == "") — отказ, чужой — пропуск.
	bad := func(key, field string) error {
		if goos == "" || key == goos {
			return ErrManifest
		}
		r.skipped = append(r.skipped, field+"."+key)
		return nil
	}
	for _, k := range known {
		raw, ok := minMap[k]
		if !ok {
			continue
		}
		var s string
		if json.Unmarshal(raw, &s) != nil || !validOS(s) {
			if err := bad(k, "minOS"); err != nil {
				return r, err
			}
			continue
		}
		if r.minOS == nil {
			r.minOS = map[string]string{}
		}
		r.minOS[k] = s
	}
	for _, k := range known {
		raw, ok := lastMap[k]
		if !ok {
			continue
		}
		steps, ok := ladder(raw, r.minOS[k], version)
		if !ok {
			if err := bad(k, "lastOn"); err != nil {
				return r, err
			}
			continue
		}
		if r.lastOn == nil {
			r.lastOn = map[string]map[string]string{}
		}
		r.lastOn[k] = steps
	}
	return r, nil
}

func validOS(s string) bool { _, ok := ParseOS(s); return ok }

// objectOf — поле как объект «ключ → сырое значение»; null и отсутствие — nil. Известных ключей
// больше maxKnownKeys — ErrManifest.
func objectOf(raw json.RawMessage, known []string) (map[string]json.RawMessage, error) {
	if s := bytes.TrimSpace(raw); len(s) == 0 || string(s) == "null" {
		return nil, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || m == nil {
		return nil, ErrManifest
	}
	n := 0
	for _, k := range known {
		if _, ok := m[k]; ok {
			n++
		}
	}
	if n > maxKnownKeys {
		return nil, ErrManifest
	}
	return m, nil
}

// ladder — лестница одной ОС. need — minOS этой ОС (без неё лестницы нет). Правила: ≤ maxSteps
// ступеней; ключ — версия системы не выше need; значение — версия программы строго старше
// version; пороги не повторяются; чем выше порог, тем не старше версия.
func ladder(raw json.RawMessage, need, version string) (map[string]string, bool) {
	needV, ok := ParseOS(need)
	if !ok {
		return nil, false
	}
	var steps map[string]string
	if json.Unmarshal(raw, &steps) != nil || steps == nil || len(steps) > maxSteps {
		return nil, false
	}
	for key, val := range steps {
		kv, ok := ParseOS(key)
		if !ok || cmpOS(kv, needV) > 0 || !Newer(version, val) {
			return nil, false
		}
	}
	keys := stepsSorted(steps)
	for i := 1; i < len(keys); i++ {
		a, _ := ParseOS(keys[i-1])
		b, _ := ParseOS(keys[i])
		if cmpOS(a, b) == 0 || Newer(steps[keys[i-1]], steps[keys[i]]) {
			return nil, false
		}
	}
	return steps, true
}
