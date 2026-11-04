package salbum

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// WriteRel (069): у фото, папки, разметок и отметки внутри папки альбома — только относительный
// путь, без abs; фото уровнем выше — только абсолютный (не «..»: папку переносят целиком);
// пропавшее фото — абсолютный и запасной как был. Load читает то же; Write того же проекта —
// прежний вид (относительный с abs рядом); без папки альбома — отказ.
func TestWriteRelNoAbs(t *testing.T) {
	root := testkit.TempDir(t)
	base := filepath.Join(root, "Сбор", "Иванов")
	in, up := filepath.Join(base, "Фото", "a.jpg"), filepath.Join(root, "Сбор", "b.jpg")
	gone, alt := filepath.Join(root, "Нет", "c.jpg"), filepath.Join(root, "Старое", "c.jpg")
	guide := filepath.Join(base, "Разметка", "g.psd")
	for _, f := range []string{in, up, guide} {
		putFile(t, f, 10)
	}
	p := model.New()
	p.Folders = []string{filepath.Join(base, "Фото")}
	ids := map[string]string{}
	for _, f := range []string{in, up, gone} {
		ids[f] = library.IDFor(f)
		p.Photos[ids[f]] = model.PhotoRef{Path: f, W: 3, H: 2, Size: 10}
	}
	ref := p.Photos[ids[gone]]
	ref.Alt = alt
	p.Photos[ids[gone]] = ref
	p.Spreads = []*model.Spread{{ID: "0123456789a1", Photos: []string{ids[in], ids[up], ids[gone]}}}
	p.SpreadGuide = guide
	five := 5
	marks := map[string]model.Mark{ids[in]: {Path: in, Rating: &five}}
	path := filepath.Join(base, "Иванов.salbum")
	if err := WriteRel(p, path, base, model.Side{Marks: marks}); err != nil {
		t.Fatal(err)
	}
	f := readFile(t, path)
	if ff := f.Folders; len(ff) != 1 || ff[0] != (FileFolder{Path: "Фото"}) {
		t.Fatalf("папки: %+v", ff)
	}
	for id, want := range map[string][2]string{ids[in]: {"Фото/a.jpg", ""}, ids[up]: {up, ""}, ids[gone]: {gone, alt}} {
		if got := f.Photos[id]; got.Path != want[0] || got.Abs != want[1] {
			t.Errorf("фото %s: path %q abs %q, ждали %q", id, got.Path, got.Abs, want)
		}
	}
	if f.SpreadGuide != "Разметка/g.psd" || f.SpreadGuideAbs != "" {
		t.Errorf("разметка: %q, %q", f.SpreadGuide, f.SpreadGuideAbs)
	}
	if len(f.Marks) != 1 || f.Marks[0].Path != "Фото/a.jpg" || f.Marks[0].Abs != "" {
		t.Errorf("отметки: %+v", f.Marks)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if q.Photos[ids[in]].Path != in || q.Photos[ids[up]].Path != up || q.Photos[ids[gone]].Alt != alt || q.SpreadGuide != guide {
		t.Fatalf("Load: %+v, разметка %q", q.Photos, q.SpreadGuide)
	}
	if m := q.Loaded().Marks[ids[in]]; m.Path != in {
		t.Fatalf("отметка после Load: %+v", q.Loaded().Marks)
	}
	// Write — как раньше: относительный с abs рядом, «..» допустим.
	if err := Write(p, path, base, model.Side{Marks: marks}); err != nil {
		t.Fatal(err)
	}
	f = readFile(t, path)
	if got := f.Photos[ids[in]]; got.Path != "Фото/a.jpg" || got.Abs != in {
		t.Errorf("Write: %+v", got)
	}
	if got := f.Photos[ids[up]]; got.Path != "../b.jpg" || got.Abs != up {
		t.Errorf("Write уровнем выше: %+v", got)
	}
	if err := WriteRel(p, path, "", model.Side{Marks: marks}); err == nil {
		t.Fatal("WriteRel без папки альбома принят")
	}
}

func readFile(t *testing.T, path string) FileProject {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f FileProject
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"abs": ""`)) {
		t.Fatal("пустой abs в файле")
	}
	return f
}

// Антивирус переименовывает заменяемый или удаляемый файл в <имя>.tmp и держит открытым:
// запись проекта не должна зависеть от этого имени, чужой файл — не тронут.
func TestWriteSurvivesHeldDotTmp(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("занятость имени — поведение Windows")
	}
	dir := testkit.TempDir(t)
	for _, name := range []string{"черновик.salbum", "альбом.salbum"} { // несколько файлов в одной папке
		path := filepath.Join(dir, name)
		orig := []byte("старый " + name)
		if err := os.WriteFile(path+".tmp", orig, 0o644); err != nil {
			t.Fatal(err)
		}
		held, err := os.Open(path + ".tmp") // как дескриптор антивируса: без FILE_SHARE_DELETE
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = held.Close() })
		for range 2 { // второй раз — поверх уже записанного
			if err := Write(model.New(), path, "", model.Side{}); err != nil {
				t.Fatalf("запись при занятом %s.tmp: %v", name, err)
			}
		}
		if _, err := Load(path); err != nil {
			t.Fatalf("записанный %s не читается: %v", name, err)
		}
		if got, err := os.ReadFile(path + ".tmp"); err != nil || !bytes.Equal(got, orig) {
			t.Fatalf("занятый %s.tmp тронут: %q, %v", name, got, err)
		}
		noTmp(t, path)
	}
}
