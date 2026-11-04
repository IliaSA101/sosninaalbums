package decor

import (
	"errors"
	"image"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Рамочки (079, design §6): встроенные — пресеты presets_frame.go, свои и наборные — PNG с
// прозрачным окном в Рамочки/. Окно (срез и open) считается при импорте и установке набора, и
// лениво при первом Frame; в library.json его нет (§0). Нужна мета только импорту (проверка
// окна) и Frame для ApplyBorder: экран и печать берут срез из альбома (079-22).

// Пределы рамочки-картинки — как у стикера (design §6).
const (
	MaxFrameBytes = 30 << 20
	MaxFrameSide  = 4096
)

// errNoWindow — окна нет: центр непрозрачен или оно меньше 5 % площади (сц. 7).
var errNoWindow = errors.New("в середине нет прозрачного окна — фото под рамочкой не будет видно")

// errNeedPNG — JPEG и другой формат без альфы (сц. 7, 079-18).
var errNeedPNG = errors.New("нужен PNG с прозрачностью")

// winInfo — окно картинки-рамочки.
type winInfo struct {
	cut  [4]float64
	open bool
}

// frameMeta — окна рамочек по ключу библиотеки; ключ — хэш байт, значение не стареет.
type frameMeta struct {
	mu sync.Mutex
	m  map[string]winInfo
}

func (c *frameMeta) get(key string) (winInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	w, ok := c.m[key]
	return w, ok
}

func (c *frameMeta) put(key string, w winInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]winInfo{}
	}
	c.m[key] = w
}

// windowOf — окно картинки img (gen.FrameWindow); окна нет — errNoWindow.
func windowOf(img image.Image) (winInfo, error) {
	cut, open, ok := gen.FrameWindow(img)
	if !ok {
		return winInfo{}, errNoWindow
	}
	return winInfo{cut: cut, open: open}, nil
}

// readWindow декодирует файл (imgfile: пределы, бюджет памяти) и ищет окно.
func readWindow(path string) (winInfo, error) {
	img, release, err := imgfile.OpenOriented(path)
	if err != nil {
		return winInfo{}, errors.New("файл не читается как PNG")
	}
	defer release()
	return windowOf(img)
}

// FrameSrc — что ставит щелчок по плитке рамочки (ApplyBorder в App): встроенная — вид, цвета,
// толщина и отметка плитки; своя — запись фото с ключом библиотеки, срез и open.
type FrameSrc struct {
	Kind   string
	Inks   []string
	Width  float64
	Preset string          // ID плитки sa:fr-…: отметка в альбоме (как Pattern.Preset)
	Photo  model.PhotoInfo // своя: ID = LibPhotoID(Lib), Path, Lib, W, H, Size
	Cut    [4]float64
	Open   bool
}

// Frame — разбор ID плитки рамочки; путь своей — из индекса, не от интерфейса. Окно считается
// лениво, если его нет в памяти (после запуска программы); окна нет — ошибка с причиной.
func (s *Store) Frame(id string) (FrameSrc, error) {
	if p, ok := findFramePreset(id); ok {
		if !gen.FrameKnown(p.kind) {
			return FrameSrc{}, ErrNotFound
		}
		return FrameSrc{Kind: p.kind, Inks: append([]string(nil), p.inks...), Width: p.width, Preset: p.id}, nil
	}
	if !model.ValidLibKey(id) {
		return FrameSrc{}, ErrNotFound
	}
	s.mu.RLock()
	e := s.entryOf(KindFrame, id)
	path, ok := s.files[KindFrame][id]
	ok = ok && (e != nil || s.inPack(KindFrame, id))
	s.mu.RUnlock()
	if !ok {
		return FrameSrc{}, ErrNotFound
	}
	info, err := library.ReadInfo(path)
	if err != nil || info.W <= 0 || info.H <= 0 {
		return FrameSrc{}, ErrNotFound
	}
	w, have := s.frame.get(id)
	if !have {
		if w, err = readWindow(path); err != nil {
			return FrameSrc{}, err
		}
		s.frame.put(id, w)
	}
	return FrameSrc{
		Photo: model.PhotoInfo{ID: model.LibPhotoID(id), Path: path, Lib: id, W: info.W, H: info.H, Size: info.Size},
		Cut:   w.cut, Open: w.open,
	}, nil
}
