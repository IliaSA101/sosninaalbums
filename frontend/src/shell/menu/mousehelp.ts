// Справочные строки без команды: мышь, жесты, местные клавиши элементов (074, design §2 «Уходят из таблицы»).
// Команд тут нет — их клавиши меняются и живут в реестре (base/keys); здесь тексты, что не переназначаются.
// Окно «Горячие клавиши» и справочник F1 показывают эти строки рядом с командами.
import type { FeatureId } from '../../base/prefs/levels'

export interface MouseRow {
  show: string // «Колесо, Shift+колесо, тачпад»
  label: string
  group: string // id группы каталога (keymap.View.groups)
  feature?: FeatureId // строка видна с этого уровня
}

export const MOUSE_HELP: readonly MouseRow[] = [
  // Листание раскладок Пробелом — жест keyup, не команда с клавишей (коротко — раскладка, зажат — панорама).
  { show: 'Пробел (коротко)', label: 'Следующая раскладка разворота', group: 'spreads', feature: 'variants' },
  { show: 'Shift+Пробел (коротко)', label: 'Предыдущая раскладка разворота', group: 'spreads', feature: 'variants' },
  { show: 'Колесо, Shift+колесо, тачпад', label: 'Прокрутить ряд разворотов', group: 'canvas' },
  { show: 'Колесо, Shift+колесо (в увеличении)', label: 'Двигать лист по вертикали и по горизонтали', group: 'canvas', feature: 'zoom' },
  { show: 'Ctrl+колесо, щипок тачпада', label: 'Масштаб к курсору', group: 'canvas', feature: 'zoom' },
  // 095: при переносе — без feature, на всех уровнях (перенос есть всегда).
  { show: 'Колесо при переносе фото', label: 'Листать развороты, не отпуская фото', group: 'canvas' },
  { show: 'Фото или разворот у края ленты', label: 'Лента прокручивается сама', group: 'spreads' },
  // 096: живая перестановка — на всех уровнях, как и прежний перенос разворота.
  { show: 'Перетащить разворот в ленте или в «Все развороты»', label: 'Поменять порядок разворотов; Esc — отмена', group: 'spreads' },
  { show: 'Ctrl+колесо, Alt+колесо над лентой', label: 'Размер миниатюр', group: 'photos' },
  { show: 'Перетащить соседний разворот, Пробел+мышь', label: 'Прокрутить ряд разворотов', group: 'canvas' },
  { show: 'Зажатое колёсико', label: 'Двигать увеличенный разворот', group: 'canvas', feature: 'zoom' },
  { show: 'Двойной щелчок по фону', label: 'Вписать разворот в окно', group: 'canvas', feature: 'zoom' },
  { show: 'Щелчок по масштабу (внизу справа)', label: 'Бегунок масштаба', group: 'canvas', feature: 'zoom' },
  { show: 'Двойной щелчок по масштабу', label: 'Вписать разворот в окно', group: 'canvas', feature: 'zoom' },
  // Спека 015: способ выделения, а не инструмент — без feature, на всех уровнях.
  { show: 'Alt+щелчок', label: 'Выбрать объект под ним; ещё раз — следующий, по кругу', group: 'canvas' },
  { show: 'Двойной щелчок по номеру в ленте', label: 'Название разворота', group: 'spreads', feature: 'names' },
  // 094: простой щелчок по зазору и черте больше не режет и не клеит — только с Ctrl (на Mac ⌘).
  { show: 'Ctrl+щелчок между фото в ленте', label: 'Разрезать разворот (на Mac — ⌘)', group: 'spreads', feature: 'timeline' },
  { show: 'Ctrl+щелчок по черте между разворотами', label: 'Склеить развороты (на Mac — ⌘)', group: 'spreads', feature: 'timeline' },
  // 103 §3: промежуток холста; уровень не проверяется (103-9), как у «+ Новый разворот».
  { show: 'Ctrl+щелчок в промежутке между листами', label: 'Вставить разворот (на Mac — ⌘)', group: 'canvas' },
  { show: 'Двойной щелчок по тексту', label: 'Править текст', group: 'text', feature: 'text' },
  { show: 'Esc / Ctrl+Enter (в тексте)', label: 'Закончить правку текста', group: 'text', feature: 'text' },
  { show: 'Shift (поворот текста)', label: 'Угол кратен 15°', group: 'text', feature: 'textRotate' },
  { show: '«Править» в карточке разворота', label: 'Войти в режим фона', group: 'backdrop', feature: 'bgPhoto' },
  { show: 'Shift (поворот фона)', label: 'Угол кратен 15°; Alt — без привязки', group: 'backdrop', feature: 'bgPhoto' },
  { show: 'Щелчок мимо рамки фона', label: 'Выйти из режима фона', group: 'backdrop', feature: 'bgPhoto' },
  {
    show: 'Перетащить фото или рамку (ручная)',
    label: 'Переносит само место, фото остаётся в нём; в «Авто» перенос фото меняет фото местами',
    group: 'grid',
    feature: 'manual',
  },
  { show: 'Ctrl+перетаскивание фото', label: 'Поменять фото местами на ручном развороте (на Mac — ⌘)', group: 'grid', feature: 'manual' },
  { show: '⇄ в мини-панели (ручная)', label: 'Перетащить фото на другое место: фото поменяются местами', group: 'grid', feature: 'manual' },
  { show: 'Shift (размер места)', label: 'Тянуть угол с сохранением пропорций', group: 'grid', feature: 'manual' },
  { show: 'Alt (перенос места или текста)', label: 'Без привязки к направляющим', group: 'grid', feature: 'manual' },
  {
    show: 'Shift+клик по фото',
    label: 'Добавить к выделению или убрать из него: фото, место, текст (то же с Ctrl) — группа',
    group: 'grid',
    feature: 'groupSelect',
  },
]
