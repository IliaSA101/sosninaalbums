package salbum

import (
	"errors"
	"fmt"
	"log"
	"math"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// FileLoose — добавление отдельных фото в файле (v16, 097): файлы записаны как папки (path
// относительный от альбома или абсолютный и запасной abs). At — число: прочтётся и «1e9», и
// дробное, чужое значение зажимается, а не делает файл «повреждённым».
type FileLoose struct {
	ID    string       `json:"id"`
	At    float64      `json:"at"`
	Files []FileFolder `json:"files"`
}

// maxAt — предел времени добавления: 9999 год (дальше — чужие данные).
const maxAt = 253402300799

// loadLoose — отдельные фото из файла (097 §3). Сырой список сверх 2·MaxLooseAdds добавлений и
// 2·MaxLooseFiles файлов не разбирается (stat на каждый файл); битая запись (путь, устройство,
// не JPEG/PNG) пропускается со строкой в лог без путей: добавления не держат вёрстку, отказ
// открыть альбом хуже (decisions 097-4). Пределы и повторы — FoldSources после.
func loadLoose(p *model.Project, list []FileLoose, base string, v1 bool) {
	var over, bad, devs int
	if len(list) > 2*model.MaxLooseAdds {
		over, list = len(list)-2*model.MaxLooseAdds, list[:2*model.MaxLooseAdds]
	}
	ids := map[string]bool{}
	budget := 2 * model.MaxLooseFiles
	for _, fl := range list {
		a := model.LooseAdd{ID: fl.ID, At: clampAt(fl.At)}
		if !model.ValidID(a.ID) || ids[a.ID] {
			a.ID = model.NewID()
		}
		ids[a.ID] = true
		for _, ff := range fl.Files {
			if budget == 0 {
				over++
				continue
			}
			budget--
			f, err := looseFile(ff, base, v1)
			switch {
			case errors.Is(err, paths.ErrDevice):
				devs++
			case err != nil:
				bad++
			default:
				a.Files = append(a.Files, f)
			}
		}
		p.Loose = append(p.Loose, a)
	}
	if over+bad+devs > 0 {
		log.Printf("отдельные фото: пропущено — с битым путём или не фото %d, с именем устройства %d, сверх предела %d", bad, devs, over)
	}
}

// clampAt — время добавления из файла: целые секунды в [0, maxAt].
func clampAt(at float64) int64 {
	if math.IsNaN(at) || at <= 0 {
		return 0
	}
	return int64(min(at, maxAt))
}

// looseFile — файл добавления: пара путей по правилам папок (candidates); основного нет, а
// запасной есть — запасной; нет нигде — основной и Alt (encode пишет его обратно).
func looseFile(ff FileFolder, base string, v1 bool) (model.LooseFile, error) {
	path, alt, err := candidates(ff.Path, ff.Abs, base, v1)
	if err != nil {
		return model.LooseFile{}, err
	}
	if !library.IsImage(path) {
		return model.LooseFile{}, fmt.Errorf("не фото")
	}
	if alt != "" && !isFile(path) {
		if isFile(alt) && library.IsImage(alt) {
			return model.LooseFile{Path: alt}, nil
		}
		return model.LooseFile{Path: path, Alt: alt}, nil
	}
	return model.LooseFile{Path: path}, nil
}

// encodeLoose — добавления для записи; пути — по enc, как у папок.
func encodeLoose(list []model.LooseAdd, enc pathEnc) []FileLoose {
	if len(list) == 0 {
		return nil
	}
	out := make([]FileLoose, 0, len(list))
	for _, a := range list {
		fl := FileLoose{ID: a.ID, At: float64(a.At), Files: make([]FileFolder, 0, len(a.Files))}
		for _, f := range a.Files {
			var ff FileFolder
			ff.Path, ff.Abs = enc(f.Path)
			if f.Alt != "" && !isFile(f.Path) {
				ff.Abs = f.Alt // файла нет и тут: запасной путь как был
			}
			fl.Files = append(fl.Files, ff)
		}
		out = append(out, fl)
	}
	return out
}

// checkLoose — пути и пределы отдельных фото для Check: копия из резервной (Restore) не протащит
// сетевой путь мимо Load.
func checkLoose(p *model.Project) error {
	if len(p.Loose) > model.MaxLooseAdds {
		return fmt.Errorf("отдельных добавлений в проекте больше %d", model.MaxLooseAdds)
	}
	n := 0
	for _, a := range p.Loose {
		for _, f := range a.Files {
			if err := checkPair("отдельное фото", f.Path, f.Alt); err != nil {
				return err
			}
		}
		n += len(a.Files)
	}
	if n > model.MaxLooseFiles {
		return fmt.Errorf("отдельных фото в проекте больше %d", model.MaxLooseFiles)
	}
	return nil
}

// isFile — обычный файл на отвечающем томе (как fileSize).
func isFile(path string) bool {
	_, ok := fileSize(path)
	return ok
}
