// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Карточка листа по группам (спека 124 U5): порядок «Виньетка» → «Фон» → «Подписи» для всех листов, «×» вместо серой
// «Как в альбоме», «⋯» у «Виньетки», бросок фото на свёрнутый «Фон». Цвет и окна цвета — inspectorspread.color.dom.test.ts.
const setBackground = vi.hoisted(() => vi.fn(async () => undefined))
const clearVignette = vi.hoisted(() => vi.fn(async () => undefined))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    MyColors: vi.fn(async () => []),
    SetBackground: setBackground,
    ClearVignette: clearVignette,
  }
})

const { default: InspectorSpread } = await import('./InspectorSpread.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { cardGroups, cardContext } = await import('../../base/prefs/cardgroups.svelte')
const { drag } = await import('../../base/drag/drag.svelte')
const { vigOwn } = await import('../../panels/vignette/vigown.svelte')

type SpreadPatch = Record<string, unknown>
const spread = (patch: SpreadPatch = {}) => ({
  id: 's1',
  name: '',
  grid: false,
  photos: ['p1'],
  places: [],
  frames: [],
  rects: [],
  bg: ['#ffffff', '#ffffff'],
  bgSet: ['', ''],
  backdrops: [],
  patterns: [],
  captions: [],
  ...patch,
})
const VIG = { tpl: 'classic', teachers: 1, pupils: 23, ratio: '3:4', group: false, ttext: '', shape: '', round: 0 }

function show(patch: SpreadPatch = {}, can: (f: string) => boolean = () => true, docked = true) {
  store.view = {
    ...store.view,
    rev: 5,
    settings: { pageW: 300, pageH: 300, bg: '' },
    spreads: [spread(patch)],
  } as never
  vi.spyOn(prefs, 'can').mockImplementation(can as never)
  render(InspectorSpread, { context: new Map([cardContext(() => docked)]) })
}

const groups = () => Array.from(document.querySelectorAll('[data-group]')).map((g) => g.getAttribute('data-group'))
const head = (name: RegExp | string) => screen.getByRole('button', { name })

beforeEach(() => {
  localStorage.clear()
  cardGroups.reset()
  vigOwn.close()
})
afterEach(() => {
  cleanup()
  drag.active = false
  drag.payload = null
  setBackground.mockClear()
  clearVignette.mockClear()
  vi.restoreAllMocks()
})

describe('карточка листа: порядок групп', () => {
  it('обычный разворот: Название → Фон → Подписи → Авто | Ручная', () => {
    show()
    expect(groups()).toEqual(['spread.bg', 'spread.caps'])
    const order = Array.from(document.querySelectorAll('input.name, [data-group], [data-guide="mode-toggle"]')).map((e) =>
      e.matches('[data-group]') ? e.getAttribute('data-group') : e.classList.contains('name') ? 'name' : e.getAttribute('data-guide'),
    )
    expect(order).toEqual(['name', 'spread.bg', 'spread.caps', 'mode-toggle'])
  })

  it('виньетка: Виньетка → Фон → Подписи, «Авто | Ручная» нет, «Сделать виньеткой» нет', () => {
    show({ kind: 'vignette', vig: VIG })
    expect(groups()).toEqual(['spread.vig', 'spread.bg', 'spread.caps'])
    expect(document.querySelector('[data-guide="mode-toggle"]')).toBeNull()
    expect(document.querySelector('[data-guide="vig-make"]')).toBeNull()
  })

  it('обычный разворот: «Сделать виньеткой» в конце, группы «Виньетка» нет', () => {
    show()
    expect(document.querySelector('[data-guide="vig-make"]')).toBeTruthy()
    expect(document.querySelector('[data-group="spread.vig"]')).toBeNull()
  })

  it('пустой разворот без фото и подписей: группы «Подписи» нет; один элемент — есть', () => {
    show({ photos: [] })
    expect(groups()).toEqual(['spread.bg'])
    cleanup()
    show({ photos: ['p1'] })
    expect(groups()).toContain('spread.caps')
  })
})

describe('карточка листа: уровни', () => {
  it('без цвета и фото фона группы «Фон» нет (нет пустого заголовка)', () => {
    show({}, (f) => f === 'captions')
    expect(groups()).toEqual(['spread.caps'])
  })

  it('без подписей нет группы «Подписи»; «Виньетка» есть на любом уровне', () => {
    show({ kind: 'vignette', vig: VIG }, (f) => f === 'bgColor')
    expect(groups()).toEqual(['spread.vig', 'spread.bg'])
  })

  it('только фото фона (без цвета): в «Фоне» нет строки цвета и «×»', () => {
    show({}, (f) => f === 'bgPhoto')
    expect(groups()).toEqual(['spread.bg'])
    expect(document.querySelector('.reset')).toBeNull()
  })
})

describe('«Фон»: «×» вместо серой «Как в альбоме»', () => {
  const reset = () => screen.getByRole('button', { name: 'Весь разворот: как в альбоме' }) as HTMLButtonElement

  it('серой кнопки «Как в альбоме» в теле нет, «×» есть у строки «Весь разворот»', () => {
    show()
    expect(screen.queryByRole('button', { name: 'Как в альбоме' })).toBeNull()
    expect(reset().textContent).toBe('×')
    expect(reset().title).toBe('Как в альбоме')
  })

  it('свой цвет нет — «×» серый; свой есть — один SetBackground(…, "") на щелчок', async () => {
    show()
    expect(reset().disabled).toBe(true)
    cleanup()
    show({ bg: ['#4f7a5a', '#4f7a5a'], bgSet: ['#4f7a5a', '#4f7a5a'] })
    expect(reset().disabled).toBe(false)
    await fireEvent.click(reset())
    expect(setBackground).toHaveBeenCalledExactlyOnceWith('s1', 'spread', '')
  })

  it('на листе виньетки «×» тоже есть («Весь лист»)', () => {
    store.view = { ...store.view, settings: { sheet: true, pageW: 300, pageH: 300, bg: '' } } as never
    show({ kind: 'vignette', vig: VIG, bgSet: ['#4f7a5a', '#4f7a5a'], bg: ['#4f7a5a', '#4f7a5a'] })
    expect(screen.getByRole('button', { name: /как в альбоме$/ }).textContent).toBe('×')
  })

  it('по страницам: «×» у каждой страницы и ни одного у «всего разворота»', async () => {
    show({ bg: ['#4f7a5a', '#ffffff'], bgSet: ['#4f7a5a', ''] })
    expect(screen.getAllByRole('button', { name: /как в альбоме$/ })).toHaveLength(2) // split раскрыт: цвета разные
    expect(screen.queryByRole('button', { name: 'Весь разворот: как в альбоме' })).toBeNull()
  })
})

describe('«Фон»: свёрнутая группа', () => {
  it('значение свёрнутой: как в альбоме / свой цвет / узор / фото фона', async () => {
    show()
    await fireEvent.click(head(/^Фон/))
    expect(screen.getByText('как в альбоме')).toBeTruthy()
    cleanup()
    show({ bgSet: ['#4f7a5a', '#4f7a5a'] })
    expect(screen.getByText('свой цвет')).toBeTruthy()
    cleanup()
    show({ patterns: [{ scope: 'spread', pat: { id: 'dots', fade: 0, scale: 1 } }] })
    expect(screen.getByText('узор')).toBeTruthy()
    cleanup()
    show({ backdrops: [{ photo: 'p1', scope: 'spread', fade: 0, dpi: 0 }] })
    expect(screen.getByText('+ фото фона')).toBeTruthy()
  })

  it('фон альбома (album: true) своим не считается', async () => {
    show({ backdrops: [{ photo: 'p1', scope: 'spread', fade: 0, dpi: 0, album: true }] })
    await fireEvent.click(head(/^Фон/))
    expect(screen.getByText('как в альбоме')).toBeTruthy()
  })

  it('бросок фото на свёрнутый «Фон»: раскрывается на время переноса, в память не пишется', () => {
    cardGroups.toggle('d', 'spread.bg')
    show()
    expect(head(/^Фон/).getAttribute('aria-expanded')).toBe('false')
    drag.active = true
    drag.payload = { kind: 'photos', ids: ['p1'] }
    flushSync()
    expect(head(/^Фон/).getAttribute('aria-expanded')).toBe('true')
    expect(cardGroups.isOpen('d', 'spread.bg')).toBe(false)
    drag.active = false
    drag.payload = null
    flushSync()
    expect(head(/^Фон/).getAttribute('aria-expanded')).toBe('false')
  })

  it('перенос разворота «Фон» не раскрывает; «Подписи» при переносе фото тоже', () => {
    cardGroups.toggle('d', 'spread.bg')
    cardGroups.toggle('d', 'spread.caps')
    show()
    drag.active = true
    drag.payload = { kind: 'spread', id: 's1', index: 0 }
    flushSync()
    expect(head(/^Фон/).getAttribute('aria-expanded')).toBe('false')
    drag.payload = { kind: 'photos', ids: ['p1'] }
    flushSync()
    expect(head(/^Подписи/).getAttribute('aria-expanded')).toBe('false')
  })

  it('якоря F1 spread-bg и sheet-caps есть и в свёрнутых телах', async () => {
    cardGroups.toggle('d', 'spread.bg')
    cardGroups.toggle('d', 'spread.caps')
    show()
    expect(document.querySelector('[data-guide="spread-bg"]')).toBeTruthy()
    expect(document.querySelector('[data-guide="sheet-caps"]')).toBeTruthy()
    expect(document.querySelector('[data-group="spread.bg"] .body')!.hasAttribute('inert')).toBe(true)
  })
})

describe('«Подписи»: значение свёрнутой', () => {
  it('нет подписей — «нет»; свои подписи — «свои»; общий стиль с шаблоном — «к каждому фото»', async () => {
    show()
    await fireEvent.click(head(/^Подписи/))
    expect(screen.getByText('нет')).toBeTruthy()
    cleanup()
    show({ captions: [{ id: 'c1' }] })
    expect(screen.getByText('свои')).toBeTruthy()
    cleanup()
    show({ caps: { text: '{name}' } })
    expect(screen.getByText('к каждому фото')).toBeTruthy()
  })
})

describe('«Виньетка»: «⋯»', () => {
  const more = () => screen.getByRole('button', { name: 'Ещё: Виньетка' })
  const open = async () => {
    await fireEvent.click(more())
    flushSync()
  }

  it('«⋯» несёт запасной якорь vig-save; кнопок «Сохранить как шаблон…» и «Обычный разворот» в теле нет', () => {
    show({ kind: 'vignette', vig: VIG })
    expect(more().getAttribute('data-guide-hold')).toBe('vig-save')
    expect(screen.queryByRole('button', { name: /Сохранить как шаблон/ })).toBeNull()
    expect(screen.queryByRole('button', { name: /^Обычный/ })).toBeNull()
  })

  it('пункты меню — «Сохранить как шаблон…» и «Обычный разворот»; без «Свои шаблоны» — один', async () => {
    show({ kind: 'vignette', vig: VIG })
    await open()
    expect(document.querySelector('[data-item="vig-save"]')?.textContent).toContain('Сохранить как шаблон…')
    expect(document.querySelector('[data-item="vig-clear"]')?.textContent).toContain('Обычный разворот')
    cleanup()
    show({ kind: 'vignette', vig: VIG }, (f) => f !== 'templates')
    await open()
    expect(document.querySelector('[data-item="vig-save"]')).toBeNull()
    expect(document.querySelector('[data-item="vig-clear"]')).toBeTruthy()
  })

  it('«Сохранить как шаблон…» открывает окно имени для этого листа', async () => {
    show({ kind: 'vignette', vig: VIG })
    await open()
    await fireEvent.click(document.querySelector('[data-item="vig-save"]')!)
    expect(vigOwn.ask).toEqual({ kind: 'save', spread: 's1' })
  })

  it('«Обычный разворот» — один вызов ClearVignette', async () => {
    show({ kind: 'vignette', vig: VIG })
    await open()
    await fireEvent.click(document.querySelector('[data-item="vig-clear"]')!)
    expect(clearVignette).toHaveBeenCalledExactlyOnceWith('s1')
  })

  it('свёрнутая «Виньетка» пишет числа листа; «⋯» остаётся', async () => {
    show({ kind: 'vignette', vig: VIG })
    await fireEvent.click(head(/^Виньетка/))
    expect(document.querySelector('[data-group="spread.vig"] .head .sum')?.textContent).toBe('1 учитель · 23 ученика')
    expect(more()).toBeTruthy()
  })
})

describe('пары кнопок в «Фоне»', () => {
  const files = () => Array.from(document.querySelectorAll('.files'))

  it('панель деталей: «Галерея · Свой файл» и «Копировать оформление · Готовые развороты» строкой по две', () => {
    show()
    expect(files()).toHaveLength(2)
    for (const f of files()) {
      expect(f.classList.contains('narrow')).toBe(false)
      expect(f.querySelectorAll('button')).toHaveLength(2)
    }
    expect(document.querySelector('.files [data-guide="copy-decor"]')).toBeTruthy()
    expect(document.querySelector('.files [data-guide="spread-tpl"]')).toBeTruthy()
    expect(document.querySelector('.files [data-guide="bg-gallery"]')).toBeTruthy()
  })

  it('плавающая карточка (узкая): те же пары столбцом', () => {
    show({}, () => true, false)
    for (const f of files()) expect(f.classList.contains('narrow')).toBe(true)
  })

  it('без «Фонов»: «Свой файл…» один, копирования нет', () => {
    show({}, (f) => f !== 'decor')
    expect(files()).toHaveLength(1)
    expect(files()[0].classList.contains('one')).toBe(true)
    expect(document.querySelector('[data-guide="copy-decor"]')).toBeNull()
  })
})
