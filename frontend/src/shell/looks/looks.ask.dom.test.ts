// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// U2: вопрос «Применить» (067), тосты «Вернуть», «Поставить на разворот N / обложку» и «Разворот макета» — на подставном lookApi.
const api = vi.hoisted(() => ({
  catalog: vi.fn(),
  info: vi.fn(),
  count: vi.fn(),
  plan: vi.fn(),
  apply: vi.fn(),
  planStep: vi.fn(),
  applyStep: vi.fn(),
  newSpread: vi.fn(),
  rename: vi.fn(),
  hide: vi.fn(),
}))
vi.mock('./lookapi', () => ({ lookApi: api }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])) }
})

const { default: LooksDialog } = await import('./LooksDialog.svelte')
const { looks } = await import('./looks.svelte')
const { store } = await import('../../base/core/store.svelte')
const Api = await import('../../../wailsjs/go/app/App')

const A = 'my:0a1b2c3d4e5f6071'
const sp = (id: string) => ({
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
})
const view = (rev: number, extra: Record<string, unknown> = {}) => ({
  rev,
  folders: [],
  settings: { pageW: 300, pageH: 300, sheet: false },
  spreads: [sp('s1'), sp('s2'), sp('s3')],
  ...extra,
})
const plan = (o: Record<string, unknown> = {}) => ({
  mode: 'series',
  trial: false,
  touched: 0,
  full: false,
  spreads: 9,
  places: 46,
  drafts: 0,
  replaced: 0,
  anchors: 0,
  cover: false,
  coverTouched: false,
  coverReplaced: false,
  left: 0,
  prev: '',
  missing: 0,
  ...o,
})
const gp = (o: Record<string, number> = {}) => ({ frozen: 0, left: 0, texts: 0, stickers: 0, ...o })
const info = (cover: boolean) => ({
  steps: Array.from({ length: 4 }, (_, k) => ({ name: `Шаг ${k + 1}`, role: '', places: 3, pages: 2 })),
  cover,
  min: 1,
  max: 20,
})

const text = () => document.body.textContent ?? ''
// Последняя из одноимённых: вопрос лежит в Modal поверх окна, его «Отмена» и «Применить» — позже форм окна
const btn = (label: string) =>
  Array.from(document.querySelectorAll('button'))
    .filter((b) => b.textContent?.trim() === label)
    .at(-1) as HTMLButtonElement
const pick = async () => {
  await fireEvent.click(document.querySelector('button.tile') as HTMLElement)
  await vi.waitFor(() => expect(btn('Применить').disabled).toBe(false))
}
// Ответ Go «правка прошла»: вид с новым rev приходит в store.run.
const edited = (rev: number) => async () => ({ view: view(rev), plan: plan() })

async function opened(cover = false) {
  api.info.mockImplementation(async () => info(cover))
  api.catalog.mockResolvedValue({
    kind: 'album',
    themes: [{ id: 'all', name: 'Все', count: 1 }],
    items: [{ id: A, name: 'Светлая', theme: 'w', url: '', own: false, aspect: 0, w: 0, h: 0, steps: 4, from: 10, cover }],
    recent: [],
    readOnly: false,
  })
  render(LooksDialog)
  await looks.show()
  await tick()
  await pick()
}

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.info.mockImplementation(async () => info(false))
  api.count.mockImplementation(async (_id: string, n: number) => ({ n, places: n * 5, fit: 0 }))
  api.plan.mockImplementation(async (_id: string, o: { mode: string; spreads: number }) => plan({ mode: o.mode, spreads: o.spreads }))
  api.apply.mockImplementation(edited(2))
  api.planStep.mockResolvedValue(gp())
  api.applyStep.mockImplementation(async () => ({ view: view(2), left: 0, missing: 0 }))
  looks.close()
  Object.assign(looks, { cat: null, theme: '', sel: '', spreads: 0, pick: 'bg', step: 1, info: null, cnt: null, plan: null, busy: false })
  store.toast = null
  store.onCover = false
  store.cover = null
  store.apply(view(1) as never)
})
afterEach(() => {
  cleanup()
  looks.close()
})

describe('«Применить»: вопрос замены своего (сц. 6)', () => {
  it('свой фон заменится: вопрос со словами спеки, альбом не тронут; «Отмена» возвращает в окно', async () => {
    api.plan.mockImplementation(async (_i: string, o: { mode: string; spreads: number }) =>
      plan({ mode: o.mode, spreads: o.spreads, replaced: 3, anchors: 3 }),
    )
    await opened()
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(text()).toContain('Применить макет?'))
    expect(text()).toContain(
      'У 3 разворотов заменится свой фон. 3 разворота станут оформленными — «Весь альбом заново» их не пересоберёт. Применить?',
    )
    expect(api.apply).not.toHaveBeenCalled()
    await fireEvent.click(btn('Отмена'))
    await tick()
    expect(looks.ask).toBeNull()
    expect(looks.open).toBe(true)
    expect(api.apply).not.toHaveBeenCalled()
  })

  it('макет с обложкой: вопрос называет развороты и обложку (0.28-131-174)', async () => {
    api.plan.mockImplementation(async (_i: string, o: { mode: string; spreads: number }) =>
      plan({ mode: o.mode, spreads: o.spreads, touched: 3, cover: true, coverTouched: true }),
    )
    await opened(true)
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(text()).toContain('Применить макет?'))
    expect(text()).toContain('Ваши правки на 2 разворотах и обложке заменятся. Применить?')
    expect(api.apply).not.toHaveBeenCalled()
  })

  it('«Применить» в вопросе ставит именно тот план, о котором спросили: один ApplyLook, окно закрыто, тост «Вернуть»', async () => {
    api.plan.mockImplementation(async (_i: string, o: { mode: string; spreads: number }) =>
      plan({ mode: o.mode, spreads: o.spreads, touched: 2 }),
    )
    await opened()
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(text()).toContain('Ваши правки на 2 разворотах заменятся. Применить?'))
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(looks.open).toBe(false))
    expect(api.apply).toHaveBeenCalledTimes(1)
    expect(api.apply).toHaveBeenCalledWith(A, { mode: 'series', spreads: 4 })
    expect(store.toast?.text).toBe('Макет «Светлая»: 9 разворотов, 46 мест')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('фото уйдут в ленту — тот же вопрос, что у 067', async () => {
    api.plan.mockImplementation(async (_i: string, o: { mode: string; spreads: number }) =>
      plan({ mode: o.mode, spreads: o.spreads, left: 3 }),
    )
    await opened()
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(text()).toContain('3 фото вернутся в ленту. Применить?'))
  })

  it('Esc в окне вопроса то же, что «Отмена»: вопрос закрыт, окно макетов и альбом не тронуты', async () => {
    api.plan.mockImplementation(async (_i: string, o: { mode: string; spreads: number }) =>
      plan({ mode: o.mode, spreads: o.spreads, replaced: 1 }),
    )
    await opened()
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(looks.ask?.kind).toBe('apply'))
    await fireEvent.keyDown(window, { key: 'Escape' }) // Modal ловит Esc на window; закрывается верхнее окно — вопрос
    await tick()
    expect(text()).not.toContain('Применить макет?')
    expect(looks.open).toBe(true)
    expect(api.apply).not.toHaveBeenCalled()
  })

  it('ставится спрошенное, а не то, что в окне сейчас: число и режим сменили, пока вопрос открыт', async () => {
    // Go отвечает «разворотов макета после постановки» = 99, не тем, что просили
    api.plan.mockImplementation(async (_i: string, o: { mode: string; spreads: number }) =>
      plan({ mode: o.mode, spreads: 99, replaced: 2 }),
    )
    await opened()
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(looks.ask?.kind).toBe('apply'))
    looks.spreads = 2 // окно под вопросом успели изменить: другое число и режим
    looks.pick = 'add'
    looks.addN = 7
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(looks.open).toBe(false))
    expect(api.apply).toHaveBeenCalledTimes(1)
    expect(api.apply).toHaveBeenCalledWith(A, { mode: 'series', spreads: 4 })
  })

  it('«Добавить»: вопрос и тост берут число из запроса (сколько добавить), а не plan.spreads Go', async () => {
    api.plan.mockImplementation(async (_i: string, o: { mode: string; spreads: number }) =>
      plan({ mode: o.mode, spreads: 11, places: 8, full: true, replaced: 1 }),
    )
    api.apply.mockImplementation(async () => ({ view: view(2), plan: plan({ mode: 'add', spreads: 11, places: 8 }) }))
    await opened()
    await fireEvent.click(btn('Добавить развороты макета'))
    await vi.waitFor(() => expect(looks.mode).toBe('add'))
    await looks.setSpreads(2)
    await vi.waitFor(() => expect(looks.ready).toBe(true))
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(looks.ask?.kind).toBe('apply'))
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(looks.open).toBe(false))
    expect(api.apply).toHaveBeenCalledWith(A, { mode: 'add', spreads: 2 })
    expect(store.toast?.text).toBe('Макет «Светлая»: добавлено 2 разворота, 8 мест')
  })
})

describe('«Применить»: тосты', () => {
  it('без вопроса: ставит сразу, тост с числами из ответа Go и «Вернуть»', async () => {
    api.apply.mockImplementation(async () => ({ view: view(2), plan: plan({ spreads: 4, places: 18 }) }))
    await opened()
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(looks.open).toBe(false))
    expect(store.toast?.text).toBe('Макет «Светлая»: 4 разворота, 18 мест')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('повтор того же: версия не выросла — тост без «Вернуть»', async () => {
    api.apply.mockImplementation(async () => ({ view: view(1), plan: plan() }))
    await opened()
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(looks.open).toBe(false))
    expect(store.toast?.text).toBe('Макет «Светлая» уже стоит')
    expect(store.toast?.action).toBeUndefined()
  })

  it('«Вернуть» отменяет одним шагом', async () => {
    await opened()
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(store.toast?.action).toBeTruthy())
    store.toast?.action?.run()
    await vi.waitFor(() => expect(Api.Undo).toHaveBeenCalledTimes(1))
  })

  it('ошибка Go: окно остаётся, красный тост, «Вернуть» нет', async () => {
    api.apply.mockRejectedValue(new Error('не получилось'))
    await opened()
    await fireEvent.click(btn('Применить'))
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
    expect(looks.open).toBe(true)
    expect(store.toast?.action).toBeUndefined()
  })
})

describe('«Поставить на разворот N» / «на обложку» (сц. 7)', () => {
  it('подпись по текущему листу: номер в ленте, у обложки — «на обложку»', async () => {
    store.current = 2
    await opened()
    expect(btn('Поставить на разворот 3')).toBeTruthy()
    cleanup()
    looks.close()
    store.onCover = true
    store.cover = { sheet: sp('cover') } as never
    await opened(true)
    expect(btn('Поставить на обложку')).toBeTruthy()
    store.cover = null
  })

  it('шаг на лист: PlanLookStep → ApplyLookStep с листом, макетом и шагом; окно закрыто, тост «Вернуть»', async () => {
    store.current = 1
    await opened()
    await fireEvent.click(document.querySelectorAll('.chip')[2] as HTMLElement) // шаг 3
    await fireEvent.click(btn('Поставить на разворот 2'))
    await vi.waitFor(() => expect(looks.open).toBe(false))
    expect(api.planStep).toHaveBeenCalledWith('s2', A, 3)
    expect(api.applyStep).toHaveBeenCalledWith('s2', A, 3)
    expect(store.toast?.text).toBe('Разворот поставлен')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('фото уходят в ленту — вопрос 067; «Отмена» не ставит, «Поставить» ставит', async () => {
    api.planStep.mockResolvedValue(gp({ left: 2, stickers: 1 }))
    api.applyStep.mockImplementation(async () => ({ view: view(2), left: 2, missing: 0 }))
    await opened()
    await fireEvent.click(btn('Поставить на разворот 1'))
    await vi.waitFor(() => expect(text()).toContain('2 фото вернутся в ленту, 1 стикер заменится. Поставить?'))
    expect(api.applyStep).not.toHaveBeenCalled()
    await fireEvent.click(btn('Отмена'))
    await tick()
    expect(looks.open).toBe(true)
    expect(api.applyStep).not.toHaveBeenCalled()
    await fireEvent.click(btn('Поставить на разворот 1'))
    await vi.waitFor(() => expect(looks.ask?.kind).toBe('step'))
    await fireEvent.click(btn('Поставить'))
    await vi.waitFor(() => expect(looks.open).toBe(false))
    expect(api.applyStep).toHaveBeenCalledTimes(1)
    expect(store.toast?.text).toBe('Разворот поставлен · 2 фото вернулись в ленту')
  })

  it('повтор того же шага: версия не выросла — «Здесь уже такой разворот» без «Вернуть»', async () => {
    api.applyStep.mockImplementation(async () => ({ view: view(1), left: 0, missing: 0 }))
    await opened()
    await fireEvent.click(btn('Поставить на разворот 1'))
    await vi.waitFor(() => expect(looks.open).toBe(false))
    expect(store.toast?.text).toBe('Здесь уже такой разворот')
    expect(store.toast?.action).toBeUndefined()
  })

  it('обложку ставят только на обложку, разворот — только на разворот: иначе кнопка серая с причиной', async () => {
    store.cover = { sheet: sp('cover') } as never
    await opened(true)
    expect(looks.step).toBe(0) // плитка с обложкой показывает её
    const b = btn('Поставить на разворот 1')
    expect(b.disabled).toBe(true)
    expect(b.title).toBe('Оформление обложки ставится только на обложку: откройте её')
    await fireEvent.click(b)
    expect(api.planStep).not.toHaveBeenCalled()
    await fireEvent.click(document.querySelectorAll('.chip')[1] as HTMLElement) // шаг 1
    expect(btn('Поставить на разворот 1').disabled).toBe(false)
  })

  it('в альбоме нет обложки, а показан шаг «Обложка»: причина — «Создать обложку»', async () => {
    await opened(true)
    expect(looks.step).toBe(0)
    const b = btn('Поставить на разворот 1')
    expect(b.disabled).toBe(true)
    expect(b.title).toBe('В альбоме нет обложки: её оформление встанет при «Создать обложку»')
  })

  it('на обложке выбран шаг разворота у макета без обложки: «У этого макета нет оформления обложки»', async () => {
    store.onCover = true
    store.cover = { sheet: sp('cover') } as never
    await opened(false) // у макета нет обложки: показан шаг 1
    expect(looks.step).toBe(1)
    const b = btn('Поставить на обложку')
    expect(b.disabled).toBe(true)
    expect(b.title).toBe('У этого макета нет оформления обложки')
  })

  it('проект-лист 031: кнопки «Поставить на…» нет', async () => {
    store.apply(view(1, { settings: { pageW: 300, pageH: 300, sheet: true } }) as never)
    await opened()
    expect(Array.from(document.querySelectorAll('button')).some((b) => b.textContent?.includes('Поставить на'))).toBe(false)
  })

  it('на обложке выбран шаг разворота — серая с другой причиной; шаг «Обложка» ставится на обложку', async () => {
    store.onCover = true
    store.cover = { sheet: sp('cover') } as never
    await opened(true)
    expect(looks.step).toBe(0)
    expect(btn('Поставить на обложку').disabled).toBe(false)
    await fireEvent.click(btn('Поставить на обложку'))
    await vi.waitFor(() => expect(looks.open).toBe(false))
    expect(api.planStep).toHaveBeenCalledWith('cover', A, 0)
    expect(store.toast?.text).toBe('Обложка поставлена')
    looks.close()
    cleanup()
    store.onCover = true // правка вернула вид без обложки
    store.cover = { sheet: sp('cover') } as never
    await opened(true)
    await fireEvent.click(document.querySelectorAll('.chip')[1] as HTMLElement)
    expect(btn('Поставить на обложку').disabled).toBe(true)
    expect(btn('Поставить на обложку').title).toBe('На обложку встаёт шаг «Обложка»: выберите его в ленте шагов')
    store.cover = null
  })

  it('нет текущего разворота — кнопки нет', async () => {
    store.apply(view(1, { spreads: [] }) as never)
    await opened()
    expect(Array.from(document.querySelectorAll('button')).some((b) => b.textContent?.includes('Поставить на'))).toBe(false)
  })

  it('ошибка Go при постановке: красный тост, окно остаётся', async () => {
    api.applyStep.mockRejectedValue(new Error('нет такого разворота'))
    await opened()
    await fireEvent.click(btn('Поставить на разворот 1'))
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
    expect(looks.open).toBe(true)
  })
})

describe('«Разворот макета»', () => {
  const grown = (rev: number) =>
    view(rev, { spreads: [sp('s1'), sp('s2'), sp('s3'), sp('new')], look: { key: A, name: 'Светлая', found: true } })

  it('NewLookSpread одной правкой: переход на новый разворот и тост «Вернуть»', async () => {
    api.newSpread.mockImplementation(async () => grown(2))
    await looks.newSpread()
    expect(store.spread?.id).toBe('new')
    expect(store.toast?.text).toBe('Разворот макета «Светлая» добавлен')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })
  it('холостой вызов (версия не выросла): без перехода и без тоста', async () => {
    store.go(1)
    api.newSpread.mockImplementation(async () => view(1))
    await looks.newSpread()
    expect(store.spread?.id).toBe('s2')
    expect(store.toast).toBeNull()
  })
  it('повторный вызов, пока идёт первый, молчит: Go позван один раз', async () => {
    let done!: () => void
    api.newSpread.mockImplementation(() => new Promise((r) => (done = () => r(grown(2)))))
    const first = looks.newSpread()
    await looks.newSpread()
    expect(api.newSpread).toHaveBeenCalledTimes(1)
    done()
    await first
    api.newSpread.mockImplementation(async () => grown(3))
    await looks.newSpread() // после завершения флаг снят
    expect(api.newSpread).toHaveBeenCalledTimes(2)
  })
  it('ошибка Go («Макета нет в библиотеке») — красный тост, флаг снят', async () => {
    api.newSpread.mockRejectedValueOnce(new Error('макета «С» нет в библиотеке этого компьютера'))
    await looks.newSpread()
    expect(store.toast?.kind).toBe('error')
    api.newSpread.mockImplementation(async () => grown(2))
    await looks.newSpread()
    expect(store.toast?.text).toBe('Разворот макета «Светлая» добавлен')
  })
})
