// Разрешения агентов (150, design §7): чистая логика окна «Разрешения» — группы, общее значение, какие согласия нужны.
// Что допустимо и что станет с правкой, решает Go (AgentPerm.can); здесь только вид и порядок вопросов.

import type { SegItem } from '../../base/tpl/Seg.svelte'
import type { AgentPerm, AgentPlace } from './agents'
import { MODE_LABELS } from './agentstext'

export type Mode = 'allow' | 'ask' | 'deny'
export type PermGroup = 'read' | 'edit'
export const MODES: readonly Mode[] = ['allow', 'ask', 'deny']
// Инструменты, у которых «Всегда» значит «папку без вопроса»: нужно согласие с местами (design §7.2).
export const FOLDER_TOOLS: readonly string[] = ['add_folder', 'create_album']
// Подпись «Без вопроса — в местах съёмок» у папки и альбома нужна только при «Всегда» (design §7.1): при «Спрашивать» и «Запретить» она врёт.
export const noteShown = (p: AgentPerm): boolean => !!p.note && (p.mode === 'allow' || !FOLDER_TOOLS.includes(p.tool))
// Сколько путей мест показывать в строке, остальные — «и ещё N».
export const PLACES_SHOWN = 2

export type PermChange = { kind: 'tool'; tool: string; mode: Mode } | { kind: 'group'; group: PermGroup; mode: Mode }
export interface Consent {
  edit: boolean // первый выход правок из «Запретить»: ASK_EDIT
  folder: boolean // «Всегда» у папки или альбома: ASK_FOLDER
}

export const isMode = (m: string): m is Mode => m === 'allow' || m === 'ask' || m === 'deny'

export const rowsOf = (perms: readonly AgentPerm[], group: string): AgentPerm[] => perms.filter((p) => p.group === group)

// Общее значение строк или undefined («разные»): сегмент группы тогда ничего не отмечает.
export function commonMode(rows: readonly AgentPerm[]): Mode | undefined {
  const m = new Set(rows.map((r) => r.mode))
  const only = m.size === 1 ? [...m][0] : ''
  return isMode(only) ? only : undefined
}

// «Спрашивать» нет у чтения и запуска: пункт при этом не серый (owner-ux §2.7) — нажатие показывает причину.
export const askMissing = (p: AgentPerm): boolean => !p.can.includes('ask')
export const groupAskMissing = (rows: readonly AgentPerm[]): boolean => rows.length > 0 && rows.every(askMissing)

// Пункты сегмента: серый — значение, которого Go не принимает (кроме «Спрашивать»: оно живое и объясняет).
export function segItems(can: readonly string[], off: boolean): SegItem<Mode>[] {
  return MODES.map((id) => ({ id, label: MODE_LABELS[id], off: off || (id !== 'ask' && !can.includes(id)) }))
}
// Для группы допустимо то, что принимает хоть одна строка.
export const groupCan = (rows: readonly AgentPerm[]): string[] => [...new Set(rows.flatMap((r) => r.can))]

// Строки, которые меняет щелчок: у группы — те, кому значение допустимо.
export function targets(perms: readonly AgentPerm[], ch: PermChange): AgentPerm[] {
  const rows = ch.kind === 'tool' ? perms.filter((p) => p.tool === ch.tool) : rowsOf(perms, ch.group)
  return rows.filter((p) => p.can.includes(ch.mode))
}

// Какие согласия нужны перед изменением. editOn — AgentsInfo.edit (правки уже не все «Запретить»).
export function consentFor(perms: readonly AgentPerm[], editOn: boolean, ch: PermChange): Consent {
  const rows = targets(perms, ch)
  const edit = !editOn && ch.mode !== 'deny' && rows.some((r) => r.group === 'edit' && r.mode === 'deny')
  const folder = ch.mode === 'allow' && rows.some((r) => FOLDER_TOOLS.includes(r.tool) && r.mode !== 'allow')
  return { edit, folder }
}

// Места в строке: первые PLACES_SHOWN и сколько осталось.
export function placesShown(places: readonly AgentPlace[]): { shown: string[]; more: number } {
  return { shown: places.slice(0, PLACES_SHOWN).map((p) => p.dir), more: Math.max(0, places.length - PLACES_SHOWN) }
}

// Ключ подсветки строки: окно открывается на ней (тост «Настроить», ссылки вопроса).
export const rowKey = (tool: string): string => `tool:${tool}`
export const KEY_PLACES = 'places'
export const KEY_SEE = 'see'
