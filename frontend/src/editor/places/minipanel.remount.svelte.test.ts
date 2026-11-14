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

// CI e2e R4: панель пересоздана под неподвижной мышью при зажатом Shift — «насквозь» не должно пропадать.
describe('MiniPanel: пересоздание при зажатом Shift', () => {
  const ptr = (type: string, shiftKey: boolean) => window.dispatchEvent(new PointerEvent(type, { shiftKey, bubbles: true }))
  const through = () => document.querySelector('.minipanel')?.classList.contains('through')
  const key = (shiftKey: boolean) => window.dispatchEvent(new KeyboardEvent('keyup', { shiftKey, bubbles: true }))
  afterEach(() => ptr('pointermove', false))

  it('новый экземпляр остаётся насквозь, пока последний указатель был с Shift', () => {
    placesel.refs = ['p1']
    const a = up()
    flushSync()
    expect(through()).toBe(false)
    ptr('pointermove', true)
    flushSync()
    expect(through()).toBe(true)
    unmount(a)
    flushSync()
    const b = up() // мыши под новым экземпляром не двигали: событий указателя больше не будет
    flushSync()
    expect(through()).toBe(true)
    ptr('pointermove', false) // отпустили Shift и двинули мышь
    flushSync()
    expect(through()).toBe(false)
    unmount(b)
  })

  it('указатель без Shift, пока панели нет, сбрасывает признак для следующей', () => {
    placesel.refs = ['p1']
    const a = up()
    flushSync()
    ptr('pointermove', true)
    unmount(a)
    flushSync()
    ptr('pointermove', false) // панели нет — слушает модуль
    const b = up()
    flushSync()
    expect(through()).toBe(false)
    unmount(b)
  })

  it('Shift отпущен keyup без движения мыши — новая панель не насквозь', () => {
    placesel.refs = ['p1']
    const a = up()
    flushSync()
    ptr('pointerdown', true) // Shift+щелчок по фото
    unmount(a)
    flushSync()
    key(false) // Shift отпустили, мышь стоит
    const b = up()
    flushSync()
    expect(through()).toBe(false)
    unmount(b)
  })

  it('пересчёт наведения браузером (pointerover без модификаторов) не гасит насквозь при зажатом Shift', () => {
    placesel.refs = ['p1']
    const a = up()
    flushSync()
    window.dispatchEvent(new KeyboardEvent('keydown', { shiftKey: true, bubbles: true }))
    ptr('pointermove', true)
    flushSync()
    expect(through()).toBe(true)
    // Панель стала pointer-events: none — браузер сам шлёт pointerover, в Linux-Chromium без Shift (158 R4)
    window.dispatchEvent(new PointerEvent('pointerover', { shiftKey: false, bubbles: true }))
    document.querySelector('.minipanel')?.dispatchEvent(new PointerEvent('pointerover', { shiftKey: false, bubbles: true }))
    flushSync()
    expect(through()).toBe(true)
    unmount(a)
  })

  it('keyup с Shift (отпущена другая клавиша) без указателя признак не включает', () => {
    placesel.refs = ['p1']
    window.dispatchEvent(new Event('blur')) // указателя «не видели»
    key(true)
    const a = up()
    flushSync()
    expect(through()).toBe(false)
    unmount(a)
  })

  it('потеря фокуса окна и скрытие вкладки сбрасывают признак для новой панели', () => {
    placesel.refs = ['p1']
    for (const lose of [() => window.dispatchEvent(new Event('blur')), () => document.dispatchEvent(new Event('visibilitychange'))]) {
      ptr('pointermove', true)
      vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
      lose()
      vi.restoreAllMocks()
      const b = up()
      flushSync()
      expect(through()).toBe(false)
      unmount(b)
    }
  })
})
