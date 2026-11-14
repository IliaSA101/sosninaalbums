package app

import (
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Фирменные стикеры (спека 116): все четыре на двух листах, один шаг на каждый; в альбом уходит
// ключ генератора (без пути к файлу), поэтому круг записи и чтения не нуждается в библиотеке;
// замена на другой фирменный — рамка по новой пропорции; та же плитка — холостая; отмена.
func TestBrandStickersInAlbum(t *testing.T) {
	a := decorApp(t)
	ids := []string{"sa:st-brand-albums-black", "sa:st-brand-albums-white", "sa:st-brand-films-black", "sa:st-brand-films-white"}
	kinds := []string{"brand-albums-black", "brand-albums-white", "brand-films-black", "brand-films-white"}
	cat, err := a.DecorCatalog(decor.KindSticker)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, it := range cat.Items {
		have[it.ID] = it.Theme == decor.ThemeBrand && !it.Own
	}
	var added []string
	rev := a.editor().View().Rev
	for i, id := range ids {
		if !have[id] {
			t.Fatalf("%s нет в каталоге", id)
		}
		r, err := a.AddSticker([]string{"s1", "s2"}[i%2], 0, id)
		if err != nil || r.ID == "" || r.View.Rev != rev+i+1 {
			t.Fatalf("%s: %v %q rev %d", id, err, r.ID, r.View.Rev)
		}
		st := stickerAt(a, r.ID)
		if st == nil || st.Art == nil || st.Art.Kind != kinds[i] || st.Photo != "" || len(st.Art.Inks) != 0 || st.W <= st.H*2 {
			t.Fatalf("%s: %+v", id, st)
		}
		added = append(added, r.ID)
	}

	path := filepath.Join(testkit.TempDir(t), "brand.salbum")
	if err := a.editor().Save(path); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range added {
		s, k := p.FindSticker(id)
		if s == nil || s.Stickers[k].Art == nil || s.Stickers[k].Art.Kind != kinds[i] || s.Stickers[k].Photo != "" {
			t.Errorf("%s после чтения: %v", id, s)
		}
	}
	for _, ph := range p.Photos {
		if ph.Lib != "" || ph.Path != "" {
			t.Errorf("в альбоме запись фото от фирменного стикера: %+v", ph)
		}
	}

	// Замена: центр и ширина стоят, высота — по пропорции другого файла.
	before := stickerAt(a, added[0])
	v, err := a.ReplaceSticker("s1", added[0], ids[2])
	got := stickerAt(a, added[0])
	if err != nil || got.Art.Kind != kinds[2] || !near(got.W, before.W) || near(got.H, before.H) ||
		!near(got.Y+got.H/2, before.Y+before.H/2) {
		t.Fatalf("замена: %v %+v, было %+v", err, got, before)
	}
	if v2, err := a.ReplaceSticker("s1", added[0], ids[2]); err != nil || v2.Rev != v.Rev {
		t.Errorf("та же плитка — правка: %v rev %d→%d", err, v.Rev, v2.Rev)
	}
	a.Undo()
	if got := stickerAt(a, added[0]); got.Art.Kind != kinds[0] || !near(got.H, before.H) {
		t.Errorf("отмена замены: %+v", got)
	}
	a.Undo()
	if stickerAt(a, added[3]) != nil || stickerAt(a, added[2]) == nil {
		t.Error("отмена не сняла последний стикер")
	}
}
