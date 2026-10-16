import { beforeEach, describe, expect, it, vi } from 'vitest'

// «Копировать оформление» (049 Ф2): вызовы окна и состояние. Api.CopyDecor / PlanCopyDecor подменены моком wailsjs/go/app/App.
const api = vi.hoisted(() => ({
  CopyDecor: vi.fn(),
  PlanCopyDecor: vi.fn(),
  CopyGrid: vi.fn(),
  PlanCopyGrid: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { gridAct } = await import('./gridact.svelte')
const { planFor, runFor } = await import('./gridactions')
const { copyTargets, planQuestion, touched } = await import('../gallery/gridgallery')

const sp = (id: string, grid = false) => ({ id, photos: [], w: 600, h: 300, mid: 300, frames: [], texts: [], captions: [], grid })
const view = (rev: number, sheet = false, ids = ['s1', 's2', 's3']) => ({
  rev,
  folders: [],
  settings: { pageW: 300, pageH: 300, sheet },
  spreads: ids.map((id) => sp(id)),
})
const call = (over = {}) => ({
  decor: true,
  src: 's1',
  from: 'spread' as const,
  to: [
    { spread: 's2', scope: 'spread' as const },
    { spread: 's3', scope: 'spread' as const },
  ],
  design: true,
  texts: false,
  mirror: false,
  ...over,
})

beforeEach(() => {
  vi.clearAllMocks()
  store.toast = null
  store.apply(view(1) as never)
  gridAct.copyGrid = null
})

describe('planFor / runFor: выбор метода Go', () => {
  it('оформление — PlanCopyDecor с «С надписями» и зеркалом, раскладка не трогается', async () => {
    api.PlanCopyDecor.mockResolvedValue({ frozen: 0, left: 0, texts: 0, stickers: 2 })
    const c = call({ texts: true, mirror: true, from: 'left' })
    const plan = await planFor(c)
    expect(api.PlanCopyDecor).toHaveBeenCalledWith('s1', 'left', c.to, true, true)
    expect(api.PlanCopyGrid).not.toHaveBeenCalled()
    expect(planQuestion(plan)).toBe('2 стикера заменятся. Копировать?')
  })

  it('раскладка (decor выключен) — прежние PlanCopyGrid / CopyGrid с design', async () => {
    api.PlanCopyGrid.mockResolvedValue({ frozen: 0, left: 0, texts: 0 })
    await planFor(call({ decor: false, design: false }))
    expect(api.PlanCopyGrid).toHaveBeenCalledWith('s1', 'spread', expect.any(Array), false, false)
    expect(api.PlanCopyDecor).not.toHaveBeenCalled()
    api.CopyGrid.mockResolvedValue({ view: view(2), left: 0 })
    await runFor(call({ decor: false }), 2)
    expect(api.CopyGrid).toHaveBeenCalledTimes(1)
    expect(api.CopyDecor).not.toHaveBeenCalled()
  })

  it('чистое оформление: плана без последствий нет — вопроса нет', async () => {
    api.PlanCopyDecor.mockResolvedValue({ frozen: 0, left: 0, texts: 0, stickers: 0 })
    expect(planQuestion(await planFor(call()))).toBeNull()
  })
})

describe('runFor: копирование оформления', () => {
  it('успех: вид применён, тост с числом разворотов', async () => {
    api.CopyDecor.mockResolvedValue({ view: view(2), left: 0 })
    expect(await runFor(call(), 2)).toBe(true)
    expect(api.CopyDecor).toHaveBeenCalledWith('s1', 'spread', expect.any(Array), false, false)
    expect(store.view.rev).toBe(2)
    expect(store.toast).toMatchObject({ text: 'Оформление скопировано на 2 разворота', kind: 'info' })
  })

  it('на один разворот и на лист виньетки — склонение по режиму альбома', async () => {
    api.CopyDecor.mockResolvedValue({ view: view(2), left: 0 })
    await runFor(call(), 1)
    expect(store.toast?.text).toBe('Оформление скопировано на 1 разворот')
    api.CopyDecor.mockResolvedValue({ view: view(3, true), left: 0 })
    await runFor(call(), 5)
    expect(store.toast?.text).toBe('Оформление скопировано на 5 листов')
  })

  it('раскладка: тост со склонением по режиму — листы (031) и развороты', async () => {
    api.CopyGrid.mockResolvedValue({ view: view(2), left: 0 })
    await runFor(call({ decor: false }), 5)
    expect(store.toast?.text).toBe('Раскладка скопирована на 5 разворотов')
    api.CopyGrid.mockResolvedValue({ view: view(3, true), left: 0 })
    await runFor(call({ decor: false }), 5)
    expect(store.toast?.text).toBe('Раскладка скопирована на 5 листов')
  })

  it('холостая правка (rev не вырос): без «скопировано», шага отмены нет', async () => {
    api.CopyDecor.mockResolvedValue({ view: view(1), left: 0 })
    expect(await runFor(call(), 2)).toBe(true)
    expect(store.toast?.text).toBe('У выбранных уже такое же оформление: копировать нечего')
    expect(store.toast?.action).toBeUndefined()
    expect(store.view.rev).toBe(1)
  })

  it('отказ Go (предел стикеров): тост-ошибка, вид прежний, окно может остаться — false', async () => {
    api.CopyDecor.mockRejectedValue('на развороте 3 не больше 40 стикеров')
    expect(await runFor(call(), 2)).toBe(false)
    expect(store.toast).toMatchObject({ kind: 'error', text: 'На развороте 3 не больше 40 стикеров' })
    expect(store.view.rev).toBe(1)
  })

  it('отказ плана — исключение уходит к вызывающему (окно покажет тост и не закроется)', async () => {
    api.PlanCopyDecor.mockRejectedValue('разворот не найден')
    await expect(planFor(call())).rejects.toBe('разворот не найден')
  })
})

describe('цели окна: номера, «на все», страница, пусто', () => {
  const ids = ['s1', 's2', 's3']
  const base = { src: 's1', from: 'spread' as const, side: 'spread' as const, spreadIds: ['s2'], allIds: ids }

  it('«на все» — все развороты, исходный в счёт не идёт', () => {
    const t = copyTargets({ ...base, where: 'all' })
    expect(t).toHaveLength(3)
    expect(touched(t, 's1', 'spread')).toBe(2)
  })
  it('номера: только указанные; пустой список — целей нет (кнопка серая)', () => {
    expect(copyTargets({ ...base, where: 'spreads' })).toEqual([{ spread: 's2', scope: 'spread' }])
    const none = copyTargets({ ...base, where: 'spreads', spreadIds: [] })
    expect(none).toEqual([])
    expect(touched(none, 's1', 'spread')).toBe(0)
  })
  it('единственный разворот: «на все» затрагивает 0, «соседняя страница» — 1', () => {
    const one = { ...base, allIds: ['s1'] }
    expect(touched(copyTargets({ ...one, where: 'all' }), 's1', 'spread')).toBe(0)
    const page = copyTargets({ ...one, from: 'left', where: 'next' })
    expect(page).toEqual([{ spread: 's1', scope: 'right' }])
    expect(touched(page, 's1', 'left')).toBe(1)
  })
  it('со страницы на выбранные стороны; «новые развороты» целей не даёт', () => {
    expect(copyTargets({ ...base, from: 'right', side: 'left', where: 'all' }).every((t) => t.scope === 'left')).toBe(true)
    expect(copyTargets({ ...base, where: 'new' })).toEqual([])
  })
})

describe('gridAct: открытие и жизнь окна «Копировать оформление»', () => {
  it('openDecor ставит режим decor с листа целиком', () => {
    gridAct.openDecor('s2')
    expect(gridAct.copyGrid).toEqual({ spread: 's2', from: 'spread', mode: 'decor' })
  })
  it('обложка и пустой лист — не источник: окно не открывается', () => {
    gridAct.openDecor('cover')
    expect(gridAct.copyGrid).toBeNull()
    gridAct.openDecor('')
    expect(gridAct.copyGrid).toBeNull()
  })
  it('повторный вызов обновляет источник, а не плодит окна', () => {
    gridAct.openDecor('s1')
    gridAct.openDecor('s3')
    expect(gridAct.copyGrid?.spread).toBe('s3')
  })
  it('«Авто»-источник оформлению подходит: окно живёт, пока лист есть', () => {
    gridAct.openDecor('s1')
    gridAct.prune([sp('s1', false), sp('s2')] as never)
    expect(gridAct.copyGrid).not.toBeNull()
  })
  it('лист-источник удалён — окно закрывается', () => {
    gridAct.openDecor('s1')
    gridAct.prune([sp('s2')] as never)
    expect(gridAct.copyGrid).toBeNull()
  })
  it('«Копировать раскладку» по-прежнему закрывается, когда источник перестал быть сеткой', () => {
    gridAct.copyGrid = { spread: 's1', from: 'spread' }
    gridAct.prune([sp('s1', true)] as never)
    expect(gridAct.copyGrid).not.toBeNull()
    gridAct.prune([sp('s1', false)] as never)
    expect(gridAct.copyGrid).toBeNull()
  })
})
