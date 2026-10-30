package fontlib

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// inspectName — имя гарнитуры, как его увидит библиотека.
func inspectName(t testing.TB, b []byte) string {
	t.Helper()
	info, err := text.Inspect(b)
	if err != nil {
		t.Fatal(err)
	}
	return info.Family
}

func TestImportAddsFamily(t *testing.T) {
	s, lib, src := newStore(t)
	mReg, mBold, mIt := builtin(t, "Montserrat-Regular.ttf"), builtin(t, "Montserrat-Bold.ttf"), builtin(t, "Montserrat-Italic.ttf")
	lora := builtin(t, "Lora-Regular.ttf")
	files := []string{put(t, src, "a.ttf", mReg), put(t, src, "b.ttf", mBold), put(t, src, "c.ttf", mIt), put(t, src, "l.ttf", lora)}

	res, err := s.Import(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Skipped) != 0 || len(res.Added) != 2 || len(res.Faces) != 4 {
		t.Fatalf("итог: %+v", res)
	}
	mont := famOf(t, s, inspectName(t, mReg))
	if len(mont.Faces) != 3 || mont.Hidden || mont.NoCyr || mont.ID != text.FamilyID(mont.Name) {
		t.Errorf("Montserrat: %+v", mont)
	}
	// порядок начертаний: обычный, жирный, курсив
	if f := mont.Faces; f[0].Bold || f[0].Italic || !f[1].Bold || !f[2].Italic {
		t.Errorf("порядок начертаний: %+v", f)
	}
	for _, f := range mont.Faces {
		if _, err := os.Stat(filepath.Join(lib, f.Key+"."+f.Ext)); err != nil {
			t.Errorf("нет файла начертания: %v", err)
		}
		if b, err := s.Load(f.Key); err != nil || keyOf(b) != f.Key {
			t.Errorf("Load %s: %v", f.Key, err)
		}
	}
	// список по имени без регистра
	names := []string{}
	for _, f := range s.Families() {
		names = append(names, strings.ToLower(f.Name))
	}
	if len(names) != 2 || names[0] > names[1] {
		t.Errorf("порядок гарнитур: %v", names)
	}

	// круг: реестр переживает перезапуск
	again := Open(lib)
	if !reflect.DeepEqual(again.Families(), s.Families()) {
		t.Errorf("после Open:\n%+v\nбыло:\n%+v", again.Families(), s.Families())
	}
	// гарнитура и пустые значения: без кириллицы — NoCyr
	lat := put(t, src, "lat.otf", fixture(t, "sa-test-latin.otf"))
	res, err = again.Import([]string{lat})
	if err != nil || len(res.Added) != 1 || !res.Added[0].NoCyr {
		t.Errorf("латиница: %+v %v", res, err)
	}
	// свежая библиотека и пустой ввод
	empty, _, _ := newStore(t)
	for _, in := range [][]string{nil, {}} {
		res, err := empty.Import(in)
		if err != nil || res.Added == nil || res.Faces == nil || res.Skipped == nil || len(empty.Families()) != 0 {
			t.Errorf("пустой ввод: %+v %v", res, err)
		}
	}
	if empty.Faces(text.FamilyID("x")) != nil {
		t.Error("Faces пустой библиотеки")
	}
}

func TestImportLimits(t *testing.T) {
	s, lib, src := newStore(t)
	ok := put(t, src, "ok.ttf", builtin(t, "Montserrat-Regular.ttf"))

	// файл на 1 байт больше предела; прочий мусор, веб-шрифт и коллекция — причина в тосте
	big := put(t, src, "big.ttf", make([]byte, MaxFontBytes+1))
	junk := put(t, src, "junk.ttf", []byte("не шрифт, просто текст"))
	woff := put(t, src, "w.woff2", append([]byte("wOF2"), make([]byte, 40)...))
	ttc := put(t, src, "c.ttc", append([]byte("ttcf"), make([]byte, 40)...))
	res, err := s.Import([]string{big, junk, woff, ttc, ok})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"big.ttf": text.ErrFontTooBig.Error(), "junk.ttf": text.ErrNotFont.Error(),
		"w.woff2": text.ErrFontWeb.Error(), "c.ttc": text.ErrFontCollection.Error(),
	}
	if len(res.Skipped) != 4 || len(res.Added) != 1 {
		t.Fatalf("итог: %+v", res)
	}
	for _, sk := range res.Skipped {
		if want[sk.Name] != sk.Why {
			t.Errorf("%s: %q, ждали %q", sk.Name, sk.Why, want[sk.Name])
		}
	}
	// ровно предел файла принимается на уровне чтения (дальше решает Inspect)
	if _, err := readFont(put(t, src, "edge.ttf", make([]byte, MaxFontBytes))); err == nil || err == text.ErrFontTooBig {
		t.Errorf("файл ровно в предел: %v", err)
	}

	// за раз — не больше MaxImport: сотый обработан, сто первый пропущен
	many := make([]string, MaxImport+1)
	for i := range many {
		many[i] = filepath.Join(src, fmt.Sprintf("нет-%03d.ttf", i)) // файлов нет: каждый — пропуск
	}
	res, err = s.Import(many)
	if err != nil || len(res.Skipped) != MaxImport+1 {
		t.Fatalf("за раз: %v %d", err, len(res.Skipped))
	}
	if first, last := res.Skipped[MaxImport-1].Why, res.Skipped[MaxImport].Why; strings.Contains(first, "за раз") || !strings.Contains(last, "не больше 100") {
		t.Errorf("сотый %q, сто первый %q", first, last)
	}

	// число файлов и их размер в библиотеке
	oldN, oldB := maxFonts, maxFontsBytes
	t.Cleanup(func() { maxFonts, maxFontsBytes = oldN, oldB })
	maxFonts = 2
	lora := put(t, src, "lora.ttf", builtin(t, "Lora-Regular.ttf"))
	bold := put(t, src, "bold.ttf", builtin(t, "Montserrat-Bold.ttf"))
	res, _ = s.Import([]string{lora, bold})
	if len(res.Added) != 1 || len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Why, "больше не помещается") {
		t.Errorf("MaxFonts: %+v", res)
	}
	maxFonts = MaxFonts
	maxFontsBytes = int64(len(builtin(t, "Montserrat-Regular.ttf")) + len(builtin(t, "Lora-Regular.ttf")))
	res, _ = s.Import([]string{bold})
	if len(res.Added) != 0 || len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Why, "заполнена") {
		t.Errorf("MaxFontsBytes: %+v", res)
	}
	// ничего лишнего в папке: только два файла и реестр
	list, _ := os.ReadDir(lib)
	if len(list) != 3 {
		t.Errorf("в папке %d файлов, ждали 3: %v", len(list), list)
	}
}

func TestImportDuplicate(t *testing.T) {
	s, lib, src := newStore(t)
	reg := builtin(t, "Montserrat-Regular.ttf")
	a := put(t, src, "a.ttf", reg)
	copyOfA := put(t, src, "a-copy.ttf", reg)

	// один и тот же файл (и его копия под другим именем) — «уже есть»; второй раз — тоже
	res, _ := s.Import([]string{a, copyOfA})
	if len(res.Added) != 1 || len(res.Faces) != 1 || len(res.Skipped) != 1 || res.Skipped[0].Why != errHave.Error() {
		t.Fatalf("в одном вызове: %+v", res)
	}
	res, _ = s.Import([]string{a})
	if len(res.Added) != 0 || len(res.Faces) != 0 || len(res.Skipped) != 1 || res.Skipped[0].Why != "уже есть в библиотеке" {
		t.Errorf("повтор: %+v", res)
	}

	// то же имя и начертание, другой файл, прежний виден — сначала уберите прежний
	other := put(t, src, "other.ttf", withFsType(t, reg, 8))
	res, _ = s.Import([]string{other})
	if len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Why, "сначала уберите прежний") ||
		!strings.Contains(res.Skipped[0].Why, "обычный") || len(s.Families()) != 1 || len(s.Families()[0].Faces) != 1 {
		t.Fatalf("другой файл: %+v", res)
	}
	oldKey := s.Families()[0].Faces[0].Key

	// прежний скрыт — новый заменяет, старый файл остаётся на диске
	if err := s.Hide(s.Families()[0].ID); err != nil {
		t.Fatal(err)
	}
	res, _ = s.Import([]string{other})
	f := s.Families()
	if len(res.Skipped) != 0 || len(res.Added) != 1 || len(f) != 1 || len(f[0].Faces) != 1 || f[0].Faces[0].Key == oldKey || f[0].Hidden {
		t.Fatalf("замена скрытого: %+v %+v", res, f)
	}
	if _, err := os.Stat(filepath.Join(lib, oldKey+".ttf")); err != nil {
		t.Errorf("старый файл удалён: %v", err)
	}
	if _, err := s.Load(oldKey); err != ErrNotFound {
		t.Errorf("Load вытесненного ключа: %v", err)
	}
	// после перезапуска жив ровно один файл начертания
	if f2 := Open(lib).Families(); !reflect.DeepEqual(f2, f) {
		t.Errorf("после Open: %+v, ждали %+v", f2, f)
	}
	// файл стёрт руками, запись осталась: загрузка того же исходника возвращает файл
	newKey := f[0].Faces[0].Key
	if err := os.Remove(filepath.Join(lib, newKey+".ttf")); err != nil {
		t.Fatal(err)
	}
	res, _ = s.Import([]string{other})
	if len(res.Faces) != 1 || len(res.Skipped) != 0 {
		t.Errorf("стёртый файл: %+v", res)
	}
	if b, err := s.Load(newKey); err != nil || len(b) == 0 {
		t.Errorf("Load после восстановления: %v", err)
	}
}

func TestImportHiddenReturns(t *testing.T) {
	s, lib, src := newStore(t)
	a := put(t, src, "a.ttf", builtin(t, "Montserrat-Regular.ttf"))
	b := put(t, src, "b.ttf", builtin(t, "Montserrat-Bold.ttf"))
	l := put(t, src, "l.ttf", builtin(t, "Lora-Regular.ttf"))
	if _, err := s.Import([]string{a, b, l}); err != nil {
		t.Fatal(err)
	}
	mont := famOf(t, s, inspectName(t, builtin(t, "Montserrat-Regular.ttf")))
	if err := s.Hide(mont.ID); err != nil {
		t.Fatal(err)
	}
	if !famOf(t, s, mont.Name).Hidden {
		t.Fatal("гарнитура не скрыта")
	}
	// загрузка одного файла возвращает гарнитуру целиком, новых файлов нет, «уже есть» нет
	res, err := s.Import([]string{a})
	if err != nil || len(res.Skipped) != 0 || len(res.Faces) != 0 || len(res.Added) != 1 {
		t.Fatalf("возврат: %+v %v", res, err)
	}
	back := res.Added[0]
	if back.ID != mont.ID || back.Hidden || len(back.Faces) != 2 {
		t.Errorf("вернулась не целиком: %+v", back)
	}
	// чужая гарнитура осталась на месте и видна
	if famOf(t, s, inspectName(t, builtin(t, "Lora-Regular.ttf"))).Hidden {
		t.Error("Lora скрылась")
	}
	// возврат пережил перезапуск
	if Open(lib).Families()[0].Hidden || Open(lib).Families()[1].Hidden {
		t.Error("после Open снова скрыта")
	}
	// новое начертание скрытой гарнитуры тоже возвращает её
	if err := s.Hide(mont.ID); err != nil {
		t.Fatal(err)
	}
	it := put(t, src, "it.ttf", builtin(t, "Montserrat-Italic.ttf"))
	res, _ = s.Import([]string{it})
	if len(res.Added) != 1 || res.Added[0].Hidden || len(res.Added[0].Faces) != 3 {
		t.Errorf("новое начертание скрытой: %+v", res)
	}
}

func TestImportErrorsNoPath(t *testing.T) {
	var logs bytes.Buffer
	restore := captureLog(t, &logs)
	defer restore()
	s, lib, src := newStore(t)
	root := filepath.Dir(filepath.Dir(lib))
	missing := filepath.Join(src, "тайный-альбом", "нет.ttf")
	dir := filepath.Join(src, "папка.ttf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	unc := `\\сервер\общая\шрифт.ttf`
	res, err := s.Import([]string{missing, dir, unc, "../../x.ttf", ""})
	if err != nil || len(res.Skipped) != 5 || len(res.Added) != 0 {
		t.Fatalf("итог: %+v %v", res, err)
	}
	for _, sk := range res.Skipped {
		if strings.Contains(sk.Why, root) || strings.Contains(sk.Why, "тайный-альбом") || strings.Contains(sk.Why, "сервер") || sk.Why == "" {
			t.Errorf("причина с путём или пустая: %+v", sk)
		}
		if strings.Contains(sk.Name, root) || strings.Contains(sk.Name, string(filepath.Separator)) {
			t.Errorf("имя с путём: %q", sk.Name)
		}
	}
	// родитель библиотеки — файл: папку не создать, ошибка без пути
	blocked := put(t, src, "занято", []byte("x"))
	s2 := Open(filepath.Join(blocked, "Шрифты"))
	ok := put(t, src, "ok.ttf", builtin(t, "Montserrat-Regular.ttf"))
	if _, err := s2.Import([]string{ok}); err == nil || strings.Contains(err.Error(), root) || strings.Contains(err.Error(), "занято") {
		t.Errorf("ошибка папки: %v", err)
	}
	// ImportFolder: недопустимая папка и несуществующая
	for _, d := range []string{unc, filepath.Join(src, "тайный-альбом")} {
		if _, err := s.ImportFolder(d, map[string]bool{"x": true}); err == nil || strings.Contains(err.Error(), root) || strings.Contains(err.Error(), "тайный-альбом") || strings.Contains(err.Error(), "сервер") {
			t.Errorf("ImportFolder(%q): %v", d, err)
		}
	}
	if l := logs.String(); strings.Contains(l, root) || strings.Contains(l, "тайный-альбом") {
		t.Errorf("в журнале путь: %s", l)
	}
}

// spyInspect подменяет полную проверку: считает вызовы и смотрит, свободен ли ioMu во время
// проверки (аудит 082: зависший Inspect под замком вешал открытие альбома).
func spyInspect(t *testing.T, s *Store) (calls *int, locked *bool) {
	t.Helper()
	calls, locked = new(int), new(bool)
	orig := inspectFont
	t.Cleanup(func() { inspectFont = orig })
	inspectFont = func(b []byte) (text.FaceInfo, error) {
		*calls++
		if s.ioMu.TryLock() {
			s.ioMu.Unlock()
		} else {
			*locked = true
		}
		return orig(b)
	}
	return calls, locked
}

func TestInspectOutsideLock(t *testing.T) {
	s, lib, src := newStore(t)
	calls, locked := spyInspect(t, s)
	res, err := s.Import([]string{put(t, src, "m.ttf", builtin(t, "Montserrat-Regular.ttf")), put(t, src, "l.ttf", builtin(t, "Lora-Regular.ttf"))})
	if err != nil || len(res.Added) != 2 {
		t.Fatalf("Import: %+v %v", res, err)
	}
	album := filepath.Join(filepath.Dir(lib), "Альбом", "Шрифты")
	put(t, album, "c.ttf", builtin(t, "CormorantGaramond-Regular.ttf"))
	cor := text.FamilyID(inspectName(t, builtin(t, "CormorantGaramond-Regular.ttf")))
	if res, err := s.ImportFolder(album, map[string]bool{cor: true}); err != nil || len(res.Added) != 1 {
		t.Fatalf("ImportFolder: %+v %v", res, err)
	}
	if *calls != 3 || *locked {
		t.Errorf("проверок %d (ждали 3), под замком: %v", *calls, *locked)
	}
}

// TestImportFolderPeek — аудит 082: файлы ненужных гарнитур отсеиваются по таблице name, полный
// Inspect идёт только для нужных и для неразобранных (у тех — причина в тосте).
func TestImportFolderPeek(t *testing.T) {
	s, lib, _ := newStore(t)
	calls, _ := spyInspect(t, s)
	album := filepath.Join(filepath.Dir(lib), "Альбом", "Шрифты")
	put(t, album, "a.ttf", builtin(t, "Lora-Regular.ttf"))
	put(t, album, "b.ttf", builtin(t, "CormorantGaramond-Regular.ttf"))
	put(t, album, "c.ttf", builtin(t, "Montserrat-Regular.ttf"))
	put(t, album, "d.ttf", builtin(t, "Montserrat-Bold.ttf"))
	put(t, album, "junk.otf", []byte("не шрифт, просто текст"))
	mont := text.FamilyID(inspectName(t, builtin(t, "Montserrat-Regular.ttf")))
	res, err := s.ImportFolder(album, map[string]bool{mont: true})
	if err != nil || len(res.Added) != 1 || len(res.Faces) != 2 || len(res.Skipped) != 1 || res.Skipped[0].Name != "junk.otf" {
		t.Fatalf("итог: %+v %v", res, err)
	}
	if *calls != 3 { // два начертания Montserrat и мусор
		t.Errorf("полных проверок %d, ждали 3", *calls)
	}
	// файл без имени в таблице name отбирается по имени файла, как в readFont
	*calls = 0
	noName := withName(t, builtin(t, "Lora-Bold.ttf"))
	put(t, album, "Безымянный.ttf", noName)
	res, err = s.ImportFolder(album, map[string]bool{text.FamilyID("Безымянный"): true})
	if err != nil || len(res.Added) != 1 || res.Added[0].Name != "Безымянный" || *calls != 2 {
		t.Errorf("без имени: %+v %v, проверок %d", res, err, *calls)
	}
}

// TestImportFolderBudget — аудит 082: из папки альбома читается не больше albumBytes; сверх —
// пропуск со строкой журнала без пути.
func TestImportFolderBudget(t *testing.T) {
	var logs bytes.Buffer
	restore := captureLog(t, &logs)
	defer restore()
	orig := albumBytes
	t.Cleanup(func() { albumBytes = orig })
	reg, bold, it := builtin(t, "Montserrat-Regular.ttf"), builtin(t, "Montserrat-Bold.ttf"), builtin(t, "Montserrat-Italic.ttf")
	mont := map[string]bool{text.FamilyID(inspectName(t, reg)): true}
	for _, c := range []struct {
		name   string
		budget int64
		faces  int
	}{
		{"0 — ничего", 0, 0},
		{"на файл меньше двух", int64(len(reg) + len(bold) - 1), 1},
		{"ровно два", int64(len(reg) + len(bold)), 2},
		{"все", int64(len(reg) + len(bold) + len(it)), 3},
	} {
		s, lib, _ := newStore(t)
		album := filepath.Join(filepath.Dir(lib), "тайный-альбом", "Шрифты")
		put(t, album, "a.ttf", reg)
		put(t, album, "b.ttf", bold)
		put(t, album, "c.ttf", it)
		logs.Reset()
		albumBytes = c.budget
		res, err := s.ImportFolder(album, mont)
		if err != nil || len(res.Faces) != c.faces || len(res.Skipped) != 0 {
			t.Errorf("%s: %+v %v", c.name, res, err)
		}
		l := logs.String()
		if (c.faces < 3) != strings.Contains(l, fmt.Sprintf("пропущено файлов: %d", 3-c.faces)) {
			t.Errorf("%s: журнал %q", c.name, l)
		}
		if strings.Contains(l, "тайный-альбом") || strings.Contains(l, ".ttf") {
			t.Errorf("%s: в журнале путь: %q", c.name, l)
		}
	}
}
