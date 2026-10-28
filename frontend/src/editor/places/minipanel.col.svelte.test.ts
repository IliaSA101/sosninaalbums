// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushSync, mount, tick, unmount } from 'svelte'

// Столбец мини-панели (спека 108, владелец 2026-10-23): состав и порядок, высота по colHeight, число уголком,
// «⋯» открывает список сбоку, подсказки с противоположной от фото стороны. Геометрию happy-dom не считает —
// положение и CSS остаются за QA; здесь переходы состояния.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { store } = await import('../../base/core/store.svelte')
const { placesel } = await import('./placesel.svelte')
const { tip } = await import('../../base/ui/tip.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { colHeight } = await import('./minicol')
const { default: MiniPanel } = await import('./MiniPanel.svelte')

const place = (id: string, photo: string, x: number) => ({ id, photo, fit: '', spine: false, z: 0, x, y: 20, w: 100, h: 150, rot: 0 })
const spreadOf = (grid: boolean, ids: [string, string][]) =>
  ({
    id: 's1',
    w: 1000,
    h: 600,
    mid: 500,
    grid,
    photos: grid ? [] : ids.map(([, photo]) => photo),
    rects: grid ? [] : ids.map((_, i) => ({ x: 100 + i * 150, y: 20, w: 100, h: 150 })),
    frames: [],
    texts: [],
    captions: [],
    backdrops: [],
    places: grid ? ids.map(([id, photo], i) => place(id, photo, 100 + i * 150)) : [],
  }) as never

let host: HTMLElement
let comp: ReturnType<typeof mount> | undefined

function up(grid: boolean, ids: [string, string][]) {
  const spread = spreadOf(grid, ids)
  store.view = { ...store.view, spreads: [spread] } as never
  store.currentId = 's1'
  placesel.refs = ids.map(([id, photo]) => (grid ? id : photo))
  comp = mount(MiniPanel, {
    target: host,
    anchor: undefined,
    props: { spread, box: { x: 100, y: 20, w: 100 * ids.length, h: 150 }, width: 1000, height: 600, ppm: 1 },
  })
  flushSync()
}

beforeEach(() => {
  document.body.innerHTML = ''
  host = document.body.appendChild(document.createElement('div'))
  host.className = 'selframe'
  prefs.level = 'full'
  tip.hide()
})
afterEach(() => {
  if (comp) unmount(comp)
  comp = undefined
  placesel.refs = []
  prefs.level = 'simple'
  document.body.innerHTML = ''
})

const panel = () => document.querySelector<HTMLElement>('.minipanel')!
const buttons = () => [...panel().querySelectorAll<HTMLButtonElement>('button')]
const names = () => buttons().map((b) => b.getAttribute('aria-label') ?? '')

describe('MiniPanel: столбец (108)', () => {
  it('столбец: значки сверху вниз, корзина последняя, без translateX', () => {
    up(true, [['p1', 'a.jpg']])
    expect(panel().classList.contains('col')).toBe(true)
    expect(panel().style.transform).toBe('')
    const n = names()
    expect(n).toHaveLength(8)
    expect(n[0]).toMatch(/Зеркало ↔/)
    expect(n[1]).toMatch(/Зеркало ↕/)
    expect(n[2]).toMatch(/целиком|обрезкой/)
    expect(n[3]).toMatch(/Повернуть фото на 90°/)
    expect(n[4]).toMatch(/Наклон/)
    expect(n[5]).toMatch(/Перетащите/) // ⇄
    expect(n[6]).toMatch(/Ещё/)
    expect(n[7]).toMatch(/Убрать фото/)
    expect(buttons().at(-1)?.classList.contains('del')).toBe(true)
    // между группами — черты (⋯ и корзина — отдельные группы)
    expect(panel().querySelectorAll('.sep')).toHaveLength(5)
  })

  it('высота столбца в DOM совпадает с colHeight', () => {
    up(true, [['p1', 'a.jpg']])
    expect(panel().style.height).toBe(`${colHeight(8, 5)}px`)
    expect(panel().style.height).toBe('245px')
  })

  it('число у корзины группы — уголком, столбец остаётся одним столбцом той же ширины', () => {
    up(true, [
      ['p1', 'a.jpg'],
      ['p2', 'b.jpg'],
      ['p3', 'c.jpg'],
    ])
    expect(panel().classList.contains('col')).toBe(true)
    const badge = panel().querySelector('.badge')
    expect(badge?.textContent).toBe('3')
    expect(badge?.closest('button')?.classList.contains('del')).toBe(true)
    expect(panel().style.height).toBe(`${colHeight(6, 4)}px`) // ↔ ↕ | ↻ | одинаковый | ⋯ | корзина
  })

  it('«Авто»: нет ⇄, ⋯ раскрывает «Подпись» и заметку', async () => {
    up(false, [['p1', 'a.jpg']])
    expect(names().some((x) => /Перетащите/.test(x))).toBe(false)
    buttons()[names().findIndex((x) => /Ещё/.test(x))].click()
    flushSync()
    await tick()
    const items = [...document.querySelectorAll('[role="menu"] > button, [role="menu"] .note')].map((e) => e.textContent?.trim())
    expect(items.some((t) => t?.includes('Подпись'))).toBe(true)
    expect(items.some((t) => t?.includes('Всё остальное — правым щелчком по фото'))).toBe(true)
    expect(items.some((t) => t?.includes('Слой'))).toBe(false)
  })

  it('⋯ открывает список; повторный щелчок и Esc закрывают, соседи в body целы', async () => {
    up(true, [['p1', 'a.jpg']])
    const more = () => buttons()[names().findIndex((x) => /Ещё/.test(x))]
    const bystander = document.body.appendChild(document.createElement('div'))
    bystander.id = 'bystander'
    more().click()
    flushSync()
    await tick()
    expect(document.querySelector('[role="menu"]')).not.toBeNull()
    expect(more().getAttribute('aria-expanded')).toBe('true')
    // узел, появившийся в body после списка: закрытие не должно его снести (обход nextSibling от унесённого узла)
    document.body.appendChild(document.createElement('div')).id = 'later'
    more().click() // повторный щелчок закрывает
    flushSync()
    expect(document.querySelector('[role="menu"]')).toBeNull()
    expect(document.getElementById('bystander')).not.toBeNull()
    expect(document.getElementById('later')).not.toBeNull()
    more().click()
    flushSync()
    await tick()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    flushSync()
    expect(document.querySelector('[role="menu"]')).toBeNull()
    expect(document.getElementById('bystander')).not.toBeNull()
    expect(panel()).not.toBeNull()
  })

  it('«Простой»: ↻ | ⋯ | корзина — столбец 98 px', () => {
    prefs.level = 'simple'
    up(true, [['p1', 'a.jpg']])
    expect(buttons()).toHaveLength(3)
    expect(panel().style.height).toBe(`${colHeight(3, 2)}px`)
  })

  it('подсказки: столбец справа — side right', () => {
    up(true, [['p1', 'a.jpg']])
    buttons()[0].dispatchEvent(new Event('pointerenter'))
    expect(tip.state?.side).toBe('right')
  })
})
