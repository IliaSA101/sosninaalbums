// Размер миниатюр колесом над лентой (053): чистая часть; DOM и кадр — в Library.svelte.
import { THUMB_MAX, THUMB_MIN } from '../../base/prefs/view.svelte'
import { isZoomWheel, wheelFactor, type WheelLike } from '../../editor/zoom/zoom'

export type ThumbWheel = WheelLike & { altKey?: boolean }

// Ctrl и Cmd — как зум холста; Alt — привычка Марины (над лентой он ничем не занят).
export const isThumbWheel = (e: ThumbWheel): boolean => isZoomWheel(e) || !!e.altKey

// Размер после одного события колеса. exact — дробный накопитель: щелчок ≈ ×1,13, а малые дельты
// тачпада копятся и не теряются на округлении (округляет setThumb). Пределы lo…hi по умолчанию ленты;
// окна плиток оформления (133) дают свои. Одно место на ленту, окна оформления и щипок Mac (134).
export function thumbStep(exact: number, e: ThumbWheel, lo: number = THUMB_MIN, hi: number = THUMB_MAX): number {
  const n = exact * Math.pow(wheelFactor(e.deltaY, e.deltaMode), 0.3)
  return Math.max(lo, Math.min(hi, n))
}

// Щипок Mac (134): ряды перестраиваются, и за пальцами 1:1 не угнаться — множитель жеста в степени 0,5
// (развёл пальцы вдвое — миниатюры ×1,41). Подбирается на Mac по QA M5, место в коде одно.
export const PINCH_THUMB = 0.5

// Размер после шага щипка с множителем f. Мусорный f размер не меняет.
export function thumbScale(exact: number, f: number, lo: number = THUMB_MIN, hi: number = THUMB_MAX): number {
  const n = Number.isFinite(f) && f > 0 ? exact * Math.pow(f, PINCH_THUMB) : exact
  return Math.max(lo, Math.min(hi, n))
}

// Накопитель колеса и щипка между кадрами: дельты складываются, пока кадр не применил размер. Пока кадр ждёт,
// со значением ползунка не сверяемся — иначе вторая дельта до кадра затёрла бы первую.
export function thumbWheelAcc(lo: number = THUMB_MIN, hi: number = THUMB_MAX) {
  let exact = 0
  let pending = false
  function grow(size: number, next: (x: number) => number): boolean {
    // Размер мог поменять ползунок: копим от него, а не от прежнего дробного значения.
    if (!pending && (!exact || Math.round(exact) !== size)) exact = size
    exact = next(exact)
    const first = !pending
    pending = true
    return first
  }
  return {
    // size — текущий размер (modes.thumbSize); возвращает true, если кадр ещё не заказан.
    push(size: number, e: ThumbWheel): boolean {
      return grow(size, (x) => thumbStep(x, e, lo, hi))
    },
    // То же для щипка: f — множитель жеста (thumbScale); колесо и щипок копятся вместе.
    pushScale(size: number, f: number): boolean {
      return grow(size, (x) => thumbScale(x, f, lo, hi))
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
