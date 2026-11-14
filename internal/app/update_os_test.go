package app

import (
	"bytes"
	"errors"
	"log"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/app/updater"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/update"
)

// Правило минимальной системы (171): служба проверяет выпуск на системе тест-машины (runtime.GOOS:
// стенд-подмена в обычной сборке выключена) с версией из r.osver.

// osRules — поля манифеста: выпуску нужна система need, ступени — lastOn («порог → версия»).
func osRules(need string, last map[string]string) map[string]any {
	m := map[string]any{"minOS": map[string]any{runtime.GOOS: need}}
	if last != nil {
		m["lastOn"] = map[string]any{runtime.GOOS: last}
	}
	return m
}

func osVer(v string) func() (string, bool) { return func() (string, bool) { return v, v != "" } }

func openedURLs(t *testing.T) *[]string {
	t.Helper()
	var opened []string
	old := openURL
	t.Cleanup(func() { openURL = old })
	openURL = func(_ *App, u string) { opened = append(opened, u) }
	return &opened
}

// Система ниже minOS: фаза latest с blocked/osNeed, скачивать и ставить нечего, страница — установленной
// версии (в том числе после ошибки проверки и при dev-версии — список выпусков); система не ниже —
// обычное предложение (тест падает без правила).
func TestUpdateBlockedOS(t *testing.T) {
	opened := openedURLs(t)
	r := newUpdApp(t)
	a := r.a
	r.f.publish(t, "0.26.0", update.MinSetup, osRules("13.0", nil))
	r.osver = osVer("12.4")
	info, err := a.CheckUpdate()
	if err != nil || info.Phase != "latest" || info.Version != "" || info.Blocked != "0.26.0" || info.OSNeed != "13.0" ||
		info.OS != runtime.GOOS || info.Last != "" {
		t.Fatalf("блокировка: %v %+v", err, info)
	}
	if info, _ := a.DownloadUpdate(); info.Phase != "latest" || r.f.setup.Load() != 0 {
		t.Errorf("Download при блокировке: %+v, запросов установщика %d", info, r.f.setup.Load())
	}
	if err := a.InstallUpdate(); !errors.Is(err, updater.ErrNotReady) {
		t.Errorf("Install при блокировке: %v", err)
	}
	if err := a.OpenUpdatePage(); err != nil {
		t.Fatal(err)
	}
	// Ошибка следующей проверки блокировку не стирает: адрес прежний.
	r.f.mu.Lock()
	r.f.env = nil
	r.f.mu.Unlock()
	if info, _ := a.CheckUpdate(); info.Phase != "error" || info.Blocked != "0.26.0" {
		t.Fatalf("после ошибки: %+v", info)
	}
	if err := a.OpenUpdatePage(); err != nil {
		t.Fatal(err)
	}
	// Версия программы не версия (dev): страницы «своей» нет — список выпусков.
	r.version = "dev"
	if err := a.OpenUpdatePage(); err != nil {
		t.Fatal(err)
	}
	cur := appkit.RepoURL + "/releases/tag/v0.25.0"
	if want := []string{cur, cur, appkit.RepoURL + "/releases"}; strings.Join(*opened, " ") != strings.Join(want, " ") {
		t.Errorf("адреса %v, ждали %v", *opened, want)
	}
	// Система подходит (равна порогу) — обычное предложение, blocked пуст.
	r.version, r.osver = "0.25.0", osVer("13.0")
	r.f.publish(t, "0.26.0", update.MinSetup, osRules("13.0", nil))
	if info, _ := a.CheckUpdate(); info.Phase != "available" || info.Version != "0.26.0" || info.Blocked != "" || info.OSNeed != "" {
		t.Errorf("система подходит: %+v", info)
	}
}

// Откат (старый выпуск подан повторно при seen новее) о системе ничего не говорит: блокировка сеанса
// остаётся, «Страница выпуска» по-прежнему ведёт на установленную версию, а не на /releases/latest.
func TestUpdateBlockedKeptOnRollback(t *testing.T) {
	opened := openedURLs(t)
	r := newUpdApp(t)
	a := r.a
	r.osver = osVer("12.4")
	r.f.publish(t, "0.27.0", update.MinSetup, osRules("13.0", nil))
	if info, _ := a.CheckUpdate(); info.Blocked != "0.27.0" {
		t.Fatalf("блокировка: %+v", info)
	}
	// Лента отдаёт версию старше виденной (0.27.0), но новее установленной (0.25.0) и без minOS: откат.
	r.f.publish(t, "0.26.0", update.MinSetup, nil)
	info, _ := a.CheckUpdate()
	if info.Phase != "latest" || info.Version != "" || info.Blocked != "0.27.0" || info.OSNeed != "13.0" {
		t.Fatalf("после отката блокировка пропала: %+v", info)
	}
	_ = a.OpenUpdatePage()
	if want := appkit.RepoURL + "/releases/tag/v0.25.0"; len(*opened) != 1 || (*opened)[0] != want {
		t.Errorf("адрес после отката %v, ждали %s", *opened, want)
	}
}

// Ступень lastOn: предложение страницей (Install и Download нечего), blocked и osNeed остаются;
// тихая проверка шлёт update:offer, а не update:last, и osTold не пишет.
func TestUpdateBlockedLadder(t *testing.T) {
	opened := openedURLs(t)
	r := newUpdApp(t)
	a := r.a
	r.f.publish(t, "0.28.0", update.MinSetup, osRules("14.0", map[string]string{"14.0": "0.26.0"}))
	r.osver = osVer("13.5")
	a.upd.CheckSilent()
	info := a.UpdateInfo()
	if info.Phase != "available" || info.Version != "0.26.0" || info.Mode != "page" || info.Blocked != "0.28.0" || info.OSNeed != "14.0" {
		t.Fatalf("ступень: %+v", info)
	}
	if len(r.ev.of("update:offer")) != 1 || len(r.ev.of("update:last")) != 0 || prefs.UpdateOf(a.cfgDir).OSTold != "" {
		t.Errorf("события offer %d last %d, osTold %q", len(r.ev.of("update:offer")), len(r.ev.of("update:last")), prefs.UpdateOf(a.cfgDir).OSTold)
	}
	if err := a.InstallUpdate(); !errors.Is(err, updater.ErrNotReady) {
		t.Errorf("Install ступени: %v", err)
	}
	_ = a.OpenUpdatePage()
	if want := appkit.RepoURL + "/releases/tag/v0.26.0"; len(*opened) != 1 || (*opened)[0] != want {
		t.Errorf("адрес ступени %v", *opened)
	}
	// Ступень не новее текущей версии — предложения нет, это блокировка.
	r.version = "0.26.0"
	if info, _ := a.CheckUpdate(); info.Phase != "latest" || info.Version != "" || info.Blocked != "0.28.0" {
		t.Errorf("ступень не новее: %+v", info)
	}
}

// Тост «последняя для вашей системы»: тихая проверка шлёт update:last один раз на версию; osTold
// в config.json держит это между запусками; новая недоступная версия — тост снова; Info().Last — один раз.
func TestUpdateLastOnce(t *testing.T) {
	r := newUpdApp(t)
	a := r.a
	r.osver = osVer("12.4")
	r.f.publish(t, "0.26.0", update.MinSetup, osRules("13.0", nil))
	a.upd.CheckSilent()
	lasts := r.ev.of("update:last")
	if len(lasts) != 1 || lasts[0].(updater.Info).Last != "0.26.0" || lasts[0].(updater.Info).Blocked != "0.26.0" {
		t.Fatalf("update:last: %+v", lasts)
	}
	if prefs.UpdateOf(a.cfgDir).OSTold != "0.26.0" {
		t.Errorf("osTold: %+v", prefs.UpdateOf(a.cfgDir))
	}
	if len(r.ev.of("update:offer")) != 0 {
		t.Error("блокировка шлёт update:offer")
	}
	for _, e := range r.ev.of("update:state") {
		if e.(updater.Info).Last != "" {
			t.Error("update:state несёт last")
		}
	}
	// Опрос при запуске: событие могло прийти раньше интерфейса — один раз.
	if got := a.UpdateInfo().Last; got != "0.26.0" {
		t.Errorf("Info().Last = %q", got)
	}
	if got := a.UpdateInfo().Last; got != "" {
		t.Errorf("Info().Last второй раз = %q", got)
	}
	// Второй запуск той же недоступной версии молчит.
	r.newSession()
	a.upd.CheckSilent()
	if n := len(r.ev.of("update:last")); n != 1 {
		t.Errorf("второй запуск: update:last %d", n)
	}
	// Новая недоступная версия — снова.
	r.f.publish(t, "0.27.0", update.MinSetup, osRules("13.0", nil))
	r.newSession()
	a.upd.CheckSilent()
	lasts = r.ev.of("update:last")
	if len(lasts) != 2 || lasts[1].(updater.Info).Last != "0.27.0" || prefs.UpdateOf(a.cfgDir).OSTold != "0.27.0" {
		t.Errorf("новая версия: %+v", lasts)
	}
}

// Ручная «Проверить обновления» тоста не даёт и osTold не пишет: тост придёт при следующем запуске.
func TestUpdateLastManualFirst(t *testing.T) {
	r := newUpdApp(t)
	a := r.a
	r.osver = osVer("12.4")
	r.f.publish(t, "0.26.0", update.MinSetup, osRules("13.0", nil))
	info, _ := a.CheckUpdate()
	if info.Blocked != "0.26.0" || info.Last != "" || len(r.ev.of("update:last")) != 0 || prefs.UpdateOf(a.cfgDir).OSTold != "" {
		t.Fatalf("ручная: %+v, osTold %q", info, prefs.UpdateOf(a.cfgDir).OSTold)
	}
	// Тихая проверка в том же сеансе после ручной не идёт (фаза уже есть) — тоста нет.
	a.upd.CheckSilent()
	if len(r.ev.of("update:last")) != 0 {
		t.Error("тост в сеансе после ручной проверки")
	}
	r.newSession()
	a.upd.CheckSilent()
	if n := len(r.ev.of("update:last")); n != 1 || a.UpdateInfo().Last != "0.26.0" {
		t.Errorf("следующий запуск: update:last %d", n)
	}
}

// Стенд (источник с LocalHTTP): osTold, Seen и Checked в config.json не пишутся (171-15); тост
// на каждом новом сеансе, пока лента та же.
func TestUpdateLastStand(t *testing.T) {
	r := newUpdApp(t)
	a := r.a
	r.f.src.LocalHTTP = true
	r.osver = osVer("12.4")
	r.f.publish(t, "0.26.0", update.MinSetup, osRules("13.0", nil))
	for i := range 2 {
		r.newSession()
		a.upd.CheckSilent()
		if n := len(r.ev.of("update:last")); n != i+1 {
			t.Fatalf("сеанс %d: update:last %d", i+1, n)
		}
	}
	if u := prefs.UpdateOf(a.cfgDir); u != (prefs.Update{}) {
		t.Errorf("стенд записал config: %+v", u)
	}
}

// Версия ОС не определилась: правило не применяется (обычное предложение), в log.txt — строка
// про minOS; пропущенное поле чужой ОС — в журнале по имени, значения нет; блокировка — строка
// с версией, системой и порогом.
func TestUpdateSystemLog(t *testing.T) {
	r := newUpdApp(t)
	other := "windows"
	if runtime.GOOS == "windows" {
		other = "darwin"
	}
	rules := osRules("13.0", nil)
	rules["minOS"].(map[string]any)[other] = 5 // неверный тип в поле чужой ОС: пропуск
	r.f.publish(t, "0.26.0", update.MinSetup, rules)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	info, _ := r.a.CheckUpdate() // r.osver по умолчанию — не определилась
	if info.Phase != "available" || info.Blocked != "" {
		t.Errorf("версия ОС неизвестна: %+v", info)
	}
	for _, want := range []string{"версия системы не определилась, правило minOS не применяется",
		`пропущены неверные поля другой ОС: ["minOS.` + other + `"]`} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("в журнале нет %q: %s", want, buf.String())
		}
	}
	buf.Reset()
	r.osver = osVer("12.4")
	_, _ = r.a.CheckUpdate()
	if want := "версия 0.26.0 требует " + runtime.GOOS + " ≥ 13.0, система 12.4 — не предлагается"; !strings.Contains(buf.String(), want) {
		t.Errorf("в журнале нет %q: %s", want, buf.String())
	}
	if strings.Contains(buf.String(), "не определилась") {
		t.Errorf("версия известна, а строка «не определилась» есть: %s", buf.String())
	}
}

// «Страница выпуска» без предложения и без проверки в сеансе: osTold новее текущей версии — страница
// установленной, иначе последний выпуск.
func TestOpenUpdatePageOSTold(t *testing.T) {
	opened := openedURLs(t)
	r := newUpdApp(t)
	prefs.UpdateUpdate(r.a.cfgDir, func(u *prefs.Update) { u.OSTold = "0.26.0" })
	_ = r.a.OpenUpdatePage()
	prefs.UpdateUpdate(r.a.cfgDir, func(u *prefs.Update) { u.OSTold = "0.25.0" }) // система выросла, cur догнала
	_ = r.a.OpenUpdatePage()
	want := []string{appkit.RepoURL + "/releases/tag/v0.25.0", appkit.RepoURL + "/releases/latest"}
	if strings.Join(*opened, " ") != strings.Join(want, " ") {
		t.Errorf("адреса %v, ждали %v", *opened, want)
	}
}

// SA_UPDATE_OS, которую не разобрать, — строка в журнале (только в сборке со стендом: в выпуске
// переменная не читается и строки нет); верная подмена строки не даёт.
func TestUpdateStandBadLog(t *testing.T) {
	t.Setenv("SA_UPDATE_URL", "")
	_, auto := update.Default()
	standBuild := !auto // в сборке со стендом без SA_UPDATE_URL проверка при запуске выключена
	r := newUpdApp(t)
	r.f.publish(t, "0.26.0", update.MinSetup, nil)
	for _, c := range []struct {
		env  string
		want bool
	}{{"darwin:вот", true}, {"beos:12.6", true}, {"darwin", true}, {"darwin:12.6", false}, {"", false}} {
		t.Setenv("SA_UPDATE_OS", c.env)
		var buf bytes.Buffer
		log.SetOutput(&buf)
		_, _ = r.a.CheckUpdate()
		log.SetOutput(os.Stderr)
		if got := strings.Contains(buf.String(), "SA_UPDATE_OS не разобрана"); got != (c.want && standBuild) {
			t.Errorf("SA_UPDATE_OS=%q: строка в журнале %v (сборка со стендом %v)", c.env, got, standBuild)
		}
	}
}
