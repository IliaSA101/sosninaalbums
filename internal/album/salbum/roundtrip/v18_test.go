package roundtrip_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Формат v18 (общий на 0.24): подрезка settings.crop (104). Буквы ключа «Разброс» (105) лягут в
// этот же v18 — их голден и круг добавит спека 105.

var writeGolden18Crop = flag.Bool("golden18crop", false, "перезаписать testdata/v18-crop.salbum.json из cropGolden")

const (
	golden18Crop = "v18-crop.salbum.json"
	crop18Key    = "H(V(0,H(1,2)),3)" // 4 фото: сплошной блок, которому подрезка 10 % закрывает лист
	crop18Pair   = "H(0,1)"
)

// cropGolden — альбом с подрезкой 10 %: два листа «Авто» с выбранными ключами; на первом у фото -2
// свой кадр (asp), которого подрезка не трогает; копии одного файла — чтобы хватило фото на два листа.
func cropGolden(base string, idOf func(path string) string) *model.Project {
	p := model.New()
	p.Settings.Crop = 10
	p.Folders = []string{filepath.Join(base, "Март")}
	a, b := idOf(filepath.Join(base, "Март", "a.jpg")), idOf(filepath.Join(base, "Март", "b.jpg"))
	file := func(name string, w, h int) model.PhotoRef {
		return model.PhotoRef{Path: filepath.Join(base, "Март", name), W: w, H: h, Size: 1000}
	}
	ids := []string{a, model.CopyID(a, 2), model.CopyID(a, 3), b, model.CopyID(a, 4), model.CopyID(b, 2)}
	for i, id := range ids {
		if i == 3 || i == 5 {
			p.Photos[id] = file("b.jpg", 2000, 3000)
		} else {
			p.Photos[id] = file("a.jpg", 3000, 2000)
		}
	}
	own := p.Photos[ids[1]]
	own.Asp = 0.8
	p.Photos[ids[1]] = own
	p.Spreads = []*model.Spread{
		{ID: "5e1f0a2b3c41", Photos: ids[:4], Layout: crop18Key},
		{ID: "5e1f0a2b3c42", Photos: ids[4:], Layout: crop18Pair},
	}
	return p
}

// checkCrop — после чтения: настройка и ключи на месте, ключи строятся с подрезкой, свой кадр не тронут.
func checkCrop(t *testing.T, p *model.Project) {
	t.Helper()
	if p.Settings.Crop != 10 || len(p.Spreads) != 2 || p.Spreads[0].Layout != crop18Key || p.Spreads[1].Layout != crop18Pair {
		t.Fatalf("после чтения: crop %d, развороты %d, ключи не те", p.Settings.Crop, len(p.Spreads))
	}
	for _, s := range p.Spreads {
		asp, prm := p.LayoutIn(s.Photos)
		r, ok := layout.Build(s.Layout, asp, prm)
		if !ok || r.Key != s.Layout {
			t.Fatalf("ключ %q не строится с подрезкой", s.Layout)
		}
	}
	if own := p.OwnFrame(p.Spreads[0].Photos); own[0] || !own[1] || own[2] || own[3] {
		t.Fatalf("свой кадр на первом листе: %v", own)
	}
	if _, prm := p.LayoutIn(p.Spreads[0].Photos); prm.Crop != 0.1 || prm.Lock != 2 {
		t.Fatalf("Params листа: Crop %v, Lock %b", prm.Crop, prm.Lock)
	}
}

// Голден v18: подрезка 10 %, два листа «Авто», свой кадр. Encode = голден (кроме номера версии он
// же прежний формат 17 + settings.crop); Decode → Encode = голден; Save → Load без потерь; ключи
// строятся с подрезкой; повторная запись — те же байты.
func TestCropGoldenV18(t *testing.T) {
	withAppVersion(t, "0.24.0")
	base := albumSetup(t)
	path := filepath.Join("..", "testdata", golden18Crop)
	enc, err := salbum.Encode(cropGolden(base, fixedIDs), base)
	if err != nil {
		t.Fatal(err)
	}
	enc = norm17(enc, base)
	if *writeGolden18Crop {
		if err := os.WriteFile(path, append(enc, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	golden, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	golden = bytes.TrimSuffix(bytes.ReplaceAll(golden, []byte("\r\n"), []byte("\n")), []byte("\n"))
	if !bytes.Equal(enc, golden) {
		t.Fatalf("Encode отличается от голдена:\n%s", enc)
	}
	for _, key := range []string{`"version": 18,`, `"crop": 10`, `"asp": 0.8`, `"app": "0.24.0"`} {
		if !bytes.Contains(golden, []byte(key)) {
			t.Fatalf("в голдене нет %s", key)
		}
	}

	// Decode → Encode: ID записей — по путям, порядок словаря другой.
	p, err := salbum.Decode(denorm17(golden, base), base)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || edtest.From(p) != 18 {
		t.Fatalf("версия %d, from %d", p.Version, edtest.From(p))
	}
	checkCrop(t, p)
	out, err := salbum.Encode(p, base)
	if err != nil {
		t.Fatal(err)
	}
	back := norm17(out, base)
	for _, n := range []string{"a.jpg", "b.jpg"} {
		back = bytes.ReplaceAll(back, []byte(library.IDFor(filepath.Join(base, "Март", n))), []byte(fixedIDs(n)))
	}
	if canonJSON(t, back) != canonJSON(t, golden) {
		t.Fatalf("Decode → Encode отличается от голдена:\n%s", back)
	}

	// Save → Load → Save: проект и байты те же.
	album := filepath.Join(base, "Свадьба.salbum")
	e := edtest.Open(t, p, album)
	if err := e.Save(album); err != nil {
		t.Fatal(err)
	}
	if edtest.Backup(e) {
		t.Fatal("файл текущего формата не требует копии")
	}
	first, err := os.ReadFile(album)
	if err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(album)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(model.Marshal(p), model.Marshal(q)) || edtest.From(q) != 18 {
		t.Fatalf("потери при Save → Load:\n%s\n%s", model.Marshal(p), model.Marshal(q))
	}
	checkCrop(t, q)
	if err := edtest.Open(t, q, album).Save(album); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(album); !bytes.Equal(first, again) {
		t.Fatalf("повторная запись изменила файл:\n%s\n%s", first, again)
	}
}

// v17Bytes — файл v17 с двумя листами «Авто» и без подрезки: Encode текущей версии с номером 17.
func v17Bytes(t *testing.T, base string) []byte {
	t.Helper()
	p := cropGolden(base, library.IDFor)
	p.Settings.Crop = 0
	data, err := salbum.Encode(p, base)
	if err != nil {
		t.Fatal(err)
	}
	out := bytes.Replace(data, verTag(model.FormatVersion), verTag(17), 1)
	if bytes.Equal(out, data) || bytes.Contains(data, []byte(`"crop"`)) {
		t.Fatalf("v17 из Encode не вышел:\n%s", data)
	}
	return out
}

// v17 на диске: Load → Save делает копию «<имя> (для 0.23).salbum» с байтами исходника и пишет v18;
// подрезка 0, ключи и фото целы; потом подрезка включается и переживает круг; копия не трогается.
func TestMigrateV17ToV18(t *testing.T) {
	withAppVersion(t, "0.24.0")
	base := albumSetup(t)
	src := v17Bytes(t, base)
	path := filepath.Join(base, "Свадьба.salbum")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || edtest.From(p) != 17 || p.Settings.Crop != 0 {
		t.Fatalf("версия %d, from %d, crop %d", p.Version, edtest.From(p), p.Settings.Crop)
	}
	if p.Spreads[0].Layout != crop18Key || p.Spreads[1].Layout != crop18Pair {
		t.Fatalf("ключи после чтения v17: %q, %q", p.Spreads[0].Layout, p.Spreads[1].Layout)
	}
	e := edtest.Open(t, p, path)
	if !edtest.Backup(e) {
		t.Fatal("до записи копия нужна, но не сделана")
	}
	before := model.Marshal(p)
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	bak := filepath.Join(base, "Свадьба (для 0.23).salbum")
	if bak != salbum.BackupName(path, 17, 1) {
		t.Fatalf("имя копии %s", salbum.BackupName(path, 17, 1))
	}
	if got, err := os.ReadFile(bak); err != nil || !bytes.Equal(got, src) {
		t.Fatalf("копия для 0.23: %v, байты исходника: %v", err, bytes.Equal(got, src))
	}
	if got := e.TakeBackup(); got != bak {
		t.Fatalf("TakeBackup %q, ждали %q", got, bak)
	}
	if f := fileJSON(t, path); f.Version != 18 || string(f.App) != "0.24.0" {
		t.Fatalf("в файле версия %d, app %q", f.Version, f.App)
	}
	if now, _ := os.ReadFile(path); bytes.Contains(now, []byte(`"crop"`)) {
		t.Fatal("подрезка 0 попала в файл")
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, model.Marshal(q)) || edtest.From(q) != 18 {
		t.Fatalf("потери при v17 → v18:\n%s\n%s", before, model.Marshal(q))
	}

	// Подрезку включили: Save → Load, копия 0.23 осталась как была.
	e2 := edtest.Open(t, q, path)
	edtest.Change(e2, func(p *model.Project) { p.Settings.Crop = 15 })
	if err := e2.Save(path); err != nil {
		t.Fatal(err)
	}
	r, err := salbum.Load(path)
	if err != nil || r.Settings.Crop != 15 {
		t.Fatalf("подрезка после круга: %v, %v", r.Settings.Crop, err)
	}
	if got, _ := os.ReadFile(bak); !bytes.Equal(got, src) || e2.TakeBackup() != "" {
		t.Fatal("копия для 0.23 изменилась или названа снова")
	}
}
