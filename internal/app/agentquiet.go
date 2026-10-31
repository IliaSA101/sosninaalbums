package app

// Папка и новый альбом без вопроса при «Всегда» (150, design §4.1): план по подсказке агента —
// настоящий путь, ссылка, причина «не тихо» (AgentAsk.Why) — и проверки по порядку, дешёвое раньше.
// Применение тихой правки — agentquiet_apply.go, подсчёт папки — agentquiet_count.go.

import (
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Коды причин AgentAsk.Why (тексты — во фронте, агенту причина не сообщается).
const (
	whyBad    = "bad"               // подсказка негодна по правилу 149 (в любом режиме)
	whyPlace  = "place"             // вне мест съёмок
	whyDisk   = agentplaces.WhyDisk // внутри места, но на другом диске, не на том, что у места (153)
	whyVol    = agentplaces.WhyVol  // диск места узнать не удалось (153)
	whyRemote = "remote"            // том не локальный
	whyBig    = "big"               // слишком много файлов, папок или не уложились во время
	whyCloud  = "cloud"             // облачные заглушки
	whyShots  = "shots"             // папка снимков экрана
	whyLink   = "link"              // ссылка или junction
	whyRate   = "rate"              // четвёртая тихая за 10 минут (слот службы не выдан)
	whyDraft  = "draft"             // открытый альбом правил человек
)

// Подмены для тестов: том, атрибуты, места снимков экрана, звенья-ссылки (функции agentplaces; правила
// и их отказ — ErrPlaceBad у нулевых правил — там же, здесь только порядок проверок).
var (
	quietRemote = osx.Remote
	quietHidden = osx.Hidden
	quietShots  = agentplaces.ScreenshotDirs
	quietLink   = agentplaces.HasLinkPart
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
	cleaned, resolved, err := r.Resolve(req.Hint)
	if err != nil || r.HiddenHint(resolved, quietHidden) {
		p.why = whyBad
		return p
	}
	p.dir, p.label, p.path = cleaned, agentplaces.HintLabel(resolved), resolved // имя — настоящее, не из подсказки-ссылки
	p.link = r.Key(cleaned) != r.Key(resolved)
	if always {
		p.why, p.note = a.quietWhy(r, req, cleaned, resolved, cur)
	}
	return p
}

// quietWhy — первая причина «не тихо» по design §4.1 п. 1, 3–8 ("" — можно) и строка к ней (note).
func (a *App) quietWhy(r *agentplaces.Rules, req agents.AskReq, cleaned, resolved string, cur *album.Editor) (why, note string) {
	// Лимит — последним (ревью): настоящая причина (место, ссылка…) важнее исчерпанного бюджета.
	isLink := quietLink // подмену читаем до горутины: она может пережить предел
	linked, done := appkit.Within(r.Timeout(), func() bool { return isLink(resolved) })
	switch {
	case !done || linked: // повисший том (NFS) — как ссылка: тихо нельзя
		return whyLink, ""
	case r.VolumeMoved(cleaned, resolved): // subst-диск, отображённая папка: введённое не то, что на диске
		return whyLink, ""
	}
	if why, note = a.placeWhy(r, resolved); why != "" {
		return why, note
	}
	switch {
	case agentplaces.InAny(r, resolved, quietShots()):
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
