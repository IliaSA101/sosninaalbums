package update

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode"
)

// Манифест выпуска (design §5.1): body конверта SosninaAlbums-update.json. Подпись уже
// проверена fetch.Open; здесь — разбор по строгим правилам.
const (
	manifestFormat = 1
	appID          = "sosninaalbums"
	// FileKey — запись установщика Windows в files.
	FileKey = "windows-x64-setup"
	// MinSetup, MaxSetup — размер установщика (design §5.1): 8 МБ в жизни, пределы с запасом.
	MinSetup    = 1 << 20
	MaxSetup    = 80 << 20
	maxNotes    = 8
	maxNote     = 300
	maxBody     = 11 << 10 // body; конверт (base64 + sig) — maxEnvelope
	maxEnvelope = 16 << 10
)

// Ошибки разбора и проверки: по-русски, без адресов и путей — уходят в тост и log.txt.
var (
	ErrNewerFormat = errors.New("нужна более новая программа — скачайте её со страницы выпуска")
	ErrManifest    = errors.New("файл обновления не читается — скачайте установщик со страницы выпуска")
	// ErrVerify — подпись, sha256 или длина не сошлись.
	ErrVerify = errors.New("обновление не прошло проверку — скачайте установщик со страницы выпуска")
)

// File — установщик в манифесте: имя (строится из версии, здесь лишь сверяется), размер, sha256.
type File struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Manifest — проверенный манифест. Setup nil — выпуск без установщика Windows (только страница).
type Manifest struct {
	Version string
	Date    string
	MinAuto string // с этой версии автообновление безопасно; пусто — с любой
	Notes   []string
	Setup   *File
}

// SetupName — имя установщика Windows для версии ver; "" — ver не x.y.z. Имя на диске и в
// адресе строится только отсюда, строка из манифеста лишь сверяется.
func SetupName(ver string) string {
	if _, ok := ParseVersion(ver); !ok {
		return ""
	}
	return "SosninaAlbums-" + ver + "-windows-x64-setup.exe"
}

// Parse — разбор body: format новее нашего — ErrNewerFormat, не наш формат или приложение —
// ErrManifest; version и minAuto — x.y.z; notes — не больше 8 строк, каждая очищена и ≤ 300
// знаков; запись Windows — имя ровно SetupName(version), размер MinSetup…MaxSetup, sha256
// hex-64 нижним регистром (неверная запись — отказ всего файла, а не молчаливое «без неё»).
// Лишние поля пропускаются.
func Parse(body []byte) (Manifest, error) {
	if len(body) > maxBody {
		return Manifest{}, ErrManifest
	}
	var raw struct {
		Format  int             `json:"format"`
		App     string          `json:"app"`
		Version string          `json:"version"`
		Date    string          `json:"date"`
		MinAuto string          `json:"minAuto"`
		Notes   []string        `json:"notes"`
		Files   map[string]File `json:"files"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Manifest{}, ErrManifest
	}
	switch {
	case raw.Format > manifestFormat:
		return Manifest{}, ErrNewerFormat
	case raw.Format != manifestFormat || raw.App != appID:
		return Manifest{}, ErrManifest
	}
	if _, ok := ParseVersion(raw.Version); !ok {
		return Manifest{}, ErrManifest
	}
	if _, ok := ParseVersion(raw.MinAuto); raw.MinAuto != "" && !ok {
		return Manifest{}, ErrManifest
	}
	m := Manifest{Version: raw.Version, Date: CleanText(raw.Date, 10), MinAuto: raw.MinAuto, Notes: []string{}}
	for _, n := range raw.Notes {
		if len(m.Notes) == maxNotes {
			break
		}
		if n = CleanText(n, maxNote); n != "" {
			m.Notes = append(m.Notes, n)
		}
	}
	if f, ok := raw.Files[FileKey]; ok {
		if f.Name != SetupName(raw.Version) || f.Size < MinSetup || f.Size > MaxSetup || !validSum(f.SHA256) {
			return Manifest{}, ErrManifest
		}
		m.Setup = &f
	}
	return m, nil
}

// validSum — sha256 в hex: 64 знака 0-9a-f.
func validSum(s string) bool {
	return len(s) == 64 && strings.Trim(s, "0123456789abcdef") == ""
}

// CleanText — текст из чужого файла для показа: без управляющих и невидимых знаков, пробелы
// схлопнуты, не длиннее n знаков.
func CleanText(s string, n int) string {
	s = strings.ToValidUTF8(s, "")
	var b strings.Builder
	runes, space := 0, false
	for _, r := range s {
		if unicode.IsSpace(r) {
			space = b.Len() > 0
			continue
		}
		if !unicode.IsPrint(r) {
			continue
		}
		if space {
			if runes++; runes > n {
				break
			}
			b.WriteByte(' ')
			space = false
		}
		if runes++; runes > n {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}
