package sharer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
)

// errText — текст ошибки для пользователя по её виду. Тексты сервиса (Message) и адреса не
// показываем и не пишем: они могли бы нести id или токен (политика журнала, design §6 п. 1).
func errText(err error) string {
	var api *albums.APIError
	var ue userError
	var ve *albums.ValidationError
	switch {
	case errors.As(err, &ue):
		return string(ue)
	case errors.Is(err, context.Canceled):
		return "Выгрузка остановлена"
	case errors.Is(err, albums.ErrToken):
		return "Токен не подошёл — подключите сервис заново"
	case errors.Is(err, albums.ErrGone):
		return "Ссылки больше нет на сервере: истёк срок или её отозвали"
	case errors.Is(err, albums.ErrLimit):
		return "Сервис не принимает альбом такого размера"
	case errors.Is(err, albums.ErrBusy):
		return "Сервис просит подождать — попробуйте позже"
	case errors.Is(err, albums.ErrRedirect):
		return "Сервис перенаправляет запрос — проверьте адрес"
	case errors.Is(err, albums.ErrResponse):
		return "Сервис ответил непонятно"
	case errors.Is(err, albums.ErrTooBig):
		return "Ответ сервиса больше 1 МБ — это не тот сервис"
	case errors.As(err, &api):
		switch {
		case api.Code == albums.CodeHashMismatch:
			return "Страница дошла до сервиса повреждённой — повторите выгрузку"
		case api.Status >= 500:
			return fmt.Sprintf("Сервис не отвечает как надо (ошибка %d) — попробуйте позже", api.Status)
		}
		return fmt.Sprintf("Сервис не принял запрос (ошибка %d)", api.Status)
	case errors.As(err, &ve):
		return "Альбом не прошёл проверку: " + ve.Reason
	case errors.Is(err, context.DeadlineExceeded):
		return "Сервис не ответил вовремя"
	}
	var ne *albums.NetError
	if errors.As(err, &ne) {
		return "Нет связи с сервисом — проверьте адрес и интернет"
	}
	return err.Error() // ошибки подготовки страниц и манифеста: русский текст без секретов
}

// noServiceText — 404 на запросы не по id альбома (health, список, создание): это не «ссылки
// больше нет», а адрес без сервиса листалок.
const noServiceText = "По этому адресу нет сервиса листалок (ошибка 404) — проверьте адрес"

// noService заменяет 404 запроса не по id альбома понятным отказом; без ErrGone в цепочке, чтобы
// живая ссылка не получила пометку «нет на сервере». Прочие ошибки — как есть.
func noService(err error) error {
	if errors.Is(err, albums.ErrGone) {
		return fail(noServiceText)
	}
	return err
}

// logFail пишет в журнал только фазу и вид сбоя: статус HTTP и код, «сеть».
// Ни текста ошибки (в нём могут быть адрес и id), ни Message сервиса (политика журнала).
// Остановка — не сбой: её не пишем (по строке на страницу, как у экспорта, где остановка молчит);
// итог одной строкой пишет runShare.
func logFail(phase string, err error) {
	var api *albums.APIError
	switch {
	case errors.Is(err, context.Canceled):
		return
	case errors.As(err, &api):
		log.Printf("выгрузка: %s: ответ сервиса %d %s", phase, api.Status, api.Code)
	case errors.Is(err, albums.ErrResponse), errors.Is(err, albums.ErrTooBig), errors.Is(err, albums.ErrRedirect):
		log.Printf("выгрузка: %s: ответ не по контракту", phase)
	default:
		var ne *albums.NetError
		if errors.As(err, &ne) {
			log.Printf("выгрузка: %s: нет связи", phase)
		} else {
			log.Printf("выгрузка: %s: ошибка", phase)
		}
	}
}

// Повторы страницы (design §3.3 п. 6): до retries повторов; 429 ждёт Retry-After, но не дольше
// maxRetryAfter, иначе и 5xx с сетью — 2/4/8 с.
const (
	retries       = 3
	maxRetryAfter = 60 * time.Second
)

// retryDelay — через сколько повторить страницу после ошибки err на попытке attempt (с нуля);
// false — повтор не поможет (401, 404, 409 hash_mismatch, 413, ответ не по контракту, остановка).
func retryDelay(err error, attempt int) (time.Duration, bool) {
	if attempt >= retries || errors.Is(err, context.Canceled) {
		return 0, false
	}
	backoff := time.Duration(2<<attempt) * time.Second
	var api *albums.APIError
	if errors.As(err, &api) {
		switch {
		case api.Status == http.StatusTooManyRequests:
			if d := api.RetryAfter; d > 0 {
				return min(d, maxRetryAfter), true
			}
			return backoff, true
		case api.Status >= 500:
			return backoff, true
		}
		return 0, false
	}
	var ne *albums.NetError
	if errors.As(err, &ne) {
		return backoff, true
	}
	return 0, false
}
