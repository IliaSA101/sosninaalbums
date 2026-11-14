// Поиск фото по имени в ленте (0.26-124, design §8.4). Настройка окна, как фильтр отметок: в .salbum, undo и
// localStorage не попадает; другой альбом и перезапуск сбрасывают. Ленту режет store.visible (хук named).
import { tick } from 'svelte'
import { store } from '../../base/core/store.svelte'
import type { Photo } from '../../base/core/types'
import { gallery } from '../gallery/gallery.svelte'
import { modes } from '../../base/prefs/view.svelte'
import { buildIndex, hiddenLabel, hiddenToks, matchIds, missLabel, parseQuery, type NameIndex } from './namequery'

// Набор с паузой: индекс и счёт не пересчитываются на каждой букве. Enter применяет сразу.
const PAUSE_MS = 150

// Индекс — по библиотеке (store.photos), а не по ленте: лента меняется при каждом фильтре, библиотека — редко.
// Строится лениво, при непустом запросе; при сканировании массив меняется пачками — до запроса пересборок нет.
const indexes = new WeakMap<object, { len: number; idx: NameIndex }>()
function indexOf(photos: Photo[]): NameIndex {
  const hit = indexes.get(photos)
  if (hit && hit.len === photos.length) return hit.idx
  const idx = buildIndex(photos)
  indexes.set(photos, { len: photos.length, idx })
  return idx
}

class NameSearch {
  // Текст в поле и тот, что действует (после паузы).
  q = $state('')
  applied = $state('')
  // Поле раскрыто (⌕ нажата или Ctrl+F); пока в нём есть текст кроме пробелов, раскрыто всегда.
  open = $state(false)
  // Поле и кнопка в DOM (привязывает NameSearch.svelte): сюда Ctrl+F ставит фокус.
  field = $state<HTMLInputElement>()
  button = $state<HTMLButtonElement>()
  private timer: ReturnType<typeof setTimeout> | undefined

  toks = $derived(parseQuery(this.applied))
  active = $derived(this.toks.length > 0)
  // Поле держится в строке и не уходит в «⋯» (приоритет Infinity).
  held = $derived(this.open || this.q.trim() !== '')
  private found = $derived(this.active ? matchIds(indexOf(store.photos), this.toks) : null)
  miss = $derived(this.found?.miss ?? [])
  missText = $derived(missLabel(this.miss))
  // Найдено в библиотеке, но в ленте этих фото нет (папка, подборка, «Не в альбоме», фильтр отметок): «· скрыто: 805».
  hidden = $derived.by(() => {
    const f = this.found
    return f && f.hits.length > 0 ? hiddenToks(f.hits, new Set(store.marked.map((p) => p.id))) : []
  })
  hiddenText = $derived(hiddenLabel(this.hidden))
  // Ничего не найдено (в ленте до поиска фото есть).
  empty = $derived(this.active && store.marked.length > 0 && store.visible.length === 0)

  // Стадия ленты (хук named): найденные из переданных.
  apply(photos: Photo[]): Photo[] {
    const f = this.found
    return f ? photos.filter((p) => f.ids.has(p.id)) : photos
  }

  // Ввод: действует через паузу.
  type(v: string) {
    this.q = v
    clearTimeout(this.timer)
    this.timer = setTimeout(() => this.commit(), PAUSE_MS)
  }

  // Enter и сброс: сразу.
  commit() {
    clearTimeout(this.timer)
    this.applied = this.q
  }

  // Очистить, поле остаётся раскрытым.
  clear() {
    clearTimeout(this.timer)
    this.q = ''
    this.applied = ''
  }

  // Свернуть: пустое поле — кнопка ⌕.
  close() {
    this.clear()
    this.open = false
  }

  reset() {
    this.close()
  }

  // Ctrl+F и щелчок по ⌕. Поля в DOM ещё нет, а ⌕ могла быть в «⋯»: ждём перерисовку и кадр (полоса пересчитает ширины).
  async focus() {
    if (store.photos.length === 0 || modes.big || gallery.cur) return
    this.open = true
    await tick()
    await new Promise<void>((r) => requestAnimationFrame(() => r()))
    this.field?.focus()
    this.field?.select()
  }

  // Поле потеряло фокус: без текста (или одни пробелы) сворачивается в ⌕, с текстом держится.
  blur() {
    if (this.q.trim() === '') this.close()
  }

  // Esc в поле: непустое — очистить, пустое — свернуть и вернуть фокус на ⌕.
  async escape() {
    if (this.q.trim() !== '') return this.clear()
    this.open = false
    await tick()
    this.button?.focus()
  }
}

export const nameSearch = new NameSearch()
