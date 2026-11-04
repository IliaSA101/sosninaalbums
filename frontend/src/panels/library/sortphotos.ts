// Порядок ленты фото (075-14): «По имени» — как пришло от Go (естественный порядок по пути),
// «По дате съёмки» — по ключу shot (EXIF, иначе время файла, одна шкала — считает Go).
import type { Photo } from '../../base/core/types'
import type { LibSort } from '../../base/prefs/view.svelte'

export const LIB_SORTS: { v: LibSort; text: string; title: string }[] = [
  { v: 'name', text: 'По имени', title: 'Порядок как в папках: по имени файла' },
  { v: 'shot', text: 'По дате съёмки', title: 'По времени съёмки; у фото без него — по времени файла' },
]

// Устойчиво: равные по shot остаются в прежнем порядке (имя). 'name' возвращает тот же массив —
// производные списки ленты не пересчитываются зря.
export function sortPhotos(photos: Photo[], mode: LibSort): Photo[] {
  if (mode !== 'shot' || photos.length < 2) return photos
  return photos
    .map((p, i) => ({ p, i, k: p.shot ?? 0 }))
    .sort((a, b) => a.k - b.k || a.i - b.i)
    .map((x) => x.p)
}
