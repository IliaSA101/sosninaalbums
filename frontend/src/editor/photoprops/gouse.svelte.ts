// Переход к вхождению фото (106): лист открыт, нужный объект выделен или открыт режим фона. Один вход для карточки
// «О снимке», окна «Фото уже в альбоме» и меню ленты (и «Где ещё стоит…» меню холста, спека 108). Подсветка — это
// выделение или режим фона, как у окна копии: отдельной вспышки на холсте нет.
import { store } from '../../base/core/store.svelte'
import type { BackdropScope } from '../../base/core/types'
import { prefs } from '../../base/prefs/prefs.svelte'
import type { UseLine } from '../../base/words/whereuse'
import { guide } from '../../shell/guide/guidestate.svelte'
import { bgmode } from '../backdrop/bgmode.svelte'
import { placesel } from '../places/placesel.svelte'

export function goUse(u: UseLine) {
  // Фон альбома — не лист, а строка «Настроек альбома»: открываем панель и мигаем ею.
  if (u.kind === 'albumBg') {
    void guide.show('settings', 'ui:album-bg')
    return
  }
  // Лист исчез к щелчку — ничего.
  if (!u.spread || !store.sheet(u.spread)) return
  store.goId(u.spread)
  // Объект мог исчезнуть: тогда лист открыт, выделение пустое (без тоста).
  if (!u.pick) return
  if (u.kind === 'sticker') placesel.pickSticker(u.pick, false)
  else if (u.kind === 'backdrop') {
    // Без функции «Фото на фоне» режима нет — остаёмся на листе.
    if (prefs.can('bgPhoto')) bgmode.enter(u.spread, u.pick as BackdropScope)
  } else placesel.select([u.pick])
}
