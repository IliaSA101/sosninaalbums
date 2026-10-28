package app

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/libpacks"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Края сети библиотеки (ревью 0.17, решение №4): только чтение, пропуск только сетевых, квота
// обновления у предела, совпадение со встроенным, ветки netInstall, порядок отзыва.

// libDir — папка библиотеки App.
func libDir(a *App) string { return filepath.Join(a.cfgDir, decor.RootName) }

// fillLib — набор-заглушка id на total байт (только реестр: квота считается по нему) и
// перечитанная библиотека.
func fillLib(t *testing.T, a *App, id string, total int64) {
	t.Helper()
	var items []map[string]any
	for i := 0; total > 0; i++ {
		n := min(total, decor.MaxBgBytes)
		items = append(items, map[string]any{"key": fmt.Sprintf("my:f%015x", i), "kind": decor.KindBg, "theme": "basic", "name": "З", "bytes": n})
		total -= n
	}
	data, err := json.Marshal(map[string]any{"v": 1, "id": id, "version": 1, "title": id, "origin": decor.OriginFile, "items": items})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(libDir(a), "Наборы")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	a.openDecor()
}

// garbleZip — копия набора, где у предметов вместо содержимого мусор той же длины: архив
// цел, манифест сходится, но ни один предмет не проходит проверку (ErrPackEmpty).
func garbleZip(t *testing.T, src string) string {
	t.Helper()
	zr, err := zip.OpenReader(src)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close() //nolint:errcheck // тест: чтение
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		switch f.Name {
		case "LICENSE.txt":
		case "manifest.json": // sha256 предметов — от мусора: иначе Extract счёл бы архив битым
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatal(err)
			}
			for _, it := range m["items"].([]any) {
				it := it.(map[string]any)
				sum := sha256.Sum256(bytes.Repeat([]byte("x"), int(it["size"].(float64))))
				it["sha256"] = hex.EncodeToString(sum[:])
			}
			data, _ = json.Marshal(m)
		default:
			data = bytes.Repeat([]byte("x"), len(data))
		}
		w, _ := zw.Create(f.Name)
		_, _ = w.Write(data)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(testkit.TempDir(t), "ivory-bad.zip")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func fileSize(t *testing.T, p string) int64 {
	t.Helper()
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return fi.Size()
}

// Только чтение (реестр новее программы): кнопка — ErrReadOnly (не «не записывается на диск»),
// при запуске — молча; ни одного запроса, checked не записан; в том же сеансе проверка при
// запуске не повторяется и после того, как библиотека стала записываемой.
func TestUpdateLibraryReadOnly(t *testing.T) {
	defer setStart(0, 20*time.Hour)()
	a, l := netApp(t, true)
	ev := captureEvents(a)
	l.publish(t, nil, d1Zip(t))
	if err := os.MkdirAll(libDir(a), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := filepath.Join(libDir(a), "library.json")
	if err := os.WriteFile(reg, []byte(`{"version":99,"items":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	a.openDecor()
	_, err := a.UpdateLibrary()
	if !errors.Is(err, decor.ErrReadOnly) || errors.Is(err, fetch.ErrDisk) || err.Error() != decor.ErrReadOnly.Error() {
		t.Fatalf("кнопка: %v", err)
	}
	if _, err := a.SetLibraryNet(true, true); err != nil {
		t.Fatal(err)
	}
	a.libp.ScheduleCheck()
	a.bg.Wait()
	if l.hits.Load() != 0 || prefs.LibraryNetOf(a.cfgDir).Checked != 0 || len(ev.of("notice")) != 0 || len(ev.of("library:progress")) != 0 {
		t.Fatalf("только чтение: запросов %d, checked %d", l.hits.Load(), prefs.LibraryNetOf(a.cfgDir).Checked)
	}
	if list, _ := a.LibraryPacks(); !list.ReadOnly || list.Busy || len(list.Packs) != 1 {
		t.Fatalf("список: %+v", list)
	}
	if err := os.Remove(reg); err != nil {
		t.Fatal(err)
	}
	a.openDecor()
	a.libp.ScheduleCheck()
	a.bg.Wait()
	if l.hits.Load() != 0 {
		t.Fatalf("повтор в том же сеансе: запросов %d", l.hits.Load())
	}
	if res, err := getAll(t, a); err != nil || res.Added != 1 {
		t.Fatalf("записываемая: %v %+v", err, res)
	}
}

// Пропуск удалённых — только сетевых: из файла удалён — skipped 0, из сети — 1; оба в одной
// библиотеке.
func TestRemovePackSkipOnlyNet(t *testing.T) {
	a, l := netApp(t, true)
	src := d1Zip(t)
	if _, err := a.AddPackPath(src); err != nil {
		t.Fatal(err)
	}
	sage := rezip(t, src, func(m map[string]any) { m["id"], m["title"] = "sage", "Шалфей" })
	l.publish(t, nil, sage)
	if res, err := getAll(t, a); err != nil || res.Added != 1 {
		t.Fatalf("из сети: %v %+v", err, res)
	}
	list, err := a.RemovePack("ivory")
	if err != nil || list.Net.Skipped != 0 || len(prefs.LibraryNetOf(a.cfgDir).Skip) != 0 {
		t.Fatalf("из файла: %v %+v %v", err, list.Net, prefs.LibraryNetOf(a.cfgDir).Skip)
	}
	list, err = a.RemovePack("sage")
	if skip := prefs.LibraryNetOf(a.cfgDir).Skip; err != nil || list.Net.Skipped != 1 || len(skip) != 1 || skip[0] != "sage" {
		t.Fatalf("из сети: %v %+v %v", err, list.Net, skip)
	}
	// Неудачное удаление (нет такого) пропуск не пишет.
	if _, err := a.RemovePack("rose"); err == nil || len(prefs.LibraryNetOf(a.cfgDir).Skip) != 1 {
		t.Fatalf("нет набора: %v %v", err, prefs.LibraryNetOf(a.cfgDir).Skip)
	}
}

// Квота у предела: обновление сетевого набора не считает стоящую версию (ровно предел —
// обновляется; байт сверх — ErrLibFull без запроса архива); новый набор — так же по размеру
// архива.
func TestNetQuotaAtLimit(t *testing.T) {
	a, l := netApp(t, true)
	zip := d1Zip(t)
	l.publish(t, nil, zip)
	if res, err := getAll(t, a); err != nil || res.Added != 1 {
		t.Fatalf("первая версия: %v %+v", err, res)
	}
	v2 := rezip(t, zip, func(m map[string]any) { m["version"] = 2 })
	size := fileSize(t, v2)
	plan, _ := a.PlanRemovePack("ivory") // байты на диске: PackInfo.Bytes у набора из сети — размер zip (137-31)
	if b := plan.Bytes; b <= 0 || b > size {
		t.Fatalf("байты набора %d, архива %d: предел ниже не проверить", b, size)
	}
	l.publish(t, nil, v2)
	fillLib(t, a, "full", decor.MaxLibraryLen-size+1)
	arc := l.archives.Load()
	// Квота — по набору: ход не рвётся, набор в итоге «не скачались», архив не запрошен.
	if res, err := a.UpdateLibrary(); err != nil || len(res.Failed) != 1 || res.Failed[0].Why != decor.ErrLibFull.Error() || l.archives.Load() != arc {
		t.Fatalf("байт сверх: %v %+v, архивов %d", err, res, l.archives.Load()-arc)
	}
	fillLib(t, a, "full", decor.MaxLibraryLen-size)
	if res, err := a.UpdateLibrary(); err != nil || res.Updated != 1 || l.archives.Load() != arc+1 {
		t.Fatalf("ровно предел: %v %+v", err, res)
	}

	// Новый набор: стоящей версии нет — вычитать нечего.
	b := fresh(t, l)
	sage := rezip(t, zip, func(m map[string]any) { m["id"], m["title"] = "sage", "Шалфей" })
	l.publish(t, nil, sage)
	size = fileSize(t, sage)
	fillLib(t, b, "full", decor.MaxLibraryLen-size+1)
	arc = l.archives.Load()
	if res, err := getAll(t, b); err != nil || len(res.Failed) != 1 || res.Failed[0].Why != decor.ErrLibFull.Error() || l.archives.Load() != arc {
		t.Fatalf("новый сверх: %v %+v, архивов %d", err, res, l.archives.Load()-arc)
	}
	if prefs.LibraryNetOf(b.cfgDir).Checked == 0 { // поштучная неудача проверку закрывает
		t.Fatal("checked не записан при квоте")
	}
	fillLib(t, b, "full", decor.MaxLibraryLen-size)
	if res, err := b.DownloadAllLibrary(); err != nil || res.Added != 1 {
		t.Fatalf("новый ровно предел: %v %+v", err, res)
	}
}

// fresh — второй App с библиотекой и согласием (источник — от вызывающего).
func fresh(t *testing.T, l *netLib) *App {
	t.Helper()
	b := decorApp(t)
	l.attach(t, b) // служба наборов пересобирается до первого вызова, SetLibraryNet — уже на ней
	if _, err := b.SetLibraryNet(true, false); err != nil {
		t.Fatal(err)
	}
	return b
}

// Ветки netInstall: архив подлинный, но о другом наборе (id или версия) — «не скачались» с
// причиной; архив не открывается (minApp в манифесте новее программы) — причина pack; ни один
// предмет не прошёл — «нечего ставить» (причина первого предмета в тексте). Раньше это были
// пропуски: «Скачать» одного набора показывает причину нажатому, пропуск ему не виден (137-29).
// Библиотека прежняя, временного файла нет.
func TestNetInstallSkips(t *testing.T) {
	defer func(old []byte) { wailsJSON = old }(wailsJSON)
	wailsJSON = []byte(`{"info":{"productVersion":"0.17.0"}}`)
	a, l := netApp(t, true)
	zip := d1Zip(t)
	newer := rezip(t, zip, func(m map[string]any) { m["minApp"] = "0.99.0" })
	for name, tc := range map[string]struct {
		zip  string
		mut  func(r map[string]any)
		why  string
		item string // непусто — why только начало, дальше имя предмета
	}{
		"другой id":      {zip, func(r map[string]any) { r["id"] = "sage" }, "архив не совпадает со списком наборов", ""},
		"другая версия":  {zip, func(r map[string]any) { r["version"] = 2 }, "архив не совпадает со списком наборов", ""},
		"pack.Open":      {newer, func(r map[string]any) { r["minApp"] = "0.1.0" }, "нужна версия Sosnina Albums 0.99.0 или новее", ""},
		"нечего ставить": {garbleZip(t, zip), nil, decor.ErrPackEmpty.Error(), "Фон 1"},
	} {
		arc := l.archives.Load()
		l.publish(t, tc.mut, tc.zip)
		res, err := getAll(t, a)
		if err != nil || res.Added+res.Updated != 0 || len(res.Failed) != 1 || l.archives.Load() != arc+1 {
			t.Fatalf("%s: %v %+v", name, err, res)
		}
		k := res.Failed[0]
		if k.Name != "Свадьба — айвори" {
			t.Errorf("%s: набор %+v", name, k)
		}
		if tc.item == "" && k.Why != tc.why {
			t.Errorf("%s: причина %q, ждали %q", name, k.Why, tc.why)
		}
		// «Нечего ставить»: после общей фразы идёт причина первого предмета — с его именем.
		if tc.item != "" && (!strings.HasPrefix(k.Why, tc.why) || !strings.Contains(k.Why[len(tc.why):], tc.item)) {
			t.Errorf("%s: причина %q, ждали «%s…%s»", name, k.Why, tc.why, tc.item)
		}
		if list, _ := a.LibraryPacks(); len(list.Packs) != 1 {
			t.Fatalf("%s: библиотека изменилась: %+v", name, list.Packs)
		}
		noDownloadLeft(t, a)
	}
}

// Два набора, второй падает (хэш архива): первый остаётся, library:changed есть, checked записан
// (поштучная неудача проверку закрывает), второй — в res.Failed, а не ошибкой вызова (ход не рвётся).
func TestNetInstallSecondFails(t *testing.T) {
	a, l := netApp(t, true)
	ev := captureEvents(a)
	zip := d1Zip(t)
	sage := rezip(t, zip, func(m map[string]any) { m["id"], m["title"] = "sage", "Шалфей" })
	l.publish(t, func(r map[string]any) {
		if r["id"] == "sage" {
			r["sha256"] = strings.Repeat("0", 64)
		}
	}, zip, sage)
	res, err := getAll(t, a)
	if err != nil || res.Added != 1 || len(res.Failed) != 1 || res.Failed[0].Name != "Шалфей" || res.Failed[0].Why != "скачался с ошибкой, повторите" {
		t.Fatalf("второй: %v %+v", err, res)
	}
	list, _ := a.LibraryPacks()
	if len(list.Packs) != 2 || list.Packs[1].ID != "ivory" || list.Net.Checked == 0 || len(ev.of("library:changed")) != 1 {
		t.Fatalf("после ошибки: %+v, событий %d", list, len(ev.of("library:changed")))
	}
	if _, err := os.Stat(filepath.Join(libDir(a), "Наборы", ".dl-sage.tmp")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("временный файл остался: %v", err)
	}
}

// Предмет в скачанном архиве не сходится с манифестом, а sha архива в каталоге верный (ошибка
// издателя): тост «скачался с ошибкой», не «на диск»; без путей; в log.txt — тип; набор не
// поставлен.
func TestNetInstallItemMismatch(t *testing.T) {
	a, l := netApp(t, true)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	bad := rezip(t, d1Zip(t), func(m map[string]any) { // последний из нескольких предметов
		items := m["items"].([]any)
		items[len(items)-1].(map[string]any)["sha256"] = strings.Repeat("0", 64)
	})
	l.publish(t, nil, bad)
	res, err := getAll(t, a)
	if err != nil || res.Added != 0 || len(res.Failed) != 1 || res.Failed[0].Name != "Свадьба — айвори" || !strings.Contains(res.Failed[0].Why, "скачался с ошибкой") ||
		strings.Contains(res.Failed[0].Why, "на диск") || strings.Contains(res.Failed[0].Why, a.cfgDir) {
		t.Fatalf("итог: %v %+v", err, res)
	}
	out := buf.String()
	if strings.Contains(out, a.cfgDir) || strings.Contains(out, filepath.ToSlash(a.cfgDir)) || !strings.Contains(out, "не сходится с манифестом (") {
		t.Fatalf("log.txt: %s", out)
	}
	noDownloadLeft(t, a)
	list, _ := a.LibraryPacks()
	for _, p := range list.Packs {
		if p.ID == "ivory" {
			t.Fatalf("набор поставлен: %+v", list.Packs)
		}
	}
}

// installedOf — названия из последнего library:changed (событий должно быть ровно n).
func installedOf(t *testing.T, ev *capture, n int) []string {
	t.Helper()
	all := ev.of("library:changed")
	if len(all) != n {
		t.Fatalf("library:changed: событий %d, ждали %d", len(all), n)
	}
	c, ok := all[n-1].(libpacks.Changed)
	if !ok || c.Installed == nil {
		t.Fatalf("library:changed: %#v — нужен libChanged со списком (не null)", all[n-1])
	}
	return c.Installed
}

// Данные library:changed: сеть (оба набора сразу), обновление, удаление — названия
// (PackInfo.Title) для темы окна (073-1); удаление — пустой список, не nil (в JSON [], не null).
func TestLibraryChangedInstalled(t *testing.T) {
	a, l := netApp(t, true)
	ev := captureEvents(a)
	zip := d1Zip(t)
	sage := rezip(t, zip, func(m map[string]any) { m["id"], m["title"] = "sage", "Шалфей" })
	l.publish(t, nil, zip, sage)
	if res, err := getAll(t, a); err != nil || res.Added != 2 {
		t.Fatalf("сеть: %v %+v", err, res)
	}
	if got := installedOf(t, ev, 1); len(got) != 2 || got[0] != "Свадьба — айвори" || got[1] != "Шалфей" {
		t.Fatalf("сеть, названия: %v", got)
	}
	// Обновление обоих до версии 2: это тоже «поставлено», названия — те же.
	v2 := rezip(t, zip, func(m map[string]any) { m["version"] = 2 }, before2File(t, zip))
	v2s := rezip(t, sage, func(m map[string]any) { m["version"] = 2 }, before2File(t, sage))
	l.publish(t, nil, v2, v2s)
	if res, err := a.UpdateLibrary(); err != nil || res.Updated != 2 || res.Added != 0 {
		t.Fatalf("обновление: %v %+v", err, res)
	}
	if got := installedOf(t, ev, 2); len(got) != 2 || got[0] != "Свадьба — айвори" || got[1] != "Шалфей" {
		t.Fatalf("обновление, названия: %v", got)
	}
	// Холостая проверка: ничего нового — ни library:changed, ни library:updated (запуск).
	a.libp.StartCheck()
	if len(ev.of("library:changed")) != 2 || len(ev.of("library:updated")) != 0 {
		t.Fatalf("холостая проверка дала события: %d / %d", len(ev.of("library:changed")), len(ev.of("library:updated")))
	}
	// Удаление: пустой, но не nil список.
	if _, err := a.RemovePack("sage"); err != nil {
		t.Fatal(err)
	}
	all := ev.of("library:changed")
	if len(all) != 3 {
		t.Fatalf("удаление: событий %d", len(all))
	}
	last, ok := all[2].(libpacks.Changed)
	if !ok || last.Installed == nil || len(last.Installed) != 0 {
		t.Fatalf("удаление: %#v", all[2])
	}
	if raw, _ := json.Marshal(last); string(raw) != `{"installed":[]}` {
		t.Errorf("JSON удаления: %s", raw)
	}
}

// Файл: AddPackPath и ConfirmPack («Заменить версией N») шлют название набора; «уже есть» и
// вопрос о замене до ответа — не шлют.
func TestLibraryChangedFile(t *testing.T) {
	a := decorApp(t)
	ev := captureEvents(a)
	zip := d1Zip(t)
	if _, err := a.AddPackPath(zip); err != nil {
		t.Fatal(err)
	}
	if got := installedOf(t, ev, 1); len(got) != 1 || got[0] != "Свадьба — айвори" {
		t.Fatalf("файл: %v", got)
	}
	if r, err := a.AddPackPath(zip); err != nil || r.Ask != decor.AskSame {
		t.Fatalf("повтор: %v %+v", err, r)
	}
	installedOf(t, ev, 1)
	v2 := rezip(t, zip, func(m map[string]any) { m["version"] = 2; m["title"] = "Айвори 2" }, before2File(t, zip))
	r, err := a.AddPackPath(v2)
	if err != nil || r.Ask != decor.AskUpdate || r.Token == "" {
		t.Fatalf("вопрос: %v %+v", err, r)
	}
	installedOf(t, ev, 1) // до «Заменить» события нет
	if _, err := a.ConfirmPack(r.Token); err != nil {
		t.Fatal(err)
	}
	if got := installedOf(t, ev, 2); len(got) != 1 || got[0] != "Айвори 2" {
		t.Fatalf("замена: %v", got)
	}
}

// Ошибка на втором наборе: первый поставлен — его название в library:changed, второго нет.
func TestLibraryChangedPartial(t *testing.T) {
	a, l := netApp(t, true)
	ev := captureEvents(a)
	zip := d1Zip(t)
	sage := rezip(t, zip, func(m map[string]any) { m["id"], m["title"] = "sage", "Шалфей" })
	l.publish(t, func(r map[string]any) {
		if r["id"] == "sage" {
			r["sha256"] = strings.Repeat("0", 64)
		}
	}, zip, sage)
	if res, err := getAll(t, a); err != nil || len(res.Failed) != 1 {
		t.Fatalf("ждали второй набор в Failed: %v %+v", err, res)
	}
	if got := installedOf(t, ev, 1); len(got) != 1 || got[0] != "Свадьба — айвори" {
		t.Fatalf("названия: %v", got)
	}
}

// Строка итога в log.txt после каждой удачной проверки — и кнопкой, и при запуске; при нуле
// тоже; после ошибки — нет.
func TestLibraryLogSummary(t *testing.T) {
	a, l := netApp(t, true)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	zip := d1Zip(t)
	l.publish(t, nil, zip)
	if _, err := getAll(t, a); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "библиотека: итог — новых 1, обновлено 0, пропущено 0") {
		t.Fatalf("после установки: %q", out)
	}
	// Ноль — тоже строка (запуск).
	buf.Reset()
	a.libp.StartCheck()
	if out := buf.String(); !strings.Contains(out, "библиотека: итог — новых 0, обновлено 0, пропущено 0") {
		t.Fatalf("при нуле: %q", out)
	}
	// Обновление: версия 2 ивори; новый набор, которому нужна новая программа, не пропуск, а серая
	// строка раздела («пропущено» считает причины стоящих наборов).
	buf.Reset()
	v2 := rezip(t, zip, func(m map[string]any) { m["version"] = 2 }, before2File(t, zip))
	defer func(old []byte) { wailsJSON = old }(wailsJSON)
	wailsJSON = []byte(`{"info":{"productVersion":"0.17.0"}}`)
	later := rezip(t, zip, func(m map[string]any) { m["id"], m["title"] = "later", "Позже" })
	l.publish(t, func(r map[string]any) {
		if r["id"] == "later" {
			r["minApp"] = "0.99.0"
		}
	}, v2, later)
	if _, err := a.UpdateLibrary(); err != nil {
		t.Fatal(err)
	}
	if out := buf.String(); !strings.Contains(out, "библиотека: итог — новых 0, обновлено 1, пропущено 0") {
		t.Fatalf("обновление с пропуском: %q", out)
	}
	if list, _ := a.LibraryPacks(); len(list.Available) != 1 || !strings.Contains(list.Available[0].Why, "0.99.0") {
		t.Fatalf("серая строка: %+v", list.Available)
	}
	// Неудачная проверка итога не пишет.
	buf.Reset()
	l.mu.Lock()
	l.sig = nil
	l.mu.Unlock()
	if _, err := a.UpdateLibrary(); err == nil {
		t.Fatal("ждали ошибку")
	}
	if strings.Contains(buf.String(), "итог") {
		t.Errorf("итог после ошибки: %q", buf.String())
	}
}

// Дополнение (needs, 129 Т8): донор в плане, но не встал (архив не о том наборе) — дополнение не
// скачивается и не ставится, иначе его шаблоны ссылались бы на картинки донора, которых нет.
func TestNetAddonDonorNotInstalled(t *testing.T) {
	a, l := netApp(t, true)
	zip := d1Zip(t)
	add := rezip(t, zip, func(m map[string]any) { m["id"], m["title"] = "ivory-add", "Айвори+" })
	l.publish(t, func(r map[string]any) {
		switch r["id"] {
		case "ivory":
			r["version"] = 2 // архив v1 не совпадает со строкой каталога: донор пропущен
		case "ivory-add":
			r["needs"] = "ivory"
		}
	}, zip, add)
	res, err := getAll(t, a)
	if err != nil || res.Added+res.Updated != 0 || l.archives.Load() != 1 {
		t.Fatalf("итог: %v %+v, архивов %d", err, res, l.archives.Load())
	}
	if len(res.Failed) != 1 || res.Failed[0].Why != "архив не совпадает со списком наборов" || len(res.Skipped) != 0 {
		t.Errorf("причина у донора, у дополнения молчок: %+v %+v", res.Failed, res.Skipped)
	}
	if list, _ := a.LibraryPacks(); len(list.Packs) != 1 {
		t.Fatalf("библиотека изменилась: %+v", list.Packs)
	}
	noDownloadLeft(t, a)
}

// Дополнение стоит в каталоге раньше донора: оба ставятся за одно «Обновить», донор первым.
func TestNetAddonBeforeDonor(t *testing.T) {
	a, l := netApp(t, true)
	zip := d1Zip(t)
	add := rezip(t, zip, func(m map[string]any) { m["id"], m["title"] = "ivory-add", "Айвори+" })
	l.publish(t, func(r map[string]any) {
		if r["id"] == "ivory-add" {
			r["needs"] = "ivory"
		}
	}, add, zip)
	res, err := getAll(t, a)
	if err != nil || res.Added != 2 || len(res.Skipped) != 0 {
		t.Fatalf("итог: %v %+v", err, res)
	}
	list, _ := a.LibraryPacks()
	if len(list.Packs) != 3 || list.Packs[1].ID != "ivory" || list.Packs[2].ID != "ivory-add" {
		t.Fatalf("библиотека: %+v", list.Packs)
	}
}
