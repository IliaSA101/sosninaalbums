// Пропавшие и неизвестные рамочки (079 U5): чистая логика. Образец — свой стикер и свой фон: пунктир вместо
// рисунка, подсказка на ячейке, путь к файлу — полоса MissingBar и «Перепривязать…». Окно фото Go считает по
// срезу из альбома, поэтому фото в ячейке стоит как было: рамочка исчезает, сдвига нет.
import type { BorderView } from '../../base/core/types'
import { LIB_MISSING_TEXT } from '../../panels/library/missing'
import { STICKER_NEWER } from '../sticker/sticker'

export type BorderState = 'ok' | 'newer' | 'lost'

export const BORDER_NEWER = STICKER_NEWER // вид из новой версии: тот же текст, что у стикера
export const BORDER_OWN = 'Своя рамочка' // имя своей, которой нет ни в View, ни в каталоге
export const BORDER_LOST_FILE =
  'Файл рамочки не найден. «Перепривязать…» в меню правой кнопки или «Найти…» в полосе сверху найдут его в папке'
export const BORDER_UNLISTED =
  'Этой рамочки нет в списке библиотеки: она скрыта или удалена. В альбоме она осталась и печатается как задумано; «Сменить…» выберет другую'

// Вид новее программы — рисовать нечего; картинка без файла — тоже, но срез в альбоме цел.
export function borderState(b: BorderView, missing: readonly string[]): BorderState {
  if (!b.known) return 'newer'
  return b.pic && !!b.photo && missing.includes(b.photo) ? 'lost' : 'ok'
}

// Подсказка на ячейке; у целой рамочки её нет. Своя из библиотеки ищется загрузкой того же файла (как свои фоны и
// стикеры), без записи в библиотеке (после «Собрать в папку») — папкой.
export function borderTip(b: BorderView, missing: readonly string[], names: Readonly<Record<string, string>> = {}): string | undefined {
  const st = borderState(b, missing)
  if (st === 'newer') return BORDER_NEWER
  if (st === 'ok') return undefined
  if (b.lib) return LIB_MISSING_TEXT
  const name = names[b.photo]
  return name ? `${BORDER_LOST_FILE}: ${name}` : BORDER_LOST_FILE
}

// Меню правой кнопки: картинка рамочки без записи в библиотеке пропала — «Перепривязать…» найдёт её в папке.
export const borderNeedsRelink = (b: BorderView | undefined, missing: readonly string[]): boolean =>
  !!b && !b.lib && borderState(b, missing) === 'lost'
