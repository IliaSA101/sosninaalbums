package prefs

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Записи выгрузок по ссылке (146): <папка данных>/shares.json. Ключ — путь файла альбома
// (paths.Fold: на Windows и macOS без учёта регистра, как «Последние»). Файл читают как чужой:
// предел размера и числа записей, запись с негодным адресом, id или паролем отбрасывается.
// Токен сервиса здесь не хранится никогда — он в internal/secret.

// Пределы shares.json (design §3.5).
const (
	sharesVersion  = 1
	maxSharesFile  = 1 << 20 // больше — файл как битый
	maxShares      = 1000    // записей; больше — файл как битый, новая запись — ErrSharesFull
	maxShareTitle  = 200     // рун
	maxSharePass   = 256     // байт
	maxShareScene  = 128     // знаков хэша сцены
	shareIDMin     = 22      // id: 22…64 знака (0.32-146-27)
	shareIDMax     = 64
	sharesKeepGone = 30 * 24 * time.Hour // истёкшая запись живёт ещё столько (разбор №29)
)

var (
	// ErrSharesNewer — shares.json записан более новой программой: читаем, не пишем.
	ErrSharesNewer = errors.New("shares.json записан более новой версией программы")
	// ErrSharesUnread — shares.json есть, но не открылся или не отложен: записи не меняем,
	// чтобы не затереть исправный файл (подробности — в журнале, без путей).
	ErrSharesUnread = errors.New("не удалось прочитать shares.json")
	// ErrSharesFull — записей или файла больше предела.
	ErrSharesFull = errors.New("слишком много записей о выгрузках")
	// ErrShareRecord — запись не годится (адрес, id или пароль не прошли проверку).
	ErrShareRecord = errors.New("запись о выгрузке не годится")
	// ErrSharePath — путь альбома не годится в ключ записи.
	ErrSharePath = errors.New("путь альбома не годится для записи о выгрузке")
)

// ShareRecord — запись об одной выгрузке; поля те же, что у share.Record. Gone — сервис
// ответил 404 на действие по записи.
type ShareRecord struct {
	Origin      string    `json:"origin"`
	ID          string    `json:"id"`
	URL         string    `json:"url,omitempty"` // ссылка для клиента; пуста, пока не опубликовано
	Title       string    `json:"title,omitempty"`
	Password    string    `json:"password,omitempty"` // открытым текстом: «скопировать снова» (design §6 п. 6)
	ExpiresAt   time.Time `json:"expires_at,omitzero"`
	PublishedAt time.Time `json:"published_at,omitzero"`
	Scene       string    `json:"scene,omitempty"` // хэш сцены на момент публикации
	Gone        bool      `json:"gone,omitempty"`
	// UID — отпечаток проекта, выгруженного по ссылке (model/uid.go, T4-9): запись действует,
	// только пока у файла по пути тот же отпечаток. Негодный или пустой — запись чужая.
	UID string `json:"uid,omitempty"`
}

// ShareRules — проверки адресов, которыми prefs не владеет: их даёт internal/share
// (ServiceURL, CheckLink) через службу. prefs от share не зависит (0.32-146-T3-1). Пустая
// функция — запись не пройдёт: без правил ничего не принимается.
type ShareRules struct {
	// ServiceURL приводит адрес сервиса к origin; негодный — ошибка (share.ServiceURL).
	ServiceURL func(raw string) (string, error)
	// CheckLink — ссылка для клиента годна (share.CheckLink).
	CheckLink func(raw string) error
}

// origin — origin сервиса по правилам; пусто — не годится.
func (r ShareRules) origin(raw string) string {
	if r.ServiceURL == nil || raw == "" {
		return ""
	}
	o, err := r.ServiceURL(raw)
	if err != nil {
		return ""
	}
	return o
}

func (r ShareRules) link(raw string) bool {
	return r.CheckLink != nil && r.CheckLink(raw) == nil
}

// goodShareID — id альбома на сервисе: 22…64 знака [A-Za-z0-9_-], первый не «-».
func goodShareID(id string) bool {
	if len(id) < shareIDMin || len(id) > shareIDMax || id[0] == '-' {
		return false
	}
	for i := 0; i < len(id); i++ {
		switch c := id[i]; {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}

// cleanShareTitle — название как его увидит клиент: корректный UTF-8, без управляющих и
// невидимых знаков (bidi, нулевая ширина), не длиннее maxShareTitle рун.
func cleanShareTitle(s string) string {
	s = strings.ToValidUTF8(s, "")
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxShareTitle {
		s = string([]rune(s)[:maxShareTitle])
	}
	return s
}

// goodSharePassword — пароль клиента: ≤ maxSharePass байт, без управляющих знаков.
func goodSharePassword(p string) bool {
	if len(p) > maxSharePass || !utf8.ValidString(p) {
		return false
	}
	for _, r := range p {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// cleanShareScene — хэш сцены: печатный ASCII до maxShareScene знаков; иное — пусто (состояние
// покажет «изменён», а не подставит чужое).
func cleanShareScene(s string) string {
	if len(s) > maxShareScene {
		return ""
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; c <= ' ' || c >= 0x7f {
			return ""
		}
	}
	return s
}

// clean проверяет запись по правилам; ok == false — запись отбросить. Origin приводится к
// виду правил (ServiceURL); названия и хэш сцены чистятся, а не отбрасываются.
func (r ShareRules) clean(rec ShareRecord) (ShareRecord, bool) {
	o := r.origin(rec.Origin)
	if o == "" || !goodShareID(rec.ID) || rec.URL != "" && !r.link(rec.URL) || !goodSharePassword(rec.Password) {
		return ShareRecord{}, false
	}
	rec.Origin = o
	rec.Title = cleanShareTitle(rec.Title)
	rec.Scene = cleanShareScene(rec.Scene)
	if !model.GoodUID(rec.UID) {
		rec.UID = "" // запись остаётся, но ни к одному альбому не подходит (get считает её чужой)
	}
	rec.ExpiresAt, rec.PublishedAt = rec.ExpiresAt.UTC(), rec.PublishedAt.UTC()
	return rec, true
}
