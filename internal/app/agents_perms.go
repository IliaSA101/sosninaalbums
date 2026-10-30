package app

// Разрешения агентов по инструментам и места съёмок (спека 0.34/150, контракт Т1). Хранение —
// prefs/agentperms.go. Три метода пишут разрешения уже сейчас (тонкие обёртки над prefs); места и
// «Папка со съёмками…» — agentplaces*.go; «Открыть прежний» — agentquiet_apply.go, «Выбрать другую…» —
// agentask.go. Список Perms в AgentsInfo — по строкам таблицы службы.

import (
	"slices"

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

// Состояния места в AgentPlacesLive (design 153 §4.5); "" — всё в порядке или не успели узнать.
const (
	placeNoDisk   = "nodisk"   // диск места сейчас не подключён
	placeNoFolder = "nofolder" // диск есть, папки на нём нет
	placeHere     = "here"     // место без привязки, а по его пути сейчас диск: можно привязать
)

// AgentPlace — место съёмок: «Всегда» у папки берёт без вопроса только внутри мест.
type AgentPlace struct {
	Key    string `json:"key"`    // ключ для ForgetAgentPlace / BindAgentPlace (12 hex), не зависит от буквы диска
	Dir    string `json:"dir"`    // сохранённый путь; у привязанного в AgentPlacesLive — текущий
	Label  string `json:"label"`  // метка диска, к которому привязано; "" — без метки или не привязано
	Chosen bool   `json:"chosen"` // «Папка со съёмками…»; иначе — родитель добавленной папки
	Bound  bool   `json:"bound"`  // привязано к диску по номеру (на macOS не бывает)
	// Узкое место (только из родителя): над годом правила не пустили выше, новый год спросит снова;
	// NarrowRoot — выше года только корень диска («Диск E: целиком — Папка со съёмками…»).
	Narrow     bool   `json:"narrow"`
	NarrowRoot bool   `json:"narrowRoot"`
	State      string `json:"state"` // placeNoDisk | placeNoFolder | placeHere; только AgentPlacesLive
	Here       string `json:"here"`  // метка диска по пути непривязанного места (State = placeHere); может быть пустой
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
	permWhyAsk   = "Выгрузка и открытие альбома спрашивают всегда: папку и файл подтверждаете вы. Можно запретить."
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
		case prefs.ClassAskOnly:
			r.Why = permWhyAsk // «Всегда» нет: щелчок по приглушённой кнопке объясняет почему
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

// savedPlaces — места из config.json. Ключа agentPerms ещё нет (переход с 0.33) — предпросмотр
// засева, файл не трогается: запись случится при первом решении (seedPlaces).
func (a *App) savedPlaces() []prefs.Place {
	cfg := a.loadConfig()
	places := prefs.AgentPermsOf(cfg).Places
	if cfg.AgentPerms == nil {
		if seed := a.seedCandidate(cfg); seed.Dir != "" {
			places = []prefs.Place{seed}
		}
	}
	return places
}

// placeDTOs — места как их видит окно по config.json: диска не трогаем.
func (a *App) placeDTOs(places []prefs.Place) []AgentPlace {
	out := make([]AgentPlace, 0, len(places))
	env := a.placeEnv()
	for _, p := range places {
		narrow, root := env.narrowOf(p)
		dto := AgentPlace{Key: prefs.PlaceKey(p), Dir: p.Dir, Chosen: p.How == prefs.PlaceChosen, Bound: p.Vol != nil, Narrow: narrow, NarrowRoot: root}
		if p.Vol != nil {
			dto.Label = p.Vol.Label
		}
		out = append(out, dto)
	}
	return out
}

// agentPlaces — места для AgentsInfo; не nil. Без проб дисков: agents:state идёт на каждое событие
// окна, а спящий диск не должен просыпаться от него (диски — AgentPlacesLive по раскрытию списка).
func (a *App) agentPlaces() []AgentPlace { return a.placeDTOs(a.savedPlaces()) }

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

// ForgetAgentPlace — ✕ у места в списке: убрать одно по ключу AgentPlace.Key. Нет такого (убрано в
// другом окне) — без записи. Ключ не зависит от буквы диска, поэтому работает и после её смены.
func (a *App) ForgetAgentPlace(key string) AgentsInfo {
	defer guard("ForgetAgentPlace")
	a.seedPlaces()
	if slices.ContainsFunc(prefs.LoadAgentPerms(a.cfgDir).Places, func(p prefs.Place) bool { return prefs.PlaceKey(p) == key }) {
		prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) { p.ForgetPlace(key) })
		a.agentsChanged()
	}
	return a.agentsInfo()
}

// BindAgentPlace — ссылка «Привязать к диску» у места без привязки: место запоминает диск, который
// сейчас по его пути (admit + номер тома под пределом). Узнать диск не удалось — место прежнее и
// ошибка; привязанного или чужого ключа касание холостое. Путь окно не присылает — только ключ.
func (a *App) BindAgentPlace(key string) (info AgentsInfo, err error) {
	defer guardErr("BindAgentPlace", &err)
	a.seedPlaces()
	places := prefs.LoadAgentPerms(a.cfgDir).Places
	i := slices.IndexFunc(places, func(p prefs.Place) bool { return prefs.PlaceKey(p) == key })
	if i < 0 || places[i].Vol != nil {
		return a.agentsInfo(), nil
	}
	pl := places[i]
	env := a.placeEnv()
	dir, err := env.admit(pl.Dir, pl.How)
	if err != nil {
		return a.agentsInfo(), err
	}
	if env.rules.key(dir) != env.rules.key(pl.Dir) {
		return a.agentsInfo(), errPlaceBad // по пути теперь другая папка (ссылка): не то место, что человек видит
	}
	vol := env.volOfDir(dir)
	if vol == nil {
		return a.agentsInfo(), errVolUnknown
	}
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) { p.BindPlaces(map[string]prefs.PlaceVol{key: *vol}) })
	a.agentsChanged()
	return a.agentsInfo(), nil
}
