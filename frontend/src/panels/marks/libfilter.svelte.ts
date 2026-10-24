// Фильтр ленты по отметкам (спека 004). Настройка окна: в .salbum, undo и localStorage не попадает,
// после перезапуска сброшен. Условия читает store.visible (library/marks.ts: filterPhotos).
import { SvelteSet } from 'svelte/reactivity'
import type { Label } from '../../base/core/types'
import { filterSummary, isActive, type FlagFilter, type MarkFilter } from './marks'

class LibFilter implements MarkFilter {
  // 0 — любая оценка, 1–5 — не ниже, −1 — без оценки.
  minRating = $state(0)
  // Пусто — любой цвет; '' — «без метки». Незнакомая метка не подходит ни под один.
  colors = new SvelteSet<Label>()
  flag = $state<FlagFilter>('all')

  active = $derived(isActive(this))
  summary = $derived(filterSummary(this))

  toggleColor(c: Label) {
    if (this.colors.has(c)) this.colors.delete(c)
    else this.colors.add(c)
  }

  reset() {
    this.minRating = 0
    this.colors.clear()
    this.flag = 'all'
  }
}

export const libfilter = new LibFilter()
