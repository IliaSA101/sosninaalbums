// Подключение верхних групп к основе (docs/STRUCTURE.md, правило 9): base/ не импортирует editor/, panels/,
// shell/, поэтому store, prefs и drag зовут их через хуки, а здесь верх регистрирует себя. wire() вызывает
// main.ts до mount; тесты, которым нужен верх (фильтр ленты, обрезка выделений), — в начале файла.
import { store } from '../../base/core/store.svelte'
import { dropHooks } from '../../base/drag/dropact'
import { drag } from '../../base/drag/drag.svelte'
import { dropAsk, dropGrid } from '../../base/drag/dropgrid.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { setBackdrop } from '../../editor/backdrop/backdropact'
import { placesel } from '../../editor/places/placesel.svelte'
import { loadFonts } from '../../editor/text/fonts'
import { freePhotos, pickFor } from '../../panels/library/freephotos'
import { gallery } from '../../panels/gallery/gallery.svelte'
import { gridAct } from '../../panels/layouts/gridact.svelte'
import { folderfilter } from '../../panels/folders/folderfilter.svelte'
import { cachedFilterFolders } from '../../panels/folders/folders'
import { libfilter } from '../../panels/marks/libfilter.svelte'
import { filterPhotos } from '../../panels/marks/marks'
import { ownMarks } from '../../panels/marks/ownmarks.svelte'
import { spreadsel } from '../../panels/timeline/spreadsel.svelte'
import { spineNote } from '../../editor/cover/spinenote.svelte'
import { narrow } from './narrow'

let wired = false
const filterFolders = cachedFilterFolders()

export function wire() {
  if (wired) return
  wired = true
  store.plug({
    applied: (v) => {
      ownMarks.sync(v.marksRev)
      placesel.prune()
      spreadsel.prune(v.spreads.map((s) => s.id))
      gridAct.prune(v.spreads)
      dropAsk.prune(v)
      gallery.sync()
      folderfilter.prune(v.folders)
      // Корешок обложки пересчитывается от числа разворотов: отметка «X → Y», пока обложку не открыли (032).
      spineNote.see(v.cover?.spine ?? null, store.onCover)
    },
    went: () => {
      if (store.onCover) spineNote.dismiss()
      placesel.clear()
      gridAct.cancelAuto()
      gallery.close()
    },
    reloaded: () => {
      folderfilter.reset() // список папок у каждого альбома свой: скрытые от прежнего не переносим (054)
      ownMarks.invalidate()
      spineNote.reset()
    },
    spreadsCleared: () => spreadsel.clear(),
    // Не ждём: ошибка шрифта не должна мешать старту, текст пока рисуется запасным.
    fontsLoaded: (fonts) => void loadFonts(fonts),
    marks: () => ownMarks.map,
    // Список папок читаем только при скрытых: без фильтра правка альбома ленту не трогает.
    folders: (photos) => (folderfilter.active ? filterFolders(store.view.folders, photos, folderfilter.hidden) : photos),
    visible: (photos) => filterPhotos(photos, ownMarks.map, libfilter),
    free: freePhotos,
    pick: (moveUsed) => pickFor(store, moveUsed, libfilter.active || folderfilter.active),
    selectedRefs: () => placesel.refs,
    select: (ids) => placesel.select(ids),
    // Enter и двойной щелчок по фото в ленте: на ручной разворот — как бросок на лист, точки под курсором нет (042).
    dropGrid: async (spread, ids) => {
      await dropGrid(spread, '', ids, -1, false)
      return true
    },
  })
  prefs.narrowed = narrow
  // Предпросмотр галереи мешал бы целиться: раскладка на экране не та, что в проекте.
  drag.onStart = () => gallery.close()
  dropHooks.setBackdrop = setBackdrop
}
