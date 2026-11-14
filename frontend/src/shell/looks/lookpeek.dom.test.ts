// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Крупный просмотр макета (163 U3): шаги, Enter, «Показать весь набор», Esc, макет пропал, кадр не загрузился; library:changed
// при открытом окне (перенесено из looks.dom.test.ts).
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
const { modalStack } = await import('../../base/modal/modalstack.svelte')
const { packsUi } = await import('../packs/packs.svelte')

const A = 'my:0a1b2c3d4e5f6071'
const B = 'my:1b2c3d4e5f607182'
const C = 'my:2c3d4e5f60718293'
const item = (id: string, name: string, o: Record<string, unknown> = {}) => ({
  id,
  name,
  theme: 'w',
  url: '',
  own: false,
  aspect: 0,
  w: 0,
  h: 0,
  steps: 7,
  from: 18,
  ...o,
})
const catalog = (items: unknown[]) => ({
  kind: 'album',
  themes: [
    { id: 'all', name: 'Все', count: items.length },
    { id: 'w', name: 'Свадьба', count: items.length },
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
const withSet = () => [
  item(A, 'Светлая', { steps: 9, from: 26, cover: true, pack: 'Акварель', packId: 'p1' }),
  item(B, 'Классика', { pack: 'Акварель', packId: 'p1' }),
  item(C, 'Графика', { pack: 'Линии', packId: 'p2' }),
]

const tiles = () => Array.from(document.querySelectorAll<HTMLButtonElement>('button.tile'))
const tile = (name: string) => tiles().find((b) => b.title.startsWith(name)) as HTMLButtonElement
const btn = (label: string) => Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === label)
const shell = () => document.querySelector<HTMLElement>('[aria-label="Крупный просмотр макета"]')
const bigImg = () => Array.from(document.querySelectorAll<HTMLImageElement>('.sheet img')).at(-1)
const previewSrc = () => bigImg()?.getAttribute('src') ?? ''
const key = (k: string, init: KeyboardEventInit = {}, on: Element | null = document.activeElement) => {
  const e = new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true, ...init })
  ;(on ?? document.body).dispatchEvent(e)
  return e
}
async function peekOf(name: string) {
  await fireEvent.click(tile(name))
  await vi.waitFor(() => expect(looks.ready).toBe(true))
}

async function opened(items: unknown[] = withSet()) {
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
  Object.assign(looks, {
    cat: null,
    theme: '',
    sel: '',
    spreads: 0,
    pick: 'bg',
    step: 1,
    info: null,
    cnt: null,
    plan: null,
    modePlan: null,
    busy: false,
  })
  looks.filters = { pack: '', form: '', style: '' }
  store.toast = null
  store.view = { ...store.view, rev: 1, settings: { ...store.view.settings, pageW: 300, pageH: 300, sheet: false } } as never
})
afterEach(() => {
  cleanup()
  looks.close()
  vi.restoreAllMocks()
})

describe('LookPeek: шаги', () => {
  it('подпись, лента и строка клавиш: «имя · набор · от N фото · N разворотов», «← → — шаги · Enter — применить · Esc — назад»', async () => {
    await opened()
    await peekOf('Светлая')
    expect(shell()?.querySelector('.cap .name')?.textContent).toBe('Светлая · набор «Акварель» · от 26 фото · 9 разворотов')
    expect(shell()?.querySelector('.hint')?.textContent).toBe('← → — шаги · Enter — применить · Esc — назад')
    expect(shell()?.querySelectorAll('.chip')).toHaveLength(10)
  })

  it('← → и ‹ › — шаги; на краях ничего не листают', async () => {
    await opened()
    await peekOf('Классика') // шаги 1…7
    key('ArrowLeft')
    expect(looks.step).toBe(1)
    const next = document.querySelector('[aria-label="Следующий шаг"]') as HTMLButtonElement
    await fireEvent.click(next)
    expect(looks.step).toBe(2)
    expect(key('ArrowRight').defaultPrevented).toBe(true)
    expect(looks.step).toBe(3)
    const prev = document.querySelector('[aria-label="Предыдущий шаг"]') as HTMLButtonElement
    await fireEvent.click(prev)
    expect(looks.step).toBe(2)
    for (let k = 0; k < 10; k++) key('ArrowRight')
    expect(looks.step).toBe(7)
    await tick()
    expect(next.disabled).toBe(true)
    expect(previewSrc()).toContain('i=7&')
  })

  it('щелчок по шагу в ленте показывает его', async () => {
    await opened()
    await peekOf('Классика')
    await fireEvent.click(btn('4Шаг 4') as HTMLElement)
    expect(looks.step).toBe(4)
    expect(previewSrc()).toContain('i=4&')
    expect(btn('4Шаг 4')?.getAttribute('aria-pressed')).toBe('true')
  })

  it('с открытым окном «Применить» стрелки просмотр не листают (окно сверху)', async () => {
    await opened()
    await peekOf('Классика')
    await fireEvent.click(btn('Применить…') as HTMLElement)
    await tick()
    key('ArrowRight')
    expect(looks.step).toBe(1)
  })

  it('собранный альбом: лист просмотра — серия без bg (163-15), как и до сборки', async () => {
    api.plan.mockImplementation(async (_id: string, o: { mode: string; spreads: number }) =>
      plan({ mode: o.mode, spreads: o.spreads, full: true }),
    )
    await opened()
    await peekOf('Классика')
    expect(looks.mode).toBe('bg')
    expect(previewSrc()).toContain('bg=0')
    expect(previewSrc()).not.toContain('bg=1')
  })
})

describe('LookPeek: Enter и «Применить…»', () => {
  it('Enter открывает одно окно «Применить»; автоповтор и Enter при постановке (busy) — ничего', async () => {
    await opened()
    await peekOf('Классика')
    const before = modalStack.count()
    expect(key('Enter', { repeat: true }).defaultPrevented).toBe(true) // оболочка погасила, окна нет
    await tick()
    expect(looks.ask).toBeNull()
    looks.busy = true
    key('Enter')
    await tick()
    expect(looks.ask).toBeNull()
    looks.busy = false
    expect(key('Enter').defaultPrevented).toBe(true)
    await tick()
    expect(looks.ask?.kind).toBe('form')
    expect(modalStack.count()).toBe(before + 1)
    key('Enter') // верхнее окно — «Применить»: просмотр второго окна не открывает
    await tick()
    expect(modalStack.count()).toBe(before + 1)
    expect(api.apply).not.toHaveBeenCalled()
  })

  it('«Применить…» серая, пока карточка не пришла; Enter в это время ничего не открывает', async () => {
    let ok!: (v: ReturnType<typeof info>) => void
    api.info.mockImplementation(() => new Promise((r) => (ok = r)))
    await opened()
    await fireEvent.click(tile('Классика'))
    await tick()
    expect((btn('Применить…') as HTMLButtonElement).disabled).toBe(true)
    key('Enter')
    await tick()
    expect(looks.ask).toBeNull()
    ok(info(7))
    await vi.waitFor(() => expect((btn('Применить…') as HTMLButtonElement).disabled).toBe(false))
  })

  it('Enter на кнопке — её собственный щелчок (оболочка клавишу не берёт)', async () => {
    await opened()
    await peekOf('Классика')
    const b = btn('Применить…') as HTMLButtonElement
    b.focus()
    expect(key('Enter', {}, b).defaultPrevented).toBe(false)
  })
})

describe('LookPeek: «Показать весь набор»', () => {
  it('есть при ≥ 2 макетах набора, нет у набора из одного макета', async () => {
    await opened()
    await peekOf('Светлая')
    expect(btn('Показать весь набор')).toBeTruthy()
    await fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })
    await tick()
    await peekOf('Графика')
    expect(btn('Показать весь набор')).toBeUndefined()
  })

  it('щелчок: тема «Все», отбор по набору, просмотр закрыт, плитка в фокусе', async () => {
    await opened()
    await peekOf('Светлая')
    await fireEvent.click(btn('Показать весь набор') as HTMLElement)
    await tick()
    expect(looks.theme).toBe('all')
    expect(looks.filters.pack).toBe('p1')
    expect(looks.peek.id).toBe('')
    expect(shell()).toBeNull()
    expect(tiles().map((t) => t.dataset.decorId)).toEqual([A, B])
    await vi.waitFor(() => expect(document.activeElement).toBe(tile('Светлая')))
  })
})

describe('LookPeek: назад и закрытие', () => {
  it('Esc — к плиткам, плитка в фокусе; окно макетов цело; затем окон нет', async () => {
    await opened()
    await peekOf('Классика')
    expect(modalStack.count()).toBe(2)
    expect(key('Escape').defaultPrevented).toBe(true)
    await vi.waitFor(() => expect(document.activeElement).toBe(tile('Классика')))
    expect(shell()).toBeNull()
    expect(looks.open).toBe(true)
    expect(modalStack.count()).toBe(1)
    looks.close()
    await tick()
    expect(modalStack.count()).toBe(0)
  })

  it('✕, правый щелчок и щелчок по подложке закрывают; щелчок по листу — нет', async () => {
    await opened()
    await peekOf('Классика')
    await fireEvent.pointerDown(document.querySelector('.sheet') as HTMLElement)
    expect(shell()).not.toBeNull()
    await fireEvent.click(document.querySelector('.cap .x') as HTMLElement)
    await tick()
    expect(shell()).toBeNull()
    await peekOf('Классика')
    await fireEvent.contextMenu(shell() as HTMLElement)
    await tick()
    expect(shell()).toBeNull()
    await peekOf('Классика')
    await fireEvent.pointerDown(shell()!.parentElement as HTMLElement)
    await tick()
    expect(shell()).toBeNull()
  })

  it('макет удалили при открытом просмотре и окне «Применить» — оба закрываются', async () => {
    await opened()
    await peekOf('Классика')
    await fireEvent.click(btn('Применить…') as HTMLElement)
    await tick()
    expect(looks.ask?.kind).toBe('form')
    api.catalog.mockResolvedValue(catalog([item(A, 'Светлая', { steps: 9 })]))
    await looks.refresh()
    await tick()
    expect(looks.sel).toBe('')
    expect(looks.peek.id).toBe('')
    expect(looks.ask).toBeNull()
    expect(shell()).toBeNull()
    expect(modalStack.count()).toBe(1) // осталось окно макетов
  })

  it('кадр не загрузился: остаётся миниатюра плитки, лист не пустеет, тоста нет', async () => {
    await opened()
    await peekOf('Светлая') // шаг плитки — обложка (0): миниатюра есть
    const imgs = () => Array.from(document.querySelectorAll<HTMLImageElement>('.sheet img'))
    expect(imgs()).toHaveLength(2)
    expect(imgs()[0].getAttribute('src')).toContain('f=1')
    await fireEvent.error(imgs()[1])
    await tick()
    expect(imgs()).toHaveLength(1)
    expect(imgs()[0].getAttribute('src')).toContain('f=1')
    expect(store.toast).toBeNull()
  })

  it('другой шаг, чем у плитки: миниатюры нет, показан только крупный кадр', async () => {
    await opened()
    await peekOf('Светлая')
    await fireEvent.click(btn('1Шаг 1') as HTMLElement)
    await tick()
    expect(document.querySelectorAll('.sheet img')).toHaveLength(1)
  })
})

describe('LooksDialog: library:changed', () => {
  it('открытое окно перечитывает каталог; пропавший выбранный макет сбрасывает выбор', async () => {
    await opened()
    await peekOf('Классика')
    api.catalog.mockResolvedValue(catalog([item(A, 'Светлая', { steps: 9 })]))
    packsUi.changed([])
    await vi.waitFor(() => expect(api.catalog).toHaveBeenCalledTimes(2))
    await vi.waitFor(() => expect(looks.sel).toBe(''))
    expect(tiles()).toHaveLength(1)
    expect(looks.peek.id).toBe('') // просмотр закрылся вместе с пропавшим макетом
    expect(document.querySelector('.sheet')).toBeNull()
  })

  it('выбранный макет остался — карточка и числа перечитаны, выбор цел', async () => {
    await opened()
    await peekOf('Классика')
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
