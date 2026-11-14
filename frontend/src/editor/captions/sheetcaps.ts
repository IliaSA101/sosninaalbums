// Общий стиль подписей листа и отступ подписи (030). Чистая логика: решения «что слать в Go», без DOM.
// Размеры и перенос считает Go; здесь только значения полей и заготовка стиля листа.
import type { CaptionView, SheetCaps, SpreadView } from '../../base/core/types'

export const DEFAULT_PAD = 1 // мм, как у Go (model.DefaultPad)
export const MAX_PAD = 20
export const PAD_STEP = 0.5

// Отступ из поля: не число — прежнее значение, иначе 0…20 мм с точностью 0,1 (как зажим Go).
export function clampPad(v: number, prev = DEFAULT_PAD): number {
  if (!Number.isFinite(v)) return prev
  return Math.round(Math.min(MAX_PAD, Math.max(0, v)) * 10) / 10
}

// Действующий отступ подписи; старый Go без поля — 1 мм.
export const padOf = (c: Pick<CaptionView, 'pad'> | undefined): number => c?.pad ?? DEFAULT_PAD

// Образец для включения стиля листа: выделенная подпись, иначе первая на листе.
export function sampleCaption(sp: SpreadView | undefined, selected?: CaptionView): CaptionView | undefined {
  return selected ?? sp?.captions[0]
}

// Стиль листа при включении берёт оформление образца; «ужимать» включено (wrap = false), шаблона нет.
export function startCaps(sample: Pick<CaptionView, 'style' | 'over' | 'pad'>): SheetCaps {
  return { style: { ...sample.style }, pad: padOf(sample), over: sample.over, wrap: false, text: '' }
}

// Шаблон «подписи к каждому фото» из textarea: как есть, без пробелов по краям у пустой строки.
export const templateOf = (s: string): string => (s.trim() === '' ? '' : s)
