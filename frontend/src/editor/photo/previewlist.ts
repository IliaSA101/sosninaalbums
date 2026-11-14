// Список окна крупного просмотра (128): чистые функции без Svelte. Список замораживается при открытии:
// фильтр и отметки во время просмотра его не меняют, иначе кадр ушёл бы из-под стрелок.

export type PreviewFrom = 'library' | 'canvas'

export interface PreviewState {
  ids: string[] // замороженный список; с холста — одно фото
  at: number // номер показанного
  from: PreviewFrom
}

export const shownId = (s: PreviewState): string => s.ids[s.at]

// Из ленты: видимые фото в порядке ленты. Фото нет среди видимых (скрыто фильтром) — список из него одного.
export function openList(visible: readonly string[], id: string): PreviewState {
  const at = visible.indexOf(id)
  return at < 0 ? { ids: [id], at: 0, from: 'library' } : { ids: [...visible], at, from: 'library' }
}

// С холста: только это фото, листания нет.
export const canvasList = (id: string): PreviewState => ({ ids: [id], at: 0, from: 'canvas' })

// Шаг по списку: края без кольца, ушедшие из альбома фото пропускаются. Некуда идти — то же состояние.
export function stepList(s: PreviewState, d: 1 | -1, alive: (id: string) => boolean): PreviewState {
  if (s.from !== 'library') return s
  for (let i = s.at + d; i >= 0 && i < s.ids.length; i += d) if (alive(s.ids[i])) return { ...s, at: i }
  return s
}

// Показанное, если оно видно в ленте, иначе ближайшее видимое по списку (при равенстве — следующее); нет ни одного — null.
export function nearestVisible(s: PreviewState, visible: ReadonlySet<string>): string | null {
  for (let k = 0; k < s.ids.length; k++) {
    const next = s.ids[s.at + k]
    if (next !== undefined && visible.has(next)) return next
    const prev = s.ids[s.at - k]
    if (prev !== undefined && visible.has(prev)) return prev
  }
  return null
}

// Что выделить при закрытии: показанное — одно, и только если его нет в выделении; null — выделение не трогать.
export function closeSelection(selected: readonly string[], shown: string): string[] | null {
  return selected.includes(shown) ? null : [shown]
}

// Подпись окна: из ленты «имя · N из M», с холста — имя.
export function caption(name: string, s: PreviewState): string {
  return s.from === 'library' ? `${name} · ${s.at + 1} из ${s.ids.length}` : name
}
