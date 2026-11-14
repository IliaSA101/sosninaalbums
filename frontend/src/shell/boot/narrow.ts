// Набор функций сузился (prefs.apply): сброс выделений, режимов и окон, которых при новом наборе быть не
// должно (спека 011, design §2.3 п. 4). Состояния принадлежат editor/ и panels/, поэтому сброс здесь, а не в prefs.
import { store } from '../../base/core/store.svelte'
import { COVER_ID } from '../../base/geom/sheet'
import { prefs } from '../../base/prefs/prefs.svelte'
import { bgmode } from '../../editor/backdrop/bgmode.svelte'
import { photoui } from '../../editor/photo/photoui.svelte'
import { placesel } from '../../editor/places/placesel.svelte'
import { edit } from '../../editor/text/edit.svelte'
import { gallery } from '../../panels/gallery/gallery.svelte'
import { gridAct } from '../../panels/layouts/gridact.svelte'
import { libfilter } from '../../panels/marks/libfilter.svelte'

// Галерея сторонки обложки (127): «По фото» и «Встроенные» держатся на «Вариантах», «Мои» — на «Шаблонах»
// (0.26-127-14). gallery.open('spread', …) на обложке молчит, поэтому её не переключаем, а закрываем или сужаем.
// Возвращает true, если открыта галерея сторонки (ветки разворота её не трогают).
function narrowCover(): boolean {
  if (gallery.cur?.spread !== COVER_ID) return false
  if (!prefs.can('variants')) gallery.close()
  else if (!prefs.can('templates') && gallery.source === 'mine') gallery.source = 'all'
  return true
}

// Галерея разворота при сужении набора: без «Шаблонов» вкладка сеток уходит на раскладки «Авто» (или закрывается),
// без «Вариантов» вкладка раскладок «Авто» — на «Шаблоны», если они есть, иначе закрывается.
function narrowSpreadGallery() {
  if (!prefs.can('templates') && gallery.cur?.tab === 'grids') {
    if (prefs.can('variants')) gallery.open('spread', 'layouts')
    else gallery.close()
  }
  if (!prefs.can('variants') && gallery.cur?.tab === 'layouts') {
    if (prefs.can('templates')) gallery.open('spread', 'grids')
    else gallery.close()
  }
}

export function narrow() {
  bgmode.exit()
  placesel.clear()
  edit.clear()
  store.selectedCell = null
  photoui.close()
  if (!narrowCover()) narrowSpreadGallery()
  if (!prefs.can('templates') && gridAct.copyGrid?.mode !== 'decor') gridAct.copyGrid = null
  // «Копировать оформление…» держится на функции «Фоны и стикеры», а не на шаблонах
  if (!prefs.can('decor') && gridAct.copyGrid?.mode === 'decor') gridAct.copyGrid = null
  // Без «Отметок» фильтра в интерфейсе нет: активный сбросить, чтобы он не прятал фото невидимо.
  if (!prefs.can('marks')) libfilter.reset()
}
