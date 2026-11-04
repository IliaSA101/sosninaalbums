package fontlib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/text"
)

func TestHideKeepsFile(t *testing.T) {
	s, lib, src := newStore(t)
	a := put(t, src, "a.ttf", builtin(t, "Montserrat-Regular.ttf"))
	b := put(t, src, "b.ttf", builtin(t, "Montserrat-Bold.ttf"))
	l := put(t, src, "l.ttf", builtin(t, "Lora-Regular.ttf"))
	c := put(t, src, "c.ttf", builtin(t, "CormorantGaramond-Regular.ttf"))
	if _, err := s.Import([]string{a, b, l, c}); err != nil {
		t.Fatal(err)
	}
	mont := famOf(t, s, inspectName(t, builtin(t, "Montserrat-Regular.ttf")))
	lora := famOf(t, s, inspectName(t, builtin(t, "Lora-Regular.ttf")))

	// несколько гарнитур подряд; повторное скрытие — холостое
	for _, f := range []text.CustomFamily{mont, lora, lora} {
		if err := s.Hide(f.ID); err != nil {
			t.Fatalf("Hide: %v", err)
		}
	}
	got := s.Families()
	hidden := 0
	for _, f := range got {
		if f.Hidden {
			hidden++
		}
	}
	if len(got) != 3 || hidden != 2 {
		t.Fatalf("скрытых %d из %d: %+v", hidden, len(got), got)
	}
	// гарнитура остаётся в списке (зарегистрирована), файлы и чтение — на месте
	for _, f := range append(mont.Faces, lora.Faces...) {
		if _, err := os.Stat(filepath.Join(lib, f.Key+"."+f.Ext)); err != nil {
			t.Errorf("файл скрытого удалён: %v", err)
		}
		if _, err := s.Load(f.Key); err != nil {
			t.Errorf("Load скрытого: %v", err)
		}
	}
	if fs := s.Faces(mont.ID); len(fs) != 2 {
		t.Errorf("Faces скрытой: %+v", fs)
	}
	// круг: скрытие переживает перезапуск
	if !reflect.DeepEqual(Open(lib).Families(), got) {
		t.Error("после Open скрытие потеряно")
	}
	// края: чужой id, неверная грамматика, пустая строка
	for _, id := range []string{text.FamilyID("нет такого"), "my:123", "", "montserrat"} {
		if err := s.Hide(id); err != ErrNotFound {
			t.Errorf("Hide(%q): %v", id, err)
		}
	}
	// реестр не записался — скрытие откатывается
	if err := os.Remove(filepath.Join(lib, regFile)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(lib, regFile), 0o755); err != nil { // на месте реестра — папка: rename падает
		t.Fatal(err)
	}
	cor := famOf(t, s, inspectName(t, builtin(t, "CormorantGaramond-Regular.ttf")))
	if err := s.Hide(cor.ID); err == nil || strings.Contains(err.Error(), lib) {
		t.Errorf("Hide при сбое записи: %v", err)
	}
	if famOf(t, s, cor.Name).Hidden {
		t.Error("сбой записи не откатил скрытие")
	}
}

func TestOpenRebuildsRegistry(t *testing.T) {
	s, lib, src := newStore(t)
	a := put(t, src, "a.ttf", builtin(t, "Montserrat-Regular.ttf"))
	b := put(t, src, "b.ttf", builtin(t, "Montserrat-Bold.ttf"))
	l := put(t, src, "l.ttf", builtin(t, "Lora-Regular.ttf"))
	if _, err := s.Import([]string{a, b, l}); err != nil {
		t.Fatal(err)
	}
	want := s.Families()
	keys := func(fs []text.CustomFamily) map[string]bool {
		m := map[string]bool{}
		for _, f := range fs {
			for _, fc := range f.Faces {
				m[fc.Key] = true
			}
		}
		return m
	}

	// чужое в папке не трогается; не прошедшие проверку — тоже
	foreign := put(t, lib, "readme.txt", []byte("не шрифт"))
	badName := put(t, lib, "abc.ttf", builtin(t, "Lora-Bold.ttf"))      // имя не от ключа
	wrongSha := put(t, lib, "0123456789abcdef.ttf", []byte("не шрифт")) // ключ не совпал
	tmp1 := put(t, lib, "import-123.tmp", []byte("x"))
	tmp2 := put(t, lib, "fonts.json.456.tmp", []byte("x"))
	nested := put(t, filepath.Join(lib, "вложенная"), "x.ttf", builtin(t, "Lora-Bold.ttf"))

	for name, prepare := range map[string]func(){
		"нет реестра": func() { _ = os.Remove(filepath.Join(lib, regFile)) },
		"битый":       func() { _ = os.WriteFile(filepath.Join(lib, regFile), []byte("{не json"), 0o644) },
		"без версии":  func() { _ = os.WriteFile(filepath.Join(lib, regFile), []byte(`{"items":[]}`), 0o644) },
	} {
		prepare()
		again := Open(lib)
		if got := again.Families(); !reflect.DeepEqual(keys(got), keys(want)) || len(got) != len(want) || again.ReadOnly() {
			t.Fatalf("%s: пересборка дала %+v, ждали %+v", name, got, want)
		}
		// и записала реестр заново: следующее открытие читает его без пересборки
		if _, err := os.Stat(filepath.Join(lib, regFile)); err != nil {
			t.Errorf("%s: реестр не записан: %v", name, err)
		}
		if !reflect.DeepEqual(keys(Open(lib).Families()), keys(want)) {
			t.Errorf("%s: второе открытие отличается", name)
		}
	}
	if _, err := os.Stat(filepath.Join(lib, regFile+".bad")); err != nil {
		t.Errorf("битый реестр не отложен: %v", err)
	}
	for _, p := range []string{foreign, badName, wrongSha, nested} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("чужой файл тронут: %v", err)
		}
	}
	for _, p := range []string{tmp1, tmp2} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("остаток записи остался: %s", filepath.Base(p))
		}
	}

}

func TestOpenDropsBadItems(t *testing.T) {
	s, lib, src := newStore(t)
	a := put(t, src, "a.ttf", builtin(t, "Montserrat-Regular.ttf"))
	b := put(t, src, "b.ttf", builtin(t, "Montserrat-Bold.ttf"))
	l := put(t, src, "l.ttf", builtin(t, "Lora-Regular.ttf"))
	if _, err := s.Import([]string{a, b, l}); err != nil {
		t.Fatal(err)
	}
	want := s.Families()

	// запись реестра без файла выбрасывается; скрытое и признаки пережили круг
	if err := s.Hide(want[0].ID); err != nil {
		t.Fatal(err)
	}
	for _, fc := range want[1].Faces {
		if err := os.Remove(filepath.Join(lib, fc.Key+"."+fc.Ext)); err != nil {
			t.Fatal(err)
		}
	}
	got := Open(lib).Families()
	if len(got) != 1 || got[0].ID != want[0].ID || !got[0].Hidden {
		t.Errorf("запись без файла: %+v", got)
	}
	// реестр с мусорными записями: ключ, расширение, повтор
	raw, _ := os.ReadFile(filepath.Join(lib, regFile))
	var reg registry
	if err := json.Unmarshal(raw, &reg); err != nil || len(reg.Items) == 0 {
		t.Fatal(err)
	}
	good := *reg.Items[0]
	reg.Items = append(reg.Items, &entry{Key: "../../x", Ext: "ttf", Family: "Эксплойт"},
		&entry{Key: good.Key, Ext: "exe", Family: "Эксплойт"}, &good, nil, &entry{Key: "0000000000000000", Ext: "ttf"})
	raw, _ = json.Marshal(reg)
	if err := os.WriteFile(filepath.Join(lib, regFile), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if got2 := Open(lib).Families(); !reflect.DeepEqual(got2, got) {
		t.Errorf("мусор в реестре: %+v, ждали %+v", got2, got)
	}
}

func TestOpenRebuildKeepsNewerOfSameFace(t *testing.T) {
	s, lib, src := newStore(t)
	reg := builtin(t, "Montserrat-Regular.ttf")
	if _, err := s.Import([]string{put(t, src, "a.ttf", reg)}); err != nil {
		t.Fatal(err)
	}
	oldKey := s.Families()[0].Faces[0].Key
	if err := s.Hide(s.Families()[0].ID); err != nil {
		t.Fatal(err)
	}
	if res, _ := s.Import([]string{put(t, src, "b.ttf", withFsType(t, reg, 8))}); len(res.Faces) != 1 {
		t.Fatalf("замена: %+v", res)
	}
	newKey := s.Families()[0].Faces[0].Key
	// старый файл светлее по времени: пересборка должна оставить новый
	old := filepath.Join(lib, oldKey+".ttf")
	past := mustTime(t, -3600)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(lib, regFile)); err != nil {
		t.Fatal(err)
	}
	got := Open(lib).Families()
	if len(got) != 1 || len(got[0].Faces) != 1 || got[0].Faces[0].Key != newKey {
		t.Errorf("пересборка: %+v, ждали ключ %s", got, newKey)
	}
}

func TestOpenReadOnlyNewer(t *testing.T) {
	s, lib, src := newStore(t)
	if _, err := s.Import([]string{put(t, src, "a.ttf", builtin(t, "Montserrat-Regular.ttf"))}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(lib, regFile))
	var reg registry
	if err := json.Unmarshal(raw, &reg); err != nil {
		t.Fatal(err)
	}
	reg.Version = regVersion + 1
	newer, _ := json.Marshal(reg)
	if err := os.WriteFile(filepath.Join(lib, regFile), newer, 0o644); err != nil {
		t.Fatal(err)
	}
	ro := Open(lib)
	if !ro.ReadOnly() || len(ro.Families()) != 1 {
		t.Fatalf("новее: readOnly=%v %+v", ro.ReadOnly(), ro.Families())
	}
	other := put(t, src, "l.ttf", builtin(t, "Lora-Regular.ttf"))
	res, err := ro.Import([]string{other})
	if err != ErrReadOnly || len(res.Added) != 0 || res.Added == nil {
		t.Errorf("Import: %+v %v", res, err)
	}
	if err := ro.Hide(ro.Families()[0].ID); err != ErrReadOnly {
		t.Errorf("Hide: %v", err)
	}
	if _, err := ro.Drop([]string{ro.Families()[0].Faces[0].Key}); err != ErrReadOnly {
		t.Errorf("Drop: %v", err)
	}
	if _, err := ro.ImportFolder(src, map[string]bool{text.FamilyID("Lora"): true}); err != ErrReadOnly {
		t.Errorf("ImportFolder: %v", err)
	}
	// чтение работает, реестр не тронут
	if _, err := ro.Load(ro.Families()[0].Faces[0].Key); err != nil {
		t.Errorf("Load: %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(lib, regFile))
	if !bytes.Equal(after, newer) {
		t.Error("реестр новой версии перезаписан")
	}
	// реестр не читается (папка вместо файла) — тоже только чтение, пересборка в памяти
	if err := os.Remove(filepath.Join(lib, regFile)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(lib, regFile), 0o755); err != nil {
		t.Fatal(err)
	}
	un := Open(lib)
	if !un.ReadOnly() || len(un.Families()) != 1 {
		t.Errorf("нечитаемый реестр: readOnly=%v %+v", un.ReadOnly(), un.Families())
	}
}

func TestLoadShaMismatch(t *testing.T) {
	s, lib, src := newStore(t)
	if _, err := s.Import([]string{put(t, src, "a.ttf", builtin(t, "Montserrat-Regular.ttf"))}); err != nil {
		t.Fatal(err)
	}
	key := s.Families()[0].Faces[0].Key
	path := filepath.Join(lib, key+".ttf")
	b, err := s.Load(key)
	if err != nil || keyOf(b) != key {
		t.Fatalf("Load до подмены: %v", err)
	}
	// подмена файла на диске (даже другим настоящим шрифтом) — отказ
	if err := os.WriteFile(path, builtin(t, "Lora-Regular.ttf"), 0o644); err != nil {
		t.Fatal(err)
	}
	if b, err := s.Load(key); err != ErrSha || b != nil {
		t.Errorf("подмена: %v", err)
	}
	// пустой файл, файл стёрт, больше предела
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(key); err != ErrSha {
		t.Errorf("пустой: %v", err)
	}
	if err := os.WriteFile(path, make([]byte, MaxFontBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(key); err != text.ErrFontTooBig {
		t.Errorf("больше предела: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(key); err == nil || strings.Contains(err.Error(), lib) {
		t.Errorf("стёрт: %v", err)
	}
	// чужие ключи: неверная грамматика, путь, незарегистрированный
	for _, k := range []string{"", "../x", "ABCDEF0123456789", "0123456789abcdef", key + ".ttf"} {
		if _, err := s.Load(k); err != ErrNotFound {
			t.Errorf("Load(%q): %v", k, err)
		}
	}
}

func TestImportFolderOnlyWanted(t *testing.T) {
	s, lib, _ := newStore(t)
	root := filepath.Dir(lib)
	album := filepath.Join(root, "Альбом", "Шрифты")
	put(t, album, "m-reg.ttf", builtin(t, "Montserrat-Regular.ttf"))
	put(t, album, "M-BOLD.TTF", builtin(t, "Montserrat-Bold.ttf")) // регистр расширения не важен
	put(t, album, "lora.ttf", builtin(t, "Lora-Regular.ttf"))      // не нужен
	put(t, album, "notes.txt", []byte("заметка"))
	put(t, filepath.Join(album, "вложенная"), "cor.ttf", builtin(t, "CormorantGaramond-Regular.ttf"))
	mont := text.FamilyID(inspectName(t, builtin(t, "Montserrat-Regular.ttf")))
	cor := text.FamilyID(inspectName(t, builtin(t, "CormorantGaramond-Regular.ttf")))

	// пустой want — ничего не читается
	if res, err := s.ImportFolder(album, nil); err != nil || len(res.Added)+len(res.Faces)+len(res.Skipped) != 0 || len(s.Families()) != 0 {
		t.Fatalf("пустой want: %+v %v", res, err)
	}
	// пустая папка и папка без шрифтов
	if res, err := s.ImportFolder(filepath.Join(root, "Альбом"), map[string]bool{mont: true}); err != nil || len(res.Added) != 0 {
		t.Errorf("папка без шрифтов: %+v %v", res, err)
	}

	res, err := s.ImportFolder(album, map[string]bool{mont: true, cor: true})
	if err != nil {
		t.Fatal(err)
	}
	// Montserrat — оба начертания; Lora не нужна; вложенную папку не смотрим
	if len(res.Added) != 1 || res.Added[0].ID != mont || len(res.Added[0].Faces) != 2 || len(res.Faces) != 2 || len(res.Skipped) != 0 {
		t.Fatalf("итог: %+v", res)
	}
	if fams := s.Families(); len(fams) != 1 || fams[0].ID != mont {
		t.Errorf("в библиотеке: %+v", fams)
	}
	if list, _ := os.ReadDir(lib); len(list) != 3 { // два шрифта и реестр: ни копии Lora, ни вложенной
		t.Errorf("файлов в библиотеке: %d", len(list))
	}
	// повтор: «уже есть» молча, ничего нового; нужная гарнитура есть → ничего не пишется
	res, err = s.ImportFolder(album, map[string]bool{mont: true})
	if err != nil || len(res.Added)+len(res.Faces)+len(res.Skipped) != 0 {
		t.Errorf("повтор: %+v %v", res, err)
	}
	// непригодный файл в папке — в пропусках с причиной (тост), остальное грузится
	put(t, album, "broken.otf", []byte("не шрифт, просто текст"))
	res, _ = s.ImportFolder(album, map[string]bool{text.FamilyID(inspectName(t, builtin(t, "Lora-Regular.ttf"))): true})
	if len(res.Added) != 1 || len(res.Skipped) != 1 || res.Skipped[0].Name != "broken.otf" {
		t.Errorf("с непригодным: %+v", res)
	}
	// свежие ключи папки откатываются так же, как загруженные
	removed, err := s.Drop(res.Faces)
	if err != nil || len(removed) != 1 {
		t.Errorf("Drop свежего из папки: %v %v", removed, err)
	}
}

func TestImportFolderLimit(t *testing.T) {
	s, lib, _ := newStore(t)
	album := filepath.Join(filepath.Dir(lib), "Альбом", "Шрифты")
	mont := text.FamilyID(inspectName(t, builtin(t, "Montserrat-Regular.ttf")))
	// первые файлы по имени — мусор: в счёт идут все *.ttf|*.otf, берутся первые MaxAlbumFiles
	for i := range MaxAlbumFiles + 6 {
		put(t, album, fmt.Sprintf("a%03d.ttf", i), []byte("не шрифт"))
	}
	put(t, album, "z.ttf", builtin(t, "Montserrat-Regular.ttf"))
	res, err := s.ImportFolder(album, map[string]bool{mont: true})
	if err != nil || len(res.Added) != 0 || len(res.Skipped) != MaxAlbumFiles {
		t.Fatalf("за пределом: добавлено %d, пропусков %d (ждали %d), %v", len(res.Added), len(res.Skipped), MaxAlbumFiles, err)
	}
	// ровно MaxAlbumFiles файлов — последний ещё читается
	for i := MaxAlbumFiles - 1; i < MaxAlbumFiles+6; i++ {
		if err := os.Remove(filepath.Join(album, fmt.Sprintf("a%03d.ttf", i))); err != nil {
			t.Fatal(err)
		}
	}
	res, err = s.ImportFolder(album, map[string]bool{mont: true})
	if err != nil || len(res.Added) != 1 || len(res.Skipped) != MaxAlbumFiles-1 {
		t.Errorf("ровно предел: добавлено %d, пропусков %d, %v", len(res.Added), len(res.Skipped), err)
	}
}

func TestDropOnlyFresh(t *testing.T) {
	s, lib, src := newStore(t)
	if _, err := s.Import([]string{put(t, src, "old.ttf", builtin(t, "Montserrat-Regular.ttf"))}); err != nil {
		t.Fatal(err)
	}
	oldKey := s.Families()[0].Faces[0].Key
	s = Open(lib) // новый сеанс: прежнее не свежее

	res, err := s.Import([]string{put(t, src, "l1.ttf", builtin(t, "Lora-Regular.ttf")),
		put(t, src, "l2.ttf", builtin(t, "Lora-Bold.ttf")), put(t, src, "c.ttf", builtin(t, "CormorantGaramond-Regular.ttf"))})
	if err != nil || len(res.Faces) != 3 {
		t.Fatalf("загрузка: %+v %v", res, err)
	}
	// «окно отказало» по Lora (двум начертаниям) и по старому; ключи в обеих формах и мусор
	bad := []string{res.Faces[0], strings.TrimSuffix(res.Faces[1], ".ttf"), oldKey + ".ttf", oldKey,
		"../" + oldKey + ".ttf", "", "xyz", "0123456789abcdef.ttf", res.Faces[2][:16] + ".otf"}
	removed, err := s.Drop(bad)
	if err != nil || removed == nil {
		t.Fatalf("Drop: %v %v", removed, err)
	}
	if !reflect.DeepEqual(removed, bad[:2]) {
		t.Errorf("удалены %v, ждали %v", removed, bad[:2])
	}
	// старое на месте (файл, запись, Load), свежая Cormorant не тронута
	if _, err := os.Stat(filepath.Join(lib, oldKey+".ttf")); err != nil {
		t.Errorf("старое начертание удалено: %v", err)
	}
	if fams := s.Families(); len(fams) != 2 {
		t.Errorf("гарнитуры после отката: %+v", fams)
	}
	for _, f := range res.Faces[:2] {
		if _, err := os.Stat(filepath.Join(lib, f)); err == nil {
			t.Errorf("файл отката остался: %s", f)
		}
	}
	// повтор и пустой ввод — холостые, nil-срез не возвращается
	for _, in := range [][]string{bad, nil, {}} {
		if r, err := s.Drop(in); err != nil || r == nil || len(r) != 0 && len(in) == 0 {
			t.Errorf("повтор: %v %v", r, err)
		}
	}
	// откат пережил перезапуск; в новом сеансе ничто не свежее
	again := Open(lib)
	if len(again.Families()) != 2 {
		t.Errorf("после Open: %+v", again.Families())
	}
	if r, _ := again.Drop([]string{res.Faces[2]}); len(r) != 0 {
		t.Errorf("Drop в новом сеансе удалил %v", r)
	}
	// возвращённый из скрытых файл не свежий: Drop его не удаляет
	if err := s.Hide(s.Families()[0].ID); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.Import([]string{filepath.Join(src, "c.ttf")}); len(r.Faces) != 0 {
		t.Fatalf("возврат записал файл: %+v", r)
	}
	fresh, _ := s.Drop([]string{oldKey})
	if len(fresh) != 0 {
		t.Errorf("Drop удалил несвежее: %v", fresh)
	}
}

// TestDropRestoresReplaced — новый файл того же начертания заменил скрытый, окно ему отказало:
// Drop возвращает заменённую запись (скрытой, как была), и это переживает перезапуск. Файла
// заменённого нет — возвращать нечего.
func TestDropRestoresReplaced(t *testing.T) {
	for _, gone := range []bool{false, true} {
		dropReplaced(t, gone)
	}
}

// dropReplaced — один случай TestDropRestoresReplaced; gone — файл заменённого удалён до Drop.
func dropReplaced(t *testing.T, gone bool) {
	t.Helper()
	s, lib, src := newStore(t)
	reg := builtin(t, "Montserrat-Regular.ttf")
	lora := put(t, src, "l.ttf", builtin(t, "Lora-Regular.ttf"))
	if _, err := s.Import([]string{put(t, src, "a.ttf", reg), lora}); err != nil {
		t.Fatal(err)
	}
	mont := famOf(t, s, inspectName(t, reg))
	oldKey := mont.Faces[0].Key
	if err := s.Hide(mont.ID); err != nil {
		t.Fatal(err)
	}
	res, err := s.Import([]string{put(t, src, "b.ttf", withFsType(t, reg, 8))})
	if err != nil || len(res.Faces) != 1 || famOf(t, s, mont.Name).Hidden || famOf(t, s, mont.Name).Faces[0].Key == oldKey {
		t.Fatalf("замена скрытого: %+v %v", res, err)
	}
	if gone {
		if err := os.Remove(filepath.Join(lib, oldKey+".ttf")); err != nil {
			t.Fatal(err)
		}
	}
	if r, err := s.Drop(res.Faces); err != nil || len(r) != 1 {
		t.Fatalf("Drop: %v %v", r, err)
	}
	for name, st := range map[string]*Store{"сеанс": s, "после Open": Open(lib)} {
		fams := st.Families()
		if gone {
			if len(fams) != 1 { // только Lora
				t.Errorf("%s, файла нет: %+v", name, fams)
			}
			continue
		}
		f := famOf(t, st, mont.Name)
		if len(fams) != 2 || !f.Hidden || len(f.Faces) != 1 || f.Faces[0].Key != oldKey {
			t.Errorf("%s: %+v", name, fams)
		}
	}
	// повторный Drop того же ключа — холостой, заменённая запись не задваивается
	if r, _ := s.Drop(res.Faces); len(r) != 0 || !gone && len(famOf(t, s, mont.Name).Faces) != 1 {
		t.Errorf("повтор: %v %+v", r, s.Families())
	}
}

func TestFacesRestricted(t *testing.T) {
	s, lib, src := newStore(t)
	reg := builtin(t, "Montserrat-Regular.ttf")
	res, err := s.Import([]string{
		put(t, src, "a.ttf", withFsType(t, reg, 0x0002)),        // Restricted License
		put(t, src, "b.ttf", builtin(t, "Montserrat-Bold.ttf")), // можно переносить
		put(t, src, "l.ttf", withFsType(t, builtin(t, "Lora-Regular.ttf"), 0x0008)),
	})
	if err != nil || len(res.Added) != 2 {
		t.Fatalf("загрузка: %+v %v", res, err)
	}
	mont := famOf(t, s, inspectName(t, reg))
	faces := s.Faces(mont.ID)
	if len(faces) != 2 || !faces[0].Restricted || faces[0].Bold || faces[1].Restricted || !faces[1].Bold {
		t.Fatalf("Montserrat: %+v", faces)
	}
	for _, f := range faces {
		if f.Path != filepath.Join(lib, f.Key+"."+f.Ext) || f.Ext != "ttf" {
			t.Errorf("путь начертания: %q", f.Path)
		}
	}
	lora := s.Faces(text.FamilyID(inspectName(t, builtin(t, "Lora-Regular.ttf"))))
	if len(lora) != 1 || lora[0].Restricted {
		t.Errorf("Lora (fsType 8 разрешает перенос): %+v", lora)
	}
	// признак Restricted переживает перезапуск, в том числе пересборку реестра
	if err := os.Remove(filepath.Join(lib, regFile)); err != nil {
		t.Fatal(err)
	}
	if f := Open(lib).Faces(mont.ID); len(f) != 2 || !f[0].Restricted || f[1].Restricted {
		t.Errorf("после пересборки: %+v", f)
	}
	// чужие id
	for _, id := range []string{"", "montserrat", "my:xyz", text.FamilyID("нет такой")} {
		if f := s.Faces(id); len(f) != 0 {
			t.Errorf("Faces(%q): %+v", id, f)
		}
	}
}
