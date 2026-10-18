package salbum

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

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
