// Открытые окна по порядку открытия (спека 033): справочник бывает поверх стартового окна, и Esc
// с фокусом должны доставаться только верхнему, иначе Esc закрыл бы оба, а окна перетягивали бы фокус.
// Окна различаются числовым id, а не объектом: $state-прокси отдаёт другой объект, и === всегда ложно.
let ids = $state.raw<number[]>([])
// Окна, под которыми работает отмена: Modal с undo, то есть те, что сами правят альбом (074-28).
let undoable = $state.raw<number[]>([])
let next = 0

// Слои: окна лежат на Z_MODAL … Z_MODAL + 9 (глубина обрезана в depth), тост — на Z_TOAST, выше любого окна.
// Тост при старте («Программа обновлена», «Вышла версия») показывается поверх стартового окна, а не под ним.
export const Z_MODAL = 80
export const MODAL_DEPTH_MAX = 9
export const Z_TOAST = Z_MODAL + MODAL_DEPTH_MAX + 1

export const modalStack = {
  // Поставить окно наверх; вернуть его id. undo — под окном работают «Отменить» и «Повторить».
  push(undo = false): number {
    const id = ++next
    ids = [...ids, id]
    if (undo) undoable = [...undoable, id]
    return id
  },
  remove(id: number) {
    ids = ids.filter((x) => x !== id)
    undoable = undoable.filter((x) => x !== id)
  },
  isTop: (id: number): boolean => ids.length > 0 && ids[ids.length - 1] === id,
  // Место окна в стеке (0 — нижнее, -1 — нет): z-index окна = 80 + depth, чтобы последнее открытое
  // было сверху при любом порядке в DOM (<Guide /> в App стоит раньше окна уровней). Выше 9 не растёт:
  // тосты и всплывающие меню сидят на 90.
  depth: (id: number): number => Math.min(ids.indexOf(id), MODAL_DEPTH_MAX),
  // Открыто хоть одно окно: режимы холста под ним (045) не должны забирать Esc.
  any: (): boolean => ids.length > 0,
  // Сколько окон открыто: «только окно хода» отличают по одному окну при jobs.openShown.
  count: (): number => ids.length,
  // Верхнее окно пускает отмену и повтор. Решает именно оно: вопрос поверх «Фонов» отмену не пускает.
  topUndo: (): boolean => ids.length > 0 && undoable.includes(ids[ids.length - 1]),
}
