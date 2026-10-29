// Тексты окна «ИИ-агенты (MCP)» (143, У1): слова одним местом, как packnet.ts у «Наборов». Подписи окон,
// вопросов и подсказок — из спеки 143-mcp-read.md (сценарии 2–5). Тексты настройки клиентов приходят из Go.

import { keymap } from '../../base/keys/keymap.svelte'

export const AGENTS_TITLE = 'ИИ-агенты (MCP)'
export const AGENTS_MENU = 'ИИ-агенты (MCP)…'

export const OFF_TEXT = 'Агенты ИИ не видят программу.'
export const ON_TEXT = 'Включено'
export const ENABLE_LABEL = 'Разрешить агентам читать альбом…'
export const DISABLE_LABEL = 'Выключить'
export const DISABLE_HINT = 'Закрыть доступ: подключения агентов оборвутся'

// Вопрос включения: «Отмена» слева, «Разрешить» справа (образец — «Обратиться к GitHub?»).
export const ASK_ON = {
  title: 'Разрешить агентам ИИ читать альбом?',
  text: 'Агент увидит устройство альбома: развороты, места, размеры, оформление, наборы. Имена файлов, тексты на листах и сами фото — только если отметите ниже. Облачный агент отправит увиденное своему серверу. Программа не сообщает агенту пути к папкам. Эти настройки не защищают от агента, у которого есть свой доступ к файлам компьютера.',
  ok: 'Разрешить',
}
// Вопрос включения правок (149, design §10): «Отмена» слева, «Разрешить» справа, как у остальных.
export const ASK_EDIT = {
  title: 'Разрешить агентам ИИ менять альбом?',
  // Клавиша отмены — из реестра (на Mac это ⌘Z): слова владельца без «Ctrl+Z» руками, страж keys.guard.
  get text(): string {
    return `Агент сможет создавать альбомы, собирать развороты и менять раскладки. Каждая правка — один шаг ${keymap.hint('undo') || 'отмены'}; перед первой правкой — резервная копия. Агент может предложить папку — программа откроет выбор папки, решаете вы.`
  },
  ok: 'Разрешить',
}
export const ASK_NAMES = {
  title: 'Показать агенту имена файлов и тексты?',
  text: 'Агент увидит имена файлов, папок и альбома, тексты на листах, подписи и метки — в них бывают ФИО детей и фамилии клиентов. Облачный агент отправит их своему серверу.',
  ok: 'Показать',
}
export const ASK_PICS = {
  title: 'Показать агенту картинки?',
  text: 'Агент увидит уменьшенные фото и развороты целиком, вместе с именами файлов и текстами на листах: в подписях бывают ФИО детей, на снимках — лица. Облачный агент отправит всё это своему серверу.',
  ok: 'Показать',
}

export const SEE_TITLE = 'Что видит агент'
export const SEE_BASE =
  'Всегда: устройство альбома — развороты, места, размеры, оформление, наборы. Пути к папкам программа агенту не сообщает.'
export const SEE_NAMES = '+ имена файлов и тексты на листах'
export const SEE_PICS = '+ картинки: уменьшенные фото и развороты'

export const EDIT_TITLE = 'Правки'
export const EDIT_LABEL = 'Разрешить агентам менять альбом'
// owner-ux §2.7: вместо неактивного флажка без слов — причина и что делать.
export const EDIT_ELEVATED = 'Программа запущена от администратора — запустите её обычным способом'
export const MUTED_TEXT = 'Вопросы агентов отключены после трёх отказов'
export const MUTED_LABEL = 'Разрешить вопросы'
export const EDIT_MARK = '✎'

export const SETUP_TITLE = 'Как подключить'
export const SETUP_NOTE = 'В каждом клиенте запускается сама программа с ключом --mcp. Секретов в настройке нет.'
export const COPY_LABEL = 'Копировать'
export const COPIED = 'Настройка скопирована'
export const COPY_FAIL = 'Не удалось скопировать в буфер обмена'
export const SAVE_FILE_LABEL = 'Сохранить файл для Claude Desktop…'
export const SAVE_FILE_HINT = 'Файл расширения для Claude Desktop: откройте его двойным щелчком'
export const PORTABLE_NOTE = 'Переносная версия: после обновления сохраните файл или скопируйте настройку заново.'

export const CLIENTS_NONE = 'Сейчас никто не подключён.'
export const clientsLine = (names: string[]): string => `Подключены: ${names.join(', ')}`

export const LOG_TITLE = 'Журнал'
export const LOG_EMPTY = 'Пока агенты ничего не спрашивали.'
export const LOG_CLEAR = 'Очистить журнал'
export const LOG_CLEAR_HINT = 'Убрать записи журнала в этом окне'
export const todayLine = (n: number): string => `Сегодня запросов: ${n}`

// Вопрос агента (149, AgentAskDialog): кнопки как у AutoAskDialog; текст вопроса приходит из Go.
export const ASK_TIMEOUT = 'Агент не дождался ответа'
export const ASK_HINT = 'Агент предлагает папку'

export const CLOSE_LABEL = 'Закрыть'
export const CANCEL_LABEL = 'Отмена'

export const BADGE_LABEL = 'ИИ'
export const BADGE_HINT = 'Доступ ИИ-агентов к альбому включён — открыть окно'
export const badgeHintLive = (names: string[]): string => `Агент подключён: ${names.join(', ')} — открыть окно доступа`
