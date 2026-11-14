// Автосборка в режиме «Весь альбом заново» (107, design §2.3): какие ID уходят в Go.
// Порядок нужен Go только затем, чтобы вставить свободные фото после их соседей по ленте.

// Фото альбома по ленте (без фильтра ленты и подборки: ordered — вся лента в порядке сортировки) и,
// если addFree, свободные из free (они идут в порядке той же ленты).
export function albumIds(
  ordered: readonly { id: string }[],
  used: { has(id: string): boolean },
  free: readonly string[],
  addFree: boolean,
): string[] {
  const add = addFree ? new Set(free) : null
  return ordered.filter((p) => used.has(p.id) || !!add?.has(p.id)).map((p) => p.id)
}
