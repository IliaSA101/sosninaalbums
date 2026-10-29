// Меню правого щелчка по миниатюре ленты (спека 106): пункты «Файл ▸» меню фото (fileGroup, спека 108) и «Где стоит ▸»
// со строками вхождений (общие с меню холста — editor/photoprops/usemenu). Чистая логика; состояние — thumbmenustate.svelte.ts,
// разметка — ThumbMenu.svelte (на MenuList).
import type { FeatureId } from '../../base/prefs/levels'
import type { MenuNode } from '../../base/ui/menulist'
import type { UseSrc } from '../../base/words/whereuse'
import { compareItem, fileGroup, previewItem, similarItem, type HintFn } from '../../editor/photomenu/photomenu'
import { MAX_CELLS } from '../compare/compareset'
import { useRows } from '../../editor/photoprops/usemenu'

export interface ThumbCtx {
  photos: number // выделено фото
  missing: number // из них без файла (миниатюра не загрузилась или Go считает пропавшим)
  used: number // сколько раз стоит одно выделенное (число на плитке, store.useCount); у нескольких не нужно
  lines: UseSrc[] | null // ответ Api.WhereUsed; null — ещё не пришёл
}

// Пункты: «Сравнить» с чертой под ним (2–4 фото, 138); «Крупный просмотр» (одно фото, файл на месте — как в меню фото, 128)
// и «Похожие к этому» (одно фото, 138) с чертой под ними, «Показать в …»,
// «Копировать имя(на) файла(ов)» и «Где стоит ▸» (одно фото, число на плитке > 0).
// «Простой» (нет функции photoMenu) и пустое выделение — меню нет. Недоступное не серое, а скрыто (Р5).
// На плитке «ещё» не к месту: подпись «Где стоит».
export function thumbMenu(c: ThumbCtx, can: (f: FeatureId) => boolean = () => true, hint: HintFn = () => ''): MenuNode[] {
  if (c.photos === 0 || !can('photoMenu')) return []
  const uses = { uses: c.used > 0, usesLabel: 'Где стоит', usesSub: useRows(c.lines, 'Нигде не стоит') }
  const file = fileGroup({ photos: c.photos, missing: c.missing, one: c.photos === 1, ...uses }, can)
  // Сравнение (138): 2–4 фото — «Сравнить» первым; одно — «Похожие к этому» под «Крупным просмотром». Пропавший файл их не прячет.
  const lead: MenuNode[] = []
  if (c.photos === 1) {
    if (c.missing === 0) lead.push(previewItem(hint))
    lead.push(similarItem(hint))
  } else if (c.photos >= 2 && c.photos <= MAX_CELLS) lead.push(compareItem(hint))
  return [...lead, ...file.map((n, i) => (i === 0 && lead.length > 0 ? { ...n, sep: true } : n))]
}
