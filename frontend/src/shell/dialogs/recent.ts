// Список последних альбомов для стартового окна и «Файл ▸ Последние» (спека 010, design-ui §4).
// Go присылает путь, имя без расширения, наличие файла и время правки (unix-мс); здесь — только подписи.
import type { RecentProject } from '../../base/core/types'

export interface RecentRow {
  path: string
  name: string
  date: string // «сегодня, 14:05» · «вчера» · «12 сентября 2026»; у пропавшего пусто
  missing: boolean
  tip: string // подсказка: путь, у пропавшего — «Файл не найден»
}

const MONTHS = ['января', 'февраля', 'марта', 'апреля', 'мая', 'июня', 'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря']

const pad = (n: number) => String(n).padStart(2, '0')
const dayStart = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()

// Дата правки по местному времени. now передаётся снаружи — под тест.
export function recentDate(ms: number, now: number): string {
  if (!ms || ms <= 0) return ''
  const d = new Date(ms)
  const days = Math.round((dayStart(new Date(now)) - dayStart(d)) / 86_400_000)
  const hm = `${pad(d.getHours())}:${pad(d.getMinutes())}`
  if (days === 0) return `сегодня, ${hm}`
  if (days === 1) return 'вчера'
  const base = `${d.getDate()} ${MONTHS[d.getMonth()]}`
  return d.getFullYear() === new Date(now).getFullYear() ? base : `${base} ${d.getFullYear()}`
}

// Имя для списка: пустое (путь без имени) заменяется на «Без имени».
export const recentName = (r: Pick<RecentProject, 'name' | 'path'>): string => r.name.trim() || 'Без имени'

export function recentRows(list: readonly RecentProject[], now: number): RecentRow[] {
  return list.map((r) => ({
    path: r.path,
    name: recentName(r),
    date: r.exists ? recentDate(r.modified, now) : '',
    missing: !r.exists,
    tip: r.exists ? r.path : 'Файл не найден',
  }))
}
