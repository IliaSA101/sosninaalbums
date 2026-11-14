// Общее для окна «Папки» (024, 054, 093): ключ пути и строка состояния. Дерево и фильтр — tree.ts.
import { foldPath } from '../../base/core/platform'

// Ключ пути: регистр — по ОС (foldPath: на Linux различается), разделитель «/», без завершающего «/».
export function folderKey(p: string): string {
  return foldPath(p).replaceAll('\\', '/').replace(/\/+$/, '')
}

// Строка окна «Папки»: «В ленте 162 из 162 фото» — и при нескрытых папках, чтобы высота окна не менялась.
export const inFeedText = (shown: number, total: number): string => `В ленте ${shown} из ${total} фото`

// Есть откуда брать фото: папки или отдельные фото (097). Нет — пустая лента зовёт добавить, кнопка «+ Папка с фото».
export const hasSources = (v: { folders: readonly unknown[]; loose?: readonly unknown[] }): boolean =>
  v.folders.length > 0 || (v.loose ?? []).length > 0
