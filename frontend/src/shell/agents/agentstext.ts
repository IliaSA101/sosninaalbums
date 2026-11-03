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
    return `Агент сможет создавать альбомы, собирать развороты и менять раскладки. Каждая правка — один шаг ${keymap.hint('undo') || 'отмены'}; перед первой правкой — резервная копия. Что спрашивать, а что делать сразу — настраивается ниже.`
  },
  ok: 'Разрешить',
}
// «Всегда» у папки или альбома (150, design §7.2; тексты ждут «да» владельца). Места показываются внутри абзаца (строка мест
// с кнопкой «Папка со съёмками…» собирается в окне), поэтому текст разрезан на части вокруг них.
export const ASK_FOLDER = {
  title: 'Добавлять папки по просьбе агента без вопроса?',
  lead: 'Агент сможет сам добавить папку, которую назовёт, — в том числе прочитав её на странице или в файле, — и создать из неё альбом.',
  placesHead: 'Без вопроса — только внутри ваших папок со съёмками:',
  placesNone: 'не выбраны',
  placesTail: '; на этом компьютере и не больше 5000 фото. Иначе, и в первый раз из новой папки, программа спросит.',
  cloud: 'Облачный агент увидит имена файлов, а при «+ картинки» — и фото.',
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

// owner-ux §2.7: вместо неактивного флажка без слов — причина и что делать.
export const EDIT_ELEVATED = 'Программа запущена от администратора — запустите её обычным способом'
export const MUTED_TEXT = 'Вопросы агентов отключены после трёх отказов'
export const MUTED_LABEL = 'Разрешить вопросы'
export const EDIT_MARK = '✎'

// Окно «Разрешения» (150, design §7.1).
export const PERMS_TITLE = 'Разрешения'
export const PERMS_NOTE = 'Когда агент может пользоваться инструментами программы. «Спрашивать» — вопрос в окне программы.'
export const GROUP_READ = 'Только чтение'
export const GROUP_EDIT = 'Правки'
export const GROUP_MIXED = 'разные'
// Строки «только с вопросом» (152, класс Q): выгрузка и открытие; «Всегда» серое, под названием — почему.
export const ASK_ONLY_NOTE = 'Спрашивает всегда'
export const GROUP_OTHER_NOTE = 'Не меняют альбом — групповой выбор их не трогает'
export const MODE_LABELS = { allow: 'Всегда', ask: 'Спрашивать', deny: 'Запретить' } as const
// К причине «Спрашивать» нет у чтения: ограничить его можно уровнем.
export const READ_WHY_TAIL = ' Ограничить чтение можно выше, в «Что видит агент».'
export const PLACES_HEAD = 'Без вопроса — внутри:'
export const PLACES_NONE = 'Без вопроса — нигде: выберите папку со съёмками'
export const PLACE_CHOOSE = 'Папка со съёмками…'
export const PLACES_FORGET = 'Забыть места'
export const placesMore = (n: number): string => ` и ещё ${n}`

// Список мест (153, design §3). Образец — «Последние альбомы» стартового окна: путь, серым справа метка диска, ✕ «Убрать из списка».
export const placesList = (n: number, open: boolean): string => `Список (${n}) ${open ? '▾' : '▸'}`
export const PLACE_FORGET_ONE = 'Убрать из списка'
export const placeForgetLabel = (dir: string): string => `Убрать ${dir} из списка`
export const placeNoDisk = (label: string): string => (label ? `Диска «${label}» нет` : 'Диска нет')
export const PLACE_NOFOLDER = 'Папка не найдена'
export const placeNarrow = (name: string): string => `Новый год спросит снова. ${name} целиком — «${PLACE_CHOOSE}»`
export const placeNarrowRoot = (drive: string): string => `${drive ? `Диск ${drive}` : 'Диск'} целиком — «${PLACE_CHOOSE}»`
export const PLACE_UNBOUND = 'не привязано к диску'
export const placeBind = (here: string): string => (here ? `Привязать к диску «${here}»` : 'Привязать к этому диску')
// Заметка о подъёме мест 0.34 до корня архива (153, 0.35-153-16): висит до «Понятно».
export const liftNote = (to: string, from: string[]): string =>
  `Агент теперь берёт папки без вопроса из ${to} (раньше — из ${from.join(', ')})`
export const LIFT_OK = 'Понятно'
export const LIFT_TOAST = 'Места съёмок для агента расширены'
export const LIFT_SEE = 'Посмотреть'

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
export const ASK_LINK = '(по ссылке)'
// Ссылки вопроса (150): «Не спрашивать» открывает окно на строке инструмента, «Добавить место» — на строке мест.
export const ASK_NEVER_LINK = 'Не спрашивать — Файл ▸ ИИ-агенты (MCP)… ▸ Разрешения'
export const ASK_PLACE_LINK = 'Добавить место — Файл ▸ ИИ-агенты (MCP)… ▸ Разрешения'
// Причина вместо «Не спрашивать», когда у папки «Всегда», а тихо нельзя (Go даёт код в AgentAsk.why).
export const ASK_BAD_HINT = 'Агент предложил папку, которую программа не берёт без вас (системную, скрытую или слишком широкую).'
export const ASK_WHY: Record<string, string> = {
  remote: 'Папка не на этом компьютере',
  big: 'Больше 5000 фото или очень большая папка',
  cloud: 'Часть файлов ещё в облаке',
  shots: 'Папка снимков экрана',
  link: 'Путь идёт через ссылку на другую папку',
  rate: 'Агент уже добавил 3 папки за 10 минут',
  draft: 'Открытый альбом вы правили — заменить его без вас нельзя',
  disk: 'Это другой диск, не тот, что у места',
  vol: 'Не удалось узнать диск',
}
export const ASK_PLACE_WHY = 'Без вопроса — только в ваших папках со съёмками:'
export const PLACES_NONE_SHORT = 'не выбраны'

// Тосты правок агента (150, design §7.3–7.4).
export const OPEN_PREV_LABEL = 'Открыть прежний'
export const SETUP_LABEL = 'Настроить'
export const deniedText = (client: string, titles: string[]): string => `${client}: ${titles.map((t) => `«${t}»`).join(', ')} запрещено`

export const CLOSE_LABEL = 'Закрыть'
export const CANCEL_LABEL = 'Отмена'

export const BADGE_LABEL = 'ИИ'
export const BADGE_HINT = 'Доступ ИИ-агентов к альбому включён — открыть окно'
export const badgeHintLive = (names: string[]): string => `Агент подключён: ${names.join(', ')} — открыть окно доступа`
