package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
)

// NoSpaceError — на диске мало места: нужно Need МБ (в два раза больше установщика — файл и
// запас на распаковку).
type NoSpaceError struct{ Need int64 }

func (e *NoSpaceError) Error() string {
	return fmt.Sprintf("мало места на диске для обновления (нужно %d МБ)", e.Need)
}

// Dir — папка скачанных обновлений: <кэш пользователя>\<имя данных>\Updates (Windows:
// %LOCALAPPDATA%\SosninaAlbums\Updates; у стенда своя). name — osx.DataName. "" — кэша нет.
func Dir(name string) string {
	base := osx.CacheDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, name, "Updates")
}

// Verify — файл path сходится с записью f: обычный файл (не ссылка), размер и sha256.
// Любое расхождение — ErrVerify (причина — в Detail).
func Verify(path string, f File) error {
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() {
		return verifyFail("не обычный файл")
	}
	if st.Size() != f.Size {
		return verifyFail("длина файла не совпала")
	}
	r, err := os.Open(path)
	if err != nil {
		return verifyFail("файл не открылся")
	}
	defer r.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	h := sha256.New()
	if n, err := io.Copy(h, io.LimitReader(r, f.Size+1)); err != nil || n != f.Size {
		return verifyFail("файл не прочитался до конца")
	}
	if hex.EncodeToString(h.Sum(nil)) != f.SHA256 {
		return verifyFail("sha256 не совпал")
	}
	return nil
}

// verifyFail — ErrVerify с причиной для log.txt.
func verifyFail(why string) error {
	return &humanError{human: ErrVerify, cause: errors.New(why)}
}

// SetupPath — где лежит (или будет лежать) установщик версии ver в папке dir; "" — ver негодна.
func SetupPath(dir, ver string) string {
	name := SetupName(ver)
	if name == "" {
		return ""
	}
	return filepath.Join(dir, name)
}

// Fetch — установщик предложения o в папке dir; итог — путь к проверенному файлу. Уже скачанный
// и сверенный берётся сразу (progress получает полный размер). Иначе: место (free(dir) ≥
// 2 × размер; nil — osx.FreeBytes) → скачивание в «<имя>.part» (fetch: длина, застой, sha256
// по дороге, .part удаляется при ошибке) → переименование. Имя и адрес строятся из версии
// (SetupName), строка из манифеста лишь сверяется.
func Fetch(ctx context.Context, c *Client, o Offer, dir string, free func(string) (uint64, error), progress func(done int64)) (string, error) {
	name := SetupName(o.Version)
	if o.Setup == nil || name == "" || o.Setup.Name != name {
		return "", ErrNoInstaller
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", ErrDiskWrite
	}
	dst := filepath.Join(dir, name)
	if Verify(dst, *o.Setup) == nil {
		if progress != nil {
			progress(o.Setup.Size)
		}
		return dst, nil
	}
	_ = os.Remove(dst) // сторонний или битый — не оставляем
	if free == nil {
		free = osx.FreeBytes
	}
	need := o.Setup.Size * 2
	if n, err := free(dir); err == nil && n < uint64(need) { //nolint:gosec // размер из Parse: 1…80 МБ
		return "", &NoSpaceError{Need: (need + 1<<20 - 1) >> 20}
	}
	part := dst + ".part"
	e := fetch.Entry{Title: name, Size: o.Setup.Size, SHA256: o.Setup.SHA256, URL: c.setupURL(o.Version)}
	if err := c.fc.Download(ctx, e, part, progress); err != nil {
		return "", humanize(err)
	}
	if err := os.Rename(part, dst); err != nil {
		_ = os.Remove(part)
		return "", ErrDiskWrite
	}
	return dst, nil
}

// Clean — убирает в dir только свои имена: недокачанные «*.part» установщика и установщики версий
// не новее cur (уже стоят). Чужие файлы, папки и ссылки не трогаются. cur не версия — только .part.
func Clean(dir, cur string) {
	list, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range list {
		if e.Type().IsRegular() && ownName(e.Name(), cur) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// ownName — имя нашего устаревшего файла: «SosninaAlbums-<версия>-windows-x64-setup.exe[.part]».
func ownName(name, cur string) bool {
	base, part := strings.CutSuffix(name, ".part")
	rest, ok := strings.CutPrefix(base, "SosninaAlbums-")
	if !ok {
		return false
	}
	ver, ok := strings.CutSuffix(rest, "-windows-x64-setup.exe")
	if !ok || SetupName(ver) != base {
		return false
	}
	if part {
		return true
	}
	_, curOK := ParseVersion(cur)
	return curOK && !Newer(ver, cur)
}
