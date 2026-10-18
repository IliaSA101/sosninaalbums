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
	// Vars — поля альбома из dev-сборок v6 (030): только чтение, Load переносит их в листы
	// (moveAlbumVars), Save не пишет — поля шапки живут в Spread.Vars (решение 125).
	Vars  *albumVars `json:"vars,omitempty"`
	Cover *FileCover `json:"cover,omitempty"` // обложка (v6, 032; cover.go)
	// SpreadGuide — файл-разметка разворотов (v11, 066): относительный от папки альбома,
	// SpreadGuideAbs — запасной абсолютный (как guide/guideAbs обложки).
	SpreadGuide    string `json:"spreadGuide,omitempty"`
	SpreadGuideAbs string `json:"spreadGuideAbs,omitempty"`
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

// errTooMany — отказ открыть альбом из n разворотов (058 №18): файл 0.13 мог держать до 1000,
// обрезка потеряла бы данные, поэтому файл не трогаем и говорим, что делать. Строчная буква и
// без точки в конце — правило ошибок Go; заглавную ставит окно (errText).
func errTooMany(n int) error {
	return fmt.Errorf("в альбоме %d %s — программа открывает не больше %d. Файл не изменён: "+
		"откройте его в Sosnina Albums 0.13 и разделите на два альбома", n, spreadsWord(n), model.MaxSpreads)
}

// spreadsWord — «разворот» в числе n: 201 разворот, 202 разворота, 230 разворотов.
func spreadsWord(n int) string {
	switch {
	case n%100 >= 11 && n%100 <= 14:
		return "разворотов"
	case n%10 == 1:
		return "разворот"
	case n%10 >= 2 && n%10 <= 4:
		return "разворота"
	}
	return "разворотов"
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
	if err := checkRaw(&f); err != nil {
		return nil, err
	}
	p := model.New()
	p.SetFrom(max(f.Version, 1))
	p.Product = model.NormProduct(f.Product)
	// foldLegacy — до всех шагов, что считают «Авто»: они — по прежнему поведению (064).
	p.Settings, p.Spreads = foldLegacy(p.From(), f.Settings), f.Spreads
	for _, s := range p.Spreads {
		if s == nil {
			return nil, fmt.Errorf("файл проекта повреждён: пустой разворот")
		}
	}
	v1 := p.From() < 2
	if err := loadFolders(p, f.Folders, base, v1); err != nil {
		return nil, err
	}
	fixCoverAndGuide(p, &f, base, v1)
	if err := loadPhotos(p, f.Photos, base, v1); err != nil {
		return nil, err
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
		return nil, errTooMany(len(p.Spreads))
	}
	p.Settings = model.ClampSettings(p.Settings)
	p.FoldPages()
	moveAlbumVars(p, f.Vars) // до ClampDesign: запас знаков — уже по листам
	p.ClampDesign()
	p.PruneRefs() // фон, выброшенный зажимом (взаимоисключение), не оставляет фото без дела
	p.ClampGrid()
	migrate(p)
	if err := p.SyncGrid(true); err != nil {
		return nil, err
	}
	coverNoAuto(p)
	p.FitTexts()
	p.FitStickers()
	p.LoadOrder()
	p.SetLoaded(decodeMarks(f.Marks, base, p.Folders))
	return p, nil
}

// loadPhotos — фото из файла. Устройство в пути на этой ОС (paths.ErrDevice, 073-10) — фото
// недостающее, как ненайденный файл: путь остаётся, к файлу не обращаемся.
func loadPhotos(p *model.Project, list map[string]FilePhoto, base string, v1 bool) error {
	devs := 0
	for id, fp := range list {
		if fp.Lib != "" { // своё из библиотеки: путь — от искателя (lib.go)
			if ref, ok := resolveLib(fp); ok {
				p.Photos[id] = ref
			}
			continue
		}
		ref, err := resolvePhoto(fp, base, v1)
		if errors.Is(err, paths.ErrDevice) {
			devs++
		} else if err != nil {
			return fmt.Errorf("фото в проекте: %w", err)
		}
		p.Photos[id] = ref
	}
	if devs > 0 {
		log.Printf("фото проекта: %d с именем устройства Windows в пути — недостающие", devs)
	}
	return nil
}

// checkRaw — первые шаги по сырому файлу, до всякого разбора: новее программы — ErrNewer, сверх
// MaxSpreads — отказ (до dedupe/splitBig не гонять огромный файл), хвост мест — capRawGrid.
func checkRaw(f *FileProject) error {
	if f.Version > model.FormatVersion {
		return ErrNewer
	}
	if len(f.Spreads) > model.MaxSpreads {
		return errTooMany(len(f.Spreads))
	}
	capRawGrid(f)
	return nil
}

// rawPlaces — предел сырого списка мест листа до починки. Своя программа пишет не больше
// MaxPlaces, и ClampGrid (trimPlaces) всё равно оставит столько же; запас ×4 — чтобы у почти
// верного файла выбор, какие места оставить, по-прежнему делал trimPlaces.
const rawPlaces = 4 * model.MaxPlaces

// capRawGrid — хвост мест сверх rawPlaces у разворотов и обложки отбрасывается сразу после
// разбора, как сырой список папок (loadFolders): чужой или битый файл с сотнями тысяч мест
// иначе держал бы открытие секунды и сотни МБ в dedupe, unionGrid и ClampGrid. Отказ, как у
// MaxSpreads, не нужен: такой файл ни одна версия программы не писала, а мест сверх MaxPlaces
// Load и так не оставляет. Фото из отброшенных мест, которых нет в Photos листа, выпадают.
func capRawGrid(f *FileProject) {
	capGrid := func(s *model.Spread) {
		if s != nil && len(s.Grid) > rawPlaces {
			clear(s.Grid[rawPlaces:])
			s.Grid = s.Grid[:rawPlaces:rawPlaces]
		}
	}
	for _, s := range f.Spreads {
		capGrid(s)
	}
	if f.Cover != nil {
		capGrid(f.Cover.Sheet)
	}
}

// loadFolders — папки из файла. Сверх MaxFolders хвост отбрасывается: своя программа больше не
// пишет (AddFolder), а чужой файл с сотнями тысяч папок не держит открытие на stat каждой.
func loadFolders(p *model.Project, list []FileFolder, base string, v1 bool) error {
	var over int
	if len(list) > 2*model.MaxFolders { // с повторами: хвост сырого списка не разбираем
		over, list = len(list)-2*model.MaxFolders, list[:2*model.MaxFolders]
	}
	seen := make(map[string]bool, min(len(list), model.MaxFolders))
	devs := 0
	for _, ff := range list {
		dir, alt, err := candidates(ff.Path, ff.Abs, base, v1)
		if errors.Is(err, paths.ErrDevice) {
			// Устройство на этой ОС (Windows 10 и «CON.jpg» с 11): папку не прочитать — пропуск;
			// её фото — недостающие (decodeFile).
			devs++
			continue
		}
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
	if devs > 0 {
		log.Printf("папки проекта: пропущено с именем устройства Windows в пути %d", devs)
	}
	return nil
}

// candidates разбирает пару path/abs из файла: primary — основной путь
// (относительный уже разрешён от base), alt — запасной ("" — нет или тот же).
// Оба проходят CheckLocalPath; битый относительный путь — файл повреждён. Устройство на этой
// ОС (paths.ErrDevice: «CON.jpg» на Windows 10) — не порча: годный второй путь становится
// основным, иначе ошибка с ErrDevice и primary — путь с устройством (фото — недостающее).
func candidates(path, abs, base string, v1 bool) (primary, alt string, err error) {
	var dev string // путь с устройством: годного нет — его отдаёт ошибка ErrDevice
	var devErr error
	if abs != "" {
		if err := paths.CheckLocalPath(abs); errors.Is(err, paths.ErrDevice) {
			dev, devErr = abs, err
		} else if err != nil {
			return "", "", err
		} else {
			alt = abs
		}
	}
	devOrAlt := func(p string, err error) (string, string, error) {
		if alt != "" {
			return alt, "", nil
		}
		return p, "", err
	}
	if filepath.IsAbs(path) || v1 {
		// v1 не знал относительных путей: там любой путь проверяется как абсолютный.
		if err := paths.CheckLocalPath(path); errors.Is(err, paths.ErrDevice) {
			return devOrAlt(path, err)
		} else if err != nil {
			return "", "", err
		}
		primary = path
	} else {
		if err := CheckRel(path); err != nil && !errors.Is(err, paths.ErrDevice) {
			return "", "", err
		}
		primary = filepath.Join(base, filepath.FromSlash(path))
		perr := paths.CheckLocalPath(primary)
		switch {
		case base != "" && perr == nil:
		case base != "" && errors.Is(perr, paths.ErrDevice):
			return devOrAlt(primary, perr)
		case alt != "":
			// Альбом открыт не с локального диска (UNC): остаётся только запасной путь.
			return alt, "", nil
		case dev != "":
			return dev, "", devErr
		default:
			return "", "", fmt.Errorf("относительный путь у альбома не на локальном диске: %s", paths.LogQ(path))
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
	switch {
	case err == nil:
		fixDims(&ref, fp.Size <= 0)
	case errors.Is(err, paths.ErrDevice):
		fixDims(&ref, false) // без чтения файла: открыть путь — обратиться к устройству
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
	if errors.Is(err, paths.ErrDevice) {
		ref.Path = filepath.Clean(primary)
		return ref, err
	}
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
	if path == "" || paths.CheckLocalPath(path) != nil || !reachable(path) {
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
// Сам «.» — папка альбома. Устройство на этой ОС (paths.DeviceElem: «CON.jpg» на Windows 10) —
// ошибка с paths.ErrDevice, а не «повреждён»: файл цел, Load считает фото недостающим.
func CheckRel(r string) error {
	bad := fmt.Errorf("файл проекта повреждён: недопустимый относительный путь %s", paths.LogQ(r))
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
	for _, el := range body {
		if paths.DeviceElem(el) {
			return fmt.Errorf("относительный путь %s: %w", paths.LogQ(r), paths.ErrDevice)
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
// каждый путь — абсолютный локальный (candidates), поэтому CheckLocalPath и только он (у
// своего из библиотеки путь бывает пуст, ключ — по грамматике); CheckRel
// здесь не нужен — относительный путь CheckLocalPath не пропустит; фото с устройством в пути
// (paths.ErrDevice) — недостающее, как после Load. Без диска: Load уже стучался
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
		if r.Lib != "" && !model.ValidLibKey(r.Lib) {
			return fmt.Errorf("фото в проекте: недопустимый ключ библиотеки %s", paths.LogQ(r.Lib))
		}
		if r.Lib != "" && r.Path == "" && r.Alt == "" {
			continue // своё, которого нет в библиотеке этого компьютера
		}
		if r.Alt == "" && errors.Is(paths.CheckLocalPath(r.Path), paths.ErrDevice) {
			continue // недостающее с устройством в пути (как его оставляет Load): к файлу не обращаются
		}
		if err := checkPair("фото", r.Path, r.Alt); err != nil {
			return err
		}
	}
	for _, m := range p.Loaded() {
		if err := checkPair("отметка", m.Path, m.Alt()); err != nil {
			return err
		}
	}
	if p.Cover != nil && p.Cover.Guide != "" {
		if err := checkPair("разметка обложки", p.Cover.Guide, p.Cover.GuideAlt); err != nil {
			return err
		}
	}
	if p.SpreadGuide != "" {
		return checkPair("разметка разворотов", p.SpreadGuide, p.SpreadGuideAlt)
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
