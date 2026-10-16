// Единая таблица клавиш (спека 008, design §6.2): keyAction перебирает её, справка покажет её же.
// Порядок строк = приоритет совпадения. Строка без combos или без action — только справка.
import type { Zone } from '../../editor/places/groupsel'
import type { FeatureId } from '../../base/prefs/levels'

export const KEY_ACTIONS = [
  'undo',
  'redo',
  'save',
  'saveAs',
  'new',
  'open',
  'export',
  // Линейки (Ctrl+R, «Полный»). Ниже уровнем Ctrl+R достаётся reload: в окне Wails он перезагрузил бы
  // страницу и потерял состояние, поэтому гасится.
  'toggleRulers',
  'reload',
  'selectAll',
  // Ctrl+A на холсте: все фото, места и тексты разворота (по зоне последнего нажатия).
  'selectSpread',
  'toggleBig',
  'toggleDetails',
  'toggleSafe',
  'prevSpread',
  'nextSpread',
  'removeCell',
  'prevLayout',
  'nextLayout',
  'addSelected',
  'closeGallery',
  'clearSelection',
  // Сдвиг места стрелками (сетка, место выделено): шаг берёт обработчик (placesel.nudge).
  'nudgePlace',
  // Порядок слоёв выделенного (Ctrl+] / Ctrl+[, с Shift — на край), «Ручная».
  'layerFront',
  'layerUp',
  'layerDown',
  'layerBack',
  // Холст (спека 008): Пробел зажат — панорама; коротко без панорамы — раскладка (не у сетки).
  'spaceDown',
  'fitZoom',
  // Отметки выделенных фото ленты (спека 004): звёзды, шаг, цвет, флажок; цифру/знак берёт обработчик из e.key.
  'markRating',
  'markStep',
  'markLabel',
  'markPick',
  'markReject',
  'markUnflag',
  'zoomIn',
  'zoomOut',
  'help',
  // Справочник функций (спека 033): F1; «?» остался за справкой по клавишам.
  'guide',
  // Режим фона (спека 009, design-backdrop §6.3): выход, рамка стрелками, убрать фото фона.
  'bgExit',
  'bgNudge',
  'bgRemove',
] as const

export type KeyAction = (typeof KEY_ACTIONS)[number]

export interface KeyEvent {
  key: string
  ctrlKey: boolean
  metaKey: boolean
  altKey: boolean
  shiftKey: boolean
  repeat: boolean
}

export interface KeyContext {
  inTextField: boolean // фокус в поле ввода текста/чисел: хоткеи глушим
  onCheckbox: boolean // пробел на флажке переключает флажок
  galleryOpen: boolean
  hasSelection: boolean // есть выделенные видимые фото
  gridSpread?: boolean // текущий разворот — «Ручная» (Пробел у неё раскладки не листает — решает App при отпускании)
  placeSelected?: boolean // выделено фото, место или текст группы текущего разворота
  stickerSelected?: boolean // выделены только стикеры (048): стрелки двигают их на любом листе, и на «Авто»
  layerable?: boolean // «Ручная» и выделено место, фото, текст или стикер; «Авто» — стикер (с текстом или без) без мест: слои двигать есть что
  zone?: Zone // где было последнее нажатие мыши; не задано — библиотека (прежнее поведение)
  bgMode?: boolean // включён режим фона: Esc, стрелки и Del — его
  modal?: boolean // открыто окно вопроса или настроек: Ctrl+N/O/E под ним не действуют
  big?: boolean // крупный просмотр: ленты не видно, отметки клавишами молчат
  can?: (feature: FeatureId) => boolean // уровень интерфейса (U1): строка с чужой функцией молчит; не задано — всё доступно
}

export type Group = 'album' | 'spreads' | 'canvas' | 'photos' | 'text' | 'grid' | 'backdrop' | 'view'

// keys — e.key в нижнем регистре, русская раскладка — теми же строками ('z', 'я').
// ctrl: true — нужен Ctrl/Cmd; иначе модификатор не проверяется (как стрелки и Delete раньше).
// shift: true/false — Shift обязан быть/не быть; не задано — не важен.
// plain — без Ctrl/Cmd/Alt и без автоповтора (режимы вида: удержание иначе мигало бы);
// repeatOk — при plain автоповтор допустим (сдвиг места удержанной стрелкой).
export interface Combo {
  keys: string[]
  ctrl?: boolean
  shift?: boolean
  plain?: boolean
  repeatOk?: boolean
}

export interface Hotkey {
  action?: KeyAction // нет — строка только для справки
  combos?: Combo[] // нет — только справка (мышь, редактор текста)
  when?: (c: KeyContext) => boolean
  feature?: FeatureId // уровень, с которого строка работает и видна в справке (levels.ts)
  // Только справка: клавиша работает всегда (0 — «вписать» безвредно, Пробел ведёт и раскладку),
  // но строку про увеличение/галерею без этих функций не показываем. Достаточно любой из перечисленных.
  helpFeature?: FeatureId[]
  show: string // «Ctrl+Z», «Пробел (коротко)», «Колесо»
  menu?: string // короткая запись для строки меню, если show длинный («Ctrl+Y»)
  hidden?: boolean // в справку (F1) не попадает: служебная строка
  label: string // «Отменить»
  group: Group
}

export const GROUPS: { id: Group; title: string }[] = [
  { id: 'album', title: 'Альбом' },
  { id: 'spreads', title: 'Развороты и раскладки' },
  { id: 'canvas', title: 'Холст' },
  { id: 'photos', title: 'Фото' },
  { id: 'text', title: 'Текст' },
  { id: 'grid', title: 'Ручная раскладка' },
  { id: 'backdrop', title: 'Режим фона' },
  { id: 'view', title: 'Вид' },
]

const arrows = ['arrowleft', 'arrowright', 'arrowup', 'arrowdown']
const del = ['delete', 'backspace']
// Стрелки двигают место только на сетке с выделенным: у «Авто» сдвиг закрепил бы разворот,
// поэтому там ← → листают развороты, как обычно.
// На уровне без ручной раскладки сдвига нет — стрелки листают развороты.
const nudging = (c: KeyContext) => !c.bgMode && !!c.gridSpread && !!c.placeSelected && (!c.can || c.can('manual'))
// Стикеры (048) раскладку не закрепляют, поэтому стрелки двигают их и на «Авто».
const stickerNudging = (c: KeyContext) => !c.bgMode && !!c.stickerSelected
const bg = (c: KeyContext) => !!c.bgMode
const free = (c: KeyContext) => !c.modal
// Отметки фото (004): только в ленте — не после щелчка по холсту, не под окном, не в крупном просмотре
// и не при открытой галерее (лента свёрнута и inert).
// Уровень решает feature.
const MARKS: FeatureId = 'marks'
const marksOn = (c: KeyContext) => c.zone !== 'canvas' && !c.modal && !c.big && !c.galleryOpen
const layerOn = (c: KeyContext) => !c.bgMode && !c.modal && !!c.layerable

// Порядок важен: сдвиг места раньше листания разворотов.
export const HOTKEYS: Hotkey[] = [
  // Альбом
  { action: 'undo', combos: [{ keys: ['z', 'я'], ctrl: true, shift: false }], show: 'Ctrl+Z', label: 'Отменить', group: 'album' },
  {
    action: 'redo',
    combos: [
      { keys: ['z', 'я'], ctrl: true, shift: true },
      { keys: ['y', 'н'], ctrl: true },
    ],
    show: 'Ctrl+Y / Ctrl+Shift+Z',
    menu: 'Ctrl+Y',
    label: 'Повторить',
    group: 'album',
  },
  { action: 'save', combos: [{ keys: ['s', 'ы'], ctrl: true, shift: false }], show: 'Ctrl+S', label: 'Сохранить', group: 'album' },
  {
    action: 'saveAs',
    combos: [{ keys: ['s', 'ы'], ctrl: true, shift: true }],
    show: 'Ctrl+Shift+S',
    label: 'Сохранить как…',
    group: 'album',
  },
  { action: 'new', combos: [{ keys: ['n', 'т'], ctrl: true }], when: free, show: 'Ctrl+N', label: 'Новый альбом', group: 'album' },
  { action: 'open', combos: [{ keys: ['o', 'щ'], ctrl: true }], when: free, show: 'Ctrl+O', label: 'Открыть альбом', group: 'album' },
  { action: 'export', combos: [{ keys: ['e', 'у'], ctrl: true }], when: free, show: 'Ctrl+E', label: 'Экспорт', group: 'album' },
  {
    action: 'toggleRulers',
    feature: 'rulers',
    combos: [{ keys: ['r', 'к'], ctrl: true }],
    show: 'Ctrl+R',
    label: 'Линейки',
    group: 'view',
  },
  {
    action: 'reload',
    combos: [{ keys: ['r', 'к'], ctrl: true }],
    show: 'Ctrl+R',
    label: 'Обновление окна отключено',
    hidden: true,
    group: 'view',
  },
  {
    action: 'selectSpread',
    feature: 'groupSelect',
    combos: [{ keys: ['a', 'ф'], ctrl: true }],
    when: (c) => c.zone === 'canvas',
    show: 'Ctrl+A (на развороте)',
    menu: 'Ctrl+A',
    label: 'Выделить все фото, места и тексты разворота',
    group: 'photos',
  },
  {
    action: 'selectAll',
    combos: [{ keys: ['a', 'ф'], ctrl: true }],
    when: (c) => c.zone !== 'canvas',
    show: 'Ctrl+A (в библиотеке)',
    menu: 'Ctrl+A',
    label: 'Выделить все фото в ленте',
    group: 'photos',
  },

  // Вид
  {
    action: 'toggleBig',
    feature: 'timeline',
    combos: [{ keys: ['f', 'а'], plain: true }],
    show: 'F',
    label: 'Крупный просмотр',
    group: 'view',
  },
  { action: 'toggleDetails', combos: [{ keys: ['i', 'ш'], plain: true }], show: 'I', label: 'Панель деталей', group: 'view' },
  { action: 'toggleSafe', combos: [{ keys: ['g', 'п'], plain: true }], show: 'G', label: 'Безопасная зона', group: 'view' },
  // Отметки фото (004), как в Lightroom. Строки выше fitZoom: 0 при выделении — «убрать звёзды»,
  // без выделения или после щелчка по холсту остаётся «вписать». Без выделения 1–5 и остальные дают тост.
  {
    action: 'markRating',
    feature: MARKS,
    combos: [{ keys: ['1', '2', '3', '4', '5'], plain: true, shift: false }],
    when: marksOn,
    show: '1–5 / 0',
    label: 'Звёзды выделенным фото ленты (0 — убрать)',
    group: 'photos',
  },
  {
    action: 'markRating',
    feature: MARKS,
    combos: [{ keys: ['0'], plain: true, shift: false }],
    when: (c) => marksOn(c) && c.hasSelection,
    hidden: true,
    show: '0',
    label: 'Убрать звёзды',
    group: 'photos',
  },
  {
    action: 'markStep',
    feature: MARKS,
    combos: [{ keys: [']', 'ъ', '[', 'х'], plain: true, shift: false }],
    when: marksOn,
    show: '] / [',
    label: 'Звёзд больше / меньше',
    group: 'photos',
  },
  {
    action: 'markLabel',
    feature: MARKS,
    combos: [{ keys: ['6', '7', '8', '9'], plain: true, shift: false }],
    when: marksOn,
    show: '6 / 7 / 8 / 9',
    label: 'Метка: красная / жёлтая / зелёная / синяя (повтор у всех — снять)',
    group: 'photos',
  },
  {
    action: 'markPick',
    feature: MARKS,
    combos: [{ keys: ['p', 'з'], plain: true, shift: false }],
    when: marksOn,
    show: 'P',
    label: 'Флажок «отобрано»',
    group: 'photos',
  },
  {
    action: 'markReject',
    feature: MARKS,
    combos: [{ keys: ['x', 'ч'], plain: true, shift: false }],
    when: marksOn,
    show: 'X',
    label: 'Флажок «отклонено»',
    group: 'photos',
  },
  {
    action: 'markUnflag',
    feature: MARKS,
    combos: [{ keys: ['u', 'г'], plain: true, shift: false }],
    when: marksOn,
    show: 'U',
    label: 'Снять флажок',
    group: 'photos',
  },

  // Ctrl+0 — привычное «вписать»; в окне Wails он сбросил бы масштаб страницы, поэтому гасится (keys.ts PREVENT).
  {
    action: 'fitZoom',
    combos: [
      { keys: ['0'], plain: true },
      { keys: ['0'], ctrl: true, shift: false },
    ],
    helpFeature: ['zoom'],
    show: '0 / Ctrl+0',
    menu: 'Ctrl+0',
    label: 'Вписать разворот в окно',
    group: 'canvas',
  },
  { action: 'zoomIn', feature: 'zoom', combos: [{ keys: ['=', '+'], ctrl: true }], show: 'Ctrl+=', label: 'Увеличить', group: 'canvas' },
  { action: 'zoomOut', feature: 'zoom', combos: [{ keys: ['-'], ctrl: true }], show: 'Ctrl+−', label: 'Уменьшить', group: 'canvas' },
  { action: 'guide', combos: [{ keys: ['f1'], plain: true }], show: 'F1', label: 'Справочник функций', group: 'view' },
  { action: 'help', combos: [{ keys: ['?'], plain: true }], show: '?', label: 'Горячие клавиши', group: 'view' },

  // Режим фона: свои Esc, стрелки и Del — раньше листания, выделения и удаления
  {
    action: 'bgNudge',
    feature: 'bgPhoto',
    combos: [{ keys: arrows, plain: true, repeatOk: true }],
    when: bg,
    show: '← ↑ ↓ →',
    label: 'Сдвинуть рамку фона на 1 мм, с Shift — на 10 мм',
    group: 'backdrop',
  },
  {
    action: 'bgRemove',
    feature: 'bgPhoto',
    combos: [{ keys: del }],
    when: bg,
    show: 'Delete / Backspace',
    label: 'Убрать фото фона и выйти из режима',
    group: 'backdrop',
  },
  {
    action: 'bgExit',
    feature: 'bgPhoto',
    combos: [{ keys: ['escape'] }],
    when: (c) => bg(c) && !c.galleryOpen,
    show: 'Esc',
    label: 'Выйти из режима фона (во время жеста — отменить жест)',
    group: 'backdrop',
  },

  // Сетка: стрелки двигают выделенное место
  {
    action: 'nudgePlace',
    feature: 'manual',
    combos: [{ keys: arrows, plain: true, repeatOk: true }],
    when: nudging,
    show: '← ↑ ↓ → (ручная)',
    label: 'Сдвинуть выделенное место на 1 мм, с Shift — на 10 мм',
    group: 'grid',
  },
  // То же для стикеров на любом листе (справка про них — в строке выше)
  {
    action: 'nudgePlace',
    feature: 'decor',
    combos: [{ keys: arrows, plain: true, repeatOk: true }],
    when: stickerNudging,
    show: '← ↑ ↓ → (стикер)',
    label: 'Сдвинуть выделенный стикер на 1 мм, с Shift — на 10 мм',
    group: 'grid',
    hidden: true,
  },

  // Слои: [ вниз, ] вверх; с Shift — на задний/передний план. Русские х и ъ стоят на тех же клавишах.
  {
    action: 'layerFront',
    feature: 'layers',
    combos: [{ keys: [']', '}', 'ъ'], ctrl: true, shift: true }],
    when: layerOn,
    show: 'Ctrl+Shift+]',
    label: 'Выделенное — на передний план',
    group: 'grid',
  },
  {
    action: 'layerBack',
    feature: 'layers',
    combos: [{ keys: ['[', '{', 'х'], ctrl: true, shift: true }],
    when: layerOn,
    show: 'Ctrl+Shift+[',
    label: 'Выделенное — на задний план',
    group: 'grid',
  },
  {
    action: 'layerUp',
    feature: 'layers',
    combos: [{ keys: [']', 'ъ'], ctrl: true, shift: false }],
    when: layerOn,
    show: 'Ctrl+]',
    label: 'Выделенное — на слой выше',
    group: 'grid',
  },
  {
    action: 'layerDown',
    feature: 'layers',
    combos: [{ keys: ['[', 'х'], ctrl: true, shift: false }],
    when: layerOn,
    show: 'Ctrl+[',
    label: 'Выделенное — на слой ниже',
    group: 'grid',
  },

  // Развороты и фото
  {
    action: 'prevSpread',
    combos: [{ keys: ['arrowleft'] }],
    when: (c) => !nudging(c),
    show: '←',
    label: 'Предыдущий разворот',
    group: 'spreads',
  },
  {
    action: 'nextSpread',
    combos: [{ keys: ['arrowright'] }],
    when: (c) => !nudging(c),
    show: '→',
    label: 'Следующий разворот',
    group: 'spreads',
  },
  {
    action: 'removeCell',
    combos: [{ keys: del }],
    when: (c) => c.zone !== 'library',
    show: 'Delete / Backspace',
    menu: 'Del',
    label:
      'Выбранное фото — в ленту (в ручной раскладке место остаётся, второй раз — убрать место); блок текста — удалить; группа — все сразу. Из библиотеки не удаляет',
    group: 'photos',
  },

  // Пробел: нажатие — spaceDown (App), отпускание — canvas.spaceUp: коротко и без панорамы —
  // раскладка (её листают nextLayout/prevLayout, здесь они — строки справки), зажат — панорама.
  {
    action: 'spaceDown',
    combos: [{ keys: [' '] }],
    when: (c) => !c.onCheckbox,
    helpFeature: ['zoom'],
    show: 'Пробел + мышь',
    label: 'Двигать увеличенный разворот',
    group: 'canvas',
  },
  { action: 'nextLayout', feature: 'variants', show: 'Пробел (коротко)', label: 'Следующая раскладка разворота', group: 'spreads' },
  { action: 'prevLayout', feature: 'variants', show: 'Shift+Пробел (коротко)', label: 'Предыдущая раскладка разворота', group: 'spreads' },
  {
    action: 'addSelected',
    combos: [{ keys: ['enter'] }],
    when: (c) => c.hasSelection,
    show: 'Enter',
    label: 'Выделенные фото — на текущий разворот',
    group: 'photos',
  },

  // Esc: галерея, иначе снять выделение
  {
    action: 'closeGallery',
    combos: [{ keys: ['escape'] }],
    when: (c) => c.galleryOpen,
    helpFeature: ['variants', 'templates'],
    show: 'Esc',
    label: 'Закрыть галерею раскладок',
    group: 'spreads',
  },
  { action: 'clearSelection', combos: [{ keys: ['escape'] }], show: 'Esc', label: 'Снять выделение', group: 'photos' },

  // Только справка: мышь и жесты
  { show: 'Колесо, Shift+колесо, тачпад', label: 'Прокрутить ряд разворотов', group: 'canvas' },
  { feature: 'zoom', show: 'Колесо, Shift+колесо (в увеличении)', label: 'Двигать лист по вертикали и по горизонтали', group: 'canvas' },
  { feature: 'zoom', show: 'Ctrl+колесо, щипок тачпада', label: 'Масштаб к курсору', group: 'canvas' },
  { show: 'Перетащить соседний разворот, Пробел+мышь', label: 'Прокрутить ряд разворотов', group: 'canvas' },
  { feature: 'zoom', show: 'Зажатое колёсико', label: 'Двигать увеличенный разворот', group: 'canvas' },
  { feature: 'zoom', show: 'Двойной щелчок по фону', label: 'Вписать разворот в окно', group: 'canvas' },
  { feature: 'zoom', show: 'Щелчок по масштабу (внизу справа)', label: 'Бегунок масштаба', group: 'canvas' },
  { feature: 'zoom', show: 'Двойной щелчок по масштабу', label: 'Вписать разворот в окно', group: 'canvas' },
  // Спека 015: способ выделения, а не инструмент — без feature, на всех уровнях.
  { show: 'Alt+щелчок', label: 'Выбрать объект под ним; ещё раз — следующий, по кругу', group: 'canvas' },
  { feature: 'names', show: 'Двойной щелчок по номеру в ленте', label: 'Название разворота', group: 'spreads' },
  { feature: 'text', show: 'Двойной щелчок по тексту', label: 'Править текст', group: 'text' },
  { feature: 'text', show: 'Esc / Ctrl+Enter (в тексте)', label: 'Закончить правку текста', group: 'text' },
  { feature: 'textRotate', show: 'Shift (поворот текста)', label: 'Угол кратен 15°', group: 'text' },
  { feature: 'bgPhoto', show: '«Править» в карточке разворота', label: 'Войти в режим фона', group: 'backdrop' },
  { feature: 'bgPhoto', show: 'Shift (поворот фона)', label: 'Угол кратен 15°; Alt — без привязки', group: 'backdrop' },
  { feature: 'bgPhoto', show: 'Щелчок мимо рамки фона', label: 'Выйти из режима фона', group: 'backdrop' },
  {
    feature: 'manual',
    show: 'Перетащить фото или рамку (ручная)',
    label: 'Переносит само место, фото остаётся в нём; в «Авто» перенос фото меняет фото местами',
    group: 'grid',
  },
  { feature: 'manual', show: '⇄ в мини-панели (ручная)', label: 'Перетащить фото на другое место: фото поменяются местами', group: 'grid' },
  { feature: 'manual', show: 'Shift (размер места)', label: 'Тянуть угол с сохранением пропорций', group: 'grid' },
  { feature: 'manual', show: 'Alt (перенос места или текста)', label: 'Без привязки к направляющим', group: 'grid' },
  {
    feature: 'groupSelect',
    show: 'Shift+клик по фото',
    label: 'Добавить к выделению или убрать из него: фото, место, текст (то же с Ctrl) — группа',
    group: 'grid',
  },
]

// Совпала ли комбинация с событием; k — e.key в нижнем регистре.
export function comboMatches(cb: Combo, e: KeyEvent, k: string): boolean {
  if (!cb.keys.includes(k)) return false
  const ctrl = e.ctrlKey || e.metaKey
  if (cb.ctrl && !ctrl) return false
  if (cb.shift !== undefined && cb.shift !== e.shiftKey) return false
  if (cb.plain && (ctrl || e.altKey || (e.repeat && !cb.repeatOk))) return false
  return true
}

// Строка доступна на текущем уровне (без can в контексте — всегда).
const featureOn = (h: Hotkey, c: KeyContext): boolean => !h.feature || !c.can || c.can(h.feature)

// Строки справки (F1) для данного уровня: чужие функции не показываются. can не задан — все.
export const visibleHotkeys = (can?: (f: FeatureId) => boolean): Hotkey[] =>
  HOTKEYS.filter((h) => !h.hidden && (!can || ((!h.feature || can(h.feature)) && (!h.helpFeature || h.helpFeature.some((f) => can(f))))))

// Русская раскладка учитывается: я н ы ф а ш п стоят на тех же клавишах, что z y s a f i g.
export function findAction(e: KeyEvent, c: KeyContext): KeyAction | null {
  const k = e.key.toLowerCase()
  for (const h of HOTKEYS) {
    if (!h.action || !h.combos || (h.when && !h.when(c)) || !featureOn(h, c)) continue
    if (h.combos.some((cb) => comboMatches(cb, e, k))) return h.action
  }
  return null
}
