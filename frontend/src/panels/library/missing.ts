import type { FontFamily, FontUse, SpreadView } from '../../base/core/types'
import { COVER_ID, ownBg } from '../../base/geom/sheet'
import { baseOf } from '../../base/core/copies'

// Подсказка смены диска (Api.SuggestDrive); from === '' — подсказки нет.
export type DriveHint = { from: string; to: string; found: number; total: number }
// Итог поиска в папке (Api.RelinkFolder / RelinkFolderPath), без view.
export type RelinkCounts = { found: number; missing: number; ambiguous: number }

// Свои из библиотеки (048): в альбоме они помнятся по содержимому файла, а не по пути, поэтому на другом компьютере
// «пропавшие» иначе — «Найти пропавшие фото…» их не ищет, а загрузка того же файла находит их сама.
// Go отдаёт ID таких записей в View.libPhotos; в store.missing они лежат вместе с обычными (значок и экспорт общие).
export const LIB_MISSING_TEXT = 'Нет в библиотеке этого компьютера. Загрузите тот же файл — он найдётся сам'

// Пропавшие делятся на обычные фото (ищутся папкой) и свои из библиотеки (ищутся загрузкой).
export function splitMissing(missing: readonly string[], lib: readonly string[] | null | undefined): { photos: string[]; own: string[] } {
  const own = new Set(lib ?? [])
  return { photos: missing.filter((id) => !own.has(id)), own: missing.filter((id) => own.has(id)) }
}

// Строка полосы про свои: сколько и где; без «Найти…», потому что искать нечего — файл надо загрузить.
export function libMissingText(count: number, where = ''): string {
  const base = `Нет в библиотеке этого компьютера: ${count}`
  return `${base}${where ? ` (${where})` : ''} — загрузите те же файлы, они найдутся сами`
}

// Полоса видна, пока есть недостающие фото.
export function showMissing(missing: number): boolean {
  return missing > 0
}

// Сколько фото стоит в альбоме: по файлам (098) — копии одного файла считаются за одно фото.
export function albumPhotos(spreads: Pick<SpreadView, 'photos'>[]): number {
  return fileCount(spreads.flatMap((s) => s.photos ?? []))
}

// Сколько файлов в списке записей (ID копий — «<файл>-<n>»): «Не найдено N фото» считает файлы.
export function fileCount(ids: readonly string[]): number {
  return new Set(ids.map(baseOf)).size
}

// where — «разворот 4» (missingWhere); пусто — не знаем, где (фото нет ни на одном листе).
export function missingText(missing: number, total: number, where = ''): string {
  const base = `Не найдено ${missing} фото из ${Math.max(total, missing)}`
  return where ? `${base} — ${where}` : base
}

// Лист для поиска пропавших: ID фото лежат в photos, фоны — в backdrops[].photo, свои стикеры (048) — в
// stickers[].photo (у процедурного фото нет), рамочки-картинки (079) — в places[].border.photo; кадры и места берут photos.
type Sheet = Pick<SpreadView, 'id' | 'photos'> & {
  backdrops?: { photo: string; album?: boolean }[] | null
  stickers?: { photo?: string }[] | null
  places?: { border?: { photo?: string } }[] | null
}
type Unit = { one: string; many: string; cap: string }

const idsOf = (sp: Sheet): string[] => [
  ...(sp.photos ?? []),
  ...ownBg(sp.backdrops).map((b) => b.photo), // фото фона альбома — не этого листа (098)
  ...(sp.stickers ?? []).flatMap((s) => s.photo ?? []),
  ...(sp.places ?? []).flatMap((p) => p.border?.photo || []),
]

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

// Свои шрифты альбома (View.albumFonts, 082), которых нет в списке этого компьютера. Скрытый × остаётся в списке —
// он на месте, полосы не вызывает. albumFonts ещё нет (старый Go) — пусто.
export function missingFonts(albumFonts: readonly FontUse[] | null | undefined, fonts: readonly Pick<FontFamily, 'id'>[]): FontUse[] {
  const have = new Set(fonts.map((f) => f.id))
  return (albumFonts ?? []).filter((u) => !have.has(u.id))
}

const FONTS_SHOWN = 3 // сколько имён перечислять в полосе
export const NAMELESS_FONT = 'Свой шрифт' // имя неизвестно (шрифт взят из шаблона)

// «Нет шрифтов на этом компьютере: „Lato“, „Bodoni Moda“ — текст показан шрифтом Montserrat». Имена повторяются
// и бывают пустыми: пустые сводятся к одному «Свой шрифт» в конце; длинный список — «и ещё N».
export function fontsMissingText(names: readonly string[]): string {
  const named = [...new Set(names.filter((n) => n !== ''))]
  const items = names.some((n) => n === '') ? [...named, NAMELESS_FONT] : named
  if (items.length === 0) return ''
  const rest = items.length - FONTS_SHOWN
  const list = items
    .slice(0, FONTS_SHOWN)
    .map((n) => `„${n}“`)
    .join(', ')
  return `Нет шрифтов на этом компьютере: ${list}${rest > 0 ? ` и ещё ${rest}` : ''} — текст показан шрифтом Montserrat`
}

// Ключ набора недостающих шрифтов для «Скрыть»: по id (имя может быть пустым), как у фото.
export const fontsKey = (miss: readonly FontUse[], opened: number): string =>
  missingKey(
    miss.map((u) => u.id),
    opened,
  )
