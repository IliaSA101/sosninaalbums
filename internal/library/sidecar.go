package library

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// maxXMPLog — сколько строк об ошибках XMP пишется за одно сканирование: на папке
// с тысячами битых пакетов log.txt не раздувается, остальное — одной сводкой.
const maxXMPLog = 20

// xmpLog — ограничитель строк об ошибках XMP на одно сканирование; воркеры пишут параллельно.
type xmpLog struct{ n atomic.Int64 }

func (l *xmpLog) printf(format string, args ...any) {
	if l.n.Add(1) <= maxXMPLog {
		log.Printf(format, args...)
	}
}

// summary — сводка по строкам, не попавшим в лог.
func (l *xmpLog) summary() {
	if n := l.n.Load(); n > maxXMPLog {
		log.Printf("ошибок XMP за сканирование %d, в лог записаны первые %d", n, maxXMPLog)
	}
}

// maxSidecars — сколько .xmp обход запоминает за одно сканирование: карта сайдкаров
// не должна расти без предела на папке-свалке. Дальше сайдкары не читаются (строка в log.txt).
const maxSidecars = 100_000

// sidecar — файл .xmp, найденный обходом; размер и время — из листинга. Пустой path — сайдкара нет.
type sidecar struct {
	path      string
	size, mod int64
}

// sideSet — сайдкары, найденные обходом: путь в нижнем регистре → файл.
type sideSet struct {
	m    map[string]sidecar
	full bool // предел maxSidecars достигнут, в лог уже сказано
}

// add запоминает сайдкар из листинга. Только обычные файлы: ссылку или junction вместо
// сайдкара walk сюда не передаёт — их не читаем, как и пути из самого XMP.
func (ss *sideSet) add(p string, d fs.DirEntry) {
	if len(ss.m) >= maxSidecars {
		if !ss.full {
			ss.full = true
			log.Printf("сайдкаров .xmp больше %d, остальные не читаются", maxSidecars)
		}
		return
	}
	info, err := d.Info()
	if err != nil {
		return // размера нет — как будто сайдкара нет; найдётся при следующем чтении папок
	}
	ss.m[strings.ToLower(p)] = sidecar{path: p, size: info.Size(), mod: info.ModTime().UnixNano()}
}

func isSidecar(name string) bool { return strings.EqualFold(filepath.Ext(name), ".xmp") }

// sidecarFor ищет сайдкар фото по ключу (путь в нижнем регистре): сначала `имя.jpg.xmp`
// (darktable, digiKam), затем `имя.xmp` (Lightroom, Bridge — и для RAW рядом с JPEG).
// Только среди найденного обходом: путь из XMP или угаданный путь не открывается.
func sidecarFor(key string, sides map[string]sidecar) sidecar {
	if s, ok := sides[key+".xmp"]; ok {
		return s
	}
	if s, ok := sides[strings.TrimSuffix(key, filepath.Ext(key))+".xmp"]; ok {
		return s
	}
	return sidecar{}
}

// readScan — заголовок фото для сканера: readInfo и отметки из XMP в JPEG и сайдкара.
// Битый XMP не делает фото нечитаемым: отметок нет, строка в log.txt (через xl).
// retry — сайдкар не открылся или не дочитался (диск, блокировка): результат не запоминать.
func readScan(f scanFile, xl *xmpLog) (ph Photo, retry bool, err error) {
	ph, packet, err := readHead(f.path)
	if err != nil {
		return ph, false, err
	}
	var m Marks
	var mf markFields
	if packet != nil {
		var perr error
		if m, mf, perr = parseXMP(packet); perr != nil {
			xl.printf("XMP в файле %q: %v", paths.LogExt(ph.Name), perr)
		}
	}
	if f.side.path != "" {
		sm, sf, serr := readSidecar(f.side)
		if serr != nil {
			var pe *fs.PathError
			retry = errors.As(serr, &pe)
			if retry {
				serr = fmt.Errorf("%s: %w", pe.Op, pe.Err) // в PathError полный (личный) путь
			}
			xl.printf("сайдкар %q: %v", paths.LogExt(f.side.path), serr)
		} else {
			m = mergeMarks(m, mf, sm, sf)
		}
	}
	if m != (Marks{}) {
		ph.Xmp = &m
	}
	return ph, retry, nil
}

// readSidecar читает и разбирает сайдкар. Размер проверяется дважды: по листингу (огромный
// файл не открываем) и на чтении (файл мог вырасти после обхода).
func readSidecar(s sidecar) (Marks, markFields, error) {
	if s.size > maxXMPBytes {
		return Marks{}, 0, fmt.Errorf("%d байт, больше %d — не читается", s.size, maxXMPBytes)
	}
	f, err := os.Open(s.path)
	if err != nil {
		return Marks{}, 0, err
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	data, err := io.ReadAll(io.LimitReader(f, maxXMPBytes+1))
	if err != nil {
		return Marks{}, 0, err
	}
	if len(data) > maxXMPBytes {
		return Marks{}, 0, fmt.Errorf("больше %d байт — не читается", maxXMPBytes)
	}
	return parseXMP(data)
}

// mergeMarks: сайдкар главнее встроенного XMP по каждому полю, которое в нём встретилось;
// отсутствующее или нечитаемое поле сайдкара встроенное не затирает. Исключение: отказ
// встроенного пакета из Rating = −1 уходит вместе с рейтингом, если сайдкар задал рейтинг без флага.
func mergeMarks(base Marks, bf markFields, side Marks, f markFields) Marks {
	if f&fieldRating != 0 {
		base.Rating = side.Rating
		if f&fieldFlag == 0 && bf&fieldRejectByRating != 0 {
			base.Flag = 0
		}
	}
	if f&fieldLabel != 0 {
		base.Label, base.LabelText = side.Label, side.LabelText
	}
	if f&fieldFlag != 0 {
		base.Flag = side.Flag
	}
	return base
}
