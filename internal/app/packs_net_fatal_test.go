package app

import (
	"errors"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/libpacks"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Обрыв хода при запуске (429 на втором стоящем наборе) не глотает объявление: сохранённый список
// уже переписан, и новый набор при следующем запуске «новым» не был бы. Событие несёт и уже
// вставшее обновление, и новое; ошибка — только в log.txt.
func TestStartCheckFatalStillAnnounces(t *testing.T) {
	defer setStart(0, 20*time.Hour)()
	a, l := netApp(t, true)
	ev := captureEvents(a)
	zips := fourZips(t) // порядок наборов — как в fourZips
	l.publish(t, nil, zips[0], zips[1])
	if res, err := getAll(t, a); err != nil || res.Added != 2 { // ivory и sage стоят, список сохранён
		t.Fatalf("исходное: %v %+v", err, res)
	}
	v2 := func(src string) string {
		return rezip(t, src, func(m map[string]any) { m["version"] = 2 }, before2File(t, src))
	}
	l.seq = 2
	l.publish(t, nil, v2(zips[0]), v2(zips[1]), zips[2]) // первые два — обновления, третий — новый
	l.setMode(map[string]string{"sage": "429"})
	prefs.UpdateLibraryNet(a.cfgDir, func(n *prefs.LibraryNet) { n.Checked = 0 })
	a.libp.StartCheck()
	starts := ev.of("library:start")
	if len(starts) != 1 {
		t.Fatalf("событий library:start %d, ждали 1 при обрыве хода", len(starts))
	}
	if st := starts[0].(libpacks.Start); st.Updated != 1 || st.Fresh.Count != 1 || st.Fresh.Bytes <= 0 || st.Offer.Count != 0 {
		t.Errorf("событие: %+v, ждали обновлён 1 и новых 1", st)
	}
	if prefs.LibraryNetOf(a.cfgDir).Checked != 0 {
		t.Error("checked записан при обрыве")
	}
	if len(ev.of("notice")) != 0 {
		t.Error("обрыв при запуске дал тост об ошибке")
	}
}

// Строка каталога с id-именем устройства Windows (con) между двумя годными: fetch пропускает её
// по грамматике, а dropDevices убирает до плана. Её нет в разделе «Можно скачать» и в «не
// скачались», годные ставятся, ход не рвётся.
func TestNetDeviceIDNotFatal(t *testing.T) {
	a, l := netApp(t, true)
	zips := fourZips(t) // порядок наборов — как в fourZips
	l.publish(t, func(r map[string]any) {
		if r["id"] == "ruby" {
			r["id"] = "con"
		}
	}, zips[0], zips[2], zips[1]) // годный, con, годный
	res, err := getAll(t, a)
	if err != nil || res.Added != 2 || len(res.Failed) != 0 || len(res.Skipped) != 0 {
		t.Fatalf("итог: %v %+v", err, res)
	}
	list, err := a.LibraryPacks()
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, p := range list.Packs {
		have[p.ID] = true
	}
	if !have["ivory"] || !have["sage"] || have["con"] {
		t.Errorf("установлены: %+v", list.Packs)
	}
	for _, r := range list.Available {
		if r.ID == "con" {
			t.Errorf("con в разделе «Можно скачать»: %+v", list.Available)
		}
	}
	if list.All.Count != 0 || res.Available != 0 {
		t.Errorf("«Скачать все» ждёт %d (итог %d), ждали 0", list.All.Count, res.Available)
	}
}

// Отмена во время установки, когда набор не встал по своей причине (архив не читается как
// набор): ход отменён (ErrCancelled), а не оборван ошибкой набора; причина остаётся в «не
// скачались». Установка набора доводится до конца, следующий не начинается.
func TestNetCancelDuringFailingInstall(t *testing.T) {
	a, l := netApp(t, true)
	zips := fourZips(t)
	l.publish(t, nil, garbleZip(t, zips[0]), zips[1])
	if _, err := a.UpdateLibrary(); err != nil { // список сохранён, ничего не стоит
		t.Fatal(err)
	}
	a.emit = func(name string, data ...any) {
		if p, ok := data[0].(libpacks.Progress); name == "library:progress" && ok && p.Stage == "install" {
			a.CancelLibrary()
		}
	}
	res, err := a.DownloadAllLibrary()
	if !errors.Is(err, fetch.ErrCancelled) {
		t.Fatalf("ждали ErrCancelled, получили %v", err)
	}
	if len(res.Failed) != 1 || res.Failed[0].Name != "Свадьба — айвори" || res.Failed[0].Why == "" || res.Added != 0 || l.archives.Load() != 1 {
		t.Errorf("итог: %+v, архивов %d", res, l.archives.Load())
	}
}
