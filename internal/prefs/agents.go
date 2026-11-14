package prefs

// Agents — доступ ИИ-агентов к альбому (спека 0.31/143): On — служба включена, Level — что
// видят агенты: 1 — устройство программы, 2 — ещё имена файлов и тексты, 3 — ещё картинки.
// Edit — агентам разрешено менять альбом (149, от уровня не зависит). По умолчанию выключено
// и уровень 1 (минимум).
type Agents struct {
	On    bool `json:"on,omitempty"`
	Edit  bool `json:"edit,omitempty"`
	Level int  `json:"level,omitempty"` // 0/1 — устройство; 2 — + имена файлов и тексты; 3 — + картинки
}

// Уровни Agents.Level.
const (
	AgentsMinLevel = 1
	AgentsMaxLevel = 3
)

// clean — после правки руками: уровень вне 1…3 — минимальный (чужое значение не открывает больше).
func (a Agents) clean() Agents {
	if a.Level < AgentsMinLevel || a.Level > AgentsMaxLevel {
		a.Level = AgentsMinLevel
	}
	return a
}

// AgentsOf — настройки доступа агентов из config.json (нет — выключено, уровень 1).
func AgentsOf(dir string) Agents { return agentsIn(LoadConfig(dir)) }

func agentsIn(c Config) Agents {
	if c.Agents == nil {
		return Agents{Level: AgentsMinLevel}
	}
	return c.Agents.clean()
}

// AgentsLevelOf — уровень доступа из уже прочитанного config.json (тот же зажим, что у AgentsOf).
func AgentsLevelOf(c Config) int { return agentsIn(c).Level }

// UpdateAgents — правка настроек доступа под замком config.json; итог после чистки. Всё по
// умолчанию (выключено, уровень 1) — ключа в файле нет.
func UpdateAgents(dir string, fn func(a *Agents)) Agents {
	var out Agents
	UpdateConfig(dir, func(c *Config) {
		a := Agents{Level: AgentsMinLevel}
		if c.Agents != nil {
			a = *c.Agents
		}
		fn(&a)
		if out = a.clean(); out == (Agents{Level: AgentsMinLevel}) {
			c.Agents = nil
		} else {
			c.Agents = &out
		}
	})
	return out
}
