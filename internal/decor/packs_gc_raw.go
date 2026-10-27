package decor

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Задел совместимости с 0.28 (спека 131, design §16 «Откат на 0.27», решение 0.27-131-30): сборка
// мусора 0.27 не знает записей незнакомого вида (макет альбома `album`, файл Макеты/<16 hex>.json) и
// полей новее себя (borders[].lib шаблона v1). Поэтому ссылки на картинки читаются из сырых байт:
// всё, что похоже на ключ библиотеки my:<16 hex>, держит картинку — как бы ни был устроен JSON.

// Пределы чтения незнакомого: до чтения, а не после (П11). Превышение или сбой чтения — сборка мусора
// ничего не удаляет (rawRefs.complete = false): лучше оставить лишнее, чем стереть нужное.
const (
	rawFileMax = 1 << 20 // файл незнакомого вида; MaxLookFile 0.28 — 512 КБ
	rawFiles   = 4096    // файлов во всех незнакомых папках
)

// rawTotalMax — суммарно байт; var, чтобы тест сузил предел, не создавая десятков мегабайт.
var rawTotalMax int64 = 64 << 20

// rawLibKeys — ключи my:<16 hex> в сырых байтах (повторы убраны). Хвост из hex длиннее 16 — лишнее
// не отбрасывается: держим по первым 16, это надмножество и безопаснее.
func rawLibKeys(data []byte) []string {
	var out []string
	seen := map[string]bool{} // повторы — через карту: 1 МБ разных ключей иначе квадратичен
	for i := 0; ; {
		j := bytes.Index(data[i:], []byte(libPrefix))
		if j < 0 {
			return out
		}
		i += j + len(libPrefix)
		if i+16 > len(data) {
			return out
		}
		if !isLowerHex(data[i : i+16]) {
			continue
		}
		if k := libPrefix + string(data[i:i+16]); !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
}

func isLowerHex(b []byte) bool {
	for _, c := range b {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// rawRefs — накопитель ссылок из незнакомого; complete = false — что-то не удалось прочитать или
// превышен предел.
type rawRefs struct {
	keys     map[string]bool
	complete bool
	files    int
	total    int64
}

func (r *rawRefs) add(data []byte) {
	for _, k := range rawLibKeys(data) {
		r.keys[k] = true
	}
}

// read читает файл в пределах rawFileMax и общего бюджета; false — не прочитан (complete = false).
func (r *rawRefs) read(path string, limit int64) ([]byte, bool) {
	r.files++
	if r.files > rawFiles {
		r.complete = false
		return nil, false
	}
	data, err := readCapped(path, limit)
	r.total += int64(len(data))
	if err != nil || r.total > rawTotalMax {
		if !errors.Is(err, os.ErrNotExist) { // стёрт между ReadDir и чтением — не ошибка (данных нет, бюджет цел)
			r.complete = false
		}
		return nil, false
	}
	return data, true
}

// unknownRefs — ссылки записей незнакомого вида: файлы в незнакомых папках корня библиотеки (Макеты/),
// записи незнакомого вида в library.json и в реестрах наборов, кроме except. Свои папки видов и
// Наборы/ пропускаются: в реестрах набора лежат ключи самих картинок, они ничего не держат.
func (s *Store) unknownRefs(except *packReg) *rawRefs {
	r := &rawRefs{keys: map[string]bool{}, complete: true}
	list, err := os.ReadDir(s.root)
	if err != nil {
		r.complete = errors.Is(err, os.ErrNotExist) // папки нет — библиотека пуста, держать нечего
		return r
	}
	known := map[string]bool{packsDir: true}
	for _, d := range kindDirs {
		known[d] = true
	}
	for _, de := range list {
		if known[de.Name()] {
			continue
		}
		if de.Type()&os.ModeSymlink != 0 { // ссылка (junction) на незнакомое: не читаем, держим всё
			r.complete = false
			continue
		}
		if !de.IsDir() {
			continue
		}
		dir := filepath.Join(s.root, de.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			r.complete = false
			continue
		}
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			if f.Type()&os.ModeSymlink != 0 { // .json-ссылка: цель не читаем — держим всё
				r.complete = false
				continue
			}
			if !f.Type().IsRegular() {
				continue
			}
			if data, ok := r.read(filepath.Join(dir, f.Name()), rawFileMax); ok {
				r.add(data)
			}
		}
	}
	if data, ok := r.read(filepath.Join(s.root, regFile), maxRegSize); ok {
		r.addUnknownItems(data)
	}
	s.packRegsUnknown(r, except)
	return r
}

// packRegsUnknown — записи незнакомого вида в реестрах наборов, кроме except. Папку Наборы/ не
// перечислить (не «нет папки») или .json-ссылка в ней — не прочитано, complete = false.
func (s *Store) packRegsUnknown(r *rawRefs, except *packReg) {
	packs, err := os.ReadDir(filepath.Join(s.root, packsDir))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			r.complete = false
		}
		return
	}
	for _, f := range packs {
		id, ok := strings.CutSuffix(f.Name(), ".json")
		if !ok || except != nil && id == except.ID {
			continue
		}
		if f.Type()&os.ModeSymlink != 0 {
			r.complete = false
			continue
		}
		if !f.Type().IsRegular() {
			continue
		}
		if data, ok := r.read(filepath.Join(s.root, packsDir, f.Name()), packRegMax); ok {
			r.addUnknownItems(data)
		}
	}
}

// addUnknownItems — реестр ({"items":[…]}): из записей вида, которого нет в kindDirs (или вид не
// строка), берутся все ключи как есть. Реестр, который не разбирается, ничего не держит: 0.27 его
// и не загрузила бы (library.json уходит в .bad, набор пропускается).
func (r *rawRefs) addUnknownItems(data []byte) {
	var reg struct {
		Items []json.RawMessage `json:"items"`
	}
	if json.Unmarshal(data, &reg) != nil {
		return
	}
	for _, raw := range reg.Items {
		var head struct {
			Kind string `json:"kind"`
		}
		if json.Unmarshal(raw, &head) == nil && kindDirs[head.Kind] != "" {
			continue // известный вид: его ссылки уже знает tplRefs
		}
		r.add(raw)
	}
}
