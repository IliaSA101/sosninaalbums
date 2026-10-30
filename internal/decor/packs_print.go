package decor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
)

// Профили типографий в наборах (123 Б4, design §4): вид print, файл Типографии/<16 hex>.saprint
// под ключом от содержимого. Decor хранит байты и ничего не знает о формате: разбор делает
// printprof в App — через SetPrintCheck при установке и заново при сборке реестра. Ставит их
// только набор из подписанного каталога (origin net): «Добавить из файла» чужой zip с профилем
// не принимает, потому что профиль перекрывает встроенную типографию.

// Пределы: профиль — printprof.MaxSize, на набор и на все наборы — чтобы чтение при сборке
// реестра было ограничено (64 × 64 КБ).
const (
	maxPackPrintFile = MaxPrintFile
	maxPrintsInPack  = 16
	maxPackPrints    = MaxPrints
)

// PackPrint — профиль типографии установленного набора: откуда, версия и minApp набора и байты.
type PackPrint struct {
	Pack    string
	Version int
	MinApp  string
	Data    []byte
}

// SetPrintCheck задаёт проверку профиля набора (printprof.Parse в App); без неё проверяется
// только размер. Набор, чей профиль не прошёл, остальные предметы ставит — профиль пропускается.
func (s *Store) SetPrintCheck(f func(data []byte) error) {
	if f == nil {
		s.printCheck.Store(nil)
		return
	}
	s.printCheck.Store(&f)
}

// stagePrint — профиль набора в память: только origin net, предел размера до чтения, проверка
// printprof, ключ — sha256 содержимого (тот же профиль в двух наборах — один файл).
func (s *Store) stagePrint(i int, src PackSource, st *Staged) (si stagedItem, why string, err error) {
	if st.origin != OriginNet {
		return si, "типографию ставит только набор из каталога библиотеки — из файла она не принимается", nil
	}
	n := 0
	for _, it := range st.items {
		if it.kind == KindPrint {
			n++
		}
	}
	if n >= maxPrintsInPack {
		return si, fmt.Sprintf("в наборе больше %d типографий", maxPrintsInPack), nil
	}
	var buf printBuffer
	if err := src.Extract(i, &buf); err != nil {
		if errors.Is(err, errPrintBig) {
			return si, "файл типографии больше 64 КБ", nil
		}
		return si, "", err
	}
	data := buf.Bytes()
	if len(data) == 0 {
		return si, "файл типографии пуст", nil
	}
	if f := s.printCheck.Load(); f != nil {
		if err := (*f)(data); err != nil {
			return si, "типография не принята: " + cleanText(err.Error(), 200), nil
		}
	}
	sum := sha256.Sum256(data)
	si.key, si.ext, si.tpl, si.bytes = libPrefix+hex.EncodeToString(sum[:8]), printExt, bytes.Clone(data), int64(len(data))
	return si, "", nil
}

var errPrintBig = errors.New("файл типографии больше предела")

// printBuffer — буфер профиля не больше MaxPrintFile: предел и без доверия к источнику.
type printBuffer struct{ bytes.Buffer }

func (b *printBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > maxPackPrintFile {
		return 0, errPrintBig
	}
	return b.Buffer.Write(p)
}

// PackPrints — профили установленных наборов по порядку установки (внутри набора — по порядку
// манифеста), не больше maxPackPrints. Нечитаемый, пустой или слишком большой файл пропускается;
// разбор и проверку версий делает вызывающий.
func (s *Store) PackPrints() []PackPrint {
	type ref struct {
		pack    string
		version int
		minApp  string
		path    string
	}
	var refs []ref
	s.mu.RLock()
	for _, r := range s.packs {
		for _, e := range r.Items {
			if e.Kind != KindPrint {
				continue
			}
			if path, ok := s.files[KindPrint][e.Key]; ok {
				refs = append(refs, ref{r.ID, r.Version, r.MinApp, path})
			}
		}
	}
	s.mu.RUnlock()
	var out []PackPrint
	for _, r := range refs {
		if len(out) >= maxPackPrints {
			log.Printf("библиотека: типографий в наборах больше %d — лишние не читаются", maxPackPrints)
			break
		}
		data, err := readCapped(r.path, maxPackPrintFile)
		if err != nil || len(data) == 0 {
			continue
		}
		out = append(out, PackPrint{Pack: r.pack, Version: r.version, MinApp: r.minApp, Data: data})
	}
	return out
}

// validVer — x.y.z, части по 1…6 цифр (реестр правят руками).
func validVer(v string) bool {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 6 || strings.Trim(p, "0123456789") != "" {
			return false
		}
	}
	return true
}
