package decor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// importFrame — один файл рамочки (079, сц. 7): PNG или SVG, как стикер, но с проверкой окна.
// JPEG прозрачности не хранит — отказ; в середине нет прозрачного окна — отказ, копия удалена.
// job != nil — SVG чист и ждёт PNG от окна (проверка окна — в AddPNG). changed — реестр изменился.
func (s *Store) importFrame(theme, name, src string) (Item, *SvgJob, bool, error) {
	if paths.CheckLocalPath(src) != nil {
		return Item{}, nil, false, errors.New("файл не на локальном диске")
	}
	path, size, err := library.PlainFile(src, ".png", ".jpg", ".jpeg", ".svg")
	if errors.Is(err, library.ErrExt) {
		return Item{}, nil, false, errors.New("для рамочки подходят PNG и SVG")
	}
	if err != nil {
		return Item{}, nil, false, paths.NoPath(err)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".svg":
		return s.importSVG(KindFrame, theme, name, path, size)
	case ".jpg", ".jpeg":
		return Item{}, nil, false, errNeedPNG
	}
	if size > MaxFrameBytes {
		return Item{}, nil, false, fmt.Errorf("файл больше %d МБ", MaxFrameBytes>>20)
	}
	if err := imgfile.CheckSize(path); err != nil {
		return Item{}, nil, false, paths.NoPath(err)
	}
	dir := filepath.Join(s.root, kindDirs[KindFrame])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Item{}, nil, false, paths.NoPath(err)
	}
	tmp, key, err := copyHashed(path, dir, MaxFrameBytes)
	if err != nil {
		return Item{}, nil, false, err
	}
	defer func() { _ = os.Remove(tmp) }() // tmp меняется при уменьшении; после rename файла уже нет
	if s.alreadyHave(KindFrame, key) {
		return Item{}, nil, false, errHave
	}
	info, ext, err := checkCopy(tmp)
	if err != nil {
		return Item{}, nil, false, err
	}
	if ext != "png" { // расширение .png, а внутри JPEG
		return Item{}, nil, false, errNeedPNG
	}
	if max(info.W, info.H) > MaxFrameSide {
		small, sinfo, err := shrinkSticker(tmp, dir) // тот же предел стороны, что у стикера
		if err != nil {
			return Item{}, nil, false, err
		}
		_ = os.Remove(tmp)
		tmp, info = small, sinfo
	}
	win, err := readWindow(tmp) // окно — по копии, какая ляжет в библиотеку
	if err != nil {
		return Item{}, nil, false, err
	}
	it, changed, err := s.commit(KindFrame, theme, name, key, tmp, ext, info)
	if err == nil {
		s.frame.put(key, win)
	}
	return it, nil, changed, err
}
