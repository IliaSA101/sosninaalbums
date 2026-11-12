package app

import (
	"errors"
	"net/url"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/keymap"
)

// withVersion подменяет wails.json (источник версии) на время теста.
func withVersion(t *testing.T, ver string) {
	t.Helper()
	old := wailsJSON
	t.Cleanup(func() { wailsJSON = old })
	wailsJSON = []byte(`{"info":{"productVersion":"` + ver + `"}}`)
}

// captureOpen собирает адреса, которые ушли бы в браузер.
func captureOpen(t *testing.T) *[]string {
	t.Helper()
	opened := new([]string)
	old := openURL
	t.Cleanup(func() { openURL = old })
	openURL = func(_ *App, u string) { *opened = append(*opened, u) }
	return opened
}

// TestOpenLinkURLs: каждый id из белого списка открывает точный зашитый адрес; файлы — на метку
// своей версии; версия в форме ошибки экранирована; у dev метки нет.
func TestOpenLinkURLs(t *testing.T) {
	const repo = "https://github.com/IliaSA101/sosninaalbums"
	cases := []struct {
		ver, id, want string
	}{
		{"0.23.0", "source", repo + "/tree/v0.23.0"},
		{"0.23.0", "license", repo + "/blob/v0.23.0/NOTICE.md"},
		{"0.23.0", "telegram", "https://t.me/Ilya_Sosnin"},
		{"0.23.0", "marina", "https://sosnina-films.ru/contacts"},
		// «Об авторах» (129 §6): README библиотеки, раздел «Авторы»; метки версии нет.
		{"0.27.0", "authors", "https://github.com/IliaSA101/sosninaalbums-library#%D0%B0%D0%B2%D1%82%D0%BE%D1%80%D1%8B"},
		{"dev", "authors", "https://github.com/IliaSA101/sosninaalbums-library#%D0%B0%D0%B2%D1%82%D0%BE%D1%80%D1%8B"},
		{"0.23.0", "bug", repo + "/issues/new?template=bug.yml&version=0.23.0"},
		{"0.23.0", "idea", repo + "/issues/new?template=idea.yml"},
		// Версия с символами адреса не ломает ни путь, ни запрос.
		{"0.23.0+a b&c=d", "bug", repo + "/issues/new?template=bug.yml&version=0.23.0%2Ba+b%26c%3Dd"},
		{"0.23.0 x/y", "source", repo + "/tree/v0.23.0%20x%2Fy"},
		// dev (и версия не прочиталась) — без метки: основная ветка, форма без версии.
		{"dev", "source", repo},
		{"dev", "license", repo + "/blob/main/NOTICE.md"},
		{"dev", "bug", repo + "/issues/new?template=bug.yml"},
		{"dev", "telegram", "https://t.me/Ilya_Sosnin"},
		{"dev", "idea", repo + "/issues/new?template=idea.yml"},
		// Видео (113): всегда основная ветка, метки версии нет.
		{"0.25.0", "videos", repo + "/blob/main/docs/VIDEOS.md"},
		{"dev", "videos", repo + "/blob/main/docs/VIDEOS.md"},
		// Скачать (112): страница последнего выпуска, без метки версии (она старше файла).
		{"0.25.0", "download", repo + "/releases/latest"},
		{"dev", "download", repo + "/releases/latest"},
	}
	for _, c := range cases {
		withVersion(t, c.ver)
		opened := captureOpen(t)
		a := NewApp()
		if err := a.OpenLink(c.id); err != nil {
			t.Fatalf("%s %s: %v", c.ver, c.id, err)
		}
		if !slices.Equal(*opened, []string{c.want}) {
			t.Errorf("%s %s: открыто %q, нужно %q", c.ver, c.id, *opened, c.want)
		}
		if _, err := url.Parse(c.want); err != nil {
			t.Errorf("%s %s: адрес не разбирается: %v", c.ver, c.id, err)
		}
	}
}

// TestOpenLinkNoVersion: wails.json не прочитан (nil, пустая версия) — как dev, без метки v.
func TestOpenLinkNoVersion(t *testing.T) {
	old := wailsJSON
	t.Cleanup(func() { wailsJSON = old })
	for _, raw := range [][]byte{nil, []byte(`{`), []byte(`{"info":{"productVersion":""}}`)} {
		wailsJSON = raw
		opened := captureOpen(t)
		if err := NewApp().OpenLink("source"); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(*opened, []string{"https://github.com/IliaSA101/sosninaalbums"}) {
			t.Errorf("raw %q: открыто %q", raw, *opened)
		}
	}
}

// TestOpenLinkUnknown: любой id вне белого списка — ошибка, браузер не открыт. В том числе
// готовые адреса и близкие к настоящим id: адрес от интерфейса не принимается.
func TestOpenLinkUnknown(t *testing.T) {
	withVersion(t, "0.23.0")
	opened := captureOpen(t)
	a := NewApp()
	for _, id := range []string{"", " ", "Source", "source ", "bug.yml", "../license", "https://evil.example/",
		"javascript:alert(1)", "file:///etc/passwd", "github", "tg", "Authors", "authors ", "author"} {
		err := a.OpenLink(id)
		if !errors.Is(err, errUnknownLink) {
			t.Errorf("id %q: ошибка %v, нужна errUnknownLink", id, err)
		}
	}
	if len(*opened) != 0 {
		t.Fatalf("браузер открыт для неизвестного id: %q", *opened)
	}
}

// TestOpenLinkAll: несколько ссылок подряд открываются по одной, порядок сохраняется, чужой id
// между ними ничего не добавляет.
func TestOpenLinkAll(t *testing.T) {
	withVersion(t, "1.2.3")
	opened := captureOpen(t)
	a := NewApp()
	for _, id := range []string{"source", "nope", "idea", "telegram"} {
		_ = a.OpenLink(id)
	}
	want := []string{
		"https://github.com/IliaSA101/sosninaalbums/tree/v1.2.3",
		"https://github.com/IliaSA101/sosninaalbums/issues/new?template=idea.yml",
		"https://t.me/Ilya_Sosnin",
	}
	if !slices.Equal(*opened, want) {
		t.Fatalf("открыто %q", *opened)
	}
}

// TestOpenLinkNoEdit: открытие ссылки не трогает проект (ни шага отмены, ни rev) — холостая
// правка без шага.
func TestOpenLinkNoEdit(t *testing.T) {
	withVersion(t, "0.23.0")
	captureOpen(t)
	a := NewApp()
	before := a.editor().View()
	_ = a.OpenLink("bug")
	_ = a.OpenLink("zzz")
	if v := a.editor().View(); v.Rev != before.Rev || v.CanUndo != before.CanUndo {
		t.Fatal("OpenLink изменил проект")
	}
}

// TestAboutCommand: команда `about` есть в каталоге и без клавиши (пункт меню) — окно
// открывается из «Справки», а не с клавиатуры.
func TestAboutCommand(t *testing.T) {
	var found bool
	for _, c := range keymap.Build(nil).Commands {
		if c.ID == "about" {
			found = true
			if len(c.Keys) != 0 {
				t.Errorf("у about клавиши %v, пункт меню без клавиши", c.Keys)
			}
		}
	}
	if !found {
		t.Fatal("команды about нет в каталоге")
	}
}
