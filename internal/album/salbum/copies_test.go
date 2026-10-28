package salbum

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Копии фото на чтении (098 design §2.3, §7): номер копии живёт в ID и переживает Rekey; записи
// одного пути и номера сливаются, как в 0.22; свойства файла семьи выравниваются.

// copyPlace — место листа с фото photo (x — сдвиг, чтобы места не совпадали).
func copyPlace(n int, x float64, photo string) *model.Place {
	return &model.Place{ID: "c0ffee00000" + string(rune('0'+n)), X: x, Y: 20, W: 80, H: 60, Photo: photo}
}

// copyFile — файл текущей версии: фото photos (ID → запись), листы с местами по одному на фото
// из places (ID листа → ID фото по порядку).
func copyFile(t *testing.T, photos map[string]FilePhoto, sheets [][]string) []byte {
	t.Helper()
	f := FileProject{Version: model.FormatVersion, Settings: model.DefaultSettings(), Folders: []FileFolder{}, Photos: photos}
	n := 0
	for i, ids := range sheets {
		s := &model.Spread{ID: "5e1f0a2b3c4" + string(rune('0'+i)), Photos: []string{}}
		for j, id := range ids {
			n++
			s.Grid = append(s.Grid, copyPlace(n, 20+float64(j)*100, id))
			s.Photos = append(s.Photos, id)
		}
		f.Spreads = append(f.Spreads, s)
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// placed — фото и кадр (S; 0 — без кадра) каждого места листов по порядку; пустое место — "".
func placed(p *model.Project) (ids []string, scales []float64) {
	for _, s := range p.Sheets() {
		for _, pl := range s.Grid {
			ids = append(ids, pl.Photo)
			sc := 0.0
			if fr := p.Photos[pl.Photo].Frame; pl.Photo != "" && fr != nil {
				sc = fr.S
			}
			scales = append(scales, sc)
		}
	}
	return ids, scales
}

// fileRef — запись фото по пути path (относительный в файле, абсолютный рядом) с кадром S.
func fileRef(path, abs string, s float64) FilePhoto {
	fp := FilePhoto{PhotoRef: model.PhotoRef{Path: path, W: 3000, H: 2000, Size: 1000}, Abs: abs}
	if s != 0 {
		fp.Frame = &model.Frame{S: s}
	}
	return fp
}

// Случай критика: фото X и его копия X-2 стоят на двух листах с кадрами S=2 и S=3; альбом
// переехал вместе с фото — ID пересчитываются от нового пути, а номер копии остаётся: оба места и
// оба кадра целы. Без номера в Rekey записи слились бы, второе место опустело бы.
func TestRekeyKeepsCopies(t *testing.T) {
	old, moved := testkit.TempDir(t), testkit.TempDir(t)
	x := library.IDFor(filepath.Join(old, "a.jpg"))
	abs := filepath.Join(old, "a.jpg")
	data := copyFile(t, map[string]FilePhoto{
		x:                  fileRef("a.jpg", abs, 2),
		model.CopyID(x, 2): fileRef("a.jpg", abs, 3),
	}, [][]string{{x}, {model.CopyID(x, 2)}})
	p, err := Decode(data, moved)
	if err != nil {
		t.Fatal(err)
	}
	nx := library.IDFor(filepath.Join(moved, "a.jpg"))
	ids, scales := placed(p)
	if want := []string{nx, model.CopyID(nx, 2)}; !slices.Equal(ids, want) || !slices.Equal(scales, []float64{2, 3}) {
		t.Fatalf("места %v кадры %v, ждали %v и [2 3]", ids, scales, want)
	}
	if len(p.Photos) != 2 || p.Photos[nx].Path != p.Photos[model.CopyID(nx, 2)].Path {
		t.Fatalf("записи: %+v", p.Photos)
	}
	// Прямой вызов: номер берётся из старого ID, ID файла — из пути.
	q := model.New()
	q.Photos["old-3"] = model.PhotoRef{Path: filepath.Join(moved, "b.jpg"), W: 1, H: 1}
	q.Spreads = []*model.Spread{{ID: "s", Photos: []string{"old-3"}}}
	Rekey(q, library.IDFor)
	if id := model.CopyID(library.IDFor(filepath.Join(moved, "b.jpg")), 3); q.Spreads[0].Photos[0] != id {
		t.Fatalf("Rekey: %v, ждали %s", q.Spreads[0].Photos, id)
	}
}

// Столкновение как в 0.22: две записи одного пути с одним номером (обе без номера или обе «-2»)
// сливаются, первая в порядке ленты побеждает, второе место пустеет; копия при этом не сливается
// с исходной.
func TestRekeyCollisionAsBefore(t *testing.T) {
	dir := testkit.TempDir(t)
	abs := filepath.Join(dir, "a.jpg")
	data := copyFile(t, map[string]FilePhoto{
		"u": fileRef("a.jpg", abs, 2), "v": fileRef("a.jpg", abs, 3),
		"u-2": fileRef("a.jpg", abs, 4), "v-2": fileRef("a.jpg", abs, 5),
	}, [][]string{{"u", "v"}, {"u-2", "v-2"}})
	p, err := Decode(data, dir)
	if err != nil {
		t.Fatal(err)
	}
	a := library.IDFor(abs)
	ids, scales := placed(p)
	if want := []string{a, "", model.CopyID(a, 2), ""}; !slices.Equal(ids, want) {
		t.Fatalf("места %v, ждали %v", ids, want)
	}
	if !slices.Equal(scales, []float64{2, 0, 4, 0}) || len(p.Photos) != 2 {
		t.Fatalf("кадры %v, записей %d", scales, len(p.Photos))
	}
}

// Негодный номер копии (не число, «-1», «-100», с нулём впереди) — номер 0: запись сливается с
// исходной (как столкновение 0.22) или становится ею; крайний годный номер 99 остаётся копией.
func TestLoadCopyBadSuffix(t *testing.T) {
	dir := testkit.TempDir(t)
	abs := filepath.Join(dir, "a.jpg")
	a := library.IDFor(abs)
	photos := map[string]FilePhoto{a: fileRef("a.jpg", abs, 2)}
	sheet := []string{a}
	for _, bad := range []string{"-1", "-100", "-02", "-x", "-"} {
		photos[a+bad] = fileRef("a.jpg", abs, 3)
		sheet = append(sheet, a+bad)
	}
	photos[model.CopyID(a, model.MaxCopy)] = fileRef("a.jpg", abs, 4)
	sheet = append(sheet, model.CopyID(a, model.MaxCopy))
	z := filepath.Join(dir, "z.jpg")
	photos["зет-abc"] = fileRef("z.jpg", z, 0)
	p, err := Decode(copyFile(t, photos, [][]string{sheet, {"зет-abc"}}), dir)
	if err != nil {
		t.Fatal(err)
	}
	ids, _ := placed(p)
	want := []string{a, "", "", "", "", "", model.CopyID(a, model.MaxCopy), library.IDFor(z)}
	if !slices.Equal(ids, want) {
		t.Fatalf("места %v, ждали %v", ids, want)
	}
	if len(p.Photos) != 3 || p.Photos[a].Frame.S != 2 {
		t.Fatalf("записи: %+v", p.Photos)
	}
}

// Свойства файла в семье — по первой записи в порядке ленты (EvenFamilies): копия на первом листе
// с другими W, H, Size задаёт их и исходной; свойства использования (кадр) у каждой свои.
func TestLoadEvenFamilies(t *testing.T) {
	dir := testkit.TempDir(t)
	abs := filepath.Join(dir, "a.jpg")
	a := library.IDFor(abs)
	first := fileRef("a.jpg", abs, 3)
	first.W, first.H, first.Size = 2000, 3000, 2000
	p, err := Decode(copyFile(t, map[string]FilePhoto{
		a: fileRef("a.jpg", abs, 2), model.CopyID(a, 2): first,
	}, [][]string{{model.CopyID(a, 2)}, {a}}), dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{a, model.CopyID(a, 2)} {
		if r := p.Photos[id]; r.W != 2000 || r.H != 3000 || r.Size != 2000 || r.Path != abs {
			t.Fatalf("%s: %+v, ждали свойства файла первой в ленте", id, r)
		}
	}
	if _, sc := placed(p); !slices.Equal(sc, []float64{3, 2}) {
		t.Fatalf("кадры %v — свойства использования не свои", sc)
	}
	// Одна запись в семье — без изменений; пустой проект — ничего.
	q := model.New()
	EvenFamilies(q)
	if len(q.Photos) != 0 {
		t.Fatal("пустой проект")
	}
}

// Свой кадр из файла зажимается: ≤ 0, вне 0,01…100 — снят; годный — шаг 1e-4; края — как есть.
// У записи только на фоне листа свой кадр снимает PruneRefs (как кадр).
func TestLoadAspClamp(t *testing.T) {
	dir := testkit.TempDir(t)
	cases := []struct{ in, want float64 }{{-1, 0}, {0.005, 0}, {150, 0}, {1.77777777, 1.7778}, {0.01, 0.01}, {100, 100}, {0, 0}}
	photos := map[string]FilePhoto{}
	var sheet []string
	for i, c := range cases {
		path := filepath.Join(dir, string(rune('a'+i))+".jpg")
		fp := fileRef(filepath.Base(path), path, 0)
		fp.Asp = c.in
		photos[library.IDFor(path)] = fp
		sheet = append(sheet, library.IDFor(path))
	}
	bg := filepath.Join(dir, "bg.jpg")
	fbg := fileRef("bg.jpg", bg, 2)
	fbg.Asp = 1.5
	photos[library.IDFor(bg)] = fbg
	data := copyFile(t, photos, [][]string{sheet})
	var f FileProject
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	f.Spreads = append(f.Spreads, &model.Spread{ID: "5e1f0a2b3c49", Photos: []string{}, BgPhoto: &model.Backdrop{Photo: library.IDFor(bg), W: 600, H: 300}})
	data, _ = json.Marshal(f)
	p, err := Decode(data, dir)
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range sheet {
		if got := p.Photos[id].Asp; got != cases[i].want {
			t.Errorf("asp %v: %v, ждали %v", cases[i].in, got, cases[i].want)
		}
	}
	if r, ok := p.Photos[library.IDFor(bg)]; !ok || r.Asp != 0 || r.Frame != nil {
		t.Fatalf("фото фона: есть %v, %+v — свой кадр и кадр сняты", ok, r)
	}
}

// albumBgFile — файл с фоном альбома raw (JSON) и фото листа a; extra — ещё записи photos.
func albumBgFile(t *testing.T, dir, raw string, extra map[string]FilePhoto, sheet bool) []byte {
	t.Helper()
	abs := filepath.Join(dir, "a.jpg")
	photos := map[string]FilePhoto{library.IDFor(abs): fileRef("a.jpg", abs, 0)}
	for id, fp := range extra {
		photos[id] = fp
	}
	data := copyFile(t, photos, [][]string{{library.IDFor(abs)}})
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if raw != "" {
		m["albumBg"] = json.RawMessage(raw)
	}
	if sheet {
		s := m["settings"].(map[string]any)
		s["sheet"] = true
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Фон альбома из файла зажимается (ClampAlbumBg на чтении): и узор, и фото — снят; фото без
// записи — снят; прозрачность 0…1; у фото — только наклон; у проекта-листа страниц нет; пусто — nil.
func TestLoadAlbumBgClamp(t *testing.T) {
	dir := testkit.TempDir(t)
	const key = "my:0f1e2d3c4b5a6978"
	lib := map[string]FilePhoto{model.LibPhotoID(key): {PhotoRef: model.PhotoRef{Lib: key, W: 3000, H: 2000}}}
	libID := model.LibPhotoID(key)
	pat := `{"kind":"dots","inks":["#123456"]}`
	cases := []struct {
		name, raw string
		sheet     bool
		check     func(b *model.AlbumBg) bool
	}{
		{"нет", "", false, func(b *model.AlbumBg) bool { return b == nil }},
		{"пустой", `{}`, false, func(b *model.AlbumBg) bool { return b == nil }},
		{"узор и фото", `{"pat":` + pat + `,"photo":"` + libID + `"}`, false, func(b *model.AlbumBg) bool { return b == nil }},
		{"фото без записи", `{"photo":"0123456789abcdef"}`, false, func(b *model.AlbumBg) bool { return b == nil }},
		{"прозрачность", `{"pat":` + pat + `,"fade":1.5,"pages":true}`, false, func(b *model.AlbumBg) bool {
			return b != nil && b.Fade == 1 && b.Pages && b.Pat != nil && b.Pat.Kind == "dots"
		}},
		{"лист без страниц", `{"pat":` + pat + `,"fade":-1,"pages":true}`, true, func(b *model.AlbumBg) bool {
			return b != nil && b.Fade == 0 && !b.Pages
		}},
		{"наклон", `{"photo":"` + libID + `","frame":{"s":2,"r":95},"flipH":true,"fade":0.333}`, false, func(b *model.AlbumBg) bool {
			return b != nil && b.Photo == libID && b.FlipH && b.Fade == 0.33 && b.Frame != nil && b.Frame.R == 5 && b.Frame.S == 2
		}},
	}
	for _, c := range cases {
		p, err := Decode(albumBgFile(t, dir, c.raw, lib, c.sheet), dir)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !c.check(p.AlbumBg) {
			t.Errorf("%s: %+v", c.name, p.AlbumBg)
		}
		if _, ok := p.Photos[libID]; ok != (p.AlbumBg != nil && p.AlbumBg.Photo != "") {
			t.Errorf("%s: запись фото фона %v при фоне %+v", c.name, ok, p.AlbumBg)
		}
	}
}

// Фото, стоящее только в фоне альбома (своё из библиотеки и файл с диска), переживает чтение: до
// первого PruneRefs фон уже в проекте; ID приводятся (Rekey: своё — от ключа без номера, файл — от
// пути), ссылка фона переезжает за ним. Save → Load — то же.
func TestAlbumBgSurvivesLoad(t *testing.T) {
	dir := testkit.TempDir(t)
	const key = "my:0f1e2d3c4b5a6978"
	odd := "0123456789abcdef-2" // своё под чужим ID с номером: номер у своего отбрасывается
	data := albumBgFile(t, dir, `{"photo":"`+odd+`","fade":0.2}`,
		map[string]FilePhoto{odd: {PhotoRef: model.PhotoRef{Lib: key, W: 3000, H: 2000}}}, false)
	p, err := Decode(data, dir)
	if err != nil {
		t.Fatal(err)
	}
	libID := model.LibPhotoID(key)
	if p.AlbumBg == nil || p.AlbumBg.Photo != libID || p.Photos[libID].Lib != key {
		t.Fatalf("своё: фон %+v, записи %+v", p.AlbumBg, p.Photos)
	}
	// Файл с диска под старым ID.
	bg := filepath.Join(dir, "bg.jpg")
	data = albumBgFile(t, dir, `{"photo":"old","pages":true}`, map[string]FilePhoto{"old": fileRef("bg.jpg", bg, 0)}, false)
	q, err := Decode(data, dir)
	if err != nil {
		t.Fatal(err)
	}
	if id := library.IDFor(bg); q.AlbumBg == nil || q.AlbumBg.Photo != id || q.Photos[id].Path != bg || !q.AlbumBg.Pages {
		t.Fatalf("файл: фон %+v, записи %+v", q.AlbumBg, q.Photos)
	}
	path := filepath.Join(dir, "a.salbum")
	if err := Save(q, path); err != nil {
		t.Fatal(err)
	}
	r, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(model.Marshal(r)) != string(model.Marshal(q)) {
		t.Fatalf("Save → Load:\n%s\n%s", model.Marshal(q), model.Marshal(r))
	}
}
