package names

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Сопоставление по столбцу «Файл» (design 043 §3.3): строка ищет фото листа по имени файла.

// maxNumDigits — ячейка из одних цифр не длиннее — номер кадра (Excel делает «0012» числом 12).
const maxNumDigits = 9

// fileKey — ячейка или путь для сравнения: последнее звено пути («\» и «/»), без краевых
// пробелов, нижний регистр, «ё» = «е».
func fileKey(s string) string {
	s = strings.TrimSpace(s)
	s = s[strings.LastIndexAny(s, `\/`)+1:]
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "ё", "е")
}

// lastNumber — последняя группа цифр имени («img_0012» → 12); ok = false — цифр нет или их много.
func lastNumber(s string) (int, bool) {
	j := len(s)
	for j > 0 && (s[j-1] < '0' || s[j-1] > '9') {
		j--
	}
	i := j
	for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
		i--
	}
	return number(s[i:j])
}

// number — строка из одних цифр (1–9) как число.
func number(s string) (int, bool) {
	if s == "" || len(s) > maxNumDigits || strings.Trim(s, "0123456789") != "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// byFile — каждая строка people ищет своё место среди rows (с именем файла и без расширения;
// число — по последней группе цифр, только если фото одно). Нет пары, несколько фото или фото
// уже занято — строка уходит в left; место без пары имени не меняет.
func byFile(p *model.Project, rows []model.NamesRow, people []Entry, left func(Entry, string)) {
	exact := map[string][]int{}
	nums := map[int][]int{}
	for i, row := range rows {
		ref, ok := p.Photos[row.Photo]
		if !ok || row.No == 0 {
			continue
		}
		full := fileKey(ref.Path)
		bare := strings.TrimSuffix(full, filepath.Ext(full))
		exact[full] = append(exact[full], i)
		if bare != full {
			exact[bare] = append(exact[bare], i)
		}
		if n, ok := lastNumber(bare); ok {
			nums[n] = append(nums[n], i)
		}
	}
	for _, e := range people {
		key := fileKey(e.File)
		cand := exact[key]
		if n, ok := number(key); ok && len(cand) == 0 {
			cand = nums[n]
		}
		switch {
		case key == "" || len(cand) == 0:
			left(e, leftNoFile)
		case len(cand) > 1:
			left(e, leftMany)
		case rows[cand[0]].Who != nil:
			left(e, leftDup)
		default:
			put(&rows[cand[0]], e)
		}
	}
}
