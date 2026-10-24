// Ход выгрузки (073): чистые строки и доля для ExportRun. total = 0 — Go ещё не сообщил, сколько файлов.
export interface ExportProgress {
  done: number
  total: number
}

// Доля для <progress max=100>; null — неопределённая полоса («Готовим…»).
export function runPercent(p: ExportProgress | null): number | null {
  if (!p || p.total <= 0) return null
  return Math.min(100, Math.round((p.done / p.total) * 100))
}

export function runLine(p: ExportProgress | null): string {
  return p && p.total > 0 ? `Выгружено ${p.done} из ${p.total}` : 'Готовим…'
}
