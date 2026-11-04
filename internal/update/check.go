package update

import (
	"context"
	"errors"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// Ошибки для тоста: по-русски, без адресов и путей. Сетевые (нет связи, тайм-аут, код ответа)
// остаются текстами fetch — они не про библиотеку.
var (
	ErrNoFile      = errors.New("файл обновления на GitHub не найден — скачайте установщик со страницы выпуска")
	ErrCancelled   = errors.New("скачивание отменено")
	ErrDiskWrite   = errors.New("обновление не записывается на диск")
	ErrAddress     = errors.New("адрес обновления не из разрешённых — обновление отменено")
	ErrNoInstaller = errors.New("для этой системы установщика нет — скачайте программу со страницы выпуска")
)

// humanError — ошибка словами обновления (Error — для тоста) и исходная причина (для log.txt):
// ErrVerify скрывает, что именно не сошлось. Is видит обе.
type humanError struct{ human, cause error }

func (e *humanError) Error() string   { return e.human.Error() }
func (e *humanError) Unwrap() []error { return []error{e.human, e.cause} }

// Detail — текст ошибки для log.txt: слова обновления и причина за ними. Адресов и путей в ошибках
// fetch нет (коды ответа и готовые фразы), поэтому причину писать можно. Тексты сторожевых ошибок
// fetch говорят о библиотеке и наборах — вместо них нейтральная причина (reasonOf).
func Detail(err error) string {
	var h *humanError
	if errors.As(err, &h) {
		return h.human.Error() + " (" + reasonOf(h.cause) + ")"
	}
	return err.Error()
}

// reasonOf — причина для скобки в log.txt: для сторожевых ошибок fetch свои слова обновления,
// для остальных (проверка файла: длина, sha256) — текст как есть.
func reasonOf(cause error) string {
	for _, r := range []struct {
		is   error
		text string
	}{
		{fetch.ErrNotFound, "ответ 404"},
		{fetch.ErrSignature, "подпись не сошлась"},
		{fetch.ErrCatalog, "манифест не читается"},
		{fetch.ErrNewer, "манифест новее программы"},
		{fetch.ErrBroken, "файл скачался с ошибкой"},
		{fetch.ErrAddress, "адрес не из разрешённых"},
		{fetch.ErrCancelled, "отмена пользователем"},
		{fetch.ErrDisk, "запись на диск не удалась"},
	} {
		if errors.Is(cause, r.is) {
			return r.text
		}
	}
	return cause.Error()
}

// humanize — ошибка fetch словами обновления: тексты fetch говорят о «списке наборов».
func humanize(err error) error {
	var human error
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fetch.ErrSignature), errors.Is(err, fetch.ErrCatalog), errors.Is(err, fetch.ErrBroken),
		errors.Is(err, fetch.ErrNewer):
		human = ErrVerify
	case errors.Is(err, fetch.ErrAddress):
		human = ErrAddress
	case errors.Is(err, fetch.ErrNotFound):
		human = ErrNoFile
	case errors.Is(err, fetch.ErrCancelled):
		human = ErrCancelled
	case errors.Is(err, fetch.ErrDisk):
		human = ErrDiskWrite
	default:
		return err
	}
	return &humanError{human: human, cause: err}
}

// Offer — предложенная версия: новее текущей, из проверенного манифеста.
type Offer struct {
	Version string
	Date    string
	Notes   []string
	// Setup — установщик Windows; nil — ставить вручную: выпуска для Windows нет в манифесте или
	// minAuto новее текущей версии (между ними менялось то, что автообновление не переживёт).
	Setup *File
	// Page — только страница выпуска (Setup пуст по этой причине).
	Page bool
}

// Result — итог проверки: Offer nil — установлена последняя. Seen — наибольшая версия, виденная
// в подписанных файлах, включая этот (вызывающий пишет её в config.update.seen). Rollback —
// подписанный файл старее виденного: старый выпуск подан повторно (чужим сервером или из
// кэша), он «последняя версия», не предложение.
type Result struct {
	Offer    *Offer
	Seen     string
	Rollback bool
}

// Check — проверка обновления: манифест выпуска (конверт ≤ 16 КБ, подпись до разбора) →
// Parse → сравнение с cur. seen — наибольшая виденная ранее версия ("" — не было): файл
// ниже неё — Rollback. Предлагается только строго более новая версия, чем cur (откат
// запрещён); cur не версия (dev) — предложений нет.
func Check(ctx context.Context, c *Client, cur, seen string) (Result, error) {
	body, err := c.fc.Signed(ctx, c.src.CatalogURL, maxEnvelope)
	if err != nil {
		return Result{Seen: validOr(seen)}, humanize(err)
	}
	m, err := Parse(body)
	if err != nil {
		return Result{Seen: validOr(seen)}, err
	}
	res := Result{Seen: validOr(seen)}
	if Newer(res.Seen, m.Version) {
		res.Rollback = true
		return res, nil
	}
	res.Seen = m.Version
	if !Newer(m.Version, cur) {
		return res, nil
	}
	o := &Offer{Version: m.Version, Date: m.Date, Notes: m.Notes, Setup: m.Setup}
	if o.Setup == nil || m.MinAuto != "" && Newer(m.MinAuto, cur) {
		o.Setup, o.Page = nil, true
	}
	res.Offer = o
	return res, nil
}

// validOr — s, если это версия, иначе "" (config.json правил человек).
func validOr(s string) string {
	if _, ok := ParseVersion(s); ok {
		return s
	}
	return ""
}
