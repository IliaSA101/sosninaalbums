// Раздел «Рамочка» и строка «Прозрачность» в карточке (079 U3): чистая логика, без Svelte. Что показать по выделенным
// местам (общее значение или «—»), какие места правит каждая строка, что уходит в SetBorder. Зажимы и округление — в Go.
import type { BorderView, SpreadView } from '../../base/core/types'
import { fadePct } from '../backdrop/fade'
import { BORDER_NEWER, BORDER_OWN, borderState } from './borderstate'

export const MIXED_NAME = 'Разные рамочки'
// Пределы ползунков: те же, что зажимает Go (model.ClampBorder, design §1).
export const WIDTH = { min: 0.2, max: 20, step: 0.1 }
export const SIZE = { min: 50, max: 200, step: 1, dflt: 100 } // «Размер, %» своей рамочки

const SAME = 1e-6

export interface BorderTarget {
  ref: string // что принимает Go: ID места (сетка) или фото («Авто»)
  border?: BorderView
}

// Рамочки выделенных: у «Авто» мест нет — рамочек тоже (первая правка закрепит разворот).
export function targetsOf(sp: Pick<SpreadView, 'places'> | undefined, refs: readonly string[]): BorderTarget[] {
  return refs.map((ref) => ({
    ref,
    border: sp?.places?.find((p) => p.id === ref || (p.photo !== '' && p.photo === ref))?.border,
  }))
}

export interface BorderRows {
  all: string[] // места с рамочкой («Убрать»)
  name: string // имя вида или плитки; у разных — MIXED_NAME
  tile: string // ключ плитки каталога (preset или lib), если он один на всех; иначе ''
  own: boolean // все — картинки из библиотеки (подложка плитки — шашечка)
  newer: boolean // у кого-то вид новее программы (known = false): править нечего
  lostLib: number // свои из библиотеки без файла на этом компьютере (ищутся загрузкой того же файла)
  lostFile: number // картинки без записи в библиотеке и без файла (ищутся папкой)
  inkIds: string[][] // места, чей цвет i правит i-й пикер (у вида с одним цветом второго пикера нет)
  inks: string[] // цвета по пикерам: общий или '' («—»)
  inksFirst: string[] // цвета первого места: подложка для пикеров, где значения разные
  widthIds: string[] // встроенные: «Толщина, мм»
  width: number // общая или первого
  widthMixed: boolean
  defWidth: number | null // толщина по умолчанию, если Go её прислал и она одна на всех
  sizeIds: string[] // картинки: «Размер, %»
  size: number // проценты, общий или первого
  sizeMixed: boolean
}

const same = <T>(vals: T[], eq: (a: T, b: T) => boolean = (a, b) => a === b): boolean => vals.every((v) => eq(v, vals[0]))
const near = (a: number, b: number) => Math.abs(a - b) < SAME

// Что карточке знать сверх View (U5): ID пропавших фото и имя своей из каталога (View его не несёт).
export interface RowsCtx {
  missing?: readonly string[]
  nameOf?: (lib: string) => string | undefined
}

// Строки раздела; null — ни у кого нет рамочки («Выбрать…»).
export function borderRows(targets: readonly BorderTarget[], ctx: RowsCtx = {}): BorderRows | null {
  const have = targets.flatMap((t) => (t.border ? [{ ref: t.ref, b: t.border }] : []))
  if (have.length === 0) return null
  const known = have.filter((h) => h.b.known)
  const built = known.filter((h) => !h.b.pic)
  const pics = known.filter((h) => h.b.pic)
  const keys = have.map((h) => h.b.preset || h.b.lib)
  const missing = ctx.missing ?? []
  const lost = known.filter((h) => borderState(h.b, missing) === 'lost')
  // Имя: вид новее — подпись про версию; своя без имени в View — из каталога, иначе «Своя рамочка».
  const nameOf = (b: BorderView) => (!b.known ? BORDER_NEWER : b.name || (b.pic ? (ctx.nameOf?.(b.lib) ?? BORDER_OWN) : ''))
  const names = have.map((h) => nameOf(h.b))

  const count = Math.max(0, ...built.map((h) => h.b.inks.length))
  const inkIds = Array.from({ length: count }, (_, i) => built.filter((h) => h.b.inks.length > i).map((h) => h.ref))
  const inkAt = (i: number) => built.filter((h) => h.b.inks.length > i).map((h) => h.b.inks[i])
  const defs = built.map((h) => h.b.defWidth)

  return {
    all: have.map((h) => h.ref),
    name: same(names) ? names[0] : MIXED_NAME,
    tile: keys[0] && same(keys) ? keys[0] : '',
    own: have.every((h) => h.b.pic),
    newer: have.length > known.length,
    lostLib: lost.filter((h) => h.b.lib).length,
    lostFile: lost.filter((h) => !h.b.lib).length,
    inkIds,
    inks: inkIds.map((_, i) => (same(inkAt(i)) ? inkAt(i)[0] : '')),
    inksFirst: Array.from({ length: count }, (_, i) => inkAt(i)[0]),
    widthIds: built.map((h) => h.ref),
    width: built[0]?.b.width ?? 0,
    widthMixed: !same(
      built.map((h) => h.b.width),
      near,
    ),
    defWidth: defs.length > 0 && defs.every((d) => d !== undefined && d > 0 && near(d, defs[0] ?? 0)) ? (defs[0] ?? null) : null,
    sizeIds: pics.map((h) => h.ref),
    size: Math.round((pics[0]?.b.scale ?? 1) * 100),
    sizeMixed: !same(
      pics.map((h) => Math.round(h.b.scale * 100)),
      (a, b) => a === b,
    ),
  }
}

// Цвета для SetBorder после выбора цвета hex в пикере i: остальные пикеры держат общее значение (у разных — цвет
// первого места), потому что Go заменяет массив целиком.
export function inkPatch(rows: Pick<BorderRows, 'inks' | 'inksFirst'>, i: number, hex: string): string[] {
  return rows.inks.map((c, j) => (j === i ? hex : c || rows.inksFirst[j]))
}

export interface FadeRow {
  pct: number // общее значение в процентах или первого
  mixed: boolean
}

// Прозрачность фото photos на листе sp: у «Авто» и сетки она в кадрах (FrameView.fade).
export function fadeRow(sp: Pick<SpreadView, 'photos' | 'frames'> | undefined, photos: readonly string[]): FadeRow {
  const vals = photos.map((id) => fadePct(sp?.frames?.[sp.photos.indexOf(id)]?.fade ?? 0))
  return { pct: vals[0] ?? 0, mixed: !same(vals) }
}
