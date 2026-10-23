// Мини-панель выделенного (спека 009, 108): какие значки показать и где поставить запасную полосу.
// Столбец сбоку от фото — minicol.ts. Чистая логика; действия — в MiniPanel.svelte.
import { canFitPhoto, type SelBox } from '../../base/geom/gridedit'
import { actualAspect } from '../../base/geom/frame'
import type { IconName } from '../../base/ui/Icon.svelte'
import type { MenuNode } from '../../base/ui/menulist'
import type { FeatureId } from '../../base/prefs/levels'
import { ACTS, actOk, type ActId } from '../photo/photoacts'
import type { Rect, SpreadView } from '../../base/core/types'
import { whoOf } from '../captions/nametarget'
import { keymap } from '../../base/keys/keymap.svelte'
import { isMac } from '../../base/core/platform'

// Подсказка ⇄ и тост по щелчку: ⇄ и Ctrl+перетаскивание (⌘ на Mac) делают одно и то же (спека 100 п. 6).
export const swapTip = (mac: boolean): string =>
  `Перетащите на другое место — фото поменяются местами (или тащите фото с ${mac ? '⌘' : 'Ctrl'})`

// Значок столбца — действие каталога photoacts.ts (слово, значок, функция уровня записаны там один раз).
export type MiniId = Extract<ActId, 'flipH' | 'flipV' | 'turnCw' | 'tilt' | 'fit' | 'fitPhoto' | 'swap' | 'same' | 'more' | 'delete'>

export interface MiniItem {
  id: MiniId
  label: string // подсказка
  icon: IconName
  on?: boolean // включено (зеркало, «Целиком»)
  badge?: string // число рядом со значком («Удалить (N)» у группы)
  sep?: boolean // линия перед значком
  more?: MenuNode[] // только у 'more': пункты списка «⋯» (подпись, слой, заметка)
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
  tilt?: boolean // режим дуги наклона включён (059, 063)
  missing?: boolean // одиночное фото пропало с диска: дуга ему ни к чему
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

// Порядок пунктов «Слой ▸» (список «⋯» и меню фото).
export const LAYER_OPS = [
  { op: 'front', label: 'На передний план', icon: 'layer-front' },
  { op: 'up', label: 'Выше', icon: 'layer-up' },
  { op: 'down', label: 'Ниже', icon: 'layer-down' },
  { op: 'back', label: 'На задний план', icon: 'layer-back' },
] as const
export type LayerOp = (typeof LAYER_OPS)[number]['op']

// Строка-заметка в конце «⋯»: остальное — в меню правого щелчка.
export const MORE_NOTE = 'Всё остальное — правым щелчком по фото'

// Значок каталога со словом подсказки: слово по умолчанию — из ACTS, длинные подсказки задают сами.
const mk = (id: MiniId, label: string, extra: Partial<MiniItem> = {}): MiniItem => ({ id, label, icon: ACTS[id].icon ?? 'more', ...extra })

const TURN: MiniItem = mk('turnCw', 'Повернуть фото на 90° по часовой стрелке')

// Значок «Удалить»: подпись зависит от того, что выделено.
function deleteItem(c: MiniCtx, n: number): MiniItem {
  if (n > 1) return mk('delete', keymap.tip(`Удалить выделенное (${n})`, 'removeCell'), { badge: String(n) })
  if (c.photos === 0) return mk('delete', keymap.tip('Убрать пустое место', 'removeCell'))
  if (c.grid) return mk('delete', keymap.tip('Убрать фото с места: оно вернётся в ленту, место останется пустым', 'removeCell'))
  return mk('delete', keymap.tip('Убрать фото с разворота: оно вернётся в ленту', 'removeCell'))
}

// Группы значков про фото: зеркало, кадр, поворот и наклон (одно фото) или только зеркало и поворот (несколько).
function photoGroups(c: MiniCtx, solo: boolean): MiniItem[][] {
  if (c.photos === 0) return []
  const flips: MiniItem[] = [
    mk('flipH', 'Зеркало ↔ (по горизонтали)', { on: c.flipH }),
    mk('flipV', 'Зеркало ↕ (по вертикали)', { on: c.flipV }),
  ]
  if (!solo) return [flips, [TURN]]
  const fit: MiniItem[] = [
    mk(
      'fit',
      c.allContain
        ? 'Сейчас целиком, поля — фон страницы. Нажмите: заполнять место, лишнее обрезается'
        : 'Сейчас с обрезкой. Нажмите: фото целиком, поля — фон страницы',
      { on: c.allContain },
    ),
  ]
  if (c.differ) fit.push(mk('fitPhoto', ACTS.fitPhoto.label))
  // Дуга наклона (059, 063): одиночному фото на месте; у пропавшего снимка выпрямлять нечего.
  // Мелкому фото дуга не помещается — кнопка всё равно активна (владелец 2026-10-17): режим покажет угол в карточке.
  const tilt: MiniItem[] = c.missing ? [] : [mk('tilt', 'Наклон фото в рамке: выпрямить горизонт', { on: c.tilt ?? false })]
  return [flips, fit, [TURN, ...tilt]]
}

// Пункты списка «⋯» (владелец 2026-10-23): подпись одному фото, «Слой ▸» в «Ручной»; редкое — не значками.
// Подпись по шаблону листа ведёт к имени, а не создаёт свою подпись (037). Заметка — если меню фото есть на уровне.
function moreNodes(c: MiniCtx, solo: boolean, ok: (id: ActId) => boolean, can: (f: FeatureId) => boolean): MenuNode[] {
  const nodes: MenuNode[] = []
  const cap: ActId = c.name ? 'name' : 'caption'
  if (solo && ok(cap)) nodes.push({ id: cap, label: ACTS[cap].label })
  if (c.grid && ok('layer'))
    nodes.push({ id: 'layer', label: ACTS.layer.label, sub: LAYER_OPS.map((o) => ({ id: 'layer', arg: o.op, label: o.label })) })
  if (nodes.length > 0 && c.places > 0 && can('photoMenu')) nodes.push({ id: 'note', kind: 'note', label: MORE_NOTE, sep: true })
  return nodes
}

// Подсказка «⋯» называет то, что в этом списке есть: «Авто» — подпись, группа — слой, «Ручная» — оба.
function moreTip(more: MenuNode[]): string {
  const word: Partial<Record<string, string>> = { layer: 'слой', name: 'имя' }
  const words = more.filter((m) => m.kind !== 'note').map((m) => word[m.id] ?? 'подпись')
  return `Ещё: ${words.join(', ')}`
}

// Значки столбца сверху вниз (макет Г, владелец 2026-10-23); группы разделяет линия. can — уровень интерфейса
// (не задан — всё доступно); пустая группа пропадает вместе с чертой. «⋯» есть, пока в нём есть что-то кроме заметки.
export function miniItems(c: MiniCtx, can: (f: FeatureId) => boolean = () => true): MiniItem[] {
  const n = c.places + (c.texts ?? 0)
  if (n === 0) return []
  const solo = n === 1 && c.photos === 1
  const ok = (id: ActId) => actOk(id, can)
  const groups: MiniItem[][] = photoGroups(c, solo)
  const arrange: MiniItem[] = []
  if (c.places >= 2) arrange.push(mk('same', 'Одинаковый размер: как у последнего щёлкнутого; при выделении рамкой — как у первого'))
  if (c.grid && solo) arrange.push(mk('swap', swapTip(isMac())))
  if (arrange.length > 0) groups.push(arrange)
  const more = moreNodes(c, solo, ok, can)
  if (more.some((m) => m.kind !== 'note')) groups.push([mk('more', moreTip(more), { more })])

  groups.push([deleteItem(c, n)])

  const shown = groups.map((g) => g.filter((it) => ok(it.id))).filter((g) => g.length > 0)
  return shown.flatMap((g, i) => g.map((it, k) => (i > 0 && k === 0 ? { ...it, sep: true } : it)))
}

// Пока зажат модификатор, панель пропускает мышь к фото под ней: Shift/Ctrl+щелчок по соседу
// добавляет его к выделению, а не попадает в кнопку панели (у кнопок вариантов с модификатором нет).
// Alt — «выбрать под курсором» (015).
export type Mods = { shiftKey: boolean; ctrlKey: boolean; altKey: boolean; metaKey: boolean }

export function passThrough(e: Mods): boolean {
  return e.shiftKey || e.ctrlKey || e.altKey || e.metaKey
}

// События, по которым пересчитывается «панель пропускает мышь» (QA 0.14.1 №1). Признак не копится по keydown/keyup:
// keyup модификатора бывает потерян (Alt уходит в системное меню окна, отпускание вне окна), и панель, возникшая под
// неподвижным курсором, оставалась полупрозрачной — первый щелчок по кнопке уходил в фото. Поэтому каждое событие
// указателя несёт настоящее состояние модификаторов и перекрывает прежний признак; blur/скрытие окна и новое положение
// панели (или другое выделенное) — сброс на состояние последнего указателя (lastPointer; не задан — нет модификаторов).
export type ThroughEv =
  | ({ type: 'keydown' | 'keyup' | 'pointermove' | 'pointerdown' | 'pointerover' } & Mods)
  | { type: 'blur' | 'hidden' }
  | { type: 'place'; lastPointer?: Mods }

export function throughNext(ev: ThroughEv): boolean {
  if (ev.type === 'blur' || ev.type === 'hidden') return false
  if (ev.type === 'place') return !!ev.lastPointer && passThrough(ev.lastPointer)
  return passThrough(ev as Mods)
}

export const MINI_GAP = 8 // отступ от рамки, px
export const MINI_H = 30 // высота полосы, px
// Внутри у нижнего края (владелец 2026-10-17, owner-ux §1.10): между полосой и низом рамки — полоса переноса места (10)
// и полручки (4): средний маркер низа остаётся доступным.
export const MINI_IN_GAP = 14
export const MINI_IN_H = 160 // внутри — только если видимая часть рамки не ниже: ✥, ползунок и полоса не налезают
export const MINI_IN_PAD = 16 // и не уже ширины полосы + 16

export const ROT_KEEP = 18 // px вокруг центра ⟳, куда полоса не заходит (кружок 12 + область захвата)

// Границы, в которых полоса ищет место: видимая область холста в осях разворота (px от его угла).
// Серое поле вокруг листа — тоже место: полоса не должна прыгать только потому, что лист упёрся в край окна.
// Линейки, лента «Добавить» и плавающая карточка в неё не входят (MiniInset).
export interface MiniView {
  left: number
  top: number
  right: number
  bottom: number
  // Низ ленты разворотов под холстом, в тех же осях (095, владелец 2026-10-21): «под фото» можно заходить на ленту,
  // если низ фото виден на холсте. Нет — ленты под холстом нет.
  spill?: number
}

type Box = { left: number; top: number; width: number; height: number }

// Сколько px экрана срезать с окна холста: то, что лежит поверх него и закрывает полосу (063).
export interface MiniInset {
  left: number
  top: number
  right: number
  bottom: number
}

const NO_INSET: MiniInset = { left: 0, top: 0, right: 0, bottom: 0 }

// Окно холста (win) в осях разворота (sheet — его экранный прямоугольник; width — его ширина, px раскладки).
// Масштаб берём из прямоугольника, а не из width: так верно и при трансформации предков.
// inset — линейки и панели поверх окна; съели всё окно — считаем по целому окну.
export function visibleArea(sheet: Box, win: Box, width: number, inset?: MiniInset): MiniView | undefined {
  const k = sheet.width > 0 && width > 0 ? sheet.width / width : 0
  if (!(k > 0) || !(win.width > 0) || !(win.height > 0)) return undefined
  const i = inset && inset.left + inset.right < win.width && inset.top + inset.bottom < win.height ? inset : NO_INSET
  return {
    left: (win.left + i.left - sheet.left) / k,
    top: (win.top + i.top - sheet.top) / k,
    right: (win.left + win.width - i.right - sheet.left) / k,
    bottom: (win.top + win.height - i.bottom - sheet.top) / k,
  }
}

// Линейки сверху/слева, лента слева (вся высота полосой) и плавающая карточка справа → срезы окна. Метки
// `data-mini-edge` ставит разметка; части вне окна не считаются, срез ≥ 0.
export function chromeInset(win: Box, parts: { edge: 'top' | 'left' | 'right'; box: Box }[]): MiniInset {
  const out: MiniInset = { left: 0, top: 0, right: 0, bottom: 0 }
  const wr = win.left + win.width
  const wb = win.top + win.height
  for (const { edge, box } of parts) {
    const r = box.left + box.width
    const b = box.top + box.height
    if (!(box.width > 0) || !(box.height > 0) || r <= win.left || box.left >= wr || b <= win.top || box.top >= wb) continue
    if (edge === 'top') out.top = Math.max(out.top, b - win.top)
    else if (edge === 'left') out.left = Math.max(out.left, r - win.left)
    else out.right = Math.max(out.right, wr - box.left)
  }
  return out
}

// Тот же прямоугольник (перемер без сдвига не должен гонять перерисовку).
export const sameView = (a?: MiniView, b?: MiniView) =>
  a === b || (!!a && !!b && a.left === b.left && a.top === b.top && a.right === b.right && a.bottom === b.bottom && a.spill === b.spill)

// Столбец (спека 108, minicol.ts): ширина, зазор от правого края фото внутри и место, которое он там занимает
// (ширина + зазор + 4 px воздуха). Размеры стоят рядом с CSS MiniPanel.svelte.
export const COL_W = 34
export const COL_IN_GAP = 14
export const COL_IN_W = COL_W + COL_IN_GAP + 4
// Где столбец: справа от фото, слева или внутри у правого края.
export type ColWhere = 'right' | 'left' | 'inright'
// Ряд-запасной: под, над, внутри у низа, внутри у верха; столбец — ColWhere.
export type MiniWhere = 'below' | 'above' | 'inside' | 'top' | ColWhere

// Место полосы внутри у нижнего края видимой части рамки; null — не помещается (уже полосы + 16, ниже 160 px
// или подпись у самого верха). caps — подписи и заглушки выделенного (мм): та, что пересекает полосу у низа, сдвигает её
// выше себя; подпись у верха рамки или снаружи полосе внутри у низа не мешает (решение 21).
function insideAt(box: Rect, ppm: number, w: number, view: MiniView, caps: Rect[]): { top: number; left: number } | null {
  const by = box.y * ppm
  const boxBottom = (box.y + box.h) * ppm
  const vt = Math.max(by, view.top)
  const vb = Math.min(boxBottom, view.bottom)
  const vl = Math.max(box.x * ppm, view.left)
  const vr = Math.min((box.x + box.w) * ppm, view.right)
  if (vr - vl < w + MINI_IN_PAD || vb - vt < MINI_IN_H) return null
  const left = Math.max(vl + w / 2, Math.min((box.x + box.w / 2) * ppm, vr - w / 2))
  const px = caps
    .map((c) => ({ l: c.x * ppm, r: (c.x + c.w) * ppm, t: c.y * ppm, b: (c.y + c.h) * ppm }))
    .filter((c) => c.r > left - w / 2 && c.l < left + w / 2) // только под полосой по горизонтали
    .sort((a, b) => b.t - a.t) // снизу вверх: сдвинули над нижней — проверяем следующую
  let top = vb - MINI_IN_GAP - MINI_H
  for (const c of px) if (c.t < top + MINI_H + MINI_GAP && c.b > top) top = c.t - MINI_GAP - MINI_H
  if (top < vt) return null
  return { top, left }
}

// Нижняя граница полосы «под рамкой»: низ рамки (bottom, px) виден — до низа ленты разворотов (owner-ux §1.9,
// 2026-10-21), ушёл за край холста — до низа видимой области.
const floorOf = (view: MiniView, bottom: number): number =>
  view.spill !== undefined && bottom <= view.bottom ? Math.max(view.bottom, view.spill) : view.bottom

// Где стоит полоса (px от угла разворота), всегда по центру рамки (зажим краями видимой области view;
// не задана — лист). Запасной порядок (владелец 2026-10-17, owner-ux §1.10; основной — столбец, minicol.ts): под рамкой (низ рамки виден — до низа ленты разворотов,
// view.spill; 2026-10-21); нет места до низа видимой области —
// над рамкой, выше ⟳ (avoid — его центр, px: полоса его не закрывает); и там нет (рамка во весь экран) — внутри
// у нижнего края видимой части, по центру; не помещается — прежний запасной 0.13.1: внутри у верха, вбок от ⟳.
// w — ширина полосы, px. capBottom — низ подписи или заглушки выделенного, мм (037): «под рамкой» значит
// под этой полосой, иначе щелчки по подписи попадают в кнопки. caps — боксы подписей и заглушек выделенного, мм: внутри
// у низа полоса встаёт над теми, что её пересекают.
export function miniPlace(
  box: Rect,
  ppm: number,
  width: number,
  height: number,
  w: number,
  avoid?: { x: number; y: number },
  capBottom = 0,
  view: MiniView = { left: 0, top: 0, right: width, bottom: height },
  caps: Rect[] = [],
): { top: number; left: number; where: MiniWhere } {
  const by = box.y * ppm
  const bottom = Math.max(box.y + box.h, capBottom) * ppm
  const half = w / 2
  const mid = (box.x + box.w / 2) * ppm
  const cx = Math.max(view.left + half, Math.min(mid, view.right - half))
  const hits = (top: number, left: number) =>
    !!avoid &&
    avoid.x > left - half - ROT_KEEP &&
    avoid.x < left + half + ROT_KEEP &&
    avoid.y > top - ROT_KEEP &&
    avoid.y < top + MINI_H + ROT_KEEP
  const under = bottom + MINI_GAP
  if (under + MINI_H <= floorOf(view, bottom) && !hits(under, cx)) return { top: under, left: cx, where: 'below' }
  let above = by - MINI_GAP - MINI_H
  if (hits(above, cx)) above = avoid!.y - ROT_KEEP - MINI_H // ⟳ стоит над серединой стороны — полоса выше него
  if (above >= view.top && !hits(above, cx)) return { top: above, left: cx, where: 'above' }
  // Внутри у нижнего края видимой части рамки (фото крупнее экрана).
  const vt = Math.max(by, view.top)
  const inner = insideAt(box, ppm, w, view, caps)
  if (inner && !hits(inner.top, inner.left)) return { ...inner, where: 'inside' }
  // Запасной: внутри у верха видимой части рамки; ⟳ тут стоит у самой стороны — сдвиг вбок, ближе к центру рамки первым.
  const inside = vt + MINI_GAP
  const top = Math.max(view.top, Math.min(inside, view.bottom - MINI_H - MINI_GAP))
  const shifts = avoid
    ? [avoid.x - ROT_KEEP - half, avoid.x + ROT_KEEP + half]
        .filter((l) => l >= view.left + half && l <= view.right - half)
        .sort((a, b) => Math.abs(a - cx) - Math.abs(b - cx))
    : []
  for (const left of [cx, ...shifts]) if (!hits(top, left)) return { top, left, where: 'top' }
  return { top, left: cx, where: 'top' }
}

// Высота ползунка масштаба кадра на холсте (кружок 16 px) и зазоры: у видимого края и над полосой.
const ZOOM_H = 16
const ZOOM_EDGE = 16
const ZOOM_STEP = 4

// Рамка места для ползунка: px от угла разворота, оси места (rot — угол вокруг центра, °).
export type ZoomBox = { x: number; y: number; w: number; h: number; rot?: number }

// Все ли углы прямоугольника ползунка (в осях места, от центра) после поворота места лежат в видимой области
// и не ниже maxY (внутри у низа — выше полосы).
function zoomFits(b: ZoomBox, up: number, hw: number, view: MiniView, maxY: number): boolean {
  const a = ((b.rot ?? 0) * Math.PI) / 180
  const [sn, cs] = [Math.sin(a), Math.cos(a)]
  const cx = b.x + b.w / 2
  const cy = b.y + b.h / 2
  for (const dx of [-hw, hw])
    for (const dy of [b.h / 2 - up - ZOOM_H, b.h / 2 - up]) {
      const x = cx + dx * cs - dy * sn
      const y = cy + dx * sn + dy * cs
      if (x < view.left || x > view.right || y < view.top || y > Math.min(view.bottom, maxY)) return false
    }
  return true
}

// Насколько поднять ползунок масштаба кадра: px от низа рамки места до низа ползунка (не меньше 16 — прежнее место).
// Ползунок не прячется (owner-ux §1.6): внутри у низа он над полосой; иначе (и у столбца сбоку) у нижнего края ВИДИМОЙ части рамки, если
// низ рамки за экраном. У повёрнутого места — вдоль оси места до первой точки, где он целиком в видимой области (view);
// нет такой — прежнее место. Видимой части у рамки нет — прежнее место.
export const zoomLift = (p: { top: number; where: MiniWhere }, b: ZoomBox, view?: MiniView): number => {
  const rot = b.rot ?? 0
  const bottom = b.y + b.h
  const inside = p.where === 'inside'
  if (!rot) {
    if (inside) return bottom - p.top + 8
    if (!view || b.y >= view.bottom || bottom <= view.top || bottom <= view.bottom) return ZOOM_EDGE
    // низ за экраном: у нижнего края видимой части, но не выше верха видимой части рамки
    const top = Math.max(b.y, view.top)
    const lift = bottom - view.bottom + ZOOM_EDGE
    return lift > bottom - top - ZOOM_H ? ZOOM_EDGE : lift
  }
  const base = inside ? MINI_IN_GAP + MINI_H + 8 : ZOOM_EDGE // прежние: у повёрнутого полоса считана в чужих осях
  if (!view) return base
  const hw = Math.min(b.w * 0.6, 170) / 2
  const maxY = inside ? p.top - 8 : Infinity // внутри у низа — над полосой
  for (let up = base; up <= b.h - ZOOM_H - 8; up += ZOOM_STEP) if (zoomFits(b, up, hw, view, maxY)) return up
  return base
}

// Дуге наклона — есть ли место (063): внутри у верха видимой части, над рамкой снаружи или нигде.
// ARC_IN_W = ARC_CMIN (140) + 2·12 (кружок у концов), ARC_OUT_H — над рамкой: 18 (зазор) + 15,5 (высота дуги) + 9 (кружок) + 4.
export const ARC_IN_W = 164
export const ARC_IN_H = 160
export const ARC_OUT_H = 48
export interface ArcRoom {
  mode: 'in' | 'out' | 'none'
  vx: number // px от угла рамки: центр видимой части по x
  vy: number // верх видимой части по y (0, если верх рамки виден)
  vw: number // ширина видимой части
}

// Где встанет полоса в режиме наклона (⟳ спрятан, avoid не задан) — от неё считается место дуги. Не зависит от того, включён
// ли режим сейчас: иначе при выключенном «есть место» → включили → полоса переехала → «нет» → выход (ревью 0.14.1).
export const arcWhere = (
  box: Rect,
  ppm: number,
  width: number,
  height: number,
  w: number,
  capBottom: number,
  view: MiniView,
  caps: Rect[],
): MiniWhere => miniPlace(box, ppm, width, height, w, undefined, capBottom, view, caps).where

export const NO_ROOM: ArcRoom = { mode: 'none', vx: 0, vy: 0, vw: 0 }

// box — рамка места, мм (у повёрнутого — до поворота: оси места); view — видимая область, px разворота; where — где
// встала мини-панель В РЕЖИМЕ НАКЛОНА (⟳ спрятан, avoid не задан): над рамкой дуге снаружи нечего делать (там полоса),
// внутри у верха (запасной) — дуга внутри опускается под полосу (решение 20).
export function arcRoom(box: Rect, ppm: number, rot: number, view: MiniView, where: MiniWhere): ArcRoom {
  const bw = box.w * ppm
  const bh = box.h * ppm
  // Полоса внутри у верха занимает его: дуга — под ней (отступ + полоса + 4 px зазора), если высоты хватает.
  const down = where === 'top' ? MINI_GAP + MINI_H + 4 : 0
  // Столбец внутри у правого края (108) занимает его с зазором: дуга справа от него не лезет.
  const col = where === 'inright' ? COL_IN_W : 0
  // Повёрнутое место: дуга поворачивается с местом, видимую область не считаем; полоса сверху занимает верх и тут.
  if (rot !== 0) return bw - col >= ARC_IN_W && bh - down >= ARC_IN_H ? { mode: 'in', vx: (bw - col) / 2, vy: down, vw: bw - col } : NO_ROOM
  const bx = box.x * ppm
  const by = box.y * ppm
  const vt = Math.max(by, view.top)
  const vb = Math.min(by + bh, view.bottom)
  const vl = Math.max(bx, view.left)
  const vr = Math.min(bx + bw, view.right) - col
  if (vr - vl >= ARC_IN_W && vb - vt - down >= ARC_IN_H) return { mode: 'in', vx: (vl + vr) / 2 - bx, vy: vt - by + down, vw: vr - vl }
  if (by - view.top >= ARC_OUT_H && where !== 'above') return { mode: 'out', vx: bw / 2, vy: 0, vw: bw }
  return NO_ROOM
}
