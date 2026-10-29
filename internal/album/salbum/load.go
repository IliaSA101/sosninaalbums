package salbum

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// maxUp — сколько «..» допускает относительный путь: альбом в Ивановы\Альбом\,
// фото в Ивановы\Отбор\ — один уровень; два — запас. Дальше пишем абсолютный.
const maxUp = 2

// Файловые типы: в памяти пути абсолютные, относительные живут только здесь.
type FileProject struct {
	Version int `json:"version"`
	// App — версия программы, записавшей файл (v17, 098 design §6): справочно, в Project не идёт.
	App appField `json:"app,omitempty"`
	// UID — отпечаток проекта (146, model/uid.go): без смены номера формата; негодный — нет.
	UID string `json:"uid,omitempty"`
	// UIDAt — печать отпечатка к пути, где он выдан (model.UIDTag): у копии файла в другом пути
	// запись о ссылке пути чужая. Без смены номера формата; негодная — нет.
	UIDAt    string               `json:"uidAt,omitempty"`
	Product  string               `json:"product"`
	Folders  []FileFolder         `json:"folders"`
	Settings model.Settings       `json:"settings"`
	Spreads  []*model.Spread      `json:"spreads"`
	Photos   map[string]FilePhoto `json:"photos"`
	Marks    markList             `json:"marks,omitempty"` // свои отметки (v5, 004)
	Bins     binList              `json:"bins,omitempty"`  // подборки фото (v15, 083)
	// Fonts — свои гарнитуры текстов (v15, 082): id → название, без путей к файлам.
	Fonts map[string]model.FontRef `json:"fonts,omitempty"`
	// Vars — поля альбома из dev-сборок v6 (030): только чтение, Load переносит их в листы
	// (moveAlbumVars), Save не пишет — поля шапки живут в Spread.Vars (решение 125).
	Vars  *albumVars `json:"vars,omitempty"`
	Cover *FileCover `json:"cover,omitempty"` // обложка (v6, 032; cover.go)
	// SpreadGuide — файл-разметка разворотов (v11, 066): относительный от папки альбома,
	// SpreadGuideAbs — запасной абсолютный (как guide/guideAbs обложки).
	SpreadGuide    string `json:"spreadGuide,omitempty"`
	SpreadGuideAbs string `json:"spreadGuideAbs,omitempty"`
	// Loose — отдельные фото по добавлениям (v16, 097; loose.go): файл записан как папка.
	Loose []FileLoose `json:"loose,omitempty"`
	// AlbumBg — фон альбома (v17, 098 §4; фото — ID записи photos).
	AlbumBg *model.AlbumBg `json:"albumBg,omitempty"`
	// Look — макет альбома (v19, 131 design §3): ключ, имя, набор и запись; зажим — FixLook.
	Look *model.LookRef `json:"look,omitempty"`
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
// ни трогать (старая версия молча выбросила бы незнакомые поля). Проверять через errors.Is:
// Load отдаёт *NewerError с версией записавшей программы (appver.go).
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

// fileHead кладёт в p шапку файла: версию, изделие, отпечаток и его печать (146 T4-9: чужие
// данные, негодные — нет). Печать с путём альбома сверяет Editor при выгрузке и показе ссылки.
func fileHead(p *model.Project, f *FileProject) {
	p.SetFrom(max(f.Version, 1))
	p.Product = model.NormProduct(f.Product)
	if !model.GoodUID(f.UID) {
		return
	}
	p.UID = f.UID
	if model.GoodUIDTag(f.UIDAt) {
		p.SetUIDAt(f.UIDAt)
	}
}

// decodeFile разбирает файл: пути (с проверкой и выбором из двух), повторы,
// ID по путям, лимиты, миграция старой версии. base — папка файла.
// Порядок шагов сетки (design 007 §2): nil и объединение мест с Photos — до
// всего, что трогает места; починка мест — последней, по зажатой геометрии.
func decodeFile(data []byte, base string) (*model.Project, error) {
	if err := checkHead(data); err != nil {
		return nil, err
	}
	f := FileProject{Settings: model.DefaultSettings()}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("файл проекта повреждён: %w", err)
	}
	if err := checkRaw(&f); err != nil {
		return nil, err
	}
	p := model.New()
	fileHead(p, &f)
	// foldLegacy — до всех шагов, что считают «Авто»: они — по прежнему поведению (064).
	p.Settings, p.Spreads = foldLegacy(p.From(), f.Settings), f.Spreads
	for _, s := range p.Spreads {
		if s == nil {
			return nil, fmt.Errorf("файл проекта повреждён: пустой разворот")
		}
	}
	v1 := p.From() < 2
	if err := loadSources(p, &f, base, v1); err != nil {
		return nil, err
	}
	fixCoverAndGuide(p, &f, base, v1)
	if err := loadPhotos(p, f.Photos, base, v1); err != nil {
		return nil, err
	}
	dropNilPlaces(p) // до FixLook в loadAlbumWide: место null не держит шаг у листа, который иначе пуст
	loadAlbumWide(p, &f)
	unionGrid(p)
	dedupe(p)
	p.PruneRefs()
	Rekey(p, library.IDFor)
	EvenFamilies(p)
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
	p.PruneRefs() // запись рамочки, снятой зажимом (079: вид и картинка сразу), не остаётся без дела
	p.FixLook()   // ещё раз: зажимы выше могли снять места и фон листа с шагом
	migrate(p)
	if err := p.SyncGrid(true); err != nil {
		return nil, err
	}
	coverNoAuto(p)
	p.Fonts = model.ClampFonts(f.Fonts, p.UsedFonts())
	p.FitTexts()
	p.SyncFonts() // после FitTexts, как в Editor: таблица — по стилям, имена — из реестра
	p.FitStickers()
	p.LoadOrder()
	bins := decodeBins(f.Bins)
	p.SetLoaded(model.Side{Marks: decodeMarks(f.Marks, base, p.Folders, p.LooseFiles(), model.BinSet(bins)), Bins: bins})
	return p, nil
}

// loadAlbumWide — поля всего альбома, которые нужны до dedupe. Фон альбома — до первого PruneRefs
// (098-21): иначе запись его фото, не стоящего на листах, удалилась бы при чтении. Макет (131):
// лист с одним номером шага чужого или битого макета пуст (Empty) — dedupe его уберёт.
func loadAlbumWide(p *model.Project, f *FileProject) {
	p.AlbumBg = f.AlbumBg
	p.ClampAlbumBg()
	p.Look = f.Look
	p.FixLook()
}

// checkHead — версия до полного разбора (075-12): файл новее программы мог сменить вид поля, и
// разбор в структуру этой версии сказал бы «повреждён» вместо «сохранён более новой версией». В
// «голове» и app (098): текст ошибки называет версию, в которой файл сохранён.
func checkHead(data []byte) error {
	var head struct {
		Version int      `json:"version"`
		App     appField `json:"app"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return fmt.Errorf("файл проекта повреждён: %w", err)
	}
	if head.Version > model.FormatVersion {
		return &NewerError{Version: head.Version, App: string(head.App)}
	}
	return nil
}

// loadSources — папки и отдельные фото из файла. Источники не пересекаются (093-4): любая версия,
// и файл, поправленный руками.
func loadSources(p *model.Project, f *FileProject, base string, v1 bool) error {
	if err := loadFolders(p, f.Folders, base, v1); err != nil {
		return err
	}
	loadLoose(p, f.Loose, base, v1)
	if merged, _ := p.FoldSources(paths.Linked); merged > 0 {
		log.Printf("папки проекта: вложенных слито %d", merged)
	}
	return nil
}

// loadPhotos — фото из файла. Устройство в пути на этой ОС (paths.ErrDevice, 073-10) — фото
// недостающее, как ненайденный файл: путь остаётся, к файлу не обращаемся. Записи с теми же
// путями и размерами (копии одного файла, 098) разбираются один раз: stat не множится на
// сетевом томе.
func loadPhotos(p *model.Project, list map[string]FilePhoto, base string, v1 bool) error {
	devs := 0
	type fileKey struct {
		path, abs string
		size      int64
		w, h      int
	}
	type resolved struct {
		ref model.PhotoRef
		err error
	}
	seen := map[fileKey]resolved{}
	for id, fp := range list {
		if fp.Lib != "" { // своё из библиотеки: путь — от искателя (lib.go)
			if ref, ok := resolveLib(fp); ok {
				p.Photos[id] = ref
			}
			continue
		}
		key := fileKey{fp.Path, fp.Abs, fp.Size, fp.W, fp.H}
		r, ok := seen[key]
		if !ok {
			r.ref, r.err = resolvePhoto(fp, base, v1)
			seen[key] = r
		}
		ref, err := fp.PhotoRef, r.err
		ref.Path, ref.Alt, ref.W, ref.H, ref.Size = r.ref.Path, r.ref.Alt, r.ref.W, r.ref.H, r.ref.Size
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

// checkRaw — первые шаги по разобранному файлу: сверх MaxSpreads — отказ (до dedupe/splitBig не
// гонять огромный файл), хвост мест — capRawGrid. Версию decodeFile проверила ещё до разбора.
func checkRaw(f *FileProject) error {
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
