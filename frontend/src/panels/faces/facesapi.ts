// Подгон по лицам (спека 130, design §8): вызов Go и ход поиска. Типы ответа — из биндингов (app.FacesResult,
// model.FaceReport), своих дублей нет. Ошибки Go приходят строкой в catch — показывает вызывающий (тост, как у команд).
import { FitFaces } from '../../../wailsjs/go/app/App'
import type { app, model } from '../../../wailsjs/go/models'
import { EventsOn } from '../../../wailsjs/runtime/runtime'

export type FaceReport = model.FaceReport
export type FacesResult = app.FacesResult

// mode: '' — спросить, 'keep' — сохранить правленые кадры, 'all' — подогнать все. ref — место-образец (U2) или ''.
export type FacesMode = '' | 'keep' | 'all'

export function fitFaces(spreadID: string, ref: string, mode: FacesMode): Promise<FacesResult> {
  return FitFaces(spreadID, ref, mode)
}

// Ход поиска: faces:progress идёт на время вызова, два числа (done, total), как export:progress; первым 0, total,
// последним total, total; на листе без фото события нет. Возвращает отписку.
export function onFacesProgress(fn: (done: number, total: number) => void): () => void {
  return EventsOn('faces:progress', fn)
}
