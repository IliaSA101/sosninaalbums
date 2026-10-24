// Меню правого щелчка по миниатюре ленты (спека 106): пункты «Файл ▸» меню фото (fileGroup, спека 108) и «Где стоит ▸»
// со строками вхождений (общие с меню холста — editor/photoprops/usemenu). Чистая логика; состояние — thumbmenustate.svelte.ts,
// разметка — ThumbMenu.svelte (на MenuList).
import type { FeatureId } from '../../base/prefs/levels'
import type { MenuNode } from '../../base/ui/menulist'
import type { UseSrc } from '../../base/words/whereuse'
import { fileGroup, previewItem, type HintFn } from '../../editor/photo/photomenu'
import { useRows } from '../../editor/photoprops/usemenu'

export interface ThumbCtx {
  photos: number // выделено фото
  missing: number // из них без файла (миниатюра не загрузилась или Go считает пропавшим)
  used: number // сколько раз стоит одно выделенное (число на плитке, store.useCount); у нескольких не нужно
  lines: UseSrc[] | null // ответ Api.WhereUsed; null — ещё не пришёл
}

// Пункты: «Крупный просмотр» с чертой под ним (одно фото, файл на месте — как в меню фото, 128), «Показать в …»,
// «Копировать имя(на) файла(ов)» и «Где стоит ▸» (одно фото, число на плитке > 0).
// «Простой» (нет функции photoMenu) и пустое выделение — меню нет. Недоступное не серое, а скрыто (Р5).
// На плитке «ещё» не к месту: подпись «Где стоит».
export function thumbMenu(c: ThumbCtx, can: (f: FeatureId) => boolean = () => true, hint: HintFn = () => ''): MenuNode[] {
  if (c.photos === 0 || !can('photoMenu')) return []
  const uses = { uses: c.used > 0, usesLabel: 'Где стоит', usesSub: useRows(c.lines, 'Нигде не стоит') }
  const file = fileGroup({ photos: c.photos, missing: c.missing, one: c.photos === 1, ...uses }, can)
  if (c.photos !== 1 || c.missing > 0) return file
  return [previewItem(hint), ...file.map((n, i) => (i === 0 ? { ...n, sep: true } : n))]
}
