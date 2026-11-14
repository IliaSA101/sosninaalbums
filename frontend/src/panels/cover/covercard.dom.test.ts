// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Карточка обложки по группам (спека 124 U6): порядок, схемы и зоны в «Фото фона», «Без плашки», «По сторонкам»,
// короткие подписи плиток в узкой карточке, нет «Удалить обложку», сворачивание и память, вставки 123 на месте.
const api = vi.hoisted(() => ({ SetCoverLayout: vi.fn(async () => undefined), SetCoverSpineBg: vi.fn(async () => undefined) }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return {
    ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])),
    CoverPresets: vi.fn(async () => []),
    MyColors: vi.fn(async () => []),
    ...api,
  }
})

const { default: InspectorSpread } = await import('../../editor/inspector/InspectorSpread.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { cardGroups, cardContext } = await import('../../base/prefs/cardgroups.svelte')
const { drag } = await import('../../base/drag/drag.svelte')

const SHEET = {
  id: 'cover',
  w: 680,
  h: 240,
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
}
const SCOPES = [
  { scope: 'left', label: 'Задняя' },
  { scope: 'spine', label: 'Корешок' },
  { scope: 'right', label: 'Лицевая' },
]
const cover = (o: Record<string, unknown> = {}, sheet: Record<string, unknown> = {}) => ({
  sheet: { ...SHEET, ...sheet },
  spec: { preset: 'custom', wrap: 17, steps: [] },
  preset: 'custom',
  spine: 26,
  n: 40,
  over: false,
  layout: 'both',
  spineBg: '',
  spineText: '',
  zones: [],
  drop: [],
  scopes: SCOPES,
  safe: [],
  markup: null,
  pxW: 7000,
  pxH: 2000,
  fix: '',
  missing: '',
  ...o,
})

function show(c = cover(), can: (f: string) => boolean = () => true, docked = true) {
  store.view = { ...store.view, rev: 2, settings: { pageW: 300, pageH: 300, dpi: 150 }, spreads: [], cover: c } as never
  store.goCover()
  vi.spyOn(prefs, 'can').mockImplementation(can as never)
  render(InspectorSpread, { context: new Map([cardContext(() => docked)]) })
}

const groups = () => Array.from(document.querySelectorAll('[data-group]')).map((g) => g.getAttribute('data-group'))
const head = (name: RegExp | string) => screen.getByRole('button', { name })

beforeEach(() => {
  localStorage.clear()
  cardGroups.reset()
})
afterEach(() => {
  cleanup()
  drag.active = false
  drag.payload = null
  store.onCover = false
  for (const f of Object.values(api)) f.mockClear()
  vi.restoreAllMocks()
})

describe('карточка обложки: порядок', () => {
  it('группы: Фото фона → Корешок → Фон → Подписи → Размеры; названия листа и «Авто | Ручная» нет', () => {
    show()
    expect(groups()).toEqual(['cover.layout', 'cover.spine', 'cover.bg', 'cover.caps', 'cover.size'])
    expect(document.querySelector('input.name')).toBeNull()
    expect(document.querySelector('[data-guide="mode-toggle"]')).toBeNull()
    expect(screen.getByRole('button', { name: /^Подписи обложки/ })).toBeTruthy()
  })

  it('предупреждения (вне типографии, «не найдена», «не по шаблону» + «Исправить») — над группами и без своей группы', () => {
    show(
      cover({
        over: true,
        spec: { preset: 'x', wrap: 17, steps: [{ upto: 57 }] },
        missing: 'Типография не найдена: old',
        fix: 'Не по шаблону',
      }),
    )
    const first = document.querySelector('[data-group]')!
    for (const t of [/Больше 57/, 'Типография не найдена: old', 'Не по шаблону']) {
      const w = screen.getByText(t)
      expect(w.classList.contains('warn')).toBe(true)
      expect(w.closest('[data-group]')).toBeNull()
      expect(w.compareDocumentPosition(first) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    }
    expect(screen.getByText('Исправить').closest('[data-group]')).toBeNull()
  })

  it('заголовок группы повторён в title — полным, если не влез', () => {
    show()
    const t = document.querySelector('[data-group="cover.size"] .ttl')!
    expect(t.getAttribute('title')).toBe('Размеры для типографии')
  })

  it('«Удалить обложку» в карточке нет', () => {
    show()
    expect(screen.queryByText('Удалить обложку')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Удалить обложку' })).toBeNull()
  })
})

describe('«Фото фона» (группа со схемами обложки)', () => {
  it('три плитки в порядке 061, текущая нажата, зоны фото — в слоте группы, а не в «Фоне»', () => {
    show()
    const tiles = Array.from(document.querySelectorAll('[data-group="cover.layout"] .tile'))
    expect(tiles.map((t) => t.getAttribute('aria-label'))).toEqual(['Лицевая', 'Одно фото через корешок', 'Задняя + корешок + лицевая'])
    expect(tiles.map((t) => t.getAttribute('aria-pressed'))).toEqual(['false', 'false', 'true'])
    const slot = document.querySelector('[data-group="cover.layout"] [data-slot="backdrop"]')!
    expect(Array.from(slot.querySelectorAll('[data-drop="backdrop"]')).map((z) => z.getAttribute('data-scope'))).toEqual([
      'left',
      'spine',
      'right',
    ])
    expect(document.querySelector('[data-group="cover.bg"] [data-drop]')).toBeNull()
    expect(document.querySelector('[data-group="cover.bg"] [data-slot]')).toBeNull()
  })

  it('имя группы — «Фото фона», слова «Раскладка обложки» нет; ключ памяти cover.layout прежний (0.26-127-6)', () => {
    show()
    const g = document.querySelector('[data-group="cover.layout"]')!
    expect(g.querySelector('button[aria-expanded]')?.textContent).toContain('Фото фона')
    expect(screen.queryByText('Раскладка обложки')).toBeNull()
    expect(document.body.innerHTML).not.toContain('Раскладка обложки')
    expect(screen.getByRole('group', { name: 'Схема фото фона' })).toBeTruthy()
  })

  it('щелчок по другой плитке — один SetCoverLayout; по выбранной — ничего', async () => {
    show()
    await fireEvent.click(screen.getByRole('button', { name: 'Лицевая' }))
    expect(api.SetCoverLayout).toHaveBeenCalledExactlyOnceWith('front')
    api.SetCoverLayout.mockClear()
    await fireEvent.click(screen.getByRole('button', { name: 'Задняя + корешок + лицевая' }))
    expect(api.SetCoverLayout).not.toHaveBeenCalled()
  })

  it('широкая карточка: под схемой полные подписи; узкая: короткие слова, полная — в title и aria-label', () => {
    show()
    const text = () => Array.from(document.querySelectorAll('.tile span')).map((s) => s.textContent)
    expect(text()).toEqual(['Лицевая', 'Одно фото через корешок', 'Задняя + корешок + лицевая'])
    cleanup()
    show(cover(), () => true, false)
    expect(text()).toEqual(['Лицевая', 'Через корешок', 'Три части'])
    const wrap = screen.getByRole('button', { name: 'Одно фото через корешок' })
    expect(wrap.getAttribute('title')).toBe('Одно фото через корешок')
    expect(screen.getByRole('button', { name: 'Задняя + корешок + лицевая' }).getAttribute('title')).toBe('Задняя + корешок + лицевая')
  })

  it('значение свёрнутой группы — подпись раскладки (в узкой короткая)', async () => {
    show(cover({ layout: 'wrap' }))
    await fireEvent.click(head(/^Фото фона/))
    expect(document.querySelector('[data-group="cover.layout"] .sum')?.textContent).toBe('Одно фото через корешок')
    cleanup()
    show(cover({ layout: 'wrap' }), () => true, false)
    await fireEvent.click(head(/^Фото фона/)) // у плавающей память своя: свёрнута только теперь
    expect(document.querySelector('[data-group="cover.layout"] .sum')?.textContent).toBe('Через корешок')
  })

  it('без «Фото фона» на уровне плитки есть, зон нет', () => {
    show(cover(), (f) => f !== 'bgPhoto')
    expect(document.querySelectorAll('[data-group="cover.layout"] .tile')).toHaveLength(3)
    expect(document.querySelector('[data-slot="backdrop"]')).toBeNull()
  })

  it('бросок фото: свёрнутое «Фото фона» раскрывается на время переноса без записи; «Фон» — нет', () => {
    cardGroups.toggle('d', 'cover.layout')
    cardGroups.toggle('d', 'cover.bg')
    show()
    expect(head(/^Фото фона/).getAttribute('aria-expanded')).toBe('false')
    drag.active = true
    drag.payload = { kind: 'photos', ids: ['p1'] }
    flushSync()
    expect(head(/^Фото фона/).getAttribute('aria-expanded')).toBe('true')
    expect(head(/^Фон/).getAttribute('aria-expanded')).toBe('false')
    expect(cardGroups.isOpen('d', 'cover.layout')).toBe(false)
  })
})

describe('«Корешок»', () => {
  it('«Без плашки» вместо «нет»: серая без плашки; с плашкой — один SetCoverSpineBg("")', async () => {
    show()
    const btn = () => screen.getByRole('button', { name: 'Без плашки' }) as HTMLButtonElement
    expect(btn().textContent).toBe('Без плашки')
    expect(screen.queryByText('нет')).toBeNull()
    expect(btn().disabled).toBe(true)
    cleanup()
    show(cover({ spineBg: '#4a4643' }))
    expect(btn().disabled).toBe(false)
    await fireEvent.click(btn())
    expect(api.SetCoverSpineBg).toHaveBeenCalledExactlyOnceWith('')
  })

  it('надпись и «Плашка» в группе; значение свёрнутой — «надпись · плашка» / «без плашки»', async () => {
    show(cover({ spineText: 't1', spineBg: '#4a4643' }))
    const g = document.querySelector('[data-group="cover.spine"]')!
    expect(g.querySelector('[data-guide="cover-spine"]')).toBeTruthy()
    expect(screen.getByText('Выделить надпись')).toBeTruthy()
    expect(screen.getByText('Плашка')).toBeTruthy()
    await fireEvent.click(head(/^Корешок/))
    expect(g.querySelector('.sum')?.textContent).toBe('надпись · плашка')
    cleanup()
    show() // группа уже свёрнута и помнится
    expect(document.querySelector('[data-group="cover.spine"] .sum')?.textContent).toBe('без плашки')
  })
})

describe('«Фон» и «Подписи» обложки', () => {
  it('«По сторонкам» вместо «По страницам»; сторонки — подписи Go', () => {
    show(cover(), () => true)
    expect(screen.getByText('По сторонкам')).toBeTruthy()
    expect(screen.queryByText('По страницам')).toBeNull()
  })

  it('значение свёрнутого «Фона» — цвет и узор; фото фона сюда не пишется', async () => {
    show(cover({}, { bgSet: ['#4f7a5a', '#4f7a5a'], backdrops: [{ photo: 'p1', scope: 'right', fade: 0, dpi: 0 }] }))
    await fireEvent.click(head(/^Фон/))
    expect(document.querySelector('[data-group="cover.bg"] .sum')?.textContent).toBe('свой цвет')
  })

  it('на уровне без цвета и фото фона «Фона» нет, без подписей нет «Подписей»', () => {
    show(cover(), (f) => f === 'x')
    expect(groups()).toEqual(['cover.layout', 'cover.spine', 'cover.size'])
  })

  it('«Свой» уровень: только «Фото фона» — без лежащего узора пустой «Фон» не рисуется, с узором есть', () => {
    show(cover(), (f) => f === 'bgPhoto')
    expect(groups()).not.toContain('cover.bg')
    cleanup()
    show(cover({}, { patterns: [{ scope: 'spread', pat: { id: 'dots', fade: 0, scale: 1 } }] }), (f) => f === 'bgPhoto')
    expect(groups()).toContain('cover.bg')
  })
})

describe('«Размеры для типографии»: вставки 123 на месте', () => {
  it('типография, сторонка, итог, строка «Файл обложки, px», разметка — внутри группы', () => {
    show()
    const g = document.querySelector('[data-group="cover.size"]')!
    expect(g.querySelector('#cover-preset[data-guide="cover-print"]')).toBeTruthy()
    expect(g.querySelector('[data-guide="cover-px"]')).toBeTruthy()
    expect(g.textContent).toContain('Итог:')
    expect(g.textContent).toContain('Сторонка Ш × В, мм')
    expect(g.querySelector('[data-guide="cover-markup"]')).toBeTruthy()
  })

  it('значение свёрнутой — типография и корешок', async () => {
    show()
    await fireEvent.click(head(/^Размеры для типографии/))
    expect(document.querySelector('[data-group="cover.size"] .sum')?.textContent).toBe('Свои размеры · Корешок 26 мм при 40 разворотах')
  })
})

describe('сворачивание и память', () => {
  it('щелчок сворачивает (inert, тело не размонтировано) и запоминает; панель и плавающая — раздельно', async () => {
    show()
    const size = () => document.querySelector('[data-group="cover.size"]')!
    expect(head(/^Размеры для типографии/).getAttribute('aria-expanded')).toBe('true')
    await fireEvent.click(head(/^Размеры для типографии/))
    expect(head(/^Размеры для типографии/).getAttribute('aria-expanded')).toBe('false')
    expect(size().querySelector('.body')!.hasAttribute('inert')).toBe(true)
    expect(size().querySelector('#cover-preset')).toBeTruthy()
    expect(JSON.parse(localStorage.getItem('cardGroups')!)).toEqual({ d: ['cover.size'], f: [] })
    cleanup()
    show() // заново: свёрнутое помнится
    expect(head(/^Размеры для типографии/).getAttribute('aria-expanded')).toBe('false')
    cleanup()
    show(cover(), () => true, false)
    expect(head(/^Размеры для типографии/).getAttribute('aria-expanded')).toBe('true')
  })

  it('повторный щелчок раскрывает обратно и снимает запись', async () => {
    show()
    await fireEvent.click(head(/^Корешок/))
    await fireEvent.click(head(/^Корешок/))
    expect(head(/^Корешок/).getAttribute('aria-expanded')).toBe('true')
    expect(JSON.parse(localStorage.getItem('cardGroups')!)).toEqual({ d: [], f: [] })
  })
})
