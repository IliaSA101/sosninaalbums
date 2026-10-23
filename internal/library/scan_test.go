package library

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// scanEvent — один вызов report.
type scanEvent struct {
	photos []Photo
	pr     ScanProgress
	final  bool
}

// fastScanner — сканер с короткими интервалами отчётов, чтобы тесты не ждали по 200 мс.
func fastScanner() *Scanner {
	s := NewScanner()
	s.progressEvery, s.listEvery = time.Millisecond, time.Millisecond
	return s
}

func TestScanColor(t *testing.T) {
	dir := testkit.TempDir(t)
	icc := func(xyz [9]float64, g float64, desc []byte) []byte {
		return testkit.ICCSegment(1, 1, testkit.MatrixProfile(xyz, g, desc))
	}
	testkit.JPEGSegments(t, dir, "adobe_icc.jpg", 40, 30, icc(testkit.XYZAdobe, 2.2, testkit.DescV2("Adobe RGB (1998)")))
	testkit.JPEGSegments(t, dir, "adobe_exif.jpg", 40, 30, testkit.ExifColor(false, 0, 0xFFFF, "R03"))
	testkit.JPEGSegments(t, dir, "p3.jpg", 40, 30, icc(testkit.XYZP3, 2.2, testkit.DescV4("Display P3")))
	testkit.JPEGSegments(t, dir, "srgb.jpg", 40, 30, icc(testkit.XYZSRGB, 2.2, testkit.DescV2("sRGB IEC61966-2.1")))
	testkit.JPEGSegments(t, dir, "rot.jpg", 40, 30, testkit.ExifColor(true, 6, 0xFFFF, "R03"))
	testkit.JPEG(t, dir, "plain.jpg", 40, 30)
	testkit.CMYKJPEG(t, dir, "cmyk.jpg", 40, 30)

	photos, skipped := Scan([]string{dir})
	if len(skipped) != 0 {
		t.Fatalf("пропущено %v", skipped)
	}
	want := map[string]struct {
		color   imgfile.Color
		profile string
	}{
		"adobe_icc.jpg":  {imgfile.ColorAdobe, "Adobe RGB (1998)"},
		"adobe_exif.jpg": {imgfile.ColorAdobe, imgfile.ICCAdobeEXIF},
		"p3.jpg":         {imgfile.ColorOther, "Display P3"},
		"srgb.jpg":       {imgfile.ColorSRGB, "sRGB IEC61966-2.1"},
		"rot.jpg":        {imgfile.ColorAdobe, imgfile.ICCAdobeEXIF},
		"plain.jpg":      {imgfile.ColorNone, ""},
		"cmyk.jpg":       {imgfile.ColorOther, "CMYK"},
	}
	if len(photos) != len(want) {
		t.Fatalf("фото %d, ждали %d", len(photos), len(want))
	}
	for _, p := range photos {
		w := want[p.Name]
		if p.Color != w.color || p.Profile != w.profile {
			t.Errorf("%s: получили (%q, %q), ждали (%q, %q)", p.Name, p.Color, p.Profile, w.color, w.profile)
		}
		if st, err := os.Stat(p.Path); err != nil || p.Size != st.Size() || p.Size == 0 {
			t.Errorf("%s: размер %d, файл %v", p.Name, p.Size, err)
		}
		if p.Name == "rot.jpg" && (p.W != 30 || p.H != 40 || p.Rotate != 6) {
			t.Errorf("rot.jpg: %dx%d, поворот %d", p.W, p.H, p.Rotate)
		}
	}
}

func TestScanReportsProgress(t *testing.T) {
	dir := testkit.TempDir(t)
	const n = 24
	for i := 1; i <= n; i++ {
		testkit.JPEG(t, dir, fmt.Sprintf("img-%d.jpg", i), 20, 10) // img-2 раньше img-10
	}
	last := filepath.Join(dir, fmt.Sprintf("img-%d.jpg", n))

	s := fastScanner()
	var started atomic.Bool
	var early atomic.Bool // чтение началось раньше первого report
	release := make(chan struct{})
	s.beforeRead = func(path string) {
		if !started.Load() {
			early.Store(true)
		}
		if path == last { // последний файл «завис»: список всё равно должен расти префиксом
			select {
			case <-release:
			case <-time.After(5 * time.Second):
			}
		}
	}
	var events []scanEvent
	var once sync.Once
	photos, skipped, err := s.Scan(context.Background(), Sources{Folders: []string{dir}}, func(ph []Photo, pr ScanProgress, final bool) {
		started.Store(true)
		events = append(events, scanEvent{ph, pr, final})
		if len(ph) == n-1 {
			once.Do(func() { close(release) })
		}
	})
	if err != nil || len(skipped) != 0 || len(photos) != n {
		t.Fatalf("err=%v skipped=%v фото=%d", err, skipped, len(photos))
	}
	if early.Load() {
		t.Error("чтение началось до первого report")
	}
	if first := events[0]; !first.pr.Active || !first.pr.Walking || first.final || first.photos != nil {
		t.Errorf("первый report: %+v", first)
	}
	for i, p := range photos {
		if want := fmt.Sprintf("img-%d.jpg", i+1); p.Name != want {
			t.Fatalf("порядок: на месте %d стоит %s, ждали %s", i, p.Name, want)
		}
	}
	checkProgressEvents(t, events, photos, n)
}

// checkProgressEvents: final ровно один и последний, Found не убывает, промежуточные
// списки — растущие префиксы итогового, и хотя бы один из них неполный.
func checkProgressEvents(t *testing.T, events []scanEvent, photos []Photo, n int) {
	t.Helper()
	finals, prevLen, partial, prevFound := 0, 0, 0, 0
	for i, e := range events {
		if e.final {
			finals++
			if i != len(events)-1 {
				t.Error("final не последний")
			}
			if e.pr.Active || e.pr.Walking || e.pr.Found != n || e.pr.Read != n || len(e.photos) != n {
				t.Errorf("final: %+v, фото %d", e.pr, len(e.photos))
			}
			continue
		}
		if e.pr.Found < prevFound {
			t.Errorf("Found уменьшился: %d → %d", prevFound, e.pr.Found)
		}
		prevFound = e.pr.Found
		if e.photos == nil {
			continue
		}
		// список — префикс итогового, и он только растёт
		if len(e.photos) < prevLen || len(e.photos) > n {
			t.Errorf("список %d после %d", len(e.photos), prevLen)
		}
		for j, p := range e.photos {
			if p.ID != photos[j].ID {
				t.Fatalf("список не префикс: на месте %d %s", j, p.Name)
			}
		}
		prevLen = len(e.photos)
		if len(e.photos) > 0 && len(e.photos) < n {
			partial++
		}
	}
	if finals != 1 {
		t.Errorf("final: %d раз", finals)
	}
	if partial == 0 {
		t.Error("не было ни одного промежуточного списка")
	}
}

func TestScanCancel(t *testing.T) {
	dir := testkit.TempDir(t)
	for i := range 16 {
		testkit.JPEG(t, dir, fmt.Sprintf("f%02d.jpg", i), 20, 10)
	}
	s := fastScanner()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reading, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	s.beforeRead = func(string) {
		once.Do(func() { close(reading) })
		<-release
	}
	var finals atomic.Int32
	done := make(chan error, 1)
	go func() {
		_, _, err := s.Scan(ctx, Sources{Folders: []string{dir}}, func(_ []Photo, _ ScanProgress, final bool) {
			if final {
				finals.Add(1)
			}
		})
		done <- err
	}()
	<-reading
	cancel()
	close(release) // воркеры дочитывают текущие файлы и останавливаются
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("ошибка %v, ждали context.Canceled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Scan не вернулся после отмены")
	}
	if finals.Load() != 0 {
		t.Error("после отмены пришёл final")
	}

	// уже прочитанное осталось в памяти
	var fin ScanProgress
	if _, _, err := s.Scan(context.Background(), Sources{Folders: []string{dir}}, func(_ []Photo, pr ScanProgress, final bool) {
		if final {
			fin = pr
		}
	}); err != nil {
		t.Fatal(err)
	}
	if fin.Memo == 0 {
		t.Error("файлы, дочитанные после отмены, не попали в память")
	}
}

func TestScanCancelBeforeStart(t *testing.T) {
	dir := testkit.TempDir(t)
	testkit.JPEG(t, dir, "a.jpg", 20, 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := NewScanner().Scan(ctx, Sources{Folders: []string{dir}}, func([]Photo, ScanProgress, bool) {}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ошибка %v", err)
	}
}

func TestScanMemo(t *testing.T) {
	dir := testkit.TempDir(t)
	a := testkit.JPEG(t, dir, "a.jpg", 40, 30)
	testkit.JPEG(t, dir, "b.jpg", 30, 40)
	s := NewScanner()
	scan := func() (photos []Photo, skipped []string, memo int) {
		var fin ScanProgress
		photos, skipped, err := s.Scan(context.Background(), Sources{Folders: []string{dir}}, func(_ []Photo, pr ScanProgress, final bool) {
			if final {
				fin = pr
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		return photos, skipped, fin.Memo
	}

	if photos, skipped, memo := scan(); len(photos) != 2 || len(skipped) != 0 || memo != 0 {
		t.Fatalf("первое чтение: фото %d, пропущено %v, из памяти %d", len(photos), skipped, memo)
	}
	// содержимое испорчено при том же размере и времени → фото берётся из памяти
	st, err := os.Stat(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a, make([]byte, st.Size()), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(a, st.ModTime(), st.ModTime()); err != nil {
		t.Fatal(err)
	}
	if photos, skipped, memo := scan(); len(photos) != 2 || len(skipped) != 0 || memo != 2 {
		t.Fatalf("из памяти: фото %d, пропущено %v, из памяти %d", len(photos), skipped, memo)
	}
	// время изменилось → файл читается заново и не читается
	later := st.ModTime().Add(time.Hour)
	if err := os.Chtimes(a, later, later); err != nil {
		t.Fatal(err)
	}
	photos, skipped, memo := scan()
	if len(photos) != 1 || len(skipped) != 1 || skipped[0] != a || memo != 1 {
		t.Fatalf("после смены времени: фото %d, пропущено %v, из памяти %d", len(photos), skipped, memo)
	}
	// «нечитаемый» тоже запомнен, пока файл не меняется
	if _, skipped, memo := scan(); len(skipped) != 1 || memo != 2 {
		t.Fatalf("нечитаемый из памяти: пропущено %v, из памяти %d", skipped, memo)
	}
}

// Память держит записи только для найденных файлов и не раздувается выше лимита.
func TestScanMemoLimit(t *testing.T) {
	s := NewScanner()
	for i := range memoLimit {
		s.memo[fmt.Sprint(i)] = memoEntry{}
	}
	s.remember(scanFile{key: "new", size: 1}, scanResult{})
	if len(s.memo) != 1 {
		t.Fatalf("после лимита ждали сброс, записей %d", len(s.memo))
	}
	s.remember(scanFile{key: "unknown-size", size: -1}, scanResult{})
	if len(s.memo) != 1 {
		t.Fatal("файл без размера не должен запоминаться")
	}
}

func TestUnder(t *testing.T) {
	base := filepath.Join(os.TempDir(), "Фото")
	cases := []struct {
		path, folder string
		want         bool
	}{
		{filepath.Join(base, "a.jpg"), base, true},
		{filepath.Join(base, "sub", "deep", "a.jpg"), base, true},
		{filepath.Join(base, "a.jpg"), base + string(filepath.Separator), true},
		// регистр — по paths.Fold: на Windows и macOS не важен, на Linux «ФОТО» — другая папка
		{filepath.Join(os.TempDir(), "ФОТО", "A.JPG"), base, paths.FoldEqual("ФОТО", "Фото")},
		{filepath.Join(base, "A.JPG"), base, true},
		{filepath.Join(os.TempDir(), "Фото2", "a.jpg"), base, false},
		{base, base, false},
		{filepath.Join(os.TempDir(), "a.jpg"), base, false},
	}
	for _, c := range cases {
		if got := Under(c.path, c.folder); got != c.want {
			t.Errorf("Under(%q, %q) = %v, ждали %v", c.path, c.folder, got, c.want)
		}
	}
}

func TestLocalTarget(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("проверка форм путей Windows")
	}
	cases := []struct {
		target string
		ok     bool
	}{
		{`D:\photos\a.jpg`, true},
		{`d:/photos/a.jpg`, true},
		{`..\other\a.jpg`, true},
		{`a.jpg`, true},
		{`\\srv\share\a.jpg`, false},
		{`\\?\UNC\srv\share\a.jpg`, false},
		{`\\?\C:\a.jpg`, false},
		{`\??\UNC\srv\share\a.jpg`, false},
		{`\a.jpg`, false},
		{`C:a.jpg`, false},
		{`C:\a.jpg:stream`, false},
		{``, false},
	}
	for _, c := range cases {
		if got := localTarget(`D:\album`, c.target); got != c.ok {
			t.Errorf("localTarget(%q) = %v, ждали %v", c.target, got, c.ok)
		}
	}
	if localTarget(`\\srv\album`, `a.jpg`) {
		t.Error("относительная цель в папке на шаре не локальна")
	}
}

// Цель ссылки с устройством (073-10): без расширения — отказ всегда, «CON.jpg» — как судит ОС
// (подмена: Windows 10 — отказ, 11 — цель годная); абсолютная и относительная одинаково.
func TestLocalTargetDevices(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("устройства DOS — только Windows")
	}
	for _, target := range []string{`CON`, `x\NUL\a.jpg`, `D:\x\COM1`, `..\LPT1.`, `D:\CON\a.jpg`} {
		if localTarget(`D:\album`, target) {
			t.Errorf("localTarget(%q) пропустил устройство", target)
		}
	}
	ext := []string{`CON.jpg`, `x\aux.2024\a.jpg`, `D:\x\nul.jpg`}
	restore := paths.SetDeviceOracle(func(string) bool { return false })
	for _, target := range ext {
		if !localTarget(`D:\album`, target) {
			t.Errorf("Windows 11: localTarget(%q) = false", target)
		}
	}
	restore()
	t.Cleanup(paths.SetDeviceOracle(paths.Device))
	for _, target := range ext {
		if localTarget(`D:\album`, target) {
			t.Errorf("Windows 10: localTarget(%q) = true", target)
		}
	}
}

// Обход папки на «старой ОС» (подмена: Windows 10) пропускает «CON.jpg» и папку «Con.2024»:
// открыть их — обратиться к устройству; на «новой» — обычные фото.
func TestScanSkipsDevices(t *testing.T) {
	if runtime.GOOS != "windows" || !filepath.IsLocal("CON.jpg") {
		t.Skip("нужна ОС, где «CON.jpg» — файл (Windows 11)")
	}
	dir := testkit.TempDir(t)
	sub := filepath.Join(dir, "Con.2024")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	testkit.JPEG(t, dir, "CON.jpg", 30, 20)
	testkit.JPEG(t, sub, "b.jpg", 30, 20)
	testkit.JPEG(t, dir, "a.jpg", 30, 20)
	if photos, _ := Scan([]string{dir}); len(photos) != 3 {
		t.Fatalf("Windows 11: фото %d, ждали 3", len(photos))
	}
	t.Cleanup(paths.SetDeviceOracle(paths.Device))
	photos, _ := Scan([]string{dir})
	if len(photos) != 1 || photos[0].Name != "a.jpg" {
		t.Fatalf("Windows 10: фото %+v, ждали только a.jpg", photos)
	}
}

// Ссылка на файл с локальной целью читается; на UNC — нет (на Windows без прав симлинк не создать).
func TestScanFileSymlinks(t *testing.T) {
	root := testkit.TempDir(t)
	target := testkit.JPEG(t, testkit.TempDir(t), "real.jpg", 40, 30)
	if err := os.Symlink(target, filepath.Join(root, "link.jpg")); err != nil {
		t.Skipf("симлинк не создался: %v", err)
	}
	if photos, _ := Scan([]string{root}); len(photos) != 1 {
		t.Fatalf("ссылка на локальный файл: %d фото, ждали 1", len(photos))
	}
	unc := testkit.TempDir(t)
	netTarget := `\\127.0.0.1\nosuchshare\x.jpg`
	if runtime.GOOS != "windows" {
		netTarget = "//nosuchhost/x.jpg"
	}
	if err := os.Symlink(netTarget, filepath.Join(unc, "net.jpg")); err != nil {
		t.Skipf("симлинк на UNC не создался: %v", err)
	}
	if _, ok := statLink(filepath.Join(unc, "net.jpg")); ok {
		t.Fatal("ссылка на UNC прошла")
	}
	if photos, _ := Scan([]string{unc}); len(photos) != 0 {
		t.Fatalf("ссылка на UNC: %d фото, ждали 0", len(photos))
	}
}

// walkEntry: точка-папка внутри обхода пропускается целиком, корень с точкой — нет;
// обычная папка — запись без обработки, файл — дальше без info (её даст d.Info()).
func TestWalkEntry(t *testing.T) {
	root := mkdir(t, testkit.TempDir(t), ".album")
	hidden := mkdir(t, root, ".trash")
	plain := mkdir(t, root, "x")
	file := testkit.JPEG(t, root, "a.jpg", 20, 10)
	entry := func(p string) fs.DirEntry {
		st, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		return fs.FileInfoToDirEntry(st)
	}
	cases := []struct {
		p    string
		file bool
		ret  error
	}{
		{root, false, nil},
		{hidden, false, filepath.SkipDir},
		{plain, false, nil},
		{file, true, nil},
	}
	for _, c := range cases {
		info, file, ret := walkEntry(root, c.p, entry(c.p))
		if info != nil || file != c.file || !errors.Is(ret, c.ret) {
			t.Errorf("%s: info %v, файл %v, ret %v; ждали %v, %v", filepath.Base(c.p), info, file, ret, c.file, c.ret)
		}
	}
}

// ctxDue смотрит на отмену только каждую ctxCheckEvery-ю запись.
func TestCtxDue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ctxDue(ctx, ctxCheckEvery-1); err != nil {
		t.Errorf("между проверками: %v", err)
	}
	if err := ctxDue(ctx, 2*ctxCheckEvery); !errors.Is(err, context.Canceled) {
		t.Errorf("на проверке: %v", err)
	}
	if err := ctxDue(context.Background(), ctxCheckEvery); err != nil {
		t.Errorf("без отмены: %v", err)
	}
}

// take: «вырос» только префикс подряд прочитанных новых; ошибка — stBad, чтение считается.
func TestScannerTake(t *testing.T) {
	s := NewScanner()
	files := []scanFile{{key: "a", size: 1}, {key: "b", size: 1}, {key: "c", size: 1}}
	run := s.prepare(files)
	run.advance()
	if got := s.take(run, scanResult{i: 1, ph: Photo{Name: "b"}}); got || run.newEnd != 0 {
		t.Errorf("второй раньше первого: вырос %v, newEnd %d", got, run.newEnd)
	}
	if got := s.take(run, scanResult{i: 0, err: errors.New("битый")}); !got || run.newEnd != 2 {
		t.Errorf("первый: вырос %v, newEnd %d", got, run.newEnd)
	}
	if run.state[0] != stBad || run.state[1] != stOK || run.res[1].Name != "b" || run.read != 2 {
		t.Errorf("состояние %v, res[1] %q, прочитано %d", run.state, run.res[1].Name, run.read)
	}
	if e, ok := s.memo["a"]; !ok || !e.bad {
		t.Errorf("память для битого: %+v %v", e, ok)
	}
}
