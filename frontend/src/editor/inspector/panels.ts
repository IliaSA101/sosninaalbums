// Боковые панели правки (спека 008, panels.md): размеры, запас по бокам разворота и режим.
// Чистые функции, без Svelte.
import { HOTKEYS, type KeyAction } from '../../shell/menu/hotkeys'
import { plural } from '../../base/core/plural'

export const M = 8 // отступ панелей от краёв окна холста
export const BAR = 32 // полоса над холстом (LayoutBar) — всегда одна высота
export const BTN = 32 // кнопка ленты
export const STEP = 36 // шаг кнопок ленты
export const ICON = 18 // значок в кнопке ленты
export const INSP_W = 200 // карточка справа
export const INSP_ZONE = INSP_W + 2 * M
export const BADGE = 32 // место под ZoomBadge под карточкой
// Лента одинакова во всех режимах (спека 009, п. 2): группы «Добавить» (3: Текст, Место, Стикер — 048) и «Раскладка» (3),
// запас слева от режима не зависит.
export const RAIL_GROUPS = [3, 3]
export const RAIL_MAX = 6 // кнопок в ленте
export const COL_W = 60 // столбец без подписей кнопок: ширина под подпись группы («Раскладка»)
export const COL_GAP = 4 // между столбцами
export const HEAD = 16 // подпись группы
export const SEP = 9 // черта между группами с отступами

// Режим панелей: null — панелей нет (нет разворота, открыта галерея); 'idle' — ничего не выделено.
// text — блок текста или подпись; photo — одно фото; places — несколько выделенных или пустое место.
export type PanelMode = 'idle' | 'photo' | 'places' | 'text' | 'sticker' | null

export interface PanelCtx {
  spread: string | null // текущий разворот
  gallery: boolean // открыта галерея раскладок или сеток
  textSpread: string | null // разворот выделенного блока текста или подписи
  refs: number // выделенных ref на текущем развороте (фото и места)
  group: boolean // выделено 2 и больше объекта (места, фото, тексты)
  photo: boolean // выделено ровно одно, и это фото
  photoMissing: boolean // файла этого фото нет на диске
  sticker?: boolean // выделен один стикер и больше ничего (048)
}

// Текст → группа → стикер → фото → места → ничего; группа раньше фото: «фото + текст» и «два текста» — карточка группы. Лента — глаголы, карточка — значения. Пропавшее фото — как «ничего»:
// кадр показать нечем.
export function panelMode(c: PanelCtx): PanelMode {
  if (!c.spread || c.gallery) return null
  if (c.textSpread === c.spread) return 'text'
  if (c.group) return 'places'
  if (c.sticker) return 'sticker'
  if (c.refs === 1 && c.photo) return c.photoMissing ? 'idle' : 'photo'
  if (c.refs > 0) return 'places'
  return 'idle'
}

// Заголовок карточки группы: «3 места», «2 места, 1 текст», «2 текста», «1 текст, 2 стикера».
export function groupTitle(places: number, texts: number, stickers = 0): string {
  const parts: string[] = []
  if (places > 0) parts.push(`${places} ${plural(places, ['место', 'места', 'мест'])}`)
  if (texts > 0) parts.push(`${texts} ${plural(texts, ['текст', 'текста', 'текстов'])}`)
  if (stickers > 0) parts.push(`${stickers} ${plural(stickers, ['стикер', 'стикера', 'стикеров'])}`)
  return parts.join(', ')
}

// Заголовок сворачиваемой карточки в панели деталей: тот же, что был у тела («Разворот N», «Текст», «Подпись»…).
export interface CardCtx {
  spreadNo: number // номер текущего разворота с 1
  caption: boolean // выделена подпись, а не блок текста
  group: boolean // места/тексты группой
  places: number
  texts: number
  stickers?: number // стикеры группы (048)
  cover?: boolean // текущий лист — обложка (032): «Обложка» вместо номера
  unit?: string // «Разворот» (по умолчанию) или «Лист» в режиме виньетки (031)
}
export function cardTitle(mode: PanelMode, c: CardCtx): string {
  switch (mode) {
    case 'text':
      return c.caption ? 'Подпись' : 'Текст'
    case 'photo':
      return 'Фото'
    case 'places':
      return c.group ? groupTitle(c.places, c.texts, c.stickers ?? 0) : 'Место'
    case 'sticker':
      return 'Стикер'
    default:
      return c.cover ? 'Обложка' : `${c.unit ?? 'Разворот'} ${c.spreadNo}`
  }
}

// Высота группы из n кнопок: подпись и кнопки через зазор STEP − BTN.
export const groupHeight = (n: number): number => HEAD + Math.max(0, n) * STEP - (STEP - BTN)

// Столбцы ленты при высоте окна холста vpH: группа целиком, не влезла — новый столбец, без прокрутки.
// Результат — индексы групп в каждом столбце. Рамка ленты — 2 px.
export function railColumns(groups: number[], vpH: number): number[][] {
  const avail = Number.isFinite(vpH) ? vpH - 2 * M - 2 : 0
  const cols: number[][] = []
  let used = 0
  groups.forEach((n, i) => {
    const h = groupHeight(n)
    if (cols.length && used + SEP + h <= avail) {
      cols[cols.length - 1].push(i)
      used += SEP + h
    } else {
      cols.push([i])
      used = h
    }
  })
  return cols
}
export const railCols = (vpH: number): number => Math.max(1, railColumns(RAIL_GROUPS, vpH).length)
export const railWidth = (cols: number): number => Math.max(1, cols) * COL_W + (Math.max(1, cols) - 1) * COL_GAP + 2

// Подписи у кнопок ленты (справа от значка): только если лента — один столбец, карточка не в панели
// деталей и подпись помещается в запас слева (pad = Viewport.pad). Иначе — значки с подсказкой.
// Ширину оцениваем по числу знаков (шрифт 12 px): точный замер ради подписей не нужен.
export const LABEL_GAP = 6 // значок — подпись
export const LABEL_PAD = 10 // справа от подписи
const CH = 7.2 // px на знак
export const labelWidth = (chars: number): number => Math.max(COL_W, BTN + LABEL_GAP + Math.ceil(Math.max(0, chars) * CH) + LABEL_PAD)
export function railLabeled(vpH: number, docked: boolean, pad: number, longest: number): boolean {
  if (docked || longest < 1 || railCols(vpH) !== 1) return false
  return M + labelWidth(longest) + 2 + M <= pad
}

// Запас по каждому боку разворота, px: симметричный и постоянный — вход и выход из правки
// размер разворота не меняют. docked — карточка в панели деталей, справа только край ленты.
export function sideReserve(side: number, gap: number, docked: boolean, cols: number): number {
  const zone = docked ? M + railWidth(cols) + M : INSP_ZONE
  return Math.max(side + gap, zone)
}

// Наибольшая высота карточки: под ней — значок масштаба.
export const inspectorMaxH = (vpH: number): number => Math.max(0, vpH - 2 * M - BADGE)

// Клавиша для подсказки кнопки — из таблицы hotkeys.ts, без пояснения в скобках.
export function keyHint(action: KeyAction): string {
  const show = HOTKEYS.find((h) => h.action === action)?.show ?? ''
  return show.replace(/\s*\(.*\)\s*$/, '')
}
