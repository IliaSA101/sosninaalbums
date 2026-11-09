import { tick } from 'svelte'

// Поле поиска справочника: прошлый запрос выделяется целиком, чтобы печать заменяла его, а не дописывалась.
// Выделяем сразу и ещё раз после tick: Modal ставит фокус в своём $effect уже после монтирования поля,
// а значение bind:value приходит не раньше первого прохода эффектов.
export function selectQuery(el: Pick<HTMLInputElement, 'select'>): void {
  el.select()
  void tick().then(() => el.select())
}
