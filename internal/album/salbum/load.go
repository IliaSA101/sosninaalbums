package salbum

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// maxUp — сколько «..» допускает относительный путь: альбом в Ивановы\Альбом\,
// фото в Ивановы\Отбор\ — один уровень; два — запас. Дальше пишем абсолютный.
const maxUp = 2

// Файловые типы: в памяти пути абсолютные, относительные живут только здесь.
type FileProject struct {
	Version  int                  `json:"version"`
	Product  string               `json:"product"`
	Folders  []FileFolder         `json:"folders"`
	Settings model.Settings       `json:"settings"`
	Spreads  []*model.Spread      `json:"spreads"`
	Photos   map[string]FilePhoto `json:"photos"`
	Marks    markList             `json:"marks,omitempty"` // свои отметки (v5, 004)
}

// FileFolder — папка: path (относительный или абсолютный) и запасной abs.
type FileFolder struct {
	Path string `json:"path"`
	Abs  string `json:"abs,omitempty"`
}

// UnmarshalJSON принимает и строку: так папки записаны в v1.
func (f *FileFolder) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = FileFolder{Path: s}
		return nil
	}
	type plain FileFolder
	return json.Unmarshal(b, (*plain)(f))
}

type FilePhoto struct {
	model.PhotoRef
	Abs string `json:"abs,omitempty"` // запасной абсолютный путь
}

// Load читает проект. Относительные пути разрешаются от папки файла.
func Load(path string) (*model.Project, error) {
	data, err := ReadLimited(path)
	if err != nil {
		return nil, err
	}
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	return Decode(data, base)
}

// ReadLimited читает файл проекта не больше model.MaxFileSize; нужен и LoadBase ядра.
func ReadLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	if st, err := f.Stat(); err == nil && st.Size() > model.MaxFileSize {
		return nil, fmt.Errorf("файл проекта слишком большой (больше %d МБ)", model.MaxFileSize>>20)
	}
	// Предел и на чтении: размер из Stat мог измениться.
	data, err := io.ReadAll(io.LimitReader(f, model.MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > model.MaxFileSize {
		return nil, fmt.Errorf("файл проекта слишком большой (больше %d МБ)", model.MaxFileSize>>20)
	}
	return data, nil
}

// ErrNewer — файл записан более новой версией программы: его нельзя ни открыть,
// ни трогать (старая версия молча выбросила бы незнакомые поля). Проверять через errors.Is.
var ErrNewer = errors.New("проект сохранён более новой версией программы")

// decodeImpl — сам разбор; в тестах подменяется, чтобы имитировать панику.
var decodeImpl = decodeFile

// Decode разбирает файл под recover: паника на странном входе — ошибка «файл повреждён»,
// а не падение при каждом запуске (черновик читается на старте, и он же не уходит сам).
func Decode(data []byte, base string) (p *model.Project, err error) {
	defer func() {
		if r := recover(); r != nil {
			p, err = nil, fmt.Errorf("файл проекта повреждён: %v", r)
		}
	}()
	return decodeImpl(data, base)
}

// decodeFile разбирает файл: пути (с проверкой и выбором из двух), повторы,
// ID по путям, лимиты, миграция старой версии. base — папка файла.
// Порядок шагов сетки (design 007 §2): nil и объединение мест с Photos — до
// всего, что трогает места; починка мест — последней, по зажатой геометрии.
func decodeFile(data []byte, base string) (*model.Project, error) {
	f := FileProject{Settings: model.DefaultSettings()}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("файл проекта повреждён: %w", err)
	}
	if f.Version > model.FormatVersion {
		return nil, ErrNewer
	}
	if len(f.Spreads) > model.MaxSpreads { // сырой список: до dedupe/splitBig не гонять огромный файл
		return nil, fmt.Errorf("слишком много разворотов в проекте (больше %d)", model.MaxSpreads)
	}
	p := model.New()
	p.SetFrom(max(f.Version, 1))
	p.Product = model.NormProduct(f.Product)
	p.Settings, p.Spreads = f.Settings, f.Spreads
	for _, s := range p.Spreads {
		if s == nil {
			return nil, fmt.Errorf("файл проекта повреждён: пустой разворот")
		}
	}
	v1 := p.From() < 2
	if err := loadFolders(p, f.Folders, base, v1); err != nil {
		return nil, err
	}
	for id, fp := range f.Photos {
		ref, err := resolvePhoto(fp, base, v1)
		if err != nil {
			return nil, fmt.Errorf("фото в проекте: %w", err)
		}
		p.Photos[id] = ref
	}
	dropNilPlaces(p)
	unionGrid(p)
	dedupe(p)
	p.PruneRefs()
	Rekey(p, library.IDFor)
	p.SplitBig()
	// После splitBig: деление больших разворотов добавляет новые, а лишние
	// после Save+Load молча пропали бы.
	if len(p.Spreads) > model.MaxSpreads {
		return nil, fmt.Errorf("слишком много разворотов в проекте (больше %d)", model.MaxSpreads)
	}
	p.Settings = model.ClampSettings(p.Settings)
	p.ClampDesign()
	p.PruneRefs() // фон, выброшенный зажимом (взаимоисключение), не оставляет фото без дела
	p.ClampGrid()
	migrate(p)
	if err := p.SyncGrid(true); err != nil {
		return nil, err
	}
	p.FitTexts()
	p.LoadOrder()
	p.SetLoaded(decodeMarks(f.Marks, base, p.Folders))
	return p, nil
}

// loadFolders — папки из файла. Сверх MaxFolders хвост отбрасывается: своя программа больше не
// пишет (AddFolder), а чужой файл с сотнями тысяч папок не держит открытие на stat каждой.
func loadFolders(p *model.Project, list []FileFolder, base string, v1 bool) error {
	var over int
	if len(list) > 2*model.MaxFolders { // с повторами: хвост сырого списка не разбираем
		over, list = len(list)-2*model.MaxFolders, list[:2*model.MaxFolders]
	}
	seen := make(map[string]bool, min(len(list), model.MaxFolders))
	for _, ff := range list {
		dir, alt, err := candidates(ff.Path, ff.Abs, base, v1)
		if err != nil {
			return fmt.Errorf("папка в проекте: %w", err)
		}
		if alt != "" && !isDir(dir) {
			if isDir(alt) {
				dir = alt
			} else if !seen[dir] && len(p.Folders) < model.MaxFolders {
				// Нигде нет: запасной путь едет дальше, иначе следующая запись его потеряет.
				if p.FolderAlt == nil {
					p.FolderAlt = map[string]string{}
				}
				p.FolderAlt[dir] = alt
			}
		}
		switch {
		case seen[dir]:
		case len(p.Folders) >= model.MaxFolders:
			over++
		default:
			seen[dir] = true
			p.Folders = append(p.Folders, dir)
		}
	}
	if over > 0 {
		log.Printf("папки проекта: больше %d, отброшено записей %d", model.MaxFolders, over)
	}
	return nil
}

// candidates разбирает пару path/abs из файла: primary — основной путь
// (относительный уже разрешён от base), alt — запасной ("" — нет или тот же).
// Оба проходят CheckLocalPath; битый относительный путь — файл повреждён.
func candidates(path, abs, base string, v1 bool) (primary, alt string, err error) {
	if abs != "" {
		if err := paths.CheckLocalPath(abs); err != nil {
			return "", "", err
		}
		alt = abs
	}
	if filepath.IsAbs(path) || v1 {
		// v1 не знал относительных путей: там любой путь проверяется как абсолютный.
		if err := paths.CheckLocalPath(path); err != nil {
			return "", "", err
		}
		primary = path
	} else {
		if err := CheckRel(path); err != nil {
			return "", "", err
		}
		primary = filepath.Join(base, filepath.FromSlash(path))
		if base == "" || paths.CheckLocalPath(primary) != nil {
			// Альбом открыт не с локального диска (UNC): остаётся только запасной путь.
			if alt == "" {
				return "", "", fmt.Errorf("относительный путь у альбома не на локальном диске: %s", path)
			}
			return alt, "", nil
		}
	}
	if paths.SamePath(primary, alt) {
		alt = ""
	}
	return primary, alt, nil
}

// resolvePhoto выбирает путь фото: основной, если он на месте и байты совпали
// (или размер неизвестен); затем запасной; затем основной с другим размером
// (размеры обновляются). Нигде нет — основной, запасной хранится в Alt.
func resolvePhoto(fp FilePhoto, base string, v1 bool) (model.PhotoRef, error) {
	ref, err := resolvePaths(fp, base, v1)
	if err == nil {
		fixDims(&ref, fp.Size <= 0)
	}
	return ref, err
}

// maxAspect — предел пропорции кадра из файла. Настоящие панорамы не уже 1:20;
// запас большой, а подделанные 4000×10 (раздувают окно фона при экспорте) отсекаются.
const maxAspect = 100

// fixDims: размеры из .salbum — чужие данные. Без размера файла в записи (unverified), больше
// MaxPixels или с нелепой пропорцией они перечитываются из заголовка файла;
// если файла нет или он не читается — обнуляются (нулевые размеры раскладка терпит).
func fixDims(ref *model.PhotoRef, unverified bool) {
	w, h := int64(ref.W), int64(ref.H)
	sane := w > 0 && h > 0 && w*h <= imgfile.MaxPixels && w <= maxAspect*h && h <= maxAspect*w
	if sane && !unverified {
		return
	}
	if rw, rh, ok := probeDims(ref.Path); ok {
		ref.W, ref.H = rw, rh
		return
	}
	if !sane {
		ref.W, ref.H = 0, 0
	}
}

func resolvePaths(fp FilePhoto, base string, v1 bool) (model.PhotoRef, error) {
	ref := fp.PhotoRef
	ref.Alt = "" // alt в файл не пишется; если он там есть, это не наше
	ref.W, ref.H, ref.Size = max(ref.W, 0), max(ref.H, 0), max(ref.Size, 0)
	primary, alt, err := candidates(fp.Path, fp.Abs, base, v1)
	if err != nil {
		return ref, err
	}
	ref.Path = primary
	size, ok := fileSize(primary)
	if ok && (ref.Size == 0 || size == ref.Size) {
		ref.Size = size
		return ref, nil
	}
	if alt != "" {
		if asize, aok := fileSize(alt); aok {
			ref.Path = alt
			refresh(&ref, asize)
			return ref, nil
		}
	}
	if ok {
		refresh(&ref, size)
		return ref, nil
	}
	ref.Alt = alt
	return ref, nil
}

// refresh: файл на месте, но другой длины — фото пересохранили; размеры в
// пикселях читаются из заголовка заново.
func refresh(ref *model.PhotoRef, size int64) {
	if ref.Size != 0 && ref.Size != size {
		if w, h, ok := probeDims(ref.Path); ok {
			ref.W, ref.H = w, h
		}
	}
	ref.Size = size
}

// probeDims читает размеры из заголовка с учётом поворота EXIF: пересохранённое
// фото могли и повернуть, а пропорции нужны раскладке точные.
func probeDims(path string) (w, h int, ok bool) {
	if !reachable(path) {
		return 0, 0, false
	}
	ph, err := library.ReadInfo(path)
	if err != nil {
		return 0, 0, false
	}
	return ph.W, ph.H, true
}

// reachable — отвечает ли том пути (osx.Reachable); подменяется в тестах. Мёртвый сетевой том
// для Load — то же, что «файла нет»: пути и Alt остаются, размеры не обнуляются, а окно не ждёт
// таймаута SMB на каждом фото.
var reachable = osx.Reachable

func fileSize(path string) (int64, bool) {
	if !reachable(path) {
		return 0, false
	}
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() {
		return 0, false
	}
	return st.Size(), true
}

func isDir(path string) bool {
	if !reachable(path) {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

// CheckRel — относительный путь из файла: без диска, двоеточий и NUL, не от
// корня; элементы непустые, без «.»; «..» только в начале и не больше maxUp;
// остальное — обычные имена (filepath.IsLocal отсекает NUL, CON и прочие устройства).
// Сам «.» — папка альбома.
func CheckRel(r string) error {
	bad := fmt.Errorf("файл проекта повреждён: недопустимый относительный путь %q", r)
	if r == "." {
		return nil
	}
	if r == "" || strings.ContainsAny(r, "\x00:") || paths.IsSep(r[0]) || filepath.VolumeName(r) != "" {
		return bad
	}
	parts := strings.Split(strings.ReplaceAll(r, `\`, "/"), "/")
	up := 0
	for up < len(parts) && parts[up] == ".." {
		up++
	}
	if up > maxUp {
		return bad
	}
	body := parts[up:]
	for _, el := range body {
		if el == "" || el == "." || el == ".." {
			return bad
		}
	}
	if len(body) > 0 && !filepath.IsLocal(filepath.FromSlash(strings.Join(body, "/"))) {
		return bad
	}
	return nil
}

// RelPath — путь относительно папки альбома для записи: тот же том, не выше
// maxUp уровней. base == "" (черновик) — всегда абсолютный.
func RelPath(base, path string) (string, bool) {
	if base == "" || !filepath.IsAbs(path) || !strings.EqualFold(filepath.VolumeName(base), filepath.VolumeName(path)) {
		return "", false
	}
	r, err := filepath.Rel(base, path)
	if err != nil {
		return "", false
	}
	r = filepath.ToSlash(r)
	if CheckRel(r) != nil {
		return "", false
	}
	return r, true
}

// Check — пути проекта из памяти (NewEditor, Restore) по правилам Load: после decodeFile
// каждый путь — абсолютный локальный (candidates), поэтому CheckLocalPath и только он; CheckRel
// здесь не нужен — относительный путь CheckLocalPath не пропустит. Без диска: Load уже стучался
// к файлам, второй обход тысяч фото открытие только замедлит. Отметки — из p.Loaded().
func Check(p *model.Project) error {
	for _, dir := range p.Folders {
		if err := checkPair("папка", dir, ""); err != nil {
			return err
		}
	}
	for dir, alt := range p.FolderAlt {
		if err := checkPair("папка", dir, alt); err != nil {
			return err
		}
	}
	for _, r := range p.Photos {
		if err := checkPair("фото", r.Path, r.Alt); err != nil {
			return err
		}
	}
	for _, m := range p.Loaded() {
		if err := checkPair("отметка", m.Path, m.Alt()); err != nil {
			return err
		}
	}
	return nil
}

// checkPair — путь и запасной путь ("" — его нет) для Check.
func checkPair(what, path, alt string) error {
	err := paths.CheckLocalPath(path)
	if err == nil && alt != "" {
		err = paths.CheckLocalPath(alt)
	}
	if err != nil {
		return fmt.Errorf("%s в проекте: %w", what, err)
	}
	return nil
}

// Encode — файл проекта с отметками p.loaded (проект без Editor).
func Encode(p *model.Project, base string) ([]byte, error) { return encodeWith(p, base, p.Loaded()) }

// encodeWith — файл проекта и свои отметки marks. Пути внутри base пишутся относительными
// с абсолютным рядом; base == "" — все абсолютные.
func encodeWith(p *model.Project, base string, marks map[string]model.Mark) ([]byte, error) {
	f := FileProject{
		Version:  model.FormatVersion,
		Product:  model.NormProduct(p.Product),
		Folders:  make([]FileFolder, 0, len(p.Folders)),
		Settings: p.Settings,
		Spreads:  p.Spreads,
		Photos:   make(map[string]FilePhoto, len(p.Photos)),
	}
	if f.Spreads == nil {
		f.Spreads = []*model.Spread{}
	}
	for _, dir := range p.Folders {
		ff := FileFolder{Path: dir}
		if r, ok := RelPath(base, dir); ok {
			ff = FileFolder{Path: r, Abs: dir}
		}
		if alt := p.FolderAlt[dir]; alt != "" && !isDir(dir) {
			ff.Abs = alt // папки нет и тут: запасной путь как был
		}
		f.Folders = append(f.Folders, ff)
	}
	for id, ref := range p.Photos {
		fp := FilePhoto{PhotoRef: ref}
		fp.Alt = ""
		if r, ok := RelPath(base, ref.Path); ok {
			fp.Path, fp.Abs = r, ref.Path
		}
		if ref.Alt != "" {
			fp.Abs = ref.Alt // фото не нашлось при открытии: запасной путь как был
		}
		f.Photos[id] = fp
	}
	f.Marks = encodeMarks(marks, base)
	return json.MarshalIndent(f, "", "  ")
}

// Save пишет проект; относительные пути считаются от папки файла.
func Save(p *model.Project, path string) error { return SaveWith(p, path, p.Loaded()) }

// SaveWith — Save со своими отметками marks (Editor держит их отдельно от проекта).
func SaveWith(p *model.Project, path string, marks map[string]model.Mark) error {
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return err
	}
	return Write(p, path, base, marks)
}

// Write — запись через временный файл, чтобы сбой не испортил проект.
// base == "" — все пути абсолютные (черновик).
func Write(p *model.Project, path, base string, marks map[string]model.Mark) error {
	data, err := encodeWith(p, base, marks)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeAtomic(path, data)
}
