package app

// Папка и новый альбом без вопроса при «Всегда» (150, design §4.1): план по подсказке агента —
// настоящий путь, ссылка, причина «не тихо» (AgentAsk.Why) — и проверки по порядку, дешёвое раньше.
// Применение тихой правки — agentquiet_apply.go, подсчёт папки — agentquiet_count.go.

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Коды причин AgentAsk.Why (тексты — во фронте, агенту причина не сообщается).
const (
	whyBad    = "bad"    // подсказка негодна по правилу 149 (в любом режиме)
	whyPlace  = "place"  // вне мест съёмок
	whyDisk   = "disk"   // внутри места, но на другом диске, не на том, что у места (153)
	whyVol    = "vol"    // диск места узнать не удалось (153)
	whyRemote = "remote" // том не локальный
	whyBig    = "big"    // слишком много файлов, папок или не уложились во время
	whyCloud  = "cloud"  // облачные заглушки
	whyShots  = "shots"  // папка снимков экрана
	whyLink   = "link"   // ссылка или junction
	whyRate   = "rate"   // четвёртая тихая за 10 минут (слот службы не выдан)
	whyDraft  = "draft"  // открытый альбом правил человек
)

// Подмены для тестов: том, атрибуты, места снимков экрана, звенья-ссылки.
var (
	quietRemote = osx.Remote
	quietHidden = osx.Hidden
	quietShots  = screenshotDirs
	quietLink   = hasLinkPart
)

// Инструменты по виду вопроса: ключи разрешений и ссылка «Не спрашивать» в окне.
const (
	toolAddFolder   = "add_folder"
	toolCreateAlbum = "create_album"
	toolExportAlbum = "export_album"
	toolOpenAlbum   = "open_album"
)

// askTool — инструмент MCP по виду вопроса.
func askTool(kind string) string {
	switch kind {
	case agents.AskCreate:
		return toolCreateAlbum
	case agents.AskExport:
		return toolExportAlbum
	case agents.AskOpen:
		return toolOpenAlbum
	}
	return toolAddFolder
}

// askAllowed — инструмент вида kind не «Запретить» и процесс не от администратора: вопрос имеет
// смысл. Разрешение читается заново: человек мог сменить его, пока вопрос висел или шла проба.
func (a *App) askAllowed(kind string) bool {
	mode, blocked := a.agentPerm(askTool(kind))
	return blocked == "" && mode != prefs.PermDeny
}

// resolve — подсказка по правилам выбранной папки (строка, диск, путь после ссылок): очищенная
// строка и настоящий путь. Как folder, но путь возвращается.
func (r *folderRules) resolve(raw string) (cleaned, resolved string, err error) {
	if r == nil {
		return "", "", errFolderTooWide
	}
	cleaned, err = r.static(raw)
	if err != nil {
		return "", "", err
	}
	type probe struct {
		resolved string
		err      error
	}
	disk := r.disk // до within: зависшая проба доживает в своей горутине, поле читать позже нельзя
	res, ok := within(r.timeout, func() probe {
		p, err := disk(cleaned)
		return probe{p, err}
	})
	if !ok || res.err != nil || res.resolved == "" {
		return "", "", errFolderTooWide
	}
	if _, err = r.static(res.resolved); err != nil {
		return "", "", err
	}
	return cleaned, res.resolved, nil
}

// folderPlan — что сделать с подсказкой агента.
type folderPlan struct {
	dir   string // очищенная подсказка для окна ОС; "" — подсказки нет или негодна
	label string // последний элемент для AgentAsk.Hint
	path  string // настоящий путь годной подсказки; "" — вопрос без пути
	link  bool   // шла через ссылку
	why   string // причина «не тихо»; "" — тихо можно (при «Всегда») или режим «Спрашивать»
	note  string // строка вопроса к причине (why = disk: «Место … — на диске …»); "" — нет
}

// quiet — тихо можно: подсказка годна, причины нет.
func (p folderPlan) quiet() bool { return p.path != "" && p.why == "" }

// planFolder считает план: правила 149 — для всех режимов, остальное — только при «Всегда»
// (always: у человека «Всегда»; req.Always — к тому же тихий слот службы взят). cur — открытый
// альбом (для create).
func (a *App) planFolder(req agents.AskReq, cur *album.Editor, always bool) folderPlan {
	r := a.askM().rules()
	var p folderPlan
	if strings.TrimSpace(req.Hint) == "" {
		if always {
			p.why = whyPlace // подсказки нет — нечего проверять, тихо нельзя
		}
		return p
	}
	cleaned, resolved, err := r.resolve(req.Hint)
	if err != nil || a.hiddenHint(r, resolved) {
		p.why = whyBad
		return p
	}
	p.dir, p.label, p.path = cleaned, hintLabel(resolved), resolved // имя — настоящее, не из подсказки-ссылки
	p.link = r.key(cleaned) != r.key(resolved)
	if always {
		p.why, p.note = a.quietWhy(r, req, cleaned, resolved, cur)
	}
	return p
}

// hiddenHint — Windows: элемент под домашней с «.» или скрытый/системный ниже корня тома (аудит С2).
func (a *App) hiddenHint(r *folderRules, resolved string) bool {
	if r == nil || !r.windows {
		return false
	}
	p := r.clean(resolved)
	for _, h := range r.homes {
		if !r.inside(p, h) {
			continue
		}
		rel := strings.TrimLeft(p[min(len(r.clean(h)), len(p)):], `\`)
		for el := range strings.SplitSeq(rel, `\`) {
			if strings.HasPrefix(el, ".") {
				return true
			}
		}
	}
	isHidden := quietHidden // подмену читаем до горутины: она может пережить предел
	hidden, ok := within(r.timeout, func() bool { return isHidden(resolved) })
	return !ok || hidden
}

// quietWhy — первая причина «не тихо» по design §4.1 п. 1, 3–8 ("" — можно) и строка к ней (note).
func (a *App) quietWhy(r *folderRules, req agents.AskReq, cleaned, resolved string, cur *album.Editor) (why, note string) {
	// Лимит — последним (ревью): настоящая причина (место, ссылка…) важнее исчерпанного бюджета.
	isLink := quietLink // подмену читаем до горутины: она может пережить предел
	linked, done := within(r.timeout, func() bool { return isLink(resolved) })
	switch {
	case !done || linked: // повисший том (NFS) — как ссылка: тихо нельзя
		return whyLink, ""
	case r.volumeMoved(cleaned, resolved): // subst-диск, отображённая папка: введённое не то, что на диске
		return whyLink, ""
	}
	if why, note = a.placeWhy(r, resolved); why != "" {
		return why, note
	}
	switch {
	case inAny(r, resolved, quietShots()):
		return whyShots, ""
	case quietRemote(resolved):
		return whyRemote, ""
	}
	switch t := countQuiet(resolved); {
	case t.remote:
		return whyRemote, ""
	case t.cloud:
		return whyCloud, ""
	case t.over():
		return whyBig, ""
	}
	if req.Kind == agents.AskCreate && a.askDocOf(cur) == askDocDraft && !a.draftUntouched(cur) {
		return whyDraft, ""
	}
	if !req.Always {
		return whyRate, "" // «Всегда», а слот не выдан: бюджет тишины исчерпан
	}
	return "", ""
}

// volumeMoved — Windows: настоящий путь на другом томе, чем введённый (subst X: C:\Users\u, сетевой
// диск, ссылка на другой диск): путь прошёл через подмену, тихо нельзя. Буква сравнивается без
// регистра; путей без буквы тома (UNC) это не касается, если оба UNC.
func (r *folderRules) volumeMoved(entered, resolved string) bool {
	return r != nil && r.windows && r.fold(winVolume(r.clean(entered))) != r.fold(winVolume(r.clean(resolved)))
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

// inAny — путь внутри любой из папок.
func inAny(r *folderRules, p string, dirs []string) bool {
	for _, d := range dirs {
		if d != "" && r.inside(p, d) {
			return true
		}
	}
	return false
}

// screenshotDirs — известные папки снимков экрана: «Изображения/Screenshots» и «Снимки экрана»
// (GNOME, KDE, Windows, в том числе в OneDrive). macOS сохраняет на Рабочий стол — он вне мест.
func screenshotDirs() []string {
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
