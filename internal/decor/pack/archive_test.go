package pack

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// entry — запись тестового архива; raw — заголовок как есть (бомба, шифрование, метод).
type entry struct {
	name string
	data []byte
	raw  *zip.FileHeader
	body []byte // сжатые байты для raw
}

func zipOf(t testing.TB, list []entry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range list {
		var (
			w   io.Writer
			err error
		)
		if e.raw != nil {
			w, err = zw.CreateRaw(e.raw)
			if err == nil {
				_, err = w.Write(e.body)
			}
		} else if w, err = zw.CreateHeader(&zip.FileHeader{Name: e.name, Method: zip.Deflate}); err == nil {
			_, err = w.Write(e.data)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// goodFiles — предметы набора: два фона, стикер, шаблон (содержимое pack не смотрит).
func goodFiles() []entry {
	return []entry{
		{name: "bg/ivory-01.jpg", data: bytes.Repeat([]byte("фон один "), 50)},
		{name: "bg/ivory-02.png", data: []byte("\x89PNG\r\n\x1a\nвторой")},
		{name: "stickers/rose.png", data: []byte("\x89PNG\r\n\x1a\nроза")},
		{name: "spreads/hero.json", data: []byte(`{"v":1,"pages":1,"page":[{"color":"#ffffff"}]}`)},
	}
}

// packZip — архив с манифестом по files (вид — по папке); mut правит манифест, extra — лишние записи.
func packZip(t testing.TB, files []entry, mut func(m map[string]any), extra ...entry) []byte {
	t.Helper()
	items := make([]any, 0, len(files))
	for _, f := range files {
		kind := map[string]string{"bg": KindBg, "stickers": KindSticker, "spreads": KindSpread}[strings.Split(f.name, "/")[0]]
		items = append(items, map[string]any{"file": f.name, "kind": kind, "theme": "wedding", "name": f.name,
			"sha256": sum(f.data), "size": len(f.data)})
	}
	m := map[string]any{"format": 1, "id": "ivory", "version": 1, "minApp": "0.16.0", "title": "Айвори", "items": items}
	if mut != nil {
		mut(m)
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return zipOf(t, append(append([]entry{{name: ManifestName, data: data}}, files...), extra...))
}

func openBytes(b []byte) (*Archive, error) {
	return OpenReader(bytes.NewReader(b), int64(len(b)), "0.16.0")
}

func extractAll(t *testing.T, a *Archive) [][]byte {
	t.Helper()
	out := make([][]byte, len(a.Manifest.Items))
	for i := range a.Manifest.Items {
		var buf bytes.Buffer
		if err := a.Extract(i, &buf); err != nil {
			t.Fatalf("предмет %d: %v", i, err)
		}
		out[i] = buf.Bytes()
	}
	return out
}

func TestPackOpen(t *testing.T) {
	files := goodFiles()
	data := packZip(t, files, nil, entry{name: "LICENSE.txt", data: []byte("CC0")}, entry{name: "Лицензия.txt", data: []byte("x")},
		entry{name: "bg/", data: nil})
	a, err := openBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	got := extractAll(t, a)
	for i, f := range files {
		if !bytes.Equal(got[i], f.data) {
			t.Errorf("%s: байты не те", f.name)
		}
	}
	if a.Extract(len(files), io.Discard) == nil || a.Extract(-1, io.Discard) == nil {
		t.Error("предмет вне манифеста читается")
	}
	// Open с диска: хэш архива, обычный файл, Close.
	dir := testkit.TempDir(t)
	p := filepath.Join(dir, "ivory-1.zip")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	fa, err := Open(p, "0.16.0")
	if err != nil || fa.SHA256 != sum(data) || len(fa.Manifest.Items) != 4 {
		t.Fatalf("Open: %v %+v", err, fa)
	}
	if err := fa.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir, "0.16.0"); err == nil {
		t.Error("папка открылась как набор")
	}
	if _, err := Open(filepath.Join(dir, "нет.zip"), "0.16.0"); err == nil || strings.Contains(err.Error(), dir) {
		t.Errorf("нет файла: %v", err)
	}
}

// Архив, собранный скриптом выпуска D1 (scripts/library-pack.mjs): манифест без лишнего,
// LICENSE.txt рядом, шаблон Deflate, картинки Store.
func TestPackD1Archive(t *testing.T) {
	a, err := Open(filepath.Join("testdata", "d1-ivory.zip"), "0.16.0")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close() //nolint:errcheck // тест: чтение
	m := a.Manifest
	if m.ID != "ivory" || m.Version != 1 || m.Title != "Свадьба — айвори" || m.License != "CC0-1.0" || len(m.Items) != 4 || len(m.Skipped) != 0 {
		t.Fatalf("манифест D1: %+v", m)
	}
	kinds := ""
	for _, it := range m.Items {
		kinds += it.Kind + " "
	}
	if kinds != "bg bg sticker spread " {
		t.Errorf("виды: %s", kinds)
	}
	got := extractAll(t, a)
	if !bytes.HasPrefix(got[0], []byte("\x89PNG")) || !bytes.Contains(got[3], []byte(`"places"`)) {
		t.Error("содержимое D1 не то")
	}
}

func TestPackZipLimits(t *testing.T) {
	good := packZip(t, goodFiles(), nil)
	if _, err := OpenReader(bytes.NewReader(good), MaxZip+1, "0.16.0"); err == nil {
		t.Error("архив больше 300 МБ принят")
	}
	if _, err := openBytes([]byte("не zip")); !errors.Is(err, ErrDamaged) {
		t.Errorf("не zip: %v", err)
	}
	if _, err := openBytes(zipOf(t, goodFiles())); !errors.Is(err, ErrDamaged) {
		t.Errorf("без манифеста: %v", err)
	}
	many := make([]entry, MaxEntries)
	for i := range many {
		many[i] = entry{name: "x/" + strings.Repeat("a", i/26+1) + string(rune('a'+i%26)), data: []byte{1}}
	}
	if _, err := openBytes(packZip(t, goodFiles(), nil, many...)); err == nil || !strings.Contains(err.Error(), "2000") {
		t.Errorf("больше 2000 записей: %v", err)
	}
	// Предмет из манифеста, которого нет в архиве, и размер записи не как в манифесте.
	files := goodFiles()
	if _, err := openBytes(packZip(t, files[:3], func(m map[string]any) {
		m["items"] = append(m["items"].([]any), map[string]any{"file": "spreads/none.json", "kind": "spread", "sha256": sumA, "size": 3})
	})); !errors.Is(err, ErrDamaged) {
		t.Errorf("нет записи предмета: %v", err)
	}
	if _, err := openBytes(packZip(t, files, func(m map[string]any) {
		m["items"].([]any)[0].(map[string]any)["size"] = len(files[0].data) - 1
	})); !errors.Is(err, ErrDamaged) {
		t.Errorf("размер не как в манифесте: %v", err)
	}
	// Манифест больше 256 КБ (заголовок) — отказ до чтения.
	if _, err := openBytes(zipOf(t, []entry{{name: ManifestName, data: bytes.Repeat([]byte(" "), MaxManifest+1)}})); !errors.Is(err, ErrManifest) {
		t.Errorf("большой манифест: %v", err)
	}
	// Шифрование и чужой метод сжатия.
	for name, hdr := range map[string]*zip.FileHeader{
		"шифрование": {Name: "bg/ivory-01.jpg", Method: zip.Store, Flags: 0x1},
		"метод bzip": {Name: "bg/ivory-01.jpg", Method: 12},
	} {
		d := files[0].data
		hdr.CRC32, hdr.CompressedSize64, hdr.UncompressedSize64 = crc32.ChecksumIEEE(d), uint64(len(d)), uint64(len(d))
		if _, err := openBytes(packZip(t, files[1:], nil, entry{raw: hdr, body: d})); err == nil {
			t.Errorf("%s: принят", name)
		}
	}
}

// Бомба: заголовок врёт о размере (меньше настоящего) или степень сжатия больше 200×.
func TestPackBomb(t *testing.T) {
	zeros := make([]byte, 8<<20)
	var comp bytes.Buffer
	fw, _ := flate.NewWriter(&comp, flate.BestCompression)
	_, _ = fw.Write(zeros)
	_ = fw.Close()
	// Честный заголовок: 8 МБ нулей сжаты в ~8 КБ — больше 200× + 1 МБ.
	honest := &zip.FileHeader{Name: "bg/zero.png", Method: zip.Deflate, CRC32: crc32.ChecksumIEEE(zeros),
		CompressedSize64: uint64(comp.Len()), UncompressedSize64: uint64(len(zeros))}
	m := func(size int, s string) func(m map[string]any) {
		return func(m map[string]any) {
			m["items"] = []any{map[string]any{"file": "bg/zero.png", "kind": "bg", "sha256": s, "size": size}}
		}
	}
	if _, err := openBytes(packZip(t, nil, m(len(zeros), sum(zeros)), entry{raw: honest, body: comp.Bytes()})); !errors.Is(err, ErrDamaged) ||
		!strings.Contains(err.Error(), "бомб") {
		t.Errorf("степень сжатия: %v", err)
	}
	// Лживый заголовок: 1000 байт по заголовку и манифесту, на деле 8 МБ — читается не больше 1000.
	liar := *honest
	liar.UncompressedSize64 = 1000
	a, err := openBytes(packZip(t, nil, m(1000, sum(zeros[:1000])), entry{raw: &liar, body: comp.Bytes()}))
	if err != nil {
		t.Fatal(err)
	}
	var cw countWriter
	if err := a.Extract(0, &cw); !errors.Is(err, ErrDamaged) || cw.n > 1000 {
		t.Errorf("лживый заголовок: %v, прочитано %d", err, cw.n)
	}
	// Короче заголовка: архив обрезан или подменён.
	body := []byte("несжимаемое тело записи")
	short := &zip.FileHeader{Name: "bg/zero.png", Method: zip.Store, CRC32: crc32.ChecksumIEEE(body),
		CompressedSize64: uint64(len(body)), UncompressedSize64: uint64(len(body)) + 10}
	a, err = openBytes(packZip(t, nil, m(len(body)+10, sumA), entry{raw: short, body: body}))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Extract(0, io.Discard); !errors.Is(err, ErrDamaged) || !strings.Contains(err.Error(), "размер") {
		t.Errorf("запись короче заголовка: %v", err)
	}
}

type countWriter struct{ n int64 }

func (c *countWriter) Write(p []byte) (int, error) { c.n += int64(len(p)); return len(p), nil }

func TestPackSlipNames(t *testing.T) {
	for _, name := range []string{"../evil.png", "bg/../../evil.png", "C:\\evil.png", "c:evil.png", "/abs.png",
		"a\\b.png", "bg//x.png", "./x.png", "bg/\x01.png"} {
		data := packZip(t, goodFiles(), nil, entry{name: name, data: []byte("x")})
		if _, err := openBytes(data); err == nil {
			t.Errorf("%q: принят", name)
		}
	}
	// Повтор без учёта регистра — и у предмета, и у постороннего файла.
	for _, dup := range []string{"BG/Ivory-01.jpg", "MANIFEST.json"} {
		if _, err := openBytes(packZip(t, goodFiles(), nil, entry{name: dup, data: []byte("x")})); err == nil {
			t.Errorf("повтор %s: принят", dup)
		}
	}
}

func TestPackHashMismatch(t *testing.T) {
	files := goodFiles()
	data := packZip(t, files, func(m map[string]any) { m["items"].([]any)[2].(map[string]any)["sha256"] = sumA })
	a, err := openBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Extract(0, io.Discard); err != nil {
		t.Errorf("хороший предмет: %v", err)
	}
	if err := a.Extract(2, io.Discard); !errors.Is(err, ErrDamaged) || !strings.Contains(err.Error(), "sha256") {
		t.Errorf("подмена: %v", err)
	}
	// Ошибка записи (диск) — не «архив повреждён».
	if err := a.Extract(0, failWriter{}); errors.Is(err, ErrDamaged) || err == nil {
		t.Errorf("ошибка диска: %v", err)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("нет места") }

func TestPackNoSVG(t *testing.T) {
	files := append(goodFiles(), entry{name: "stickers/rose.svg", data: []byte("<svg/>")})
	a, err := openBytes(packZip(t, files, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Manifest.Items) != 4 || len(a.Manifest.Skipped) != 1 || a.Manifest.Skipped[0].File != "stickers/rose.svg" {
		t.Fatalf("SVG: %+v", a.Manifest)
	}
	if _, err := openBytes(packZip(t, files[4:], nil)); !errors.Is(err, ErrEmpty) {
		t.Errorf("только SVG: %v", err)
	}
}

func FuzzOpenPack(f *testing.F) {
	f.Add(packZip(f, goodFiles(), nil))
	if d1, err := os.ReadFile(filepath.Join("testdata", "d1-ivory.zip")); err == nil {
		f.Add(d1)
	}
	f.Add([]byte("PK\x05\x06" + strings.Repeat("\x00", 18)))
	f.Fuzz(func(t *testing.T, data []byte) {
		a, err := openBytes(data)
		if err != nil {
			return
		}
		for i, it := range a.Manifest.Items {
			var cw countWriter
			err := a.Extract(i, &cw)
			if cw.n > it.Size || err == nil && cw.n != it.Size {
				t.Fatalf("%s: прочитано %d из %d (%v)", it.File, cw.n, it.Size, err)
			}
		}
	})
}
