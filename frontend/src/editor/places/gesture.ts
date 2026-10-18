// Решения жестов по телу фото и полю разворота (спека 009, gestures.md). Чистая логика под тесты.

// Что делает протяжка по телу фото:
//  drag      — перенос фото через drag-систему (обмен, вставка, другой разворот, лента);
//  move      — перенос самой рамки (места);
//  moveGroup — перенос всей группы.
export type BodyAction = 'drag' | 'move' | 'moveGroup'

export interface BodyCtx {
  grid: boolean // «Ручная» (закреплённая сетка); false — «Авто»
  selected: boolean // фото уже выделено
  inGroup: boolean // выделено 2 и больше объектов, и это фото среди них
}

// «Авто»: тело всегда переносит фото (в группе — только это фото, группа ждёт).
// «Ручная»: тело переносит место; член группы — всю группу. `selected` в ответ не входит:
// невыделенное фото выделяет сам жест, поэтому действие то же.
export function bodyAction({ grid, inGroup }: BodyCtx): BodyAction {
  if (!grid) return 'drag'
  return inGroup ? 'moveGroup' : 'move'
}

// Shift или Ctrl/Cmd на щелчке — добавить к выделению; на Mac Ctrl не считается (073): определение в platform.ts.
export { additive } from '../../base/core/platform'
