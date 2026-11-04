// Тост «Загрузить своё…» (сц. 5 спеки 048): общая часть для фонов, стикеров и шрифтов (082). Лежит в base/, чтобы
// editor/ не тянул panels/decor.
const WHY_MAX = 2 // сколько причин вписывать в тост

// «Добавлено: 3, пропущено: 1 (имя — причина)». Пусто — диалог закрыли ничего не выбрав.
export function importSummary(r: {
  added: readonly unknown[]
  skipped: readonly { name: string; why: string }[]
}): { text: string; error: boolean } | null {
  const added = r.added.length
  const skipped = r.skipped.length
  if (added === 0 && skipped === 0) return null
  const head = skipped > 0 ? `Добавлено: ${added}, пропущено: ${skipped}` : `Добавлено: ${added}`
  if (skipped === 0) return { text: head, error: false }
  const why = r.skipped.slice(0, WHY_MAX).map((s) => `${s.name} — ${s.why}`)
  const rest = skipped - why.length
  return { text: `${head} (${why.join('; ')}${rest > 0 ? ` и ещё ${rest}` : ''})`, error: added === 0 }
}
