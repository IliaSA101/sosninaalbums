// Исполнение броска: что делать решает чистый dropPlan (drag/drop.ts), здесь только вызовы Go.
import * as Api from '../../../wailsjs/go/app/App'
import { dropPlan, type DropPlan } from './drop'
import { dropGrid } from './dropgrid.svelte'
import { COVER_ID } from '../geom/sheet'
import { store } from '../core/store.svelte'
import { movedText, type Moved } from '../core/strip'
import type { BackdropScope, GridResult, Payload, Target } from '../core/types'

// Фото на фон — действие редактора (editor/backdrop/backdropact.ts); подключает shell/boot/wire.ts.
export const dropHooks = {
  setBackdrop: async (_spread: string, _scope: BackdropScope, _photo: string): Promise<void> => {},
}

export async function dropOn(p: Payload, t: Target) {
  const plan = dropPlan(p, t)
  if (!plan) return
  switch (plan.op) {
    case 'moveSpread':
      // Текущий разворот следует за своим id сам (см. store.apply).
      await store.run(Api.MoveSpread(plan.id, plan.at))
      return
    case 'add':
      await addPhotos(plan)
      return
    case 'swap':
      await store.run(Api.SwapPhotos(plan.a, plan.b))
      return
    case 'newSpread':
      await store.newSpread(plan.at, plan.ids, plan.strip)
      return
    case 'remove':
      await store.run(Api.RemovePhotos(plan.ids))
      return
    case 'place':
      await placePhotos(plan.spread, plan.place, plan.ids, plan.fromSpread)
      return
    case 'reject':
      store.notify(plan.text)
      return
    case 'backdrop':
      await dropHooks.setBackdrop(plan.spread, plan.scope, plan.photo)
      return
    case 'cover':
      if (await store.createCover(plan.photo)) store.selected.clear()
      return
  }
}

// Лист с местами, кроме обложки: у неё бросок на место остаётся прежним (PlacePhotos, без вопроса и плитки).
const isGrid = (id: string) => id !== COVER_ID && !!store.sheet(id)?.grid

// Фото на лист или на фото ленты альбома.
async function addPhotos(plan: Extract<DropPlan, { op: 'add' }>) {
  // Ручной разворот (042): места, плитка, вопрос об остальных — решает Go, не AddPhotos.
  if (isGrid(plan.spread)) return dropGrid(plan.spread, '', plan.ids, plan.frac ?? -1, plan.fromSpread, plan.go)
  // Из ленты — со снятием выделения и тостом о переносе; из разворота — просто правка.
  const op = () => Api.AddPhotos(plan.spread, plan.ids, plan.at)
  const ok = plan.fromSpread ? await store.run(op()) : await store.fromStrip(plan.ids, plan.spread, op)
  if (ok && plan.go) store.goId(plan.spread)
}

// «Перенесено из …» — только по фото, которые реально встали на лист (как dropgrid): не поместившиеся остались на месте.
function landedText(moved: Moved[], spread: string): string {
  const landed = store.sheet(spread)?.photos
  return movedText(
    moved.filter((m) => landed?.includes(m.id)),
    store.view,
  )
}

// Бросок на место: ручной лист — dropGrid, обложка — PlacePhotos. Замена занятого места фото из ленты выгоняет прежнее фото в ленту.
async function placePhotos(spread: string, place: string, ids: string[], fromSpread: boolean) {
  if (isGrid(spread)) return dropGrid(spread, place, ids, -1, fromSpread)
  const owner = store.sheet(spread)?.places?.find((p) => p.id === place)?.photo
  // Go считает в left и вытесненного хозяина места (замена фото из ленты), и фото, не нашедшие места.
  const replaced = !!owner && !store.used.has(ids[0])
  let left = 0
  let named: string[] = []
  const op = async () => {
    const r = (await Api.PlacePhotos(spread, place, ids)) as unknown as GridResult
    left = r.left
    named = r.named ?? []
    return r.view
  }
  // «Перенесено из …» считаем до правки и отдаём в общий тост (как dropgrid): иначе stepToast перетрёт её.
  const moved = fromSpread ? [] : store.movedFrom(ids, spread)
  if (!(await store.run(op()))) return
  // Из ленты выделение снимается; несколько фото разошлись по местам, как у fillPlaces: оно больше не к чему.
  if (!fromSpread || ids.length > 1) store.selected.clear()
  const lost = left - (replaced ? 1 : 0)
  const short = lost > 0 ? `Не хватило мест: ${lost} фото остались ${fromSpread ? 'на прежних местах' : 'в ленте'}` : ''
  const from = landedText(moved, spread)
  store.stepToast(
    replaced ? from : [from, short].filter(Boolean).join('. '),
    replaced ? [short, 'Прежнее фото вернулось в ленту'].filter(Boolean) : [],
    named,
  )
}
