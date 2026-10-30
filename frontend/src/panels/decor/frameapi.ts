// Вызовы Go для рамочек (079): тонкий слой над биндингами. Адреса картинок и значения по умолчанию приходят из View.
import { Api } from '../../base/core/store.svelte'
import type { DecorCatalog } from './decor'

// Правка рамочки (model.BorderPatch): пусто/0 — не менять; ручная правка снимает отметку плитки (preset) в Go.
export interface BorderPatch {
  inks?: string[]
  width?: number
  scale?: number
}

export const frameApi = {
  // Каталог рамочек (вид "frame").
  catalog: (): Promise<DecorCatalog> => Api.DecorCatalog('frame') as unknown as Promise<DecorCatalog>,
  // Поставить рамочку на места ids листа spread (itemID "" — снять). Один шаг отмены; та же — холостая.
  apply: (spreadID: string, ids: string[], itemID: string) => Api.ApplyBorder(spreadID, ids, itemID),
  // Цвета, толщина, размер рамочки мест ids; серия с одним merge — один шаг отмены.
  set: (spreadID: string, ids: string[], patch: BorderPatch, merge: string) => Api.SetBorder(spreadID, ids, patch, merge),
  // Прозрачность фото ids (ID фото, не мест), доля 0…1; серия с одним merge — один шаг.
  fade: (ids: string[], fade: number, merge: string) => Api.SetPhotoFade(ids, fade, merge),
}
