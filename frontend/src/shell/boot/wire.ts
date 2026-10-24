// Подключение верхних групп к основе (docs/STRUCTURE.md, правило 9): base/ не импортирует editor/, panels/,
// shell/, поэтому store, prefs и drag зовут их через хуки, а здесь верх регистрирует себя. wire() вызывает
// main.ts до mount; тесты, которым нужен верх (фильтр ленты, обрезка выделений), — в начале файла.
import { store } from '../../base/core/store.svelte'
import { dropHooks } from '../../base/drag/dropact'
import { drag } from '../../base/drag/drag.svelte'
import { keymap } from '../../base/keys/keymap.svelte'
import { copyAsk, gateCopy } from '../../base/drag/copyask.svelte'
import { dropAsk, dropGrid } from '../../base/drag/dropgrid.svelte'
import { prefs } from '../../base/prefs/prefs.svelte'
import { modes } from '../../base/prefs/view.svelte'
import { setBackdrop } from '../../editor/backdrop/backdropact'
import { placesel } from '../../editor/places/placesel.svelte'
import { checkFaces, loadAndCheck, loadFonts } from '../../editor/text/fonts'
import { autoPick, freePhotos, notRejected, pickFor } from '../../panels/library/freephotos'
import { sortPhotos } from '../../panels/library/sortphotos'
import { gallery } from '../../panels/gallery/gallery.svelte'
import { gridAct } from '../../panels/layouts/gridact.svelte'
import { drop as dropOnBin } from '../../panels/bins/binact'
import { binsel } from '../../panels/bins/binsel.svelte'
import { filterBin } from '../../panels/bins/bins'
import { folderfilter } from '../../panels/folders/folderfilter.svelte'
import { cachedFilterTree } from '../../panels/folders/tree'
import { libfilter } from '../../panels/marks/libfilter.svelte'
import { filterPhotos } from '../../panels/marks/marks'
import { ownMarks } from '../../panels/marks/ownmarks.svelte'
import { spreadsel } from '../../panels/timeline/spreadsel.svelte'
import { spineNote } from '../../editor/cover/spinenote.svelte'
import { narrow } from './narrow'
import { extOpen } from './extopen.svelte'
import { newerAsk } from '../ask/newerask.svelte'

let wired = false
const filterFolders = cachedFilterTree()

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
      copyAsk.prune(v)
      gallery.sync()
      folderfilter.prune(v.folders, v.loose)
      binsel.prune(v.bins ?? []) // удалённая (и отменённая «Новая») подборка больше не показана
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
      binsel.reset() // подборки у каждого альбома свои (083)
      folderfilter.reset() // список папок у каждого альбома свой: скрытые от прежнего не переносим (054)
      ownMarks.invalidate()
      spineNote.reset()
    },
    spreadsCleared: () => spreadsel.clear(),
    // Не ждём: ошибка шрифта не должна мешать старту, текст пока рисуется запасным.
    // Отказ окна при обычном использовании своего шрифта — тост (082-14); событие `fonts` ещё проверяет свежие начертания.
    fontsLoaded: (fonts, fresh) => {
      const notify = (text: string) => store.notify(text, 'error')
      if (fresh) loadAndCheck(fonts, notify)
      else void loadFonts(fonts, notify)
    },
    freshFonts: (files) => void checkFaces(files).catch(() => {}), // откат тихий, как у loadAndCheck
    keymapLoaded: (view) => keymap.apply(view),
    marks: () => ownMarks.map,
    order: (photos) => sortPhotos(photos, modes.libSort),
    // Скрытые — ключи узлов дерева (093): без фильтра правка альбома ленту не трогает.
    folders: (photos) => (folderfilter.active ? filterFolders(photos, folderfilter.hidden) : photos),
    // Подборка (083): после папок; без неё и на «Простом» — тот же массив.
    bin: (photos) => (binsel.shown ? filterBin(photos, binsel.shown, binsel.index) : photos),
    visible: (photos) => filterPhotos(photos, ownMarks.map, libfilter),
    free: freePhotos,
    pick: (moveUsed) => pickFor(store, moveUsed, libfilter.active || folderfilter.active, binsel.curName(store.view.bins ?? [])),
    pickFree: () => autoPick(store.visible, [], store.used, store.backdropUsed, ownMarks.map, true).ids,
    notRejected: (ids) => notRejected(ids, store.byId, ownMarks.map),
    selectedRefs: () => placesel.refs,
    select: (ids) => placesel.select(ids),
    // Enter и двойной щелчок по фото в ленте: на ручной разворот — как бросок на лист, точки под курсором нет (042).
    dropGrid: async (spread, ids) => {
      await dropGrid(spread, '', ids, -1, false)
      return true
    },
    gate: gateCopy, // 098: фото, уже стоящее в альбоме, — окно «Фото уже в альбоме»
    external: (name) => extOpen.request(name), // 112: .salbum, пришедший до подписки на событие
    newer: (info) => newerAsk.show(info), // 112: альбом из более новой версии
  })
  prefs.narrowed = narrow
  // Предпросмотр галереи мешал бы целиться: раскладка на экране не та, что в проекте.
  drag.onStart = () => gallery.close()
  dropHooks.setBackdrop = setBackdrop
  dropHooks.bin = async (ids) => void (await dropOnBin(ids))
}
