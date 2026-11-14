// Строки «Ч/б, яркость, контраст» в карточке фото (068): чистая логика, без Svelte. Зажим и «нет тона» решает Go;
// здесь — подпись числа, что уходит в SetTone и когда вызов не нужен (холостое — без шага отмены).
import { MAX_TONE } from '../../base/geom/tone'

export type ToneRow = 'bright' | 'contrast'

export interface ToneVals {
  gray: boolean
  bright: number
  contrast: number
}

// Тон фото из вида кадра; нет вида (фото не на развороте) — без тона.
export function toneOf(fv?: { gray?: boolean; bright?: number; contrast?: number }): ToneVals {
  return { gray: !!fv?.gray, bright: fv?.bright ?? 0, contrast: fv?.contrast ?? 0 }
}

// Число справа от подписи: «+12», «−5» (U+2212), «0».
export function signed(v: number): string {
  if (v > 0) return `+${v}`
  if (v < 0) return `−${-v}`
  return '0'
}

// Значение ползунка в допустимые пределы (range и так держит, но событие бывает извне).
export const clampRow = (v: number): number => (Number.isFinite(v) ? Math.min(MAX_TONE, Math.max(-MAX_TONE, Math.round(v))) : 0)

// Тон с новым значением строки (живой показ).
export function withRow(t: ToneVals, row: ToneRow, v: number): ToneVals {
  return { ...t, [row]: clampRow(v) }
}

// Аргументы SetTone(photoID, bright, contrast) после правки строки; null — значение то же, звать Go не нужно.
// Второе число берётся из вида: SetTone принимает оба сразу.
export function toneArgs(cur: ToneVals, row: ToneRow, v: number): [number, number] | null {
  const next = withRow(cur, row, v)
  if (next[row] === cur[row]) return null
  return [next.bright, next.contrast]
}

// «Ч/б» / «Цвет»: SetGray — переключатель «всем или никому», поэтому у одного фото зовём его, только если состояние
// меняется. Повторный щелчок по уже выбранному — false (шага отмены нет).
export function needsGray(cur: ToneVals, wantGray: boolean): boolean {
  return cur.gray !== wantGray
}

// Пока вызов Go из карточки не вернулся, новые щелчки и отпускания ползунка игнорируются: SetGray — переключатель,
// а вид для «needsGray» мог ещё не обновиться — двойной щелчок по «Ч/б» переключил бы дважды (два шага отмены).
// run → false, если предыдущий вызов ещё идёт (fn не звали).
export function pendingGate() {
  let busy = false
  return {
    get busy() {
      return busy
    },
    async run(fn: () => Promise<unknown>): Promise<boolean> {
      if (busy) return false
      busy = true
      try {
        await fn()
      } finally {
        busy = false
      }
      return true
    },
  }
}
