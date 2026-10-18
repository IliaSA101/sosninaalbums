package salbum

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// pathEnc — путь из памяти для записи: path (относительный или абсолютный) и запасной abs ("" —
// не пишется).
type pathEnc func(path string) (string, string)

// encFor — правило путей записи. Обычное: внутри base (тот же том, не выше maxUp) —
// относительный с абсолютным рядом, иначе абсолютный; base == "" — все абсолютные. relOnly
// (сбор в папку, 069): внутри самой base — только относительный, без abs (папку переносят
// целиком, abs указал бы в никуда и выдал бы структуру дисков); вне base — только абсолютный:
// «..» после переноса папки привёл бы к чужому файлу.
func encFor(base string, relOnly bool) pathEnc {
	return func(path string) (string, string) {
		r, ok := RelPath(base, path)
		switch {
		case !ok:
			return path, ""
		case !relOnly:
			return r, path
		case r == ".." || strings.HasPrefix(r, "../"):
			return path, ""
		}
		return r, ""
	}
}

// Encode — файл проекта с отметками p.loaded (проект без Editor).
func Encode(p *model.Project, base string) ([]byte, error) {
	return encodeWith(p, p.Loaded(), encFor(base, false))
}

// encodeWith — файл проекта и свои отметки marks; пути — по enc.
func encodeWith(p *model.Project, marks map[string]model.Mark, enc pathEnc) ([]byte, error) {
	f := FileProject{
		Version:  model.FormatVersion,
		Product:  model.NormProduct(p.Product),
		Folders:  make([]FileFolder, 0, len(p.Folders)),
		Settings: p.Settings,
		Spreads:  p.Spreads,
		Photos:   make(map[string]FilePhoto, len(p.Photos)),
		Cover:    encodeCover(p.Cover, enc),
	}
	if f.Spreads == nil {
		f.Spreads = []*model.Spread{}
	}
	for _, dir := range p.Folders {
		var ff FileFolder
		ff.Path, ff.Abs = enc(dir)
		if alt := p.FolderAlt[dir]; alt != "" && !isDir(dir) {
			ff.Abs = alt // папки нет и тут: запасной путь как был
		}
		f.Folders = append(f.Folders, ff)
	}
	for id, ref := range p.Photos {
		fp := FilePhoto{PhotoRef: ref}
		fp.Alt = ""
		if ref.Lib != "" {
			fp.Path = "" // своё — по ключу: путей к библиотеке и личных путей в альбоме нет
			f.Photos[id] = fp
			continue
		}
		fp.Path, fp.Abs = enc(ref.Path)
		if ref.Alt != "" {
			fp.Abs = ref.Alt // фото не нашлось при открытии: запасной путь как был
		}
		f.Photos[id] = fp
	}
	f.SpreadGuide, f.SpreadGuideAbs = encodeGuide(p.SpreadGuide, p.SpreadGuideAlt, enc)
	f.Marks = encodeMarks(marks, enc)
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
	return writeEnc(p, path, marks, encFor(base, false))
}

// WriteRel — Write для собранной папки (069): пути внутри base — только относительные, без
// запасного abs; вне base (пропавшие фото) — абсолютные.
func WriteRel(p *model.Project, path, base string, marks map[string]model.Mark) error {
	if base == "" {
		return fmt.Errorf("запись с относительными путями без папки альбома")
	}
	return writeEnc(p, path, marks, encFor(base, true))
}

func writeEnc(p *model.Project, path string, marks map[string]model.Mark, enc pathEnc) error {
	data, err := encodeWith(p, marks, enc)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeAtomic(path, data)
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	// Заранее лежащий .tmp (жёсткая ссылка, чужой файл) убираем, а не пишем сквозь него:
	// иначе запись пошла бы в чужой файл по ссылке.
	if fi, err := os.Lstat(tmp); err == nil && fi.IsDir() {
		return fmt.Errorf("на месте временного файла %s лежит папка", tmp)
	}
	if err := os.Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := writeSynced(tmp, data); err != nil {
		_ = os.Remove(tmp) // уже есть ошибка записи; остаток .tmp безвреден
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp) // уже есть ошибка записи; остаток .tmp безвреден
		return err
	}
	return nil
}

// writeSynced пишет файл и сбрасывает его на диск до переименования: иначе после
// сбоя питания вместо проекта может остаться пустой файл. O_EXCL — не открывать
// чужой файл, подложенный на место path.
func writeSynced(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close() // уже есть ошибка записи
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close() // уже есть ошибка записи
		return err
	}
	return f.Close()
}

// backupPath — копия файла старой версии: <путь>.v<from>.bak (from — версия на чтении).
func backupPath(path string, from int) string { return fmt.Sprintf("%s.v%d.bak", path, from) }

// BackupOld копирует файл старой версии перед первой записью поверх него, чтобы
// можно было вернуться на прежнюю программу. Копия уже есть — не трогаем;
// исходника нет или он уже новой версии — копировать нечего.
func BackupOld(path string, from int) error {
	bak := backupPath(path, from)
	_, err := os.Lstat(bak)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	data, err := ReadLimited(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var head struct {
		Version int `json:"version"`
	}
	if json.Unmarshal(data, &head) == nil && head.Version >= model.FormatVersion {
		return nil
	}
	return writeAtomic(bak, data)
}
