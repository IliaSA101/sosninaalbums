// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Галерея сторонки обложки (127-17): вкладка по умолчанию, ряды, «−N», щелчок, наведение, уровни. Go подменён.
const calls: { name: string; args: unknown[] }[] = []
let answer: (scope: string, n: number) => unknown = () => ({})

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  CoverGridChoices: async (scope: string, n: number) => {
    calls.push({ name: 'CoverGridChoices', args: [scope, n] })
    return answer(scope, n)
  },
  ApplyCoverGrid: async (scope: string, boxes: unknown) => {
    calls.push({ name: 'ApplyCoverGrid', args: [scope, boxes] })
    return { view: coverView(9), left: 2, named: [] }
  },
  SetCustom: async () => {},
  SetLevel: async () => ({ first: false }),
  LayoutChoices: async () => ({}),
  Marks: async () => ({}),
}))
vi.mock('../../shell/guide/guidestate.svelte', () => ({ guide: { show: () => {} } }))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { default: CoverGridGallery } = await import('./CoverGridGallery.svelte')
const { store } = await import('../../base/core/store.svelte')
const { gallery } = await import('./gallery.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')

const BOX = { x: 160, y: 10, w: 130, h: 280 }
const sides = (places: number) => [
  { scope: 'left', label: 'Задняя', box: { ...BOX, x: 10 }, places: 0, known: false, alone: false },
  { scope: 'right', label: 'Лицевая', box: BOX, places, known: places > 0, alone: false },
]
function coverView(rev: number, places = 3) {
  return {
    rev,
    marksRev: 0,
    folders: [],
    settings: { sheet: false, pageW: 300, pageH: 300 },
    spreads: [{ id: 's1', photos: [] }],
    cover: {
      sheet: {
        id: 'cover',
        photos: ['a', 'b'],
        rects: [
          { x: 170, y: 20, w: 50, h: 50 },
          { x: 230, y: 20, w: 50, h: 50 },
        ],
        w: 300,
        h: 300,
        mid: 150,
      },
      sides: sides(places),
      spine: 10,
    },
  } as never
}
const tile = (id: string, drop = 0, n = 2) => ({
  id,
  name: '',
  scope: 'page',
  drop,
  boxes: Array.from({ length: n }, (_, i) => ({ x: i / n, y: 0, w: 1 / n, h: 1 })),
})
const full = (n = 3) => ({
  scope: 'right',
  n,
  box: BOX,
  places: 3,
  photos: 3,
  byPhoto: [tile('p1'), tile('p2')],
  builtin: [tile('b1', 1)],
  mine: [tile('m1', 0, 3)],
})
const cards = () => [...document.querySelectorAll<HTMLButtonElement>('button.card')]
const side = (label: string) =>
  [...document.querySelectorAll<HTMLButtonElement>('.vseg button')].find((b) => b.textContent?.startsWith(label))
const flush = async () => {
  await tick()
  await new Promise((r) => setTimeout(r, 0))
  await tick()
}

async function open(places = 3, scope: 'left' | 'right' = 'right') {
  store.onCover = false
  store.apply(coverView(5, places))
  store.goCover()
  gallery.open(scope, 'grids')
  const r = render(CoverGridGallery, { g: { spread: 'cover', scope } })
  await flush()
  return r
}

beforeEach(() => {
  calls.length = 0
  store.toast = null
  answer = () => full()
  prefs.level = 'full'
})
afterEach(() => {
  cleanup()
  gallery.close()
})

describe('галерея сторонки обложки', () => {
  it('вкладка по умолчанию — число мест сторонки; шапка без вкладок и «Сохранить…»', async () => {
    await open(3)
    expect(calls[0]).toEqual({ name: 'CoverGridChoices', args: ['right', 3] })
    expect(document.querySelector('[aria-label="Число мест"] .on, [aria-label="Число мест"] [aria-pressed="true"]')?.textContent).toBe('3')
    expect(document.querySelector('.save')).toBeNull()
    expect(document.querySelector('[aria-label="Что показать"]')).toBeNull()
    expect(document.querySelector('.title')?.textContent).toBe('Лицевая')
  })

  it('0 мест — вкладка «1»', async () => {
    answer = () => ({ ...full(1), places: 0, byPhoto: [] })
    await open(0)
    expect(calls[0].args).toEqual(['right', 1])
  })

  it('«Все»: по фото, встроенные, мои; слева четыре пункта; «−N» только у плиток с потерей', async () => {
    await open()
    expect(cards()).toHaveLength(4)
    expect([...document.querySelectorAll('.vseg button')].map((b) => b.textContent)).toEqual(['Все4', 'По фото2', 'Встроенные1', 'Мои1'])
    expect(cards().map((c) => c.querySelector('.drop')?.textContent ?? '')).toEqual(['', '', '−1', ''])
  })

  it('«По фото» пуст — пункт серый с причиной, плитки идут дальше', async () => {
    answer = (_, n) => (n === 5 ? { ...full(5), byPhoto: [] } : full(n))
    await open(3)
    const seg = document.querySelector('[aria-label="Число мест"]')!
    await fireEvent.click([...seg.querySelectorAll('button')].find((b) => b.textContent === '5')!)
    await flush()
    const b = side('По фото')!
    expect(b.getAttribute('aria-disabled')).toBe('true')
    expect(b.title).toBe('На сторонке 3 места — откройте вкладку «3»')
    expect(cards()).toHaveLength(2)
  })

  it('пункт слева переключает ряд', async () => {
    await open()
    await fireEvent.click(side('Встроенные')!)
    expect(cards()).toHaveLength(1)
    await fireEvent.click(side('Мои')!)
    expect(cards()).toHaveLength(1)
  })

  it('число мест — новый запрос Go', async () => {
    await open()
    const seg = document.querySelector('[aria-label="Число мест"]')!
    await fireEvent.click([...seg.querySelectorAll('button')].find((b) => b.textContent === '5')!)
    await flush()
    expect(calls.at(-1)).toEqual({ name: 'CoverGridChoices', args: ['right', 5] })
  })

  it('щелчок: галерея закрывается, Go получает доли как есть, лишние фото — тост «Вернуть»', async () => {
    await open()
    const first = cards()[0]
    await fireEvent.click(first)
    await flush()
    expect(gallery.cur).toBeNull()
    const ap = calls.find((c) => c.name === 'ApplyCoverGrid')!
    expect(ap.args[0]).toBe('right')
    expect(ap.args[1]).toEqual(full().byPhoto[0].boxes)
    expect(store.toast?.text).toBe('2 фото вернулись в ленту')
    expect(store.toast?.action?.label).toBe('Вернуть')
  })

  it('наведение: предпросмотр на листе обложки в мм листа; уход курсора снимает', async () => {
    await open()
    await fireEvent.mouseEnter(cards()[0])
    expect(gallery.preview?.spread).toBe('cover')
    // две пары фото лицевой встают в половины зоны: x = 160 и 160 + 65
    expect(gallery.preview?.rects.map((r) => [r.x, r.w])).toEqual([
      [160, 65],
      [225, 65],
    ])
    await fireEvent.mouseLeave(cards()[0])
    expect(gallery.preview).toBeNull()
  })

  it('мои плитки только ставятся: ни ✎, ни ×', async () => {
    await open()
    expect(document.querySelector('.tools')).toBeNull()
  })

  // Ряды по уровням: «Варианты» держат «По фото» и «Встроенные», «Шаблоны» — «Мои» (0.26-127-14).
  const reset = () => {
    cleanup()
    gallery.close()
    prefs.custom = null
    prefs.level = 'full'
  }

  it('без уровня «Шаблоны» нет «Мои»: ни пункта, ни плиток в «Все»', async () => {
    prefs.level = 'simple'
    await open()
    expect(side('Мои')).toBeUndefined()
    expect(side('Встроенные')).toBeDefined()
    expect(side('По фото')).toBeDefined()
    expect(cards()).toHaveLength(3) // p1, p2, b1 — без m1 (три места)
    reset()
  })

  it('без «Вариантов» нет «По фото» и «Встроенных»: ни пунктов, ни плиток в «Все»', async () => {
    prefs.toggle('variants', false)
    await open()
    expect(prefs.can('variants')).toBe(false)
    expect(prefs.can('templates')).toBe(true)
    expect(side('По фото')).toBeUndefined()
    expect(side('Встроенные')).toBeUndefined()
    expect(side('Мои')).toBeDefined()
    expect(cards()).toHaveLength(1) // только m1
    expect(cards()[0].querySelectorAll('.cell')).toHaveLength(3)
    reset()
  })
})
