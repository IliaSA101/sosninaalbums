package collect

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// ErrDiskFull — диск папки сбора кончился посреди копирования (проверять errors.Is).
var ErrDiskFull = errors.New("диск заполнен")

// Reserve — запас места сверх файлов: альбом, резервная копия, служебное ФС.
const Reserve = 16 << 20

// tmpSuffix — хвост своих временных файлов: только их сбор удаляет (обрыв прошлого сбора).
const tmpSuffix = ".sacopy.tmp"

// bufSize — кусок копирования: между кусками проверяется отмена.
const bufSize = 1 << 20

// progressEvery — как часто звать ход (и обязательно в конце).
const progressEvery = 200 * time.Millisecond

// Progress — ход копирования: файлов и байт скопировано из всего к копированию (без Skip).
type Progress func(doneFiles, totalFiles int, doneBytes, totalBytes int64)

// Подмены для тестов: свободное место, запись во временный файл, часы хода, переименование.
var (
	freeBytes = osx.FreeBytes
	newWriter = func(f *os.File) io.Writer { return struct{ io.Writer }{f} } // без ReaderFrom: кусками, с отменой
	now       = time.Now
	// syncClose — сброс на диск и закрытие временного: здесь всплывает нехватка места у
	// отложенной записи. Close и при ошибке Sync — дескриптор не течёт.
	syncClose = func(f *os.File) error { return errors.Join(f.Sync(), f.Close()) }
	// renameNoReplace — переименование временного без замены чужого файла.
	renameNoReplace = osx.RenameNoReplace
)

// CheckSpace — хватает ли места на томе папки dir (её может ещё не быть: смотрим ближайшую
// существующую папку выше) под need байт и Reserve.
func CheckSpace(dir string, need int64) error {
	at := dir
	// Нет и корня тома — спросим про сам dir: FreeBytes вернёт ошибку.
	for st, err := os.Stat(at); (err != nil || !st.IsDir()) && filepath.Dir(at) != at; st, err = os.Stat(at) {
		at = filepath.Dir(at)
	}
	free, err := freeBytes(at)
	if err != nil {
		return fmt.Errorf("не удалось узнать свободное место: %w", err)
	}
	want := max(need, 0) + Reserve
	have := int64(math.MaxInt64)
	if free < math.MaxInt64 {
		have = int64(free)
	}
	if have >= want {
		return nil
	}
	disk := "на диске"
	if vol := filepath.VolumeName(dir); vol != "" {
		disk = "на диске " + vol
	}
	return fmt.Errorf("не хватает места %s: нужно %s, свободно %s", disk, SizeText(want), SizeText(have))
}

// Copy копирует файлы плана pl (кроме Skip) в папку сбора dir. Пишет только под своими именами и
// только если имени ещё нет: временный файл (O_EXCL) → переименование без замены. Имя заняли
// после плана — файл ложится под следующим « (N)» (Rel в pl меняется, сайдкар идёт следом),
// сайдкар с занятым именем не копируется (SideBusy, из Items убирается). Отмена ctx — между
// кусками: недокопированный временный удаляется, скопированные остаются (повтор их пропустит).
// copied — сколько файлов скопировано. Ошибка нехватки места — ErrDiskFull.
func Copy(ctx context.Context, dir string, pl *Plan, step Progress) (copied int, err error) {
	if err := prepareDirs(dir, *pl); err != nil {
		return 0, err
	}
	// Убранные сайдкары помечены пустым Rel: убрать и при ошибке — итог считает по Items.
	defer func() { pl.Items = slices.DeleteFunc(pl.Items, func(it Item) bool { return it.Rel == "" }) }()
	moved := map[string]string{} // nameKey прежнего Rel (и его же без расширения) → новый
	var total int
	for _, it := range pl.Items {
		if !it.Skip {
			total++
		}
	}
	var doneBytes int64
	last := time.Time{}
	report := func(force bool) {
		if step == nil {
			return
		}
		if t := now(); force || t.Sub(last) >= progressEvery {
			last = t
			step(copied, total, doneBytes, pl.Bytes)
		}
	}
	report(true)
	for i := range pl.Items {
		it := &pl.Items[i]
		if it.Skip {
			continue
		}
		if err := ctx.Err(); err != nil {
			return copied, err
		}
		if it.Kind == KindXMP {
			it.Rel = follow(it.Rel, moved)
		}
		rel, err := copyOne(ctx, dir, *it, func(n int64) { doneBytes += n; report(false) })
		switch {
		case errors.Is(err, errBusy):
			pl.SideBusy++
			it.Rel = ""
			continue
		case err != nil:
			return copied, err
		}
		if rel != it.Rel {
			moved[nameKey(it.Rel)] = rel
			moved[nameKey(stripExt(it.Rel))] = stripExt(rel)
			it.Rel = rel
		}
		copied++
		report(false)
	}
	report(true)
	return copied, nil
}

// prepareDirs — папка сбора и каталоги плана: каждое звено от папки сбора — настоящий каталог
// (Lstat: через ссылку или junction не пишем); остатки своих временных файлов прошлого обрыва
// удаляются.
func prepareDirs(dir string, pl Plan) error {
	if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		_ = os.MkdirAll(dir, 0o755) // не создалась — скажет isDir
	}
	if err := isDir(dir); err != nil {
		return err
	}
	done := map[string]bool{"": true}
	for _, it := range pl.Items {
		if !relLocal(it.Rel) {
			return fmt.Errorf("недопустимый путь в плане сбора %q", it.Rel)
		}
		parent := path.Dir(it.Rel)
		if parent == "." || done[nameKey(parent)] {
			continue
		}
		cur := dir
		for _, el := range strings.Split(parent, "/") {
			cur = filepath.Join(cur, el)
			_ = os.Mkdir(cur, 0o755) // уже есть или не создалась — разберёт isDir (Lstat)
			if err := isDir(cur); err != nil {
				return err
			}
		}
		done[nameKey(parent)] = true
		cleanTmp(cur)
	}
	return nil
}

// isDir — путь — настоящий каталог, не ссылка и не junction.
func isDir(p string) error {
	st, err := os.Lstat(p)
	if err != nil {
		return paths.NoPath(err)
	}
	if st.Mode().Type() != fs.ModeDir {
		return fmt.Errorf("на месте папки «%s» — файл или ссылка: выберите другую папку", filepath.Base(p))
	}
	return nil
}

// cleanTmp удаляет свои временные файлы обрыва в каталоге (только обычные файлы с tmpSuffix).
func cleanTmp(dir string) {
	ents, _ := os.ReadDir(dir) // не читается — убирать нечего: запись сама скажет об ошибке
	for _, e := range ents {
		if e.Type().IsRegular() && strings.HasSuffix(lower(e.Name()), tmpSuffix) {
			_ = os.Remove(filepath.Join(dir, e.Name())) // не удалился — не помеха: имя временного с pid
		}
	}
}

// errBusy — имя сайдкара в папке сбора заняли после плана: под другим именем он не нужен
// (программы ищут его по имени фото).
var errBusy = errors.New("имя сайдкара занято")

// copyOne — один файл через временный рядом; rel — под каким именем он лёг.
func copyOne(ctx context.Context, dir string, it Item, onBytes func(int64)) (rel string, err error) {
	final := filepath.Join(dir, filepath.FromSlash(it.Rel))
	name := path.Base(it.Rel)
	if err := chain(dir, it.Rel); err != nil {
		return "", err
	}
	if !reachable(it.Src) {
		return "", fmt.Errorf("диск с файлом «%s» не отвечает", name)
	}
	in, err := os.Open(it.Src)
	if err != nil {
		return "", fmt.Errorf("не открылся файл «%s»: %w", name, paths.NoPath(err))
	}
	rd := &ctxReader{ctx: ctx, r: in, on: onBytes}
	defer rd.release(in)
	st, err := in.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() != it.Size {
		return "", fmt.Errorf("файл «%s» изменился во время сбора", name)
	}
	tmp := final + "." + strconv.Itoa(os.Getpid()) + tmpSuffix
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", diskFull(err)
	}
	defer func() {
		if err != nil {
			_ = out.Close()    // уже есть ошибка
			_ = os.Remove(tmp) // свой временный; не удалился — уберёт следующий сбор
		}
	}()
	n, err := io.CopyBuffer(newWriter(out), rd, make([]byte, bufSize))
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", diskFull(err)
	}
	if n != it.Size {
		return "", fmt.Errorf("файл «%s» изменился во время сбора", name)
	}
	if err = syncClose(out); err != nil {
		return "", diskFull(err)
	}
	// Копирование долгое: звено папки могли подменить ссылкой — через неё не переименовываем.
	if err = chain(dir, it.Rel); err != nil {
		return "", err
	}
	if rel, err = place(dir, tmp, it); err == nil {
		// Время источника — для «уже на месте» при следующем сборе; не встало — файл
		// скопируется ещё раз под « (2)», данные целы.
		_ = os.Chtimes(filepath.Join(dir, filepath.FromSlash(rel)), time.Now(), st.ModTime())
	}
	return rel, err
}

// place переименовывает временный в имя плана, а занятое (чужой файл появился после плана,
// или ФС не различает имена, которые различил план) — в следующее « (N)». Без замены: Rename
// на Windows заменил бы чужой файл, появившийся между проверкой и переименованием.
func place(dir, tmp string, it Item) (string, error) {
	for n := 1; n <= maxDup; n++ {
		rel := dupRel(it.Rel, n)
		err := renameNoReplace(tmp, filepath.Join(dir, filepath.FromSlash(rel)))
		switch {
		case err == nil:
			return rel, nil
		case !errors.Is(err, fs.ErrExist):
			return "", paths.NoPath(err)
		case it.Kind == KindXMP:
			return "", errBusy
		}
	}
	return "", fmt.Errorf("в папке «%s» слишком много файлов с именем «%s»", path.Dir(it.Rel), path.Base(it.Rel))
}

// dupRel — rel с номером n, как у плана (pick): «имя (n).ext»; n = 1 — rel как есть.
func dupRel(rel string, n int) string {
	if n == 1 {
		return rel
	}
	ext := path.Ext(rel)
	return fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(rel, ext), n, ext)
}

func stripExt(rel string) string { return strings.TrimSuffix(rel, path.Ext(rel)) }

// follow — Rel сайдкара за фото, легшим при копировании под другим именем: «фото.jpg.xmp» и
// «фото.xmp» (nameDir) — по новому имени фото.
func follow(rel string, moved map[string]string) string {
	if to, ok := moved[nameKey(strings.TrimSuffix(rel, ".xmp"))]; ok {
		return to + ".xmp"
	}
	return rel
}

// chain — папка сбора и каждое звено папки файла rel — настоящие каталоги: prepareDirs проверял
// их до начала, а за время копирования звено могли подменить ссылкой или junction.
func chain(dir, rel string) error {
	cur := dir
	// "" — сама папка сбора; «.» (файл прямо в ней) Join тоже сводит к ней.
	for _, el := range append([]string{""}, strings.Split(path.Dir(rel), "/")...) {
		cur = filepath.Join(cur, el)
		if err := isDir(cur); err != nil {
			return err
		}
	}
	return nil
}

// ctxReader — чтение кусками с проверкой отмены и счётом байт для хода. Кусок читается в
// горутине: чтение с умершего сетевого диска висит до таймаута SMB, а «Остановить» должно
// сработать сразу.
type ctxReader struct {
	ctx  context.Context
	r    io.Reader
	on   func(int64)
	left chan readResult // чтение, брошенное по отмене: ещё не вернулось
}

type readResult struct {
	n   int
	err error
}

func (c *ctxReader) Read(b []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	ch := make(chan readResult, 1)
	go func() {
		n, err := c.r.Read(b)
		ch <- readResult{n, err}
	}()
	select {
	case r := <-ch:
		if r.n > 0 && c.on != nil {
			c.on(int64(r.n))
		}
		return r.n, r.err
	case <-c.ctx.Done():
		c.left = ch // b ещё пишется брошенным чтением: CopyBuffer после ошибки его не читает
		return 0, c.ctx.Err()
	}
}

// release закрывает источник: после брошенного чтения — когда оно вернётся (Close на Windows
// ждал бы висящее чтение, и «Остановить» висело бы вместе с ним).
func (c *ctxReader) release(f io.Closer) {
	if c.left == nil {
		_ = f.Close() // чтение: ошибка закрытия ничего не теряет
		return
	}
	go func() {
		<-c.left
		_ = f.Close()
	}()
}

// diskFull — ошибка нехватки места как ErrDiskFull (ENOSPC; Windows: ERROR_DISK_FULL 112,
// ERROR_HANDLE_DISK_FULL 39), прочие — без путей.
func diskFull(err error) error {
	full := errors.Is(err, syscall.ENOSPC)
	if runtime.GOOS == "windows" {
		full = full || errors.Is(err, syscall.Errno(112)) || errors.Is(err, syscall.Errno(39))
	}
	if full {
		return fmt.Errorf("%w: %w", ErrDiskFull, paths.NoPath(err))
	}
	return paths.NoPath(err)
}
