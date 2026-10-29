package album_test

// Тесты реплея и сверка с эталоном (сценарий — replay_test.go, 110-Т1): реплей идёт на всех ОС,
// ведущая машина — Linux, а падало на CI Windows (0.23).

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// replayRun — сценарий с вымышленным диском Q: и его строки эталона (строка — шаг).
// Диск Q: вымышленный: «том на месте» — иначе свёртка источников зависела бы от машины
// (нет тома — запись не сворачивается, 0.22-sec-1).
func replayRun(t *testing.T) (*replay, []byte) {
	t.Helper()
	t.Cleanup(paths.SetVolumeOracle(func(string) bool { return true }))
	r := replayScenario(t)
	var buf bytes.Buffer
	buf.WriteString("[\n")
	for i, s := range r.steps {
		line, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(line)
		if i < len(r.steps)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("]\n")
	return r, buf.Bytes()
}

// replayGolden — эталон своей ОС: пути Windows (`Q:\…`) и прочих (`/Q/…`) в записи разные.
func replayGolden() string {
	if runtime.GOOS == "windows" {
		return filepath.Join("testdata", "replay.json.gz")
	}
	return filepath.Join("testdata", "replay-unix.json.gz")
}

func TestEditorReplay(t *testing.T) {
	_, got := replayRun(t)
	compareReplay(t, replayGolden(), got)
}

// TestEditorReplayMethods — шаг на каждый публичный метод Editor, кроме View и Save (они идут в
// каждом шаге-правке) и ID (номер Editor в процессе, 087-3: зависит от порядка тестов, в эталон
// не годится; проверка — TestEditorViewSrc), Close (после него реплей правок не идёт; проверка —
// TestEditorClose): новый метод без шага — провал. Не зависит от эталона и ОС.
func TestEditorReplayMethods(t *testing.T) {
	r, _ := replayRun(t)
	seen := map[string]bool{"View": true, "Save": true, "ID": true, "Close": true}
	for _, s := range r.steps {
		seen[strings.SplitN(s.Step, ":", 2)[0]] = true
	}
	typ := reflect.TypeFor[*album.Editor]()
	for i := range typ.NumMethod() {
		if name := typ.Method(i).Name; !seen[name] {
			t.Errorf("метод %s без шага в реплее", name)
		}
	}
}

// TestEditorReplayStable — два прогона дают одни и те же байты: дрейф от порядка обхода карт,
// времени или состояния процесса виден сразу, без эталона. Первый разошедшийся шаг — в сообщении.
func TestEditorReplayStable(t *testing.T) {
	if testing.Short() {
		t.Skip("второй прогон сценария — не быстрый уровень")
	}
	_, a := replayRun(t)
	_, b := replayRun(t)
	if bytes.Equal(a, b) {
		return
	}
	al, bl := strings.Split(string(a), "\n"), strings.Split(string(b), "\n")
	for i := range max(len(al), len(bl)) {
		if i >= len(al) || i >= len(bl) || al[i] != bl[i] {
			// строка 0 — «[», шаг n — строка n+1
			t.Fatalf("прогоны разошлись на шаге %d из %d", max(i-1, 0), max(len(al), len(bl))-3)
		}
	}
	t.Fatal("прогоны разошлись (перевод строк?)")
}

// readReplayGz читает эталон: gzip → тот же построчный JSON.
func readReplayGz(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(zr)
}

// writeReplayGz пишет эталон сжатым без имени и времени в заголовке (воспроизводимо).
func writeReplayGz(path string, data []byte) error {
	var out bytes.Buffer
	zw, err := gzip.NewWriterLevel(&out, gzip.BestCompression)
	if err != nil {
		return err
	}
	if _, err := zw.Write(data); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

// compareReplay сверяет с эталоном построчно (строка — шаг) и называет первый расходящийся шаг.
// Эталон хранится сжатым (2 МБ текста не пускает pre-commit); при расхождении текущий
// реплей лежит несжатым в $GOTMPDIR/replay-got.json — для diff руками.
func compareReplay(t *testing.T, path string, got []byte) {
	t.Helper()
	want, err := readReplayGz(path)
	if *edtest.Update {
		// Равное не переписываем: байты gzip зависят от версии Go — лишний шум в git.
		if err == nil && bytes.Equal(got, want) {
			return
		}
		if err := writeReplayGz(path, got); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err != nil {
		t.Fatalf("%v (создать: -update)", err)
	}
	if bytes.Equal(got, want) {
		return
	}
	dump := ""
	if dir := os.Getenv("GOTMPDIR"); dir != "" {
		p := filepath.Join(dir, "replay-got.json")
		if os.WriteFile(p, got, 0o644) == nil {
			dump = "\nтекущий реплей: " + p
		}
	}
	gl, wl := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
	for i := range max(len(gl), len(wl)) {
		var g, w string
		if i < len(gl) {
			g = gl[i]
		}
		if i < len(wl) {
			w = wl[i]
		}
		if g == w {
			continue
		}
		at := 0
		for at < min(len(g), len(w)) && g[at] == w[at] {
			at++
		}
		cut := func(s string) string { return s[max(0, at-150):min(len(s), at+150)] }
		t.Fatalf("строка %d разошлась с эталоном (символ %d):\nесть:  …%s…\nждали: …%s…%s", i, at, cut(g), cut(w), dump)
	}
	t.Fatal("эталон отличается (перевод строк?)" + dump)
}
