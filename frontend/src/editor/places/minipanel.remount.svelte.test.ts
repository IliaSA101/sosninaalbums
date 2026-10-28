// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushSync, mount, unmount } from 'svelte'

// Владелец 2026-10-22 (спека 100 п. 5): после жеста мини-панель не возвращалась. Портал уносит полосу в body, и Svelte
// при скрытии ветки сносил всех соседей справа от якоря (другие якоря, направляющие). Проверяем DOM: соседи целы,
// якорь на месте, в body ровно одна полоса. Мини-панель одна и та же у SelFrame (фото) и GroupFrame (группа).
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { store } = await import('../../base/core/store.svelte')
const { placesel } = await import('./placesel.svelte')
const { default: MiniPanel } = await import('./MiniPanel.svelte')

const place = (id: string, photo: string, x: number) => ({ id, photo, fit: '', spine: false, z: 0, x, y: 20, w: 50, h: 50, rot: 0 })
const spread = {
  id: 's1',
  w: 300,
  h: 150,
  mid: 150,
  grid: true,
  photos: [],
  rects: [],
  frames: [],
  texts: [],
  captions: [],
  backdrops: [],
  places: [place('p1', 'a.jpg', 10), place('p2', 'b.jpg', 80)],
} as never

let host: HTMLElement
let guard: HTMLElement
let guide: HTMLElement

beforeEach(() => {
  document.body.innerHTML = ''
  host = document.body.appendChild(document.createElement('div'))
  host.className = 'selframe'
  // Соседи справа от мини-панели в SelFrame: якорь соседней ветки и направляющая.
  guard = host.appendChild(document.createElement('span'))
  guard.className = 'neighbour-anchor'
  guide = host.appendChild(document.createElement('div'))
  guide.className = 'guide'
  store.view = { ...store.view, spreads: [spread] } as never
  store.currentId = 's1'
})
afterEach(() => {
  placesel.refs = []
  document.body.innerHTML = ''
})

const panels = () => document.querySelectorAll('.minipanel').length
const up = () =>
  mount(MiniPanel, {
    target: host,
    anchor: guard,
    props: { spread, box: { x: 10, y: 20, w: 50 * placesel.refs.length, h: 50 }, width: 300, height: 150, ppm: 1 },
  })

describe.each([
  ['одиночное фото', ['p1']],
  ['группа', ['p1', 'p2']],
])('MiniPanel: возврат после жеста — %s', (_name, refs) => {
  it('скрытие ветки не трогает соседей, якорь возвращается в лист, в body одна полоса', () => {
    placesel.refs = refs
    for (let round = 0; round < 3; round++) {
      const c = up()
      flushSync()
      expect(host.querySelectorAll('.anchor')).toHaveLength(1)
      expect(panels()).toBe(1)
      unmount(c) // жест начался: panelBox = null
      flushSync()
      expect(panels()).toBe(0)
      expect(host.querySelector('.anchor')).toBeNull()
      expect(host.contains(guard)).toBe(true)
      expect(host.contains(guide)).toBe(true)
    }
  })
  it('якорь внутри листа (measure берёт его родителя), а не сирота', () => {
    placesel.refs = refs
    const c = up()
    flushSync()
    expect(host.querySelector('.anchor')?.parentElement).toBe(host)
    unmount(c)
  })
})
