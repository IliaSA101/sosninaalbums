// Набор функций сузился (prefs.apply): сброс выделений, режимов и окон, которых при новом наборе быть не
// должно (спека 011, design §2.3 п. 4). Состояния принадлежат editor/ и panels/, поэтому сброс здесь, а не в prefs.
import { store } from '../../base/core/store.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { bgmode } from '../../editor/backdrop/bgmode.svelte'
import { photoui } from '../../editor/photo/photoui.svelte'
import { placesel } from '../../editor/places/placesel.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { gallery } from '../../panels/gallery/gallery.svelte'
import { gridAct } from '../../panels/layouts/gridact.svelte'
import { libfilter } from '../../panels/marks/libfilter.svelte'

export function narrow() {
  bgmode.exit()
  placesel.clear()
  edit.clear()
  store.selectedCell = null
  photoui.close()
  if (!prefs.can('templates')) {
    if (gallery.cur?.tab === 'grids') {
      if (prefs.can('variants')) gallery.open('spread', 'layouts')
      else gallery.close()
    }
    if (gridAct.copyGrid?.mode !== 'decor') gridAct.copyGrid = null
  }
  // «Копировать оформление…» держится на функции «Фоны и стикеры», а не на шаблонах
  if (!prefs.can('decor') && gridAct.copyGrid?.mode === 'decor') gridAct.copyGrid = null
  // Без «Вариантов» вкладка раскладок «Авто» не нужна: на «Шаблоны», если они есть, иначе закрыть.
  if (!prefs.can('variants') && gallery.cur?.tab === 'layouts') {
    if (prefs.can('templates')) gallery.open('spread', 'grids')
    else gallery.close()
  }
  // Без «Отметок» фильтра в интерфейсе нет: активный сбросить, чтобы он не прятал фото невидимо.
  if (!prefs.can('marks')) libfilter.reset()
}
