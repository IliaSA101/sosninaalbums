package agents

import "errors"

// agentErr — единственный вид ошибки, текст которой уходит агенту: фиксированные строки без путей
// и без данных альбома (design §3 «Одна точка ответа»). Всё остальное обёртка заменяет на
// errInternal, а подробности пишет в log.txt без путей.
type agentErr struct{ text string }

func (e *agentErr) Error() string { return e.text }

func newAgentErr(text string) *agentErr { return &agentErr{text: text} }

// Список ошибок наружу. Текст читает модель и пересказывает человеку: по-русски, со словами,
// где что включить. errInternal («внутренняя ошибка программы») — в sessions.go.
var (
	errNoAlbum   = newAgentErr("альбом не открыт — откройте или создайте альбом в Sosnina Albums")
	errNoSpread  = newAgentErr("нет такого разворота — номер с 1, «cover» (обложка) или id из list_spreads")
	errNoPhoto   = newAgentErr("нет такого фото — id берите из list_photos")
	errNoImages  = newAgentErr("Картинки агенту выключены: Файл ▸ ИИ-агенты ▸ «+ картинки»")
	errTimeout   = newAgentErr("время вышло — попробуйте ещё раз или запросите меньше")
	errCallBusy  = newAgentErr("занято: идут другие запросы — повторите через секунду")
	errTooBig    = newAgentErr("ответ слишком велик — уменьшите limit")
	errTooBigOne = newAgentErr("ответ слишком велик для одного вызова — запросите другой разворот или другое фото")
	errBadKind   = newAgentErr("inAlbum есть только у фонов, стикеров, рамок и макетов (kind: bg, sticker, frame, look)")
	errPhotoGone = newAgentErr("файл этого фото недоступен (нет на месте или не читается) — состояние видно в list_photos, поле status")
)

// badArgs — отказ схемы входа: в тексте только то, что прислал сам агент (имена полей, значения).
func badArgs(detail string) *agentErr {
	r := []rune(detail)
	if len(r) > 300 {
		detail = string(r[:300]) + "…"
	}
	return newAgentErr("неверные аргументы вызова: " + detail)
}

// toAgent приводит ошибку к тому, что можно показать агенту; прочее — errInternal. Второй
// результат — исходная ошибка для log.txt (nil, если она своя).
func toAgent(err error) (out *agentErr, orig error) {
	var ae *agentErr
	if errors.As(err, &ae) {
		return ae, nil
	}
	return errInternal, err
}
