// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Ряд отбора «Набор · Вид · Стиль» окон оформления (133-11): скрыт без вариантов, выбор держится после нового открытия и сбрасывается
// сменой вида окна, подпись без приставки, строка «Новые», «Показать весь набор» в просмотре. Go подменён.
const api = vi.hoisted(() => ({ DecorCatalog: vi.fn(), LibraryPacks: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))

const { default: DecorDialog } = await import('../decor/DecorDialog.svelte')
const { decor } = await import('../decor/decor.svelte')
const { store } = await import('../../base/core/store.svelte')
const { modes } = await import('../../base/prefs/view.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { useCatalog, dropCatalog } = await import('../../base/keys/keymap.fixture')
const { peek } = await import('./peek.svelte')
const { NO_SEL } = await import('./browse')

type Item = Record<string, unknown>
const item = (id: string, over: Item = {}) => ({
  id,
  theme: 'a',
  name: id,
  url: `/media/lib/t/${id}?v=1`,
  peek: `/media/lib/p/${id}?v=1`,
  own: false,
  aspect: 0,
  w: 0,
  h: 0,
  ...over,
})
const school = (id: string, over: Item = {}) => item(id, { pack: 'Школа', packId: 'school', theme: 'a', ...over })
const wed = (id: string, over: Item = {}) => item(id, { pack: 'Свадьба', packId: 'wed', theme: 'b', ...over })
const ITEMS = [
  wed('w1', { name: 'Кольца' }),
  wed('wc', { name: 'Золотой уголок', form: 'corner' }),
  school('s1', { name: 'Значки: Колокольчик', style: 'Значки' }),
  school('s2', { name: 'Значки: Ромашка', style: 'Значки' }),
  school('s3', { name: 'Значки: Звонок', style: 'Значки' }),
  school('s4', { name: 'Рисунки: Рюкзак', style: 'Рисунки' }),
  school('s5', { name: 'Рисунки: Мел', style: 'Рисунки' }),
  school('s6', { name: 'Рисунки: Глобус', style: 'Рисунки' }),
  school('sc', { name: 'Уголок школьный', form: 'corner' }),
  item('sa:x', { name: 'Звезда', packId: 'sa:builtin', url: '/media/gen/art?k=x&v=1', peek: undefined }),
  item('my:a', { name: 'Логотип', own: true, theme: 'mine' }),
]
const THEMES = [
  { id: 'new', name: 'Новые', count: 2 },
  { id: 'all', name: 'Все', count: ITEMS.length },
  { id: 'a', name: 'Школа', count: 8 },
  { id: 'b', name: 'Свадьба', count: 2 },
  { id: 'mine', name: 'Мои', count: 1 },
]
const cat = (kind: string, items: Item[] = ITEMS, themes: Item[] = THEMES) => ({
  kind,
  themes,
  items,
  recent: [],
  new: ['wc', 'w1'],
  readOnly: false,
})
const sheet = (id: string) => ({
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
  n: 0,
})
type Kind = 'bg' | 'sticker' | 'frame' | 'spread'
const open = async (kind: Kind, c = cat(kind)) => {
  api.DecorCatalog.mockResolvedValue(c)
  decor.show(kind, 's1', 'spread')
  await vi.waitFor(() => expect(decor.cat?.kind).toBe(kind))
  flushSync()
}
const reopen = async (kind: Kind) => {
  decor.close()
  flushSync()
  await open(kind)
}

const row = () => document.querySelector<HTMLElement>('[aria-label="Отбор плиток"]')
const packSel = () => document.querySelector<HTMLSelectElement>('#decor-pack')
const seg = (label: string) => document.querySelector<HTMLElement>(`[aria-label="${label}"]`)
const segBtn = (label: string, text: string) =>
  Array.from(seg(label)?.querySelectorAll('button') ?? []).find((b) => b.textContent === text)!
const tileIds = () => Array.from(document.querySelectorAll<HTMLElement>('.tiles [data-decor-id]')).map((e) => e.dataset.decorId)
const labels = () => Array.from(document.querySelectorAll('.line1 .name')).map((e) => e.textContent)
const side = (name: string) =>
  Array.from(document.querySelectorAll<HTMLElement>('[aria-label="Темы"] button')).find((b) => b.textContent?.startsWith(name))!
const sideText = (name: string) => side(name).textContent
async function choose(el: HTMLSelectElement, value: string) {
  el.value = value
  await fireEvent.change(el)
  flushSync()
}

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.LibraryPacks.mockResolvedValue({ packs: [], net: {}, busy: false, readOnly: false })
  useCatalog()
  localStorage.clear()
  modes.decorTile = {}
  prefs.level = 'simple'
  prefs.custom = null
  decor.close()
  decor.cat = null
  decor.theme = ''
  decor.filters = { ...NO_SEL }
  decor.busy = false
  peek.close()
  store.view = { rev: 1, folders: [], settings: { pageW: 300, pageH: 300 }, spreads: [sheet('s1')] } as never
  vi.spyOn(decor, 'pick').mockResolvedValue(undefined)
  render(DecorDialog)
})
afterEach(() => {
  cleanup()
  decor.close()
  dropCatalog()
  vi.restoreAllMocks()
})

describe('133-11: ряд отбора', () => {
  it('нечего выбирать — ряда нет', async () => {
    // один набор, один вид, стилей нет
    await open('sticker', cat('sticker', [school('s1'), school('s2'), item('my:a', { own: true })]))
    expect(tileIds()).toHaveLength(3)
    expect(row()).toBeNull()
  })

  it('есть что выбирать — «Набор», «Вид», «Стиль» с «Все» первым', async () => {
    await open('sticker')
    decor.setTheme('all')
    flushSync()
    expect(row()).not.toBeNull()
    expect(Array.from(packSel()!.options).map((o) => o.textContent)).toEqual(['Все наборы', 'Свадьба', 'Школа', 'Встроенные'])
    expect(seg('Вид стикеров')!.textContent).toBe('ВсеСтикерыУголки')
    expect(seg('Стиль стикеров')!.textContent).toBe('ВсеЗначкиРисунки')
  })

  it('подпись и её элемент лежат в одной группе (переносятся вместе)', async () => {
    await open('sticker')
    decor.setTheme('all')
    flushSync()
    const grp = (el: Element | null) => el!.closest('.grp') as HTMLElement
    const lbl = (text: string) => Array.from(row()!.querySelectorAll('.lbl')).find((e) => e.textContent === text)!
    expect(grp(lbl('Набор'))).toBe(grp(packSel()))
    expect(grp(lbl('Вид'))).toBe(grp(seg('Вид стикеров')))
    expect(grp(lbl('Стиль'))).toBe(grp(seg('Стиль стикеров')))
    expect(new Set([grp(lbl('Набор')), grp(lbl('Вид')), grp(lbl('Стиль'))]).size).toBe(3)
    expect(grp(lbl('Набор')).parentElement).toBe(row())
  })

  it('в узкой теме остаются только нужные элементы: в «Мои» ряда нет, в «Свадьбе» нет «Стиля»', async () => {
    await open('sticker')
    decor.setTheme('mine')
    flushSync()
    expect(row()).toBeNull()
    decor.setTheme('b')
    flushSync()
    expect(packSel()).toBeNull() // в теме один набор
    expect(seg('Вид стикеров')).not.toBeNull()
    expect(seg('Стиль стикеров')).toBeNull()
  })

  it('набор отбирает плитки и числа слева; «Все наборы» возвращает', async () => {
    await open('sticker')
    decor.setTheme('all')
    flushSync()
    expect(tileIds()).toHaveLength(ITEMS.length)
    await choose(packSel()!, 'wed')
    expect(tileIds()).toEqual(['w1', 'wc'])
    expect(sideText('Все')).toBe('Все2')
    expect(sideText('Школа')).toBe('Школа8') // в теме набора «Свадьба» нет, а выбранного там нет — «Все»
    expect(sideText('Мои')).toBe('Мои1')
    await choose(packSel()!, '')
    expect(tileIds()).toHaveLength(ITEMS.length)
  })

  it('щелчок по строке слева: число в ней равно числу плиток после щелчка (при наборе и при «Уголках»)', async () => {
    await open('sticker')
    decor.setTheme('all')
    flushSync()
    const num = (name: string) => Number(side(name).querySelector('em')?.textContent)
    for (const set of [() => choose(packSel()!, 'wed'), () => fireEvent.click(segBtn('Вид стикеров', 'Уголки'))]) {
      decor.filters = { ...NO_SEL }
      decor.setTheme('all')
      flushSync()
      await set()
      flushSync()
      for (const name of ['Новые', 'Все', 'Школа', 'Свадьба', 'Мои']) {
        const n = num(name)
        await fireEvent.click(side(name))
        flushSync()
        expect(tileIds().length, `${name} ${JSON.stringify(decor.filters)}`).toBe(n)
      }
    }
  })

  it('«Стиль» — подпись без приставки, при «Все» полное имя', async () => {
    await open('sticker')
    decor.setTheme('a')
    flushSync()
    expect(labels()).toContain('Значки: Колокольчик')
    await fireEvent.click(segBtn('Стиль стикеров', 'Значки'))
    flushSync()
    expect(tileIds()).toEqual(['s1', 's2', 's3'])
    expect(labels()).toEqual(['Колокольчик', 'Ромашка', 'Звонок'])
    expect(document.querySelector<HTMLElement>('[data-decor-id="s1"]')!.title).toContain('Значки: Колокольчик') // полное имя в подсказке
    await fireEvent.click(segBtn('Стиль стикеров', 'Все'))
    flushSync()
    expect(labels()).toContain('Значки: Колокольчик')
  })

  it('строка «Новые» с подсказкой; плитки — по cat.new в порядке каталога', async () => {
    await open('sticker')
    decor.setTheme('new')
    flushSync()
    expect(side('Новые').title).toContain('за последние 30 дней')
    expect(tileIds()).toEqual(['wc', 'w1'])
    expect(sideText('Новые')).toBe('Новые2')
  })
})

describe('133-11: память отбора', () => {
  it('«Уголки» держатся после нового открытия окна того же вида', async () => {
    await open('sticker')
    decor.setTheme('all')
    flushSync()
    await fireEvent.click(segBtn('Вид стикеров', 'Уголки'))
    flushSync()
    expect(tileIds()).toEqual(['wc', 'sc'])
    await reopen('sticker')
    expect(decor.filters.form).toBe('corner')
    expect(segBtn('Вид стикеров', 'Уголки').getAttribute('aria-pressed')).toBe('true')
    expect(tileIds().every((id) => ['wc', 'sc'].includes(id!))).toBe(true)
  })

  it('смена вида окна — всё «Все»; обратно тоже', async () => {
    await open('sticker')
    decor.setTheme('all')
    decor.setFilter('form', 'corner')
    decor.setFilter('pack', 'school')
    await reopen('bg')
    expect(decor.filters).toEqual(NO_SEL)
    await reopen('sticker')
    expect(decor.filters).toEqual(NO_SEL)
  })

  it('выбранного набора в теме нет — показ «Все», выбор в памяти; вернулись в тему — снова действует', async () => {
    await open('sticker')
    decor.setTheme('all')
    flushSync()
    await choose(packSel()!, 'wed')
    decor.setTheme('mine')
    flushSync()
    expect(tileIds()).toEqual(['my:a'])
    expect(decor.filters.pack).toBe('wed')
    decor.setTheme('all')
    flushSync()
    expect(tileIds()).toEqual(['w1', 'wc'])
  })
})

describe('133-11: «Показать весь набор»', () => {
  const rclick = (id: string) => {
    document
      .querySelector<HTMLElement>(`[data-decor-id="${id}"]`)!
      .dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, button: 2 }))
    flushSync()
  }
  const showSet = () =>
    Array.from(document.querySelectorAll<HTMLButtonElement>('.cap button')).find((b) => b.textContent === 'Показать весь набор')

  it('у предмета набора кнопка рядом с «Поставить»: тема «Все», отбор по набору, просмотр закрыт, плитка в фокусе', async () => {
    await open('sticker')
    decor.setTheme('b')
    decor.setFilter('form', 'corner')
    flushSync()
    rclick('wc')
    const btn = showSet()!
    expect(btn).toBeDefined()
    expect(btn.title).toBe('Показать весь набор «Свадьба»')
    expect(btn.nextElementSibling?.textContent).toBe('Поставить')
    await fireEvent.click(btn)
    flushSync()
    expect(peek.id).toBe('')
    expect(decor.theme).toBe('all')
    expect(decor.filters).toEqual({ pack: 'wed', form: '', style: '' }) // вид и стиль — «Все»
    expect(tileIds()).toEqual(['w1', 'wc'])
    expect(packSel()!.value).toBe('wed')
    await vi.waitFor(() => expect(document.activeElement).toBe(document.querySelector('[data-decor-id="wc"]')))
    expect(decor.pick).not.toHaveBeenCalled()
  })

  it('у встроенного и своего кнопки нет', async () => {
    await open('sticker')
    decor.setTheme('all')
    flushSync()
    rclick('sa:x')
    expect(showSet()).toBeUndefined()
    expect(document.querySelector('.cap .primary')).not.toBeNull()
    document.querySelector('.cap .x')!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    flushSync()
    rclick('my:a')
    expect(showSet()).toBeUndefined()
  })

  it('Enter на кнопке — её собственный щелчок, плитку не ставит', async () => {
    await open('sticker')
    decor.setTheme('a')
    flushSync()
    rclick('s1')
    const btn = showSet()!
    btn.focus()
    btn.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
    flushSync()
    expect(decor.pick).not.toHaveBeenCalled()
  })
})

describe('прокрутка плиток', () => {
  let siv: ReturnType<typeof vi.fn>
  beforeEach(() => {
    siv = vi.fn()
    Object.defineProperty(HTMLElement.prototype, 'scrollIntoView', { value: siv, configurable: true, writable: true })
    vi.spyOn(decor, 'standing').mockReturnValue('s2')
  })
  afterEach(() => {
    delete (HTMLElement.prototype as unknown as Record<string, unknown>).scrollIntoView
  })
  const tiles = () => document.querySelector<HTMLElement>('.tiles')!
  const tick = () => new Promise<void>((r) => queueMicrotask(r))
  const shownTo = (el: HTMLElement) => siv.mock.contexts.includes(el)

  it('открытие и повторное открытие держат стоящую плитку в виду (seenIf)', async () => {
    await open('sticker')
    decor.setTheme('a')
    flushSync()
    expect(shownTo(document.querySelector('.tile.current')!)).toBe(true)
    siv.mockClear()
    await reopen('sticker')
    expect(shownTo(document.querySelector('.tile.current')!)).toBe(true)
  })

  it('перечитывание каталога (✎, ×, library:changed) прокрутку не трогает', async () => {
    await open('sticker')
    decor.setTheme('a')
    flushSync()
    tiles().scrollTop = 150
    await decor.refresh()
    flushSync()
    await tick()
    expect(tiles().scrollTop).toBe(150)
  })

  it('смена темы и отбора: к стоящей плитке, если она есть, иначе в начало', async () => {
    await open('sticker')
    decor.setTheme('a')
    flushSync()
    tiles().scrollTop = 150
    decor.setTheme('b') // стоящей (s2) здесь нет
    flushSync()
    await tick()
    expect(tiles().scrollTop).toBe(0)
    tiles().scrollTop = 150
    siv.mockClear()
    decor.setTheme('a')
    flushSync()
    await tick()
    expect(shownTo(document.querySelector('.tile.current')!)).toBe(true)
    expect(tiles().scrollTop).toBe(150)
  })
})
