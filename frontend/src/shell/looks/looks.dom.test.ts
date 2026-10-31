// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Окно «Макеты альбома» на подставном lookApi (131 U1): открытие, выбор плитки, «Что сделать», формат, пустое, память выбора.
const api = vi.hoisted(() => ({
  catalog: vi.fn(),
  info: vi.fn(),
  count: vi.fn(),
  plan: vi.fn(),
  apply: vi.fn(),
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
const { modes } = await import('../../base/prefs/view.svelte')
const { packsUi } = await import('../packs/packs.svelte')

const A = 'my:0a1b2c3d4e5f6071'
const B = 'my:1b2c3d4e5f607182'
const item = (id: string, name: string, o: Record<string, unknown> = {}) => ({
  id,
  name,
  theme: 'w',
  url: '',
  own: false,
  aspect: 0,
  w: 0,
  h: 0,
  ...o,
})
const catalog = (items: unknown[]) => ({
  kind: 'album',
  themes: [
    { id: 'all', name: 'Все', count: items.length },
    { id: 'w', name: 'Свадьба', count: items.length },
    { id: 'mine', name: 'Мои', count: 0 },
  ],
  items,
  recent: [],
  readOnly: false,
})
const info = (n: number, cover = false) => ({
  steps: Array.from({ length: n }, (_, k) => ({ name: `Шаг ${k + 1}`, role: k === 1 ? 'repeat' : '', places: 3, pages: 2 })),
  cover,
  min: 1,
  max: 20,
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
  left: 0,
  prev: '',
  missing: 0,
  ...o,
})

const tiles = () => Array.from(document.querySelectorAll<HTMLButtonElement>('button.tile'))
const tile = (name: string) => tiles().find((b) => b.title.startsWith(name)) as HTMLButtonElement
const text = () => document.body.textContent ?? ''
const btn = (label: string) => Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === label)
const previewSrc = () => document.querySelector<HTMLImageElement>('.sheet img')?.getAttribute('src') ?? ''
const spreadsInput = () => document.querySelector<HTMLInputElement>('#look-spreads')

async function opened(items = [item(A, 'Светлая', { steps: 9, from: 26, cover: true }), item(B, 'Классика', { steps: 7, from: 18 })]) {
  api.catalog.mockResolvedValue(catalog(items))
  render(LooksDialog)
  await looks.show()
  await tick()
}

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.info.mockImplementation(async (id: string) => (id === A ? info(9, true) : info(7)))
  api.count.mockImplementation(async (_id: string, n: number) => ({ n, places: n * 5, fit: 0 }))
  api.plan.mockImplementation(async (_id: string, o: { mode: string; spreads: number }) => plan({ mode: o.mode, spreads: o.spreads }))
  api.apply.mockImplementation(async () => ({ view: store.view, plan: plan() }))
  looks.close()
  Object.assign(looks, { cat: null, theme: '', sel: '', spreads: 0, pick: 'bg', step: 1, info: null, cnt: null, plan: null, busy: false })
  store.view = { ...store.view, rev: 1, settings: { ...store.view.settings, pageW: 300, pageH: 300, sheet: false } } as never
  modes.details = false
  modes.sections.settings = false
})
afterEach(() => {
  cleanup()
  looks.close()
  vi.restoreAllMocks()
})

describe('LooksDialog: плитки и предпросмотр', () => {
  it('плитки с подписью; картинка плитки — лицевая сторона обложки (f=1, половина листа) или разворот 1, адрес через lookUrl', async () => {
    await opened()
    expect(text()).toContain('от 26 фото · 9 разворотов')
    expect(text()).toContain('от 18 фото · 7 разворотов')
    const [a, b] = tiles().map((t) => t.querySelector('img')?.getAttribute('src') ?? '')
    expect(a).toMatch(/^\/media\/look\/0a1b2c3d4e5f6071\?v=1&i=0&px=\d+&r=1&bg=0&f=1$/)
    expect(b).toMatch(/^\/media\/look\/1b2c3d4e5f607182\?v=1&i=1&px=\d+&r=2&bg=0$/)
  })

  it('до выбора справа подсказка, «Применить» серая', async () => {
    await opened()
    expect(text()).toContain('Выберите макет слева')
    expect((btn('Применить') as HTMLButtonElement).disabled).toBe(true)
    expect(document.querySelector('.sheet')).toBeNull()
  })

  it('щелчок по плитке показывает макет справа и ничего не ставит', async () => {
    await opened()
    await fireEvent.click(tile('Светлая'))
    await vi.waitFor(() => expect(spreadsInput()).not.toBeNull())
    expect(api.apply).not.toHaveBeenCalled()
    expect(tile('Светлая').getAttribute('aria-pressed')).toBe('true')
    expect(previewSrc()).toContain('/media/look/0a1b2c3d4e5f6071?v=1&i=0&')
    expect(spreadsInput()?.value).toBe('9')
    expect(text()).toContain('мест 45')
    // лента: обложка, 1…9; ↻ у повторяемого
    const chips = Array.from(document.querySelectorAll('.chip')).map((c) => c.textContent?.trim())
    expect(chips.slice(0, 3)).toEqual(['Обложка', '1Шаг 1', '2Шаг 2↻'])
    expect(chips).toHaveLength(10)
  })

  it('повторный щелчок по выбранной плитке — холостой (карточка не запрашивается заново)', async () => {
    await opened()
    await fireEvent.click(tile('Классика'))
    await vi.waitFor(() => expect(spreadsInput()).not.toBeNull())
    await fireEvent.click(tile('Классика'))
    await tick()
    expect(api.info).toHaveBeenCalledTimes(1)
  })

  it('‹ › и стрелки листают шаги; на краях ‹ серая; в поле числа стрелки не листают', async () => {
    await opened()
    await fireEvent.click(tile('Классика')) // без обложки: шаги 1…7
    await vi.waitFor(() => expect(spreadsInput()).not.toBeNull())
    expect(previewSrc()).toContain('i=1&')
    expect((document.querySelector('[aria-label="Предыдущий шаг"]') as HTMLButtonElement).disabled).toBe(true)
    await fireEvent.click(document.querySelector('[aria-label="Следующий шаг"]') as HTMLElement)
    expect(previewSrc()).toContain('i=2&')
    await fireEvent.keyDown(document.activeElement ?? document.body, { key: 'ArrowRight' })
    expect(previewSrc()).toContain('i=3&')
    await fireEvent.keyDown(document.activeElement ?? document.body, { key: 'ArrowLeft' })
    expect(previewSrc()).toContain('i=2&')
    await fireEvent.keyDown(spreadsInput() as HTMLElement, { key: 'ArrowRight' })
    expect(previewSrc()).toContain('i=2&')
    await fireEvent.click(btn('5Шаг 5') as HTMLElement)
    expect(previewSrc()).toContain('i=5&')
  })
})

describe('LooksDialog: «Что сделать» и «Разворотов»', () => {
  it('без фото в местах «Что сделать» нет, поле «Разворотов» есть', async () => {
    await opened()
    await fireEvent.click(tile('Светлая'))
    await vi.waitFor(() => expect(spreadsInput()).not.toBeNull())
    expect(text()).not.toContain('Что сделать')
    expect(previewSrc()).toContain('bg=0')
  })

  it('при plan.full — «Что сделать»: «Фоны по серии» (без «Разворотов», предпросмотр bg=1) и «Добавить развороты макета»', async () => {
    api.plan.mockImplementation(async (_id: string, o: { mode: string; spreads: number }) =>
      plan({ mode: o.mode, spreads: o.spreads, full: true }),
    )
    await opened()
    await fireEvent.click(tile('Светлая'))
    await vi.waitFor(() => expect(text()).toContain('Что сделать'))
    expect(btn('Фоны по серии')?.getAttribute('aria-pressed')).toBe('true')
    expect(spreadsInput()).toBeNull()
    expect(text()).toContain('Стикеры макета легли бы на ваши фото')
    expect(previewSrc()).toContain('bg=1')
    await fireEvent.click(btn('Добавить развороты макета') as HTMLElement)
    expect(spreadsInput()).not.toBeNull()
    expect(text()).not.toContain('Стикеры макета легли бы')
    expect(previewSrc()).toContain('bg=0')
  })

  it('«Сменить макет целиком…» — подпись, когда альбом уже по макету (plan.prev), фото нет', async () => {
    api.plan.mockImplementation(async (_id: string, o: { mode: string; spreads: number }) => plan({ mode: o.mode, prev: 'Классика' }))
    await opened()
    await fireEvent.click(tile('Светлая'))
    await vi.waitFor(() => expect(text()).toContain('Сменить макет целиком можно, пока в местах нет фото'))
    expect(text()).not.toContain('Что сделать')
  })

  it('число разворотов: зажим в пределах Go, пересчёт мест и плана', async () => {
    await opened()
    await fireEvent.click(tile('Светлая'))
    await vi.waitFor(() => expect(spreadsInput()).not.toBeNull())
    const input = spreadsInput() as HTMLInputElement
    input.value = '99'
    await fireEvent.change(input)
    await vi.waitFor(() => expect(text()).toContain('мест 100'))
    expect(input.value).toBe('20')
    expect(api.plan).toHaveBeenLastCalledWith(A, { mode: 'series', spreads: 20 })
    input.value = ''
    await fireEvent.change(input)
    await vi.waitFor(() => expect(text()).toContain('мест 5'))
    expect(input.value).toBe('1')
  })

  it('«Подобрать под N свободных фото» — без выделения, только с фото и подбором от Go; ставит число разворотов', async () => {
    vi.spyOn(store.hooks, 'pickFree').mockReturnValue(Array.from({ length: 64 }, (_, k) => `p${k}`))
    api.count.mockImplementation(async (_id: string, n: number, k: number) => ({ n, places: n * 5, fit: k > 0 ? 14 : 0 }))
    await opened()
    await fireEvent.click(tile('Светлая'))
    await vi.waitFor(() => expect(btn('Подобрать под 64 свободных фото')).toBeTruthy())
    expect(api.count).toHaveBeenLastCalledWith(A, 9, 64)
    await fireEvent.click(btn('Подобрать под 64 свободных фото') as HTMLElement)
    await vi.waitFor(() => expect(spreadsInput()?.value).toBe('14'))
  })

  it('с выделением K — выбранные фото, и подпись говорит «выбранных»', async () => {
    vi.spyOn(store, 'selectedIds', 'get').mockReturnValue(['a', 'b', 'c'])
    api.count.mockImplementation(async (_id: string, n: number, k: number) => ({ n, places: n * 5, fit: k > 0 ? 2 : 0 }))
    await opened()
    await fireEvent.click(tile('Светлая'))
    await vi.waitFor(() => expect(btn('Подобрать под 3 выбранных фото')).toBeTruthy())
    expect(text()).not.toContain('свободных фото')
  })

  it('нет фото для подбора — ссылки нет', async () => {
    await opened()
    await fireEvent.click(tile('Светлая'))
    await vi.waitFor(() => expect(spreadsInput()).not.toBeNull())
    expect(text()).not.toContain('Подобрать под')
  })
})

describe('LooksDialog: формат, память, пустое', () => {
  it('«Формат альбома: 30×30 · Изменить…»: закрывает окно и раскрывает «Настройки альбома»', async () => {
    await opened()
    expect(text()).toContain('Формат альбома: 30×30')
    await fireEvent.click(btn('Изменить…') as HTMLElement)
    expect(looks.open).toBe(false)
    expect(modes.details).toBe(true)
    expect(modes.sections.settings).toBe(true)
    expect(document.querySelector('[role="dialog"]')).toBeNull()
  })

  it('выбор и число разворотов помнятся: после «Изменить…» окно открывается на той же плитке', async () => {
    await opened()
    await fireEvent.click(tile('Классика'))
    await vi.waitFor(() => expect(spreadsInput()).not.toBeNull())
    const input = spreadsInput() as HTMLInputElement
    input.value = '12'
    await fireEvent.change(input)
    await vi.waitFor(() => expect(text()).toContain('мест 60'))
    await fireEvent.click(btn('Изменить…') as HTMLElement)
    api.info.mockClear()
    await looks.show()
    await tick()
    await vi.waitFor(() => expect(spreadsInput()?.value).toBe('12'))
    expect(tile('Классика').getAttribute('aria-pressed')).toBe('true')
    expect(previewSrc()).toContain('/media/look/1b2c3d4e5f607182')
  })

  it('выбранный макет пропал из библиотеки — выбор сброшен', async () => {
    await opened()
    await fireEvent.click(tile('Классика'))
    await vi.waitFor(() => expect(spreadsInput()).not.toBeNull())
    looks.close()
    api.catalog.mockResolvedValue(catalog([item(A, 'Светлая', { steps: 9, cover: true })]))
    await looks.show()
    await tick()
    expect(looks.sel).toBe('')
    expect(text()).toContain('Выберите макет слева')
  })

  it('пустая библиотека: «Макетов пока нет…» и «Наборы…»', async () => {
    const show = vi.spyOn(packsUi, 'show').mockResolvedValue()
    await opened([])
    expect(text()).toContain('Макетов пока нет — их приносят наборы библиотеки')
    expect(tiles()).toHaveLength(0)
    expect(btn('Применить')).toBeUndefined()
    await fireEvent.click(btn('Наборы…') as HTMLElement)
    expect(show).toHaveBeenCalled()
  })

  it('пустая тема: слова темы, окно целое', async () => {
    await opened()
    const mine = Array.from(document.querySelectorAll('.vseg button')).find((b) => b.textContent?.startsWith('Мои'))
    await fireEvent.click(mine as HTMLElement)
    await tick()
    expect(text()).toContain('Здесь появятся ваши макеты')
    expect(tiles()).toHaveLength(0)
  })

  it('один макет в библиотеке — окно как обычно', async () => {
    await opened([item(A, 'Светлая', { steps: 1 })])
    expect(tiles()).toHaveLength(1)
    expect(text()).toContain('1 разворот')
  })
})

describe('LooksDialog: «Применить»', () => {
  it('ApplyLook с режимом и числом разворотов, окно закрывается', async () => {
    await opened()
    await fireEvent.click(tile('Светлая'))
    await vi.waitFor(() => expect(spreadsInput()).not.toBeNull())
    await fireEvent.click(btn('Применить') as HTMLElement)
    await vi.waitFor(() => expect(looks.open).toBe(false))
    expect(api.apply).toHaveBeenCalledWith(A, { mode: 'series', spreads: 9 })
  })

  it('в собранном альбоме по умолчанию «Фоны по серии»', async () => {
    api.plan.mockImplementation(async (_id: string, o: { mode: string; spreads: number }) =>
      plan({ mode: o.mode, spreads: o.spreads, full: true }),
    )
    await opened()
    await fireEvent.click(tile('Классика'))
    await vi.waitFor(() => expect(text()).toContain('Что сделать'))
    await fireEvent.click(btn('Применить') as HTMLElement)
    await vi.waitFor(() => expect(api.apply).toHaveBeenCalledWith(B, { mode: 'bg', spreads: 7 }))
  })

  it('заменится своё — не ставит без вопроса (вопрос рисует U2)', async () => {
    api.plan.mockImplementation(async (_id: string, o: { mode: string; spreads: number }) => plan({ mode: o.mode, replaced: 3 }))
    await opened()
    await fireEvent.click(tile('Светлая'))
    await vi.waitFor(() => expect(spreadsInput()).not.toBeNull())
    await fireEvent.click(btn('Применить') as HTMLElement)
    await tick()
    expect(api.apply).not.toHaveBeenCalled()
    expect(looks.ask?.kind).toBe('apply')
    expect(looks.open).toBe(true)
  })

  it('правка альбома событиями Go (формат, другая правка) пересчитывает числа и план', async () => {
    await opened()
    await fireEvent.click(tile('Светлая'))
    await vi.waitFor(() => expect(spreadsInput()).not.toBeNull())
    api.plan.mockClear()
    store.view = { ...store.view, rev: 2 } as never
    await vi.waitFor(() => expect(api.plan).toHaveBeenCalledTimes(1))
  })
})

describe('LooksDialog: ✎ и × у своих', () => {
  it('переименование и удаление идут в библиотеку, каталог перечитывается', async () => {
    await opened([item(A, 'Светлая', { steps: 9 }), item(B, 'Мой', { steps: 7, own: true })])
    await fireEvent.click(document.querySelector('[aria-label="Переименовать макет"]') as HTMLElement)
    const field = document.querySelector<HTMLInputElement>('input[type="text"]') as HTMLInputElement
    expect(field.value).toBe('Мой')
    field.value = ' Новый '
    await fireEvent.input(field)
    await fireEvent.click(btn('Сохранить') as HTMLElement)
    await vi.waitFor(() => expect(api.rename).toHaveBeenCalledWith(B, 'Новый'))
    await vi.waitFor(() => expect(api.catalog).toHaveBeenCalledTimes(2))
    await fireEvent.click(document.querySelector('[aria-label="Убрать макет из библиотеки"]') as HTMLElement)
    expect(text()).toContain('Макет «Мой» будет убран из библиотеки')
    await fireEvent.click(btn('Убрать') as HTMLElement)
    await vi.waitFor(() => expect(api.hide).toHaveBeenCalledWith(B))
    await vi.waitFor(() => expect(api.catalog).toHaveBeenCalledTimes(3)) // открытие, после ✎, после ×
  })
})

// Отложенный ответ: тест сам решает, когда Go «отвечает».
function later<T>() {
  let ok!: (v: T) => void
  const p = new Promise<T>((r) => (ok = r))
  return { p, ok }
}

describe('LooksDialog: гонки ответов', () => {
  it('смена плитки при отложенном ответе: «Применить» серая, ApplyLook чужого макета не уходит', async () => {
    await opened()
    await fireEvent.click(tile('Светлая'))
    await vi.waitFor(() => expect(spreadsInput()).not.toBeNull())
    const slow = later<ReturnType<typeof info>>()
    api.info.mockImplementation((id: string) => (id === B ? slow.p : Promise.resolve(info(9, true))))
    await fireEvent.click(tile('Классика'))
    await tick()
    expect((btn('Применить') as HTMLButtonElement).disabled).toBe(true)
    api.plan.mockClear()
    await looks.apply() // прямой вызов в обход серой кнопки
    expect(api.apply).not.toHaveBeenCalled()
    expect(api.plan).not.toHaveBeenCalled()
    slow.ok(info(7))
    await vi.waitFor(() => expect((btn('Применить') as HTMLButtonElement).disabled).toBe(false))
    await fireEvent.click(btn('Применить') as HTMLElement)
    await vi.waitFor(() => expect(api.apply).toHaveBeenCalledWith(B, { mode: 'series', spreads: 7 }))
  })

  it('устаревший ответ карточки (A пришёл после B) выбор B не портит', async () => {
    const slowA = later<ReturnType<typeof info>>()
    api.info.mockImplementation((id: string) => (id === A ? slowA.p : Promise.resolve(info(7))))
    await opened()
    await fireEvent.click(tile('Светлая'))
    await fireEvent.click(tile('Классика'))
    await vi.waitFor(() => expect(spreadsInput()?.value).toBe('7'))
    slowA.ok(info(9, true))
    await tick()
    await tick()
    expect(looks.sel).toBe(B)
    expect(looks.info?.steps).toHaveLength(7)
    expect(looks.spreads).toBe(7)
    expect(looks.chips.some((c) => c.i === 0)).toBe(false)
  })

  it('устаревший ответ чисел (n=9 после n=5) не затирает свежий', async () => {
    const first = later<{ n: number; places: number; fit: number }>()
    await opened()
    api.count.mockImplementationOnce(() => first.p)
    await fireEvent.click(tile('Светлая')) // count(9) повис
    await vi.waitFor(() => expect(api.count).toHaveBeenCalledTimes(1))
    await vi.waitFor(() => expect(looks.info).not.toBeNull())
    await looks.setSpreads(5)
    await vi.waitFor(() => expect(text()).toContain('мест 25'))
    first.ok({ n: 9, places: 999, fit: 0 })
    await tick()
    await tick()
    expect(looks.cnt?.places).toBe(25)
  })

  it('sync не отменяет идущую загрузку карточки: макет всё равно выбирается', async () => {
    const slow = later<ReturnType<typeof info>>()
    await opened()
    api.info.mockImplementation(() => slow.p)
    await fireEvent.click(tile('Классика'))
    store.view = { ...store.view, rev: 2 } as never // правка Go, пока карточка в пути
    await tick()
    slow.ok(info(7))
    await vi.waitFor(() => expect(looks.ready).toBe(true))
    expect(spreadsInput()?.value).toBe('7')
  })

  it('устаревший ответ каталога после закрытия окна отбрасывается', async () => {
    const slow = later<ReturnType<typeof catalog>>()
    api.catalog.mockImplementation(() => slow.p)
    render(LooksDialog)
    const shown = looks.show()
    looks.close()
    slow.ok(catalog([item(A, 'Светлая')]))
    await shown
    expect(looks.cat).toBeNull()
  })

  it('свежий PlanLook при «Применить» важнее показанного: фото уйдут в ленту (left) — вопрос, ApplyLook нет', async () => {
    await opened()
    await fireEvent.click(tile('Светлая'))
    await vi.waitFor(() => expect(looks.ready).toBe(true))
    api.plan.mockResolvedValueOnce(plan({ left: 3 }))
    await fireEvent.click(btn('Применить') as HTMLElement)
    await vi.waitFor(() => expect(looks.ask?.kind).toBe('apply'))
    expect(api.apply).not.toHaveBeenCalled()
  })
})

describe('LooksDialog: «Добавить развороты макета»', () => {
  const fullPlan = (_id: string, o: { mode: string; spreads: number }) =>
    plan({ mode: o.mode, spreads: o.spreads, full: true, places: o.mode === 'add' ? o.spreads * 3 : 46 })

  it('поле — сколько добавить (начало 1), места из плана добавления, подбора нет', async () => {
    vi.spyOn(store.hooks, 'pickFree').mockReturnValue(['p1', 'p2', 'p3'])
    api.count.mockImplementation(async (_id: string, n: number) => ({ n, places: n * 5, fit: 14 }))
    api.plan.mockImplementation(async (id: string, o: { mode: string; spreads: number }) => fullPlan(id, o))
    await opened()
    await fireEvent.click(tile('Светлая'))
    await vi.waitFor(() => expect(text()).toContain('Что сделать'))
    await fireEvent.click(btn('Добавить развороты макета') as HTMLElement)
    await vi.waitFor(() => expect(spreadsInput()?.value).toBe('1'))
    await vi.waitFor(() => expect(text()).toContain('мест 3'))
    expect(text()).not.toContain('Подобрать под')
    const input = spreadsInput() as HTMLInputElement
    input.value = '4'
    await fireEvent.change(input)
    await vi.waitFor(() => expect(text()).toContain('мест 12'))
    await fireEvent.click(btn('Применить') as HTMLElement)
    await vi.waitFor(() => expect(api.apply).toHaveBeenCalledWith(A, { mode: 'add', spreads: 4 }))
    // число серии при этом не тронуто
    expect(looks.spreads).toBe(9)
  })
})

describe('LooksDialog: library:changed', () => {
  it('открытое окно перечитывает каталог; пропавший выбранный макет сбрасывает выбор', async () => {
    await opened()
    await fireEvent.click(tile('Классика'))
    await vi.waitFor(() => expect(looks.ready).toBe(true))
    api.catalog.mockResolvedValue(catalog([item(A, 'Светлая', { steps: 9 })]))
    packsUi.changed([])
    await vi.waitFor(() => expect(api.catalog).toHaveBeenCalledTimes(2))
    await vi.waitFor(() => expect(looks.sel).toBe(''))
    expect(tiles()).toHaveLength(1)
    expect(text()).toContain('Выберите макет слева')
  })

  it('выбранный макет остался — карточка и числа перечитаны, выбор цел', async () => {
    await opened()
    await fireEvent.click(tile('Классика'))
    await vi.waitFor(() => expect(looks.ready).toBe(true))
    api.info.mockClear()
    looks.changed([])
    await vi.waitFor(() => expect(api.info).toHaveBeenCalledTimes(1))
    expect(looks.sel).toBe(B)
  })

  it('закрытое окно не ходит в Go; следующее открытие встаёт на тему свежего набора (073-1)', async () => {
    cleanup()
    packsUi.changed(['Школа-осень'])
    expect(api.catalog).not.toHaveBeenCalled()
    api.catalog.mockResolvedValue({
      ...catalog([item(A, 'Светлая'), item(B, 'Выпускной', { theme: 's', pack: 'Школа-осень' })]),
      themes: [
        { id: 'all', name: 'Все', count: 2 },
        { id: 'w', name: 'Свадьба', count: 1 },
        { id: 's', name: 'Школа', count: 1 },
      ],
    })
    render(LooksDialog)
    await looks.show()
    expect(looks.theme).toBe('s')
    // один раз: повторное открытие тему не навязывает
    looks.close()
    looks.theme = 'w'
    await looks.show()
    expect(looks.theme).toBe('w')
  })
})

describe('LooksDialog: Esc и ключ картинки', () => {
  it('Esc закрывает окно, выбор помнится', async () => {
    await opened()
    await fireEvent.click(tile('Классика'))
    await vi.waitFor(() => expect(looks.ready).toBe(true))
    await fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })
    expect(looks.open).toBe(false)
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(looks.sel).toBe(B)
  })

  it('id без 16 hex — картинку не просим', async () => {
    await opened([item('my:не-ключ', 'Битый', { steps: 3 })])
    expect(tiles()).toHaveLength(1)
    expect(tiles()[0].querySelector('img')?.hasAttribute('src')).toBe(false)
  })
})
