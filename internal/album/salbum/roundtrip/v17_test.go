package roundtrip_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Формат v17 (098): копии фото, свой кадр, фон альбома, версия программы в файле.

var writeGolden17Copies = flag.Bool("golden17copies", false, "перезаписать testdata/v17-copies.salbum.json из copiesGolden")

// verTag — строка версии в файле: `"version": N,`.
func verTag(n int) []byte { return fmt.Appendf(nil, `"version": %d,`, n) }

// asCurrent — файл с номером версии n, поднятый до текущей (голдены остаются на своей версии).
func asCurrent(data []byte, n int) []byte {
	return bytes.Replace(data, verTag(n), verTag(model.FormatVersion), 1)
}

const (
	golden17Copies = "v17-copies.salbum.json"
	g17Key         = "my:0123456789abcdef" // своё фото из библиотеки — фон альбома
	g17Dir         = "{{DIR}}"
)

// copiesGolden — альбом под base: фото a.jpg стоит на обложке (копия -3) и на двух листах
// (исходная запись и копия -2) с разными кадрами и своим кадром (asp), b.jpg — рядом без копий;
// фон альбома — своё фото, на втором листе он скрыт. idOf — ID записи по пути файла: в голдене
// заглушки, не зависящие от папки и ОС, при чтении — library.IDFor.
func copiesGolden(base string, idOf func(path string) string) *model.Project {
	p := model.New()
	p.Folders = []string{filepath.Join(base, "Март")}
	a, b := idOf(filepath.Join(base, "Март", "a.jpg")), idOf(filepath.Join(base, "Март", "b.jpg"))
	file := func(name string) model.PhotoRef {
		return model.PhotoRef{Path: filepath.Join(base, "Март", name), W: 3000, H: 2000, Size: 1000}
	}
	use := func(r model.PhotoRef, s float64, asp float64) model.PhotoRef {
		r.Frame, r.Asp = &model.Frame{S: s, X: 0.25, Y: -0.5}, asp
		return r
	}
	a2, a3 := model.CopyID(a, 2), model.CopyID(a, 3)
	p.Photos[a] = use(file("a.jpg"), 2, 0)
	p.Photos[a2] = use(file("a.jpg"), 3, 1.5)
	p.Photos[a3] = use(file("a.jpg"), 1.25, 0.8)
	p.Photos[b] = file("b.jpg")
	bg := model.LibPhotoID(g17Key)
	p.Photos[bg] = model.PhotoRef{Lib: g17Key, W: 1200, H: 800}
	pl := func(id string, x float64, photo string) *model.Place {
		return &model.Place{ID: id, X: x, Y: 20, W: 80, H: 60, Photo: photo}
	}
	p.Spreads = []*model.Spread{
		{ID: "5e1f0a2b3c41", Photos: []string{a, b}, Grid: []*model.Place{pl("c0ffee000001", 20, a), pl("c0ffee000002", 120, b)}},
		{ID: "5e1f0a2b3c42", Photos: []string{a2}, NoAlbumBg: true, Grid: []*model.Place{pl("c0ffee000003", 20, a2)}},
	}
	preset, _ := model.CoverPresetOf(model.CoverDefault)
	spec := preset.Spec
	spec.Preset = preset.ID
	p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID, Photos: []string{a3}, Grid: []*model.Place{pl("c0ffee000004", 320, a3)}},
		Spec: model.ClampCoverSpec(spec)}
	p.AlbumBg = &model.AlbumBg{Photo: bg, Frame: &model.Frame{S: 1.5, X: 0.5}, FlipH: true, Fade: 0.4, Pages: true}
	return p
}

// fixedIDs — ID записей голдена: по имени файла, не по пути.
func fixedIDs(path string) string {
	return map[string]string{"a.jpg": "1a2b3c4d5e6f7a8b", "b.jpg": "2b3c4d5e6f7a8b9c"}[filepath.Base(path)]
}

// norm17 — файл без папки альбома и разделителей ОС: base → {{DIR}}, «\\» → «/».
func norm17(data []byte, base string) []byte {
	q, _ := json.Marshal(base)
	data = bytes.ReplaceAll(data, bytes.Trim(q, `"`), []byte(g17Dir))
	return bytes.ReplaceAll(data, []byte(`\\`), []byte("/"))
}

// denorm17 — обратно: {{DIR}} → папка base (через «/», Decode принимает оба разделителя).
func denorm17(data []byte, base string) []byte {
	q, _ := json.Marshal(filepath.ToSlash(base))
	return bytes.ReplaceAll(data, []byte(g17Dir), bytes.Trim(q, `"`))
}

// canonJSON — тот же JSON с ключами по порядку и одним отступом: ID записей photos зависят от
// папки, а порядок словаря — от ID.
func canonJSON(t *testing.T, data []byte) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// withAppVersion — SetAppVersion на время теста (не параллельного: версия общая на пакет).
func withAppVersion(t *testing.T, v string) {
	t.Helper()
	salbum.SetAppVersion(v)
	t.Cleanup(func() { salbum.SetAppVersion("") })
}

// albumSetup — папка с a.jpg, b.jpg и файлом фона «из библиотеки»; искатель ключа g17Key.
func albumSetup(t *testing.T) (base string) {
	t.Helper()
	base = testkit.TempDir(t)
	for _, n := range []string{"a.jpg", "b.jpg"} {
		edtest.PutFile(t, filepath.Join(base, "Март", n), 1000)
	}
	lib := filepath.Join(testkit.TempDir(t), "bg.jpg")
	edtest.PutFile(t, lib, 500)
	edtest.LibFinder(t, map[string]string{g17Key: lib})
	return base
}

// wantTail — копии, кадры и фон альбома после чтения проекта из папки base: то же, что записано
// в голдене, с ID по путям.
func wantTail(t *testing.T, p *model.Project, base, uid string) {
	t.Helper()
	want := copiesGolden(base, library.IDFor)
	want.UID = uid                   // отпечаток (146 T4-9): в голдене v17 его нет, после Save → Load — записанный
	for id, r := range want.Photos { // путь своего — от искателя
		if r.Lib != "" {
			r.Path = p.Photos[id].Path
			want.Photos[id] = r
		}
	}
	if a, b := model.Marshal(want), model.Marshal(p); !bytes.Equal(a, b) {
		t.Fatalf("после чтения:\n%s\nждали\n%s", b, a)
	}
}

// Голден v17: копии на обложке и двух листах, asp, albumBg, noAlbumBg, app. Encode = голден с
// текущим номером версии (формат дальше только добавил поля, 104); Decode → Encode = то же (ID
// записей — по путям, порядок словаря другой); Save → Load без потерь; альбом с фото переехал в другую папку — копии, кадры и фон целы.
func TestCopiesGoldenV17(t *testing.T) {
	withAppVersion(t, "0.23.0")
	base := albumSetup(t)
	path := filepath.Join("..", "testdata", golden17Copies)
	enc, err := salbum.Encode(copiesGolden(base, fixedIDs), base)
	if err != nil {
		t.Fatal(err)
	}
	enc = norm17(enc, base)
	if *writeGolden17Copies {
		old := bytes.Replace(enc, verTag(model.FormatVersion), verTag(17), 1) // голден остаётся на своей версии
		if err := os.WriteFile(path, append(old, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	golden, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	golden = bytes.TrimSuffix(bytes.ReplaceAll(golden, []byte("\r\n"), []byte("\n")), []byte("\n"))
	if bytes.Equal(asCurrent(golden, 17), golden) {
		t.Fatal("в голдене нет номера версии 17")
	}
	// Файл v17 без подрезки: голден = Encode, кроме номера версии.
	if !bytes.Equal(enc, asCurrent(golden, 17)) {
		t.Fatalf("Encode отличается от голдена:\n%s", enc)
	}
	for _, key := range []string{`"app": "0.23.0"`, `"asp": 1.5`, `"albumBg"`, `"noAlbumBg": true`, `-2"`, `-3"`} {
		if !bytes.Contains(golden, []byte(key)) {
			t.Fatalf("в голдене нет %s", key)
		}
	}
	if bytes.Contains(golden, []byte("bg.jpg")) || bytes.Contains(golden, []byte(base)) {
		t.Fatalf("в голдене путь библиотеки или папки альбома:\n%s", golden)
	}

	// Decode: номера копий остаются, ID пересчитаны по путям.
	file := denorm17(golden, base)
	p, err := salbum.Decode(file, base)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || edtest.From(p) != 17 {
		t.Fatalf("версия %d, from %d", p.Version, edtest.From(p))
	}
	wantTail(t, p, base, "")
	out, err := salbum.Encode(p, base)
	if err != nil {
		t.Fatal(err)
	}
	back := norm17(out, base)
	for _, n := range []string{"a.jpg", "b.jpg"} { // ID по пути → заглушки голдена
		back = bytes.ReplaceAll(back, []byte(library.IDFor(filepath.Join(base, "Март", n))), []byte(fixedIDs(n)))
	}
	if canonJSON(t, back) != canonJSON(t, asCurrent(golden, 17)) {
		t.Fatalf("Decode → Encode отличается от голдена:\n%s", back)
	}
	if err := edtest.GridInvariants(p); err != nil {
		t.Fatal(err)
	}

	// Save → Load: файл альбома рядом с папкой Март. Свой формат записи — текущий (копия для 0.23
	// и переход v17 → v18 — TestMigrateV17ToV18).
	p.SetFrom(model.FormatVersion)
	album := filepath.Join(base, "Свадьба.salbum")
	e := edtest.Open(t, p, album)
	if err := e.Save(album); err != nil {
		t.Fatal(err)
	}
	if edtest.Backup(e) {
		t.Fatal("файл текущего формата не требует копии")
	}
	q, err := salbum.Load(album)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(model.Marshal(p), model.Marshal(q)) || edtest.From(q) != model.FormatVersion {
		t.Fatalf("потери при Save → Load:\n%s\n%s", model.Marshal(p), model.Marshal(q))
	}
	if f := fileJSON(t, album); string(f.App) != "0.23.0" || f.AlbumBg == nil || f.Version != model.FormatVersion {
		t.Fatalf("в файле: версия %d, app %q, albumBg %+v", f.Version, f.App, f.AlbumBg)
	}

	// Переезд: папка альбома целиком в другое место, ID пересчитываются от новых путей.
	moved := filepath.Join(testkit.TempDir(t), "Перенос")
	if err := os.Rename(base, moved); err != nil {
		t.Fatal(err)
	}
	m, err := salbum.Load(filepath.Join(moved, "Свадьба.salbum"))
	if err != nil {
		t.Fatal(err)
	}
	wantTail(t, m, moved, p.UID)
	if err := edtest.SourcesInvariant(m); err != nil {
		t.Fatal(err)
	}
}

// v16Bytes — файл v16 с одним листом и фото: Encode текущей версии с номером 16 и без app.
func v16Bytes(t *testing.T, base string) []byte {
	t.Helper()
	p := model.New()
	a := filepath.Join(base, "Март", "a.jpg")
	id := library.IDFor(a)
	p.Folders = []string{filepath.Join(base, "Март")}
	p.Photos[id] = model.PhotoRef{Path: a, W: 3000, H: 2000, Size: 1000, Frame: &model.Frame{S: 2}}
	p.Spreads = []*model.Spread{{ID: "5e1f0a2b3c41", Photos: []string{id},
		Grid: []*model.Place{{ID: "c0ffee000001", X: 20, Y: 20, W: 80, H: 60, Photo: id}}}}
	salbum.SetAppVersion("")
	data, err := salbum.Encode(p, base)
	if err != nil {
		t.Fatal(err)
	}
	out := bytes.Replace(data, verTag(model.FormatVersion), verTag(16), 1)
	if bytes.Equal(out, data) || bytes.Contains(data, []byte(`"app"`)) {
		t.Fatalf("v16 из Encode не вышел:\n%s", data)
	}
	return out
}

// v16 на диске: Load → Save делает копию «<имя> (для 0.22).salbum» с байтами исходника, пишет
// текущий формат с app; имя копии TakeBackup отдаёт один раз; второй Save копию не делает.
func TestMigrateV16ToV17(t *testing.T) {
	base := albumSetup(t)
	src := v16Bytes(t, base)
	withAppVersion(t, "0.23.0")
	path := filepath.Join(base, "Свадьба.salbum")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || edtest.From(p) != 16 {
		t.Fatalf("версия %d, from %d", p.Version, edtest.From(p))
	}
	e := edtest.Open(t, p, path)
	if !edtest.Backup(e) || e.TakeBackup() != "" {
		t.Fatal("до записи копия нужна, но не сделана")
	}
	before := model.Marshal(p)
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	bakPath := filepath.Join(base, "Свадьба (для 0.22).salbum")
	if bakPath != salbum.BackupName(path, 16, 1) {
		t.Fatalf("имя копии %s", salbum.BackupName(path, 16, 1))
	}
	if bak, err := os.ReadFile(bakPath); err != nil || !bytes.Equal(bak, src) {
		t.Fatalf("копия для 0.22: %v, байты исходника: %v", err, bytes.Equal(bak, src))
	}
	if got := e.TakeBackup(); got != bakPath {
		t.Fatalf("TakeBackup %q, ждали %q", got, bakPath)
	}
	if got := e.TakeBackup(); got != "" {
		t.Fatalf("второй TakeBackup %q: имя отдаётся один раз", got)
	}
	if f := fileJSON(t, path); f.Version != model.FormatVersion || string(f.App) != "0.23.0" {
		t.Fatalf("в файле версия %d, app %q", f.Version, f.App)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, model.Marshal(q)) || edtest.From(q) != model.FormatVersion {
		t.Fatalf("потери при v16 → текущий:\n%s\n%s", before, model.Marshal(q))
	}
	// Второй Save: копию не делает ни заново, ни « 2», имени нет.
	if err := os.WriteFile(bakPath, []byte("метка"), 0o644); err != nil {
		t.Fatal(err)
	}
	edtest.NewSpread(e, -1, edtest.Photos("x"))
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(bakPath); string(got) != "метка" || e.TakeBackup() != "" {
		t.Fatal("второй Save тронул копию или назвал её")
	}
	if _, err := os.Stat(salbum.BackupName(path, 16, 2)); err == nil {
		t.Fatal("лишняя копия « 2»")
	}
	// Открыли уже записанный файл заново: текущий формат, копии нет.
	e2 := edtest.Open(t, q, path)
	if err := e2.Save(path); err != nil || e2.TakeBackup() != "" {
		t.Fatalf("файл текущего формата не требует копии: %v", err)
	}
}

// Черновик от прежней программы (v16): первая запись копирует его в «черновик (для 0.22).salbum»,
// следующие — нет.
func TestDraftV16ToV17(t *testing.T) {
	base := albumSetup(t)
	src := v16Bytes(t, base)
	withAppVersion(t, "0.23.0")
	draft := filepath.Join(testkit.TempDir(t), "черновик.salbum")
	if err := os.WriteFile(draft, src, 0o644); err != nil {
		t.Fatal(err)
	}
	dp, err := salbum.Load(draft)
	if err != nil {
		t.Fatal(err)
	}
	de := edtest.Open(t, dp, draft)
	if err := de.WriteDraft(draft); err != nil {
		t.Fatal(err)
	}
	dbak := filepath.Join(filepath.Dir(draft), "черновик (для 0.22).salbum")
	if got, err := os.ReadFile(dbak); err != nil || !bytes.Equal(got, src) || de.TakeBackup() != dbak {
		t.Fatalf("копия черновика: %v, байты исходника: %v", err, bytes.Equal(got, src))
	}
	if f := fileJSON(t, draft); f.Version != model.FormatVersion {
		t.Fatalf("черновик записан в версии %d", f.Version)
	}
	if err := de.WriteDraft(draft); err != nil || de.TakeBackup() != "" { // автосохранение дальше
		t.Fatalf("повторный черновик: %v", err)
	}
}

// Без копий, своего кадра и фона альбома файл прежний байт в байт: Encode с app = Encode без него
// + строка app; ключей asp, albumBg, noAlbumBg нет (omitempty); v16 Decode → Encode совпадает
// с исходником, кроме номера версии. Края: пустой альбом и несколько листов.
func TestNoFieldsSameBytes(t *testing.T) {
	base := albumSetup(t)
	src := v16Bytes(t, base)
	p, err := salbum.Decode(src, base)
	if err != nil {
		t.Fatal(err)
	}
	for name, proj := range map[string]*model.Project{"v16": p, "пустой": model.New(), "листы": twoSheets(base)} {
		salbum.SetAppVersion("")
		plain, err := salbum.Encode(proj, base)
		if err != nil {
			t.Fatal(err)
		}
		salbum.SetAppVersion("0.23.0")
		with, err := salbum.Encode(proj, base)
		salbum.SetAppVersion("")
		if err != nil {
			t.Fatal(err)
		}
		cur := fmt.Sprintf("{\n  \"version\": %d,\n", model.FormatVersion)
		want := strings.Replace(string(plain), cur, cur+"  \"app\": \"0.23.0\",\n", 1)
		if string(with) != want || want == string(plain) {
			t.Fatalf("%s: Encode с app отличается не только строкой app:\n%s", name, with)
		}
		for _, key := range []string{`"asp"`, `"albumBg"`, `"noAlbumBg"`, `-2"`, `"crop"`} {
			if bytes.Contains(plain, []byte(key)) {
				t.Fatalf("%s: в файле без новых полей есть %s", name, key)
			}
		}
		if name == "v16" {
			if want := asCurrent(src, 16); !bytes.Equal(plain, want) {
				t.Fatalf("v16 Decode → Encode отличается от исходника:\n%s\nждали\n%s", plain, want)
			}
		}
	}
}

// twoSheets — два листа без новых полей: у нескольких объектов лишних ключей тоже нет.
func twoSheets(base string) *model.Project {
	p := copiesGolden(base, library.IDFor)
	p.AlbumBg, p.Cover = nil, nil
	for id, r := range p.Photos {
		if model.CopyNo(id) != 0 || r.Lib != "" { // копии и своё — не в этом файле
			delete(p.Photos, id)
		}
	}
	for _, s := range p.Spreads { // копии убраны вместе с местами
		s.NoAlbumBg = false
		s.Grid = slicesKeep(s.Grid, p.Photos)
		s.Photos = nil
		for _, pl := range s.Grid {
			s.Photos = append(s.Photos, pl.Photo)
		}
	}
	for id, r := range p.Photos {
		r.Asp = 0
		p.Photos[id] = r
	}
	return p
}

// slicesKeep — места, чьё фото есть в photos.
func slicesKeep(grid []*model.Place, photos map[string]model.PhotoRef) []*model.Place {
	var out []*model.Place
	for _, pl := range grid {
		if _, ok := photos[pl.Photo]; ok {
			out = append(out, pl)
		}
	}
	return out
}

// Все имена копии «(для 0.22)», «... 2» ... «... 99» заняты чужими файлами: Save и WriteDraft отказывают
// ошибкой, исходник и чужие файлы целы, копия остаётся нужной; освободили имя - запись идёт,
// копия встаёт на него.
func TestBackupAllBusy(t *testing.T) {
	base := albumSetup(t)
	src := v16Bytes(t, base)
	withAppVersion(t, "0.23.0")
	path := filepath.Join(base, "Свадьба.salbum")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	for k := 1; k <= 99; k++ {
		if err := os.WriteFile(salbum.BackupName(path, 16, k), []byte("чужое"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	e := edtest.Open(t, p, path)
	if err := e.Save(path); err == nil || !strings.Contains(err.Error(), "копия") {
		t.Fatalf("Save при занятых именах: %v", err)
	}
	if err := e.WriteDraft(path); err == nil || !strings.Contains(err.Error(), "копия") {
		t.Fatalf("WriteDraft при занятых именах: %v", err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, src) || !edtest.Backup(e) || e.TakeBackup() != "" {
		t.Fatal("после отказа исходник тронут или копия считается сделанной")
	}
	free := salbum.BackupName(path, 16, 57)
	if err := os.Remove(free); err != nil {
		t.Fatal(err)
	}
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(free); !bytes.Equal(got, src) || e.TakeBackup() != free {
		t.Fatal("копия не встала на освободившееся имя")
	}
	if got, _ := os.ReadFile(salbum.BackupName(path, 16, 56)); string(got) != "чужое" {
		t.Fatal("чужой файл затёрт")
	}
}
