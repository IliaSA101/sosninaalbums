// Package prefs — настройки программы в папке данных: config.json (последние папки
// диалогов, уровень интерфейса, «Свой», «Последние») и хранилище шаблонов templates.json.
package prefs

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Виды диалогов: у каждого своя последняя папка.
const (
	DirProject = "project" // «Открыть» и «Сохранить как»
	DirPhotos  = "photos"  // «Добавить папку»: родитель последней добавленной
	DirLoose   = "loose"   // «Добавить фото» (097): папка первого выбранного файла
	DirExport  = "export"
	DirCollect = "collect" // «Собрать проект в папку» (069): выбранная папка, не папка альбома в ней
)

// Config — маленький JSON настроек программы (config.json в папке данных).
type Config struct {
	LastProject string            `json:"lastProject"`
	LastDirs    map[string]string `json:"lastDirs,omitempty"` // вид диалога → папка
	Level       string            `json:"level,omitempty"`    // уровень интерфейса; пусто — первый запуск
	HideStart   bool              `json:"hideStart,omitempty"`
	Recent      []string          `json:"recent,omitempty"` // последние альбомы, новые первыми, ≤ maxRecent
	Custom      *CustomLevel      `json:"custom,omitempty"` // набор «Своего»; остаётся при смене уровня
	// RecentDecor — недавние плитки библиотеки (048): вид (bg, sticker) → ID, новые первыми.
	RecentDecor map[string][]string `json:"recentDecor,omitempty"`
	// LibraryNet — согласие на сеть библиотеки и удалённые наборы (067 Ф3, libnet.go).
	LibraryNet *LibraryNet `json:"libraryNet,omitempty"`
	// Agents — доступ ИИ-агентов к альбому (143, agents.go): включение и уровень.
	Agents *Agents `json:"agents,omitempty"`
	// Hotkeys — свои горячие клавиши (074): id команды → её сочетания, только отличия от
	// стандартных (keymap.Overrides). Правила и пределы — internal/keymap, здесь не толкуются.
	Hotkeys map[string][]string `json:"hotkeys,omitempty"`
	// MyColors — «Свои цвета» окна выбора цвета (109): "#rrggbb", новые первыми, ≤ MaxMyColors.
	// Правила списка — colors.go.
	MyColors []string `json:"myColors,omitempty"`
	// Tours — состояние туров обучения (113): id → "pending" | "done" (tours.go). Чужие id
	// (новая версия) при записи сохраняются; чистка — только при чтении в Of.
	Tours map[string]string `json:"tours,omitempty"`
	// GuideLearned — отметки «Изучил» справочника F1 (140): id статей в порядке отметки;
	// правила списка — guide.go. Чужие id (новая версия) хранятся.
	GuideLearned []string `json:"guideLearned,omitempty"`
	// Update — проверка и установка обновления программы (111, update.go).
	Update *Update `json:"update,omitempty"`
	// Assoc — открытие .salbum двойным щелчком (112): "" — не решено (стартовое окно спросит),
	// "on" — согласие (связь обновляется сама), "off" — не спрашивать и не регистрировать.
	Assoc string `json:"assoc,omitempty"`
	// AssocLater — версия major.minor, на которой нажали «Не сейчас»: вопрос вернётся в следующей.
	AssocLater string `json:"assocLater,omitempty"`
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

// ConfigPath — config.json в папке данных dir. Пустая dir — папки данных нет (нет абсолютной
// папки настроек, 140 Н4): путь пуст, а не «config.json» от текущей папки.
func ConfigPath(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.json")
}

// memCfg — config.json без папки данных (dir == ""): настройки живут до выхода, на диск не
// пишутся. Под cfgMu. Тот же JSON, что в файле: копии не делят карты и срезы.
var memCfg []byte

// LoadConfig читает config.json под замком.
func LoadConfig(dir string) Config {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	return loadConfig(dir)
}

// maxConfigSize — предел config.json: настоящий весит килобайты; подсунутый гигабайтный файл
// не должен читаться целиком (Keymap() зовёт LoadConfig на каждом GetState).
const maxConfigSize = 1 << 20

// readConfigFile читает config.json не больше maxConfigSize; больше — как отсутствующий.
// Пустой путь — настройки в памяти (memCfg).
func readConfigFile(path string) ([]byte, error) {
	if path == "" {
		if memCfg == nil {
			return nil, os.ErrNotExist
		}
		return memCfg, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxConfigSize+1))
	if err == nil && len(data) > maxConfigSize {
		err = errConfigTooBig
	}
	return data, err
}

var errConfigTooBig = errors.New("config.json больше предела")

func loadConfig(dir string) Config {
	var c Config
	if data, err := readConfigFile(ConfigPath(dir)); err == nil {
		// Битый конфиг — как отсутствующий: просто откроется черновик.
		_ = json.Unmarshal(data, &c)
	}
	return c
}

// privateMode — права config.json и templates.json на Linux и Mac: недавние папки и имена из
// альбомов (аудит 0.23-sec), как у резервных копий. library.WriteFileAtomic остаётся 0644:
// им пишутся выгрузка и альбомы, которые пользователь отдаёт дальше.
const privateMode os.FileMode = 0o600

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
	if old, err := readConfigFile(path); err == nil {
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
	if path == "" {
		memCfg = data
		return
	}
	if err := library.WriteFileAtomicMode(path, data, privateMode); err != nil {
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
	fallback := ""
	// Без домашней папки Join дал бы относительное «Documents» — папку от текущей.
	if home, err := os.UserHomeDir(); err == nil && filepath.IsAbs(home) {
		fallback = filepath.Join(home, "Documents")
	}
	return PickDir(last, fallback)
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

// maxRecentDecor — недавних плиток на вид (решение 0.12 №98).
const maxRecentDecor = 12

// PushRecentDecor ставит id наверх без повторов; неверные ID (файл правили руками) отбрасываются,
// список режется до maxRecentDecor.
func PushRecentDecor(list []string, id string) []string {
	out := make([]string, 0, maxRecentDecor)
	if validDecorID(id) {
		out = append(out, id)
	}
	for _, v := range list {
		if len(out) == maxRecentDecor {
			break
		}
		if validDecorID(v) && !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	return out
}

// RememberDecor — поставленную плитку вида kind в недавние (пишет App после правки).
func RememberDecor(dir, kind, id string) {
	if !validDecorKind(kind) || !validDecorID(id) {
		return
	}
	UpdateConfig(dir, func(c *Config) {
		if c.RecentDecor == nil {
			c.RecentDecor = map[string][]string{}
		}
		c.RecentDecor[kind] = PushRecentDecor(c.RecentDecor[kind], id)
	})
}

// RecentDecor — недавние вида kind после проверки (файл мог править человек).
func RecentDecor(dir, kind string) []string {
	if !validDecorKind(kind) {
		return []string{}
	}
	return PushRecentDecor(LoadConfig(dir).RecentDecor[kind], "")
}

// validDecorKind — вид библиотеки [a-z]{1,16}, как в реестре decor.
func validDecorKind(k string) bool {
	return k != "" && len(k) <= 16 && strings.Trim(k, "abcdefghijklmnopqrstuvwxyz") == ""
}

// validDecorID — sa:<пресет> ([a-z0-9-]{1,48}) или my:<16 hex>; prefs не знает decor (граница пакетов).
func validDecorID(id string) bool {
	if rest, ok := strings.CutPrefix(id, "sa:"); ok {
		return rest != "" && len(rest) <= 48 && strings.Trim(rest, "abcdefghijklmnopqrstuvwxyz0123456789-") == ""
	}
	if rest, ok := strings.CutPrefix(id, "my:"); ok {
		return len(rest) == 16 && strings.Trim(rest, "0123456789abcdef") == ""
	}
	return false
}
