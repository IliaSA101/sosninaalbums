// Открытые окна по порядку открытия (спека 033): справочник бывает поверх стартового окна, и Esc
// с фокусом должны доставаться только верхнему, иначе Esc закрыл бы оба, а окна перетягивали бы фокус.
// Окна различаются числовым id, а не объектом: $state-прокси отдаёт другой объект, и === всегда ложно.
let ids = $state.raw<number[]>([])
let next = 0

export const modalStack = {
  // Поставить окно наверх; вернуть его id.
  push(): number {
    const id = ++next
    ids = [...ids, id]
    return id
  },
  remove(id: number) {
    ids = ids.filter((x) => x !== id)
  },
  isTop: (id: number): boolean => ids.length > 0 && ids[ids.length - 1] === id,
}
