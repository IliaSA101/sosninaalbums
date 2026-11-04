package library

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestReadScanCloudOnly: фото-заглушка (scanFile.cloud) не открывается вовсе — путь даже не существует, а ответ
// не PathError; ошибка несёт готовую причину и не запоминается (retry): после скачивания файл прочтётся.
func TestReadScanCloudOnly(t *testing.T) {
	missing := filepath.Join(testkit.TempDir(t), "нет.jpg")
	_, retry, err := readScan(scanFile{path: missing, key: missing, cloud: true}, &xmpLog{})
	if !errors.Is(err, ErrCloudOnly) {
		t.Fatalf("ошибка %v, ожидалась ErrCloudOnly", err)
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		t.Errorf("файл открывался: %v", err)
	}
	if !retry {
		t.Error("результат заглушки запомнится: после скачивания файл не перечитается")
	}
	if why := whyOf(err); why != whyCloudOnly {
		t.Errorf("причина %q", why)
	}
}

// TestReadScanCloudSidecar: сайдкар-заглушка не читается (путь не открывается — иначе была бы ошибка и retry),
// фото при этом читается; обычный сайдкар рядом по-прежнему даёт отметки.
func TestReadScanCloudSidecar(t *testing.T) {
	dir := testkit.TempDir(t)
	photo := testkit.ExportJPEG(t, dir, "a.jpg", 64, 48)
	gone := filepath.Join(dir, "нет.xmp")
	ph, retry, err := readScan(scanFile{path: photo, key: photo, side: sidecar{path: gone, size: 10, cloud: true}}, &xmpLog{})
	if err != nil || retry {
		t.Fatalf("фото с сайдкаром-заглушкой: err=%v retry=%v", err, retry)
	}
	if ph.W != 64 || ph.H != 48 {
		t.Errorf("размеры %dx%d", ph.W, ph.H)
	}
	// Тот же путь без признака облака — сайдкар открывается и не находится: контроль, что флаг и есть причина.
	if _, retry, _ := readScan(scanFile{path: photo, key: photo, side: sidecar{path: gone, size: 10}}, &xmpLog{}); !retry {
		t.Error("без признака облака пропавший сайдкар должен дать retry")
	}
}

// TestReadHeadRegularStillRead: обычный файл (не облачный) читается как прежде, ReadInfo не отказывает.
func TestReadHeadRegularStillRead(t *testing.T) {
	photo := testkit.ExportJPEG(t, testkit.TempDir(t), "a.jpg", 32, 24)
	ph, err := ReadInfo(photo)
	if err != nil || ph.W != 32 || ph.H != 24 {
		t.Fatalf("ReadInfo: %+v, %v", ph, err)
	}
}

// fakeEntry — запись обхода с заданным типом.
type fakeEntry struct{ mode fs.FileMode }

func (fakeEntry) Name() string        { return "a.xmp" }
func (fakeEntry) Size() int64         { return 1 }
func (f fakeEntry) Mode() fs.FileMode { return f.mode }
func (fakeEntry) ModTime() time.Time  { return time.Time{} }
func (f fakeEntry) IsDir() bool       { return f.mode.IsDir() }
func (fakeEntry) Sys() any            { return nil }

// TestRegularEntry: обычный файл — да; папка, ссылка, канал — нет; ModeIrregular вне Windows — нет (облачных заглушек
// там не бывает), на Windows решает тег (windows-тест).
func TestRegularEntry(t *testing.T) {
	for name, c := range map[string]struct {
		m    fs.FileMode
		want bool
	}{
		"обычный": {0o644, true}, "папка": {fs.ModeDir, false}, "ссылка": {fs.ModeSymlink, false},
		"канал": {fs.ModeNamedPipe, false}, "irregular без тега": {fs.ModeIrregular, false},
	} {
		if got := regularEntry("p", fs.FileInfoToDirEntry(fakeEntry{c.m})); got != c.want {
			t.Errorf("%s: %v, ожидалось %v", name, got, c.want)
		}
	}
}

// TestSidecarCloudFlagBreaksMemo: сайдкар с признаком облака и без него — разные значения: память заголовков
// (сравнение sidecar) после скачивания файла не отдаст прежний результат без отметок.
func TestSidecarCloudFlagBreaksMemo(t *testing.T) {
	a := sidecar{path: "p", size: 1, mod: 2}
	b := a
	b.cloud = true
	if a == b {
		t.Fatal("признак облака не входит в сравнение сайдкаров")
	}
}
