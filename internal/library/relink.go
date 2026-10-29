package library

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// MaxRelinkFiles — сколько записей обхода допускает поиск пропавших фото.
const MaxRelinkFiles = 200_000

// ErrTooManyFiles — папка для поиска слишком велика.
var ErrTooManyFiles = errors.New("слишком много файлов, укажите папку ближе к фото")

// Want — пропавшее фото, которое ищем.
type Want struct {
	ID   string
	Old  string // прежний путь: из него берутся имя и хвост папок
	Size int64  // байты файла; 0 — неизвестны, тогда сверяются W×H из заголовка
	W, H int
}

// Found — итог поиска.
type Found struct {
	Paths     map[string]string // ID → найденный путь
	Ambiguous []string          // ID, у которых несколько равноценных кандидатов
}

// SplitPath режет путь на элементы по любому из разделителей: старый путь мог
// быть записан и на другой ОС.
func SplitPath(p string) []string {
	return paths.SplitPath(p)
}

// FindByName ищет пропавшие фото в dir: совпасть должны имя файла (без учёта
// регистра) и байты; если байты неизвестны — W×H из заголовка. Читаются только
// заголовки файлов с подходящим именем. Несколько кандидатов: побеждает самый
// длинный общий хвост папок со старым путём, ничья — Ambiguous. Точки-папки
// и ссылки на папки пропускаются (как в Scan); больше limit записей (<= 0 —
// MaxRelinkFiles) — ErrTooManyFiles.
func FindByName(ctx context.Context, dir string, want []Want, limit int) (Found, error) {
	if limit <= 0 {
		limit = MaxRelinkFiles
	}
	byName := wantByName(want)
	cands := make(map[int][]string)
	visited := 0
	dir = filepath.Clean(dir) // как Scanner.walk: корень-ссылку с «\» на конце не обходить по цели
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if visited++; visited > limit {
			return ErrTooManyFiles
		}
		if err := ctxDue(ctx, visited); err != nil {
			return err
		}
		info, file, ret := walkEntry(dir, p, d)
		if !file {
			return ret
		}
		addCands(want, byName[strings.ToLower(d.Name())], p, d, info, cands)
		return nil
	})
	if err != nil {
		return Found{}, err
	}
	if err := ctx.Err(); err != nil {
		return Found{}, err
	}
	res := Found{Paths: map[string]string{}}
	for i, c := range cands {
		if p, ok := pickByTail(c, want[i].Old); ok {
			res.Paths[want[i].ID] = p
		} else {
			res.Ambiguous = append(res.Ambiguous, want[i].ID)
		}
	}
	slices.Sort(res.Ambiguous)
	return res, nil
}

// wantByName — имя файла в нижнем регистре → индексы пропавших фото с этим именем.
func wantByName(want []Want) map[string][]int {
	byName := make(map[string][]int, len(want))
	for i, w := range want {
		if el := SplitPath(w.Old); len(el) > 0 {
			name := strings.ToLower(el[len(el)-1])
			byName[name] = append(byName[name], i)
		}
	}
	return byName
}

// addCands дописывает файл p в кандидаты тех из idx, с кем он сошёлся. Сведения о файле
// берутся только у файла с подходящим именем: info == nil — из d.Info().
func addCands(want []Want, idx []int, p string, d fs.DirEntry, info fs.FileInfo, cands map[int][]string) {
	if len(idx) == 0 {
		return
	}
	if info == nil {
		if info, _ = d.Info(); info == nil {
			return
		}
	}
	var head *Photo // заголовок читаем не больше раза на файл
	for _, i := range idx {
		if matches(want[i], p, info.Size(), &head) {
			cands[i] = append(cands[i], p)
		}
	}
}

// matches сверяет файл с пропавшим фото по байтам, а без них — по W×H заголовка.
func matches(w Want, path string, size int64, head **Photo) bool {
	if w.Size > 0 {
		return size == w.Size
	}
	if w.W <= 0 || w.H <= 0 {
		return true // сверять нечем: решает имя
	}
	if *head == nil {
		ph, err := readInfo(path)
		if err != nil {
			ph = Photo{}
		}
		*head = &ph
	}
	return (*head).W == w.W && (*head).H == w.H
}

// pickByTail выбирает кандидата с самым длинным общим хвостом папок со старым
// путём; ничья на первом месте — ok = false.
func pickByTail(cands []string, old string) (string, bool) {
	oldDirs := SplitPath(old)
	if len(oldDirs) > 0 {
		oldDirs = oldDirs[:len(oldDirs)-1]
	}
	best, bestScore, tie := "", -1, false
	for _, c := range cands {
		el := SplitPath(c)
		s := commonTail(el[:len(el)-1], oldDirs)
		switch {
		case s > bestScore:
			best, bestScore, tie = c, s, false
		case s == bestScore:
			tie = true
		}
	}
	return best, !tie && best != ""
}

// commonTail — сколько элементов с конца совпало (без учёта регистра).
func commonTail(a, b []string) int {
	n := 0
	for n < len(a) && n < len(b) && strings.EqualFold(a[len(a)-1-n], b[len(b)-1-n]) {
		n++
	}
	return n
}

// driveSample — сколько пропавших путей проверяется с подменённой буквой, пока
// ищется диск-кандидат: 26 букв × 20 проверок, а не весь список на каждую.
const driveSample = 20

// DriveHint — подсказка «Диск E: → F:». From пуст — подсказки нет.
type DriveHint struct {
	From  string `json:"from"`  // буква пропавшего диска, «E»
	To    string `json:"to"`    // буква, на которой фото нашлись
	Found int    `json:"found"` // сколько пропавших фото с диска From лежит там же на To
	Total int    `json:"total"` // сколько пропавших фото было на диске From
}

// MissingOf — ID фото, чьих файлов нет (exists — false), по порядку ID. Тип записи общий,
// чтобы library не знал альбом (album/model): путь берёт path.
func MissingOf[V any](refs map[string]V, path func(V) string, exists func(string) bool) []string {
	out := []string{}
	for id, r := range refs {
		if !exists(path(r)) {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// SuggestDrive — выбор буквы по путям пропавших фото; exists подменяется в тестах.
// Стоимость — не больше 26×driveSample проверок на кандидатов и по одной на пропавшее фото.
func SuggestDrive(missing []string, exists func(string) bool) DriveHint {
	letterOf := func(p string) string {
		if vol := filepath.VolumeName(p); len(vol) == 2 && vol[1] == ':' {
			l, _ := paths.DriveLetter(vol)
			return l
		}
		return ""
	}
	counts := map[string]int{}
	for _, p := range missing {
		if l := letterOf(p); l != "" {
			counts[l]++
		}
	}
	from := ""
	for l, n := range counts {
		if n > counts[from] || n == counts[from] && (from == "" || l < from) {
			from = l
		}
	}
	if from == "" {
		return DriveHint{}
	}
	var onFrom []string
	for _, p := range missing {
		if letterOf(p) == from {
			onFrom = append(onFrom, p)
		}
	}
	count := func(to string, list []string) int {
		n := 0
		for _, p := range list {
			if np, ok := paths.SwapDrive(p, from, to); ok && exists(np) {
				n++
			}
		}
		return n
	}
	best, bestN := "", 0
	for c := 'A'; c <= 'Z'; c++ {
		to := string(c)
		if to == from || !exists(to+`:\`) {
			continue
		}
		if n := count(to, onFrom[:min(len(onFrom), driveSample)]); n > bestN {
			best, bestN = to, n
		}
	}
	if best == "" {
		return DriveHint{}
	}
	return DriveHint{From: from, To: best, Found: count(best, onFrom), Total: len(onFrom)}
}
