package libpacks

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Пример в компактной редакции (спека 142): строка «Скачать» в List без каталога и сети, докачка
// по кнопке (opDemo) и внутри проверки при запуске.

func (e *demoEnv) traffic() int32 { return e.srv.catHits.Load() + e.srv.assetHits.Load() }

// «Скачать» в строке примера: без каталога, адрес — Prefix + Asset, чужие байты (тот же размер,
// другой sha256), 404 и ответ 500 — отказ с причиной, пример не встаёт, мусора в библиотеке нет.
func TestDownloadDemoPinned(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	e.publishDemoAsset()
	res, err := e.svc.Download(demoPackID)
	if err != nil || !res.Picked || res.Added != 1 || res.Bytes != int64(len(e.demoZip)) {
		t.Fatalf("Download: %v %+v", err, res)
	}
	e.mustStand(demoPackID, decor.OriginDemo)
	e.wantZipBytes(demoPackID, int64(len(e.demoZip))) // 137-31: пример из сети — размер zip, как в «Можно скачать»
	if e.srv.catHits.Load() != 0 || !slices.Equal(e.srv.asked(), []string{demoAsset1}) {
		t.Errorf("каталог %d, запрошено %v", e.srv.catHits.Load(), e.srv.asked())
	}
	if got := e.ev.of("library:changed"); len(got) != 1 || len(got[0].(Changed).Installed) != 1 {
		t.Errorf("library:changed: %+v", got)
	}
	if list, _ := e.svc.List(); len(list.Available) != 0 || list.Net.DemoOffer {
		t.Errorf("строка осталась: %+v", list.Available)
	}
	if _, err := e.svc.Download(demoPackID); err != ErrGone { // поставлен — строки нет
		t.Errorf("повторное Download: %v", err)
	}

	// Подмена ассета: тот же размер, другой байт.
	bad := slices.Clone(e.demoZip)
	bad[len(bad)/2] ^= 0xff
	for name, set := range map[string]func(e *demoEnv){
		"чужие байты": func(e *demoEnv) { e.srv.setAsset(demoAsset1, bad) },
		"обрезан":     func(e *demoEnv) { e.srv.setAsset(demoAsset1, e.demoZip[:len(e.demoZip)-5]) },
		"404":         func(e *demoEnv) { e.srv.setStatus(demoAsset1, 404) },
		"500":         func(e *demoEnv) { e.srv.setStatus(demoAsset1, 500) },
	} {
		e2 := newDemoEnv(t, false, nil)
		set(e2)
		res, err := e2.svc.Download(demoPackID)
		e2.mustNotStand(demoPackID)
		if res.Picked || res.Why == "" || (err == nil && len(res.Failed) != 1) || e2.svc.Busy() {
			t.Errorf("%s: %v %+v", name, err, res)
		}
		if list, _ := e2.svc.List(); !slices.Equal(e2.rowIDs(list), []string{demoPackID}) { // строка осталась
			t.Errorf("%s: строки %v", name, e2.rowIDs(list))
		}
		if left, _ := filepath.Glob(filepath.Join(e2.dir, decor.RootName, "*", ".dl-*")); len(left) != 0 {
			t.Errorf("%s: остатки скачивания %v", name, left)
		}
	}
	// Пример в пропуске («удалён вами») без «Вернуть» не качается.
	e3 := newDemoEnv(t, false, nil)
	e3.publishDemoAsset()
	prefs.UpdateLibraryNet(e3.dir, func(n *prefs.LibraryNet) { n.SetSkip(demoPackID, true) })
	if _, err := e3.svc.Download(demoPackID); err != ErrGone || e3.traffic() != 0 {
		t.Errorf("в пропуске: %v, запросов %d", err, e3.traffic())
	}
}

// Адрес ассета не из библиотеки (пустой префикс источника, чужой хост) — строки и докачки нет.
func TestDemoAssetRejected(t *testing.T) {
	for name, src := range map[string]fetch.Source{
		"пустой префикс": {},
		"чужой хост":     {Prefix: "https://evil.example/dl/", Hosts: []string{"github.com"}},
	} {
		e := newDemoEnv(t, false, func(h *Host) { h.Fetcher = func() *fetch.Client { return fetch.New(src) } })
		list, err := e.svc.List()
		if err != nil || len(list.Available) != 0 || list.Net.DemoOffer {
			t.Errorf("%s: строка есть: %v %+v", name, err, list)
		}
		if _, err := e.svc.Download(demoPackID); err != ErrGone {
			t.Errorf("%s: Download: %v", name, err)
		}
		e.svc.StartCheck()
		e.mustNotStand(demoPackID)
		if e.traffic() != 0 || !strings.Contains(e.log.String(), "адрес ассета не прошёл проверку") {
			t.Errorf("%s: запросов %d, log %q", name, e.traffic(), e.log.String())
		}
	}
}

// Строка примера — первой, скрыта, когда есть файл, пример, пропуск или любой донор.
func TestAvailDemoRow(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	e.publishChain()
	if _, err := e.svc.Update(); err != nil {
		t.Fatal(err)
	}
	list, err := e.svc.List()
	if err != nil || !slices.Equal(e.rowIDs(list), []string{demoPackID, docMain}) {
		t.Fatalf("строки: %v %v", err, e.rowIDs(list))
	}
	a := list.Available[0]
	if a.Title != "Свадьба — синяя акварель (пример)" || a.About != "О примере" || a.License != "CC0-1.0" || a.Version != 1 ||
		a.Bg != 4 || a.Sticker != 7 || a.Spread != 4 || a.Frame != 2 || a.Bytes != int64(len(e.demoZip)) ||
		a.AddonTitles == nil || len(a.AddonTitles) != 0 || a.Donor != "" || a.Why != "" || a.Removed {
		t.Errorf("поля строки: %+v", a)
	}
	if !list.Net.DemoOffer || list.All.Count != 1 {
		t.Errorf("Net %+v, «Скачать все» %+v", list.Net, list.All)
	}

	hidden := map[string]func(e *demoEnv){
		"есть файл":    func(e *demoEnv) { e.hasFile.Store(true) },
		"пример стоит": func(e *demoEnv) { e.addFile(e.demoZip, "demo") },
		"пропуск": func(e *demoEnv) {
			prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.SetSkip(demoPackID, true) })
		},
		"донор стоит": func(e *demoEnv) { e.addFile(e.layoutsZip, "layouts") },
		"донор в пропуске": func(e *demoEnv) {
			prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.SetSkip(docFrames, true) })
		},
	}
	for name, hide := range hidden {
		e := newDemoEnv(t, false, nil)
		hide(e)
		list, _ := e.svc.List()
		if slices.Contains(e.rowIDs(list), demoPackID) || list.Net.DemoOffer {
			t.Errorf("%s: строка примера осталась: %v", name, e.rowIDs(list))
		}
	}
	// Без примера в программе (SA_DEMO=off, Host.Demo пуст) строки нет и сеть не трогается.
	off := newDemoEnv(t, false, func(h *Host) { h.Demo = func() Demo { return Demo{} } })
	if list, _ := off.svc.List(); len(list.Available) != 0 || list.Net.DemoOffer {
		t.Errorf("пример выключен, а строка есть: %+v", list.Available)
	}
}

// Строка есть до первого обращения к сети: списка на диске нет, согласия нет, запросов нет.
func TestAvailDemoRowNoCatalog(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.Allowed = false })
	list, err := e.svc.List()
	if err != nil || !slices.Equal(e.rowIDs(list), []string{demoPackID}) || list.AvailAt != 0 || list.All.Count != 0 || !list.Net.DemoOffer {
		t.Fatalf("список: %v %+v", err, list)
	}
	if e.traffic() != 0 {
		t.Errorf("запросов в сеть: %d", e.traffic())
	}
	if list.Available == nil || len(list.Packs) == 0 {
		t.Errorf("пустые срезы: %+v", list)
	}
}

// Пример не «новый набор»: ни «Обновить», ни проверка при запуске не считают его в Fresh, и он не
// объявляется заново на следующей проверке.
func TestFreshIgnoresDemo(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	e.publishChain()
	for i := range 2 {
		res, err := e.svc.Update()
		want := 1 - i // main — новый в первую проверку, дальше он уже в списке
		if err != nil || res.Fresh != want {
			t.Fatalf("Update %d: %v Fresh %d, ждали %d", i, err, res.Fresh, want)
		}
		if res.Available != 1 { // «Скачать все» — main (frames и layouts — его дополнения)
			t.Errorf("Update %d: Available %d", i, res.Available)
		}
	}
	e2 := newDemoEnv(t, false, nil)
	e2.publishChain()
	e2.publishDemoAsset()
	e2.svc.StartCheck()
	got := e2.ev.of("library:start")
	if len(got) != 1 || got[0].(Start).Fresh.Count != 1 || got[0].(Start).Updated != 0 {
		t.Errorf("library:start: %+v", got)
	}
	if strings.Contains(e2.log.String(), "ушла из списка") {
		t.Errorf("log: %q", e2.log.String())
	}
}

// «Скачать все» и его число не знают примера: строка в списке есть, а в плане и подписи — нет.
func TestAllCountIgnoresDemo(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	e.publishChain()
	e.publishDemoAsset()
	if _, err := e.svc.Update(); err != nil {
		t.Fatal(err)
	}
	list, _ := e.svc.List()
	if len(list.Available) != 2 || list.All.Count != 1 {
		t.Fatalf("строк %d, «Скачать все» %+v", len(list.Available), list.All)
	}
	var want int64
	for _, z := range [][]byte{e.mainZip, e.framesZip, e.layoutsZip} {
		want += int64(len(z))
	}
	if list.All.Bytes != want {
		t.Errorf("байты «Скачать все»: %d, ждали %d (без примера)", list.All.Bytes, want)
	}
	res, err := e.svc.DownloadAll()
	if err != nil || res.Added != 3 {
		t.Fatalf("DownloadAll: %v %+v", err, res)
	}
	if slices.Contains(e.srv.asked(), demoAsset1) || strings.Contains(e.log.String(), "ушла из списка") {
		t.Errorf("запрошено %v, log %q", e.srv.asked(), e.log.String())
	}
	e.mustNotStand(demoPackID)
}

// Компактная с согласием и «Проверять при запуске»: пример докачивается в той же проверке, без
// тоста; ошибка сети — log.txt, строка «Скачать» остаётся; checked пишется в обоих случаях.
func TestStartCheckDownloadsDemo(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	e.publishChain()
	e.publishDemoAsset()
	e.svc.StartCheck()
	e.mustStand(demoPackID, decor.OriginDemo)
	e.wantZipBytes(demoPackID, int64(len(e.demoZip))) // то же при докачке в проверке запуска (startDemo)
	if got := e.ev.of("library:changed"); len(got) != 1 || len(got[0].(Changed).Installed) != 0 {
		t.Errorf("library:changed (без названий): %+v", got)
	}
	if got := e.ev.of("library:start"); len(got) != 1 || got[0].(Start).Fresh.Count != 1 || got[0].(Start).Updated != 0 {
		t.Errorf("library:start: %+v", got) // только каталог: пример в «новых» не считается
	}
	if prefs.LibraryNetOf(e.dir).Checked == 0 || e.svc.Busy() || e.relinks.Load() != 1 {
		t.Errorf("checked %d, busy %v, relink %d", prefs.LibraryNetOf(e.dir).Checked, e.svc.Busy(), e.relinks.Load())
	}
	if n := slices.Index(e.srv.asked(), demoAsset1); n < 0 {
		t.Errorf("ассет не запрошен: %v", e.srv.asked())
	}

	// Сбой ассета: проверка прошла, пример не встал, причина в log.txt, строка остаётся.
	f := newDemoEnv(t, false, nil)
	f.publishChain()
	f.srv.setStatus(demoAsset1, 500)
	f.svc.StartCheck()
	f.mustNotStand(demoPackID)
	list, _ := f.svc.List()
	if !strings.Contains(f.log.String(), "не поставлен") || !slices.Contains(f.rowIDs(list), demoPackID) ||
		prefs.LibraryNetOf(f.dir).Checked == 0 || len(f.ev.of("library:changed")) != 0 {
		t.Errorf("сбой: log %q, строки %v", f.log.String(), f.rowIDs(list))
	}

	// Каталог не получен (нет сети) — пример не пробуется: нет второго запроса.
	g := newDemoEnv(t, false, nil)
	g.publishDemoAsset()
	g.svc.StartCheck() // каталог не опубликован: подпись пустая, разбор не проходит
	g.mustNotStand(demoPackID)
	if slices.Contains(g.srv.asked(), demoAsset1) {
		t.Error("ассет качался после неудачной проверки")
	}

	// Есть файл (стандартная) — StartCheck сам из сети не качает; пример удалён вами — тоже.
	for name, setup := range map[string]func(*demoEnv){
		"файл": func(e *demoEnv) { e.hasFile.Store(true) },
		"пропуск": func(e *demoEnv) {
			prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.SetSkip(demoPackID, true) })
		},
	} {
		h := newDemoEnv(t, false, nil)
		h.publishChain()
		h.publishDemoAsset()
		setup(h)
		h.svc.StartCheck()
		if slices.Contains(h.srv.asked(), demoAsset1) {
			t.Errorf("%s: ассет качался", name)
		}
	}
}

// Одна проверка при запуске и обновляет наборы, и ставит пример: замок не потерян, «пропущена» в
// log.txt нет; PlaceDemo без файла рядом замка не берёт.
func TestStartCheckNotLostToDemo(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	const extra = "extra-pack"
	e.srv.publish(row{id: extra, version: 1, zip: packZip(t, extra, 1, 40)})
	if res, err := e.svc.Download(extra); err != nil || !res.Picked {
		t.Fatalf("первая версия: %v %+v", err, res)
	}
	e.srv.publish(row{id: extra, version: 2, zip: packZip(t, extra, 2, 40, 41)},
		row{id: docMain, version: 1, zip: e.mainZip})
	e.publishDemoAsset()
	done := make(chan struct{})
	go func() { e.svc.PlaceDemo(t.Context()); close(done) }() // без файла — выход без замка
	e.svc.StartCheck()
	<-done
	got := e.ev.of("library:start")
	if len(got) != 1 || got[0].(Start).Updated != 1 || got[0].(Start).Fresh.Count != 1 {
		t.Errorf("library:start: %+v", got)
	}
	e.mustStand(demoPackID, decor.OriginDemo)
	for _, p := range e.st.Packs() {
		if p.ID == extra && p.Version != 2 {
			t.Errorf("набор не обновлён: %+v", p)
		}
	}
	if strings.Contains(e.log.String(), "пропущена") {
		t.Errorf("проверка пропущена: %q", e.log.String())
	}
}

// Строка каталога с id примера не ставится и не вытесняет стоящий пример: причина в итоге.
func TestNetPlanDemoID(t *testing.T) {
	e := newDemoEnv(t, true, nil)
	e.place()
	e.srv.publish(row{id: demoPackID, version: 1, zip: e.demoZip})
	res, err := e.svc.Update()
	if err != nil || res.Added+res.Updated != 0 || !slices.ContainsFunc(res.Skipped, func(k Skip) bool { return k.Why == whyDemo }) {
		t.Fatalf("Update: %v %+v", err, res)
	}
	e.mustStand(demoPackID, decor.OriginDemo)
	if slices.Contains(e.srv.asked(), relOf(demoPackID, 1)) {
		t.Error("архив строки с id примера скачан")
	}
}

// «Скачать» основной набор тянет -frames и -layouts: пример уходит в том же замке, файлы целы; без
// дополнения (в пропуске) пример остаётся.
func TestDemoRetiredByPick(t *testing.T) {
	e := newDemoEnv(t, true, nil)
	e.place()
	e.publishChain()
	if _, err := e.svc.Update(); err != nil {
		t.Fatal(err)
	}
	res, err := e.svc.Download(docMain)
	if err != nil || res.Added != 3 || !res.Picked {
		t.Fatalf("Download: %v %+v", err, res)
	}
	e.mustNotStand(demoPackID)
	if len(e.skip()) != 0 {
		t.Errorf("пропуск: %v", e.skip())
	}
	for _, w := range []int{21, 22, 23, 24} {
		if _, ok := e.st.Find(keyOf(pngW(t, w))); !ok {
			t.Errorf("предмет %d потерян", w)
		}
	}
	// Дополнение в пропуске: встали не все — пример остаётся.
	f := newDemoEnv(t, true, nil)
	f.place()
	f.publishChain()
	prefs.UpdateLibraryNet(f.dir, func(n *prefs.LibraryNet) { n.SetSkip(docLayouts, true) })
	if _, err := f.svc.Download(docMain); err != nil {
		t.Fatal(err)
	}
	f.mustStand(docMain, decor.OriginNet)
	f.mustNotStand(docLayouts)
	f.mustStand(demoPackID, decor.OriginDemo)
}
