// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte'

// Карточка «Фото» по группам (спека 124 U3): порядок групп и якорей F1, страж владельца 2026-10-18 («Цвет | Ч/б» сразу
// за «Повернуть · Зеркало»), свёрнутая группа держит тело и пишет значение, уровни, узкая карточка.
const api = vi.hoisted(() => ({ TurnPhotos: vi.fn(), Person: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), ...api }
})

const { default: InspectorFrame } = await import('./InspectorFrame.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { unfoldFor } = await import('../../shell/guide/spotstate.svelte')
const { cardGroups, cardContext } = await import('../../base/prefs/cardgroups.svelte')
const { LEVELS } = await import('../../base/prefs/levels')
const { features } = await import('../../base/prefs/levelcustom')
type FeatureId = import('../../base/prefs/levels').FeatureId

const GROUPS = ['photo.frame', 'photo.tone', 'photo.shape', 'photo.name', 'photo.place', 'photo.about']
const docked = { context: new Map([cardContext(() => true)]) }
const floating = { context: new Map([cardContext(() => false)]) }

function setup(over: { grid?: boolean; frame?: object; place?: object } = {}) {
  const grid = over.grid ?? true
  store.photos = [{ id: 'a', name: 'IMG_0798.jpg', w: 2667, h: 4000 }] as never
  store.view = {
    ...store.view,
    minDPI: 200,
    spreads: [
      {
        id: 's1',
        grid,
        photos: ['a'],
        places: [{ id: 'p', photo: 'a', x: 10, y: 10, w: 195.7, h: 299, shape: 'rect', round: 0, ...over.place }],
        frames: [{ s: 1.2, x: 0, y: 0, img: { x: 0, y: 0, w: 100, h: 100 }, dpi: 340, tilt: 2.5, ...over.frame }],
        rects: [],
        captions: [],
      },
    ],
  } as never
  vi.spyOn(store, 'selectedCell', 'get').mockReturnValue('a')
  placesel.refs = ['p']
}
const can = (on: FeatureId[] | 'all') => vi.spyOn(prefs, 'can').mockImplementation((f) => on === 'all' || on.includes(f))
const heads = () => [...document.querySelectorAll('[data-group]')].map((g) => g.getAttribute('data-group'))
const anchors = () => [...document.querySelectorAll('[data-guide]')].map((e) => e.getAttribute('data-guide'))
const head = (name: RegExp | string) => screen.getByRole('button', { name })

beforeEach(() => {
  localStorage.clear()
  cardGroups.reset()
  api.Person.mockResolvedValue({ last: '', first: '', middle: '', subject: '', teacher: false, fromFile: false, place: false })
})
afterEach(() => {
  cleanup()
  placesel.refs = []
  api.TurnPhotos.mockClear()
  vi.restoreAllMocks()
})

describe('карточка «Фото»: порядок', () => {
  it('группы идут Кадр, Цвет и тон, Форма и рамочка, Имя на фото, Место, О снимке — все открыты', () => {
    setup()
    can('all')
    render(InspectorFrame, docked)
    expect(heads()).toEqual(GROUPS)
    for (const g of GROUPS) expect(document.querySelector(`[data-group="${g}"] .body`)!.hasAttribute('inert')).toBe(false)
  })

  it('владелец 2026-10-18: «Цвет | Ч/б» сразу за «Повернуть · Зеркало» (между ними только граница группы)', () => {
    setup()
    can('all')
    render(InspectorFrame, docked)
    const a = anchors()
    const flip = a.indexOf('flip-toggle')
    expect(flip).toBeGreaterThan(-1)
    expect(a[flip + 1]).toBe('adjust')
    // Внутри «Цвета и тона»: переключатель «Цвет | Ч/б» первым, потом ползунки
    const adj = document.querySelector('[data-guide="adjust"]')!
    expect([...adj.querySelectorAll('button')].slice(0, 2).map((b) => b.textContent)).toEqual(['Цвет', 'Ч/б'])
    // «Зеркало» и «Повернуть» — последняя строка «Кадра»: после неё в DOM следует заголовок «Цвета и тона»
    const row = document.querySelector('.trow')!
    const frameBody = document.querySelector('[data-group="photo.frame"] .inner')!
    expect(frameBody.lastElementChild).toBe(row)
    expect(document.querySelector('[data-group="photo.frame"]')!.nextElementSibling!.getAttribute('data-group')).toBe('photo.tone')
  })

  it('якоря F1 на месте и в свёрнутых группах (тело не размонтируется)', async () => {
    setup()
    can('all')
    render(InspectorFrame, docked)
    for (const g of ['Кадр', 'Цвет и тон', 'Форма и рамочка', 'Имя на фото', 'Место', 'О снимке'])
      await fireEvent.click(head(new RegExp(`^${g}`)))
    const a = anchors()
    for (const k of [
      'tilt',
      'fit-toggle',
      'flip-toggle',
      'adjust',
      'photo-fade',
      'shape-toggle',
      'border',
      'person',
      'snap',
      'layer-row',
      'photo-about',
    ])
      expect(a, k).toContain(k)
    expect(heads()).toEqual(GROUPS)
    expect(JSON.parse(localStorage.getItem('cardGroups')!).d).toEqual(GROUPS)
  })

  it('свёрнутая группа пишет значение: Кадр, Цвет и тон, Форма, Имя, Место, О снимке', async () => {
    setup()
    can('all')
    render(InspectorFrame, docked)
    for (const g of GROUPS) {
      const name = {
        'photo.frame': 'Кадр',
        'photo.tone': 'Цвет и тон',
        'photo.shape': 'Форма и рамочка',
        'photo.name': 'Имя на фото',
        'photo.place': 'Место',
        'photo.about': 'О снимке',
      }[g]!
      await fireEvent.click(head(new RegExp(`^${name}`)))
    }
    const sum = (g: string) => document.querySelector(`[data-group="${g}"] .sum`)?.textContent
    expect(sum('photo.frame')).toBe('120 % · с обрезкой · 2,5°')
    expect(sum('photo.tone')).toBe('цвет')
    expect(sum('photo.shape')).toBe('прямоугольник · без рамочки')
    await waitFor(() => expect(sum('photo.name')).toBe('нет'))
    expect(sum('photo.place')).toBe('195,7 × 299 мм')
    expect(sum('photo.about')).toBe('2667×4000 px')
  })

  it('имя на фото: значение из Go попадает в свёрнутую группу', async () => {
    api.Person.mockResolvedValue({ last: 'Иванов', first: 'Иван', middle: '', subject: '', teacher: false, fromFile: true, place: false })
    setup()
    can('all')
    render(InspectorFrame, docked)
    await fireEvent.click(head(/^Имя на фото/))
    await waitFor(() => expect(document.querySelector('[data-group="photo.name"] .sum')?.textContent).toBe('Иванов Иван'))
  })
})

describe('карточка «Фото»: F1 раскрывает группу с якорем', () => {
  it.each([
    ['photo.frame', 'tilt'],
    ['photo.frame', 'flip-toggle'],
    ['photo.frame', 'fit-toggle'],
    ['photo.tone', 'adjust'],
    ['photo.tone', 'photo-fade'],
    ['photo.shape', 'shape-toggle'],
    ['photo.shape', 'border'],
    ['photo.name', 'person'],
    ['photo.place', 'snap'],
    ['photo.place', 'layer-row'],
    ['photo.about', 'photo-about'],
  ])('%s: якорь %s — свёрнутая группа открывается и запоминается', async (group, anchor) => {
    setup()
    can('all')
    render(InspectorFrame, docked)
    await fireEvent.click(document.querySelector(`[data-group="${group}"] .head`)!)
    expect(cardGroups.isOpen('d', group as never)).toBe(false)
    expect(unfoldFor(`[data-guide="${anchor}"]`)).toBe(true)
    expect(cardGroups.isOpen('d', group as never)).toBe(true)
  })
})

describe('карточка «Фото»: имя файла, dpi, рамочка', () => {
  it('dpi стоит справа от имени файла; px — в подсказке и в «О снимке»', () => {
    setup()
    can('all')
    render(InspectorFrame, docked)
    const top = document.querySelector('.top')!
    expect(top.firstElementChild!.textContent).toBe('IMG_0798.jpg')
    const dpi = top.querySelector('.dpi')!
    expect(dpi.textContent!.trim()).toBe('≈ 340 dpi')
    expect(dpi.getAttribute('title')).toContain('2667×4000 px')
    expect(dpi.classList.contains('low')).toBe(false)
    expect(document.querySelector('[data-group="photo.about"] .px')!.textContent).toBe('2667×4000 px')
  })

  it('мало dpi — красным, в подсказке порог печати', () => {
    setup({ frame: { dpi: 120 } })
    can('all')
    render(InspectorFrame, docked)
    const dpi = document.querySelector('.top .dpi')!
    expect(dpi.classList.contains('low')).toBe(true)
    expect(dpi.getAttribute('title')).toContain('Нужно от 200 dpi')
  })

  it('нет dpi — справа пусто, px остаётся в «О снимке»', () => {
    setup({ frame: { dpi: 0 } })
    can('all')
    render(InspectorFrame, docked)
    expect(document.querySelector('.top .dpi')).toBeNull()
    expect(document.querySelector('.px')!.textContent).toBe('2667×4000 px')
  })

  it('«Рамочка · нет · Выбрать…» — одна строка', () => {
    setup()
    can('all')
    render(InspectorFrame, docked)
    const bd = document.querySelector('[data-guide="border"]')!
    expect(bd.classList.contains('empty')).toBe(true)
    expect(bd.textContent!.replace(/\s+/g, ' ').trim()).toBe('Рамочка нет Выбрать…')
  })
})

describe('«Повернуть · Зеркало»', () => {
  it('в панели — слова и четыре кнопки; в узкой — слов нет, у кнопок title и aria-label', () => {
    setup()
    can('all')
    const a = render(InspectorFrame, docked)
    const row = document.querySelector('.trow')!
    expect(row.textContent).toContain('Повернуть')
    expect(row.textContent).toContain('Зеркало')
    expect(row.querySelectorAll('button')).toHaveLength(4)
    a.unmount()
    render(InspectorFrame, floating)
    const narrow = document.querySelector('.trow')!
    expect(narrow.textContent!.trim()).toBe('')
    for (const b of narrow.querySelectorAll('button')) {
      expect(b.getAttribute('aria-label')).toBeTruthy()
      expect(b.getAttribute('title')).toBeTruthy()
    }
  })

  it('↺ и ↻ зовут TurnPhotos с одним фото карточки', async () => {
    setup()
    can('all')
    render(InspectorFrame, docked)
    await fireEvent.click(screen.getByRole('button', { name: 'Повернуть на 90° против часовой' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Повернуть на 90° по часовой' }))
    expect(api.TurnPhotos.mock.calls).toEqual([
      [['a'], -1],
      [['a'], 1],
    ])
  })

  it('уровень без зеркала — только поворот, без поворота — только зеркало, без обоих — строки нет', () => {
    setup()
    can(['frame', 'turn'])
    const a = render(InspectorFrame, docked)
    expect(document.querySelectorAll('.trow button')).toHaveLength(2)
    expect(document.querySelector('[data-guide="flip-toggle"]')).toBeNull()
    a.unmount()
    can(['frame', 'flip'])
    const b = render(InspectorFrame, docked)
    expect(document.querySelectorAll('.trow button')).toHaveLength(2)
    expect(document.querySelector('[data-guide="flip-toggle"]')).not.toBeNull()
    b.unmount()
    can(['frame'])
    render(InspectorFrame, docked)
    expect(document.querySelector('.trow')).toBeNull()
  })
})

describe('карточка «Фото»: уровни и крайние случаи', () => {
  it('на каждом уровне нет группы с пустым телом (сетка и «Авто»)', () => {
    const levels = [...LEVELS.map((l) => [String(l), features(l, null)] as const), ['«Свой» без функций', new Set<FeatureId>()] as const]
    for (const [name, set] of levels) {
      for (const grid of [true, false]) {
        setup({ grid })
        vi.spyOn(prefs, 'can').mockImplementation((f) => set.has(f))
        render(InspectorFrame, docked)
        for (const g of document.querySelectorAll('[data-group]'))
          expect(g.querySelector('.inner')!.children.length, `${name} ${g.getAttribute('data-group')}`).toBeGreaterThan(0)
        cleanup()
        vi.restoreAllMocks()
      }
    }
  })

  it('«Простой»: кадр и форма скрыты, «О снимке» остаётся', () => {
    setup()
    can([])
    render(InspectorFrame, docked)
    expect(heads()).toEqual(['photo.about'])
  })

  it('«Вписано целиком» — в «Кадре» заметка и «целиком» в значении', async () => {
    setup({ place: { fit: 'contain' } })
    can('all')
    render(InspectorFrame, docked)
    expect(screen.getByText('Вписано целиком')).toBeTruthy()
    await fireEvent.click(head(/^Кадр/))
    expect(document.querySelector('[data-group="photo.frame"] .sum')!.textContent).toBe('целиком · 2,5°')
  })

  it('«Авто»: «Расположения» (слои) нет — оно только на сетке', () => {
    setup({ grid: false })
    can('all')
    render(InspectorFrame, docked)
    expect(document.querySelector('[data-guide="layer-row"]')).toBeNull()
  })

  it('память раздельна: свёрнутое в панели не сворачивает плавающую карточку', async () => {
    setup()
    can('all')
    const a = render(InspectorFrame, docked)
    await fireEvent.click(head(/^Кадр/))
    a.unmount()
    render(InspectorFrame, floating)
    expect(head(/^Кадр/).getAttribute('aria-expanded')).toBe('true')
  })
})
