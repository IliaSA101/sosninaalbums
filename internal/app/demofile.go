package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/app/libpacks"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/demopack"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Файл примера у программы (спека 142 §2). В exe он не встроен (0.30-142-33): стандартные
// установщики кладут его рядом, компактная докачивает. Пути закреплены в коде — не из настроек
// и не от пользователя.

// demoExe — exe для поиска файла: на Mac папка берётся после раскрытия ссылок (запуск через
// симлинк в /usr/local/bin иначе искал бы Contents/Resources рядом со ссылкой).
func demoExe(goos, exe string) string {
	if goos != "darwin" {
		return exe
	}
	if res, err := filepath.EvalSymlinks(exe); err == nil {
		return res
	}
	return exe
}

// demoPaths — кандидаты файла примера по ОС и пути exe, по порядку. Чистая функция над
// строками: путь Windows разбирается и на другой ОС (тест).
func demoPaths(goos, exe string) []string {
	sep, seps := "/", "/"
	if goos == "windows" {
		sep, seps = `\`, `\/`
	}
	var out []string
	if i := strings.LastIndexAny(exe, seps); i >= 0 { // exe неизвестен (os.Executable не сработал) — без папки
		dir := exe[:i]
		// /usr/bin/data/… на Linux безвредно, но лишнее: пакет данных кладёт файл в /usr/share.
		if goos != "linux" || dir != "/usr/bin" {
			out = append(out, dir+sep+"data"+sep+demopack.File)
		}
		if goos == "darwin" { // .app/Contents/MacOS → Contents/Resources (под подписью .app)
			out = append(out, path.Join(dir, "..", "Resources", demopack.File))
		}
	}
	if goos == "linux" { // пакет sosninaalbums-demo
		out = append(out, "/usr/share/sosninaalbums/"+demopack.File)
	}
	return out
}

// demoFile — файл примера рядом с программой: Has дёшево, Read проверяет байты целиком.
type demoFile struct {
	cands []string
	size  int64  // ожидаемый размер (demopack.Size; в тестах — малый)
	sum   string // ожидаемый sha256 (demopack.SHA256)

	mu     sync.Mutex
	probed bool
	path   string // найденный кандидат; "" — файла нет
	bad    bool   // Read отказал: до конца сеанса файла «нет»
	logged bool
}

// newDemoFile — файл примера для запущенного exe.
func newDemoFile(goos, exe string) *demoFile {
	return &demoFile{cands: demoPaths(goos, demoExe(goos, exe)), size: demopack.Size, sum: demopack.SHA256}
}

// Has — есть ли файл: первый кандидат — обычный файл (не ссылка) нужного размера. Один раз за
// сеанс: 17 МБ при запуске и при каждом открытии окна не читаются.
func (d *demoFile) Has() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.probed {
		d.probed = true
		d.path = d.probe()
	}
	return d.path != "" && !d.bad
}

// probe — первый подходящий кандидат. Путь не локальный (UNC, устройство) — пропуск: переносная
// копия с сетевой шары ведёт себя как компактная.
func (d *demoFile) probe() string {
	for _, p := range d.cands {
		if paths.CheckLocalPath(p) != nil {
			continue
		}
		if st, err := os.Lstat(p); err == nil && st.Mode().IsRegular() && st.Size() == d.size {
			return p
		}
	}
	return ""
}

// Read — байты файла, когда решено ставить (под замком службы): размер по дескриптору, чтение
// не больше size+1 байт, sha256 тех же байтов. Подмена между Has и Read ничего не даёт: дальше
// идут ровно проверенные байты. Отказ — Has до конца сеанса false, строка в log.txt без пути.
func (d *demoFile) Read() ([]byte, bool) {
	if !d.Has() {
		return nil, false
	}
	d.mu.Lock()
	p := d.path
	d.mu.Unlock()
	data, why := d.load(p)
	if why != "" {
		d.fail(why)
		return nil, false
	}
	return data, true
}

// load читает и проверяет файл; причина отказа — фиксированная строка (ошибки ОС несут путь).
func (d *demoFile) load(p string) ([]byte, string) {
	f, st, err := library.OpenLocal(p)
	if err != nil {
		return nil, "не открывается"
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	if !st.Mode().IsRegular() || st.Size() != d.size {
		return nil, "размер не тот"
	}
	var buf bytes.Buffer
	buf.Grow(int(d.size) + 1) // размер уже сверён, предел памяти — size+1
	if _, err := io.Copy(&buf, io.LimitReader(f, d.size+1)); err != nil {
		return nil, "не читается"
	}
	if int64(buf.Len()) != d.size {
		return nil, "размер не тот"
	}
	sum := sha256.Sum256(buf.Bytes())
	if hex.EncodeToString(sum[:]) != d.sum {
		return nil, "сумма не сходится"
	}
	return buf.Bytes(), ""
}

// fail — файл больше не предлагается; в log.txt один раз за сеанс (стандартная с v1 в data\
// после выхода v2 писала бы строку на каждом запуске).
func (d *demoFile) fail(why string) {
	d.mu.Lock()
	d.bad = true
	first := !d.logged
	d.logged = true
	d.mu.Unlock()
	if first {
		log.Printf("пример: файл рядом с программой не принят (%s)", why)
	}
}

// demoOff — SA_DEMO=off: пример выключен (пустая библиотека на стенде QA, 0.30-142-13). Только
// в сборке для разработки; в выпуске забытая переменная ничего не меняет.
func demoOff(env string, dev bool) bool { return dev && env == "off" }

// makeDemo собирает Demo для службы наборов; off — пустое значение.
func makeDemo(f *demoFile, off bool) libpacks.Demo {
	if off {
		return libpacks.Demo{}
	}
	return libpacks.Demo{
		ID: demopack.ID, Version: demopack.Version, Related: demopack.Related, Asset: demopack.Asset,
		Entry: fetch.Entry{
			ID: demopack.ID, Version: demopack.Version, Title: demopack.Title, About: demopack.About,
			License: demopack.License, Size: demopack.Size, SHA256: demopack.SHA256,
			Counts: fetch.Counts{Bg: demopack.Bg, Sticker: demopack.Sticker, Spread: demopack.Spread, Frame: demopack.Frame},
		},
		Has: f.Has, Read: f.Read,
	}
}

// demo — пример для Host.Demo: файл ищется по запущенному exe один раз за сеанс.
func (a *App) demo() libpacks.Demo {
	if a.demoFn != nil {
		return a.demoFn()
	}
	a.demoOnce.Do(func() {
		exe, _ := os.Executable() // ошибка — exe пуст: без папки программы, остаются системные пути
		a.demoF = newDemoFile(runtime.GOOS, exe)
	})
	return makeDemo(a.demoF, demoOff(os.Getenv("SA_DEMO"), osx.DevBuild))
}
