// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'

// Поле поиска фото по имени (0.26-124): ⌕, Esc в два шага, ✕, «Найдено N из M», Ctrl+F, «Показано» фильтра отметок. Go подменён.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ Marks: async () => ({}) }))

const { default: NameSearch } = await import('./NameSearch.svelte')
const { default: MarkFilter } = await import('./MarkFilter.svelte')
const { default: LibraryBar } = await import('../library/LibraryBar.svelte')
const { store } = await import('../../base/core/store.svelte')
const { nameSearch } = await import('./namefind.svelte')
const { libfilter } = await import('./libfilter.svelte')
const { ownMarks } = await import('./ownmarks.svelte')
const { modes } = await import('../../base/prefs/view.svelte')
const { gallery } = await import('../gallery/gallery.svelte')
const { keymap } = await import('../../base/keys/keymap.svelte')
const { useCatalog, dropCatalog } = await import('../../base/keys/keymap.fixture')
const { run } = await import('../../shell/menu/commands')
const { wire } = await import('../../shell/boot/wire')

wire()

const NAMES = ['IMG_0798.JPG', 'IMG_0801.JPG', 'IMG_0805.JPG', 'IMG_1798.JPG', 'Свадьба_015.JPG']
const ph = (name: string) => ({ id: name, path: `D:/a/${name}`, name, w: 100, h: 100 })
const ids = () => store.visible.map((p) => p.id)

const field = () => screen.getByRole('searchbox', { name: 'Поиск по имени файла' }) as HTMLInputElement
const lens = () => screen.getByRole('button', { name: 'Найти фото по имени' }) as HTMLButtonElement
const reset = () => screen.queryByRole('button', { name: 'Сбросить поиск' })
const found = () => screen.queryByRole('status')?.textContent?.replace(/\s+/g, ' ').trim()
// Кадр после tick: focus() ждёт перерисовку и кадр.
const frame = () => new Promise<void>((r) => requestAnimationFrame(() => r()))
const settle = async () => {
  await tick()
  await frame()
  await tick()
}
const type = async (text: string) => {
  await fireEvent.input(field(), { target: { value: text } })
}
const key = (el: Element, k: string, o: KeyboardEventInit = {}) => fireEvent.keyDown(el, { key: k, ...o })

beforeEach(() => {
  useCatalog()
  nameSearch.reset()
  libfilter.reset()
  modes.big = false
  gallery.cur = null as never
  store.photos = NAMES.map(ph) as never
  store.filter = 'all'
  ownMarks.map = {}
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
  dropCatalog()
  nameSearch.reset()
})

describe('⌕ и поле', () => {
  it('свёрнуто — кнопка с подсказкой и клавишей; поля нет', () => {
    render(NameSearch)
    expect(lens().title).toBe('Найти фото по имени (Ctrl+F)')
    expect(screen.queryByRole('searchbox')).toBeNull()
  })

  it('пустая лента — ⌕ неактивен', () => {
    store.photos = []
    render(NameSearch)
    expect(lens().disabled).toBe(true)
  })

  it('щелчок раскрывает поле с фокусом', async () => {
    render(NameSearch)
    await fireEvent.click(lens())
    await settle()
    expect(field().placeholder).toBe('Имя или номера')
    expect(document.activeElement).toBe(field())
  })

  it('набор действует через паузу, Enter — сразу', async () => {
    vi.useFakeTimers()
    render(NameSearch)
    nameSearch.open = true
    flushSync()
    await type('798')
    expect(ids()).toHaveLength(5) // пауза ещё идёт
    vi.advanceTimersByTime(150)
    flushSync()
    expect(ids()).toEqual(['IMG_0798.JPG'])
    await type('801')
    await key(field(), 'Enter')
    expect(ids()).toEqual(['IMG_0801.JPG'])
  })
})

describe('«Найдено N из M»', () => {
  const open = async (q: string) => {
    render(NameSearch)
    nameSearch.open = true
    flushSync()
    await type(q)
    await key(field(), 'Enter')
  }

  it('считает найденные из ленты до поиска', async () => {
    await open('798')
    expect(found()).toBe('Найдено 1 из 5')
  })

  it('ненайденные номера — «· нет: …», больше трёх — числом с подсказкой', async () => {
    await open('798, 7811, 7815')
    expect(found()).toBe('Найдено 1 из 5 · нет: 7811, 7815')
    await type('1, 2, 3, 4, 5, 798')
    await key(field(), 'Enter')
    expect(found()).toBe('Найдено 1 из 5 · нет 5')
    expect(screen.getByTitle('Не найдено: 1, 2, 3, 4, 5').textContent).toContain('· нет 5')
  })

  it('M — после папок, подборки и отметок; «Показано» фильтра отметок поиск не меняет', async () => {
    ownMarks.map = { 'IMG_0798.JPG': { rating: 5 }, 'IMG_0801.JPG': { rating: 5 }, 'IMG_1798.JPG': { rating: 5 } }
    libfilter.minRating = 5
    render(MarkFilter)
    const label = () => screen.getByRole('button', { name: /Показано/ }).textContent?.trim()
    expect(label()).toBe('Показано 3 из 5')
    render(NameSearch)
    nameSearch.open = true
    flushSync()
    await type('801')
    await key(field(), 'Enter')
    expect(found()).toBe('Найдено 1 из 3')
    expect(label()).toBe('Показано 3 из 5')
    await fireEvent.click(screen.getByRole('button', { name: /Показано/ }))
    expect(screen.getByText('Показано 3 из 5', { selector: '.count' })).toBeTruthy() // строка в окне фильтра
  })

  it('найдено, но скрыто лентой — «· скрыто: 805» в той же строке, рядом с «нет»', async () => {
    ownMarks.map = { 'IMG_0798.JPG': { rating: 5 }, 'IMG_0801.JPG': { rating: 5 } }
    libfilter.minRating = 5 // IMG_0805 без звёзд — в ленте её нет
    await open('798, 801, 805, 7777')
    expect(found()).toBe('Найдено 2 из 2 · нет: 7777 · скрыто: 805')
    expect(screen.getByTitle('Есть в библиотеке, но скрыто фильтрами ленты: 805').textContent).toContain('· скрыто: 805')
    libfilter.reset()
    await tick()
    expect(found()).toBe('Найдено 3 из 5 · нет: 7777')
  })

  it('нет запроса — нет и строки', () => {
    render(NameSearch)
    nameSearch.open = true
    flushSync()
    expect(found()).toBeUndefined()
    expect(reset()).toBeNull()
  })
})

describe('✕ и Esc', () => {
  const open = async (q: string) => {
    render(NameSearch)
    nameSearch.open = true
    flushSync()
    field().focus()
    await type(q)
    await key(field(), 'Enter')
  }

  it('✕ очищает, поле остаётся с фокусом', async () => {
    await open('798')
    await fireEvent.click(reset()!)
    expect(field().value).toBe('')
    expect(ids()).toHaveLength(5)
    expect(document.activeElement).toBe(field())
    expect(reset()).toBeNull()
  })

  it('поле из одних пробелов не держит раскрытие: потеря фокуса сворачивает и очищает его', async () => {
    render(NameSearch)
    nameSearch.open = true
    flushSync()
    await type('   ')
    expect(nameSearch.held).toBe(true) // пока открыто, раскрыто
    await fireEvent.blur(field())
    await tick()
    expect(screen.queryByRole('searchbox')).toBeNull()
    expect(nameSearch.q).toBe('')
    nameSearch.q = '  ' // пробелы без раскрытия не держат поле в строке
    expect(nameSearch.held).toBe(false)
  })

  it('Esc в поле из пробелов сворачивает его сразу', async () => {
    render(NameSearch)
    nameSearch.open = true
    flushSync()
    await type('  ')
    await key(field(), 'Escape')
    await settle()
    expect(screen.queryByRole('searchbox')).toBeNull()
  })

  it('Esc в два шага: непустое — очистить, пустое — свернуть и вернуть фокус на ⌕', async () => {
    await open('798')
    const onWindow = vi.fn()
    window.addEventListener('keydown', onWindow)
    await key(field(), 'Escape')
    expect(field().value).toBe('')
    expect(ids()).toHaveLength(5)
    await key(field(), 'Escape')
    await settle()
    expect(screen.queryByRole('searchbox')).toBeNull()
    expect(document.activeElement).toBe(lens())
    expect(onWindow).not.toHaveBeenCalled() // Esc не доходит до окна: выделение не снимается
    window.removeEventListener('keydown', onWindow)
  })

  it('потеря фокуса: пустое поле сворачивается, непустое остаётся', async () => {
    await open('798')
    await fireEvent.blur(field())
    expect(field().value).toBe('798')
    await fireEvent.click(reset()!)
    await fireEvent.blur(field())
    await tick()
    expect(screen.queryByRole('searchbox')).toBeNull()
  })

  it('поле в фокусе не отдаёт клавиши программе: Ctrl+F выделяет набранное и гасит поиск браузера', async () => {
    await open('798')
    const sel = vi.spyOn(field(), 'select')
    const e = new KeyboardEvent('keydown', { key: 'f', code: 'KeyF', ctrlKey: true, bubbles: true, cancelable: true })
    field().dispatchEvent(e)
    expect(e.defaultPrevented).toBe(true)
    expect(sel).toHaveBeenCalled()
    expect(field().value).toBe('798')
  })
})

describe('Ctrl+F', () => {
  it('команда findPhotos раскрывает поле: поля в DOM ещё нет — фокус ставится после перерисовки и кадра', async () => {
    render(NameSearch)
    expect(screen.queryByRole('searchbox')).toBeNull()
    await run('findPhotos')
    expect(document.activeElement).toBe(field())
  })

  it('поле уже открыто — фокус и выделение набранного', async () => {
    render(NameSearch)
    nameSearch.open = true
    flushSync()
    await type('798')
    const sel = vi.spyOn(field(), 'select')
    await nameSearch.focus()
    expect(document.activeElement).toBe(field())
    expect(sel).toHaveBeenCalled()
  })

  it('пустая лента, «Во весь экран», галерея — молчит', async () => {
    render(NameSearch)
    store.photos = []
    await nameSearch.focus()
    expect(nameSearch.open).toBe(false)
    store.photos = NAMES.map(ph) as never
    modes.big = true
    await nameSearch.focus()
    expect(nameSearch.open).toBe(false)
    modes.big = false
    gallery.cur = {} as never
    await nameSearch.focus()
    expect(nameSearch.open).toBe(false)
  })

  it('подсказка ⌕ показывает назначенную клавишу; снятая — без скобок', () => {
    useCatalog({ findPhotos: [] })
    render(NameSearch)
    expect(lens().title).toBe('Найти фото по имени')
    expect(keymap.hint('findPhotos')).toBe('')
  })
})

describe('узкая строка: ⌕ в «⋯»', () => {
  // Размеров в happy-dom нет: полоса 300 px, каждая группа 100 px, наблюдатель сразу отдаёт размер.
  class Obs {
    constructor(private cb: (e: unknown[]) => void) {}
    observe() {
      this.cb([])
    }
    unobserve() {}
    disconnect() {}
  }
  const inBar = (el: Element | null) => !!el?.closest('[data-bar="library"]')

  beforeEach(() => {
    vi.stubGlobal('ResizeObserver', Obs)
    vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(300)
    vi.spyOn(Element.prototype, 'getBoundingClientRect').mockReturnValue({ width: 100, height: 30 } as DOMRect)
    store.view = { ...store.view, folders: ['D:/a'] } as never
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  it('пустое поле уходит в «⋯» раньше «Все N ▾»; Ctrl+F выводит его в строку и ставит фокус', async () => {
    render(LibraryBar)
    await settle()
    expect(screen.queryByRole('button', { name: 'Найти фото по имени' })).toBeNull() // в закрытом «⋯»
    expect(document.querySelector('[data-more]')).not.toBeNull()
    await run('findPhotos')
    await settle()
    expect(inBar(field())).toBe(true)
    expect(document.activeElement).toBe(field())
  })

  // Полоса 440 px при группах по 100: помещаются четыре группы без «⋯»-запаса не хватает — уходит «size» (10), затем search (45).
  const guide = (g: string) => document.querySelector(`[data-bar="library"] [data-guide="${g}"]`)

  it('порядок в строке: ⌕ стоит перед «Все N ▾» (фильтром)', async () => {
    vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(0) // ширина неизвестна — показано всё
    render(LibraryBar)
    await settle()
    const s = guide('name-search')!
    const f = guide('mark-filter')!
    expect(s && f).toBeTruthy()
    expect(s.compareDocumentPosition(f) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('тесно: пустое поле уходит в «⋯» раньше фильтра, а открытое остаётся в строке и вытесняет фильтр', async () => {
    vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(440)
    render(LibraryBar)
    await settle()
    expect(guide('name-search')).toBeNull()
    expect(guide('mark-filter')).not.toBeNull()
    expect(document.querySelector('[data-guide-hold]')!.getAttribute('data-guide-hold')).toContain('name-search') // «⋯» мигает вместо ⌕
    await run('findPhotos')
    await settle()
    expect(guide('name-search')).not.toBeNull()
    expect(guide('mark-filter')).toBeNull()
  })

  it('непустое поле остаётся в строке, даже когда оно потеряло фокус', async () => {
    render(LibraryBar)
    await run('findPhotos')
    await settle()
    await type('798')
    await key(field(), 'Enter')
    await fireEvent.blur(field())
    await settle()
    expect(inBar(field())).toBe(true)
    expect(found()).toBe('Найдено 1 из 5')
  })
})
