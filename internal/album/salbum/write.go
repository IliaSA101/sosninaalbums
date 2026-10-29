package salbum

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
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

// Encode — файл проекта с отметками и подборками p.loaded (проект без Editor).
func Encode(p *model.Project, base string) ([]byte, error) {
	return encodeWith(p, p.Loaded(), encFor(base, false))
}

// encodeWith — файл проекта, свои отметки и подборки side; пути — по enc. Печать отпечатка —
// p.UIDAt() как есть: ставит её только выдача отпечатка (album/editor_uid.go, collect.Save), запись
// по другому пути её не меняет (146 T4-9).
func encodeWith(p *model.Project, side model.Side, enc pathEnc) ([]byte, error) {
	f := FileProject{
		Version:  model.FormatVersion,
		App:      appField(curApp()),
		UID:      p.UID,
		UIDAt:    sealOf(p),
		Product:  model.NormProduct(p.Product),
		Folders:  make([]FileFolder, 0, len(p.Folders)),
		Settings: p.Settings,
		Spreads:  p.Spreads,
		Photos:   make(map[string]FilePhoto, len(p.Photos)),
		Cover:    encodeCover(p.Cover, enc),
		Fonts:    p.Fonts, // только названия: файлы шрифтов альбом не несёт (082-8)
		AlbumBg:  p.AlbumBg,
		Look:     p.Look,
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
	f.Loose = encodeLoose(p.Loose, enc)
	f.Bins = slices.Clone(side.Bins)
	f.Marks = encodeMarks(side.Marks, model.BinSet(side.Bins), enc)
	return json.MarshalIndent(f, "", "  ")
}

// Save пишет проект; относительные пути считаются от папки файла.
func Save(p *model.Project, path string) error { return SaveWith(p, path, p.Loaded()) }

// SaveWith — Save со своими отметками и подборками side (Editor держит их отдельно от проекта).
func SaveWith(p *model.Project, path string, side model.Side) error {
	base, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return err
	}
	return Write(p, path, base, side)
}

// Write — запись через временный файл, чтобы сбой не испортил проект.
// base == "" — все пути абсолютные (черновик).
func Write(p *model.Project, path, base string, side model.Side) error {
	return writeEnc(p, path, side, encFor(base, false))
}

// WriteRel — Write для собранной папки (069): пути внутри base — только относительные, без
// запасного abs; вне base (пропавшие фото) — абсолютные.
func WriteRel(p *model.Project, path, base string, side model.Side) error {
	if base == "" {
		return fmt.Errorf("запись с относительными путями без папки альбома")
	}
	return writeEnc(p, path, side, encFor(base, true))
}

// sealOf — печать для записи: только при годных отпечатке и печати (без отпечатка она ничья).
func sealOf(p *model.Project) string {
	if !model.GoodUID(p.UID) || !model.GoodUIDTag(p.UIDAt()) {
		return ""
	}
	return p.UIDAt()
}

func writeEnc(p *model.Project, path string, side model.Side, enc pathEnc) error {
	data, err := encodeWith(p, side, enc)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeAtomic(path, data)
}

// writeAtomic пишет через случайное <путь>.*.tmp (O_EXCL, Sync, Rename): постоянное <путь>.tmp
// на Windows временно держит чужой процесс (антивирус) — и запись молча не шла; подложенные
// на постоянное имя ссылка или папка тоже больше не мешают.
func writeAtomic(path string, data []byte) error { return library.WriteFileAtomic(path, data) }

// maxBackupNo — сколько имён копии пробовать: « 2» … « 99», дальше — ошибка записи.
const maxBackupNo = 99

// BackupOld копирует файл старой версии перед первой записью поверх него, чтобы можно было
// вернуться на прежнюю программу: «<имя> (для 0.22).salbum» рядом (BackupName, design §6).
// Ответ — имя сделанной копии; "" — копировать нечего: исходника нет, он уже текущей версии
// или копия с теми же байтами уже лежит. Имя совпало с самим файлом (открыли копию) — дальше;
// занято другим файлом — « 2», « 3»… (чужой файл не трогаем); все заняты — ошибка, и запись
// поверх исходника не идёт.
func BackupOld(path string, from int) (string, error) {
	data, err := ReadLimited(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var head struct {
		Version int `json:"version"`
	}
	if json.Unmarshal(data, &head) == nil && head.Version >= model.FormatVersion {
		return "", nil
	}
	for k := 1; k <= maxBackupNo; k++ {
		bak := BackupName(path, from, k)
		if paths.SamePath(bak, path) {
			continue
		}
		switch busy, err := sameBytes(bak, data); {
		case err != nil:
			return "", err
		case busy == bakFree:
			return bak, writeAtomic(bak, data)
		case busy == bakSame:
			return "", nil
		}
	}
	return "", fmt.Errorf("не удалось сохранить копию для версии %s: все имена заняты", LastApp(from))
}

// bakState — что лежит на месте имени копии.
type bakState int

const (
	bakFree  bakState = iota // ничего нет
	bakSame                  // файл с теми же байтами: копия уже сделана
	bakOther                 // другое (чужой файл, папка, ссылка) — не трогаем
)

// sameBytes — состояние имени bak против байтов исходника data. Ссылку не разыменовываем
// (Lstat): подложенная ссылка — «чужое».
func sameBytes(bak string, data []byte) (bakState, error) {
	st, err := os.Lstat(bak)
	if errors.Is(err, fs.ErrNotExist) {
		return bakFree, nil
	}
	if err != nil {
		return bakOther, err
	}
	if !st.Mode().IsRegular() || st.Size() != int64(len(data)) {
		return bakOther, nil
	}
	got, err := ReadLimited(bak)
	if err != nil {
		return bakOther, err
	}
	if bytes.Equal(got, data) {
		return bakSame, nil
	}
	return bakOther, nil
}
