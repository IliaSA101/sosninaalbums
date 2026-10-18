// Package pack — разбор и проверка набора библиотеки (спека 067, design §4.1–§4.2): манифест,
// архив zip и потоковое чтение предметов со сверкой sha256 и размера. Только stdlib (depguard):
// картинки и шаблоны проверяет decor уже в своей копии — здесь целостность, имена и пределы.
//
// Граница безопасности: ни одно имя из архива не становится путём — имя служит лишь ключом
// сверки с манифестом; размер считается по фактическим байтам, а не по заголовкам zip.
package pack

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Пределы набора (design §4.2).
const (
	Format      = 1         // версия манифеста; больше — набор новее программы
	MaxZip      = 300 << 20 // архив целиком
	MaxEntries  = 2000      // записей в zip: 1000 предметов + папки и прочее
	MaxManifest = 256 << 10
	MaxItems    = 1000
	MaxTotal    = 1 << 30   // весь набор распакованным
	MaxBg       = 60 << 20  // фон; меньше своего (200 МБ): набор готовят заранее
	MaxSticker  = 30 << 20  // стикер
	MaxSpread   = 256 << 10 // шаблон разворота (= model.MaxTplFile)
	// ManifestName — манифест в корне архива.
	ManifestName = "manifest.json"
)

// Виды предметов набора (как виды библиотеки decor).
const (
	KindBg      = "bg"
	KindSticker = "sticker"
	KindSpread  = "spread"
)

// Ошибки для интерфейса: по-русски, без путей.
var (
	ErrNewer    = errors.New("набор из новой версии программы — нужна новая версия Sosnina Albums")
	ErrManifest = errors.New("манифест набора не читается")
	ErrDamaged  = errors.New("архив набора повреждён")
	ErrEmpty    = errors.New("в наборе нечего ставить")
)

// Manifest — manifest.json набора после проверки. Items — только предметы, которые эта версия
// ставит; остальные — в Skipped с причиной.
type Manifest struct {
	Format  int    `json:"format"`
	ID      string `json:"id"`
	Version int    `json:"version"`
	MinApp  string `json:"minApp"`
	Title   string `json:"title"`
	About   string `json:"about"`
	Author  string `json:"author"`
	License string `json:"license"`
	Source  string `json:"source"` // текст, не ссылка (решение 14)
	Items   []Item `json:"items"`
	Skipped []Skip `json:"-"`
}

// Item — предмет набора: имя записи архива, вид, тема, имя для показа, sha256 и размер байтов.
type Item struct {
	File   string `json:"file"`
	Kind   string `json:"kind"`
	Theme  string `json:"theme"`
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Skip — предмет, который не ставится, и причина для тоста.
type Skip struct {
	File string
	Why  string
}

// ParseManifest — разбор и проверка манифеста. app — версия программы x.y.z; другая строка
// (сборка dev) minApp не проверяет. Неизвестные поля пропускаются (манифест новее читается).
// Отказ целиком — шапка, имя, хэш или размер любого предмета не по правилам: такой манифест
// собран не нашим скриптом. Неизвестный вид, SVG и предмет больше предела — пропуск со строкой.
func ParseManifest(data []byte, app string) (Manifest, error) {
	if len(data) > MaxManifest {
		return Manifest{}, fmt.Errorf("%w: больше %d КБ", ErrManifest, MaxManifest>>10)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, ErrManifest
	}
	if err := m.checkHead(app); err != nil {
		return Manifest{}, err
	}
	if len(m.Items) == 0 || len(m.Items) > MaxItems {
		return Manifest{}, fmt.Errorf("%w: предметов должно быть от 1 до %d", ErrManifest, MaxItems)
	}
	items := m.Items
	m.Items = make([]Item, 0, len(items))
	seen := make(map[string]bool, len(items))
	var total int64
	for _, it := range items {
		if !ValidName(it.File) {
			return Manifest{}, fmt.Errorf("%w: недопустимое имя файла «%s»", ErrManifest, Clean(it.File, 64))
		}
		low := strings.ToLower(it.File)
		if seen[low] {
			return Manifest{}, fmt.Errorf("%w: файл «%s» указан дважды", ErrManifest, it.File)
		}
		seen[low] = true
		if !validSum(it.SHA256) || it.Size < 1 {
			return Manifest{}, fmt.Errorf("%w: у «%s» нет sha256 или размера", ErrManifest, it.File)
		}
		if why := itemSkip(it); why != "" {
			m.Skipped = append(m.Skipped, Skip{File: it.File, Why: why})
			continue
		}
		if total += it.Size; total > MaxTotal {
			return Manifest{}, fmt.Errorf("%w: распакованным больше %d ГБ", ErrManifest, MaxTotal>>30)
		}
		if !validKind(it.Theme) {
			it.Theme = "basic" // тему знает decor: незнакомую он тоже сводит к «Универсальным»
		}
		it.Name = Clean(it.Name, 60)
		m.Items = append(m.Items, it)
	}
	if len(m.Items) == 0 {
		return Manifest{}, ErrEmpty
	}
	return m, nil
}

// checkHead — шапка манифеста: формат, id, версия, minApp, тексты (чистятся и режутся).
func (m *Manifest) checkHead(app string) error {
	switch {
	case m.Format > Format:
		return ErrNewer
	case m.Format != Format:
		return fmt.Errorf("%w: нет format", ErrManifest)
	case !ValidID(m.ID):
		return fmt.Errorf("%w: недопустимый id набора", ErrManifest)
	case m.Version < 1 || m.Version > 999999:
		return fmt.Errorf("%w: версия набора — от 1 до 999999", ErrManifest)
	}
	need, ok := parseVer(m.MinApp)
	if !ok {
		return fmt.Errorf("%w: minApp — x.y.z", ErrManifest)
	}
	if have, ok := parseVer(app); ok && cmpVer(need, have) > 0 {
		return fmt.Errorf("нужна версия Sosnina Albums %s или новее", m.MinApp)
	}
	if m.Title = Clean(m.Title, 60); m.Title == "" {
		return fmt.Errorf("%w: у набора нет названия", ErrManifest)
	}
	m.About, m.Author = Clean(m.About, 300), Clean(m.Author, 60)
	m.License, m.Source = Clean(m.License, 40), Clean(m.Source, 200)
	return nil
}

// itemSkip — причина, по которой эта версия предмет не ставит; "" — ставит.
func itemSkip(it Item) string {
	ext := path.Ext(it.File)
	switch it.Kind {
	case KindBg, KindSticker:
		if ext == ".svg" {
			return "SVG в наборах не принимается — нужен PNG или JPEG" // решение 9
		}
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
			return "не картинка JPEG или PNG"
		}
	case KindSpread:
		if ext != ".json" {
			return "шаблон разворота — не .json"
		}
	default:
		return "незнакомый вид — нужна новая версия программы"
	}
	if limit := KindLimit(it.Kind); it.Size > limit {
		return fmt.Sprintf("больше предела набора (%s)", sizeText(limit))
	}
	return ""
}

// KindLimit — предел распакованного предмета вида kind; незнакомый — 0.
func KindLimit(kind string) int64 {
	switch kind {
	case KindBg:
		return MaxBg
	case KindSticker:
		return MaxSticker
	case KindSpread:
		return MaxSpread
	}
	return 0
}

func sizeText(n int64) string {
	if n >= 1<<20 {
		return strconv.FormatInt(n>>20, 10) + " МБ"
	}
	return strconv.FormatInt(n>>10, 10) + " КБ"
}

// ValidID — id набора ^[a-z0-9][a-z0-9-]{2,39}$ и не устройство Windows: из него строится имя
// файла реестра <id>.json, а «nul.json» или «com1.json» — устройство, не файл.
func ValidID(id string) bool {
	if len(id) < 3 || len(id) > 40 || id[0] == '-' || deviceName(id) {
		return false
	}
	for i := range len(id) {
		if c := id[i]; !lowerDigit(c) && c != '-' {
			return false
		}
	}
	return true
}

// ValidName — имя предмета ^[a-z0-9][a-z0-9._-]{0,63}(/[a-z0-9][a-z0-9._-]{0,63}){0,2}$ без «..».
func ValidName(name string) bool {
	if name == "" || strings.Contains(name, "..") {
		return false
	}
	parts := strings.Split(name, "/")
	if len(parts) > 3 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 64 || !lowerDigit(p[0]) {
			return false
		}
		for i := 1; i < len(p); i++ {
			if c := p[i]; !lowerDigit(c) && c != '.' && c != '_' && c != '-' {
				return false
			}
		}
	}
	return true
}

func lowerDigit(c byte) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' }

// validKind — тема [a-z]{1,16} (как вид записи библиотеки).
func validKind(k string) bool {
	if k == "" || len(k) > 16 {
		return false
	}
	for i := range len(k) {
		if k[i] < 'a' || k[i] > 'z' {
			return false
		}
	}
	return true
}

// validSum — sha256: 64 hex нижним регистром.
func validSum(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := range len(s) {
		if c := s[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// parseVer — x.y.z, части по 1…6 цифр.
func parseVer(s string) ([3]int, bool) {
	var v [3]int
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		if p == "" || len(p) > 6 || strings.Trim(p, "0123456789") != "" {
			return v, false
		}
		v[i], _ = strconv.Atoi(p)
	}
	return v, true
}

func cmpVer(a, b [3]int) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] > b[i] {
				return 1
			}
			return -1
		}
	}
	return 0
}

// Clean — текст для показа: без управляющих и невидимых знаков, пробелы схлопнуты, не длиннее
// maxRunes знаков; пусто — "". Как имя своего в decor: чужой текст не ломает строку окна.
func Clean(s string, maxRunes int) string {
	var b strings.Builder
	n, space := 0, false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = b.Len() > 0
			continue
		}
		if r == utf8.RuneError || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		if space {
			if n+1 >= maxRunes {
				break
			}
			b.WriteByte(' ')
			n++
			space = false
		}
		if n == maxRunes {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// deviceName — имя устройства Windows без учёта регистра (как paths.Device: pack — только stdlib).
// Знаки $ и ¹²³ грамматика id не пропускает, но проверка их держит — защита в глубину.
func deviceName(id string) bool {
	switch id = strings.ToLower(id); id {
	case "con", "prn", "aux", "nul", "conin$", "conout$":
		return true
	}
	r := []rune(id)
	return len(r) == 4 && (strings.HasPrefix(id, "com") || strings.HasPrefix(id, "lpt")) && strings.ContainsRune("0123456789¹²³", r[3])
}
