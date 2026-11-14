package pack

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Пределы каталога zip до zip.NewReader: тот читает заголовки каталога подряд, пока они
// годны, не глядя на объявленное число записей, — архив в 300 МБ из миллионов пустых записей
// съел бы память и время раньше проверки MaxEntries.
const (
	maxDir  = 1 << 20   // каталог: 2000 записей по ~250 Б с большим запасом
	eocdLen = 22        // конец каталога без комментария
	tailLen = 65 * 1024 // где archive/zip ищет конец каталога
	loc64   = 20        // локатор ZIP64
	end64   = 56        // конец каталога ZIP64
)

// checkDir читает конец каталога (и ZIP64) так же, как archive/zip, и отказывает, если
// записей объявлено больше MaxEntries или от начала каталога до конца файла больше maxDir
// и хвоста (конец каталога, комментарий). Конца каталога нет или он битый — nil: отказ
// скажет zip.NewReader.
func checkDir(r io.ReaderAt, size int64) error {
	n := min(size, tailLen)
	buf := make([]byte, n)
	if _, err := r.ReadAt(buf, size-n); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: не читается как zip", ErrDamaged)
	}
	p := eocdAt(buf)
	if p < 0 {
		return nil
	}
	end := size - n + int64(p)
	b := buf[p:]
	records := uint64(binary.LittleEndian.Uint16(b[10:]))
	dirSize := uint64(binary.LittleEndian.Uint32(b[12:]))
	dirOff := uint64(binary.LittleEndian.Uint32(b[16:]))
	if records == 0xffff || dirSize == 0xffff || dirOff == 0xffffffff { // условие archive/zip
		if p64, ok := zip64End(r, end); ok {
			var rec [end64]byte
			if _, err := r.ReadAt(rec[:], p64); err != nil || binary.LittleEndian.Uint32(rec[:]) != 0x06064b50 {
				return nil
			}
			end = p64
			records = binary.LittleEndian.Uint64(rec[32:])
			dirSize = binary.LittleEndian.Uint64(rec[40:])
			dirOff = binary.LittleEndian.Uint64(rec[48:])
		}
	}
	if records > MaxEntries {
		return fmt.Errorf("в архиве больше %d записей", MaxEntries)
	}
	if dirSize > uint64(end) { //nolint:gosec // end ≥ 0: смещение внутри файла
		return nil // каталог до начала файла — отказ скажет zip.NewReader
	}
	// archive/zip начинает чтение с end − dirSize, а если там нет заголовка и dirOff раньше —
	// с dirOff: берём раннее из двух.
	start := end - int64(dirSize) //nolint:gosec // dirSize ≤ end
	if dirOff < uint64(start) {   //nolint:gosec // start ≥ 0
		start = int64(dirOff) //nolint:gosec // dirOff < start
	}
	if dirSize > maxDir || size-start > maxDir+tailLen+loc64+end64 {
		return fmt.Errorf("%w: каталог архива больше %d МБ", ErrDamaged, maxDir>>20)
	}
	return nil
}

// eocdAt — последняя подпись конца каталога в хвосте b с комментарием, не вылезающим за
// конец (как findSignatureInBlock в archive/zip); нет — −1.
func eocdAt(b []byte) int {
	for i := len(b) - eocdLen; i >= 0; i-- {
		if binary.LittleEndian.Uint32(b[i:]) == 0x06054b50 {
			if int(binary.LittleEndian.Uint16(b[i+eocdLen-2:]))+eocdLen+i > len(b) {
				return -1
			}
			return i
		}
	}
	return -1
}

// zip64End — смещение конца каталога ZIP64 по локатору перед концом каталога (как
// findDirectory64End в archive/zip: диск 0 из одного).
func zip64End(r io.ReaderAt, end int64) (int64, bool) {
	if end < loc64 {
		return 0, false
	}
	var b [loc64]byte
	if _, err := r.ReadAt(b[:], end-loc64); err != nil {
		return 0, false
	}
	if binary.LittleEndian.Uint32(b[:]) != 0x07064b50 || binary.LittleEndian.Uint32(b[4:]) != 0 ||
		binary.LittleEndian.Uint32(b[16:]) != 1 {
		return 0, false
	}
	p := binary.LittleEndian.Uint64(b[8:])
	if p > 1<<62 {
		return 0, false // ReadAt по такому смещению не прочтёт: archive/zip откажет сам
	}
	return int64(p), true
}
