// Пять команд разворота из меню «Разворот» (141): можно ли и почему нет. Чистая логика без store; условия
// «разрезать/склеить» берутся из ленты (cutState/joinState) — один источник с чертой, зоной разреза и правым щелчком.
import type { SpreadView } from '../../base/core/types'
import type { Unit } from '../../base/geom/sheet'
import { ASK_TAIL, cutState, joinState, VIG_CUT_WHY } from '../../panels/timeline/timeline'

export interface Gate {
  ok: boolean
  why: string // причина серого; пусто, когда можно
  ask?: string // ручной в паре: действие идёт через вопрос
}

export interface SpreadCmds {
  insertAt: number | null // куда встанет пустой разворот; null — серое
  cut: Gate & { id: string; at: number } // at — номер фото в sp.photos
  join: Gate & { id: string }
}

export const NO_COVER_CUT = 'Обложку не разрезать'
export const NO_COVER_JOIN = 'Обложку не склеить'
export const CUT_PICK = 'Выделите одно фото: разрез пройдёт перед ним'
export const CUT_FIRST = 'Перед первым фото резать нечего'

// Подсказка пункта: серое — причина; доступное с ручным в паре — что будет; иначе без подсказки.
export function gateHint(g?: Gate): string | undefined {
  if (!g) return undefined
  if (!g.ok) return g.why || undefined
  return g.ask ? `${g.ask}: ${ASK_TAIL}` : undefined
}

// curId — текущий лист (null — листов нет), onCover — открыта обложка, sel — фото, выделенные на текущем листе,
// max — предел фото в развороте.
export function spreadCmds(
  spreads: readonly SpreadView[],
  curId: string | null,
  onCover: boolean,
  sel: readonly string[],
  max: number,
  u: Unit,
): SpreadCmds {
  const gray = { ok: false, why: '', id: '' }
  const nothing: SpreadCmds = { insertAt: null, cut: { ...gray, at: -1 }, join: gray }
  if (curId === null) return nothing
  if (onCover) {
    return { insertAt: 0, cut: { ...gray, why: NO_COVER_CUT, id: curId, at: -1 }, join: { ...gray, why: NO_COVER_JOIN, id: curId } }
  }
  const i = spreads.findIndex((s) => s.id === curId)
  if (i < 0) return nothing
  return { insertAt: i + 1, cut: cutGate(spreads[i], sel, u), join: joinGate(spreads, i, max, u) }
}

function cutGate(sp: SpreadView, sel: readonly string[], u: Unit): Gate & { id: string; at: number } {
  const no = (why: string) => ({ ok: false, why, id: sp.id, at: -1 })
  if (sp.kind === 'vignette') return no(VIG_CUT_WHY)
  const j = sel.length === 1 ? sp.photos.indexOf(sel[0]) : -1
  if (j < 0) return no(CUT_PICK)
  if (j === 0) return no(CUT_FIRST)
  const st = cutState(sp, j, u)
  return { ok: st.ok, why: st.why, ask: st.ask, id: sp.id, at: j }
}

function joinGate(spreads: readonly SpreadView[], i: number, max: number, u: Unit): Gate & { id: string } {
  const id = spreads[i].id
  if (i >= spreads.length - 1) return { ok: false, why: `Склеивать не с чем: это последний ${u.one}`, id }
  const st = joinState(spreads as SpreadView[], i, max, u)
  return { ok: st.ok, why: st.why, ask: st.ask, id }
}
