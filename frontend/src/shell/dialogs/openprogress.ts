// Ход прогрева раскладок при открытии альбома (073, решение 073-5): чистые строки и доля для OpenProgress.
// Числа считает Go (open:progress): первое событие приходит не раньше 150 мс счёта, последнее — всегда (total, total).
export interface OpenProgress {
  done: number
  total: number
}

// Доля для <progress max=100>; total неизвестен — null (неопределённая полоса).
export function openPercent(p: OpenProgress | null): number | null {
  if (!p || p.total <= 0) return null
  return Math.min(100, Math.round((p.done / p.total) * 100))
}

export function openLine(p: OpenProgress | null): string {
  return p && p.total > 0 ? `Раскладки: ${Math.min(p.done, p.total)} из ${p.total}` : 'Считаем раскладки…'
}
