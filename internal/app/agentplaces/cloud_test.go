package agentplaces

// Облака по настройкам клиентов (160 Т3): обёртка CloudRootsLive и место из корня Dropbox. Чтение настроек и реестр —
// internal/osx/cloud*_test.go.

import (
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestCloudRootsLive: корни из настроек клиентов добавляются к окружению без повторов; фильтр локальности — CheckLocalPath;
// для чужой ОС (таблицы тестов) настройки не читаются; не уложились в срок — только окружение.
func TestCloudRootsLive(t *testing.T) {
	old, oldBudget := osCloudRoots, cloudBudget
	t.Cleanup(func() { osCloudRoots, cloudBudget = old, oldBudget })
	env := map[string]string{"OneDrive": `C:\Users\u\OneDrive`}
	get := func(k string) string { return env[k] }
	dropbox, one := testkit.AbsPath("Данные/Dropbox"), `C:\Users\u\OneDrive`

	var gotHome string
	var local func(string) error
	osCloudRoots = func(home string, l func(string) error) []string {
		gotHome, local = home, l
		return []string{dropbox, one, dropbox}
	}
	// Своя ОС процесса: окружение плюс настройки, повторы схлопнуты.
	got := CloudRoots(runtime.GOOS, "/h", get)
	want := append(slices.Clone(got), dropbox)
	live := CloudRootsLive(runtime.GOOS, "/h", get)
	for _, p := range want {
		if !slices.Contains(live, p) {
			t.Errorf("нет %q в %q", p, live)
		}
	}
	if seen := map[string]bool{}; len(live) > 0 {
		for _, p := range live {
			if seen[p] {
				t.Errorf("повтор %q в %q", p, live)
			}
			seen[p] = true
		}
	}
	if gotHome != "/h" || local == nil || local("//srv/share/x") == nil || local("") == nil {
		t.Errorf("фильтр: home=%q, фильтр задан: %v", gotHome, local != nil)
	}

	// Чужая ОС — настроек нет.
	called := false
	osCloudRoots = func(string, func(string) error) []string { called = true; return []string{dropbox} }
	other := "windows"
	if runtime.GOOS == "windows" {
		other = "linux"
	}
	if live = CloudRootsLive(other, "/h", get); called || slices.Contains(live, dropbox) {
		t.Errorf("настройки прочитаны для чужой ОС: %v, %q", called, live)
	}

	// Повисшее чтение: срок вышел — только окружение, вызов не ждёт.
	hang := make(chan struct{})
	t.Cleanup(func() { close(hang) })
	osCloudRoots = func(string, func(string) error) []string { <-hang; return []string{dropbox} }
	cloudBudget = 20 * time.Millisecond
	start := time.Now()
	live = CloudRootsLive(runtime.GOOS, "/h", get)
	if slices.Contains(live, dropbox) || !slices.Equal(live, CloudRoots(runtime.GOOS, "/h", get)) {
		t.Errorf("после срока: %q", live)
	}
	if d := time.Since(start); d > testkit.Limit(5*time.Second) {
		t.Errorf("вызов ждал повисшее чтение %v", d)
	}
}

// TestVirtualCloudSeam: VirtualCloud отдаёт ответ шва osVirtualCloud вместе с путём (на любой ОС; настоящая
// проба — internal/osx).
func TestVirtualCloudSeam(t *testing.T) {
	old := osVirtualCloud
	t.Cleanup(func() { osVirtualCloud = old })
	var got []string
	osVirtualCloud = func(p string) bool { got = append(got, p); return p == "G:/Мой диск" }
	if !VirtualCloud("G:/Мой диск") || VirtualCloud("D:/Фото") {
		t.Error("ответ шва не передан как есть")
	}
	if len(got) != 2 || got[0] != "G:/Мой диск" || got[1] != "D:/Фото" {
		t.Errorf("пути в шов: %q", got)
	}
}

// TestPlaceNotFromDropbox: корень Dropbox из настроек и его прямая подпапка — не место из родителя, подпапка второго
// уровня — место (как у OneDrive).
func TestPlaceNotFromDropbox(t *testing.T) {
	e := unixEnv("linux")
	e.Cloud = append(e.Cloud, "/data/Dropbox")
	for _, c := range []struct {
		dir string
		bad bool
	}{
		{"/data/Dropbox", true}, {"/data/Dropbox/Фото", true}, {"/data/Dropbox/Фото/2026", false}, {"/data/Съёмки/2026", false},
	} {
		if err := e.Static(c.dir, prefs.PlaceParent); (err != nil) != c.bad {
			t.Errorf("%s: %v, ждали отказ=%v", c.dir, err, c.bad)
		}
	}
}
