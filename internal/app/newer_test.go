package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Текст «файл новее программы» (098 design §6): с версией из поля app — major.minor обеих
// программ; без неё или с не-версией — прежний смысл; не ErrNewer — текст ошибки как есть.
func TestNewerText(t *testing.T) {
	newer := func(app string) error { return &salbum.NewerError{Version: model.FormatVersion + 1, App: app} }
	cases := []struct {
		err        error
		what, mine string
		want       string
	}{
		{newer("0.25.0"), "альбом", "0.23.0", "альбом сохранён в версии 0.25. Эта программа — 0.23: обновите её"},
		{newer("0.25.1-rc2+abc"), "Черновик", "0.23.1", "Черновик сохранён в версии 0.25. Эта программа — 0.23: обновите её"},
		{newer("1.0"), "альбом", "dev", "альбом сохранён в версии 1.0: обновите программу"},
		{newer("0.25"), "альбом", "", "альбом сохранён в версии 0.25: обновите программу"},
		{newer(""), "альбом", "0.23.0", "альбом сохранён более новой версией программы"},
		{newer("dev"), "альбом", "0.23.0", "альбом сохранён более новой версией программы"},
		{newer("0."), "альбом", "0.23.0", "альбом сохранён более новой версией программы"},
		{fmt.Errorf("чтение: %w", salbum.ErrNewer), "Черновик", "0.23.0", "Черновик сохранён более новой версией программы"},
		{fmt.Errorf("обёртка: %w", newer("0.30.2")), "альбом", "0.23.0", "альбом сохранён в версии 0.30. Эта программа — 0.23: обновите её"},
		{errors.New("файл проекта повреждён"), "альбом", "0.23.0", "файл проекта повреждён"},
	}
	for _, c := range cases {
		if got := newerTextFor(c.err, c.what, c.mine); got != c.want {
			t.Errorf("%v (%q, %q):\n%q\nждали\n%q", c.err, c.what, c.mine, got, c.want)
		}
	}
	if err := openErr(newer("0.25.0")); !strings.HasPrefix(err.Error(), "альбом сохранён в версии 0.25") {
		t.Errorf("openErr: %v", err)
	}
	if plain := errors.New("x"); openErr(plain) != plain {
		t.Error("openErr меняет чужую ошибку")
	}
	// Открытие файла новее программы через openPath — текст с версией.
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	path := filepath.Join(testkit.TempDir(t), "a"+projectExt)
	if err := os.WriteFile(path, fmt.Appendf(nil, `{"version":%d,"app":"0.25.0"}`, model.FormatVersion+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.openPath(path); err == nil || !strings.Contains(err.Error(), "в версии 0.25") {
		t.Fatalf("openPath: %v", err)
	}
	if _, _, notice := a.loadStartup(path); !strings.Contains(notice, "Не открылся проект: альбом сохранён в версии 0.25") {
		t.Fatalf("старт: %q", notice)
	}
}

// Тост о копии старого формата: первая запись поверх v16 (SaveProject и автосохранение) — одно
// уведомление с именем копии и версией 0.22; повтор — без тоста; в тексте только имя, без папки.
func TestBackupNotice(t *testing.T) {
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	var mu sync.Mutex
	var notes []string
	a.emit = func(name string, data ...any) {
		if name == "notice" {
			mu.Lock()
			notes = append(notes, data[0].(string))
			mu.Unlock()
		}
	}
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "Свадьба"+projectExt)
	if err := os.WriteFile(path, []byte(`{"version":16,"spreads":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	a.ed.Store(mustEditor(t, p, path))
	if _, err := a.SaveProject(); err != nil {
		t.Fatal(err)
	}
	want := "Сохранена копия для версии 0.22: «Свадьба (для 0.22)" + projectExt + "»"
	if len(notes) != 1 || notes[0] != want {
		t.Fatalf("уведомления %q, ждали %q", notes, want)
	}
	if _, err := os.Stat(filepath.Join(dir, "Свадьба (для 0.22)"+projectExt)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveProject(); err != nil {
		t.Fatal(err)
	}
	a.flushSave()
	if len(notes) != 1 {
		t.Fatalf("повтор: %q", notes)
	}
	// Черновик старой версии — автосохранение: тост один раз.
	notes = nil
	if err := os.WriteFile(a.draftPath(), []byte(`{"version":15,"spreads":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(a.draftPath())
	if err != nil {
		t.Fatal(err)
	}
	a.ed.Store(mustEditor(t, q, ""))
	a.flushSave()
	a.flushSave()
	if want := "Сохранена копия для версии 0.21: «черновик (для 0.21)" + projectExt + "»"; len(notes) != 1 || notes[0] != want {
		t.Fatalf("черновик: %q, ждали %q", notes, want)
	}
	// Имя без хвоста версии — общий текст; пустое — без тоста.
	if got := backupText(filepath.Join(dir, "x.salbum")); got != "Сохранена копия для прежней версии: «x.salbum»" {
		t.Errorf("без версии: %q", got)
	}
}
