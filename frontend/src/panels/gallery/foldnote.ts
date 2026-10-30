// Строка галереи «Ещё N раскладок с фото на сгибе» (105, В1-в, решение 0.24-q4): правило сгиба прежнее,
// но фотограф должен знать, что «Разброс» бывает и без него. Число считает Go (SpreadView.foldHidden).
import { plural } from '../../base/words/plural'
import { guide } from '../../shell/guide/guidestate.svelte'
import type { FamilyFilter } from './variants'

// Заметка: текст до ссылки и сама ссылка. null — показывать нечего.
export function foldNote(hidden: number | undefined): { text: string; link: string } | null {
  const n = hidden ?? 0
  if (n <= 0) return null
  return {
    text: `Ещё ${n} ${plural(n, ['раскладка', 'раскладки', 'раскладок'])} с фото на сгибе — `,
    link: 'Настройки альбома ▸ Любые фото',
  }
}

// Скрытые раскладки — «Разброс» и центр, то есть «Фигуры»: под другим семейством строка только мешала бы.
export const foldNoteShown = (family: FamilyFilter): boolean => family === 'all' || family === 'figure'

// Ссылка — тем же ходом, что кнопка «Показать» в справочнике F1 (статья «Фото через корешок»): галерея закрывается,
// «Настройки альбома» открыты, переключатель мигает. Подменяется в тестах.
export const foldLink = {
  open: (): void => void guide.show('fold', 'ui:fold-mode'),
}
