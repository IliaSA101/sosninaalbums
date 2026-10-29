// Чистая логика окна «Отправить по ссылке» (146, design §5): какой вид показать, строки хода, сводка, срок.
// Решения и тексты ссылки — у Go; здесь только выбор экрана по ShareView и слова интерфейса.
import { plural } from '../../base/words/plural'
import type { SegItem } from '../../base/tpl/Seg.svelte'
import type { LinkState, ShareProgress, ShareView } from './shareapi'
import { BUSY_OTHER, PREPARING, PUBLISHING } from './sharetext'

// Пункт «Отправить по ссылке…» закрыт: нет листов или проект — лист-виньетка (031); идущая своя выгрузка окно открывает (ход), как у «Экспорта».
export const shareShut = (sheets: number, sheetDoc: boolean, sharing: boolean): boolean => (sheets === 0 || sheetDoc) && !sharing

// Экран окна. busyOther и run — до остального; blocked (не сохранён и т. п.) раньше подключения (0.32-146-T5-7).
export type Screen = 'busyOther' | 'run' | 'blocked' | 'connect' | 'other' | 'incomplete' | 'expired' | 'link' | 'form'

export function screenOf(v: ShareView): Screen {
  if (v.busyOther) return 'busyOther'
  if (v.busy) return 'run'
  // Причина мешает выгрузке, но готовую ссылку (копировать, отозвать) она не отменяет: «Сохранить…» — всегда первым,
  // остальные причины — над ссылкой строкой (ShareDialog), экран ссылки остаётся.
  if (v.album.block && (v.album.save || !hasLink(v))) return 'blocked'
  const st: LinkState = v.link?.state ?? 'none'
  const on = !!v.service.host && v.service.token
  // Без подключения править нечего: ссылка (если есть) только копируется, под ней — форма подключения.
  if (!on) return 'connect'
  return LINK_SCREEN[st]
}

// Экран подключённого сервиса по состоянию ссылки (design §3.4)
const LINK_SCREEN: Record<LinkState, Screen> = {
  none: 'form',
  other: 'other',
  incomplete: 'incomplete',
  expired: 'expired',
  gone: 'expired',
  ok: 'link',
  changed: 'link',
}

// Ссылку можно показать (копировать, открыть) без подключения: опубликована и срок не прошёл.
export const hasLink = (v: ShareView): boolean => !!v.link && ['ok', 'changed', 'other'].includes(v.link.state) && !!v.link.url

// Срок: Месяц · 3 месяца · Год (Days: 30 | 90 | 365). Сервис с меньшим пределом срока гасит лишние (0 — предел неизвестен).
export const DAYS = [30, 90, 365] as const
export const DAYS_ITEMS = (max: number): SegItem<number>[] => [
  { id: 30, label: 'Месяц', off: max > 0 && max < 30 },
  { id: 90, label: '3 месяца', off: max > 0 && max < 90 },
  { id: 365, label: 'Год', off: max > 0 && max < 365 },
]

const KEY = 'shareDays'
export const DEFAULT_DAYS = 90

// Допустимый срок: чужое значение — по умолчанию; больше предела сервиса — самый длинный из допустимых.
export function normDays(n: unknown, max = 0): number {
  const d = DAYS.includes(n as 30 | 90 | 365) ? (n as number) : DEFAULT_DAYS
  if (max <= 0 || d <= max) return d
  return [...DAYS].reverse().find((x) => x <= max) ?? DAYS[0]
}

// Последний выбор срока — свойство программы, не альбома (localStorage, как у экспорта).
export function loadDays(max = 0): number {
  try {
    return normDays(JSON.parse(localStorage.getItem(KEY) ?? 'null'), max)
  } catch {
    return normDays(null, max)
  }
}
export function saveDays(d: number) {
  try {
    localStorage.setItem(KEY, JSON.stringify(d))
  } catch {
    // Не сохранилось — не страшно.
  }
}

// Длинная сторона страницы книги (design §3.2) — копия Go `export.ShareLongSide`, в контракте §4 поля нет; рендер целиком в Go.
const LONG_SIDE = 1600

const mb = (bytes: number): string => {
  const v = bytes / 1e6
  return v >= 10 ? String(Math.round(v)) : v.toFixed(1).replace('.', ',').replace(/,0$/, '')
}

// «62 страницы до 1600 px, ≈ 24 МБ»
export function summaryLine(pages: number, bytes: number): string {
  const head = `${pages} ${plural(pages, ['страница', 'страницы', 'страниц'])} до ${LONG_SIDE} px`
  return bytes > 0 ? `${head}, ≈ ${mb(bytes)} МБ` : head
}

// Строка хода: «Готовим страницы: 12 из 62» → «Загружено 18 из 40 · 7 из 16 МБ» → «Публикуем…».
export function runLine(p: ShareProgress | null): string {
  if (!p) return PREPARING
  if (p.phase === 'publish') return PUBLISHING
  if (p.phase === 'pages') return p.total > 0 ? `Готовим страницы: ${p.done} из ${p.total}` : PREPARING
  const files = `Загружено ${p.done} из ${p.total}`
  return p.totalBytes > 0 ? `${files} · ${mb(p.bytes)} из ${mb(p.totalBytes)} МБ` : files
}

// Доля 0…100 для <progress>; null — неопределённая полоса (ждём первое событие, публикация).
export function runPercent(p: ShareProgress | null): number | null {
  if (!p || p.phase === 'publish') return null
  const [a, b] = p.phase === 'upload' && p.totalBytes > 0 ? [p.bytes, p.totalBytes] : [p.done, p.total]
  if (b <= 0) return null
  return Math.max(0, Math.min(100, Math.round((a / b) * 100)))
}

// Пустая причина не показывается; у чужой выгрузки — всегда своя строка.
export const blockText = (v: ShareView): string => (v.busyOther ? BUSY_OTHER : v.album.block)
