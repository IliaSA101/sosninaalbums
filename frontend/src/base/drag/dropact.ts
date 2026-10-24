// Исполнение броска: что делать решает чистый dropPlan (drag/drop.ts), здесь только вызовы Go.
import * as Api from '../../../wailsjs/go/app/App'
import { dropPlan, type DropPlan } from './drop'
import { dropGrid } from './dropgrid.svelte'
import { COVER_ID } from '../geom/sheet'
import { isCopyReq } from '../core/copies'
import { store } from '../core/store.svelte'
import { movedText, type Moved } from '../core/strip'
import type { BackdropScope, GridResult, Payload, Target } from '../core/types'

// Фото на фон — действие редактора (editor/backdrop/backdropact.ts); подключает shell/boot/wire.ts.
export const dropHooks = {
  setBackdrop: async (_spread: string, _scope: BackdropScope, _photo: string): Promise<void> => {},
  // Фото на кнопку «Подборки» (083) — panels/bins/binact.ts; основа панели не импортирует.
  bin: async (_ids: string[]): Promise<void> => {},
}

// false — правка не прошла (живая перестановка 096 возвращает соседей на места); остальные броски — true.
export async function dropOn(p: Payload, t: Target): Promise<boolean> {
  const plan = dropPlan(p, t)
  if (!plan) return false
  if (plan.op === 'moveSpread') return moveSpread(plan.id, plan.at)
  await act(plan)
  return true
}

// Перенесённый разворот становится текущим, как по щелчку по группе (096 сц. 5). После отмены текущий следует
// за id сам (store.apply).
async function moveSpread(id: string, at: number): Promise<boolean> {
  if (!(await store.run(Api.MoveSpread(id, at)))) return false
  store.goId(id)
  return true
}

async function act(plan: DropPlan) {
  switch (plan.op) {
    case 'add':
      await addPhotos(plan)
      return
    case 'move':
      // Без тоста и снятия выделения, как прежний add из разворота; «Авто» или ручной — решает Go.
      await store.run(Api.MovePhoto(plan.id, plan.spread, plan.at))
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
    case 'bin':
      await dropHooks.bin(plan.ids)
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
  const op = (ids: string[]) => Api.AddPhotos(plan.spread, ids, plan.at)
  const ok = plan.fromSpread ? await store.run(op(plan.ids)) : await store.fromStrip(plan.ids, plan.spread, op)
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

// 098: из ленты фото, уже стоящее в альбоме, проходит ворота (окно копии); с холста ID как есть. null — «Отмена».
const gated = (ids: string[], fromSpread: boolean): Promise<string[] | null> => (fromSpread ? Promise.resolve(ids) : store.hooks.gate(ids))

// Фото не занимает места: свободное или запрос копии (новая запись) — вытесненного им хозяина места считает Go в left.
const isFresh = (id: string) => isCopyReq(id) || !store.used.has(id)

// Фото, занимающее место сейчас.
const ownerOf = (spread: string, place: string) => store.sheet(spread)?.places?.find((p) => p.id === place)?.photo

// Бросок на место: ручной лист — dropGrid, обложка — PlacePhotos. Замена занятого места фото из ленты выгоняет прежнее фото в ленту.
async function placePhotos(spread: string, place: string, ids: string[], fromSpread: boolean) {
  if (isGrid(spread)) return dropGrid(spread, place, ids, -1, fromSpread)
  const sent = await gated(ids, fromSpread)
  if (!sent) return
  ids = sent
  // Go считает в left и вытесненного хозяина места (замена фото из ленты), и фото, не нашедшие места.
  const replaced = !!ownerOf(spread, place) && isFresh(ids[0])
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
