package libpacks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Пример из файла у программы (спека 142, design §4): без сети, под замком, тихо.

func setLockPoll(t *testing.T, d time.Duration) {
	t.Helper()
	old := lockPoll
	lockPoll = d
	t.Cleanup(func() { lockPoll = old })
}

// Файл есть: пример встаёт без сети и вопроса, origin demo, одно library:changed без названий,
// пережил круг запись → чтение реестра; строки «Скачать» больше нет.
func TestPlaceDemoFromFile(t *testing.T) {
	e := newDemoEnv(t, true, nil)
	e.svc.PlaceDemo(t.Context())
	e.mustStand(demoPackID, decor.OriginDemo)
	fz, _ := e.fileZip.Load().([]byte)
	e.wantZipBytes(demoPackID, int64(len(fz))) // 137-31: подпись не зависит от пути установки
	if got := e.ev.of("library:changed"); len(got) != 1 || len(got[0].(Changed).Installed) != 0 {
		t.Errorf("library:changed: %+v", got)
	}
	if e.relinks.Load() != 1 || e.srv.catHits.Load()+e.srv.assetHits.Load() != 0 {
		t.Errorf("relink %d, запросов в сеть %d", e.relinks.Load(), e.srv.catHits.Load()+e.srv.assetHits.Load())
	}
	list, err := e.svc.List()
	if err != nil || len(list.Available) != 0 || list.Net.DemoOffer || e.svc.Busy() {
		t.Fatalf("список: %v %+v", err, list)
	}
	again := decor.Open(e.dir) // перезапуск программы
	for _, p := range again.Packs() {
		if p.ID == demoPackID && (p.Origin != decor.OriginDemo || p.Sticker != 4 || !p.Removable || p.Bytes != int64(len(fz))) {
			t.Errorf("после чтения реестра: %+v", p)
		}
	}
	// Повтор (следующий запуск): стоит — файл не читается заново, событий нет.
	reads := e.reads.Load()
	e.svc.PlaceDemo(t.Context())
	if e.reads.Load() != reads || len(e.ev.of("library:changed")) != 1 {
		t.Errorf("повторная установка: чтений %d→%d, событий %d", reads, e.reads.Load(), len(e.ev.of("library:changed")))
	}
}

// Нет файла и нет согласия: ни замка, ни чтения, ни одного запроса; «Скачать» без согласия —
// ErrNoConsent до сети.
func TestPlaceDemoNoFileNoConsent(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.Allowed = false })
	release, ok := e.svc.Hold() // замок занят: PlaceDemo без файла его не ждёт
	if !ok {
		t.Fatal("замок занят")
	}
	done := make(chan struct{})
	go func() { e.svc.PlaceDemo(t.Context()); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("PlaceDemo без файла ждёт замок")
	}
	release()
	if _, err := e.svc.Download(demoPackID); err != ErrNoConsent {
		t.Errorf("Download без согласия: %v", err)
	}
	if e.reads.Load() != 0 || e.srv.catHits.Load()+e.srv.assetHits.Load() != 0 || len(e.ev.of("library:changed")) != 0 {
		t.Errorf("чтений %d, запросов %d, событий %d", e.reads.Load(), e.srv.catHits.Load()+e.srv.assetHits.Load(), len(e.ev.of("library:changed")))
	}
	e.mustNotStand(demoPackID)
}

// Пример или любой донор «удалён вами» — не ставится и файл не читается.
func TestPlaceDemoSkipped(t *testing.T) {
	for _, id := range []string{demoPackID, docMain, docFrames, docLayouts} {
		e := newDemoEnv(t, true, nil)
		prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.SetSkip(id, true) })
		e.svc.PlaceDemo(t.Context())
		e.mustNotStand(demoPackID)
		if e.reads.Load() != 0 {
			t.Errorf("%s в пропуске, а файл читался", id)
		}
	}
}

// Стоит любой один из доноров — пример не ставится (0.30-142-22).
func TestPlaceDemoAnyRelated(t *testing.T) {
	zips := map[string]func(e *demoEnv) []byte{
		docMain:    func(e *demoEnv) []byte { return e.mainZip },
		docFrames:  func(e *demoEnv) []byte { return e.framesZip },
		docLayouts: func(e *demoEnv) []byte { return e.layoutsZip },
	}
	for id, zb := range zips {
		e := newDemoEnv(t, true, nil)
		e.addFile(zb(e), id)
		e.svc.PlaceDemo(t.Context())
		e.mustNotStand(demoPackID)
		if e.reads.Load() != 0 {
			t.Errorf("стоит %s, а файл читался", id)
		}
	}
}

// Библиотека только для чтения (реестр новее программы): не ставится, в log.txt причина без пути.
func TestPlaceDemoReadOnly(t *testing.T) {
	e := newDemoEnv(t, true, nil)
	root := filepath.Join(e.dir, decor.RootName)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "library.json"), []byte(`{"version":99,"items":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	e = newDemoEnvOver(t, e.dir)
	if !e.st.ReadOnly() {
		t.Fatal("библиотека не стала только для чтения")
	}
	e.svc.PlaceDemo(t.Context())
	e.mustNotStand(demoPackID)
	if out := e.log.String(); !strings.Contains(out, "только для чтения") || strings.Contains(out, e.dir) {
		t.Errorf("log: %q", out)
	}
	// «Вернуть» на такой библиотеке: пропуск снят, пример не встал, ответ без паники.
	prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.SetSkip(demoPackID, true) })
	if n, err := e.svc.ResetSkip(); err != nil || n.DemoBack || len(e.skip()) != 0 {
		t.Errorf("ResetSkip на чтении: %v %+v", err, n)
	}
}

// newDemoEnvOver — служба над уже заготовленной папкой настроек dir (реестр правлен руками).
func newDemoEnvOver(t *testing.T, dir string) *demoEnv {
	t.Helper()
	e := newDemoEnv(t, true, func(h *Host) {
		st := decor.Open(dir)
		h.CfgDir = func() string { return dir }
		h.Store = func() (*decor.Store, error) { return st, nil }
	})
	return e
}

// Библиотека заполнена: пример не ставится, ошибка квоты в log.txt словами, паники нет.
func TestPlaceDemoLibFull(t *testing.T) {
	e := newDemoEnv(t, true, nil)
	root := filepath.Join(e.dir, decor.RootName)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	var items []string
	for i := range decor.MaxLibraryLen/decor.MaxBgBytes + 1 {
		items = append(items, fmt.Sprintf(`{"key":"my:%016x","kind":"bg","theme":"mine","name":"x","ext":"png","w":1,"h":1,"bytes":%d}`, i+1, decor.MaxBgBytes))
	}
	reg := `{"version":1,"items":[` + strings.Join(items, ",") + `]}`
	if err := os.WriteFile(filepath.Join(root, "library.json"), []byte(reg), 0o600); err != nil {
		t.Fatal(err)
	}
	e = newDemoEnvOver(t, e.dir)
	if e.st.Used() <= decor.MaxLibraryLen-int64(len(e.demoZip)) {
		t.Fatalf("библиотека не заполнена: %d", e.st.Used())
	}
	e.svc.PlaceDemo(t.Context())
	e.mustNotStand(demoPackID)
	if out := e.log.String(); !strings.Contains(out, "библиотека заполнена") {
		t.Errorf("log: %q", out)
	}
	if e.svc.Busy() {
		t.Error("замок не отпущен")
	}
}

// Фон ждёт замок службы (TryLock с повтором), а не падает с «занято»; выход программы снимает ожидание.
func TestPlaceDemoWaitsLock(t *testing.T) {
	setLockPoll(t, 5*time.Millisecond)
	e := newDemoEnv(t, true, nil)
	release, ok := e.svc.Hold()
	if !ok {
		t.Fatal("замок занят")
	}
	done := make(chan struct{})
	go func() { e.svc.PlaceDemo(t.Context()); close(done) }()
	time.Sleep(60 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("PlaceDemo не дождался замка")
	default:
	}
	e.mustNotStand(demoPackID)
	release()
	<-done
	e.mustStand(demoPackID, decor.OriginDemo)

	// Выход программы (StopCtx) во время ожидания: ничего не ставится, горутина выходит.
	e2 := newDemoEnv(t, true, nil)
	release2, _ := e2.svc.Hold()
	defer release2()
	ctx, cancel := context.WithCancel(t.Context())
	done2 := make(chan struct{})
	go func() { e2.svc.PlaceDemo(ctx); close(done2) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-done2:
	case <-time.After(2 * time.Second):
		t.Fatal("ожидание замка не снялось при выходе")
	}
	e2.mustNotStand(demoPackID)
}

// Все проверки — уже под замком: донор, поставленный другой операцией, пока фон ждал, пример
// не пускает (проверка до ожидания замка этого не увидела бы).
func TestPlaceDemoChecksUnderLock(t *testing.T) {
	setLockPoll(t, 5*time.Millisecond)
	e := newDemoEnv(t, true, nil)
	release, _ := e.svc.Hold()
	done := make(chan struct{})
	go func() { e.svc.PlaceDemo(t.Context()); close(done) }()
	time.Sleep(20 * time.Millisecond)
	// «Другая операция» ставит донора в своём замке.
	arcSG := stageZip(t, e, e.mainZip, decor.OriginNet)
	if _, err := e.st.InstallPack(arcSG, nil); err != nil {
		t.Fatal(err)
	}
	arcSG.Close()
	release()
	<-done
	e.mustNotStand(demoPackID)
	if e.reads.Load() != 0 {
		t.Error("файл прочитан после появления донора")
	}
}

// Пример старшей версии в программе заменяет стоящий младший тихо; той же и новее — нет.
func TestPlaceDemoUpgrade(t *testing.T) {
	e := newDemoEnv(t, true, nil)
	e.place()
	reads := e.reads.Load()
	e.svc.PlaceDemo(t.Context()) // та же версия
	if e.reads.Load() != reads {
		t.Error("та же версия прочитана заново")
	}
	e.version = 2
	e.fileZip.Store(packZip(t, demoPackID, 2, 21, 22, 23, 24, 25))
	e.svc.PlaceDemo(t.Context())
	for _, p := range e.st.Packs() {
		if p.ID == demoPackID && (p.Version != 2 || p.Sticker != 5 || p.Origin != decor.OriginDemo) {
			t.Errorf("после обновления: %+v", p)
		}
	}
	e.version = 1 // файл старше стоящего: ничего
	e.svc.PlaceDemo(t.Context())
	for _, p := range e.st.Packs() {
		if p.ID == demoPackID && p.Version != 2 {
			t.Errorf("понижение: %+v", p)
		}
	}
}

// Битые, пустые, усечённые и чужие байты файла — отказ без установки и без паники.
func TestPlaceDemoBadBytes(t *testing.T) {
	e0 := newDemoEnv(t, true, nil)
	good := e0.demoZip
	cases := map[string][]byte{
		"пусто":            {},
		"мусор":            []byte("это не zip"),
		"усечённый":        good[:len(good)/2],
		"только заголовок": good[:4],
		"другой набор":     packZip(t, docMain, 1, 21, 22),
		"другая версия":    packZip(t, demoPackID, 7, 21),
	}
	for name, z := range cases {
		e := newDemoEnv(t, true, nil)
		e.fileZip.Store(z)
		e.svc.PlaceDemo(t.Context())
		e.mustNotStand(demoPackID)
		if e.svc.Busy() {
			t.Errorf("%s: замок не отпущен", name)
		}
	}
	// Read отказал (подмена файла): пример не встаёт, строки «Скачать» при Has == false нет проверки здесь.
	e := newDemoEnv(t, true, nil)
	e.fileZip.Store([]byte(nil))
	e.svc.PlaceDemo(t.Context())
	e.mustNotStand(demoPackID)
}

// Пропуск и донор не мешают «Remove» уже стоящего примера: удаление пишет пропуск, список его
// показывает как «удалён вами» и не предлагает «Скачать».
func TestRemoveDemoSkips(t *testing.T) {
	e := newDemoEnv(t, true, nil)
	e.place()
	list, err := e.svc.Remove(demoPackID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(e.skip(), []string{demoPackID}) || !list.Net.DemoSkipped || list.Net.Skipped != 1 || len(list.Available) != 0 {
		t.Errorf("после удаления: skip %v, %+v", e.skip(), list)
	}
	e.mustNotStand(demoPackID)
	e.svc.PlaceDemo(t.Context()) // следующий запуск: пропуск держит
	e.mustNotStand(demoPackID)

	// Пример, поставленный «из файла» (откат на 0.29 и обратно), — тоже в пропуск.
	e2 := newDemoEnv(t, false, nil)
	e2.addFile(e2.demoZip, "demo")
	e2.mustStand(demoPackID, decor.OriginFile)
	if _, err := e2.svc.Remove(demoPackID); err != nil || !slices.Equal(e2.skip(), []string{demoPackID}) {
		t.Errorf("пример из файла: %v %v", err, e2.skip())
	}
	// Обычный набор из файла в пропуск не попадает, как раньше.
	e2.addFile(e2.mainZip, "main")
	if _, err := e2.svc.Remove(docMain); err != nil || slices.Contains(e2.skip(), docMain) {
		t.Errorf("набор из файла: %v %v", err, e2.skip())
	}
}

// «Вернуть» при примере в пропуске: пропуск снят, пример встал сразу из файла, ответ demoBack,
// событие окну.
func TestResetSkipPlacesDemo(t *testing.T) {
	e := newDemoEnv(t, true, nil)
	e.place()
	if _, err := e.svc.Remove(demoPackID); err != nil {
		t.Fatal(err)
	}
	prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.SetSkip("school-autumn", true) }) // и чужой пропуск
	changed := len(e.ev.of("library:changed"))
	n, err := e.svc.ResetSkip()
	if err != nil || !n.DemoBack || n.Skipped != 0 || n.DemoSkipped || len(e.skip()) != 0 {
		t.Fatalf("ResetSkip: %v %+v skip %v", err, n, e.skip())
	}
	e.mustStand(demoPackID, decor.OriginDemo)
	if len(e.ev.of("library:changed")) != changed+1 || e.svc.Busy() {
		t.Errorf("событий %d→%d, busy %v", changed, len(e.ev.of("library:changed")), e.svc.Busy())
	}
	// Без пропуска примера DemoBack не бывает.
	prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.SetSkip("other-pack", true) })
	if n, err := e.svc.ResetSkip(); err != nil || n.DemoBack || n.Skipped != 0 {
		t.Errorf("чужой пропуск: %v %+v", err, n)
	}
	if n, err := e.svc.ResetSkip(); err != nil || n.DemoBack { // пусто — config.json не переписывается
		t.Errorf("пустой пропуск: %v %+v", err, n)
	}
}

// «Вернуть» без файла: пропуск снят, пример не встал, строка «Скачать» снова видна; сеть не тронута.
func TestResetSkipNoFile(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.SetSkip(demoPackID, true); n.SetSkip("other-pack", true) })
	if list, _ := e.svc.List(); len(list.Available) != 0 || !list.Net.DemoSkipped || list.Net.Skipped != 2 {
		t.Fatalf("до: %+v", list)
	}
	n, err := e.svc.ResetSkip()
	if err != nil || n.DemoBack || n.Skipped != 0 || !n.DemoOffer {
		t.Fatalf("ResetSkip: %v %+v", err, n)
	}
	list, _ := e.svc.List()
	if !slices.Equal(e.rowIDs(list), []string{demoPackID}) || e.srv.catHits.Load()+e.srv.assetHits.Load() != 0 {
		t.Errorf("после: %v, запросов %d", e.rowIDs(list), e.srv.catHits.Load()+e.srv.assetHits.Load())
	}
	e.mustNotStand(demoPackID)
}

// «Вернуть» при занятой службе — ErrBusy, пропуск не тронут, пример не встал.
func TestResetSkipBusy(t *testing.T) {
	e := newDemoEnv(t, true, nil)
	prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.SetSkip(demoPackID, true) })
	release, _ := e.svc.Hold()
	n, err := e.svc.ResetSkip()
	release()
	if err != ErrBusy || n.Skipped != 1 || !n.DemoSkipped || !slices.Equal(e.skip(), []string{demoPackID}) {
		t.Fatalf("занято: %v %+v skip %v", err, n, e.skip())
	}
	e.mustNotStand(demoPackID)
	if n, err := e.svc.ResetSkip(); err != nil || !n.DemoBack { // освободилось — встаёт
		t.Errorf("после освобождения: %v %+v", err, n)
	}
}

// Пропуск примера не считается, пока стоят все три донора (0.30-142-28); запись остаётся.
func TestDemoSkipDroppedWhenCovered(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	prefs.UpdateLibraryNet(e.dir, func(n *prefs.LibraryNet) { n.SetSkip(demoPackID, true) })
	e.addFile(e.mainZip, "main")
	e.addFile(e.framesZip, "frames")
	list, _ := e.svc.List()
	if !list.Net.DemoSkipped || list.Net.Skipped != 1 {
		t.Fatalf("два донора из трёх: %+v", list.Net)
	}
	e.addFile(e.layoutsZip, "layouts")
	list, _ = e.svc.List()
	if list.Net.DemoSkipped || list.Net.Skipped != 0 || !slices.Contains(e.skip(), demoPackID) {
		t.Errorf("три донора: %+v skip %v", list.Net, e.skip())
	}
	if _, err := e.svc.Remove(docLayouts); err != nil { // донор ушёл — пропуск снова считается
		t.Fatal(err)
	}
	if list, _ = e.svc.List(); !list.Net.DemoSkipped || list.Net.Skipped != 1 {
		t.Errorf("после ухода донора: %+v", list.Net)
	}
}

// Пример уходит, когда стоят доноры, покрывающие все его ключи; предметы целы, пропуск не пишется.
func TestDemoRetiredByFull(t *testing.T) {
	e := newDemoEnv(t, true, nil)
	e.place()
	e.addFile(e.mainZip, "main") // двух из трёх мало: s23 держат рамки, s24 — макеты
	e.addFile(e.framesZip, "frames")
	e.mustStand(demoPackID, decor.OriginDemo)
	e.addFile(e.layoutsZip, "layouts") // теперь s21…s24 есть у доноров
	e.mustNotStand(demoPackID)
	for _, w := range []int{21, 22, 23, 24} {
		if _, ok := e.st.Find(keyOf(pngW(t, w))); !ok {
			t.Errorf("предмет %d потерян при уходе примера", w)
		}
	}
	if len(e.skip()) != 0 || strings.Contains(e.log.String(), "не убран") {
		t.Errorf("пропуск %v, log %q", e.skip(), e.log.String())
	}
	e.svc.PlaceDemo(t.Context()) // на следующем запуске не возвращается: доноры стоят
	e.mustNotStand(demoPackID)
}

// Пример, поставленный «из файла» (origin file), узнаётся по id и уходит так же.
func TestDemoRetiredByID(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	e.addFile(e.demoZip, "demo")
	e.mustStand(demoPackID, decor.OriginFile)
	e.addFile(e.mainZip, "main")
	e.addFile(e.framesZip, "frames")
	e.mustStand(demoPackID, decor.OriginFile)
	e.addFile(e.layoutsZip, "layouts")
	e.mustNotStand(demoPackID)
}

// Донор не покрывает все ключи примера (другая версия донора без ключа) — пример остаётся.
func TestDemoNotRetiredUncovered(t *testing.T) {
	e := newDemoEnv(t, false, nil)
	e.addFile(packZip(t, demoPackID, 1, 21, 22, 23, 24, 26), "demo") // s26 нет ни у одного донора
	e.addFile(e.mainZip, "main")
	e.addFile(e.framesZip, "frames")
	e.addFile(e.layoutsZip, "layouts")
	e.mustStand(demoPackID, decor.OriginFile)
	if len(e.skip()) != 0 {
		t.Errorf("пропуск: %v", e.skip())
	}
}

// stageZip — Staged из байтов архива (чужая «операция» ставит донора мимо службы).
func stageZip(t *testing.T, e *demoEnv, z []byte, origin string) *decor.Staged {
	t.Helper()
	p := filepath.Join(e.dir, "stage.zip")
	if err := os.WriteFile(p, z, 0o600); err != nil {
		t.Fatal(err)
	}
	arc, err := pack.Open(p, e.svc.h.Version())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = arc.Close() })
	sg, err := e.st.StagePack(decorManifest(arc.Manifest), arc, origin, arc.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	return sg
}
