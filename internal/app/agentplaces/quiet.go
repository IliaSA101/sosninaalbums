package agentplaces

// Правила тихой проверки папки (150, design §4.1) без App: разбор подсказки по диску, смена тома,
// известные папки снимков экрана. Порядок проверок и крючки — internal/app/agentquiet.go.

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
)

// Resolve — подсказка по правилам выбранной папки (строка, диск, путь после ссылок): очищенная
// строка и настоящий путь. Как Folder, но путь возвращается.
func (r *Rules) Resolve(raw string) (cleaned, resolved string, err error) {
	if !r.ok() {
		return "", "", r.refusal()
	}
	cleaned, err = r.Static(raw)
	if err != nil {
		return "", "", err
	}
	type probe struct {
		resolved string
		err      error
	}
	disk, dir := r.disk, cleaned // до Within: зависшая проба доживает в своей горутине, поля и именованный результат читать позже нельзя
	res, ok := appkit.Within(r.timeout, func() probe {
		p, err := disk(dir)
		return probe{p, err}
	})
	if !ok || res.err != nil || res.resolved == "" {
		return "", "", ErrTooWide
	}
	if _, err = r.Static(res.resolved); err != nil {
		return "", "", err
	}
	return cleaned, res.resolved, nil
}

// HiddenHint — Windows: элемент под домашней с «.» или скрытый/системный ниже корня тома (аудит С2).
// hidden — проба атрибутов (osx.Hidden или подмена теста); читается вызывающим до горутины.
func (r *Rules) HiddenHint(resolved string, hidden func(string) bool) bool {
	if !r.ok() || !r.windows {
		return false
	}
	p := r.Clean(resolved)
	for _, h := range r.homes {
		if !r.Inside(p, h) {
			continue
		}
		rel := strings.TrimLeft(p[min(len(r.Clean(h)), len(p)):], `\`)
		for el := range strings.SplitSeq(rel, `\`) {
			if strings.HasPrefix(el, ".") {
				return true
			}
		}
	}
	got, ok := appkit.Within(r.timeout, func() bool { return hidden(resolved) })
	return !ok || got
}

// VolumeMoved — Windows: настоящий путь на другом томе, чем введённый (subst X: C:\Users\u, сетевой
// диск, ссылка на другой диск): путь прошёл через подмену, тихо нельзя. Буква сравнивается без
// регистра; путей без буквы тома (UNC) это не касается, если оба UNC.
func (r *Rules) VolumeMoved(entered, resolved string) bool {
	return r != nil && r.windows && r.fold(winVolume(r.Clean(entered))) != r.fold(winVolume(r.Clean(resolved)))
}

// winVolume — том Windows-пути: «X:» или «\\сервер\ресурс»; "" — тома нет.
func winVolume(p string) string {
	switch {
	case len(p) >= 2 && p[1] == ':':
		return p[:2]
	case strings.HasPrefix(p, `\\`):
		el := strings.SplitN(p[2:], `\`, 3)
		if len(el) >= 2 {
			return `\\` + el[0] + `\` + el[1]
		}
	}
	return ""
}

// InAny — путь внутри любой из папок.
func InAny(r *Rules, p string, dirs []string) bool {
	for _, d := range dirs {
		if d != "" && r.Inside(p, d) {
			return true
		}
	}
	return false
}

// ScreenshotDirs — известные папки снимков экрана: «Изображения/Screenshots» и «Снимки экрана»
// (GNOME, KDE, Windows, в том числе в OneDrive). macOS сохраняет на Рабочий стол — он вне мест.
func ScreenshotDirs() []string {
	var bases []string
	if d := osx.PicturesDir(); d != "" {
		bases = append(bases, d)
	}
	if home, err := os.UserHomeDir(); err == nil && filepath.IsAbs(home) {
		bases = append(bases, filepath.Join(home, "Pictures"), filepath.Join(home, "Изображения"))
	}
	if od := os.Getenv("OneDrive"); od != "" && filepath.IsAbs(od) {
		bases = append(bases, filepath.Join(od, "Pictures"), filepath.Join(od, "Изображения"))
	}
	var out []string
	for _, b := range bases {
		out = append(out, filepath.Join(b, "Screenshots"), filepath.Join(b, "Снимки экрана"))
	}
	return out
}
