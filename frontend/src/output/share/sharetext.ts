// Тексты окна «Отправить по ссылке» (146, design §5): слова одним местом. Готовые строки про дату, ссылку и ошибки
// приходят из Go (ShareLink.until/published, тексты отказов); здесь — только то, что пишет интерфейс.

export const SHARE_TITLE = 'Отправить по ссылке'
export const SHARE_MENU = 'Отправить по ссылке…'
// Тост, если команда пришла не из меню (там пункт серый): та же причина, что у «Экспорта» без разворотов.
export const SHARE_EMPTY = 'Отправлять нечего: в альбоме нет разворотов'
export const SHARE_SHEET = 'Лист виньетки нельзя отправить по ссылке' // дословно причина Go (album.block)

// Подключение
export const URL_LABEL = 'Адрес'
export const URL_PLACEHOLDER = 'https://…'
export const TOKEN_LABEL = 'Токен'
export const CONNECT_LABEL = 'Подключить'
export const CONNECTING_LABEL = 'Подключаем…'
export const CONNECT_HINT =
  'Адрес и токен даёт владелец сервиса листалок. Токен программа сохранит в хранилище паролей системы, если оно есть.'
export const SESSION_NOTE = 'Токен не сохранится — в системе нет хранилища паролей, после перезапуска введите снова'
export const FORGET_LABEL = 'Отключить'

// Строка 152-ФЗ (0.32-146-28): до подключения вместо «сервер <хост>» — «этот сервер» (0.32-146-T5-3).
export const dataHint = (host: string): string =>
  `Страницы альбома и название уходят на ${host ? 'сервер ' + host : 'этот сервер'}. Их увидит любой, у кого есть ссылка (и пароль, если задан). Согласие клиента на показ — по вашему договору.`

export const serviceLine = (host: string): string => `Сервис: ${host}`

// Форма
export const NAME_LABEL = 'Название'
export const PASSWORD_LABEL = 'Пароль'
export const PASSWORD_PLACEHOLDER = 'Без пароля'
export const INVENT_LABEL = 'Придумать'
export const TERM_LABEL = 'Срок'
export const CANCEL_LABEL = 'Отмена'
export const CLOSE_LABEL = 'Закрыть'
export const START_LABEL = 'Выгрузить'
export const SAVE_LABEL = 'Сохранить…'
export const WAIT_LINE = 'Программа задала вопрос в отдельном окне — ответьте на него, чтобы продолжить'
export const BUSY_OTHER = 'Идёт выгрузка другого альбома — дождитесь её конца'

// Ход
export const RUN_LABEL = 'Ход выгрузки'
export const RUN_HINT = 'Окно можно закрыть — выгрузка продолжится.'
export const STOP_LABEL = 'Остановить'
export const STOPPING_LABEL = 'Останавливаем…'
export const PUBLISHING = 'Публикуем…'
export const PREPARING = 'Готовим…'

// Ссылка
export const COPY_CLIENT_LABEL = 'Скопировать для клиента'
export const OPEN_LABEL = 'Открыть'
export const NO_PASSWORD = 'Без пароля'
export const CHANGE_LABEL = 'Изменить'
export const SAVE_PASSWORD_LABEL = 'Сохранить'
export const EXTEND_LABEL = 'Продлить'
export const EXTEND_NOTE = 'Срок считается от сегодня.'
export const REVOKE_LABEL = 'Отозвать ссылку'
export const CHANGED_LINE = 'Альбом изменён после выгрузки'
export const UPDATE_LABEL = 'Обновить по ссылке'
export const PASSWORD_ELSEWHERE = 'Пароль задан на другом компьютере'
// Пароль не известен этому компьютеру (альбом найден на сервисе, 167): сервис пароль не отдаёт; «Изменить» задаёт новый.
export const passwordLine = (p: string, unknown = false): string => (p ? `Пароль: ${p}` : unknown ? PASSWORD_ELSEWHERE : NO_PASSWORD)
export const termLine = (until: string): string => `Срок: ${until}`
export const otherLine = (host: string): string => `Ссылка выгружена на ${host}. Чтобы править её, подключите ${host}.`

// Причины вместо формы
export const INCOMPLETE_LINE = 'Выгрузка не закончена'
export const CONTINUE_LABEL = 'Продолжить'
export const EXPIRED_LINE = 'Срок истёк — альбом удалён с сервера'
export const GONE_LINE = 'Ссылку отозвали или она истекла — альбом удалён с сервера'
export const AGAIN_LABEL = 'Выгрузить заново'

// Альбом уже выгружали с другого пути или компьютера (167): вопрос «обновить или как новый» (экран moved)
export const MOVED_ASK = 'Обновить этот альбом или выгрузить как новый?'
// Название прежней выгрузки отдаёт Go (ShareLink.publishedTitle): у чужой записи title пуст (167-12). Нет названия — текст без имени.
export const movedUpdateNote = (title: string): string =>
  `Обновить — страницы альбома${title ? ` «${title}»` : ''} у клиента заменятся страницами этого файла.`
export const MOVED_NEW_NOTE =
  'Как новый — прежняя ссылка останется у прежнего файла; если его больше нет, программа не сможет её показать и отозвать.'
export const NEW_ALBUM_LABEL = 'Выгрузить как новый'
export const UPDATE_ALBUM_LABEL = 'Обновить этот альбом'

// Поиск прежнего альбома на сервисе (167)
export const FIND_LINE = 'Альбом уже выгружали — ищем его на сервисе…'
export const FIND_LOST_LINE = 'Прежний альбом на сервисе не найден — будет новая ссылка'
export const FIND_NEW_LINE = 'Будет новая ссылка'
export const RETRY_LABEL = 'Повторить'

// Тосты
export const DONE_TOAST = 'Ссылка готова'
export const STOPPED_TOAST = 'Выгрузка остановлена'
export const failedToast = (err: string): string => `Выгрузка не удалась: ${err}`
export const REVOKED_TOAST = 'Ссылка отозвана'
// После «Изменить» и «Продлить» (159-Б): until уже с «до» из Go, второго «до» не добавляем.
export const PASSWORD_SET_TOAST = 'Пароль задан — альбом открывается только по нему'
export const PASSWORD_CHANGED_TOAST = 'Пароль изменён — прежние входы клиента сброшены'
export const PASSWORD_REMOVED_TOAST = 'Пароль снят — альбом открывается по ссылке без пароля'
export const termToast = (until: string): string => `Срок ссылки — ${until}`
export const COPIED = 'Текст для клиента скопирован'
export const COPY_FAIL = 'Не удалось скопировать в буфер обмена'
