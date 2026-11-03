package pack

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

// zip64Of переписывает конец каталога архива без комментария в ZIP64: запись конца ZIP64 с
// records записями, локатор и конец каталога с 0xffff — archive/zip берёт числа из ZIP64.
func zip64Of(t *testing.T, z []byte, records uint64) []byte {
	t.Helper()
	at := len(z) - eocdLen
	if binary.LittleEndian.Uint32(z[at:]) != 0x06054b50 {
		t.Fatal("нет конца каталога")
	}
	dirSize, dirOff := binary.LittleEndian.Uint32(z[at+12:]), binary.LittleEndian.Uint32(z[at+16:])
	out := bytes.Clone(z[:at])
	le := binary.LittleEndian
	out = le.AppendUint32(out, 0x06064b50)
	out = le.AppendUint64(out, end64-12)
	out = le.AppendUint16(out, 45)
	out = le.AppendUint16(out, 45)
	out = le.AppendUint32(out, 0)
	out = le.AppendUint32(out, 0)
	out = le.AppendUint64(out, records)
	out = le.AppendUint64(out, records)
	out = le.AppendUint64(out, uint64(dirSize))
	out = le.AppendUint64(out, uint64(dirOff))
	out = le.AppendUint32(out, 0x07064b50)
	out = le.AppendUint32(out, 0)
	out = le.AppendUint64(out, uint64(at))
	out = le.AppendUint32(out, 1)
	eocd := bytes.Clone(z[at:])
	le.PutUint16(eocd[8:], 0xffff)
	le.PutUint16(eocd[10:], 0xffff)
	le.PutUint32(eocd[16:], 0xffffffff)
	return append(out, eocd...)
}

// rewrap — архив z заново: те же записи, ещё n записей с комментарием 1100 Б (каталог растёт,
// число записей — нет) и комментарий архива comment.
func rewrap(t *testing.T, z []byte, n int, comment string) []byte {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(z), int64(len(z)))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range r.File {
		if err := zw.Copy(f); err != nil {
			t.Fatal(err)
		}
	}
	for i := range n {
		name := "x/" + strings.Repeat("a", i/26+1) + string(rune('a'+i%26))
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store, Comment: strings.Repeat("к", 550)})
		if err == nil {
			_, err = w.Write([]byte{1})
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.SetComment(comment); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// Число записей и размер каталога проверяются по концу каталога до разбора: объявлено больше
// 2000 записей (и в ZIP64) или каталог больше 1 МБ — отказ; годный ZIP64 и длинный комментарий
// архива — открываются.
func TestPackZipDirectory(t *testing.T) {
	good := packZip(t, goodFiles(), nil)
	if _, err := openBytes(good); err != nil {
		t.Fatalf("годный: %v", err)
	}
	// Конец каталога объявляет 60000 записей.
	many := bytes.Clone(good)
	binary.LittleEndian.PutUint16(many[len(many)-eocdLen+8:], 60000)
	binary.LittleEndian.PutUint16(many[len(many)-eocdLen+10:], 60000)
	if _, err := openBytes(many); err == nil || !strings.Contains(err.Error(), "2000") {
		t.Errorf("60000 записей в конце каталога: %v", err)
	}
	// ZIP64: 2⁴⁰ записей — отказ; столько, сколько есть, — открывается (разбор ZIP64 верный).
	if _, err := openBytes(zip64Of(t, good, 1<<40)); err == nil || !strings.Contains(err.Error(), "2000") {
		t.Errorf("ZIP64 с 2^40 записей: %v", err)
	}
	if _, err := openBytes(zip64Of(t, good, MaxEntries+1)); err == nil || !strings.Contains(err.Error(), "2000") {
		t.Errorf("ZIP64 с 2001 записью: %v", err)
	}
	if _, err := openBytes(zip64Of(t, good, uint64(len(goodFiles())+1))); err != nil {
		t.Errorf("годный ZIP64: %v", err)
	}
	// Каталог больше 1 МБ при записях меньше 2000: длинные комментарии записей.
	if _, err := openBytes(rewrap(t, good, 1000, "")); !errors.Is(err, ErrDamaged) || !strings.Contains(err.Error(), "каталог") {
		t.Errorf("каталог больше 1 МБ: %v", err)
	}
	if _, err := openBytes(rewrap(t, good, 800, "")); err != nil {
		t.Errorf("каталог меньше 1 МБ: %v", err)
	}
	// Комментарий архива почти 64 КБ: конец каталога ищется, как в archive/zip.
	if _, err := openBytes(rewrap(t, good, 0, strings.Repeat("z", 65000))); err != nil {
		t.Errorf("с комментарием архива: %v", err)
	}
	// Мусор и пустота — отказ разбора, не паника.
	for _, b := range [][]byte{nil, []byte("PK\x05\x06"), bytes.Repeat([]byte("PK\x05\x06"), 10)} {
		if _, err := openBytes(b); err == nil {
			t.Errorf("мусор %q принят", b)
		}
	}
}

// id набора — не устройство Windows (имя реестра <id>.json), в любом регистре.
func TestPackIDDevice(t *testing.T) {
	for _, id := range []string{"con", "prn", "aux", "nul", "com0", "com1", "com9", "lpt1", "lpt9"} {
		if ValidID(id) {
			t.Errorf("%s принят", id)
		}
		if !deviceName(strings.ToUpper(id)) {
			t.Errorf("%s в верхнем регистре — не устройство", id)
		}
	}
	for _, id := range []string{"CONIN$", "conout$", "Com²"} {
		if !deviceName(id) {
			t.Errorf("%s — не устройство", id)
		}
	}
	for _, id := range []string{"conx", "com10", "console", "nul-pack", "lpt", "ivory", "aux1"} {
		if !ValidID(id) {
			t.Errorf("%s отклонён", id)
		}
	}
	if _, err := ParseManifest(manifestJSON(t, func(m map[string]any) { m["id"] = "nul" }), "0.16.0"); err == nil {
		t.Error("манифест с id nul принят")
	}
	if _, err := ParseManifest(manifestJSON(t, func(m map[string]any) { m["id"] = "nul-pack" }), "0.16.0"); err != nil {
		t.Errorf("манифест с id nul-pack: %v", err)
	}
}
