// Строка наклона/скругления (057, сценарий 7): одна сессия правки на всю строку — ползунок и поле. Сессию рвёт
// только фокус, ушедший за пределы строки; переход ползунок ↔ поле её сохраняет (иначе два шага отмены).
export function leftRow(row: { contains(n: Node | null): boolean }, related: EventTarget | null): boolean {
  return related === null || !row.contains(related as Node)
}

// Клавиша в числовом поле строки: Enter применяет значение и снимает фокус (иначе Ctrl+Z — нативная отмена
// текста, а уход из поля потом даёт лишнюю правку), Esc — откат к значению из Go и тоже снимает фокус.
export function fieldKey(key: string): 'apply' | 'cancel' | null {
  if (key === 'Enter') return 'apply'
  if (key === 'Escape') return 'cancel'
  return null
}
