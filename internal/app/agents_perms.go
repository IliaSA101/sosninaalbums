package app

// Разрешения агентов по инструментам и места съёмок (спека 0.34/150, контракт Т1). Хранение —
// prefs/agentperms.go. Три метода пишут разрешения уже сейчас (тонкие обёртки над prefs); места и
// «Папка со съёмками…» — agentplaces*.go; «Открыть прежний» — agentquiet_apply.go, «Выбрать другую…» —
// agentask.go. Список Perms в AgentsInfo — по строкам таблицы службы.

import (
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// AgentPerm — строка окна «ИИ-агенты ▸ Разрешения».
type AgentPerm struct {
	Tool  string   `json:"tool"`  // имя MCP
	Title string   `json:"title"` // «Добавить папку»
	Group string   `json:"group"` // read | edit | other (запуск и показ — вне группового щелчка)
	Mode  string   `json:"mode"`  // allow | ask | deny
	Can   []string `json:"can"`   // допустимые значения
	Why   string   `json:"why"`   // причина, если «Спрашивать» нет в Can
	Note  string   `json:"note"`  // пояснение под строкой
}

// AgentPlace — место съёмок: «Всегда» у папки берёт без вопроса только внутри мест.
type AgentPlace struct {
	Dir    string `json:"dir"`
	Chosen bool   `json:"chosen"` // «Папка со съёмками…»; иначе — родитель добавленной папки
}

// Группы SetAgentPermGroup.
const (
	permGroupRead = "read" // классы R
	permGroupEdit = "edit" // классы E, F, A
)

// Строки окна: запуск программы живёт у посредника, службе он неизвестен.
const startToolName, startToolTitle = "start_program", "Запустить программу"

// Пояснения под строкой (AgentPerm.Why и Note): тексты из Go, интерфейс только показывает.
const (
	permWhyRead  = "Чтение не спрашивает: на одну просьбу агент читает десятки раз. Можно разрешить или запретить."
	permWhyStart = "Запуск нужен, когда программа закрыта, — спросить негде. Можно разрешить или запретить."
	permNoteImg  = "Без «+ картинки» отвечает отказом"
	permNoteDir  = "Без вопроса — в местах съёмок"
)

// agentPermRows — строки «Разрешения» для AgentsInfo: инструменты службы с названиями из её таблицы и
// запуск программы; значения — сверенный вид prefs (миграция, откат, умолчания).
func (a *App) agentPermRows() []AgentPerm {
	p := prefs.LoadAgentPerms(a.cfgDir)
	tools := a.agents.Tools()
	rows := make([]AgentPerm, 0, len(tools)+1)
	add := func(name, title, class string) {
		r := AgentPerm{Tool: name, Title: title, Group: permGroupOf(class), Mode: p.Mode(name), Can: prefs.PermCan(name)}
		if r.Mode == "" {
			r.Mode = p.NewMode(class == prefs.ClassRead)
		}
		switch class {
		case prefs.ClassRead:
			r.Why = permWhyRead
			if name == "get_photo_preview" || name == "get_spread_preview" || name == "get_contact_sheet" {
				r.Note = permNoteImg
			}
		case prefs.ClassStart:
			r.Why = permWhyStart
		case prefs.ClassFolder, prefs.ClassAlbum:
			r.Note = permNoteDir
		}
		rows = append(rows, r)
	}
	for _, t := range tools {
		add(t.Name, t.Title, t.Class)
	}
	add(startToolName, startToolTitle, prefs.ClassStart)
	return rows
}

// permGroupOf — группа окна по классу: запуск и показ разворота вне группового щелчка.
func permGroupOf(class string) string {
	switch class {
	case prefs.ClassRead:
		return permGroupRead
	case prefs.ClassEdit, prefs.ClassFolder, prefs.ClassAlbum:
		return permGroupEdit
	}
	return "other"
}

// agentPlaces — места для AgentsInfo; не nil. Ключа agentPerms ещё нет (переход с 0.33) — показывается
// предпросмотр засева, файл не трогается: запись случится при первом решении (seedPlaces).
func (a *App) agentPlaces() []AgentPlace {
	cfg := a.loadConfig()
	places := prefs.AgentPermsOf(cfg).Places
	if cfg.AgentPerms == nil {
		if seed := a.seedCandidate(cfg); seed.Dir != "" {
			places = []prefs.Place{seed}
		}
	}
	out := make([]AgentPlace, 0, len(places))
	for _, p := range places {
		out = append(out, AgentPlace{Dir: p.Dir, Chosen: p.How == prefs.PlaceChosen})
	}
	return out
}

// SetAgentPerm ставит разрешение одному инструменту. Режим вне допустимых (PermCan) и чужое
// имя — без изменений: решает Go, интерфейс только показывает.
func (a *App) SetAgentPerm(tool, mode string) AgentsInfo {
	defer guard("SetAgentPerm")
	if prefs.AgentToolClass(tool) != "" {
		a.setPerms(func(p *prefs.AgentPerms) { setPerm(p, tool, mode) })
	}
	return a.agentsInfo()
}

// SetAgentPermGroup ставит разрешение группе: read — чтение, edit — обратимые правки, папка и
// альбом. Запуск программы и показ разворота группа не трогает. Строкам, где режима нет, он не
// ставится («Спрашивать» у чтения).
func (a *App) SetAgentPermGroup(group, mode string) AgentsInfo {
	defer guard("SetAgentPermGroup")
	if group == permGroupRead || group == permGroupEdit {
		a.setPerms(func(p *prefs.AgentPerms) {
			for _, tool := range prefs.AgentToolNames() {
				c := prefs.AgentToolClass(tool)
				if group == permGroupRead && c == prefs.ClassRead || group == permGroupEdit && (c == prefs.ClassEdit || c == prefs.ClassFolder || c == prefs.ClassAlbum) {
					setPerm(p, tool, mode)
				}
			}
		})
	}
	return a.agentsInfo()
}

// setPerm — одно значение, если оно допустимо для инструмента.
func setPerm(p *prefs.AgentPerms, tool, mode string) {
	for _, m := range prefs.PermCan(tool) {
		if m == mode {
			p.Tools[tool] = mode
			return
		}
	}
}

// setPerms правит разрешения одной записью и сообщает окнам; открытый вопрос, чей инструмент стал
// «Запретить», служба закрывает сразу.
func (a *App) setPerms(fn func(p *prefs.AgentPerms)) {
	a.seedPlaces() // первая запись после обновления: место из последней папки фото — до неё
	prefs.UpdateAgentPerms(a.cfgDir, fn)
	a.agents.PermsChanged()
	a.agentsChanged()
}

// ForgetAgentPlaces — «Забыть места»: список мест пуст, разрешения инструментов целы.
func (a *App) ForgetAgentPlaces() AgentsInfo {
	defer guard("ForgetAgentPlaces")
	prefs.ForgetPlaces(a.cfgDir)
	a.agentsChanged()
	return a.agentsInfo()
}
