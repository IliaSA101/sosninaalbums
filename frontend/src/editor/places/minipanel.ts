// Мини-панель выделенного (спека 009, gestures.md «Мини-панель»): какие значки показать и где
// поставить полосу. Чистая логика; действия — в MiniPanel.svelte.
import { canFitPhoto, type SelBox } from '../../base/geom/gridedit'
import { actualAspect } from '../../base/geom/frame'
import type { IconName } from '../../base/ui/Icon.svelte'
import type { FeatureId } from '../../base/prefs/levels'
import type { Rect, SpreadView } from '../../base/core/types'
import { whoOf } from '../captions/nametarget'

export type MiniId = 'flipH' | 'flipV' | 'turn' | 'fit' | 'fitPhoto' | 'caption' | 'name' | 'layer' | 'swap' | 'same' | 'delete'

export interface MiniItem {
  id: MiniId
  label: string // подсказка
  icon: IconName
  on?: boolean // включено (зеркало, «Целиком»)
  badge?: string // число рядом со значком («Удалить (N)» у группы)
  sep?: boolean // линия перед значком
}

// Что выделено на текущем развороте.
export interface MiniCtx {
  grid: boolean // «Ручная»
  places: number // выделенных рамок (фото и пустых мест)
  texts?: number // блоков текста в группе (одиночный текст мини-панели не имеет)
  photos: number // из них с фото
  differ: boolean // у кого-то пропорции места не совпадают с фото
  allContain: boolean
  flipH: boolean // зеркало ↔ включено у всех выделенных фото
  flipV: boolean
  name?: boolean // у единственного фото подпись идёт по шаблону с полями имени (037)
}

export function miniContext(s: SpreadView | undefined, sel: SelBox[], texts = 0): MiniCtx {
  const withPhoto = sel.filter((b) => b.photo)
  const frames = withPhoto.map((b) => ({ b, fv: s?.frames?.[s.photos.indexOf(b.photo)] }))
  return {
    grid: !!s?.grid,
    places: sel.length,
    texts,
    photos: withPhoto.length,
    differ: frames.some(({ b, fv }) => !!fv && fv.img.h > 0 && !canFitPhoto(b, actualAspect(fv))),
    allContain: withPhoto.length > 0 && withPhoto.every((b) => b.fit === 'contain'),
    flipH: frames.length > 0 && frames.every(({ fv }) => !!fv?.flipH),
    flipV: frames.length > 0 && frames.every(({ fv }) => !!fv?.flipV),
    name: withPhoto.length === 1 && whoOf(s, withPhoto[0].photo).length > 0,
  }
}

// Какой функции уровня принадлежит значок (levels.ts): чужие уровню не показываются.
// Подпись, ↻ 90° и «Удалить» — с «Простого».
const FEATURE: Partial<Record<MiniId, FeatureId>> = {
  flipH: 'flip',
  flipV: 'flip',
  turn: 'turn',
  fit: 'frame',
  fitPhoto: 'frame',
  caption: 'captions',
  name: 'captions',
  same: 'manual',
  layer: 'layers',
  swap: 'manual',
}

const TURN: MiniItem = { id: 'turn', label: 'Повернуть фото на 90° по часовой стрелке', icon: 'turn-cw' }

// Значок «Удалить»: подпись зависит от того, что выделено.
function deleteItem(c: MiniCtx, n: number): MiniItem {
  if (n > 1) return { id: 'delete', label: `Удалить выделенное (${n}) (Delete)`, icon: 'trash', badge: String(n) }
  if (c.photos === 0) return { id: 'delete', label: 'Убрать пустое место (Delete)', icon: 'trash' }
  if (c.grid) return { id: 'delete', label: 'Убрать фото с места: оно вернётся в ленту, место останется пустым (Delete)', icon: 'trash' }
  return { id: 'delete', label: 'Убрать фото с разворота: оно вернётся в ленту (Delete)', icon: 'trash' }
}

// Группы значков про фото: зеркало, кадр (одно фото) или только поворот (несколько).
function photoGroups(c: MiniCtx, solo: boolean): MiniItem[][] {
  if (c.photos === 0) return []
  const flips: MiniItem[] = [
    { id: 'flipH', label: 'Зеркало ↔ (по горизонтали)', icon: 'flip-h', on: c.flipH },
    { id: 'flipV', label: 'Зеркало ↕ (по вертикали)', icon: 'flip-v', on: c.flipV },
  ]
  if (!solo) return [flips, [TURN]]
  const fit: MiniItem[] = [
    {
      id: 'fit',
      label: c.allContain
        ? 'Сейчас целиком, поля — фон страницы. Нажмите: заполнять место, лишнее обрезается'
        : 'Сейчас с обрезкой. Нажмите: фото целиком, поля — фон страницы',
      icon: 'contain',
      on: c.allContain,
    },
  ]
  if (c.differ) fit.push({ id: 'fitPhoto', label: 'Подогнать место под пропорции фото', icon: 'fit-photo' })
  // Подпись по шаблону листа — кнопка ведёт к имени, а не создаёт свою подпись (037).
  const cap: MiniItem = c.name
    ? { id: 'name', label: 'Имя под фото', icon: 'caption' }
    : { id: 'caption', label: 'Подпись к фото', icon: 'caption' }
  return [flips, fit, [cap, TURN]]
}

// Значки слева направо по таблице gestures.md; группы разделяет линия. can — уровень интерфейса
// (не задан — всё доступно); пустая группа пропадает вместе с чертой.
export function miniItems(c: MiniCtx, can: (f: FeatureId) => boolean = () => true): MiniItem[] {
  const n = c.places + (c.texts ?? 0)
  if (n === 0) return []
  const solo = n === 1 && c.photos === 1
  const groups: MiniItem[][] = photoGroups(c, solo)
  const arrange: MiniItem[] = []
  if (c.places >= 2)
    arrange.push({
      id: 'same',
      label: 'Одинаковый размер: как у последнего щёлкнутого; при выделении рамкой — как у первого',
      icon: 'same-size',
    })
  if (c.grid) arrange.push({ id: 'layer', label: 'Слой', icon: 'layers' })
  if (c.grid && solo) arrange.push({ id: 'swap', label: 'Перетащите на другое место — фото поменяются местами', icon: 'swap' })
  if (arrange.length > 0) groups.push(arrange)

  groups.push([deleteItem(c, n)])

  const shown = groups.map((g) => g.filter((it) => !FEATURE[it.id] || can(FEATURE[it.id]!))).filter((g) => g.length > 0)
  return shown.flatMap((g, i) => g.map((it, k) => (i > 0 && k === 0 ? { ...it, sep: true } : it)))
}

// Порядок значка «Слой ▾» (меню в LayerMenu и пункты «Слой ▸» в меню фото).
export const LAYER_OPS = [
  { op: 'front', label: 'На передний план', icon: 'layer-front' },
  { op: 'up', label: 'Выше', icon: 'layer-up' },
  { op: 'down', label: 'Ниже', icon: 'layer-down' },
  { op: 'back', label: 'На задний план', icon: 'layer-back' },
] as const
export type LayerOp = (typeof LAYER_OPS)[number]['op']

// Пока зажат модификатор, панель пропускает мышь к фото под ней: Shift/Ctrl+щелчок по соседу
// добавляет его к выделению, а не попадает в кнопку панели (у кнопок вариантов с модификатором нет).
// Alt — «выбрать под курсором» (015).
export function passThrough(e: { shiftKey: boolean; ctrlKey: boolean; altKey: boolean; metaKey: boolean }): boolean {
  return e.shiftKey || e.ctrlKey || e.altKey || e.metaKey
}

export const MINI_GAP = 8 // отступ от рамки, px
export const MINI_H = 30 // высота полосы, px

export const ROT_KEEP = 18 // px вокруг центра ⟳, куда полоса не заходит (кружок 12 + область захвата)

// Где стоит полоса (px от угла разворота): под рамкой, иначе над ней, иначе внутри у верхнего края;
// по горизонтали — по центру рамки, не выходя за разворот. w — ширина полосы, px.
// avoid — центр ⟳ (px): полоса его не закрывает. capBottom — низ подписи или заглушки выделенного, мм (037):
// «под рамкой» значит под этой полосой, иначе щелчки по подписи попадают в кнопки. Сначала пробуем те же места по порядку; если ⟳
// мешает — полоса сдвигается вбок вдоль рамки (ближайший к центру свободный сдвиг), иначе встаёт внутри.
export function miniPlace(
  box: Rect,
  ppm: number,
  width: number,
  height: number,
  w: number,
  avoid?: { x: number; y: number },
  capBottom = 0,
): { top: number; left: number } {
  const by = box.y * ppm
  const bh = Math.max(box.y + box.h, capBottom) * ppm - by
  const half = w / 2
  const cx = Math.max(half, Math.min((box.x + box.w / 2) * ppm, width - half))
  const hits = (top: number, left: number) =>
    !!avoid &&
    avoid.x > left - half - ROT_KEEP &&
    avoid.x < left + half + ROT_KEEP &&
    avoid.y > top - ROT_KEEP &&
    avoid.y < top + MINI_H + ROT_KEEP
  const outside: number[] = []
  if (by + bh + MINI_GAP + MINI_H <= height) outside.push(by + bh + MINI_GAP)
  if (by - MINI_GAP - MINI_H >= 0) outside.push(by - MINI_GAP - MINI_H)
  const inside = by + MINI_GAP // у верхнего края внутри — последнее место
  // Сдвиг вбок: край полосы встаёт за круг ⟳; левый и правый варианты, ближе к центру рамки — первым.
  const shifts = avoid
    ? [avoid.x - ROT_KEEP - half, avoid.x + ROT_KEEP + half]
        .filter((l) => l >= half && l <= width - half)
        .sort((a, b) => Math.abs(a - cx) - Math.abs(b - cx))
    : []
  for (const tops of [outside, [inside]]) {
    for (const top of tops) if (!hits(top, cx)) return { top, left: cx }
    for (const top of tops) for (const left of shifts) if (!hits(top, left)) return { top, left }
  }
  return { top: inside, left: cx }
}
