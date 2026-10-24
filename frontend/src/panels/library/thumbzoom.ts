// Размер миниатюр колесом над лентой (053): чистая часть; DOM и кадр — в Library.svelte.
import { THUMB_MAX, THUMB_MIN } from '../../base/prefs/view.svelte'
import { isZoomWheel, wheelFactor, type WheelLike } from '../../editor/zoom/zoom'

export type ThumbWheel = WheelLike & { altKey?: boolean }

// Ctrl и Cmd — как зум холста; Alt — привычка Марины (над лентой он ничем не занят).
export const isThumbWheel = (e: ThumbWheel): boolean => isZoomWheel(e) || !!e.altKey

// Размер после одного события колеса. exact — дробный накопитель: щелчок ≈ ×1,13, а малые дельты
// тачпада копятся и не теряются на округлении (округляет setThumb).
export function thumbStep(exact: number, e: ThumbWheel): number {
  const n = exact * Math.pow(wheelFactor(e.deltaY, e.deltaMode), 0.3)
  return Math.max(THUMB_MIN, Math.min(THUMB_MAX, n))
}

// Накопитель колеса между кадрами: дельты складываются, пока кадр не применил размер. Пока кадр ждёт,
// со значением ползунка не сверяемся — иначе вторая дельта до кадра затёрла бы первую.
export function thumbWheelAcc() {
  let exact = 0
  let pending = false
  return {
    // size — текущий размер (modes.thumbSize); возвращает true, если кадр ещё не заказан.
    push(size: number, e: ThumbWheel): boolean {
      // Размер мог поменять ползунок: копим от него, а не от прежнего дробного значения.
      if (!pending && (!exact || Math.round(exact) !== size)) exact = size
      exact = thumbStep(exact, e)
      const first = !pending
      pending = true
      return first
    },
    // Целевой размер на кадре; после этого накопитель снова сверяется с ползунком.
    take(): number {
      pending = false
      return Math.round(exact)
    },
    reset() {
      pending = false
      exact = 0
    },
  }
}
