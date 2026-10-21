package salbum

import (
	"cmp"
	"encoding/json"
	"log"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// fileMark — своя отметка в файле: путь как у фото (относительный + запасной abs).
type fileMark struct {
	Path   string  `json:"path"`
	Abs    string  `json:"abs,omitempty"`
	Rating *int    `json:"rating,omitempty"`
	Label  *string `json:"label,omitempty"`
	Flag   *int    `json:"flag,omitempty"`
	Bins   binIDs  `json:"bins,omitempty"` // ID подборок (v15, 083)
}

// binIDs — членство отметки в файле. Вторично, как сама отметка: не список — подборок у записи
// нет, не строка — элемент пропускается; оценка и метка записи при этом целы. Своя программа
// пишет не больше MaxBins ID — хвост длиннее 2·MaxBins не разбирается.
type binIDs []string

func (l *binIDs) UnmarshalJSON(b []byte) error {
	*l = nil
	var raw []json.RawMessage
	if json.Unmarshal(b, &raw) != nil {
		return nil
	}
	for _, r := range raw[:min(len(raw), 2*model.MaxBins)] {
		var id string
		if json.Unmarshal(r, &id) == nil {
			*l = append(*l, id)
		}
	}
	return nil
}

// encodeMarks — список для файла, по пути в нижнем регистре: стабильный дифф. У записи — только
// ID из known (подборки файла): членство без подборки в файл не уходит; запись, где после этого
// ничего не осталось, не пишется.
func encodeMarks(marks map[string]model.Mark, known map[string]bool, enc pathEnc) []fileMark {
	if len(marks) == 0 {
		return nil
	}
	list := slices.Collect(maps.Values(marks))
	slices.SortFunc(list, func(a, b model.Mark) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.Path), strings.ToLower(b.Path)), strings.Compare(a.Path, b.Path))
	})
	out := make([]fileMark, 0, len(list))
	for _, m := range list {
		m = m.WithBins(slices.DeleteFunc(slices.Clone(m.Bins), func(id string) bool { return !known[id] }))
		if m.None() {
			continue
		}
		fm := fileMark{Rating: m.Rating, Label: m.Label, Flag: m.Flag, Bins: m.Bins}
		fm.Path, fm.Abs = enc(m.Path)
		if m.Alt() != "" {
			fm.Abs = m.Alt()
		}
		out = append(out, fm)
	}
	return out
}

// markList — список marks из файла. Отметки вторичны: запись неверного типа становится пустой
// (decodeMarks считает её битой), а marks не списком — нет отметок; файл открывается.
type markList []fileMark

func (l *markList) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		log.Printf("свои отметки: поле marks не список — отметки не прочитаны")
		*l = nil
		return nil
	}
	out := make(markList, len(raw))
	for i, r := range raw {
		if json.Unmarshal(r, &out[i]) != nil {
			out[i] = fileMark{} // разбор мог заполнить часть полей
		}
	}
	*l = out
	return nil
}

// binList — список bins из файла (v15, 083), как markList: поле не списком — подборок нет,
// запись неверного вида — пустая (ClampBins отбросит её по ID); файл открывается. Своя
// программа пишет не больше MaxBins — хвост длиннее 2·MaxBins чужого файла не разбирается.
type binList []model.Bin

func (l *binList) UnmarshalJSON(b []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		log.Printf("подборки: поле bins не список — подборки не прочитаны")
		*l = nil
		return nil
	}
	raw = raw[:min(len(raw), 2*model.MaxBins)]
	out := make(binList, len(raw))
	for i, r := range raw {
		if json.Unmarshal(r, &out[i]) != nil {
			out[i] = model.Bin{}
		}
	}
	*l = out
	return nil
}

// decodeBins — подборки файла после зажима; поправки — одной строкой без имён (имена — данные
// пользователя, в log.txt не пишутся).
func decodeBins(l binList) []model.Bin {
	bins, fixed := model.ClampBins(l)
	if fixed > 0 {
		log.Printf("подборки: отброшено или поправлено записей — %d", fixed)
	}
	return bins
}

// maxMarkDirs — сколько разных папок decodeMarks проверяет на диске; дальше путь остаётся
// основным с запасным (данные целы), а файл с сотней тысяч папок не держит открытие.
const maxMarkDirs = 1000

// decodeMarks — отметки из файла. Отметки вторичны: битая запись отбрасывается, а не весь файл.
// Наличие файлов не проверяется (диск не подключён — автосохранение стёрло бы отбор);
// путь выбирается по наличию папки, а не файла: stat на папку, не на 5000 фото.
// Подборки записи — только из known (подборки файла после ClampBins): чужие ID отбрасываются,
// запись, где ничего не осталось, считается пустой.
func decodeMarks(list []fileMark, base string, folders []string, known map[string]bool) map[string]model.Mark {
	out := map[string]model.Mark{}
	var bad, dup, empty int
	// Своя программа больше maxMarks не пишет: длиннее — чужой файл, хвост не разбираем.
	if len(list) > 2*model.MarksLimit() {
		bad, list = len(list)-2*model.MarksLimit(), list[:2*model.MarksLimit()]
	}
	dirs := map[string]bool{}
	hasDir := func(path string) bool {
		d := filepath.Dir(path)
		ok, seen := dirs[d]
		if !seen && len(dirs) < maxMarkDirs {
			ok = isDir(d)
			dirs[d] = ok
		}
		return ok
	}
	type entry struct {
		id     string
		m      model.Mark
		viaAlt bool // путь выбран запасной
	}
	all := make([]entry, 0, len(list))
	for _, fm := range list {
		path, alt, err := candidates(fm.Path, fm.Abs, base, false)
		if err != nil {
			bad++
			continue
		}
		path = filepath.Clean(path) // как IDFor: иначе запись и чтение давали бы разные пути
		if alt != "" {
			alt = filepath.Clean(alt)
		}
		viaAlt := false
		if alt != "" && !hasDir(path) && hasDir(alt) {
			path, alt, viaAlt = alt, "", true
		} else if alt != "" && hasDir(path) {
			alt = ""
		}
		m := model.Mark{Path: path, Rating: model.KeepIf(fm.Rating, model.OkRating), Label: model.KeepIf(fm.Label, model.OkLabel), Flag: model.KeepIf(fm.Flag, model.OkFlag)}.WithAlt(alt).WithBins(knownBins(fm.Bins, known))
		if m.None() {
			empty++
			continue
		}
		all = append(all, entry{library.IDFor(path), m, viaAlt})
	}
	// Повтор ID: остаётся запись, найденная по основному пути, — запасной путь старше
	// (альбом переехал, а файл на новом месте отмечен заново). Среди равных — первая в файле.
	kept := make([]int, 0, len(all))
	for _, alt := range []bool{false, true} {
		for i, en := range all {
			if en.viaAlt != alt {
				continue
			}
			if _, ok := out[en.id]; ok {
				dup++
				continue
			}
			out[en.id] = en.m
			kept = append(kept, i)
		}
	}
	slices.Sort(kept)
	order := make([]string, len(kept)) // ID в порядке файла: обрезка по пределу детерминирована
	for k, i := range kept {
		order[k] = all[i].id
	}
	if bad+dup+empty > 0 {
		log.Printf("свои отметки: отброшено записей — с битым путём %d, повторов %d, пустых %d", bad, dup, empty)
	}
	if over := len(out) - model.MarksLimit(); over > 0 {
		TrimMarks(out, order, folders)
		log.Printf("свои отметки: больше %d, лишние %d отброшены", model.MarksLimit(), over)
	}
	return out
}

// knownBins — ID из ids, которые есть в known; сверх MaxBins разных — чужой файл, хвост не нужен.
func knownBins(ids []string, known map[string]bool) []string {
	var out []string
	for _, id := range ids {
		if known[id] && !slices.Contains(out, id) {
			out = append(out, id)
		}
		if len(out) == model.MaxBins {
			break
		}
	}
	return out
}

// TrimMarks оставляет maxMarks отметок: сначала уходят отметки вне папок проекта, затем
// последние в файле (order — ID в порядке файла).
func TrimMarks(out map[string]model.Mark, order []string, folders []string) {
	under := UnderAny(folders)
	in, out2 := make([]string, 0, len(order)), make([]string, 0, len(order))
	for _, id := range order {
		if under(out[id].Path) {
			in = append(in, id)
		} else {
			out2 = append(out2, id)
		}
	}
	for _, id := range append(in, out2...)[model.MarksLimit():] {
		delete(out, id)
	}
}

// UnderAny — library.Under по любой из папок за O(log папок): Under по каждой папке на каждую
// отметку открывал чужой файл минутами. Вложенные папки отбрасываются — в оставшемся наборе
// ни один ключ не префикс другого, и подходить может только ближайший снизу.
func UnderAny(folders []string) func(path string) bool {
	sep := string(filepath.Separator)
	keys := make([]string, 0, len(folders))
	for _, f := range folders {
		k := strings.ToLower(filepath.Clean(f))
		if !strings.HasSuffix(k, sep) {
			k += sep
		}
		keys = append(keys, k)
	}
	slices.Sort(keys)
	top := keys[:0]
	for _, k := range keys {
		if len(top) == 0 || !strings.HasPrefix(k, top[len(top)-1]) {
			top = append(top, k)
		}
	}
	return func(path string) bool {
		p := strings.ToLower(filepath.Clean(path))
		i, found := slices.BinarySearch(top, p)
		return found || i > 0 && strings.HasPrefix(p, top[i-1])
	}
}
