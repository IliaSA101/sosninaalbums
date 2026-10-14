package salbum

// Предел 64 фото и места на разворот при чтении файла (спека 027, формат v6).

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestLoadPlaces64: 65 мест в файле → 64 (лишнее пустое с конца), все 64 фото на местах.
func TestLoadPlaces64(t *testing.T) {
	if model.MaxPlaces != 64 || model.MaxPhotos != 64 {
		t.Fatalf("MaxPlaces = %d, MaxPhotos = %d", model.MaxPlaces, model.MaxPhotos)
	}
	dir := testkit.TempDir(t)
	var names []string
	for i := range 64 {
		names = append(names, fmt.Sprint("g", i))
	}
	_, ids := gridFile(t, dir, names, nil)
	s := &model.Spread{ID: "s"}
	for i := range 65 {
		ph := ""
		if i < 64 {
			ph = ids[names[i]]
			s.Photos = append(s.Photos, ph)
		}
		s.Grid = append(s.Grid, box(model.NewID(), float64(i%8)*30, float64(i/8)*30, 20, 20, ph))
	}
	data, _ := gridFile(t, dir, names, []*model.Spread{s})
	p := mustDecode(t, data, dir)
	if len(p.Spreads) != 1 || len(p.Spreads[0].Grid) != 64 || len(model.GridPhotos(p.Spreads[0])) != 64 {
		t.Fatalf("мест %d, фото на местах %d", len(p.Spreads[0].Grid), len(model.GridPhotos(p.Spreads[0])))
	}
}

// TestLoadSplitBig64: разворот из 100 фото (чужой или битый файл) режется на 64 + 36.
func TestLoadSplitBig64(t *testing.T) {
	p, dir := model.New(), testkit.TempDir(t)
	ids := make([]string, 100)
	for i := range ids {
		path := filepath.Join(dir, fmt.Sprint("p", i, ".jpg"))
		ids[i] = library.IDFor(path)
		p.Photos[ids[i]] = model.PhotoRef{Path: path, W: 3, H: 2}
	}
	p.Spreads = []*model.Spread{{ID: "s", Photos: ids}}
	path := filepath.Join(dir, "a.salbum")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil || len(q.Spreads) != 2 || len(q.Spreads[0].Photos) != 64 || len(q.Spreads[1].Photos) != 36 {
		t.Fatalf("ждали 64 + 36: err=%v, разворотов %d", err, len(q.Spreads))
	}
}
