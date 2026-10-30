// Разрешения агентов (150, design §7): чистая логика окна «Разрешения» — группы, общее значение, какие согласия нужны.
// Что допустимо и что станет с правкой, решает Go (AgentPerm.can); здесь только вид и порядок вопросов.

import type { SegItem } from '../../base/tpl/Seg.svelte'
import type { AgentPerm, AgentPlace, AgentPlaceLift } from './agents'
import { MODE_LABELS, PLACE_NOFOLDER, PLACE_UNBOUND, placeBind, placeNarrow, placeNarrowRoot, placeNoDisk } from './agentstext'

export type Mode = 'allow' | 'ask' | 'deny'
export type PermGroup = 'read' | 'edit'
export const MODES: readonly Mode[] = ['allow', 'ask', 'deny']
// Инструменты, у которых «Всегда» значит «папку без вопроса»: нужно согласие с местами (design §7.2).
export const FOLDER_TOOLS: readonly string[] = ['add_folder', 'create_album']
// Подпись «Без вопроса — в местах съёмок» у папки и альбома нужна только при «Всегда» (design §7.1): при «Спрашивать» и «Запретить» она врёт.
export const noteShown = (p: AgentPerm): boolean => !!p.note && (p.mode === 'allow' || !FOLDER_TOOLS.includes(p.tool))
// Строка «только с вопросом» (152, класс Q: выгрузка, открытие): «Спрашивать» и «Запретить», «Всегда» нет вовсе — и в окне серое, и пояснение.
export const askOnly = (p: AgentPerm): boolean => p.can.includes('ask') && !p.can.includes('allow')
// У такой строки «Всегда» не серое, а объясняет (как «Спрашивать» у чтения): щелчок значения не меняет, под строкой — why от Go.
// Без why (старый ответ Go) кнопка остаётся серой.
export const allowMissing = (p: AgentPerm): boolean => askOnly(p) && !!p.why
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

// Пункты сегмента: серый — значение, которого Go не принимает (кроме «Спрашивать»: оно живое и объясняет;
// и «Всегда» у строки «только с вопросом» при allowWhy).
export function segItems(can: readonly string[], off: boolean, allowWhy = false): SegItem<Mode>[] {
  return MODES.map((id) => ({
    id,
    label: MODE_LABELS[id],
    off: off || (id !== 'ask' && !(id === 'allow' && allowWhy) && !can.includes(id)),
  }))
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
export const KEY_LIFTED = 'lifted'

// ───────────── Список мест (153) ─────────────
export interface PlaceRow {
  key: string
  dir: string
  right: string // серым справа: метка диска, при отсутствии — «Диска «…» нет» / «Папка не найдена»
  dim: boolean // места нет: строка бледная
  hints: string[] // серые слова под путём
  bind: string // подпись ссылки «Привязать…»; "" — ссылки нет
}

const parts = (dir: string): string[] => dir.split(/[\\/]/).filter(Boolean)
// «Изображения» из `~/Изображения/2026`: имя папки над местом; «E:» из `E:\2026`.
const parentName = (dir: string): string => parts(dir).slice(0, -1).pop() ?? ''
const driveOf = (dir: string): string => /^[A-Za-z]:/.exec(dir)?.[0] ?? ''

// Строки списка: сохранённое из AgentsInfo, а когда пришёл AgentPlacesLive — живое поверх него (по ключу). Убранное в Go
// пропадает из info.places, значит и из списка, даже если live устарел. mac — привязки нет вовсе (0.36): ни слов, ни ссылки.
export function placeRows(places: readonly AgentPlace[], live: readonly AgentPlace[] | null, mac: boolean): PlaceRow[] {
  return places.map((saved) => {
    const l = live?.find((x) => x.key === saved.key)
    const p = l ?? saved
    const hints: string[] = []
    if (p.narrowRoot) hints.push(placeNarrowRoot(driveOf(p.dir)))
    else if (p.narrow) hints.push(placeNarrow(parentName(p.dir)))
    const unbound = !mac && !p.bound
    if (unbound) hints.push(PLACE_UNBOUND)
    // Ссылка — только когда диск по пути известен Go: до ответа AgentPlacesLive «этот диск» мог бы оказаться чужим.
    const bind = unbound && l?.state === 'here' ? placeBind(p.here) : ''
    const right = p.state === 'nodisk' ? placeNoDisk(p.label) : p.state === 'nofolder' ? PLACE_NOFOLDER : p.label
    return { key: p.key, dir: p.dir, right, dim: p.state === 'nodisk' || p.state === 'nofolder', hints, bind }
  })
}

// Поднятые места для заметки: на каждое «куда» — все «откуда» (без повторов).
export function liftGroups(lifted: readonly AgentPlaceLift[]): { to: string; from: string[] }[] {
  const out: { to: string; from: string[] }[] = []
  for (const l of lifted) {
    const g = out.find((x) => x.to === l.to) ?? out[out.push({ to: l.to, from: [] }) - 1]
    if (!g.from.includes(l.from)) g.from.push(l.from)
  }
  return out
}
