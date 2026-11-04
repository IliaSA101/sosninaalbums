// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'

// F1 «Показать» к «Фону», «Подписям» и «Сохранить как шаблон…» виньетки (спека 124 U5): свёрнутая группа раскрывается
// и запоминается; кнопка ушла в «⋯» «Виньетки» — мигает «⋯» с подсказкой «В меню «⋯»».
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), MyColors: vi.fn(async () => []) }
})

const { default: InspectorSpread } = await import('./InspectorSpread.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { cardGroups, cardContext } = await import('../../base/prefs/cardgroups.svelte')
const { spot, seek } = await import('../../shell/guide/spotstate.svelte')

const VIG = { tpl: 'classic', teachers: 1, pupils: 23, ratio: '3:4', group: false, ttext: '', shape: '', round: 0 }

function show(vignette: boolean) {
  store.view = {
    ...store.view,
    rev: 5,
    settings: { pageW: 300, pageH: 300, bg: '' },
    spreads: [
      {
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
        ...(vignette ? { kind: 'vignette', vig: VIG } : {}),
      },
    ],
  } as never
  vi.spyOn(prefs, 'can').mockReturnValue(true)
  render(InspectorSpread, { context: new Map([cardContext(() => true)]) })
}

beforeEach(() => {
  localStorage.clear()
  cardGroups.reset()
  Object.defineProperty(window, 'innerWidth', { value: 1200, configurable: true })
  Object.defineProperty(window, 'innerHeight', { value: 700, configurable: true })
  // happy-dom не считает раскладку: каждый элемент «виден»
  const rect = { left: 10, top: 20, width: 50, height: 20, right: 60, bottom: 40, x: 10, y: 20, toJSON: () => ({}) }
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(rect as DOMRect)
  vi.spyOn(HTMLElement.prototype, 'getClientRects').mockReturnValue([rect] as unknown as DOMRectList)
})
afterEach(() => {
  cleanup()
  spot.cancel()
  vi.restoreAllMocks()
})

describe('F1 к группам карточки листа', () => {
  it('«Фон» свёрнут: показ раскрывает и запоминает группу', async () => {
    cardGroups.toggle('d', 'spread.bg')
    show(false)
    const found = await seek('ui:spread-bg')
    expect(found?.el.getAttribute('data-guide')).toBe('spread-bg')
    expect(cardGroups.isOpen('d', 'spread.bg')).toBe(true)
    expect(JSON.parse(localStorage.getItem('cardGroups')!)).toEqual({ d: [], f: [] })
  })

  it('«Подписи» свёрнуты: показ раскрывает группу', async () => {
    cardGroups.toggle('d', 'spread.caps')
    show(false)
    const found = await seek('ui:sheet-caps')
    expect(found?.el.getAttribute('data-guide')).toBe('sheet-caps')
    expect(cardGroups.isOpen('d', 'spread.caps')).toBe(true)
  })

  it('«Сохранить как шаблон…» виньетки: мигает «⋯» группы с подсказкой, группу не раскрывает', async () => {
    cardGroups.toggle('d', 'spread.vig')
    show(true)
    const found = await seek('ui:vig-save')
    expect(found?.el.getAttribute('aria-label')).toBe('Ещё: Виньетка')
    expect(found?.hint).toBe('В меню «⋯»')
    expect(cardGroups.isOpen('d', 'spread.vig')).toBe(false)
  })
})
