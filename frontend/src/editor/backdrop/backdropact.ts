// Действия с фото на фоне: вызовы Go (SetBackdrop, PickBackdropFile, ClearBackdrop) и вход в режим фона.
import * as Api from '../../../wailsjs/go/app/App'
import { backdropOf } from './backdrop'
import { bgmode } from './bgmode.svelte'
import { store } from '../../base/core/store.svelte'
import { sheetIn } from '../../base/geom/sheet'
import type { BackdropScope } from '../../base/core/types'

// После правки с постановкой фото: фон появился или сменился (rev вырос) — сразу в режим фона
// (владелец: кадр по умолчанию почти всегда хочется поправить). Отказ диалога rev не меняет.
async function putThenEdit(spread: string, scope: BackdropScope, op: () => Promise<unknown>) {
  const rev = store.view.rev
  if (!(await store.run(op()))) return
  if (store.view.rev === rev) return
  if (backdropOf(sheetIn(store.view, spread), scope)) bgmode.enter(spread, scope)
}

export const setBackdrop = (spread: string, scope: BackdropScope, photo: string) =>
  putThenEdit(spread, scope, () => Api.SetBackdrop(spread, scope, photo))

export const pickBackdropFile = (spread: string, scope: BackdropScope) =>
  putThenEdit(spread, scope, () => Api.PickBackdropFile(spread, scope))

export const clearBackdrop = (spread: string, scope: BackdropScope) => store.run(Api.ClearBackdrop(spread, scope))

// «Править» в карточке: режим включает только эта кнопка и постановка фото.
export const editBackdrop = (spread: string, scope: BackdropScope) => bgmode.enter(spread, scope)

// Полоса режима и карточка: зеркало и «По области» — одним шагом undo, режим остаётся.
export const flipBackdrop = (spread: string, scope: BackdropScope, axis: 'h' | 'v') => store.run(Api.FlipBackdrop(spread, scope, axis))

export const fitBackdrop = (spread: string, scope: BackdropScope) => store.run(Api.FitBackdrop(spread, scope))
