// Выбор в окне «Горячие клавиши» (074-29, макет Г): выбор клавиши на карте и выбор строки в списке дают одну картину —
// выбранная команда, её клавиши подсвечены, остальные приглушены. Чистая логика без Svelte.
import { codeOfChord, modsOfChord } from '../../base/keys/chord'
import type { KeyLocked, KeymapEntry, KeymapView } from '../../base/types/keys'

// Слой карты: какие модификаторы «зажаты» (Mod — Ctrl на Windows, ⌘ на Mac).
export interface Mods {
  mod: boolean
  alt: boolean
  shift: boolean
}
export const NO_MODS: Readonly<Mods> = { mod: false, alt: false, shift: false }

// Выбрана команда (строка списка) или клавиша карты; pick — какая из команд клавиши открыта в карточке.
export type Sel = { kind: 'cmd'; id: string } | { kind: 'key'; chord: string; pick: string | null } | null

export interface SelCtx {
  index: Map<string, KeymapEntry[]> // сочетание → команды, старшая область первой (keymap.index)
  locked: KeyLocked[]
  visible: (e: KeymapEntry) => boolean // команда есть на уровне интерфейса
}

// Сочетание клавиши карты на слое.
export const chordOf = (code: string, m: Mods): string =>
  [m.mod && 'Mod', m.alt && 'Alt', m.shift && 'Shift', code].filter(Boolean).join('+')

// Слой по сочетанию: щелчок по «Ctrl+D» включает слой Ctrl.
export function modsOf(chord: string): Mods {
  const m = modsOfChord(chord)
  return { mod: m.includes('Mod'), alt: m.includes('Alt'), shift: m.includes('Shift') }
}

// Причина, почему сочетание служебное (замок на карте); '' — свободно для назначения. Правила — как keymap.ReservedReason в Go.
export function reasonOf(chord: string, locked: KeyLocked[]): string {
  const code = codeOfChord(chord)
  for (const r of locked) {
    if (r.anyMods && code === r.chord) return r.reason
    if (r.prefix && chord.startsWith(r.chord + '+')) return r.reason
    if (!r.anyMods && !r.prefix && chord === r.chord) return r.reason
  }
  return ''
}

// Команды на сочетании, старшая область первой; all — и скрытые на уровне.
export const cmdsAt = (c: SelCtx, chord: string, all = false): KeymapEntry[] =>
  (c.index.get(chord) ?? []).filter((e) => all || c.visible(e))

// Команда выбора: для строки — она сама; для клавиши — выбранная в переключателе или старшая. Служебная клавиша
// и свободная — без команды (у них своя карточка).
export function curId(c: SelCtx, sel: Sel): string | null {
  if (!sel) return null
  if (sel.kind === 'cmd') return sel.id
  if (reasonOf(sel.chord, c.locked)) return null
  const list = cmdsAt(c, sel.chord)
  return (list.find((e) => e.id === sel.pick) ?? list[0])?.id ?? null
}

// Приглушать ли остальные клавиши: выбрана клавиша (любая) или команда с клавишей на этом слое.
export function dimOthers(view: KeymapView, sel: Sel, mods: Mods): boolean {
  if (!sel) return false
  if (sel.kind === 'key') return true
  const e = view.commands.find((x) => x.id === sel.id)
  return !!e && e.keys.some((k) => chordOf(codeOfChord(k), mods) === k)
}

// Одна клавиша карты на слое: что на ней и как показать.
export interface KeyCell {
  chord: string
  cmds: KeymapEntry[] // команды уровня, старшая первой
  why: string // причина замка, '' — не служебная
  selected: boolean // выбрана нажатием или щелчком (рамка)
  hl: boolean // клавиша выбранной команды
}

export function keyCell(c: SelCtx, code: string, mods: Mods, sel: Sel, cur: string | null, view: KeymapView): KeyCell {
  const chord = chordOf(code, mods)
  const e = cur ? view.commands.find((x) => x.id === cur) : undefined
  return {
    chord,
    cmds: cmdsAt(c, chord),
    why: reasonOf(chord, c.locked),
    selected: sel?.kind === 'key' && sel.chord === chord,
    hl: !!e && e.keys.includes(chord),
  }
}

// Области пересекаются, если одна из них «везде» или это режим фона и холст/лента (design §2).
const overlap = (a: string, b: string): boolean => a !== b && (a === 'app' || b === 'app' || a === 'bg' || b === 'bg')

// Команды той же клавиши в пересекающихся областях другого ранга: у кого главнее область, тот и сработает.
export function shadowsOf(view: KeymapView, index: Map<string, KeymapEntry[]>, e: KeymapEntry, chord: string): KeymapEntry[] {
  const rank = new Map(view.scopes.map((s) => [s.id, s.rank]))
  const r = (x: KeymapEntry) => rank.get(x.scope) ?? 0
  return (index.get(chord) ?? []).filter((o) => o.id !== e.id && overlap(o.scope, e.scope) && r(o) !== r(e))
}

export const rankOf = (view: KeymapView, e: KeymapEntry): number => view.scopes.find((s) => s.id === e.scope)?.rank ?? 0
