package prefs

// Update — состояние обновления программы (спека 111, internal/update): Off — «Проверять при
// запуске» снято (по умолчанию проверка включена, поэтому хранится выключение), Checked — unix
// мс последней удачной проверки, Seen — наибольшая версия в подписанных файлах выпуска (старее
// неё файл — повтор старого выпуска, не предложение), Pending и PendingAt — версия, до которой
// запущено обновление, и когда: после перезапуска по ним программа говорит «обновлена» или «не
// установилось».
type Update struct {
	Off       bool   `json:"off,omitempty"`
	Checked   int64  `json:"checked,omitempty"`
	Seen      string `json:"seen,omitempty"`
	Pending   string `json:"pending,omitempty"`
	PendingAt int64  `json:"pendingAt,omitempty"`
}

// clean — после правки руками: время не отрицательное, версии x.y.z (иначе пусто); отметка
// времени без версии не нужна.
func (u Update) clean() Update {
	u.Checked, u.PendingAt = max(u.Checked, 0), max(u.PendingAt, 0)
	if !validVersion(u.Seen) {
		u.Seen = ""
	}
	if !validVersion(u.Pending) {
		u.Pending = ""
	}
	if u.Pending == "" {
		u.PendingAt = 0
	}
	return u
}

// UpdateOf — состояние обновления из config.json (нет — проверка включена, ничего не было).
func UpdateOf(dir string) Update {
	c := LoadConfig(dir)
	if c.Update == nil {
		return Update{}
	}
	return c.Update.clean()
}

// UpdateUpdate — правка состояния под замком config.json; итог после чистки. Всё по
// умолчанию — ключа в файле нет.
func UpdateUpdate(dir string, fn func(u *Update)) Update {
	var out Update
	UpdateConfig(dir, func(c *Config) {
		var u Update
		if c.Update != nil {
			u = *c.Update
		}
		fn(&u)
		if out = u.clean(); out == (Update{}) {
			c.Update = nil
		} else {
			c.Update = &out
		}
	})
	return out
}

// validVersion — x.y.z, части по 1…4 цифры (как update.ParseVersion; prefs не знает update).
func validVersion(s string) bool {
	parts, n := 1, 0
	for i := range len(s) {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			if n++; n > 4 {
				return false
			}
		case c == '.' && n > 0 && parts < 3:
			parts, n = parts+1, 0
		default:
			return false
		}
	}
	return parts == 3 && n > 0
}
