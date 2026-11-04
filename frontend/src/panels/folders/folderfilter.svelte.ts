// Фильтр ленты по папкам (054, дерево — 093). Как фильтр отметок — настройка окна: в .salbum, undo и localStorage
// не попадает; в отличие от него сбрасывается и при «Новый»/«Открыть», а убранные папки забываются.
// Ленту режет store.inFolders (хук folders → filterTree). Раскрытие ▸/▾ живёт здесь же и сбрасывается так же.
import { SvelteSet } from 'svelte/reactivity'
import type { LooseView } from '../../base/core/types'
import { looseKeys } from './loose'
import { onlyThis, openByDefault, pruneHidden, toggleSubtree, type TreeNode } from './tree'

class FolderFilter {
  // Ключи узлов дерева, чьи свои фото скрыты.
  hidden = new SvelteSet<string>()
  // Узлы, у которых раскрытие не как по умолчанию (openByDefault): папка и ветка «Фото отдельно» раскрыты, остальное свёрнуто.
  flip = new SvelteSet<string>()

  active = $derived(this.hidden.size > 0)

  toggle(node: TreeNode) {
    toggleSubtree(node, this.hidden)
  }

  only(forest: readonly TreeNode[], node: TreeNode) {
    onlyThis(forest, node, this.hidden)
  }

  // Лента пуста из-за фильтра: фото есть, показанных нет.
  allHidden(shown: number, total: number): boolean {
    return this.active && total > 0 && shown === 0
  }

  showAll() {
    this.hidden.clear()
  }

  isOpen(node: TreeNode): boolean {
    return openByDefault(node) !== this.flip.has(node.key)
  }

  toggleOpen(node: TreeNode) {
    if (this.flip.has(node.key)) this.flip.delete(node.key)
    else this.flip.add(node.key)
  }

  // Раскрыть узлы (предков подсвечиваемой строки, новую папку).
  expand(nodes: readonly TreeNode[]) {
    for (const n of nodes) if (!this.isOpen(n)) this.toggleOpen(n)
  }

  // Раскрыть узел по ключу, даже если его ещё нет в дереве (событие пришло раньше чтения ленты).
  openKey(key: string, byDefault: boolean) {
    if (byDefault) this.flip.delete(key)
    else this.flip.add(key)
  }

  // Список папок или отдельных изменился: ключи вне всех добавленных забыть (и при одной папке — подпапку можно скрыть).
  // Ключи отдельных считаются, только если есть что чистить: prune идёт на каждую правку.
  prune(roots: readonly string[], loose: readonly LooseView[] = []) {
    if (this.hidden.size === 0 && this.flip.size === 0) return
    const keys = looseKeys(loose)
    if (this.hidden.size > 0) pruneHidden(this.hidden, roots, keys)
    if (this.flip.size > 0) pruneHidden(this.flip, roots, keys)
  }

  reset() {
    this.hidden.clear()
    this.flip.clear()
  }
}

export const folderfilter = new FolderFilter()
