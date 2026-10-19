import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Фон на набор листов (077): состояние окна «Фоны» — выбор листов, вопрос, постановка, тосты. Go подменён.
// Отдельный файл от decordialog.test.ts: свои заготовки Go и вид из нескольких разворотов.
const api = vi.hoisted(() => ({
  DecorCatalog: vi.fn(),
  ApplyBackground: vi.fn(),
  PlanBackgroundTo: vi.fn(),
  ApplyBackgroundTo: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { decor } = await import('./decor.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { spreadsel } = await import('../timeline/spreadsel.svelte')
const { bgmode } = await import('../../editor/backdrop/bgmode.svelte')

const OWN = 'my:0123456789abcdef'
const catalog = () => ({
  kind: 'bg',
  themes: [{ id: 'all', name: 'Все', count: 2 }],
  items: [
    { id: 'sa:dots', theme: 'basic', name: 'Горох', url: '/media/gen/pat?k=dots&v=1', own: false, aspect: 0, w: 0, h: 0, color: '#fff' },
    { id: OWN, theme: 'mine', name: 'Моё', url: '/media/lib/t/0123456789abcdef?v=1', own: true, aspect: 1.5, w: 3000, h: 2000 },
  ],
  recent: [],
  readOnly: false,
})

const sheet = (id: string, n = 0) => ({
  id,
  photos: [],
  w: 600,
  h: 300,
  mid: 300,
  frames: [],
  texts: [],
  captions: [],
  bg: ['#fff', '#fff'],
  bgSet: ['', ''],
  backdrops: null,
  n,
})
const ids = (n: number) => Array.from({ length: n }, (_, i) => `s${i + 1}`)
const view = (n = 5, rev = 1, o: Record<string, unknown> = {}) => ({
  rev,
  folders: [],
  settings: { pageW: 300, pageH: 300 },
  spreads: ids(n).map((id) => sheet(id)),
  ...o,
})
const tg = (spreads: string[], scope = 'spread') => spreads.map((spread) => ({ spread, scope }))

const open = async (spread = 's2', scope: 'spread' | 'left' | 'right' = 'spread') => {
  decor.show('bg', spread, scope)
  await vi.waitFor(() => expect(decor.cat).not.toBeNull())
}
const tile = (i = 0) => decor.cat!.items[i]

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.DecorCatalog.mockImplementation(async () => catalog())
  api.PlanBackgroundTo.mockResolvedValue({ spreads: 2, photos: 0 })
  api.ApplyBackgroundTo.mockImplementation(async () => ({ view: view(5, 2), spreads: 2 }))
  api.ApplyBackground.mockResolvedValue(view(5, 2))
  decor.close()
  spreadsel.clear()
  prefs.level = 'standard' // «Выделение группы» (groupSelect) — со Стандартного
  store.toast = null
  store.apply(view() as never)
})
afterEach(() => {
  spreadsel.clear()
  prefs.level = 'simple'
})

describe('выбор при открытии', () => {
  it('без выделения в ленте — «на этот разворот», поле пустое', async () => {
    await open()
    expect(decor.bgPick).toBe('this')
    expect(decor.bgText).toBe('')
  })

  it('выделено 2+ — «на выделенные»; одно выделенное — «на этот»', async () => {
    spreadsel.toggle('s3')
    await open()
    expect(decor.bgPick).toBe('this')
    spreadsel.toggle('s4')
    await open()
    expect(decor.bgPick).toBe('sel')
  })

  it('без «Выделения группы» на уровне выделенные не считаются', async () => {
    prefs.level = 'simple'
    spreadsel.toggle('s3')
    spreadsel.toggle('s4')
    await open()
    expect(decor.bgPick).toBe('this')
    expect(decor.selIds()).toEqual([])
  })

  it('каждое открытие — заново: прежний выбор и номера сброшены', async () => {
    spreadsel.toggle('s3')
    spreadsel.toggle('s4')
    await open()
    decor.bgPick = 'nums'
    decor.bgText = '1-2'
    decor.numsFocus = 2
    decor.close()
    spreadsel.clear()
    await open()
    expect(decor.bgPick).toBe('this')
    expect(decor.bgText).toBe('')
    expect(decor.numsFocus).toBe(0)
  })

  it('обложка, один разворот и другие окна — строки нет, выбор «этот»', async () => {
    spreadsel.toggle('s3')
    spreadsel.toggle('s4')
    store.view = { ...store.view, cover: { sheet: sheet('cover') } } as never
    decor.show('bg', 'cover', 'right')
    expect(decor.bgPick).toBe('this')
    store.apply(view(1) as never)
    decor.show('bg', 's1', 'spread')
    expect(decor.bgPick).toBe('this')
    store.apply(view(5) as never)
    decor.show('sticker', 's2', 'spread')
    expect(decor.bgPick).toBe('this')
    decor.show('spread', 's2', 'spread')
    expect(decor.bgPick).toBe('this')
  })
})

describe('цели', () => {
  it('«выделенные» — в порядке альбома; «все» — без обложки; «Куда» — одна на все', async () => {
    spreadsel.toggle('s4')
    spreadsel.toggle('s3')
    await open('s2', 'left')
    expect(decor.bgPlan()).toEqual({ pick: 'sel', targets: tg(['s3', 's4'], 'left'), error: null })
    decor.bgPick = 'all'
    expect(decor.bgPlan().targets).toEqual(tg(ids(5), 'left'))
  })

  it('номера: «1-3, 5»; ошибка — только при выборе номеров', async () => {
    await open()
    decor.bgPick = 'nums'
    decor.bgText = '1-3, 5'
    expect(decor.bgPlan().targets).toEqual(tg(['s1', 's2', 's3', 's5']))
    decor.bgText = '99'
    expect(decor.bgPlan().error).toBe('Разворота 99 нет (всего 5)')
    decor.bgPick = 'all'
    expect(decor.bgPlan().error).toBeNull()
  })
})

describe('щелчок по плитке', () => {
  it('«на этот разворот» — прежний ApplyBackground, окно закрыто, набора нет', async () => {
    await open('s2', 'left')
    await decor.pick(tile())
    expect(api.ApplyBackground).toHaveBeenCalledWith('s2', 'left', 'sa:dots')
    expect(api.PlanBackgroundTo).not.toHaveBeenCalled()
    expect(api.ApplyBackgroundTo).not.toHaveBeenCalled()
    expect(decor.cur).toBeNull()
  })

  it('номер только открытого листа — тоже прежний путь', async () => {
    await open()
    decor.bgPick = 'nums'
    decor.bgText = '2'
    await decor.pick(tile())
    expect(api.ApplyBackground).toHaveBeenCalledWith('s2', 'spread', 'sa:dots')
    expect(api.ApplyBackgroundTo).not.toHaveBeenCalled()
  })

  it('«все»: PlanBackgroundTo, затем ApplyBackgroundTo с теми же целями; окно закрыто, тост, режим фона не открыт', async () => {
    api.PlanBackgroundTo.mockResolvedValue({ spreads: 5, photos: 0 })
    api.ApplyBackgroundTo.mockResolvedValue({ view: view(5, 2), spreads: 5 })
    await open()
    decor.bgPick = 'all'
    await decor.pick(tile(1)) // своя картинка: на наборе режим фона не открывается
    expect(api.PlanBackgroundTo).toHaveBeenCalledWith(tg(ids(5)), OWN)
    expect(api.ApplyBackgroundTo).toHaveBeenCalledWith(tg(ids(5)), OWN)
    expect(api.ApplyBackground).not.toHaveBeenCalled()
    expect(store.view.rev).toBe(2)
    expect(decor.cur).toBeNull()
    expect(store.toast?.text).toBe('Фон поставлен на 5 разворотов')
    expect(bgmode.cur).toBeNull()
  })

  it('«выделенные» с левой страницей', async () => {
    spreadsel.toggle('s3')
    spreadsel.toggle('s4')
    await open('s2', 'left')
    await decor.pick(tile())
    expect(api.ApplyBackgroundTo).toHaveBeenCalledWith(tg(['s3', 's4'], 'left'), 'sa:dots')
    expect(store.toast?.text).toBe('Фон поставлен на 2 разворота')
  })

  it('на всех уже этот фон (spreads = 0) — тост «У выбранных уже этот фон»', async () => {
    api.ApplyBackgroundTo.mockResolvedValue({ view: view(5, 1), spreads: 0 })
    await open()
    decor.bgPick = 'all'
    await decor.pick(tile())
    expect(store.toast?.text).toBe('У выбранных уже этот фон')
    expect(store.view.rev).toBe(1)
    expect(decor.cur).toBeNull()
  })

  it('неверные номера: вызовов Go нет, окно открыто, фокус в поле', async () => {
    await open()
    decor.bgPick = 'nums'
    await decor.pick(tile()) // пусто
    expect(decor.numsFocus).toBe(1)
    decor.bgText = '99'
    await decor.pick(tile())
    expect(decor.numsFocus).toBe(2)
    for (const f of [api.PlanBackgroundTo, api.ApplyBackgroundTo, api.ApplyBackground]) expect(f).not.toHaveBeenCalled()
    expect(decor.cur).not.toBeNull()
  })

  it('второй щелчок, пока идёт прогон, не нужен', async () => {
    let done!: (v: unknown) => void
    api.PlanBackgroundTo.mockImplementationOnce(() => new Promise((r) => (done = r)))
    await open()
    decor.bgPick = 'all'
    const first = decor.pick(tile())
    await decor.pick(tile())
    done({ spreads: 5, photos: 0 })
    await first
    expect(api.PlanBackgroundTo).toHaveBeenCalledTimes(1)
    expect(api.ApplyBackgroundTo).toHaveBeenCalledTimes(1)
  })
})

describe('вопрос «Поставить фон?»', () => {
  beforeEach(() => api.PlanBackgroundTo.mockResolvedValue({ spreads: 4, photos: 1 }))

  it('уходят фото фона — вопрос с числами, ничего не ставится, окно открыто', async () => {
    await open()
    decor.bgPick = 'nums'
    decor.bgText = '1-3, 5'
    await decor.pick(tile(1))
    expect(decor.ask).toMatchObject({
      kind: 'bgMany',
      id: OWN,
      text: '1 фото фона заменится. Поставить фон на 4 разворота?',
      targets: tg(['s1', 's2', 's3', 's5']),
    })
    expect(api.ApplyBackgroundTo).not.toHaveBeenCalled()
    expect(decor.cur).not.toBeNull()
  })

  it('«Отмена» — назад к плиткам, ничего не изменилось; повтор и «Поставить» — постановка и закрытие', async () => {
    await open()
    decor.bgPick = 'all'
    await decor.pick(tile(1))
    decor.dismiss()
    expect(decor.ask).toBeNull()
    expect(decor.cur).not.toBeNull()
    expect(api.ApplyBackgroundTo).not.toHaveBeenCalled()
    expect(store.view.rev).toBe(1)
    await decor.pick(tile(1))
    await decor.confirmBgMany()
    expect(api.ApplyBackgroundTo).toHaveBeenCalledTimes(1)
    expect(decor.ask).toBeNull()
    expect(decor.cur).toBeNull()
    expect(store.view.rev).toBe(2)
  })

  it('«Поставить» без вопроса и commit() вопрос не трогают', async () => {
    await open()
    await decor.confirmBgMany()
    expect(api.ApplyBackgroundTo).not.toHaveBeenCalled()
    decor.bgPick = 'all'
    await decor.pick(tile())
    expect(await decor.commit()).toBe(false)
    expect(decor.ask?.kind).toBe('bgMany')
  })
})

describe('ошибки Go', () => {
  it('прогон упал — тост, окно открыто, вопроса нет', async () => {
    api.PlanBackgroundTo.mockRejectedValue('область не подходит')
    await open()
    decor.bgPick = 'all'
    await decor.pick(tile())
    expect(store.toast).toMatchObject({ kind: 'error', text: 'Область не подходит' })
    expect(decor.cur).not.toBeNull()
    expect(decor.busy).toBe(false)
  })

  it('постановка упала — проект не тронут, окно открыто', async () => {
    api.ApplyBackgroundTo.mockRejectedValue('нет такого фона')
    await open()
    decor.bgPick = 'all'
    await decor.pick(tile())
    expect(store.toast).toMatchObject({ kind: 'error', text: 'Нет такого фона' })
    expect(decor.cur).not.toBeNull()
    expect(store.view.rev).toBe(1)
    expect(decor.busy).toBe(false)
  })

  it('окно закрыли, пока считался вопрос, — ничего не ставится', async () => {
    let done!: (v: unknown) => void
    api.PlanBackgroundTo.mockImplementationOnce(() => new Promise((r) => (done = r)))
    await open()
    decor.bgPick = 'all'
    const p = decor.pick(tile())
    decor.close()
    done({ spreads: 5, photos: 0 })
    await p
    expect(api.ApplyBackgroundTo).not.toHaveBeenCalled()
  })
})
