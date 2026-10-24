package app

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/update"
)

// logTo — собрать лог теста; по завершении лог возвращается в stderr.
func logTo(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

// Тихая проверка при запуске не затирает итог ручной: скачанное (ready) остаётся, в сеть она
// не идёт и тоста не шлёт; «Перезапустить» после неё работает.
func TestSilentCheckKeepsPhase(t *testing.T) {
	r := newUpdApp(t)
	a := r.a
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	if _, err := a.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	if info, err := a.DownloadUpdate(); err != nil || info.Phase != "ready" {
		t.Fatalf("скачивание: %v %+v", err, info)
	}
	hits, states := r.f.hits.Load(), len(r.ev.of("update:state"))
	a.checkUpdate(true)
	if r.f.hits.Load() != hits || len(r.ev.of("update:offer")) != 0 || len(r.ev.of("update:state")) != states {
		t.Errorf("тихая проверка поверх ready: запросов %d→%d, offer %d", hits, r.f.hits.Load(), len(r.ev.of("update:offer")))
	}
	if info := a.UpdateInfo(); info.Phase != "ready" || info.Version != "0.25.1" {
		t.Fatalf("фаза затёрта: %+v", info)
	}
	if err := a.InstallUpdate(); err != nil || len(r.starts) != 1 {
		t.Errorf("установка после тихой проверки: %v, запусков %d", err, len(r.starts))
	}
	// Любая другая фаза тоже не трогается; пустая (idle) — проверка идёт.
	for _, ph := range []string{"error", "available", "latest", "checking"} {
		q := newUpdApp(t)
		q.f.publish(t, "0.25.1", update.MinSetup, nil)
		q.a.upd.phase = ph
		q.a.checkUpdate(true)
		if q.f.hits.Load() != 0 || q.a.upd.phase != ph {
			t.Errorf("фаза %s: запросов %d, фаза %q", ph, q.f.hits.Load(), q.a.upd.phase)
		}
	}
	q := newUpdApp(t)
	q.f.publish(t, "0.25.1", update.MinSetup, nil)
	q.a.checkUpdate(true)
	if q.f.hits.Load() == 0 || len(q.ev.of("update:offer")) != 1 {
		t.Errorf("пустая фаза: запросов %d, offer %d", q.f.hits.Load(), len(q.ev.of("update:offer")))
	}
}

// Стенд QA: версии ленты не пишутся в config.json (Seen, Checked) и не сверяются с виденным
// ранее — предложение приходит даже при Seen новее; настоящий источник пишет как раньше.
func TestUpdateStandNotSaved(t *testing.T) {
	r := newUpdApp(t)
	r.f.src.LocalHTTP = true
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	prefs.UpdateUpdate(r.a.cfgDir, func(u *prefs.Update) { u.Seen = "0.26.0" })
	buf := logTo(t)
	info, err := r.a.CheckUpdate()
	if err != nil || info.Phase != "available" || info.Version != "0.25.1" {
		t.Fatalf("стенд: %v %+v", err, info)
	}
	if u := prefs.UpdateOf(r.a.cfgDir); u.Seen != "0.26.0" || u.Checked != 0 || info.Checked != 0 {
		t.Errorf("config стенда: %+v", u)
	}
	if !strings.Contains(buf.String(), "источник — стенд") {
		t.Errorf("лог: %q", buf.String())
	}
	// Чистый config: стенд не создаёт ключ update совсем.
	q := newUpdApp(t)
	q.f.src.LocalHTTP = true
	q.f.publish(t, "0.25.1", update.MinSetup, nil)
	if _, err := q.a.CheckUpdate(); err != nil || prefs.UpdateOf(q.a.cfgDir) != (prefs.Update{}) {
		t.Errorf("пустой config: %v %+v", err, prefs.UpdateOf(q.a.cfgDir))
	}
	// Контроль: не стенд пишет Seen и Checked.
	p := newUpdApp(t)
	p.f.publish(t, "0.25.1", update.MinSetup, nil)
	if _, err := p.a.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	if u := prefs.UpdateOf(p.a.cfgDir); u.Seen != "0.25.1" || u.Checked == 0 {
		t.Errorf("config выпуска: %+v", u)
	}
}

// В лог — причина, пользователю — прежний текст: подпись и sha256 повторной проверки.
func TestUpdateLogDetail(t *testing.T) {
	buf := logTo(t)
	r := newUpdApp(t)
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	r.f.mu.Lock()
	r.f.env = bytes.Replace(r.f.env, []byte(`"sig":"`), []byte(`"sig":"AAAA`), 1)
	r.f.mu.Unlock()
	info, _ := r.a.CheckUpdate()
	if info.Phase != "error" || info.Error != update.ErrVerify.Error() {
		t.Fatalf("пользователю: %+v", info)
	}
	if !strings.Contains(buf.String(), "проверка не удалась: "+update.ErrVerify.Error()+" (") ||
		!strings.Contains(buf.String(), "(подпись не сошлась)") {
		t.Errorf("лог проверки без причины: %q", buf.String())
	}

	// Повторная проверка: файл подменён — в логе, что не сошёлся sha256.
	buf.Reset()
	g := newUpdApp(t)
	g.f.publish(t, "0.25.1", update.MinSetup, nil)
	if _, err := g.a.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	if _, err := g.a.DownloadUpdate(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(g.a.upd.dir, update.SetupName("0.25.1"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw[0] ^= 0xff
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := g.a.InstallUpdate(); !errors.Is(err, update.ErrVerify) || err.Error() != update.ErrVerify.Error() {
		t.Fatalf("пользователю: %v", err)
	}
	if !strings.Contains(buf.String(), "sha256 не совпал") {
		t.Errorf("лог повторной проверки: %q", buf.String())
	}
}

// Отказ запуска установщика и сборки командной строки: в логе причина, пути и имени папки нет.
func TestInstallLogNoPath(t *testing.T) {
	buf := logTo(t)
	r := newUpdApp(t)
	r.f.publish(t, "0.25.1", update.MinSetup, nil)
	r.a.upd.start = func(string, string) error { return &os.PathError{Op: "fork", Path: r.a.upd.dir, Err: os.ErrPermission} }
	if _, err := r.a.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.a.DownloadUpdate(); err != nil {
		t.Fatal(err)
	}
	if err := r.a.InstallUpdate(); !errors.Is(err, errUpdStart) {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "установщик не запустился: fork: ") || strings.Contains(out, r.a.upd.dir) {
		t.Errorf("лог запуска: %q", out)
	}

	// Путь с кавычкой: CmdLine отказывает (ErrPath), в логе слова без пути.
	buf.Reset()
	q := newUpdApp(t)
	q.f.publish(t, "0.25.1", update.MinSetup, nil)
	q.a.upd.dir = filepath.Join(testkit.TempDir(t), `Up"dates`)
	if _, err := q.a.CheckUpdate(); err != nil {
		t.Fatal(err)
	}
	if _, err := q.a.DownloadUpdate(); err != nil {
		t.Fatal(err)
	}
	if err := q.a.InstallUpdate(); !errors.Is(err, errUpdStart) || len(q.starts) != 0 {
		t.Fatalf("кавычка в пути: %v, запусков %d", err, len(q.starts))
	}
	out = buf.String()
	if !strings.Contains(out, update.ErrPath.Error()) || strings.Contains(out, `Up"dates`) {
		t.Errorf("лог командной строки: %q", out)
	}
}

// Разбор аргументов запуска идёт до setupLog: строки копятся и уходят в лог только в flushLaunchLog,
// до этого в log (stderr GUI-exe) не пишется ничего.
func TestInitLaunchDefersLog(t *testing.T) {
	dir := testkit.TempDir(t)
	good := filepath.Join(dir, "a"+projectExt)
	cases := []struct {
		name string
		args []string
		wait bool
		want []string // подстроки строк лога; nil — лога нет
		file string
	}{
		{"прежняя версия не закрылась", []string{afterPIDFlag, "4242"}, false, []string{"прежняя версия не закрылась"}, ""},
		{"аргумент отклонён", []string{"//srv/share/a" + projectExt}, true, []string{"аргумент запуска отклонён"}, ""},
		{"обе строки", []string{afterPIDFlag, "7", "//srv/share/a" + projectExt}, false,
			[]string{"аргумент запуска отклонён", "прежняя версия не закрылась"}, ""},
		{"всё хорошо — лога нет", []string{afterPIDFlag, "7", good}, true, nil, good},
		{"нет аргументов", nil, true, nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf := logTo(t)
			a := newApp(t)
			a.initLaunchFrom(c.args, "", func(int, time.Duration) bool { return c.wait })
			if buf.Len() != 0 {
				t.Fatalf("до открытия лога записано: %q", buf.String())
			}
			if a.argFile != c.file || len(a.launchLog) != len(c.want) {
				t.Fatalf("файл %q, накоплено %q", a.argFile, a.launchLog)
			}
			a.flushLaunchLog()
			for _, w := range c.want {
				if !strings.Contains(buf.String(), w) {
					t.Errorf("в логе нет %q: %q", w, buf.String())
				}
			}
			if c.want == nil && buf.Len() != 0 {
				t.Errorf("лишнее в логе: %q", buf.String())
			}
			// Второй сброс ничего не пишет.
			buf.Reset()
			a.flushLaunchLog()
			if buf.Len() != 0 || a.launchLog != nil {
				t.Errorf("повторный сброс: %q", buf.String())
			}
		})
	}
	// Отказанный путь в лог идёт без пути.
	buf := logTo(t)
	a := newApp(t)
	a.initLaunchFrom([]string{"//srv/share/секрет" + projectExt}, "", func(int, time.Duration) bool { return true })
	a.flushLaunchLog()
	if strings.Contains(buf.String(), "секрет") {
		t.Errorf("путь в логе: %q", buf.String())
	}
}
