package salbum

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Тон фото в файле (068, v12): photos.*.adj, omitempty.

// toneProject — разворот из трёх фото в dir: у a ч/б с тоном, у b только контраст, у c без тона.
func toneProject(dir string) (*model.Project, [3]string) {
	p := model.New()
	var ids [3]string
	tones := []*model.Tone{{Gray: true, Bright: 50, Contrast: -50}, {Contrast: 7}, nil}
	for i, name := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		path := filepath.Join(dir, name)
		ids[i] = library.IDFor(path)
		p.Photos[ids[i]] = model.PhotoRef{Path: path, W: 300, H: 200, Adj: tones[i]}
	}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: ids[:]}}
	return p, ids
}

func TestV12ToneRoundtrip(t *testing.T) {
	dir := testkit.TempDir(t)
	p, ids := toneProject(dir)
	path := filepath.Join(dir, "a.salbum")
	first, q := saveLoad(t, p, path)
	if f := fileJSON(t, path); f.Version != 13 || bytes.Count(first, []byte(`"adj"`)) != 2 {
		t.Fatalf("файл v%d:\n%s", f.Version, first)
	}
	if !bytes.Contains(first, []byte(`"gray": true`)) || !bytes.Contains(first, []byte(`"c": 7`)) {
		t.Fatalf("поля тона в файле:\n%s", first)
	}
	if a := q.Photos[ids[0]].Adj; a == nil || *a != (model.Tone{Gray: true, Bright: 50, Contrast: -50}) {
		t.Fatalf("a: %+v", a)
	}
	if b := q.Photos[ids[1]].Adj; b == nil || *b != (model.Tone{Contrast: 7}) {
		t.Fatalf("b: %+v", b)
	}
	if q.Photos[ids[2]].Adj != nil {
		t.Fatal("c получил тон")
	}
	second, _ := saveLoad(t, q, path)
	if !bytes.Equal(first, second) {
		t.Fatalf("второй круг отличается:\n%s\n%s", first, second)
	}
}

// v11 (0.15) открывается без тона; первая запись — текущая версия без adj, BackupOld кладёт .v11.bak с исходником.
func TestV11LoadsWithoutTone(t *testing.T) {
	dir := testkit.TempDir(t)
	p, ids := toneProject(dir)
	for _, id := range ids {
		r := p.Photos[id]
		r.Adj = nil
		p.Photos[id] = r
	}
	data, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	src := bytes.Replace(data, fmt.Appendf(nil, `"version": %d,`, model.FormatVersion), []byte(`"version": 11,`), 1)
	path := filepath.Join(dir, "old.salbum")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if q.From() != 11 || q.Version != model.FormatVersion {
		t.Fatalf("from %d, версия %d", q.From(), q.Version)
	}
	for _, id := range ids {
		if q.Photos[id].Adj != nil {
			t.Fatalf("тон у %s в файле v11", id)
		}
	}
	if err := BackupOld(path, q.From()); err != nil {
		t.Fatal(err)
	}
	if err := Save(q, path); err != nil {
		t.Fatal(err)
	}
	if bak, err := os.ReadFile(path + ".v11.bak"); err != nil || !bytes.Equal(bak, src) {
		t.Fatalf(".v11.bak: %v", err)
	}
	if out, _ := os.ReadFile(path); bytes.Contains(out, []byte(`"adj"`)) || fileJSON(t, path).Version != 13 {
		t.Fatalf("запись v11:\n%s", out)
	}
}

// Мусор из файла зажимается при Load, нулевой тон снимается; дробь в тоне — файл не принят.
func TestLoadClampsTone(t *testing.T) {
	dir := testkit.TempDir(t)
	ph := func(name, adj string) (string, string) {
		path := filepath.Join(dir, name)
		b, _ := json.Marshal(path)
		id := library.IDFor(path)
		return id, fmt.Sprintf(`%q:{"path":%s,"w":300,"h":200,"adj":%s}`, id, b, adj)
	}
	a, ja := ph("a.jpg", `{"b":900,"c":-1000000000}`)
	b, jb := ph("b.jpg", `{}`)
	c, jc := ph("c.jpg", `{"gray":true,"b":-51}`)
	file := fmt.Appendf(nil, `{"version":12,"settings":{"pageW":281,"pageH":284},"spreads":[{"id":"s1","photos":[%q,%q,%q]}],"photos":{%s,%s,%s}}`,
		a, b, c, ja, jb, jc)
	p := mustDecode(t, file, dir)
	if got := p.Photos[a].Adj; got == nil || *got != (model.Tone{Bright: 50, Contrast: -50}) {
		t.Errorf("a: %+v", got)
	}
	if got := p.Photos[b].Adj; got != nil {
		t.Errorf("пустой тон не снят: %+v", got)
	}
	if got := p.Photos[c].Adj; got == nil || *got != (model.Tone{Gray: true, Bright: -50}) {
		t.Errorf("c: %+v", got)
	}
	bad := bytes.Replace(file, []byte(`"b":900`), []byte(`"b":1.5`), 1)
	if _, err := Decode(bad, dir); err == nil {
		t.Error("дробная яркость принята")
	}
}
