// «Убрать из альбома» в полосе пропавших (075-13): фото уходит из мест, с фонов, из своих стикеров и рамочек
// одним шагом отмены. Бросок фото в ленту (RemovePhotos) фон оставляет — там другой смысл.
import { Api, store } from '../../base/core/store.svelte'

// Тост «Убрано фото: N» — только если правка прошла (ошибка Go уже показана красным тостом через run).
export async function purgeMissing(photos: readonly string[]): Promise<boolean> {
  const ids = [...new Set(photos)]
  if (ids.length === 0) return false
  if (!(await store.run(Api.PurgePhotos(ids)))) return false
  store.notify(`Убрано фото: ${ids.length}`)
  return true
}
