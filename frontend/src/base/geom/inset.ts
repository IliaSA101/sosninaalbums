// Окно рамочки места (079, design §2): фото садится в окно внутри места — место минус отступы рамочки.
// Окно для экрана присылает Go (FrameView.win); здесь зеркало geom.InsetBox/ClampInset только для живого
// просмотра, пока тянут ползунок «Скругление» и ответа Go ещё нет. Паритет — testdata/parity/inset.json.
import type { Rect } from '../core/types'
import type { Outline } from './shape'

export type Ins = [number, number, number, number] // отступы l, t, r, b, мм в осях места

// Доля стороны, которую могут занять два отступа одной оси: окно не меньше 30 % стороны.
export const MAX_INSET_PAIR = 0.7

const pos = (v: number | undefined): number => (v !== undefined && Number.isFinite(v) && v > 0 ? v : 0)

// Отступы места w×h после зажима: мусор и минус — 0, пара оси сверх предела сжимается пропорционально.
export function clampInset(ins: readonly number[], w: number, h: number): Ins {
  const out: Ins = [pos(ins[0]), pos(ins[1]), pos(ins[2]), pos(ins[3])]
  const pair = (a: 0 | 1, b: 2 | 3, side: number) => {
    const lim = MAX_INSET_PAIR * Math.max(side, 0)
    const s = out[a] + out[b]
    if (s > lim) {
      const k = lim / s
      out[a] *= k
      out[b] *= k
    }
  }
  pair(0, 2, w)
  pair(1, 3, h)
  return out
}

// Окно в месте cell формы o при отступах ins: место минус отступы; овал остаётся овалом, скругление
// R становится max(R − min(ins), 0). Без отступов — само место и его форма.
export function insetBox(cell: Rect, o: Outline | undefined, ins: readonly number[]): { win: Rect; outline: Outline | undefined } {
  const c = clampInset(ins, cell.w, cell.h)
  if (c.every((v) => v === 0)) return { win: cell, outline: o }
  const win = { x: cell.x + c[0], y: cell.y + c[1], w: cell.w - c[0] - c[2], h: cell.h - c[1] - c[3] }
  if (o?.oval) return { win, outline: { oval: true } }
  const radius = Math.max((o?.radius ?? 0) - Math.min(...c), 0)
  return { win, outline: radius > 0 ? { radius } : undefined }
}
