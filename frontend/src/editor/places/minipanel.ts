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

// Границы, в которых полоса ищет место: видимая область холста в осях разворота (px от его угла).
// Серое поле вокруг листа — тоже место: полоса не должна прыгать только потому, что лист упёрся в край окна.
export interface MiniView {
  left: number
  top: number
  right: number
  bottom: number
}

type Box = { left: number; top: number; width: number; height: number }

// Окно холста (win) в осях разворота (sheet — его экранный прямоугольник; width — его ширина, px раскладки).
// Масштаб берём из прямоугольника, а не из width: так верно и при трансформации предков.
export function visibleArea(sheet: Box, win: Box, width: number): MiniView | undefined {
  const k = sheet.width > 0 && width > 0 ? sheet.width / width : 0
  if (!(k > 0) || !(win.width > 0) || !(win.height > 0)) return undefined
  return {
    left: (win.left - sheet.left) / k,
    top: (win.top - sheet.top) / k,
    right: (win.left + win.width - sheet.left) / k,
    bottom: (win.top + win.height - sheet.top) / k,
  }
}

// Тот же прямоугольник (перемер без сдвига не должен гонять перерисовку).
export const sameView = (a?: MiniView, b?: MiniView) =>
  a === b || (!!a && !!b && a.left === b.left && a.top === b.top && a.right === b.right && a.bottom === b.bottom)

// Где стоит полоса (px от угла разворота), всегда по центру рамки (зажим краями видимой области view;
// не задана — лист). Порядок: под рамкой; нет места до низа видимой области — над рамкой, выше ⟳ (avoid — его
// центр, px: полоса его не закрывает); и там нет — внутри у верха видимой части (рамка занимает весь экран),
// там полоса при нужде сдвигается вбок от ⟳. w — ширина полосы, px. capBottom — низ подписи или заглушки выделенного,
// мм (037): «под рамкой» значит под этой полосой, иначе щелчки по подписи попадают в кнопки.
export function miniPlace(
  box: Rect,
  ppm: number,
  width: number,
  height: number,
  w: number,
  avoid?: { x: number; y: number },
  capBottom = 0,
  view: MiniView = { left: 0, top: 0, right: width, bottom: height },
): { top: number; left: number } {
  const by = box.y * ppm
  const bottom = Math.max(box.y + box.h, capBottom) * ppm
  const half = w / 2
  const cx = Math.max(view.left + half, Math.min((box.x + box.w / 2) * ppm, view.right - half))
  const hits = (top: number, left: number) =>
    !!avoid &&
    avoid.x > left - half - ROT_KEEP &&
    avoid.x < left + half + ROT_KEEP &&
    avoid.y > top - ROT_KEEP &&
    avoid.y < top + MINI_H + ROT_KEEP
  const under = bottom + MINI_GAP
  if (under + MINI_H <= view.bottom && !hits(under, cx)) return { top: under, left: cx }
  let above = by - MINI_GAP - MINI_H
  if (hits(above, cx)) above = avoid!.y - ROT_KEEP - MINI_H // ⟳ стоит над серединой стороны — полоса выше него
  if (above >= view.top && !hits(above, cx)) return { top: above, left: cx }
  // Внутри у верха видимой части рамки; ⟳ тут стоит у самой стороны — сдвиг вбок, ближе к центру рамки первым.
  const inside = Math.max(by, view.top) + MINI_GAP
  const top = Math.max(view.top, Math.min(inside, view.bottom - MINI_H - MINI_GAP))
  const shifts = avoid
    ? [avoid.x - ROT_KEEP - half, avoid.x + ROT_KEEP + half]
        .filter((l) => l >= view.left + half && l <= view.right - half)
        .sort((a, b) => Math.abs(a - cx) - Math.abs(b - cx))
    : []
  for (const left of [cx, ...shifts]) if (!hits(top, left)) return { top, left }
  return { top, left: cx }
}
