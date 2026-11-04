// Какие группы строки не помещаются и уходят в «⋯» (спека 024). Чистая логика: ширины измеряет вызывающий.

export interface RowGroup {
  id: string
  w: number | undefined // неизвестная ширина считается нулём: группа сначала показывается, потом измеряется
  prio: number // больше — уходит позже; Infinity — никогда
}

// Сумма ширин + зазоры ≤ avail → ничего не прячем. Иначе прячем по возрастанию prio (при равном — правее раньше),
// прибавив один раз место под «⋯» (moreW + gap), пока не влезет или не останутся группы с Infinity.
export function fitGroups(groups: readonly RowGroup[], avail: number, gap: number, moreW: number): Set<string> {
  const hidden = new Set<string>()
  const total = (extra: number) => {
    const shown = groups.filter((g) => !hidden.has(g.id))
    return shown.reduce((s, g) => s + (g.w ?? 0), 0) + gap * Math.max(0, shown.length - 1) + extra
  }
  if (total(0) <= avail) return hidden

  // Порядок ухода: prio по возрастанию, при равенстве — правее (больший индекс) раньше.
  const order = groups
    .map((g, i) => ({ g, i }))
    .filter(({ g }) => g.prio !== Infinity)
    .sort((a, b) => a.g.prio - b.g.prio || b.i - a.i)
  for (const { g } of order) {
    hidden.add(g.id)
    if (total(moreW + gap) <= avail) break
  }
  return hidden
}
