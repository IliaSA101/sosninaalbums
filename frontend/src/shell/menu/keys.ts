// Диспетчер горячих клавиш (спека 074, design §7.2): событие и контекст -> id команды по реестру
// (base/keys: клавиши приходят из Go, меняются пользователем). Само действие выполняет commands.run.
// Здесь только то, что решается синхронно в keydown: поле ввода, виджет, окно, область, повтор, условия.
import { eventChord, modsOfChord, codeOfChord, type ChordEvent } from '../../base/keys/chord'
import { keymap } from '../../base/keys/keymap.svelte'
import type { FeatureId } from '../../base/prefs/levels'
import type { KeymapEntry, KeyScope } from '../../base/types/keys'
import type { Zone } from '../../editor/places/groupsel'

export interface KeyEvent extends ChordEvent {
  repeat: boolean
}

// Верхнее окно: undo — оно само правит альбом (Modal undo), под ним работают «Отменить» и «Повторить»;
// shut — окно любое другое (вопрос, экспорт, сбор, открытие…): команды молчат. Нет окна — не задано.
export type DialogState = 'undo' | 'shut'

export interface KeyContext {
  inTextField: boolean // фокус в поле ввода текста/чисел: клавиши молчат
  popup?: boolean // фокус внутри окна панели (Popover, data-barpop): Enter там — кнопка или поле окна, не «Перенести на разворот»
  widget?: boolean // фокус на ползунке, select или списке: стрелки и PageUp/PageDown/Home/End — его (074-19)
  onCheckbox: boolean // пробел на флажке переключает флажок
  galleryOpen: boolean
  hasSelection: boolean // есть выделенные видимые фото
  gridSpread?: boolean // текущий разворот — «Ручная»: у неё стрелки двигают место
  placeSelected?: boolean // выделено фото, место или текст группы текущего разворота
  stickerSelected?: boolean // выделены только стикеры (048): стрелки двигают их на любом листе, и на «Авто»
  layerable?: boolean // «Ручная» и выделено место, фото, текст или стикер (или стикер на «Авто»): слои двигать есть что
  copyable?: boolean // на листе выделено то, что копируется (место, фото, надпись, стикер), и не идёт ввод надписи (099)
  pasteable?: boolean // буфер не пуст и есть лист, куда вставлять (099)
  nativeSel?: boolean // выделен текст страницы вне холста (справочник, «О снимке»): Ctrl+C его, а не листа (099)
  zone?: Zone // где было последнее нажатие мыши; не задано — лента фото
  bgMode?: boolean // включён режим фона: его Esc, стрелки и Del
  big?: boolean // «Во весь экран» (F): ленты не видно, отметки клавишами молчат
  dialog?: DialogState // открыто окно (074-18, 074-28)
  preview?: 'library' | 'canvas' // открыт крупный просмотр (128): откуда; из ленты под окном проходят отметки
  can?: (feature: FeatureId) => boolean // уровень интерфейса: команда с чужой функцией молчит; не задано — всё доступно
  menuOn?: (id: string) => boolean // пункт меню команды виден на уровне и не серый; не задано — да
}

// Поля ввода: текст и числа, а также блоки с contenteditable (редактор текста на развороте).
// Ползунок, флажок, радио и кнопки клавиши не глушат.
export const TEXT_FIELDS =
  "textarea, input:not([type='range']):not([type='checkbox']):not([type='radio']):not([type='button']):not([type='submit']), [contenteditable]:not([contenteditable='false'])"

// Виджеты, которым принадлежат стрелки и PageUp/PageDown/Home/End (074-19).
export const WIDGETS = "input[type='range'], select, [role='slider'], [role='listbox'], [role='radiogroup'], [role='menu']"

// Цель события бывает document или window — у них нет closest.
type Closest = { closest?(sel: string): unknown } | null | undefined
export const isTextField = (el: Closest) => typeof el?.closest === 'function' && !!el.closest(TEXT_FIELDS)
export const isWidget = (el: Closest) => typeof el?.closest === 'function' && !!el.closest(WIDGETS)

const WIDGET_KEYS = new Set(['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'PageUp', 'PageDown', 'Home', 'End'])
const widgetKey = (chord: string) => !modsOfChord(chord).some((m) => m === 'Mod' || m === 'Alt') && WIDGET_KEYS.has(codeOfChord(chord))

// Область действует: «везде» всегда; холст и лента фото — по зоне последнего щелчка (не пересекаются); фон — в режиме фона.
const active = (scope: KeyScope, c: KeyContext): boolean =>
  scope === 'app' || (scope === 'canvas' ? c.zone === 'canvas' : scope === 'library' ? c.zone !== 'canvas' : !!c.bgMode)

// Стрелки двигают место только на сетке с выделенным: у «Авто» сдвиг закрепил бы разворот,
// поэтому там ← → листают развороты. На уровне без ручной раскладки сдвига нет.
const gridNudging = (c: KeyContext) => !c.bgMode && !!c.gridSpread && !!c.placeSelected && (!c.can || c.can('manual'))
// Стикеры (048) раскладку не закрепляют, поэтому стрелки двигают их и на «Авто» (функция «Фоны и стикеры»).
const stickerNudging = (c: KeyContext) => !c.bgMode && !!c.stickerSelected && (!c.can || c.can('decor'))
const nudging = (c: KeyContext) => gridNudging(c) || stickerNudging(c)
// Отметки (004): не в «Во весь экран» и не при открытой галерее (лента свёрнута и inert). Зону решает область library.
const marksOn = (c: KeyContext) => !c.big && !c.galleryOpen
// Подборки (083): то же, и функция уровня (Go проверяет feature, но когда can не задан — всё доступно).
const binsOn = (c: KeyContext) => marksOn(c) && (!c.can || c.can('bins'))

// Буфер (099): холст под галереей не виден, в режиме фона и в «Во весь экран» выделять нечего; выделенный текст страницы — родной.
const clipOn = (c: KeyContext) => !c.bgMode && !c.galleryOpen && !c.big && !c.nativeSel
const copyOn = (c: KeyContext) => clipOn(c) && !!c.copyable
const pasteOn = (c: KeyContext) => clipOn(c) && !!c.pasteable

// Крупный просмотр (128): в зоне ленты — как отметки (не в «Во весь экран» и не при галерее), на холсте — без галереи и
// режима фона. Нечего открыть — команда сама скажет тостом.
const previewOn = (c: KeyContext) => (c.zone === 'canvas' ? !c.galleryOpen && !c.bgMode : marksOn(c))
// Под окном просмотра из ленты проходят отметки показанному фото (0.26-128-6); 0 остаётся окну («вписать»).
const previewMark = (cmd: KeymapEntry, c: KeyContext) => c.preview === 'library' && cmd.group === 'marks' && cmd.id !== 'markRating0'

// Условия команд (прежние when таблицы). Команда без записи — без условий; без клавиши по умолчанию — «пункт меню».
const WHEN: Readonly<Record<string, (c: KeyContext) => boolean>> = {
  markRating0: (c) => marksOn(c) && c.hasSelection, // без выделения 0 остаётся «вписать»
  ...Object.fromEntries(
    [
      'markRating1',
      'markRating2',
      'markRating3',
      'markRating4',
      'markRating5',
      'markUp',
      'markDown',
      'markRed',
      'markYellow',
      'markGreen',
      'markBlue',
      'markPick',
      'markReject',
      'markUnflag',
    ].map((id) => [id, marksOn]),
  ),
  photoPreview: previewOn,
  findPhotos: marksOn, // поле поиска в ленте: в «Во весь экран» и при галерее ленты нет (0.26-124-10)
  // Подборки (083): как отметки, плюс уровень; Del — только с выделением (без него в ленте Del молчит, как раньше).
  binToggle: (c) => binsOn(c),
  binNew: (c) => binsOn(c),
  binAll: (c) => binsOn(c),
  binRemove: (c) => binsOn(c) && c.hasSelection,
  prevSpread: (c) => !nudging(c),
  nextSpread: (c) => !nudging(c),
  nudgePlace: nudging,
  cutItems: copyOn,
  copyItems: copyOn,
  duplicateItems: copyOn,
  pasteItems: pasteOn,
  closeGallery: (c) => c.galleryOpen,
  bgExit: (c) => !c.galleryOpen, // при открытой галерее Esc закрывает её
  addSelected: (c) => c.hasSelection,
  spaceDown: (c) => !c.onCheckbox,
  ...Object.fromEntries(
    ['layerFront', 'layerUp', 'layerDown', 'layerBack'].map((id) => [id, (c: KeyContext) => !c.bgMode && !!c.layerable]),
  ),
}

// Область действует. Отметки показанному фото — и когда последнее нажатие было на холсте (окно открыли с ленты, 128).
const scopeOk = (cmd: KeymapEntry, c: KeyContext) => active(cmd.scope, c) || previewMark(cmd, c)

// Под окном: «Отменить»/«Повторить» только под окнами правки (074-28), плюс отметки под просмотром из ленты (128).
const dialogOk = (cmd: KeymapEntry, c: KeyContext) => !c.dialog || (c.dialog === 'undo' && !!(cmd.dialogs || previewMark(cmd, c)))

function allowed(cmd: KeymapEntry, e: KeyEvent, c: KeyContext): boolean {
  if (!scopeOk(cmd, c) || !dialogOk(cmd, c)) return false
  if (e.repeat && !cmd.repeat) return false // удержание повторяет только сдвиг, масштаб, отмену, слои и листание
  if (cmd.feature && c.can && !c.can(cmd.feature as FeatureId)) return false
  const when = WHEN[cmd.id]
  if (when) return when(c)
  // Команда без клавиши по умолчанию — пункт меню: срабатывает, пока он виден и не серый (074-8).
  return cmd.defaults.length > 0 || !c.menuOn || c.menuOn(cmd.id)
}

// Событие -> id команды. В поле ввода команды молчат (F1 решает App раньше); виджету — его стрелки.
export function keyCommand(e: KeyEvent, c: KeyContext): string | null {
  if (c.inTextField) return null
  const chord = eventChord(e)
  if (!chord) return null
  if (c.widget && widgetKey(chord)) return null
  if (c.popup && chord === 'Enter') return null
  for (const cmd of keymap.index.get(chord) ?? []) if (allowed(cmd, e, c)) return cmd.id
  return null
}

// Команды, у которых гасится поведение браузера по умолчанию (прокрутка, зум страницы, справка).
const PREVENT = new Set([
  'undo',
  'redo',
  'save',
  'saveAs',
  'new',
  'open',
  'export',
  'toggleRulers',
  'zoomIn',
  'zoomOut',
  'fitZoom',
  'selectAll',
  'selectSpread',
  'nudgePlace',
  'layerFront',
  'layerUp',
  'layerDown',
  'layerBack',
  'spaceDown',
  'help',
  'guide',
  'bgNudge',
])

// Сработавшая команда: гасим браузер у списка и у любого сочетания с Ctrl/Cmd или Alt (свои Ctrl+P/F/U в стенде).
export function preventsDefault(id: string, e?: ChordEvent): boolean {
  if (PREVENT.has(id)) return true
  const chord = e && eventChord(e)
  return !!chord && modsOfChord(chord).some((m) => m === 'Mod' || m === 'Alt')
}

// Страж браузера (без совпадения): перезагрузка и «назад» в стенде уронили бы состояние; в exe их и так глушит Wails.
const BROWSER = new Set(['Mod+KeyR', 'Mod+Shift+KeyR', 'F5', 'Alt+ArrowLeft', 'Alt+ArrowRight'])
export const guardsBrowser = (e: ChordEvent): boolean => {
  const chord = eventChord(e)
  return !!chord && BROWSER.has(chord)
}
