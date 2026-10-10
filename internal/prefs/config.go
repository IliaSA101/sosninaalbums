// Package prefs — настройки программы в папке данных: config.json (последние папки
// диалогов, уровень интерфейса, «Свой», «Последние») и хранилище шаблонов templates.json.
package prefs

import (
	"encoding/json"
	"log"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Виды диалогов: у каждого своя последняя папка.
const (
	DirProject = "project" // «Открыть» и «Сохранить как»
	DirPhotos  = "photos"  // «Добавить папку»: родитель последней добавленной
	DirExport  = "export"
)

// Config — маленький JSON настроек программы (config.json в папке данных).
type Config struct {
	LastProject string            `json:"lastProject"`
	LastDirs    map[string]string `json:"lastDirs,omitempty"` // вид диалога → папка
	Level       string            `json:"level,omitempty"`    // уровень интерфейса; пусто — первый запуск
	HideStart   bool              `json:"hideStart,omitempty"`
	Recent      []string          `json:"recent,omitempty"` // последние альбомы, новые первыми, ≤ maxRecent
	Custom      *CustomLevel      `json:"custom,omitempty"` // набор «Своего»; остаётся при смене уровня
}

// configKeys — ключи config.json, которыми владеет эта версия (по тегам Config).
// Остальные при записи не трогаем: их пишет другая версия программы.
var configKeys = func() []string {
	t := reflect.TypeFor[Config]()
	keys := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		keys = append(keys, strings.Split(t.Field(i).Tag.Get("json"), ",")[0])
	}
	return keys
}()

// cfgMu — config.json читается и пишется целиком: read-modify-write под замком.
// Файл один на процесс, поэтому и замок общий.
var cfgMu sync.Mutex

// ConfigPath — config.json в папке данных dir.
func ConfigPath(dir string) string { return filepath.Join(dir, "config.json") }

// LoadConfig читает config.json под замком.
func LoadConfig(dir string) Config {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	return loadConfig(dir)
}

func loadConfig(dir string) Config {
	var c Config
	if data, err := os.ReadFile(ConfigPath(dir)); err == nil {
		// Битый конфиг — как отсутствующий: просто откроется черновик.
		_ = json.Unmarshal(data, &c)
	}
	return c
}

// UpdateConfig читает конфиг, применяет fn и пишет обратно атомарно: настройки не должны
// затирать друг друга (проект и последние папки меняются из разных методов).
// Чужие ключи (из более новой версии) переписываются как были.
func UpdateConfig(dir string, fn func(c *Config)) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	c := loadConfig(dir)
	fn(&c)
	path := ConfigPath(dir)
	raw := map[string]json.RawMessage{}
	if old, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(old, &raw) // битый файл — как пустой
	}
	if raw == nil { // файл "null"
		raw = map[string]json.RawMessage{}
	}
	for _, k := range configKeys {
		delete(raw, k) // omitempty-поле, ставшее пустым, должно исчезнуть
	}
	own, _ := json.Marshal(c)
	var mine map[string]json.RawMessage
	_ = json.Unmarshal(own, &mine)
	maps.Copy(raw, mine)
	data, _ := json.MarshalIndent(raw, "", "  ")
	if err := library.WriteFileAtomic(path, data); err != nil {
		log.Printf("config.json: %v", paths.NoPath(err))
	}
}

// RememberDir запоминает папку d для диалога вида kind. Нелокальные пути не пишем.
func RememberDir(dir, kind, d string) {
	if d == "" || paths.CheckLocalPath(d) != nil {
		return
	}
	UpdateConfig(dir, func(c *Config) {
		if c.LastDirs == nil {
			c.LastDirs = map[string]string{}
		}
		c.LastDirs[kind] = d
	})
}

// DialogDir — DefaultDirectory для диалога вида kind ("" — пусть решает система).
func DialogDir(dir, kind string) string {
	last := LoadConfig(dir).LastDirs[kind]
	home, _ := os.UserHomeDir()
	return PickDir(last, filepath.Join(home, "Documents"))
}

// PickDir выбирает папку для диалога: последняя, если это локальный путь и папка есть;
// иначе ближайший существующий родитель; иначе fallback (если он есть); иначе "".
func PickDir(last, fallback string) string {
	if last != "" && paths.CheckLocalPath(last) == nil {
		for p := filepath.Clean(last); ; p = filepath.Dir(p) {
			if isDir(p) {
				return p
			}
			if filepath.Dir(p) == p {
				break
			}
		}
	}
	if fallback != "" && isDir(fallback) {
		return fallback
	}
	return ""
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
