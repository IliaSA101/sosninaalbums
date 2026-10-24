package pack

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// maxRatio — во сколько раз запись может быть больше сжатой (+ slack): сильнее жмутся только
// нули и повторы — признак zip-бомбы, а не картинки.
const (
	maxRatio = 200
	slack    = 1 << 20
)

// Archive — открытый и проверенный набор: манифест, хэш архива и записи предметов по имени.
// Архив не в памяти (ReaderAt): предмет читается потоком в Extract.
type Archive struct {
	Manifest Manifest
	SHA256   string // архив целиком: реестр набора, сверка с каталогом (Open; у OpenReader — "")

	files  map[string]*zip.File
	closer io.Closer
}

// Open открывает набор-файл: обычный файл не больше MaxZip, sha256 архива, затем OpenReader.
// app — версия программы для minApp. Ошибки — без пути.
func Open(path, app string) (*Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.New("файл набора не открывается")
	}
	a, err := openFile(f, app)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	a.closer = f
	return a, nil
}

func openFile(f *os.File, app string) (*Archive, error) {
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() {
		return nil, errors.New("набор — не обычный файл")
	}
	if st.Size() > MaxZip {
		return nil, fmt.Errorf("архив набора больше %d МБ", MaxZip>>20)
	}
	h := sha256.New()
	if _, err := io.Copy(h, io.NewSectionReader(f, 0, st.Size())); err != nil {
		return nil, errors.New("файл набора не читается")
	}
	a, err := OpenReader(f, st.Size(), app)
	if err != nil {
		return nil, err
	}
	a.SHA256 = hex.EncodeToString(h.Sum(nil))
	return a, nil
}

// Close закрывает файл архива (у OpenReader — ничего).
func (a *Archive) Close() error {
	if a.closer == nil {
		return nil
	}
	return a.closer.Close()
}

// OpenReader — проверки архива (design §4.2): число записей и размер каталога (до разбора), имена (опасное — отказ всего
// архива, незнакомое — запись пропускается), повтор без учёта регистра, метод и шифрование,
// степень сжатия по заголовку, манифест, каждая запись предмета есть и её размер = манифесту.
func OpenReader(r io.ReaderAt, size int64, app string) (*Archive, error) {
	if size > MaxZip {
		return nil, fmt.Errorf("архив набора больше %d МБ", MaxZip>>20)
	}
	if err := checkDir(r, size); err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(r, size)
	if errors.Is(err, zip.ErrInsecurePath) {
		return nil, errUnsafe
	}
	if err != nil {
		return nil, fmt.Errorf("%w: не читается как zip", ErrDamaged)
	}
	if len(zr.File) > MaxEntries {
		return nil, fmt.Errorf("в архиве больше %d записей", MaxEntries)
	}
	files, err := entries(zr.File, size)
	if err != nil {
		return nil, err
	}
	mf := files[ManifestName]
	if mf == nil {
		return nil, fmt.Errorf("%w: нет %s", ErrDamaged, ManifestName)
	}
	data, err := readEntry(mf, MaxManifest)
	if err != nil {
		return nil, err
	}
	m, err := ParseManifest(data, app)
	if err != nil {
		return nil, err
	}
	for _, it := range m.Items {
		zf := files[it.File]
		if zf == nil {
			return nil, fmt.Errorf("%w: нет «%s»", ErrDamaged, it.File)
		}
		if zf.UncompressedSize64 != uint64(it.Size) { //nolint:gosec // Size ≥ 1 после ParseManifest
			return nil, fmt.Errorf("%w: размер «%s» не сходится с манифестом", ErrDamaged, it.File)
		}
	}
	return &Archive{Manifest: m, files: files}, nil
}

var errUnsafe = errors.New("недопустимое имя в архиве набора")

// entries — проверка всех записей; итог — обычные файлы с именем по грамматике (папки,
// симлинки и чужие имена вроде LICENSE.txt не читаются).
func entries(list []*zip.File, size int64) (map[string]*zip.File, error) {
	seen := make(map[string]bool, len(list))
	files := make(map[string]*zip.File, len(list))
	for _, zf := range list {
		name := strings.TrimSuffix(zf.Name, "/")
		if unsafeName(name) {
			return nil, errUnsafe
		}
		low := strings.ToLower(name)
		if seen[low] {
			return nil, fmt.Errorf("%w: имя «%s» повторяется", ErrDamaged, Clean(name, 64))
		}
		seen[low] = true
		if zf.Flags&0x1 != 0 {
			return nil, errors.New("зашифрованный архив не принимается")
		}
		if zf.Method != zip.Store && zf.Method != zip.Deflate {
			return nil, fmt.Errorf("%w: незнакомый метод сжатия", ErrDamaged)
		}
		if zf.CompressedSize64 > uint64(size) || //nolint:gosec // size ≥ 0: длина файла
			zf.UncompressedSize64 > maxRatio*zf.CompressedSize64+slack {
			return nil, fmt.Errorf("%w: запись сжата слишком сильно (похоже на zip-бомбу)", ErrDamaged)
		}
		if zf.Mode().IsRegular() && !strings.HasSuffix(zf.Name, "/") && (ValidName(name) || name == ManifestName) {
			files[name] = zf
		}
	}
	return files, nil
}

// unsafeName — имя, опасное как путь (абсолютное, с диском, «\», «..», управляющие знаки):
// в путь оно не идёт, но такой архив собран не для нас — отказ целиком (защита в глубину).
func unsafeName(name string) bool {
	if name == "" || strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\:") {
		return true
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	for p := range strings.SplitSeq(name, "/") {
		if p == "" || p == "." || p == ".." {
			return true
		}
	}
	return false
}

// readEntry — запись целиком в память (манифест): не больше limit по фактическим байтам.
func readEntry(zf *zip.File, limit int64) ([]byte, error) {
	if zf.UncompressedSize64 > uint64(limit) { //nolint:gosec // limit — константа пакета
		return nil, fmt.Errorf("%w: больше %d КБ", ErrManifest, limit>>10)
	}
	rc, err := zf.Open()
	if err != nil {
		return nil, fmt.Errorf("%w: %s не читается", ErrDamaged, ManifestName)
	}
	defer rc.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil || int64(len(data)) > limit || uint64(len(data)) != zf.UncompressedSize64 {
		return nil, fmt.Errorf("%w: %s не читается", ErrDamaged, ManifestName)
	}
	return data, nil
}

// Extract пишет предмет i манифеста в w потоком: не больше Size байт (лишний байт — отказ),
// затем сверка размера и sha256 с манифестом. Ошибка чтения или сверки — ErrDamaged (архив
// подменён или битый); ошибка записи в w возвращается как есть (диск).
func (a *Archive) Extract(i int, w io.Writer) error {
	if i < 0 || i >= len(a.Manifest.Items) {
		return fmt.Errorf("%w: нет предмета %d", ErrDamaged, i)
	}
	it := a.Manifest.Items[i]
	zf := a.files[it.File]
	if zf == nil {
		return fmt.Errorf("%w: нет «%s»", ErrDamaged, it.File)
	}
	rc, err := zf.Open()
	if err != nil {
		return fmt.Errorf("%w: «%s» не читается", ErrDamaged, it.File)
	}
	defer rc.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	h := sha256.New()
	out := &trackWriter{w: w}
	n, err := io.Copy(io.MultiWriter(out, h), io.LimitReader(rc, it.Size))
	if out.err != nil {
		return out.err
	}
	if err == nil && n == it.Size {
		// Ещё байт — запись длиннее манифеста (бомба, подмена); конец — zip сверяет CRC.
		var extra [1]byte
		if k, rerr := rc.Read(extra[:]); k > 0 || rerr != nil && !errors.Is(rerr, io.EOF) {
			n++
		}
	}
	if err != nil || n != it.Size {
		return fmt.Errorf("%w: «%s» не сходится с манифестом по размеру", ErrDamaged, it.File)
	}
	if hex.EncodeToString(h.Sum(nil)) != it.SHA256 {
		return fmt.Errorf("%w: «%s» не сходится с манифестом (sha256)", ErrDamaged, it.File)
	}
	return nil
}

// trackWriter запоминает ошибку записи: io.Copy не говорит, какая сторона сломалась.
type trackWriter struct {
	w   io.Writer
	err error
}

func (t *trackWriter) Write(p []byte) (int, error) {
	n, err := t.w.Write(p)
	if err != nil && t.err == nil {
		t.err = err
	}
	return n, err
}
