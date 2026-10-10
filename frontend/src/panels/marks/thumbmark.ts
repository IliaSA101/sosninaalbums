// Отметки на миниатюре (спека 004, п. 11–12): звёзды и текст подсказки. Чистая логика под тест.
import type { Mark } from '../../base/core/types'
import { LABEL_VIEW } from './markbar'

const LABEL_NAME: Record<string, string> = Object.fromEntries(LABEL_VIEW.map((c) => [c.id, c.name]))

// «★★★» по оценке; вне 1–5 — пусто или пять.
export function stars(rating: number): string {
  const n = Math.min(5, Math.max(0, Math.trunc(rating)))
  return '★'.repeat(n)
}

// Название метки для подсказки. Незнакомая — её текст из файла (только текстом, не HTML), в одну строку.
function labelText(m: Mark): string {
  if (m.label === '') return ''
  if (m.label === 'other') return (m.labelText ?? '').replace(/\s+/g, ' ').trim() || 'своя метка'
  return LABEL_NAME[m.label] ?? ''
}

const FLAG_NAME = { '-1': 'отклонено', '0': '', '1': 'отобрано' } as const

// Подсказка: «имя · ★★★★ · метка: красная · отобрано · в Lightroom: ★★★★★». Пустые части пропускаются.
// «В Lightroom» — только то, чем отметка из файла отличается от действующей (m.lr задан, если своя расходится).
export function markTitle(name: string, m: Mark): string {
  const parts = [name]
  if (m.rating > 0) parts.push(stars(m.rating))
  const label = labelText(m)
  if (label) parts.push(`метка: ${label}`)
  if (m.flag !== 0) parts.push(FLAG_NAME[m.flag])
  if (m.lr) {
    const d: string[] = []
    if (m.lr.rating !== m.rating) d.push(m.lr.rating > 0 ? stars(m.lr.rating) : 'без оценки')
    if (m.lr.label !== m.label) d.push(labelText(m.lr) ? `метка: ${labelText(m.lr)}` : 'без метки')
    if (m.lr.flag !== m.flag) d.push(FLAG_NAME[m.lr.flag] || 'без флажка')
    if (d.length > 0) parts.push(`в Lightroom: ${d.join(', ')}`)
  }
  return parts.join(' · ')
}
