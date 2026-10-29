package pack

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Паритет проверки архива с scripts/pack-check.mjs (спека 143, ступень 1): те же случаи testdata/pack-archive-cases.json
// читает scripts/pack-check-zip.test.mjs. Расходятся итоги — автор набора увидит в скрипте не то, что скажет программа.

// archiveFile — запись случая: имя (текстом или байтами), содержимое, создатель, внешние атрибуты и дескриптор данных.
type archiveFile struct {
	Name    string  `json:"name"`
	NameHex string  `json:"nameHex"`
	Text    string  `json:"text"`
	MadeBy  *uint16 `json:"madeBy"`
	Attrs   uint32  `json:"attrs"`
	DD      bool    `json:"dd"`
}

func (f archiveFile) rawName(t *testing.T) string {
	t.Helper()
	if f.NameHex == "" {
		return f.Name
	}
	b, err := hex.DecodeString(f.NameHex)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type archiveCase struct {
	Name         string        `json:"name"`
	Prefix       int           `json:"prefix"`
	EOCDRecords  *uint16       `json:"eocdRecords"`
	DirSizeMinus int           `json:"dirSizeMinus"`
	Files        []archiveFile `json:"files"`
	Extra        []archiveFile `json:"extra"`
	Want         struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	} `json:"want"`
}

// caseArchive собирает архив случая: манифест, файлы, лишние записи; все — Store, как у скрипта.
func caseArchive(t *testing.T, c archiveCase) []byte {
	t.Helper()
	items := make([]any, 0, len(c.Files))
	for _, f := range c.Files {
		items = append(items, map[string]any{"file": f.Name, "kind": KindSpread, "theme": "basic", "name": "n",
			"sha256": sum([]byte(f.Text)), "size": len(f.Text)})
	}
	mdata, err := json.Marshal(map[string]any{"format": 1, "id": "arch-test", "version": 1, "minApp": "0.1.0", "title": "Т", "items": items})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	buf.Write(make([]byte, c.Prefix))
	zw := zip.NewWriter(&buf)
	add := func(name, text string, madeBy *uint16, attrs uint32, dd bool) {
		fh := &zip.FileHeader{Name: name, Method: zip.Store, CRC32: crc32.ChecksumIEEE([]byte(text)),
			CompressedSize64: uint64(len(text)), UncompressedSize64: uint64(len(text)), ExternalAttrs: attrs}
		if dd {
			fh.Flags = 0x8
		}
		if madeBy != nil {
			fh.CreatorVersion = *madeBy
		}
		w, err := zw.CreateRaw(fh)
		if err == nil {
			_, err = io.WriteString(w, text)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	add(ManifestName, string(mdata), nil, 0, false)
	for _, f := range c.Files {
		add(f.rawName(t), f.Text, f.MadeBy, f.Attrs, f.DD)
	}
	for _, f := range c.Extra {
		add(f.rawName(t), f.Text, f.MadeBy, f.Attrs, f.DD)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	p := bytes.LastIndex(b, []byte("PK\x05\x06"))
	if c.DirSizeMinus != 0 { // размер каталога в конце не сходится с настоящим
		binary.LittleEndian.PutUint32(b[p+12:], uint32(int(binary.LittleEndian.Uint32(b[p+12:]))-c.DirSizeMinus)) //nolint:gosec // размер каталога — малое число
	}
	if c.EOCDRecords != nil { // число записей в конце каталога (диск и всего)
		binary.LittleEndian.PutUint16(b[p+8:], *c.EOCDRecords)
		binary.LittleEndian.PutUint16(b[p+10:], *c.EOCDRecords)
	}
	return b
}

func TestArchiveCases(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "pack-archive-cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []archiveCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) < 20 {
		t.Fatalf("случаев %d: файл урезан", len(file.Cases))
	}
	names := map[string]bool{}
	for _, c := range file.Cases {
		if names[c.Name] {
			t.Errorf("случай «%s» повторяется", c.Name)
		}
		names[c.Name] = true
		b := caseArchive(t, c)
		a, err := OpenReader(bytes.NewReader(b), int64(len(b)), "")
		for i := 0; err == nil && i < len(c.Files); i++ {
			err = a.Extract(i, io.Discard)
		}
		switch {
		case c.Want.OK && err != nil:
			t.Errorf("%s: отказ «%v», ждали успех", c.Name, err)
		case !c.Want.OK && err == nil:
			t.Errorf("%s: успех, ждали отказ «%s»", c.Name, c.Want.Error)
		case !c.Want.OK && !strings.Contains(err.Error(), c.Want.Error):
			t.Errorf("%s: отказ «%v», ждали «%s»", c.Name, err, c.Want.Error)
		}
	}
}
