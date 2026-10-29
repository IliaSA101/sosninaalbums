package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/app/ownprint"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Файл .saprint: импорт, замена по вопросу, «Поделиться». Окно и запись — printprof_test.go.

// profileFile — файл name.saprint с данными в своей папке теста; путь.
func profileFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(testkit.TempDir(t), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// encoded — профиль в байты файла.
func encoded(t *testing.T, p printprof.Profile) []byte {
	t.Helper()
	data, err := printprof.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// builtinCopy — встроенный профиль id под названием name (id и изделия прежние).
func builtinCopy(t *testing.T, id, name string) printprof.Profile {
	t.Helper()
	p, ok := printprof.Builtin().ByID(id)
	if !ok {
		t.Fatalf("встроенной %s нет", id)
	}
	p.Name, p.Gen = name, name
	p.Products = slices.Clone(p.Products) // снимок реестра общий и только для чтения
	return p
}

// ownFile — профиль своей типографии id (вид own-<8 hex>) на основе Фабрики: как после «Сохранить».
func ownFile(t *testing.T, a *App, id, name string, version int) printprof.Profile {
	t.Helper()
	b := buildOwn(editOf(t, a, "fabrika", name), id, printprof.Builtin(), 2)
	if len(b.errs) != 0 {
		t.Fatalf("сборка %s: %v", id, b.errs)
	}
	b.p.Version = version
	return b.p
}

func pickOpen(t *testing.T, path string) {
	t.Helper()
	old := openFileDialog
	t.Cleanup(func() { openFileDialog = old })
	openFileDialog = func(context.Context, wr.OpenDialogOptions) (string, error) { return path, nil }
}

func pickSave(t *testing.T, path string) *wr.SaveDialogOptions {
	t.Helper()
	old := saveFileDialog
	t.Cleanup(func() { saveFileDialog = old })
	var got wr.SaveDialogOptions
	saveFileDialog = func(_ context.Context, o wr.SaveDialogOptions) (string, error) { got = o; return path, nil }
	return &got
}

// Файл с id встроенной сохраняется копией под новым own-…: встроенная не тронута, изделия
// и «по умолчанию» переименованы вместе.
func TestImportBuiltinIdCopies(t *testing.T) {
	a, ev := printApp(t)
	orig, _ := printprof.Builtin().ByID("fabrika")
	path := profileFile(t, "f.saprint", encoded(t, builtinCopy(t, "fabrika", "Фабрика у друга")))
	pickOpen(t, path)
	res, err := a.ImportPrintProfile()
	if err != nil || res.Ask != "" || !res.Copied || res.Canceled {
		t.Fatalf("импорт: %+v %v", res, err)
	}
	id := res.Info.ID
	if !printprof.ValidOwnID(id) || res.Info.Source != printprof.SourceOwn || res.Info.Name != "Фабрика у друга" || id == "fabrika" {
		t.Fatalf("сводка: %+v", res.Info)
	}
	p, ok := printprof.Current().ByID(id)
	if !ok || p.Default != printprof.OwnProductID(id, 1) || p.Products[2].ID != printprof.OwnProductID(id, 3) {
		t.Fatalf("копия: %+v", p)
	}
	if now, _ := printprof.Current().ByID("fabrika"); now.Name != orig.Name || printprof.Current().Source("fabrika") != printprof.SourceBuiltin {
		t.Errorf("встроенная изменилась: %+v", now)
	}
	if len(res.Lines) != 3 || !strings.HasPrefix(res.Lines[0], "Форматы: 20×20 — 4724 × 2398 px") || res.Lines[2] != "Сайт: fabrika-fotoknigi.com" ||
		res.Lines[1] != "Файлы: 1-00.jpg — обложка, 1-01.jpg…1-12.jpg" {
		t.Errorf("сводка: %q", res.Lines)
	}
	if ev.count("print:changed") != 1 || ownCount(a) != 1 {
		t.Errorf("событий %d, своих %d", ev.count("print:changed"), ownCount(a))
	}
	// Тот же файл ещё раз: название уже занято копией.
	if _, err := a.ownp.ImportPath(path); err == nil || err.Error() != "Такое название уже есть" || ownCount(a) != 1 {
		t.Errorf("повтор: %v, своих %d", err, ownCount(a))
	}
	// Название встроенной в файле с id встроенной — тоже отказ.
	if _, err := a.ownp.ImportPath(profileFile(t, "g.saprint", encoded(t, builtinCopy(t, "fsfera", "Фотосфера")))); err == nil {
		t.Error("название встроенной принято")
	}
	// Любой чужой id (не own-…) — копия.
	other := builtinCopy(t, "fabrika", "Другой id")
	other.ID, other.Default = "mytypo", "mytypo-1"
	for i := range other.Products {
		other.Products[i].ID = "mytypo-" + strings.TrimPrefix(other.Products[i].ID, "fabrika-")
	}
	other.Default = other.Products[0].ID
	r2, err := a.ownp.ImportPath(profileFile(t, "h.saprint", encoded(t, other)))
	if err != nil || !r2.Copied || !printprof.ValidOwnID(r2.Info.ID) {
		t.Errorf("чужой id: %+v %v", r2, err)
	}
}

// Свой id, которого нет, сохраняется как есть (файл от того же человека потом заменит её);
// изделия не вида <id>-<n> переименовываются.
func TestImportOwnKeepsID(t *testing.T) {
	a, _ := printApp(t)
	p := ownFile(t, a, "own-1a2b3c4d", "От подруги", 1)
	res, err := a.ownp.ImportPath(profileFile(t, "o.saprint", encoded(t, p)))
	if err != nil || res.Copied || res.Info.ID != "own-1a2b3c4d" {
		t.Fatalf("импорт: %+v %v", res, err)
	}
	// Изделия без числа в конце — реестр свою не принял бы: копия под новым id.
	bad := ownFile(t, a, "own-2b3c4d5e", "С чужими изделиями", 1)
	for i := range bad.Products {
		bad.Products[i].ID = "own-2b3c4d5e-x" + string(rune('a'+i))
	}
	bad.Default = bad.Products[0].ID
	r2, err := a.ownp.ImportPath(profileFile(t, "b.saprint", encoded(t, bad)))
	if err != nil || !r2.Copied || r2.Info.ID == "own-2b3c4d5e" {
		t.Fatalf("чужие изделия: %+v %v", r2, err)
	}
	if got, ok := printprof.Current().ByID(r2.Info.ID); !ok || got.Default != printprof.OwnProductID(r2.Info.ID, 1) {
		t.Errorf("изделия не переименованы: %+v", got)
	}
}

// Совпал свой id — вопрос; «Заменить» меняет содержимое под тем же ключом; ответ один раз.
func TestImportReplaceOwn(t *testing.T) {
	a, ev := printApp(t)
	v1 := ownFile(t, a, "own-aaaa0001", "Общая", 1)
	if _, err := a.ownp.ImportPath(profileFile(t, "1.saprint", encoded(t, v1))); err != nil {
		t.Fatal(err)
	}
	keys := func() []string {
		var out []string
		for _, f := range a.mustStore(t).Prints() {
			out = append(out, f.Key)
		}
		return out
	}
	before := keys()
	v2 := v1
	v2.Version, v2.Page.Bleed = 2, 5
	path := profileFile(t, "2.saprint", encoded(t, v2))
	res, err := a.ownp.ImportPath(path)
	want := "Заменить типографию «Общая» (версия 1) типографией из файла (версия 2)?"
	if err != nil || res.Ask != "replace" || res.Question != want || res.Token == "" {
		t.Fatalf("вопрос: %+v %v", res, err)
	}
	if cur, _ := printprof.Current().ByID("own-aaaa0001"); cur.Version != 1 || ev.count("print:changed") != 1 {
		t.Fatal("до ответа типография уже заменена")
	}
	// Чужой токен и пустой — устарело; вопрос от этого не пропал.
	for _, tok := range []string{"", "неверный"} {
		if _, err := a.ConfirmImportPrintProfile(tok); err != ownprint.ErrStale {
			t.Errorf("токен %q: %v", tok, err)
		}
	}
	res, err = a.ownp.ImportPath(path) // повторный вопрос вытеснил прежний токен
	if err != nil || res.Ask != "replace" {
		t.Fatal(err)
	}
	ok, err := a.ConfirmImportPrintProfile(res.Token)
	if err != nil || ok.Info.Version != 2 || ok.Info.ID != "own-aaaa0001" {
		t.Fatalf("замена: %+v %v", ok, err)
	}
	if cur, _ := printprof.Current().ByID("own-aaaa0001"); cur.Version != 2 || cur.Page.Bleed != 5 || ownCount(a) != 1 {
		t.Errorf("после замены: %+v", cur)
	}
	if after := keys(); len(after) != 1 || after[0] != before[0] {
		t.Errorf("ключ файла сменился: %v → %v", before, after)
	}
	if ev.count("print:changed") != 2 {
		t.Errorf("событий %d", ev.count("print:changed"))
	}
	if _, err := a.ConfirmImportPrintProfile(res.Token); err != ownprint.ErrStale {
		t.Errorf("второй ответ: %v", err)
	}
	// Те же данные — «такая уже есть», токена нет.
	same, err := a.ownp.ImportPath(path)
	if err != nil || same.Ask != "same" || same.Token != "" {
		t.Errorf("то же: %+v %v", same, err)
	}
}

// Вопрос устаревает: по времени, при удалении типографии и при названии, занятом за это время.
func TestImportReplaceStale(t *testing.T) {
	a, _ := printApp(t)
	v1 := ownFile(t, a, "own-bbbb0001", "Исходная", 1)
	if _, err := a.ownp.ImportPath(profileFile(t, "1.saprint", encoded(t, v1))); err != nil {
		t.Fatal(err)
	}
	v2 := v1
	v2.Version = 2
	path := profileFile(t, "2.saprint", encoded(t, v2))
	ask := func() string {
		t.Helper()
		r, err := a.ownp.ImportPath(path)
		if err != nil || r.Ask != "replace" {
			t.Fatalf("вопрос: %+v %v", r, err)
		}
		return r.Token
	}
	tok := ask()
	a.ownp.ExpirePending()
	if _, err := a.ConfirmImportPrintProfile(tok); err != ownprint.ErrStale {
		t.Errorf("просрочен: %v", err)
	}
	tok = ask()
	if _, err := a.DeletePrintProfile("own-bbbb0001"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ConfirmImportPrintProfile(tok); err != ownprint.ErrStale || ownCount(a) != 0 {
		t.Errorf("удалена: %v, своих %d", err, ownCount(a))
	}
	// Название файла занято другой своей — отказ уже на вопросе.
	if _, err := a.ownp.ImportPath(profileFile(t, "1.saprint", encoded(t, v1))); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ownp.ImportPath(profileFile(t, "x.saprint", encoded(t, ownFile(t, a, "own-bbbb0002", "Занятое", 1)))); err != nil {
		t.Fatal(err)
	}
	clash := v2
	clash.Name, clash.Gen = "Занятое", "Занятое"
	if _, err := a.ownp.ImportPath(profileFile(t, "c.saprint", encoded(t, clash))); err == nil || err.Error() != "Такое название уже есть" {
		t.Errorf("название другой своей: %v", err)
	}
}

// Плохие файлы: ошибка со строкой без пути, реестр и хранилище не меняются, паники нет.
func TestImportBadFiles(t *testing.T) {
	a, ev := printApp(t)
	good := encoded(t, ownFile(t, a, "own-cccc0001", "Годная", 1))
	dir := testkit.TempDir(t)
	big := []byte(strings.Repeat(" ", printprof.MaxSize+1))
	cases := []struct {
		name string
		path string
		want string
	}{
		{"пустой", profileFile(t, "e.saprint", nil), "Файл типографии не читается: профиль: пустой файл"},
		{"усечённый", profileFile(t, "t.saprint", good[:len(good)/2]), "Файл типографии не читается"},
		{"один байт", profileFile(t, "o.saprint", []byte("{")), "Файл типографии не читается"},
		{"мусор", profileFile(t, "m.saprint", []byte{0xff, 0xfe, 0x00, 0x01}), "Файл типографии не читается"},
		{"лишнее после значения", profileFile(t, "x.saprint", append(append([]byte{}, good...), []byte(`{"a":1}`)...)), "Файл типографии не читается"},
		{"неизвестное поле", profileFile(t, "u.saprint", []byte(strings.Replace(string(good), `"format": 1,`, `"format": 1, "zzz": 1,`, 1))), "Файл типографии не читается"},
		{"format 2", profileFile(t, "f.saprint", []byte(strings.Replace(string(good), `"format": 1`, `"format": 2`, 1))), "Файл типографии не читается"},
		{"на байт больше предела", profileFile(t, "b.saprint", big), "файл типографии больше 64 КБ"},
		{"огромный", profileFile(t, "h.saprint", make([]byte, 8<<20)), "файл типографии больше 64 КБ"},
		{"не то расширение", profileFile(t, "p.json", good), "это не файл типографии"},
		{"без файла", filepath.Join(dir, "нет.saprint"), "файл типографии не найден"},
		{"папка", mkdirNamed(t, dir, "d.saprint"), "файл типографии не найден"},
		{"относительный путь", "p.saprint", "не на локальном диске"},
		{"пустой путь", "", "не на локальном диске"},
	}
	for _, c := range cases {
		_, err := a.ownp.ImportPath(c.path)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, ждали %q", c.name, err, c.want)
			continue
		}
		if strings.Contains(err.Error(), dir) {
			t.Errorf("%s: в ошибке путь: %v", c.name, err)
		}
	}
	if ownCount(a) != 0 || ev.count("print:changed") != 0 || len(a.mustStore(t).Prints()) != 0 {
		t.Errorf("плохие файлы дошли до реестра: своих %d", ownCount(a))
	}
	// Ровно предел и годный профиль, дополненный пробелами до 64 КБ, читается.
	padded := append(append([]byte{}, good...), []byte(strings.Repeat(" ", printprof.MaxSize-len(good)))...)
	if r, err := a.ownp.ImportPath(profileFile(t, "pad.saprint", padded)); err != nil || r.Info.ID != "own-cccc0001" {
		t.Errorf("ровно предел: %+v %v", r, err)
	}
}

func mkdirNamed(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.Mkdir(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// Диалог закрыт без выбора — отмена, не ошибка.
func TestImportDialogCanceled(t *testing.T) {
	a, ev := printApp(t)
	pickOpen(t, "")
	res, err := a.ImportPrintProfile()
	if err != nil || !res.Canceled || res.Lines == nil || ev.count("print:changed") != 0 {
		t.Errorf("отмена: %+v %v", res, err)
	}
	save := pickSave(t, "")
	own := saved(t, a, editOf(t, a, "fabrika", "Для отмены"))
	sh, err := a.SharePrintProfile(own.ID)
	if err != nil || !sh.Canceled || sh.Name != "" {
		t.Errorf("отмена «Поделиться»: %+v %v", sh, err)
	}
	if save.DefaultFilename != "Для отмены.saprint" {
		t.Errorf("имя по умолчанию: %q", save.DefaultFilename)
	}
}

// «Поделиться» → «Добавить из файла» на другом компьютере: тот же id и профиль.
func TestSharePrintProfile(t *testing.T) {
	a, _ := printApp(t)
	res := saved(t, a, editOf(t, a, "fabrika", "Для друга"))
	dir := testkit.TempDir(t)
	// Расширение дописывается; существующий «чужой» файл с дописанным именем не затирается молча.
	pickSave(t, filepath.Join(dir, "Для друга"))
	sh, err := a.SharePrintProfile(res.ID)
	if err != nil || sh.Name != "Для друга.saprint" {
		t.Fatalf("поделиться: %+v %v", sh, err)
	}
	if _, err := a.SharePrintProfile(res.ID); err != ownprint.ErrExists {
		t.Errorf("повтор с дописанным именем: %v", err)
	}
	pickSave(t, filepath.Join(dir, "Для друга.saprint")) // подтверждённая диалогом замена — можно
	if _, err := a.SharePrintProfile(res.ID); err != nil {
		t.Errorf("замена выбранного файла: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "Для друга.saprint"))
	if err != nil {
		t.Fatal(err)
	}
	p, ok := printprof.Current().ByID(res.ID)
	if !ok || string(data) != string(encoded(t, p)) {
		t.Fatal("файл не равен профилю")
	}
	// Встроенную, чужой id и пустой — нельзя.
	for _, id := range []string{"fabrika", "fsfera", "netprint", "нет", ""} {
		if _, err := a.SharePrintProfile(id); err != ownprint.ErrNotOwn {
			t.Errorf("«Поделиться» %q: %v", id, err)
		}
	}
	// Другой компьютер: пустая папка, тот же файл — своя с тем же id.
	b := newApp(t)
	b.cfgDir, b.emit = testkit.TempDir(t), func(string, ...any) {}
	b.openDecor()
	t.Cleanup(func() { printprof.Install(nil) })
	b.ownp.Reload()
	if _, err := b.ownp.ImportPath(filepath.Join(dir, "Для друга.saprint")); err != nil {
		t.Fatal(err)
	}
	got, ok := printprof.Current().ByID(res.ID)
	if !ok || string(encoded(t, got)) != string(data) {
		t.Errorf("на другом компьютере: %+v", got)
	}
}

// Файл не в локальной папке: диалог вернул сетевой или относительный путь.
func TestShareRemotePath(t *testing.T) {
	a, _ := printApp(t)
	res := saved(t, a, editOf(t, a, "fabrika", "Сетевая"))
	pickSave(t, "относительный.saprint")
	if _, err := a.SharePrintProfile(res.ID); err != ownprint.ErrRemote {
		t.Errorf("относительный путь: %v", err)
	}
}

// Fuzz: любые байты в файле — ошибка или годная типография в пределах; паники нет, путь в
// ошибке не светится, принятая своя убирается удалением. Инвариант (аудит 123): разбор прошёл
// и название свободно — импорт без ошибки; принятая своя с тем же файлом — «такая уже есть», с
// другой версией — вопрос о замене (токен: чужой, просроченный и повторный — «устарел»).
func FuzzImportSaprint(f *testing.F) {
	seed := func(p printprof.Profile) {
		if data, err := printprof.Encode(p); err == nil {
			f.Add(data)
		}
	}
	for _, id := range []string{"fabrika", "fsfera", "netprint"} {
		p, _ := printprof.Builtin().ByID(id)
		p.Name = "Копия " + id
		seed(p)
	}
	f.Add([]byte{})
	f.Add([]byte("{"))
	f.Add([]byte(`{"format":1}`))
	dir := testkit.TempDir(f)
	a := NewApp()
	close(a.ready)
	a.cfgDir, a.emit = testkit.TempDir(f), func(string, ...any) {}
	a.openDecor()
	f.Cleanup(func() { a.closeBackground(); printprof.Install(nil) })
	a.ownp.Reload()
	path := filepath.Join(dir, "f.saprint")
	f.Fuzz(func(t *testing.T, data []byte) {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		parsed, perr := printprof.Parse(data)
		free := perr == nil && !ownprint.NameTaken(printprof.Current(), parsed.Name, "")
		res, err := a.ownp.ImportPath(path)
		switch {
		case err != nil:
			if strings.Contains(err.Error(), dir) {
				t.Fatalf("в ошибке путь: %v", err)
			}
			if free {
				t.Fatalf("разбор прошёл, название свободно, а импорт отказал: %v", err)
			}
		case res.Ask == "":
			p, ok := printprof.Current().ByID(res.Info.ID)
			if !ok || p.Validate() != nil || !printprof.ValidOwnID(p.ID) {
				t.Fatalf("принята негодная: %+v", res.Info)
			}
			fuzzReimport(t, a, path, p)
			if _, derr := a.DeletePrintProfile(p.ID); derr != nil {
				t.Fatal(derr)
			}
		}
		a.ownp.DropPending()
	})
}

// fuzzReimport — принятая своя p: тот же файл даёт «same», версия выше — вопрос о замене; ответ
// с чужим, просроченным и повторным токеном — ownprint.ErrStale, верный — замена.
func fuzzReimport(t *testing.T, a *App, path string, p printprof.Profile) {
	t.Helper()
	write := func(data []byte) {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	data, err := printprof.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	write(data)
	if res, err := a.ownp.ImportPath(path); err != nil || res.Ask != "same" {
		t.Fatalf("повтор того же файла: %q, %v", res.Ask, err)
	}
	p.Version++
	if data, err = printprof.Encode(p); err != nil {
		return // версия на пределе — вопроса не будет
	}
	write(data)
	ask := func() ownprint.Import {
		res, err := a.ownp.ImportPath(path)
		if err != nil || res.Ask != "replace" || res.Token == "" {
			t.Fatalf("вопрос о замене: %q, %v", res.Ask, err)
		}
		return res
	}
	res := ask()
	if _, err := a.ConfirmImportPrintProfile(res.Token + "0"); err != ownprint.ErrStale {
		t.Fatalf("чужой токен: %v", err)
	}
	res = ask()
	a.ownp.ExpirePending()
	if _, err := a.ConfirmImportPrintProfile(res.Token); err != ownprint.ErrStale {
		t.Fatalf("просроченный токен: %v", err)
	}
	res = ask()
	if _, err := a.ConfirmImportPrintProfile(res.Token); err != nil {
		t.Fatalf("замена: %v", err)
	}
	if _, err := a.ConfirmImportPrintProfile(res.Token); err != ownprint.ErrStale {
		t.Fatalf("повторный токен: %v", err)
	}
	if cur, ok := printprof.Current().ByID(p.ID); !ok || cur.Version != p.Version {
		t.Fatalf("после замены: %+v", cur)
	}
}
