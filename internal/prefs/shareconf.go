package prefs

import (
	"errors"
	"log"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// ShareConfig — подключение к сервису листалок (146) в config.json → share. Токена здесь нет и
// быть не может: он в internal/secret (0.32-146-8).
type ShareConfig struct {
	URL string `json:"url"` // origin сервиса; пусто — не подключено
}

// ErrShareURL — адрес сервиса не прошёл проверку правил.
var ErrShareURL = errors.New("адрес сервиса не годится")

// ErrShareSave — config.json не записался (текст без пути и подробностей ОС).
var ErrShareSave = errors.New("не удалось сохранить адрес сервиса")

// ShareURLOf — origin сервиса из config.json. Адрес читается по тем же правилам, что и при
// вводе: подделанный или устаревший файл даёт "" («не подключено», разбор №15), а не токен,
// отправленный на чужой адрес.
func ShareURLOf(dir string, rules ShareRules) string {
	c := LoadConfig(dir)
	if c.Share == nil {
		return ""
	}
	return rules.origin(c.Share.URL)
}

// SetShareURL проверяет адрес правилами и пишет его origin в config.json → share, остальные
// ключи не трогает. Пустой адрес убирает ключ. Негодный — ErrShareURL, файл прежний; файл не
// записался — ErrShareSave (адрес в нём прежний).
func SetShareURL(dir, raw string, rules ShareRules) (string, error) {
	var o string
	if raw != "" {
		if o = rules.origin(raw); o == "" {
			return "", ErrShareURL
		}
	}
	err := UpdateConfigErr(dir, func(c *Config) {
		if o == "" {
			c.Share = nil
			return
		}
		c.Share = &ShareConfig{URL: o}
	})
	if err != nil {
		log.Printf("config.json: %v", paths.NoPath(err))
		return "", ErrShareSave
	}
	return o, nil
}
