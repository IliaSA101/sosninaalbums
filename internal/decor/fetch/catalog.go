package fetch

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Пределы каталога (design §5, конверт — 0.17 решение №2): конверт целиком, body после base64,
// подпись в base64.
const (
	Format      = 1
	MaxEnvelope = 1400 << 10
	MaxCatalog  = 1 << 20
	MaxSig      = 200
	MaxPacks    = 500
	// MaxZip — архив набора (= pack.MaxZip: fetch не импортирует pack).
	MaxZip   = 300 << 20
	maxCount = 1000
)

// Catalog — проверенный каталог: номер выпуска seq, наборы по порядку файла; Skipped —
// отброшенные строки.
type Catalog struct {
	Seq     int64
	Packs   []Entry
	Skipped []Skip
}

// Entry — строка каталога: что за набор, какой версии, сколько весит, где лежит и его sha256.
type Entry struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
	MinApp  string `json:"minApp"`
	Title   string `json:"title"`
	About   string `json:"about"`
	License string `json:"license"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	URL     string `json:"url"`
	Counts  Counts `json:"counts"`
	// Needs — id набора-донора у дополнения (0.27): без донора netPlan его не ставит. Пусто — обычный набор.
	Needs string `json:"needs,omitempty"`
}

// Counts — сколько предметов каждого вида (для строки окна до скачивания).
type Counts struct {
	Bg      int `json:"bg"`
	Sticker int `json:"sticker"`
	Spread  int `json:"spread"`
	Frame   int `json:"frame"` // 079: старые версии лишний ключ пропускают
	Print   int `json:"print"` // 123: профили типографий
}

// Skip — строка каталога, которая не берётся, и причина (в log.txt).
type Skip struct {
	ID  string
	Why string
}

// Open — конверт catalog.json (0.17 решение №2): {"sig": base64 ed25519 по байтам body, "body":
// base64 внутреннего JSON}. Один файл, а не каталог и подпись порознь: CDN кэширует их
// независимо, и новый каталог со старой подписью выглядел бы взломом. Подпись сверяется до
// разбора body; итог — байты body.
func Open(env []byte, keys []ed25519.PublicKey) ([]byte, error) {
	if len(env) > MaxEnvelope {
		return nil, ErrCatalog
	}
	var e struct {
		Sig  string `json:"sig"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(env, &e); err != nil || e.Body == "" || len(e.Body) > base64.StdEncoding.EncodedLen(MaxCatalog) {
		return nil, ErrCatalog
	}
	body, err := base64.StdEncoding.DecodeString(e.Body)
	if err != nil || len(body) > MaxCatalog {
		return nil, ErrCatalog
	}
	if err := Verify(body, []byte(e.Sig), keys); err != nil {
		return nil, err
	}
	return body, nil
}

// Verify — подпись body (base64 ed25519, ≤ MaxSig) любым из ключей; до разбора JSON.
func Verify(data, sig []byte, keys []ed25519.PublicKey) error {
	s := bytes.TrimSpace(sig)
	if len(s) == 0 || len(sig) > MaxSig {
		return ErrSignature
	}
	raw, err := base64.StdEncoding.DecodeString(string(s))
	if err != nil || len(raw) != ed25519.SignatureSize {
		return ErrSignature
	}
	for _, k := range keys {
		if len(k) == ed25519.PublicKeySize && ed25519.Verify(k, data, raw) {
			return nil
		}
	}
	return ErrSignature
}

// ParseCatalog — разбор подписанного body: {format, seq, packs}. Шапка не по правилам (формат,
// seq < 1, больше MaxPacks наборов) — отказ целиком; строка
// с плохим id, версией, размером, хэшем или адресом (не с префиксом src.Prefix и не из белого
// списка) — пропуск; повтор id — берётся первая. Неизвестные поля пропускаются.
func ParseCatalog(data []byte, src Source) (Catalog, error) {
	if len(data) > MaxCatalog {
		return Catalog{}, ErrCatalog
	}
	var raw struct {
		Format int     `json:"format"`
		Seq    int64   `json:"seq"`
		Packs  []Entry `json:"packs"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return Catalog{}, ErrCatalog
	}
	switch {
	case raw.Format > Format:
		return Catalog{}, ErrNewer
	case raw.Format != Format || raw.Seq < 1 || len(raw.Packs) > MaxPacks:
		return Catalog{}, ErrCatalog
	}
	cat := Catalog{Seq: raw.Seq, Packs: make([]Entry, 0, len(raw.Packs)), Skipped: []Skip{}}
	seen := map[string]bool{}
	for i, e := range raw.Packs {
		why := entrySkip(&e, src)
		if why == "" && seen[e.ID] {
			why = "повтор id"
		}
		if why != "" {
			id := cleanText(e.ID, 40)
			if !validID(e.ID) {
				id = "#" + strconv.Itoa(i+1)
			}
			cat.Skipped = append(cat.Skipped, Skip{ID: id, Why: why})
			continue
		}
		seen[e.ID] = true
		cat.Packs = append(cat.Packs, e)
	}
	return cat, nil
}

// entrySkip — причина пропуска строки; "" — строка годна (тексты почищены, счётчики зажаты).
func entrySkip(e *Entry, src Source) string {
	switch {
	case !validID(e.ID):
		return "недопустимый id"
	case e.Version < 1 || e.Version > 999999:
		return "версия вне 1…999999"
	case !validVer(e.MinApp):
		return "minApp не x.y.z"
	case e.Size < 1 || e.Size > MaxZip:
		return "размер вне предела"
	case !validSum(e.SHA256):
		return "нет sha256"
	case !src.assetURL(e.URL):
		return "адрес не из библиотеки"
	case e.Needs != "" && (!validID(e.Needs) || e.Needs == e.ID):
		return "недопустимый needs"
	}
	if e.Title = cleanText(e.Title, 60); e.Title == "" {
		e.Title = e.ID
	}
	e.About, e.License = cleanText(e.About, 300), cleanText(e.License, 40)
	clampCount := func(n int) int { return min(max(n, 0), maxCount) }
	e.Counts = Counts{Bg: clampCount(e.Counts.Bg), Sticker: clampCount(e.Counts.Sticker), Spread: clampCount(e.Counts.Spread), Frame: clampCount(e.Counts.Frame), Print: clampCount(e.Counts.Print)}
	return ""
}

// assetURL — адрес архива: префикс источника, затем «<метка>/<файл>» из [A-Za-z0-9._-] без «..»,
// и сам адрес проходит белый список.
func (s Source) assetURL(raw string) bool {
	rest, ok := strings.CutPrefix(raw, s.Prefix)
	if !ok || s.Prefix == "" || strings.Contains(rest, "..") {
		return false
	}
	parts := strings.Split(rest, "/")
	if len(parts) > 2 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 100 || strings.TrimLeft(p, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-") != "" {
			return false
		}
	}
	u, err := url.Parse(raw)
	return err == nil && s.allowed(u)
}

// NeedsApp — набор требует программу новее app (x.y.z); другая строка (сборка dev) — нет.
func (e Entry) NeedsApp(app string) bool {
	need, ok1 := parseVer(e.MinApp)
	have, ok2 := parseVer(app)
	if !ok1 || !ok2 {
		return false
	}
	for i := range need {
		if need[i] != have[i] {
			return need[i] > have[i]
		}
	}
	return false
}

// Catalog — конверт каталога источника: подпись до разбора, затем body. seq меньше minSeq
// (последнего принятого) — откат к старому подписанному каталогу из истории репозитория:
// ErrSignature; равный — повтор того же выпуска, принимается.
func (c *Client) Catalog(ctx context.Context, minSeq int64) (Catalog, error) {
	ctx, cancel := context.WithTimeout(ctx, smallTimeout)
	defer cancel()
	env, err := c.small(ctx, c.src.CatalogURL, MaxEnvelope)
	if err != nil {
		return Catalog{}, err
	}
	body, err := Open(env, c.src.Keys)
	if err != nil {
		return Catalog{}, err
	}
	cat, err := ParseCatalog(body, c.src)
	if err != nil {
		return Catalog{}, err
	}
	if cat.Seq < minSeq {
		return Catalog{}, ErrSignature
	}
	return cat, nil
}

// Signed — подписанный конверт по адресу raw (из белого списка источника): не больше limit байт,
// подпись ключами источника проверяется до разбора; итог — байты body. Для файлов, которые не
// каталог наборов (манифест обновления программы, спека 111): разбор и пределы — у вызывающего.
func (c *Client) Signed(ctx context.Context, raw string, limit int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, smallTimeout)
	defer cancel()
	env, err := c.small(ctx, raw, limit)
	if err != nil {
		return nil, err
	}
	return Open(env, c.src.Keys)
}

// small — маленький файл целиком: больше limit — ErrCatalog (тело дальше не читается).
func (c *Client) small(ctx context.Context, raw string, limit int64) ([]byte, error) {
	resp, err := c.get(ctx, raw)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	if resp.ContentLength > limit {
		return nil, ErrCatalog
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, netErr(ctx, err)
	}
	if int64(len(data)) > limit {
		return nil, ErrCatalog
	}
	return data, nil
}

// validID — ^[a-z0-9][a-z0-9-]{2,39}$ (как pack.ValidID; устройства Windows проверяет decor).
func validID(id string) bool {
	if len(id) < 3 || len(id) > 40 || id[0] == '-' {
		return false
	}
	return strings.Trim(id, "abcdefghijklmnopqrstuvwxyz0123456789-") == ""
}

func validSum(s string) bool {
	return len(s) == 64 && strings.Trim(s, "0123456789abcdef") == ""
}

func validVer(s string) bool {
	_, ok := parseVer(s)
	return ok
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

// cleanText — текст для показа: без управляющих и невидимых знаков, пробелы схлопнуты, не
// длиннее n знаков (как pack.Clean): чужая строка не ломает окно и тост.
func cleanText(s string, n int) string {
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
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
