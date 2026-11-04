package salbum

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

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
func resolvePhoto(d Disk, fp FilePhoto, base string, v1 bool) (model.PhotoRef, error) {
	ref, err := resolvePaths(d, fp, base, v1)
	switch {
	case err == nil:
		fixDims(d, &ref, fp.Size <= 0)
	case errors.Is(err, paths.ErrDevice):
		fixDims(d, &ref, false) // без чтения файла: открыть путь — обратиться к устройству
	}
	return ref, err
}

// maxAspect — предел пропорции кадра из файла. Настоящие панорамы не уже 1:20;
// запас большой, а подделанные 4000×10 (раздувают окно фона при экспорте) отсекаются.
const maxAspect = 100

// fixDims: размеры из .salbum — чужие данные. Без размера файла в записи (unverified), больше
// MaxPixels или с нелепой пропорцией они перечитываются из заголовка файла;
// если файла нет или он не читается — обнуляются (нулевые размеры раскладка терпит).
func fixDims(d Disk, ref *model.PhotoRef, unverified bool) {
	w, h := int64(ref.W), int64(ref.H)
	sane := w > 0 && h > 0 && w*h <= imgfile.MaxPixels && w <= maxAspect*h && h <= maxAspect*w
	if sane && !unverified {
		return
	}
	if rw, rh, ok := probeDimsOn(d, ref.Path); ok {
		ref.W, ref.H = rw, rh
		return
	}
	if !sane {
		ref.W, ref.H = 0, 0
	}
}

func resolvePaths(d Disk, fp FilePhoto, base string, v1 bool) (model.PhotoRef, error) {
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
	size, ok := fileSizeOn(d, primary)
	if ok && (ref.Size == 0 || size == ref.Size) {
		ref.Size = size
		return ref, nil
	}
	if alt != "" {
		if asize, aok := fileSizeOn(d, alt); aok {
			ref.Path = alt
			refresh(d, &ref, asize)
			return ref, nil
		}
	}
	if ok {
		refresh(d, &ref, size)
		return ref, nil
	}
	ref.Alt = alt
	return ref, nil
}

// refresh: файл на месте, но другой длины — фото пересохранили; размеры в
// пикселях читаются из заголовка заново.
func refresh(d Disk, ref *model.PhotoRef, size int64) {
	if ref.Size != 0 && ref.Size != size {
		if w, h, ok := probeDimsOn(d, ref.Path); ok {
			ref.W, ref.H = w, h
		}
	}
	ref.Size = size
}

// probeDims — probeDimsOn на настоящем диске (RelinkLib и прочее вне разбора файла).
func probeDims(path string) (w, h int, ok bool) { return probeDimsOn(osDisk{}, path) }

// reachable — отвечает ли том пути (osx.Reachable); подменяется в тестах. Мёртвый сетевой том
// для Load — то же, что «файла нет»: пути и Alt остаются, размеры не обнуляются, а окно не ждёт
// таймаута SMB на каждом фото.
var reachable = osx.Reachable

// fileSize — размер обычного файла path на настоящем диске (fileSizeOn): ссылка — только с
// локальной целью, по ссылке на сетевую шару Load не ходит.
func fileSize(path string) (int64, bool) { return fileSizeOn(osDisk{}, path) }

// isDir — path — папка на настоящем диске (ссылка — как fileSize).
func isDir(path string) bool { return isDirOn(osDisk{}, path) }
