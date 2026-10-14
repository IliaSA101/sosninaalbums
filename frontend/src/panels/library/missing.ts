import type { SpreadView } from '../../base/core/types'
import { COVER_ID } from '../../base/geom/sheet'

// Подсказка смены диска (Api.SuggestDrive); from === '' — подсказки нет.
export type DriveHint = { from: string; to: string; found: number; total: number }
// Итог поиска в папке (Api.RelinkFolder / RelinkFolderPath), без view.
export type RelinkCounts = { found: number; missing: number; ambiguous: number }

// Полоса видна, пока есть недостающие фото.
export function showMissing(missing: number): boolean {
  return missing > 0
}

// Сколько фото стоит в альбоме (в развороте нет повторов, лишние в проекте не хранятся).
export function albumPhotos(spreads: Pick<SpreadView, 'photos'>[]): number {
  return spreads.reduce((n, s) => n + (s.photos?.length ?? 0), 0)
}

// where — «разворот 4» (missingWhere); пусто — не знаем, где (фото нет ни на одном листе).
export function missingText(missing: number, total: number, where = ''): string {
  const base = `Не найдено ${missing} фото из ${Math.max(total, missing)}`
  return where ? `${base} — ${where}` : base
}

// Лист для поиска пропавших: ID фото лежат в photos, фоны — в backdrops[].photo (кадры и места берут photos).
type Sheet = Pick<SpreadView, 'id' | 'photos'> & { backdrops?: { photo: string }[] | null }
type Unit = { one: string; many: string; cap: string }

const idsOf = (sp: Sheet): string[] => [...(sp.photos ?? []), ...(sp.backdrops ?? []).map((b) => b.photo)]

// Сколько пропавших фото на листе (метка в ленте).
export function missingIn(sp: Sheet, missing: ReadonlySet<string>): number {
  return missing.size === 0 ? 0 : new Set(idsOf(sp).filter((id) => missing.has(id))).size
}

// Место пропавшего фото: лист (id; обложка — COVER_ID), номер разворота (обложка — 0) и ID фото. Порядок — как на ленте.
export type Spot = { sheet: string; no: number; photo: string }

export function missingSpots(sheets: Sheet[], missing: readonly string[]): Spot[] {
  if (missing.length === 0) return []
  const miss = new Set(missing)
  const seen = new Set<string>()
  const out: Spot[] = []
  let no = 0
  for (const sp of sheets) {
    const cover = sp.id === COVER_ID
    if (!cover) no++
    for (const id of idsOf(sp)) {
      if (!miss.has(id) || seen.has(id)) continue
      seen.add(id)
      out.push({ sheet: sp.id, no: cover ? 0 : no, photo: id })
    }
  }
  return out
}

const WHERE_MAX = 8 // сколько номеров перечислять в полосе

// «разворот 4», «развороты 2, 4», «обложка», «обложка, лист 3»; номер — позиция среди разворотов (обложка не считается).
export function missingWhere(spots: Spot[], unit: Pick<Unit, 'one' | 'many'>): string {
  const nums = [...new Set(spots.filter((s) => s.no > 0).map((s) => s.no))]
  const parts: string[] = []
  if (spots.some((s) => s.no === 0)) parts.push('обложка')
  if (nums.length === 1) parts.push(`${unit.one} ${nums[0]}`)
  else if (nums.length > 1) {
    const rest = nums.length - WHERE_MAX
    parts.push(`${unit.many} ${nums.slice(0, WHERE_MAX).join(', ')}${rest > 0 ? ` и ещё ${rest}` : ''}`)
  }
  return parts.join(', ')
}

// Имена файлов пропавших (ID → имя, Api.MissingNames).
export type Names = Readonly<Record<string, string>>

// Строка списка «Показать»: «Разворот 4 · имя.jpg»; имени нет в карте — только место.
export function spotLabel(s: Spot, unit: Pick<Unit, 'cap'>, names: Names): string {
  const at = s.no === 0 ? 'Обложка' : `${unit.cap} ${s.no}`
  const name = names[s.photo]
  return name ? `${at} · ${name}` : at
}

// Подсказка на заглушке пропавшего фото; имени нет в карте — без него.
export function missTip(id: string, names: Names): string {
  const name = names[id]
  return name ? `Файл не найден: ${name}` : 'Файл фото не найден'
}

// Ключ набора пропавших для «Скрыть»: другой набор или другое открытие альбома — полоса снова видна.
export function missingKey(missing: readonly string[], opened: number): string {
  return missing.length === 0 ? '' : `${opened}|${[...missing].sort().join('\n')}`
}

// Скрыта ли полоса: ключ совпал с запомненным.
export const isHidden = (hidden: string, key: string): boolean => key !== '' && hidden === key

// Кнопка смены диска; пусто — подсказки нет (или она ничего не даст).
export function driveLabel(h: DriveHint | null | undefined): string {
  if (!h || !h.from || !h.to || h.found <= 0) return ''
  return `Диск ${h.from}: → ${h.to}: (найдено ${h.found})`
}

// Итоговый тост поиска в папке. Пусто — Go ничего не искал (отмена диалога).
export function relinkSummary(r: RelinkCounts): string {
  if (r.found <= 0 && r.missing <= 0 && r.ambiguous <= 0) return ''
  const parts = [`Найдено ${r.found}`]
  if (r.missing > 0) parts.push(`не найдено ${r.missing}`)
  if (r.ambiguous > 0) parts.push(`неоднозначно ${r.ambiguous}`)
  return parts.join(', ')
}
