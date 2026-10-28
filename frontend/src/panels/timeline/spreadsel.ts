// Выделение групп в ленте альбома (спека 019 F): чистые функции над списками id.
// Порядок в выделении не важен: удаляются развороты в порядке альбома (spreadsel.svelte.ts).
import { toggleKey } from '../../base/core/platform'

// Ctrl+щелчок: группа входит в выделение или выходит из него.
export function toggle(sel: readonly string[], id: string): string[] {
  return sel.includes(id) ? sel.filter((x) => x !== id) : [...sel, id]
}

// Shift+щелчок: все группы между опорной и той, по которой щёлкнули (включая обе). ids — порядок альбома.
// Нет опоры или её уже нет в альбоме — выделяется только щёлкнутая.
export function range(ids: readonly string[], anchor: string | null, to: string): string[] {
  const b = ids.indexOf(to)
  if (b < 0) return []
  const a = anchor === null ? -1 : ids.indexOf(anchor)
  if (a < 0) return [to]
  return ids.slice(Math.min(a, b), Math.max(a, b) + 1)
}

// Выбросить исчезнувшие развороты (удалены, склеены, открыт другой альбом). Ничего не исчезло — тот же массив.
export function prune(sel: readonly string[], ids: readonly string[]): readonly string[] {
  const have = new Set(ids)
  return sel.every((x) => have.has(x)) ? sel : sel.filter((x) => have.has(x))
}

// Щелчок собирает выделение (Ctrl/Cmd/Shift), а не переходит к группе. Для фото внутри группы — то же.
export function isSelectClick(e: { ctrlKey: boolean; metaKey: boolean; shiftKey: boolean }): boolean {
  return toggleKey(e) || e.shiftKey
}
