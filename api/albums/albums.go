// Package albums — открытый контракт /v1 сервиса листалок Sosnina Albums: типы манифеста и запросов,
// проверки (одни и те же в программе и в сервисе), Go-клиент. Источник истины — v1/openapi.yaml;
// поддельный сервер и набор проверок любой реализации — в подпакете albumstest.
//
// Модуль без внешних зависимостей (yaml нужен только тесту сверки с OpenAPI) и под лицензией
// Apache-2.0: его подключает и программа, и сервис.
package albums

import "time"

// Версия схемы манифеста и допустимые значения полей (design §2.2).
const (
	SchemaVersion = 1
	TypeJPEG      = "image/jpeg"
	TypeWebP      = "image/webp"
	RoleFront     = "front" // лицевая сторона обложки, только n = 1
	RoleBack      = "back"  // задняя сторона обложки, только n = K
	RolePage      = "page"
)

// Пределы формата, не зависящие от сервиса (они же maxLength/minimum/maximum в OpenAPI).
const (
	MaxTitle        = 200  // знаков в названии
	MaxTitleBytes   = 800  // и в байтах
	MaxGenerator    = 100  // знаков в generator
	MinPageMM       = 10   // page_mm.w/h, мм
	MaxPageMM       = 2000 // верхняя граница page_mm, мм
	MaxSide         = 20000
	MinPassword     = 4 // знаков в пароле
	MaxPassword     = 64
	MaxPasswordSize = 256 // и в байтах
	MinID           = 22  // знаков в id: 128 бит в base64url
	MaxID           = 64
	MinToken        = 16 // знаков в токене
	MaxToken        = 512
	MaxBase         = 200 // знаков в адресе сервиса
	MaxURL          = 500 // знаков в ссылке из ответа
	MaxBody         = 1 << 20
)

// Manifest — публичная часть книги: то, что сервис отдаёт просмотрщику как manifest.json. Пароль и
// срок в него не входят по устройству типа — они в CreateRequest.
type Manifest struct {
	Schema    int    `json:"schema"`
	Generator string `json:"generator"`
	Title     string `json:"title"`
	Book      Book   `json:"book"`
	Pages     []Page `json:"pages"`
}

// Book — устройство книги: есть ли обложка и размер страницы после обреза, мм.
type Book struct {
	Cover  bool `json:"cover"`
	PageMM Size `json:"page_mm"`
}

// Size — ширина и высота, мм.
type Size struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Page — страница по порядку книги. Белая (Blank): файла нет, type/sha256/bytes/w/h пусты.
// File в manifest.json пишет сервис; на входе от клиента он обязан быть пуст (Validate, разбор С7).
type Page struct {
	N      int    `json:"n"`
	Role   string `json:"role"`
	Blank  bool   `json:"blank,omitempty"`
	File   string `json:"file,omitempty"`
	Type   string `json:"type,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
	Bytes  int64  `json:"bytes,omitempty"`
	W      int    `json:"w,omitempty"`
	H      int    `json:"h,omitempty"`
}

// Limits — пределы сервиса (GET /v1/health → limits). Нули в запросе клиента — DefaultLimits.
type Limits struct {
	PageBytes  int64 `json:"page_bytes"`
	Pages      int   `json:"pages"`
	AlbumBytes int64 `json:"album_bytes"`
	TitleLen   int   `json:"title_len"`
	MaxDays    int   `json:"max_days"`
}

// DefaultLimits — пределы design §2.3, пока сервис не сказал свои.
func DefaultLimits() Limits {
	return Limits{PageBytes: 15 << 20, Pages: 300, AlbumBytes: 1 << 30, TitleLen: MaxTitle, MaxDays: 365}
}

// OrDefault подставляет DefaultLimits вместо нулевых и отрицательных значений; название длиннее
// MaxTitle зажимается: формат не пускает больше, что бы ни ответил сервис.
func (l Limits) OrDefault() Limits {
	d := DefaultLimits()
	if l.PageBytes <= 0 {
		l.PageBytes = d.PageBytes
	}
	if l.Pages <= 0 {
		l.Pages = d.Pages
	}
	if l.AlbumBytes <= 0 {
		l.AlbumBytes = d.AlbumBytes
	}
	if l.TitleLen <= 0 || l.TitleLen > MaxTitle {
		l.TitleLen = d.TitleLen
	}
	if l.MaxDays <= 0 {
		l.MaxDays = d.MaxDays
	}
	return l
}

// CreateRequest — тело POST /v1/albums: манифест плюс то, что клиенту видеть не нужно.
type CreateRequest struct {
	Manifest  Manifest  `json:"manifest"`
	ExpiresAt time.Time `json:"expires_at"`
	Password  string    `json:"password,omitempty"`
}

// UpdateRequest — тело PUT /v1/albums/{id}: новая редакция манифеста того же альбома.
type UpdateRequest struct {
	Manifest Manifest `json:"manifest"`
}

// Patch — тело PATCH /v1/albums/{id}. Nil — не менять; Password: "" — снять пароль.
type Patch struct {
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	Password       *string    `json:"password,omitempty"`
	RevokeSessions bool       `json:"revoke_sessions,omitempty"`
}

// Created — ответ 201 на POST /v1/albums: ссылка не работает до Publish.
type Created struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	Missing   []int     `json:"missing"`
}

// Updated — ответ 200 на PUT /v1/albums/{id}: какие страницы новой редакции сервису ещё нужны.
type Updated struct {
	Missing []int `json:"missing"`
}

// Published — ответ 200 на POST /v1/albums/{id}/publish.
type Published struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Album — альбом в списке и ответ PATCH.
type Album struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	ExpiresAt   time.Time `json:"expires_at"`
	HasPassword bool      `json:"has_password"`
	Published   bool      `json:"published"`
	Pages       int       `json:"pages"`
}

// AlbumList — ответ GET /v1/albums.
type AlbumList struct {
	Albums []Album `json:"albums"`
}

// Health — ответ GET /v1/health (без токена).
type Health struct {
	API    string `json:"api"`
	Limits Limits `json:"limits"`
}

// Error — тело ошибки {"error": {...}}. Missing — только у 409 pages_missing.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Missing []int  `json:"missing,omitempty"`
}

// ErrorBody — обёртка {"error": Error}.
type ErrorBody struct {
	Error Error `json:"error"`
}

// Коды ошибок (поле error.code).
const (
	CodeBadManifest  = "bad_manifest"  // 400
	CodeBadPage      = "bad_page"      // 400: номер, тип или сигнатура страницы
	CodeUnauthorized = "unauthorized"  // 401
	CodeNotFound     = "not_found"     // 404: нет, истёк, отозван, чужой — одинаково
	CodeHashMismatch = "hash_mismatch" // 409
	CodePagesMissing = "pages_missing" // 409
	CodeTooLarge     = "too_large"     // 413
	CodeRateLimited  = "rate_limited"  // 429
	CodeInternal     = "internal"      // 500
)
