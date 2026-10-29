package share

import "time"

// Kind — состояние ссылки альбома (design §3.4); значения уходят в интерфейс как есть.
type Kind string

const (
	StateNone       Kind = "none"       // записи нет: форма «Выгрузить»
	StateIncomplete Kind = "incomplete" // есть id, публикации нет: «Продолжить»
	StateOK         Kind = "ok"         // опубликована, срок не прошёл, сцена та же
	StateChanged    Kind = "changed"    // опубликована, сцена другая: «Обновить по ссылке»
	StateExpired    Kind = "expired"    // срок прошёл: «Выгрузить заново»
	StateGone       Kind = "gone"       // сервис ответил 404 на действие: ссылку отозвали или она истекла
	StateOther      Kind = "other"      // запись другого сервиса (адрес сменили)
)

// Record — запись о выгрузке альбома (по ней и текущей сцене считается состояние). Поля те же,
// что в shares.json (Т3); prefs её в share не отдаёт — службе сборка из одной в другую.
type Record struct {
	Origin      string    // origin сервиса, куда выгружено
	ID          string    // id альбома на сервисе (после Create)
	URL         string    // ссылка для клиента; пуста, пока не опубликовано
	Title       string    // название при выгрузке
	Password    string    // пароль клиента; пуст — без пароля
	ExpiresAt   time.Time // срок ссылки
	PublishedAt time.Time // когда опубликовано
	Scene       string    // хэш сцены на момент публикации (SceneHash)
	Gone        bool      // сервис ответил 404 на действие (Update, Patch, Revoke)
}

// State — состояние ссылки офлайн: по записи, origin подключённого сервиса, хэшу текущей сцены
// и времени. Сети нет. Порядок проверок: чужой сервис, «нет на сервере», незавершённая
// выгрузка, срок, правка после выгрузки. rec == nil — записи нет.
func State(rec *Record, origin, scene string, now time.Time) Kind {
	switch {
	case rec == nil || rec.ID == "" && rec.URL == "":
		return StateNone
	case rec.Origin != origin:
		return StateOther
	case rec.Gone:
		return StateGone
	case rec.URL == "":
		return StateIncomplete
	case !rec.ExpiresAt.IsZero() && !now.Before(rec.ExpiresAt):
		return StateExpired
	case rec.Scene != scene:
		return StateChanged
	}
	return StateOK
}
