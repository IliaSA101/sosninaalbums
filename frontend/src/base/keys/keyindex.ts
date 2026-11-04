// Индекс «сочетание → команды» по виду от Go (чистая часть реестра; хранилище — keymap.svelte.ts).
import type { KeymapEntry, KeymapView } from '../types/keys'
import { canonChord } from './chord'

export const EMPTY_KEYMAP: KeymapView = { commands: [], groups: [], scopes: [], locked: [], custom: {}, changed: 0 }

// Команды по сочетанию: сначала область старше (rank ↓), внутри области — порядок каталога.
// Сочетание, не прошедшее Canon (испорченный вид), в индекс не попадает.
export function buildIndex(view: KeymapView): Map<string, KeymapEntry[]> {
  const rank = new Map(view.scopes.map((s) => [s.id, s.rank]))
  const out = new Map<string, KeymapEntry[]>()
  for (const e of view.commands) {
    for (const k of e.keys) {
      const c = canonChord(k)
      if (!c) continue
      const list = out.get(c)
      if (!list) out.set(c, [e])
      else if (!list.includes(e)) list.push(e)
    }
  }
  const r = (e: KeymapEntry) => rank.get(e.scope) ?? 0
  for (const list of out.values()) list.sort((a, b) => r(b) - r(a)) // sort устойчив: порядок каталога держится
  return out
}
