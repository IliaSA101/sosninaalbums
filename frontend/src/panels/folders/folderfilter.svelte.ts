// Фильтр ленты по папкам (спека 054). Как фильтр отметок — настройка окна: в .salbum, undo и localStorage
// не попадает; в отличие от него сбрасывается и при «Новый»/«Открыть», а убранные папки забываются.
// Ленту режет store.inFolders (хук folders → filterFolders).
import { SvelteSet } from 'svelte/reactivity'
import { folderKey } from './folders'

class FolderFilter {
  // Ключи скрытых папок (folderKey).
  hidden = new SvelteSet<string>()

  active = $derived(this.hidden.size > 0)

  isHidden(folder: string): boolean {
    return this.hidden.has(folderKey(folder))
  }

  toggle(folder: string) {
    const k = folderKey(folder)
    if (this.hidden.has(k)) this.hidden.delete(k)
    else this.hidden.add(k)
  }

  // Сколько папок списка скрыто (повторная папка считается отдельной строкой окна).
  hiddenIn(folders: readonly string[]): number {
    return folders.filter((f) => this.isHidden(f)).length
  }

  // Скрыты все папки списка (лента пуста из-за фильтра).
  allHidden(folders: readonly string[]): boolean {
    return folders.length > 0 && this.hiddenIn(folders) === folders.length
  }

  showAll() {
    this.hidden.clear()
  }

  // Список папок изменился: убранные забыть; осталась одна — скрывать нечего.
  prune(folders: readonly string[]) {
    if (this.hidden.size === 0) return
    if (folders.length < 2) return this.showAll()
    const have = new Set(folders.map(folderKey))
    for (const k of [...this.hidden]) if (!have.has(k)) this.hidden.delete(k)
  }

  reset() {
    this.showAll()
  }
}

export const folderfilter = new FolderFilter()
