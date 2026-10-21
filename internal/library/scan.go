package library

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Частота событий сканирования: интерфейс не должен захлёбываться на больших папках.
const (
	ProgressEvery = 200 * time.Millisecond // ≤ 5 событий хода в секунду
	ListEvery     = 500 * time.Millisecond // список в ленту — ≤ 2 раз в секунду
	memoLimit     = 100_000                // записей в памяти заголовков, дальше она сбрасывается
	ctxCheckEvery = 256                    // как часто обход смотрит на отмену (в записях)
)

// ScanProgress — ход сканирования для интерфейса.
type ScanProgress struct {
	Active  bool `json:"active"`  // сканирование идёт
	Walking bool `json:"walking"` // ещё обходим папки: Found растёт
	Found   int  `json:"found"`   // файлов-фото найдено
	Read    int  `json:"read"`    // из них прочитано (в том числе из памяти)

	// Сводка для лога; заполняется в финальном отчёте и в интерфейс не уходит.
	Memo     int           `json:"-"` // сколько файлов взято из памяти
	WalkTime time.Duration `json:"-"`
	ReadTime time.Duration `json:"-"`
}

// memoEntry — прочитанный заголовок файла; верен, пока не изменились размер и время
// и фото, и его сайдкара (появился, пропал, изменился — фото перечитывается целиком:
// заголовок — несколько КБ, дешевле отдельной памяти сайдкаров).
type memoEntry struct {
	size, mod int64
	side      sidecar
	photo     Photo
	bad       bool // файл не читается как картинка
}

// Scanner помнит прочитанные заголовки (путь в нижнем регистре → размер, mtime, Photo или «нечитаем»),
// чтобы «+ Папка», «Убрать папку» и Undo не перечитывали старые файлы.
// Не больше memoLimit записей, дальше память сбрасывается.
type Scanner struct {
	mu   sync.Mutex
	memo map[string]memoEntry

	last   fingerprint // отпечаток последнего законченного Scan (fingerprint.go)
	lastOK bool        // last заполнен: Scan с тех пор не начинался заново без конца

	progressEvery, listEvery time.Duration     // ProgressEvery и ListEvery; в тестах короче
	beforeRead               func(path string) // только для тестов: вызывается перед чтением файла
}

func NewScanner() *Scanner {
	return &Scanner{memo: map[string]memoEntry{}, progressEvery: ProgressEvery, listEvery: ListEvery}
}

// scanFile — файл-кандидат, найденный обходом.
type scanFile struct {
	path, key string  // key — путь в нижнем регистре
	size, mod int64   // size < 0: размер неизвестен, в память не смотрим и не пишем
	side      sidecar // .xmp рядом; пустой — нет
}

const (
	stPending = iota
	stOK
	stBad
)

type scanResult struct {
	i     int
	ph    Photo
	err   error
	retry bool // сайдкар не дочитался: не запоминать
}

// scanRun — состояние одного сканирования. Пишет в него только горутина Scan (сборщик).
type scanRun struct {
	files    []scanFile
	res      []Photo
	state    []uint8
	fromMemo []bool
	unk      []int // индексы файлов, которых в памяти нет
	next     int   // сколько первых из unk уже обработано подряд
	newEnd   int   // новые файлы с индексом меньше newEnd обработаны подряд
	read     int
	memo     int
}

// advance сдвигает «подряд обработанный префикс» новых файлов.
func (r *scanRun) advance() {
	for r.next < len(r.unk) && r.state[r.unk[r.next]] != stPending {
		r.next++
	}
	r.newEnd = len(r.files)
	if r.next < len(r.unk) {
		r.newEnd = r.unk[r.next]
	}
}

// list — известные фото в порядке ленты: все из памяти и прочитанный префикс новых.
func (r *scanRun) list() []Photo {
	out := make([]Photo, 0, len(r.files))
	for i := range r.files {
		if r.state[i] == stOK && (r.fromMemo[i] || i < r.newEnd) {
			out = append(out, r.res[i])
		}
	}
	return out
}

func (r *scanRun) progress() ScanProgress {
	return ScanProgress{Active: true, Found: len(r.files), Read: r.read}
}

// prepare подставляет из памяти всё, у чего совпали размер и время.
func (s *Scanner) prepare(files []scanFile) *scanRun {
	m := len(files)
	r := &scanRun{files: files, res: make([]Photo, m), state: make([]uint8, m), fromMemo: make([]bool, m)}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, f := range files {
		e, ok := s.memo[f.key]
		if !ok || f.size < 0 || e.size != f.size || e.mod != f.mod || e.side != f.side {
			r.unk = append(r.unk, i)
			continue
		}
		r.fromMemo[i], r.memo = true, r.memo+1
		if e.bad {
			r.state[i] = stBad
			continue
		}
		r.state[i] = stOK
		r.res[i] = e.photo
		r.res[i].Path, r.res[i].Name = f.path, filepath.Base(f.path) // регистр пути мог быть другим
	}
	r.read = r.memo
	return r
}

// Sources — источники ленты: папки (с подпапками) и отдельные файлы (097).
type Sources struct{ Folders, Files []string }

// Scan находит фото в папках (с подпапками) и отдельных файлах и читает их заголовки.
// Файлы, которые не удалось прочитать, попадают в skipped.
//
// report вызывается только из горутины Scan, не параллельно: сразу в начале, потом не чаще
// ProgressEvery, в конце — с final = true. photos != nil — новый список известных фото в порядке
// ленты (не чаще ListEvery): известные по памяти сразу, новые — подряд идущим префиксом, чтобы
// ничего не прыгало. Отмена через ctx: новые задания не выдаются, Scan возвращает ctx.Err()
// без финального report; уже прочитанное остаётся в памяти.
func (s *Scanner) Scan(ctx context.Context, src Sources,
	report func(photos []Photo, pr ScanProgress, final bool)) (photos []Photo, skipped []string, err error) {
	report(nil, ScanProgress{Active: true, Walking: true}, false)
	s.invalidateLast()

	t0 := time.Now()
	files, err := s.walk(ctx, src, func(found int) {
		report(nil, ScanProgress{Active: true, Walking: true, Found: found}, false)
	})
	if err != nil {
		return nil, nil, err
	}
	walkTime := time.Since(t0)

	// Порядок задан заранее: списки потом собираются за один проход, без сортировки.
	sort.Slice(files, func(i, j int) bool { return naturalLess(files[i].key, files[j].key) })
	run := s.prepare(files)
	run.advance()
	report(run.list(), run.progress(), false)

	t1 := time.Now()
	var xl xmpLog
	s.readNew(ctx, run, report, &xl)
	xl.summary()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	s.setLast(fingerprintOf(src, files))
	m := len(files)
	photos = make([]Photo, 0, m)
	for i, f := range files {
		switch run.state[i] {
		case stOK:
			photos = append(photos, run.res[i])
		case stBad:
			skipped = append(skipped, f.path)
		}
	}
	report(photos, ScanProgress{Found: m, Read: m, Memo: run.memo, WalkTime: walkTime, ReadTime: time.Since(t1)}, true)
	return photos, skipped, nil
}

// readNew читает заголовки новых файлов NumCPU воркерами по порядку и ведёт отчёты.
// Возвращается, когда воркеры закончили (при отмене они дочитывают только текущий файл).
// Ошибки XMP воркеры пишут через xl — общий на сканирование.
func (s *Scanner) readNew(ctx context.Context, run *scanRun, report func([]Photo, ScanProgress, bool), xl *xmpLog) {
	if len(run.unk) == 0 {
		return
	}
	results := s.startReaders(ctx, run, xl)

	tick := time.NewTicker(s.progressEvery)
	defer tick.Stop()
	lastList := time.Now()
	changed, grown := false, false // что-то прочитано / префикс новых вырос с прошлого отчёта
	for {
		select {
		case r, ok := <-results:
			if !ok {
				return
			}
			g := s.take(run, r)
			changed, grown = true, grown || g
		case <-tick.C:
			if ctx.Err() != nil {
				continue // отменено: ничего не шлём, ждём, пока воркеры закончат
			}
			due := grown && time.Since(lastList) >= s.listEvery
			if !changed && !due {
				continue
			}
			var list []Photo
			if due {
				list, lastList, grown = run.list(), time.Now(), false
			}
			report(list, run.progress(), false)
			changed = false
		}
	}
}

// startReaders запускает воркеров по run.unk; канал результатов закрывается, когда они
// закончили. При отмене новые задания не выдаются.
func (s *Scanner) startReaders(ctx context.Context, run *scanRun, xl *xmpLog) <-chan scanResult {
	jobs := make(chan int)
	results := make(chan scanResult)
	var wg sync.WaitGroup
	for range min(runtime.NumCPU(), len(run.unk)) {
		wg.Go(func() {
			for i := range jobs {
				if s.beforeRead != nil {
					s.beforeRead(run.files[i].path)
				}
				ph, retry, err := readScan(run.files[i], xl)
				results <- scanResult{i, ph, err, retry}
			}
		})
	}
	go func() {
		defer close(jobs)
		for _, i := range run.unk {
			select {
			case jobs <- i:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { wg.Wait(); close(results) }()
	return results
}

// take записывает результат воркера в run и память; true — префикс новых вырос.
func (s *Scanner) take(run *scanRun, r scanResult) bool {
	if r.err != nil {
		run.state[r.i] = stBad
	} else {
		run.state[r.i], run.res[r.i] = stOK, r.ph
	}
	run.read++
	s.remember(run.files[r.i], r)
	before := run.newEnd
	run.advance()
	return run.newEnd != before
}

func (s *Scanner) remember(f scanFile, r scanResult) {
	var pe *fs.PathError
	if f.size < 0 || r.retry || (r.err != nil && errors.As(r.err, &pe)) {
		return // ошибка открытия/чтения может быть временной (диск, блокировка): не запоминаем
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.memo) >= memoLimit {
		s.memo = map[string]memoEntry{}
	}
	s.memo[f.key] = memoEntry{size: f.size, mod: f.mod, side: f.side, photo: r.ph, bad: r.err != nil}
}

// walk обходит папки, как раньше: ссылки на папки и скрытые папки пропускаются, дубли тоже.
// Размер и время берутся из d.Info(): на Windows это данные листинга, лишнего вызова нет.
// Попутно собирает сайдкары .xmp и в конце приписывает их к фото.
func (s *Scanner) walk(ctx context.Context, src Sources, tick func(found int)) ([]scanFile, error) {
	w := scanWalk{seen: map[string]bool{}, sides: sideSet{m: map[string]sidecar{}},
		last: time.Now(), every: s.progressEvery, tick: tick}
	visited := 0
	for _, dir := range src.Folders {
		// Корень-ссылка не обходится, как и ссылка внутри (walkEntry): так договорено с
		// paths.Reaches — она ничего не накрывает (0.22-sec-1). Clean — «C:\Ссылка\» с разделителем
		// на конце WalkDir прошёл бы по цели без проверки её локальности.
		dir = filepath.Clean(dir)
		// Ошибки отдельных файлов callback пропускает сам; корень недоступен — просто нет фото.
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			visited++
			if err := ctxDue(ctx, visited); err != nil {
				return err
			}
			if d.Type().IsRegular() && isSidecar(d.Name()) {
				w.sides.add(p, d)
				return nil
			}
			info, file, ret := walkEntry(dir, p, d)
			if !file {
				return ret
			}
			w.addImage(p, d, info)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	for i, p := range src.Files {
		if err := ctxDue(ctx, i+1); err != nil {
			return nil, err
		}
		w.addLoose(p)
	}
	files := w.files
	if len(w.sides.m) > 0 {
		for i := range files {
			files[i].side = sidecarFor(files[i].key, w.sides.m)
		}
	}
	return files, ctx.Err()
}

// scanWalk — что walk собрал по всем папкам.
type scanWalk struct {
	files []scanFile
	seen  map[string]bool // ключи уже взятых файлов: папки могут пересекаться
	sides sideSet
	last  time.Time // время прошлого tick
	every time.Duration
	tick  func(found int)
}

// addImage берёт файл-фото, если он ещё не встречался; info == nil — из d.Info().
func (w *scanWalk) addImage(p string, d fs.DirEntry, info fs.FileInfo) {
	w.add(p, d.Name(), info, func() fs.FileInfo { i, _ := d.Info(); return i })
}

// add — общее тело addImage и addLoose; load отдаёт сведения о файле, когда info == nil.
func (w *scanWalk) add(p, name string, info fs.FileInfo, load func() fs.FileInfo) {
	key := strings.ToLower(p)
	if !isImage(name) || w.seen[key] {
		return
	}
	w.seen[key] = true
	f := scanFile{path: p, key: key, size: -1}
	if info == nil {
		info = load()
	}
	if info != nil {
		f.size, f.mod = info.Size(), info.ModTime().UnixNano()
	}
	w.files = append(w.files, f)
	if now := time.Now(); now.Sub(w.last) >= w.every {
		w.last = now
		w.tick(len(w.files))
	}
}

// ctxDue смотрит на отмену раз в ctxCheckEvery записей обхода: ctx.Err() на каждой — лишняя работа.
func ctxDue(ctx context.Context, visited int) error {
	if visited%ctxCheckEvery == 0 {
		return ctx.Err()
	}
	return nil
}

// walkEntry — общий для walk и FindByName разбор записи обхода папки root.
// file = true — запись-файл, обрабатывать дальше; info — сведения о цели ссылки, у обычного
// файла nil (берутся из d.Info() по надобности). file = false — callback возвращает ret:
// nil — пропустить запись, SkipDir — не заходить в точку-папку.
func walkEntry(root, p string, d fs.DirEntry) (info fs.FileInfo, file bool, ret error) {
	if p != root && paths.DeviceElem(d.Name()) {
		// «CON.jpg» с Windows 11 на Windows 10 — устройство: открыть его — обратиться к консоли.
		if d.IsDir() {
			return nil, false, filepath.SkipDir
		}
		return nil, false, nil
	}
	if d.Type()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		// Ссылка или junction на папку может вести в цикл; ссылки на файлы читаем,
		// если цель локальная (ссылка на UNC открыла бы сетевую шару).
		st, ok := statLink(p)
		if !ok {
			return nil, false, nil
		}
		return st, true, nil
	}
	if d.IsDir() {
		if p != root && strings.HasPrefix(d.Name(), ".") {
			return nil, false, filepath.SkipDir
		}
		return nil, false, nil
	}
	return nil, true, nil
}

// Scan — одноразовое сканирование без отчётов и памяти (тесты и простые случаи).
func Scan(folders []string) ([]Photo, []string) {
	photos, skipped, _ := NewScanner().Scan(context.Background(), Sources{Folders: folders}, func([]Photo, ScanProgress, bool) {})
	return photos, skipped
}
