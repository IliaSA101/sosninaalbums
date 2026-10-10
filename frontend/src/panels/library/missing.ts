import type { SpreadView } from '../../base/core/types'

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

export function missingText(missing: number, total: number): string {
  return `Не найдено ${missing} фото из ${Math.max(total, missing)}`
}

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
