// Тексты окна «Отправить по ссылке» (146, design §5): слова одним местом. Готовые строки про дату, ссылку и ошибки
// приходят из Go (ShareLink.until/published, тексты отказов); здесь — только то, что пишет интерфейс.

export const SHARE_TITLE = 'Отправить по ссылке'
export const SHARE_MENU = 'Отправить по ссылке…'
// Тост, если команда пришла не из меню (там пункт серый): та же причина, что у «Экспорта» без разворотов.
export const SHARE_EMPTY = 'Отправлять нечего: в альбоме нет разворотов'
export const SHARE_SHEET = 'Лист-виньетка не выгружается по ссылке' // как причина Go (album.block)

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
export const passwordLine = (p: string): string => (p ? `Пароль: ${p}` : NO_PASSWORD)
export const termLine = (until: string): string => `Срок: ${until}`
export const otherLine = (host: string): string => `Ссылка выгружена на ${host}. Чтобы править её, подключите ${host}.`

// Причины вместо формы
export const INCOMPLETE_LINE = 'Выгрузка не закончена'
export const CONTINUE_LABEL = 'Продолжить'
export const EXPIRED_LINE = 'Срок истёк — альбом удалён с сервера'
export const GONE_LINE = 'Ссылку отозвали или она истекла — альбом удалён с сервера'
export const AGAIN_LABEL = 'Выгрузить заново'

// Тосты
export const DONE_TOAST = 'Ссылка готова'
export const STOPPED_TOAST = 'Выгрузка остановлена'
export const failedToast = (err: string): string => `Выгрузка не удалась: ${err}`
export const REVOKED_TOAST = 'Ссылка отозвана'
export const COPIED = 'Текст для клиента скопирован'
export const COPY_FAIL = 'Не удалось скопировать в буфер обмена'
