package library

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// fingerprint — набор файлов-фото и сайдкаров обхода: число и сумма хешей
// (путь с регистром, размер, время, сайдкар). Сумма, а не цепочка: порядок обхода не важен.
type fingerprint struct {
	key string // список папок через \n по правилу paths.Fold
	n   int
	sum uint64
}

// FNV-64a: хеш в цикле без выделений — обход зовётся раз в ~10 с.
const (
	fnvOffset = 14695981039346656037
	fnvPrime  = 1099511628211
)

func fnvAdd(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime
	}
	return h
}

// fingerprintOf считает отпечаток по файлам обхода. В него входят и нечитаемые
// (stBad) файлы: иначе их появление или правка остались бы незамеченными.
func fingerprintOf(src Sources, files []scanFile) fingerprint {
	key := strings.Join(src.Folders, "\n")
	if len(src.Files) > 0 {
		key += "\x00" + strings.Join(src.Files, "\n") // отдельное фото не сойдёт за папку
	}
	fp := fingerprint{key: paths.Fold(key), n: len(files)}
	for _, f := range files {
		// Поля разделены нулём; без сайдкара путь пуст, размер и время — нули.
		h := uint64(fnvOffset)
		for _, part := range []string{f.path, strconv.FormatInt(f.size, 10), strconv.FormatInt(f.mod, 10),
			f.side.path, strconv.FormatInt(f.side.size, 10), strconv.FormatInt(f.side.mod, 10)} {
			h = fnvAdd(fnvAdd(h, part), "\x00")
		}
		fp.sum += h
	}
	return fp
}

// Changed обходит источники тем же walk, что Scan, и сравнивает с отпечатком последнего
// законченного Scan. Список папок или отдельных файлов другой или Scan ещё не заканчивался (или отменён) — true.
// took — длительность обхода. Память заголовков не трогает.
func (s *Scanner) Changed(ctx context.Context, src Sources) (changed bool, took time.Duration, err error) {
	t0 := time.Now()
	files, err := s.walk(ctx, src, func(int) {})
	took = time.Since(t0)
	if err != nil {
		return false, took, err
	}
	fp := fingerprintOf(src, files)
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.lastOK || s.last != fp, took, nil
}

// setLast запоминает отпечаток законченного Scan.
func (s *Scanner) setLast(fp fingerprint) {
	s.mu.Lock()
	s.last, s.lastOK = fp, true
	s.mu.Unlock()
}

// invalidateLast сбрасывает отпечаток в начале Scan: отменённое чтение не должно
// оставить отпечаток от прежнего списка файлов.
func (s *Scanner) invalidateLast() {
	s.mu.Lock()
	s.last, s.lastOK = fingerprint{}, false
	s.mu.Unlock()
}

// MergeNatural — два списка ленты в одном естественном порядке по пути (как Scan).
// Каждый список уже упорядочен; при равных путях сначала элементы a. Входные списки не меняются.
func MergeNatural(a, b []Photo) []Photo {
	out := make([]Photo, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		// b идёт первым только если строго меньше: порядок равных стабилен.
		if naturalLess(strings.ToLower(b[j].Path), strings.ToLower(a[i].Path)) {
			out = append(out, b[j])
			j++
		} else {
			out = append(out, a[i])
			i++
		}
	}
	out = append(out, a[i:]...)
	return append(out, b[j:]...)
}
