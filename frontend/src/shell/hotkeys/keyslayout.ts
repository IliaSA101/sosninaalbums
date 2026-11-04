// Раскладка окна «Горячие клавиши» (074-29/30, макет Г): сетка из трёх блоков — карта, список, карточка — в двух видах,
// размер клавиш карты и перетаскиваемые границы. Чистые функции: размеры блоков приходят измеренными из DOM,
// геометрия самой карты (клавиши в долях единицы) — из board.ts.
import { DEF_KEYS_SPLIT, type KeysMode, type KeysSplit } from '../../base/prefs/keyswin'
import { BOARD_H, BOARD_W } from './board'

export const GAP = 8 // толщина границы между блоками, px
// Минимумы блоков в пикселях: ни карта, ни список не схлопываются.
export const MIN = { map: 190, lowA: 130, listB: 190, rightB: 430, card: 110 }
export const U_MIN = 16
export const U_AUTO = 50 // «по умолчанию»: клавиши не крупнее 50 px — остальное достаётся списку
export const U_MAX = 64
export const U_LIST_MAX = 60
export const U_MINI = 34 // мельче — подписи команд нечитаемы, остаются буква и цвет группы
export const LEGEND_GAP = 18
const HEAD_GAP = 6

export type SplitKey = 'ra' | 'rw' | 'rh'

const pct = (r: number): string => (r * 100).toFixed(2)
const clamp = (v: number, lo: number, hi: number): number => (lo > hi ? (lo + hi) / 2 : Math.max(lo, Math.min(hi, v)))

// Дорожки сетки обоих видов (CSS-переменные тела окна); доли — в fr, пределы — в px.
export function tracks(s: KeysSplit, autoH: number | null): { rA: string; cB: string; rB: string } {
  const g = `${GAP}px`
  const rA =
    s.ra === null
      ? `${autoH === null ? 'auto' : autoH + 'px'} ${g} minmax(${MIN.lowA}px,1fr)`
      : `minmax(${MIN.map}px,${pct(s.ra)}fr) ${g} minmax(${MIN.lowA}px,${pct(1 - s.ra)}fr)`
  return {
    rA,
    cB: `minmax(${MIN.listB}px,${pct(s.rw)}fr) ${g} minmax(${MIN.rightB}px,${pct(1 - s.rw)}fr)`,
    rB: `minmax(${MIN.card}px,${pct(1 - s.rh)}fr) ${g} minmax(${MIN.map}px,${pct(s.rh)}fr)`,
  }
}

export const tracksStyle = (s: KeysSplit, autoH: number | null): string => {
  const t = tracks(s, autoH)
  return `--rA:${t.rA};--cB:${t.cB};--rB:${t.rB}`
}

// Граница перетащена: x, y — положение указателя относительно тела окна, w, h — его размер.
export function dragSplit(k: SplitKey, s: KeysSplit, p: { x: number; y: number }, body: { w: number; h: number }): KeysSplit {
  if (k === 'rw') {
    const w = body.w - GAP
    return { ...s, rw: clamp((p.x - GAP / 2) / w, MIN.listB / w, 1 - MIN.rightB / w) }
  }
  const h = body.h - GAP
  const t = (p.y - GAP / 2) / h
  if (k === 'ra') return { ...s, ra: clamp(t, MIN.map / h, 1 - MIN.lowA / h) }
  return { ...s, rh: clamp(1 - t, MIN.map / h, 1 - MIN.card / h) }
}

// Двойной щелчок по границе — «авто»: доля по умолчанию.
export const resetSplit = (k: SplitKey, s: KeysSplit): KeysSplit => ({ ...s, [k]: DEF_KEYS_SPLIT[k] })

export interface FitIn {
  mode: KeysMode
  autoRows: boolean // «Карта крупно» без своей границы: карта берёт столько, сколько нужно клавишам
  paneW: number // блок карты
  paneH: number
  bodyH: number // тело окна целиком (вся высота сетки)
  headH: number // «Список крупно»: шапка карты (слой и легенда)
  legendH: number // высота легенды, когда она под картой
  legendW: number // ширина легенды справа
}

export interface Fit {
  u: number // размер единицы клавиши, px
  below: boolean // «Карта крупно»: легенда ушла под карту
  autoH: number | null // высота карты в «авто» (под клавиши); null — граница своя
}

const unit = (w: number, h: number, max: number): number => Math.max(U_MIN, Math.min(max, Math.floor(Math.min(w / BOARD_W, h / BOARD_H))))

// Размер клавиш по свободному месту карты. «Карта крупно»: легенда справа от карты, как в А; на узком окне клавиши
// справа не дотягивают до своего предела, а под картой (на всю ширину) вырастают — тогда легенда уходит под карту.
export function fitBoard(i: FitIn): Fit {
  if (i.mode === 'list') {
    return { u: unit(i.paneW, i.paneH - i.headH - HEAD_GAP, U_LIST_MAX), below: false, autoH: null }
  }
  const cap = i.autoRows ? U_AUTO : U_MAX
  const availH = i.autoRows ? i.bodyH - GAP - MIN.lowA : i.paneH
  const right = Math.min(cap, unit(i.paneW - i.legendW - LEGEND_GAP, availH, U_MAX))
  const under = Math.min(cap, unit(i.paneW, availH - i.legendH - HEAD_GAP, U_MAX))
  const below = under > right
  const u = below ? under : right
  // «Авто»: карта ровно по клавишам (под легендой, если она ниже карты)
  const autoH = i.autoRows ? Math.ceil(BOARD_H * u) + 2 + (below ? i.legendH + HEAD_GAP : 0) : null
  return { u, below, autoH }
}
