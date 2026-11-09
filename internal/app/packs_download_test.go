package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/libpacks"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// «Скачать» и «Скачать все» (спека 137 Т4): выборочная установка по сохранённому списку.

func skipOf(a *App) []string { return prefs.LibraryNetOf(a.cfgDir).Skip }

func standing(t *testing.T, a *App) []string {
	t.Helper()
	list, err := a.LibraryPacks()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, p := range list.Packs {
		if p.Origin == decor.OriginNet {
			ids = append(ids, p.ID)
		}
	}
	return ids
}

// addonOf — строка каталога «ivory-add» как дополнение к «ivory».
func addonOf(r map[string]any) {
	if r["id"] == "ivory-add" {
		r["needs"] = "ivory"
	}
}

// withAddon — каталог из ivory и его дополнения; список уже сохранён «Обновить» (новых не ставит).
func withAddon(t *testing.T) *App {
	t.Helper()
	a, l := netApp(t, true)
	zip := d1Zip(t)
	add := rezip(t, zip, func(m map[string]any) { m["id"], m["title"] = "ivory-add", "Айвори+" })
	l.publish(t, addonOf, zip, add)
	warm(t, a)
	return a
}

// warm — сохранённый список от «Обновить»: новых наборов он не ставит, checked сбрасывается.
func warm(t *testing.T, a *App) {
	t.Helper()
	if _, err := a.UpdateLibrary(); err != nil {
		t.Fatal(err)
	}
	prefs.UpdateLibraryNet(a.cfgDir, func(n *prefs.LibraryNet) { n.Checked = 0 })
}

// «Скачать» ставит один набор из четырёх: остальные не запрошены; нажатый стоит (picked), список
// раздела укоротился; checked «Скачать» не трогает (проверку обновлений стоящих он не заменяет).
func TestDownloadOnePack(t *testing.T) {
	a, l := netApp(t, true)
	ev := captureEvents(a)
	l.publish(t, nil, fourZips(t)...)
	warm(t, a)
	prefs.UpdateLibraryNet(a.cfgDir, func(n *prefs.LibraryNet) { n.Checked = 12345 })
	res, err := a.DownloadLibraryPack("sage")
	if err != nil || !res.Picked || res.Why != "" || res.Added != 1 || res.Available != 3 || l.archives.Load() != 1 {
		t.Fatalf("скачать: %v %+v, архивов %d", err, res, l.archives.Load())
	}
	if got := standing(t, a); !slices.Equal(got, []string{"sage"}) {
		t.Fatalf("встало: %v", got)
	}
	if prefs.LibraryNetOf(a.cfgDir).Checked != 12345 {
		t.Error("«Скачать» одного набора сдвинул checked")
	}
	if got := installedOf(t, ev, 1); len(got) != 1 || got[0] != "Шалфей" {
		t.Errorf("library:changed: %v", got)
	}
	// Стоящий набор: «уже стоит», без скачивания.
	again, err := a.DownloadLibraryPack("sage")
	if err != nil || again.Picked || again.Why != "уже стоит" || l.archives.Load() != 1 {
		t.Fatalf("повтор: %v %+v", err, again)
	}
}

// Донор приходит с дополнениями; дополнение тянет нестоящего донора.
func TestDownloadWithAddons(t *testing.T) {
	a := withAddon(t)
	res, err := a.DownloadLibraryPack("ivory")
	if err != nil || !res.Picked || res.Added != 2 {
		t.Fatalf("донор с дополнением: %v %+v", err, res)
	}
	if got := standing(t, a); !slices.Equal(got, []string{"ivory", "ivory-add"}) {
		t.Fatalf("встало: %v", got)
	}
}

func TestDownloadAddonPullsDonor(t *testing.T) {
	a := withAddon(t)
	res, err := a.DownloadLibraryPack("ivory-add")
	if err != nil || !res.Picked || res.Added != 2 {
		t.Fatalf("дополнение: %v %+v", err, res)
	}
	if got := standing(t, a); !slices.Equal(got, []string{"ivory", "ivory-add"}) {
		t.Fatalf("донор не пришёл: %v", got)
	}
}

// Удалённое вами дополнение «Скачать» донора не возвращает и пометку с него не снимает.
func TestDownloadDonorKeepsRemovedAddon(t *testing.T) {
	a := withAddon(t)
	if _, err := a.DownloadLibraryPack("ivory"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RemovePack("ivory-add"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RemovePack("ivory"); err != nil {
		t.Fatal(err)
	}
	if skip := skipOf(a); len(skip) != 2 {
		t.Fatalf("пропуск: %v", skip)
	}
	res, err := a.DownloadLibraryPack("ivory")
	if err != nil || !res.Picked || res.Added != 1 {
		t.Fatalf("донор: %v %+v", err, res)
	}
	if got := standing(t, a); !slices.Equal(got, []string{"ivory"}) {
		t.Fatalf("дополнение вернулось: %v", got)
	}
	if skip := skipOf(a); !slices.Equal(skip, []string{"ivory-add"}) {
		t.Fatalf("пометки: %v — у донора снята, у дополнения остаётся", skip)
	}
	// Донор стоит, дополнение удалено: «Скачать» донора ничего не возвращает.
	if again, err := a.DownloadLibraryPack("ivory"); err != nil || again.Picked || again.Added != 0 {
		t.Fatalf("повтор: %v %+v", err, again)
	}
}

// Нажатие на удалённый набор снимает пометку «удалён вами» — после удачной установки; чужие
// пометки остаются.
func TestDownloadClearsSkip(t *testing.T) {
	a, l := netApp(t, true)
	l.publish(t, nil, fourZips(t)...)
	if _, err := getAll(t, a); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RemovePack("ruby"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RemovePack("plum"); err != nil {
		t.Fatal(err)
	}
	res, err := a.DownloadLibraryPack("ruby")
	if err != nil || !res.Picked {
		t.Fatalf("скачать: %v %+v", err, res)
	}
	if skip := skipOf(a); !slices.Equal(skip, []string{"plum"}) {
		t.Fatalf("пометки: %v", skip)
	}
}

// Неудача пометку не стирает: битый архив, нет связи.
func TestDownloadFailKeepsSkip(t *testing.T) {
	a, l := netApp(t, true)
	zip := d1Zip(t)
	l.publish(t, nil, zip)
	if _, err := getAll(t, a); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RemovePack("ivory"); err != nil {
		t.Fatal(err)
	}
	l.publish(t, func(r map[string]any) { r["sha256"] = strings.Repeat("0", 64) }, zip)
	res, err := a.DownloadLibraryPack("ivory")
	if err != nil || res.Picked || res.Why != "скачался с ошибкой, повторите" || len(res.Failed) != 1 || !slices.Equal(skipOf(a), []string{"ivory"}) {
		t.Fatalf("битый: %v %+v %v", err, res, skipOf(a))
	}
	// Нет связи: у одного набора ошибка — причиной в итоге, не ошибкой вызова.
	l.publish(t, nil, zip)
	l.setMode(map[string]string{"ivory": "drop"})
	res, err = a.DownloadLibraryPack("ivory")
	if err != nil || res.Picked || res.Why != fetch.ErrOffline.Error() || !slices.Equal(skipOf(a), []string{"ivory"}) {
		t.Fatalf("нет связи: %v %+v %v", err, res, skipOf(a))
	}
	l.setMode(nil)
	noDownloadLeft(t, a)
}

// Набор исчез из свежего списка: ErrGone, пометка остаётся, список переписан; id не по
// грамматике — ErrGone до сети.
func TestDownloadGoneKeepsSkip(t *testing.T) {
	a, l := netApp(t, true)
	zip := d1Zip(t)
	sage := rezip(t, zip, func(m map[string]any) { m["id"], m["title"] = "sage", "Шалфей" })
	l.publish(t, nil, zip, sage)
	if _, err := getAll(t, a); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RemovePack("ivory"); err != nil {
		t.Fatal(err)
	}
	l.publish(t, nil, sage) // ivory исчез из свежего списка
	hits, arcs := l.hits.Load(), l.archives.Load()
	if _, err := a.DownloadLibraryPack("ivory"); !errors.Is(err, libpacks.ErrGone) || l.archives.Load() != arcs {
		t.Fatalf("нет в списке: %v", err)
	}
	if l.hits.Load() != hits+1 || !slices.Equal(skipOf(a), []string{"ivory"}) {
		t.Errorf("запросов %d (нужен один каталог), пометки %v", l.hits.Load()-hits, skipOf(a))
	}
	if list, _ := a.LibraryPacks(); len(list.Available) != 0 {
		t.Errorf("раздел: %+v", list.Available)
	}
	hits = l.hits.Load()
	for _, id := range []string{"", "../x", "ЕСТЬ", strings.Repeat("a", 500)} {
		if _, err := a.DownloadLibraryPack(id); !errors.Is(err, libpacks.ErrGone) {
			t.Errorf("%q: %v", id, err)
		}
	}
	if l.hits.Load() != hits {
		t.Error("запросы с недопустимым id")
	}
}

// Без согласия, при занятой службе и в режиме «только чтение» — ни одного запроса.
func TestDownloadNoConsentBusyReadOnly(t *testing.T) {
	a, l := netApp(t, false)
	l.publish(t, nil, d1Zip(t))
	if _, err := a.DownloadLibraryPack("ivory"); !errors.Is(err, libpacks.ErrNoConsent) {
		t.Fatalf("без согласия: %v", err)
	}
	if _, err := a.DownloadAllLibrary(); !errors.Is(err, libpacks.ErrNoConsent) {
		t.Fatalf("все без согласия: %v", err)
	}
	if _, err := a.SetLibraryNet(true, false); err != nil {
		t.Fatal(err)
	}
	release, ok := a.libp.Hold()
	if !ok {
		t.Fatal("занято с начала")
	}
	if _, err := a.DownloadLibraryPack("ivory"); !errors.Is(err, libpacks.ErrBusy) {
		t.Fatalf("занято: %v", err)
	}
	if _, err := a.DownloadAllLibrary(); !errors.Is(err, libpacks.ErrBusy) {
		t.Fatalf("все, занято: %v", err)
	}
	release()
	if err := os.MkdirAll(libDir(a), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := filepath.Join(libDir(a), "library.json")
	if err := os.WriteFile(reg, []byte(`{"version":99,"items":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	a.openDecor()
	if _, err := a.DownloadLibraryPack("ivory"); !errors.Is(err, decor.ErrReadOnly) {
		t.Fatalf("только чтение: %v", err)
	}
	if _, err := a.DownloadAllLibrary(); !errors.Is(err, decor.ErrReadOnly) {
		t.Fatalf("все, только чтение: %v", err)
	}
	if l.hits.Load() != 0 {
		t.Fatalf("запросов: %d", l.hits.Load())
	}
}

// Архив без единого годного предмета: набор не встал, причина — нажатому («нечего ставить»).
func TestDownloadOneBrokenZip(t *testing.T) {
	a, l := netApp(t, true)
	l.publish(t, nil, garbleZip(t, d1Zip(t)))
	warm(t, a)
	res, err := a.DownloadLibraryPack("ivory")
	if err != nil || res.Picked || res.Added != 0 || len(res.Failed) != 1 || !strings.HasPrefix(res.Why, decor.ErrPackEmpty.Error()) {
		t.Fatalf("нечего ставить: %v %+v", err, res)
	}
	if len(standing(t, a)) != 0 {
		t.Error("набор поставлен")
	}
	noDownloadLeft(t, a)
}

// «Скачать все» ставит ровно строки, что видел пользователь (список до операции): набор,
// появившийся в каталоге после, не ставится, а считается новым (fresh); «удалён вами» не входит.
func TestDownloadAllShownOnly(t *testing.T) {
	a, l := netApp(t, true)
	zips := fourZips(t)
	l.publish(t, nil, zips[:3]...)
	warm(t, a)
	l.publish(t, nil, zips...) // plum появился после того, как список был показан
	res, err := a.DownloadAllLibrary()
	if err != nil || res.Added != 3 || res.Fresh != 1 || res.Available != 1 || l.archives.Load() != 3 {
		t.Fatalf("все: %v %+v, архивов %d", err, res, l.archives.Load())
	}
	if got := standing(t, a); slices.Contains(got, "plum") {
		t.Fatalf("новое поставлено: %v", got)
	}
	// Удалённый набор повторно не ставится.
	if _, err := a.RemovePack("ruby"); err != nil {
		t.Fatal(err)
	}
	res, err = a.DownloadAllLibrary()
	if err != nil || res.Added != 1 || res.Available != 0 || !slices.Equal(skipOf(a), []string{"ruby"}) {
		t.Fatalf("после удаления: %v %+v", err, res)
	}
	// Без сохранённого списка ставить нечего (но ход — не ошибка).
	b := fresh(t, l)
	if res, err := b.DownloadAllLibrary(); err != nil || res.Added != 0 || res.Fresh != 4 {
		t.Fatalf("без списка: %v %+v", err, res)
	}
}

// «Обновить» ставит только стоящее: новая версия стоящего, новый набор — нет.
func TestUpdateOnlyInstalled(t *testing.T) {
	a, l := netApp(t, true)
	src := d1Zip(t)
	l.publish(t, nil, src)
	if _, err := getAll(t, a); err != nil {
		t.Fatal(err)
	}
	v2 := rezip(t, src, func(m map[string]any) { m["version"] = 2 }, before2File(t, src))
	sage := rezip(t, src, func(m map[string]any) { m["id"], m["title"] = "sage", "Шалфей" })
	l.publish(t, nil, v2, sage)
	arc := l.archives.Load()
	res, err := a.UpdateLibrary()
	if err != nil || res.Updated != 1 || res.Added != 0 || l.archives.Load() != arc+1 || res.Available != 1 || res.Fresh != 1 {
		t.Fatalf("обновить: %v %+v, архивов %d", err, res, l.archives.Load()-arc)
	}
}

// Одно из двух обновлений не скачалось — второе встало, ход не оборван.
func TestNetUpdateContinuesAfterFailure(t *testing.T) {
	a, l := netApp(t, true)
	zip := d1Zip(t)
	sage := rezip(t, zip, func(m map[string]any) { m["id"], m["title"] = "sage", "Шалфей" })
	l.publish(t, nil, zip, sage)
	if _, err := getAll(t, a); err != nil {
		t.Fatal(err)
	}
	v2 := rezip(t, zip, func(m map[string]any) { m["version"] = 2 }, before2File(t, zip))
	v2s := rezip(t, sage, func(m map[string]any) { m["version"] = 2 }, before2File(t, sage))
	l.publish(t, nil, v2, v2s)
	l.setMode(map[string]string{"ivory": "404"})
	res, err := a.UpdateLibrary()
	if err != nil || res.Updated != 1 || len(res.Failed) != 1 || res.Failed[0].Name != "Свадьба — айвори" {
		t.Fatalf("итог: %v %+v", err, res)
	}
}

// Предел времени набора рвёт весь ход: следующий набор упрётся так же и не запрашивается.
func TestNetPackLimitStops(t *testing.T) {
	a, l := netApp(t, true)
	rebuildPacks(t, a, func(h *libpacks.Host) {
		h.Fetcher = func() *fetch.Client {
			c := fetch.New(l.src)
			c.SetTimeouts(time.Minute, 150*time.Millisecond) // застой далеко, пол предела набора узкий
			return c
		}
	})
	l.publish(t, nil, fourZips(t)...)
	l.setMode(map[string]string{"ivory": "hang"})
	warm(t, a)
	res, err := a.DownloadAllLibrary()
	if !errors.Is(err, fetch.ErrSlow) || !strings.Contains(err.Error(), "Связь слишком медленная") || l.archives.Load() != 1 || res.Added != 0 {
		t.Fatalf("предел: %v %+v, архивов %d", err, res, l.archives.Load())
	}
	if prefs.LibraryNetOf(a.cfgDir).Checked != 0 {
		t.Error("checked записан при обрыве")
	}
}

// Отмена во время установки: набор доустанавливается, следующий не начинается.
func TestCancelDuringInstall(t *testing.T) {
	a, l := netApp(t, true)
	l.publish(t, nil, fourZips(t)...)
	warm(t, a)
	cancelled := false
	a.emit = func(name string, data ...any) {
		if name != "library:progress" || len(data) == 0 {
			return
		}
		if p, ok := data[0].(libpacks.Progress); ok && p.Stage == "install" && !cancelled {
			cancelled = true
			a.CancelLibrary()
		}
	}
	res, err := a.DownloadAllLibrary()
	if !errors.Is(err, fetch.ErrCancelled) || res.Added != 1 || l.archives.Load() != 1 {
		t.Fatalf("отмена: %v %+v, архивов %d", err, res, l.archives.Load())
	}
	if got := standing(t, a); len(got) != 1 {
		t.Fatalf("первый не доставлен: %v", got)
	}
}

// Проверка при запуске: одно событие library:start — обновления и новое вместе.
func TestStartCheckOneEvent(t *testing.T) {
	defer setStart(0, 20*time.Hour)()
	a, l := netApp(t, true)
	ev := captureEvents(a)
	src := d1Zip(t)
	l.publish(t, nil, src)
	if _, err := getAll(t, a); err != nil {
		t.Fatal(err)
	}
	v2 := rezip(t, src, func(m map[string]any) { m["version"] = 2 }, before2File(t, src))
	sage := rezip(t, src, func(m map[string]any) { m["id"], m["title"] = "sage", "Шалфей" })
	l.publish(t, nil, v2, sage)
	a.libp.StartCheck()
	starts := ev.of("library:start")
	if len(starts) != 1 {
		t.Fatalf("событий %d", len(starts))
	}
	if st := starts[0].(libpacks.Start); st.Updated != 1 || st.Fresh.Count != 1 || st.Fresh.Bytes != fileSize(t, sage) || st.Offer.Count != 0 {
		t.Errorf("событие: %+v", st)
	}
	// Повтор: список уже новый — ничего не объявляется.
	a.libp.StartCheck()
	if len(ev.of("library:start")) != 1 {
		t.Error("новое объявлено дважды")
	}
}

// Неудача отдельного набора при запуске — только в log.txt: события нет.
func TestStartCheckFailedSilent(t *testing.T) {
	defer setStart(0, 20*time.Hour)()
	a, l := netApp(t, true)
	ev := captureEvents(a)
	src := d1Zip(t)
	l.publish(t, nil, src)
	if _, err := getAll(t, a); err != nil {
		t.Fatal(err)
	}
	v2 := rezip(t, src, func(m map[string]any) { m["version"] = 2 }, before2File(t, src))
	l.publish(t, func(r map[string]any) { r["sha256"] = strings.Repeat("0", 64) }, v2)
	a.libp.StartCheck()
	if n := len(ev.of("library:start")) + len(ev.of("notice")); n != 0 {
		t.Fatalf("неудача дала события: %d", n)
	}
	if prefs.LibraryNetOf(a.cfgDir).Checked == 0 {
		t.Error("поштучная неудача проверку закрывает")
	}
}

// Сохранённый список не читается (битый файл): новое не объявляется — иначе все наборы были бы
// «новыми»; после проверки список снова читается.
func TestStartCheckBrokenCacheNoAnnounce(t *testing.T) {
	defer setStart(0, 20*time.Hour)()
	a, l := netApp(t, true)
	ev := captureEvents(a)
	l.publish(t, nil, fourZips(t)...)
	if err := os.WriteFile(filepath.Join(a.cfgDir, "library-catalog.json"), []byte("{не json"), 0o600); err != nil {
		t.Fatal(err)
	}
	a.libp.StartCheck()
	if n := len(ev.of("library:start")); n != 0 {
		t.Fatalf("битый список объявил новое: %d", n)
	}
	if list, _ := a.LibraryPacks(); len(list.Available) != 4 {
		t.Fatalf("список после проверки: %+v", list.Available)
	}
}
