// Подключение верхних групп к основе (docs/STRUCTURE.md, правило 9): base/ не импортирует editor/, panels/,
// shell/, поэтому store, prefs и drag зовут их через хуки, а здесь верх регистрирует себя. wire() вызывает
// main.ts до mount; тесты, которым нужен верх (фильтр ленты, обрезка выделений), — в начале файла.
import { store } from '../../base/core/store.svelte'
import { dropHooks } from '../../base/drag/dropact'
import { drag } from '../../base/drag/drag.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { setBackdrop } from '../../editor/backdrop/backdropact'
import { placesel } from '../../editor/places/placesel.svelte'
import { loadFonts } from '../../editor/text/fonts'
import { freePhotos, pickFor } from '../../panels/library/freephotos'
import { gallery } from '../../panels/layouts/gallery.svelte'
import { gridAct } from '../../panels/layouts/gridact.svelte'
import { libfilter } from '../../panels/marks/libfilter.svelte'
import { filterPhotos } from '../../panels/marks/marks'
import { ownMarks } from '../../panels/marks/ownmarks.svelte'
import { spreadsel } from '../../panels/timeline/spreadsel.svelte'
import { narrow } from './narrow'

let wired = false

export function wire() {
  if (wired) return
  wired = true
  store.plug({
    applied: (v) => {
      ownMarks.sync(v.marksRev)
      placesel.prune()
      spreadsel.prune(v.spreads.map((s) => s.id))
      gridAct.prune(v.spreads)
      gallery.sync()
    },
    went: () => {
      placesel.clear()
      gridAct.cancelAuto()
      gallery.close()
    },
    reloaded: () => ownMarks.invalidate(),
    spreadsCleared: () => spreadsel.clear(),
    // Не ждём: ошибка шрифта не должна мешать старту, текст пока рисуется запасным.
    fontsLoaded: (fonts) => void loadFonts(fonts),
    marks: () => ownMarks.map,
    visible: (photos) => filterPhotos(photos, ownMarks.map, libfilter),
    free: freePhotos,
    pick: (moveUsed) => pickFor(store, moveUsed, libfilter.active),
    selectedRefs: () => placesel.refs,
    select: (ids) => placesel.select(ids),
  })
  prefs.narrowed = narrow
  // Предпросмотр галереи мешал бы целиться: раскладка на экране не та, что в проекте.
  drag.onStart = () => gallery.close()
  dropHooks.setBackdrop = setBackdrop
}
