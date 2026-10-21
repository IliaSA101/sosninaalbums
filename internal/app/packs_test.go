package app

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// d1Zip — набор, собранный скриптом выпуска D1 (scripts/library-pack.mjs), в своей папке.
func d1Zip(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "decor", "pack", "testdata", "d1-ivory.zip"))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(testkit.TempDir(t), "ivory-1.zip")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// rezip — копия набора с правкой манифеста и без записей drop (обновлённая версия набора).
func rezip(t *testing.T, src string, mut func(m map[string]any), drop ...string) string {
	t.Helper()
	zr, err := zip.OpenReader(src)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close() //nolint:errcheck // тест: чтение
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		if slices.Contains(drop, f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		if f.Name == "manifest.json" {
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatal(err)
			}
			mut(m)
			items := m["items"].([]any)
			m["items"] = slices.DeleteFunc(items, func(it any) bool { return slices.Contains(drop, it.(map[string]any)["file"].(string)) })
			data, _ = json.Marshal(m)
		}
		w, _ := zw.Create(f.Name)
		_, _ = w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(testkit.TempDir(t), "ivory-2.zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// events — счётчик событий App по имени.
type events struct {
	mu sync.Mutex
	n  map[string]int
}

func catchEvents(a *App) *events {
	ev := &events{n: map[string]int{}}
	a.emit = func(name string, _ ...any) {
		ev.mu.Lock()
		ev.n[name]++
		ev.mu.Unlock()
	}
	return ev
}

func (e *events) count(name string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.n[name]
}

func packBgIDs(t *testing.T, a *App) []string {
	t.Helper()
	cat, err := a.DecorCatalog(decor.KindBg)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, it := range cat.Items {
		if it.Pack != "" {
			ids = append(ids, it.ID)
		}
	}
	return ids
}

// Набор D1 файлом: ставится, виден в каталоге и списке, повтор — «уже есть», удаляется.
func TestAddPackPath(t *testing.T) {
	a := decorApp(t)
	ev := catchEvents(a)
	rev := a.editor().View().Rev
	res, err := a.AddPackPath(d1Zip(t))
	if err != nil || res.Ask != "" || res.Token != "" || len(res.Skipped) != 0 || res.Skipped == nil {
		t.Fatalf("установка: %v %+v", err, res)
	}
	p := res.Pack
	if p.ID != "ivory" || p.Title != "Свадьба — айвори" || p.Version != 1 || p.Bg != 2 || p.Sticker != 1 || p.Spread != 1 ||
		p.Origin != decor.OriginFile || !p.Removable || p.License != "CC0-1.0" {
		t.Fatalf("набор: %+v", p)
	}
	if ev.count("library:changed") != 1 {
		t.Errorf("событий library:changed %d", ev.count("library:changed"))
	}
	if v := a.editor().View(); v.Rev != rev || v.CanUndo {
		t.Errorf("установка без своих альбома дала правку: rev %d→%d", rev, v.Rev)
	}
	ids := packBgIDs(t, a)
	if len(ids) != 2 {
		t.Fatalf("плитки набора: %v", ids)
	}
	list, err := a.LibraryPacks()
	if err != nil || len(list.Packs) != 2 || list.Packs[0].Origin != decor.OriginBuiltin || list.Packs[1].ID != "ivory" || list.Busy || list.ReadOnly {
		t.Fatalf("список: %v %+v", err, list)
	}
	// Тот же набор ещё раз — «уже есть», ничего не пишется и события нет.
	again, err := a.AddPackPath(d1Zip(t))
	if err != nil || again.Ask != decor.AskSame || again.FromVersion != 1 || again.Pack.ID != "ivory" || ev.count("library:changed") != 1 {
		t.Fatalf("повтор: %v %+v", err, again)
	}
	plan, err := a.PlanRemovePack("ivory")
	if err != nil || plan.Files != 4 || plan.Kept != 0 || plan.Bytes == 0 {
		t.Fatalf("план удаления: %v %+v", err, plan)
	}
	list, err = a.RemovePack("ivory")
	if err != nil || len(list.Packs) != 1 || ev.count("library:changed") != 2 || len(packBgIDs(t, a)) != 0 {
		t.Fatalf("удаление: %v %+v", err, list)
	}
	if _, err := a.RemovePack(decor.BuiltinPack); !errors.Is(err, decor.ErrBuiltin) {
		t.Errorf("встроенный: %v", err)
	}
	if _, err := a.RemovePack("ivory"); !errors.Is(err, decor.ErrNotFound) || ev.count("library:changed") != 2 {
		t.Errorf("второй раз: %v", err)
	}
}

// Новая версия — вопрос с токеном; «Заменить» ставит её и убирает ключи старой; токен разовый.
func TestConfirmPack(t *testing.T) {
	a := decorApp(t)
	ev := catchEvents(a)
	v1 := d1Zip(t)
	if _, err := a.AddPackPath(v1); err != nil {
		t.Fatal(err)
	}
	before := packBgIDs(t, a)
	v2 := rezip(t, v1, func(m map[string]any) { m["version"] = 2 }, "bg/ivory-02.png")
	res, err := a.AddPackPath(v2)
	if err != nil || res.Ask != decor.AskUpdate || res.FromVersion != 1 || res.Token == "" || res.Pack.Version != 2 || res.Pack.Bg != 1 {
		t.Fatalf("вопрос: %v %+v", err, res)
	}
	if ev.count("library:changed") != 1 || len(packBgIDs(t, a)) != 2 {
		t.Fatal("до ответа набор уже заменён")
	}
	if _, err := a.ConfirmPack("чужой"); !errors.Is(err, errPackStale) {
		t.Errorf("чужой токен: %v", err)
	}
	// Чужой токен вопрос не снимает — «Заменить» по настоящему проходит.
	done, err := a.ConfirmPack(res.Token)
	if err != nil || done.Ask != "" || done.Pack.Version != 2 || done.FromVersion != 1 || ev.count("library:changed") != 2 {
		t.Fatalf("замена: %v %+v", err, done)
	}
	if got := packBgIDs(t, a); !slices.Equal(got, before[:1]) {
		t.Errorf("после замены плитки %v, ждали %v", got, before[:1])
	}
	if _, err := a.ConfirmPack(res.Token); !errors.Is(err, errPackStale) {
		t.Errorf("второй ответ: %v", err)
	}
	// Старая версия поверх новой — «стоит новее», ничего не меняется.
	if old, err := a.AddPackPath(v1); err != nil || old.Ask != decor.AskOlder || old.FromVersion != 2 {
		t.Errorf("старее: %v %+v", err, old)
	}
	// Вопрос, на который не ответили, снимает следующая операция; распакованное удаляется.
	v3 := rezip(t, v1, func(m map[string]any) { m["version"] = 3 })
	r3, err := a.AddPackPath(v3)
	if err != nil || r3.Ask != decor.AskUpdate {
		t.Fatal(err)
	}
	if _, err := a.RemovePack("nope"); !errors.Is(err, decor.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := a.ConfirmPack(r3.Token); !errors.Is(err, errPackStale) {
		t.Errorf("вопрос пережил другую операцию: %v", err)
	}
	assertNoTmpLib(t, a)
}

func assertNoTmpLib(t *testing.T, a *App) {
	t.Helper()
	_ = filepath.WalkDir(filepath.Join(a.cfgDir, decor.RootName), func(p string, d os.DirEntry, err error) error {
		if err != nil || !strings.HasSuffix(p, ".tmp") {
			return nil
		}
		if testkit.OwnTmp(filepath.Base(p)) {
			t.Errorf("остался временный %s", p)
		} else {
			t.Logf("чужой временный файл (антивирус) %s", p)
		}
		return nil
	})
}

// Просроченный вопрос не ставит набор (10 минут — подменяем срок).
func TestConfirmPackExpired(t *testing.T) {
	a := decorApp(t)
	v1 := d1Zip(t)
	if _, err := a.AddPackPath(v1); err != nil {
		t.Fatal(err)
	}
	res, err := a.AddPackPath(rezip(t, v1, func(m map[string]any) { m["version"] = 2 }))
	if err != nil || res.Token == "" {
		t.Fatal(err)
	}
	a.packs.pendMu.Lock()
	a.packs.pending.until = time.Now().Add(-time.Second)
	a.packs.pendMu.Unlock()
	if _, err := a.ConfirmPack(res.Token); !errors.Is(err, errPackStale) {
		t.Errorf("просроченный: %v", err)
	}
	if list, _ := a.LibraryPacks(); list.Packs[1].Version != 1 {
		t.Errorf("просроченный поставился: %+v", list.Packs[1])
	}
	assertNoTmpLib(t, a)
}

// Путь: только локальный обычный файл .zip; плохой архив — причина без пути; занято — отказ.
func TestAddPackPathChecks(t *testing.T) {
	a := decorApp(t)
	dir := testkit.TempDir(t)
	if err := os.Mkdir(filepath.Join(dir, "папка.zip"), 0o755); err != nil {
		t.Fatal(err)
	}
	junk := filepath.Join(dir, "мусор.zip")
	if err := os.WriteFile(junk, []byte("не zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	slip := rezip(t, d1Zip(t), func(map[string]any) {})
	data, _ := os.ReadFile(slip)
	data = bytes.ReplaceAll(data, []byte("LICENSE.txt"), []byte("../LICE.txt"))
	if err := os.WriteFile(slip, data, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]string{
		"относительный": "ivory.zip",
		"не zip":        filepath.Join(dir, "набор.txt"),
		"папка":         filepath.Join(dir, "папка.zip"),
		"нет файла":     filepath.Join(dir, "нет.zip"),
		"мусор":         junk,
		"../ в архиве":  slip,
		"сетевой":       `\\server\share\ivory.zip`,
	} {
		res, err := a.AddPackPath(p)
		if err == nil || res.Skipped == nil {
			t.Errorf("%s: принят %+v", name, res)
			continue
		}
		if strings.Contains(err.Error(), dir) || strings.Contains(err.Error(), p) {
			t.Errorf("%s: путь в тексте ошибки: %v", name, err)
		}
		if (name == "сетевой" || name == "относительный") && !errors.Is(err, errPackRemote) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if !a.packs.lock() {
		t.Fatal("занято с начала")
	}
	if _, err := a.AddPackPath(d1Zip(t)); !errors.Is(err, errPackBusy) {
		t.Errorf("занято: %v", err)
	}
	if list, _ := a.LibraryPacks(); !list.Busy {
		t.Error("список не видит занятость")
	}
	if _, err := a.RemovePack("ivory"); !errors.Is(err, errPackBusy) {
		t.Errorf("удаление при занятости: %v", err)
	}
	a.packs.unlock()
	if list, _ := a.LibraryPacks(); len(list.Packs) != 1 || list.Busy {
		t.Errorf("библиотека изменилась: %+v", list)
	}
	assertNoTmpLib(t, a)
	b := bgApp(t) // библиотека не открыта
	if _, err := b.AddPackPath(d1Zip(t)); !errors.Is(err, errNoLibrary) {
		t.Errorf("без библиотеки: %v", err)
	}
	if list, err := b.LibraryPacks(); !errors.Is(err, errNoLibrary) || list.Packs == nil {
		t.Errorf("список без библиотеки: %v %+v", err, list)
	}
}

// Альбом с другого компьютера: набор находит пропавшие фоны (один шаг, Undo возвращает);
// удаление набора файл, стоящий в открытом альбоме, не трогает.
func TestPackRelink(t *testing.T) {
	a := decorApp(t)
	zipPath := d1Zip(t)
	if _, err := a.AddPackPath(zipPath); err != nil {
		t.Fatal(err)
	}
	ids := packBgIDs(t, a)
	if _, err := a.ApplyBackground("s1", "spread", ids[0]); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := a.editor().Save(path); err != nil {
		t.Fatal(err)
	}
	b := decorApp(t) // своя пустая библиотека
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	b.ed.Store(mustEditor(t, p, path))
	pid := model.LibPhotoID(ids[0])
	if r, _ := b.editor().Ref(pid); r.Path != "" {
		t.Fatalf("нашлось без набора: %+v", r)
	}
	rev := b.editor().View().Rev
	if _, err := b.AddPackPath(zipPath); err != nil {
		t.Fatal(err)
	}
	r, _ := b.editor().Ref(pid)
	if want, _ := b.decorLib.Load().Find(ids[0]); r.Path == "" || r.Path != want || b.editor().View().Rev != rev+1 {
		t.Fatalf("не нашлось: %+v, ждали %q; rev %d→%d", r, want, rev, b.editor().View().Rev)
	}
	b.Undo()
	if r, _ := b.editor().Ref(pid); r.Path != "" {
		t.Errorf("отмена не вернула пустой путь: %+v", r)
	}
	b.Redo()
	plan, err := b.PlanRemovePack("ivory")
	if err != nil || plan.Kept != 1 || plan.Files != 3 {
		t.Fatalf("план при открытом альбоме: %v %+v", err, plan)
	}
	if _, err := b.RemovePack("ivory"); err != nil {
		t.Fatal(err)
	}
	if got, ok := b.decorLib.Load().Find(ids[0]); !ok || got != r.Path {
		t.Errorf("фон открытого альбома удалён: %q %v", got, ok)
	}
	waitMissing(t, b)
	if m := b.GetState().Missing; len(m) != 0 {
		t.Errorf("после удаления набора пропавшие: %v", m)
	}
}

// ShowLibraryFolder открывает ровно папку библиотеки (создаёт, если её нет); без библиотеки и
// при недоступной папке — ошибка без пути, проводник не запускается.
func TestShowLibraryFolder(t *testing.T) {
	var opened []string
	old := openDir
	openDir = func(d string) error { opened = append(opened, d); return nil }
	t.Cleanup(func() { openDir = old })

	if err := bgApp(t).ShowLibraryFolder(); !errors.Is(err, errNoLibrary) || len(opened) != 0 {
		t.Fatalf("без библиотеки: %v %v", err, opened)
	}

	a := decorApp(t)
	root := a.decorLib.Load().Root()
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if err := a.ShowLibraryFolder(); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 1 || opened[0] != root {
		t.Fatalf("открыто %v, ждали %s", opened, root)
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		t.Fatalf("папка не создана: %v", err)
	}

	// Недоступна: на месте папки файл — MkdirAll падает, путь в тексте не светится.
	b := decorApp(t)
	broot := b.decorLib.Load().Root()
	if err := os.RemoveAll(broot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(broot, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := b.ShowLibraryFolder()
	if err == nil || strings.Contains(err.Error(), broot) || len(opened) != 1 {
		t.Fatalf("недоступная папка: %v %v", err, opened)
	}

	// Проводник не запустился — ошибка тоже без пути.
	openDir = func(d string) error { return errors.New("exec " + d) }
	err = a.ShowLibraryFolder()
	if err == nil || strings.Contains(err.Error(), root) {
		t.Fatalf("отказ проводника: %v", err)
	}
}

// «Добавить из файла…»: отказ от диалога — пустой итог; выбор — установка и папка диалога.
func TestAddPackFile(t *testing.T) {
	a := decorApp(t)
	old := openFileDialog
	t.Cleanup(func() { openFileDialog = old })
	pick := ""
	var opts wr.OpenDialogOptions
	openFileDialog = func(_ context.Context, o wr.OpenDialogOptions) (string, error) { opts = o; return pick, nil }
	if res, err := a.AddPackFile(); err != nil || res.Pack.ID != "" || res.Skipped == nil {
		t.Fatalf("отказ: %v %+v", err, res)
	}
	if len(opts.Filters) != 1 || opts.Filters[0].Pattern != "*.zip" {
		t.Errorf("фильтр диалога %+v", opts.Filters)
	}
	pick = d1Zip(t)
	if res, err := a.AddPackFile(); err != nil || res.Pack.ID != "ivory" {
		t.Fatalf("выбор: %v %+v", err, res)
	}
	if got := prefs.DialogDir(a.cfgDir, dirPack); got != filepath.Dir(pick) {
		t.Errorf("папка диалога %q", got)
	}
}

// badZip — набор, где манифест годен, а все картинки не читаются: decor отбрасывает каждую.
func badZip(t *testing.T, names ...string) string {
	t.Helper()
	return namedZip(t, ".png", names...)
}

// namedZip — набор из предметов bg/bad-N<ext> с данными-«не картинками»; ext задаёт, кто
// отбросит предмет: .png — decor (не читается), .svg — манифест pack (SVG не принимается).
func namedZip(t *testing.T, ext string, names ...string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	items := make([]map[string]any, 0, len(names))
	for i, name := range names {
		data := []byte("не картинка " + name)
		sum := sha256.Sum256(data)
		file := fmt.Sprintf("bg/bad-%d%s", i, ext)
		items = append(items, map[string]any{"file": file, "kind": "bg", "theme": "wedding", "name": name,
			"sha256": hex.EncodeToString(sum[:]), "size": len(data)})
		w, _ := zw.Create(file)
		_, _ = w.Write(data)
	}
	m, _ := json.Marshal(map[string]any{"format": 1, "id": "bad", "version": 1, "minApp": "0.16.0", "title": "Плохой",
		"license": "CC0-1.0", "items": items})
	w, _ := zw.Create("manifest.json")
	_, _ = w.Write(m)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(testkit.TempDir(t), "bad-1.zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Все предметы отброшены: ErrPackEmpty, и причина (имя первого, почему, сколько ещё) — в тексте
// ошибки: Skipped до тоста не доходит. Путей в тексте нет, библиотека прежняя.
func TestAddPackEmpty(t *testing.T) {
	a := decorApp(t)
	for _, c := range []struct {
		names []string
		more  string
	}{
		{[]string{"Фон А"}, ""},
		{[]string{"Фон А", "Фон Б", "Фон В"}, "(и ещё 2)"},
	} {
		p := badZip(t, c.names...)
		res, err := a.AddPackPath(p)
		if !errors.Is(err, decor.ErrPackEmpty) || len(res.Skipped) != len(c.names) {
			t.Fatalf("%v: %v %+v", c.names, err, res)
		}
		msg := err.Error()
		if res.Skipped[0].Why == "" || !strings.Contains(msg, "Фон А — "+res.Skipped[0].Why) ||
			strings.Contains(msg, "Фон Б") || strings.Contains(msg, filepath.Dir(p)) || strings.Contains(msg, a.cfgDir) {
			t.Errorf("%v: текст %q", c.names, msg)
		}
		if (c.more == "") == strings.Contains(msg, "и ещё") || c.more != "" && !strings.Contains(msg, c.more) {
			t.Errorf("%v: «и ещё» в %q", c.names, msg)
		}
	}
	if list, _ := a.LibraryPacks(); len(list.Packs) != 1 {
		t.Errorf("пустой набор поставился: %+v", list.Packs)
	}
	assertNoTmpLib(t, a)
	// Другие ошибки packEmptyErr не трогает; без причин — ошибка как есть.
	if err := packEmptyErr(errPackBusy, []DecorSkip{{Name: "x", Why: "y"}}); err.Error() != errPackBusy.Error() {
		t.Errorf("чужая ошибка: %v", err)
	}
	if err := packEmptyErr(decor.ErrPackEmpty, nil); err.Error() != decor.ErrPackEmpty.Error() {
		t.Errorf("без причин: %v", err)
	}
}

// Манифест отбросил все предметы (SVG): текст ошибки AddPackPath несёт первую причину и «и ещё N»,
// без путей; errors.Is(pack.ErrEmpty) держится. Раньше причина терялась вместе с пустым Manifest.
func TestAddPackManifestEmpty(t *testing.T) {
	a := decorApp(t)
	for _, c := range []struct {
		names []string
		more  string
	}{
		{[]string{"Фон А"}, ""},
		{[]string{"Фон А", "Фон Б", "Фон В"}, "(и ещё 2)"},
	} {
		p := namedZip(t, ".svg", c.names...)
		_, err := a.AddPackPath(p)
		if !errors.Is(err, pack.ErrEmpty) {
			t.Fatalf("%v: %v", c.names, err)
		}
		msg := err.Error()
		if !strings.Contains(msg, "bg/bad-0.svg — ") || !strings.Contains(msg, "SVG") ||
			strings.Contains(msg, "bad-1") || strings.Contains(msg, filepath.Dir(p)) || strings.Contains(msg, a.cfgDir) {
			t.Errorf("%v: текст %q", c.names, msg)
		}
		if (c.more == "") == strings.Contains(msg, "и ещё") || c.more != "" && !strings.Contains(msg, c.more) {
			t.Errorf("%v: «и ещё» в %q", c.names, msg)
		}
	}
	if list, _ := a.LibraryPacks(); len(list.Packs) != 1 {
		t.Errorf("пустой набор поставился: %+v", list.Packs)
	}
	assertNoTmpLib(t, a)
}

func packIDs(t *testing.T, a *App, kind string) []string {
	t.Helper()
	cat, err := a.DecorCatalog(kind)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, it := range cat.Items {
		if it.Pack != "" {
			ids = append(ids, it.ID)
		}
	}
	return ids
}

// Ключи набора только в истории (отменены — в возврате; после Redo — в отмене и возврате)
// держатся при удалении набора: «В открытом альбоме всё останется» — и после Ctrl+Y фон и
// стикер на месте.
func TestRemovePackKeepsHistory(t *testing.T) {
	a := decorApp(t)
	if _, err := a.AddPackPath(d1Zip(t)); err != nil {
		t.Fatal(err)
	}
	if keys := a.editor().LibKeys(); len(keys) != 0 {
		t.Fatalf("ключи пустого альбома: %v", keys)
	}
	bg, st := packIDs(t, a, decor.KindBg)[0], packIDs(t, a, decor.KindSticker)[0]
	if _, err := a.ApplyBackground("s1", "spread", bg); err != nil {
		t.Fatal(err)
	}
	if _, err := a.AddSticker("s2", 0, st); err != nil {
		t.Fatal(err)
	}
	a.Undo()
	a.Undo()
	if refs := a.editor().Refs(); len(refs) != 0 {
		t.Fatalf("после отмены в проекте %v", refs)
	}
	if keys := a.editor().LibKeys(); len(keys) != 2 || !keys[bg] || !keys[st] {
		t.Fatalf("ключи из возврата: %v", keys)
	}
	a.Redo() // фон — в проекте, стикер — только в возврате
	plan, err := a.PlanRemovePack("ivory")
	if err != nil || plan.Kept != 2 || plan.Files != 2 {
		t.Fatalf("план: %v %+v", err, plan)
	}
	if _, err := a.RemovePack("ivory"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{bg, st} {
		if _, ok := a.decorLib.Load().Find(id); !ok {
			t.Errorf("файл %s из истории удалён", id)
		}
	}
	a.Redo()
	waitMissing(t, a)
	if v := a.editor().View(); v.CanRedo || len(a.GetState().Missing) != 0 {
		t.Errorf("после возврата пропавшие: %v", a.GetState().Missing)
	}
}
