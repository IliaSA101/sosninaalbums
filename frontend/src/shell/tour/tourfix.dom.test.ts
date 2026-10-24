// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'

// Ответ SetTour не мгновенный, как у Go: между finish и ответом tourDue ещё true.
const setTour = vi.hoisted(() =>
  vi.fn(async (_id: string, _state: string) => {
    await new Promise((r) => setTimeout(r, 20))
    return { level: 'full', showStart: false, first: false, tourDue: false, tourMore: false }
  }),
)
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ SetTour: setTour, OpenLink: async () => {} }))

const { default: Tour } = await import('./Tour.svelte')
const { tour } = await import('./tourstate.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { spot } = await import('../guide/spotstate.svelte')

const wait = (ms: number) => new Promise((r) => setTimeout(r, ms))
const rect = { left: 10, top: 20, width: 50, height: 30, right: 60, bottom: 50, x: 10, y: 20, toJSON: () => ({}) }

function anchor(id = 'folders') {
  const el = document.createElement('button')
  el.setAttribute('data-guide', id)
  el.getBoundingClientRect = () => rect
  document.body.append(el)
  return el
}

afterEach(async () => {
  cleanup()
  tour.finish('done')
  await wait(40)
  spot.cancel()
  document.body.innerHTML = ''
  setTour.mockClear()
  tour.started = false
})

describe('тур: повторное открытие после закрытия', () => {
  it('старт из меню при tourDue → «Пропустить» → тур не открылся, последняя запись done', async () => {
    prefs.load({ level: 'full', showStart: true, first: false, custom: null, tourDue: true })
    store.loaded = true
    render(Tour)
    tour.start('menu')
    flushSync()
    expect(tour.open).toBe(true)
    tour.finish('skip')
    flushSync()
    await wait(100)
    flushSync()
    expect(tour.open).toBe(false)
    expect(setTour.mock.calls.at(-1)).toEqual(['first', 'done'])
    expect(setTour.mock.calls.some((c) => c[1] === 'pending')).toBe(false)
  })

  it('автозапуск остаётся: при tourDue слой открывает приветствие сам (pending)', async () => {
    prefs.load({ level: 'full', showStart: false, first: false, custom: null, tourDue: true })
    store.loaded = true
    tour.started = false
    render(Tour)
    flushSync()
    expect(tour.open).toBe(true)
    expect(tour.from).toBe('auto')
    expect(setTour).toHaveBeenCalledWith('first', 'pending')
  })
})

describe('тур: кольцо при замене элемента-цели', () => {
  it('шаг 1: кнопку заменили другой с тем же data-guide — кольцо вернулось', async () => {
    prefs.load({ level: 'full', showStart: false, first: false, custom: null, tourDue: false })
    store.loaded = true
    render(Tour)
    tour.start('menu')
    flushSync()
    tour.go(0)
    flushSync()
    const a = anchor()
    await wait(100)
    flushSync()
    expect(tour.ring).not.toBeNull()
    a.remove()
    const b = anchor()
    await wait(300)
    flushSync()
    expect(tour.ring).not.toBeNull()
    expect(spot.cur?.el).toBe(b)
    expect(tour.missing).toBe(false)
  })

  it('элемент снят, замены нет — место «нет на экране», кольца нет, без зацикливания', async () => {
    prefs.load({ level: 'full', showStart: false, first: false, custom: null, tourDue: false })
    store.loaded = true
    render(Tour)
    tour.start('menu')
    flushSync()
    tour.go(0)
    flushSync()
    const a = anchor()
    await wait(100)
    a.remove()
    await wait(1200)
    flushSync()
    expect(tour.ring).toBeNull()
    expect(tour.missing).toBe(true)
  })
})

describe('тур: ловушка и открытые окна', () => {
  const trapsOn = () => !document.querySelector('[data-tour-layer]')?.classList.contains('pass')

  async function openTour() {
    prefs.load({ level: 'full', showStart: false, first: false, custom: null, tourDue: false })
    store.loaded = true
    render(Tour)
    tour.start('menu')
    flushSync()
    await tick()
  }

  it('меню «⋯» (role=menu в body) — ловушка пропускает щелчки; закрылось — вернулась', async () => {
    await openTour()
    expect(trapsOn()).toBe(true)
    const menu = document.createElement('div')
    menu.setAttribute('role', 'menu')
    document.body.append(menu)
    await wait(10)
    flushSync()
    expect(tour.popup).toBe(true)
    expect(trapsOn()).toBe(false)
    expect(document.querySelector('[data-tour-layer]')?.classList.contains('lit')).toBe(true)
    menu.remove()
    await wait(10)
    flushSync()
    expect(tour.popup).toBe(false)
    expect(trapsOn()).toBe(true)
  })

  it('поповер (data-barpop) тоже пропускается', async () => {
    await openTour()
    const pop = document.createElement('div')
    pop.setAttribute('data-barpop', 'x')
    document.body.append(pop)
    await wait(10)
    flushSync()
    expect(trapsOn()).toBe(false)
  })

  it('финиш при открытом меню: флаг сброшен, следующий тур начинается с ловушки', async () => {
    await openTour()
    const menu = document.createElement('div')
    menu.setAttribute('role', 'menu')
    document.body.append(menu)
    await wait(10)
    expect(tour.popup).toBe(true)
    tour.finish('done')
    flushSync()
    expect(tour.popup).toBe(false)
    menu.remove()
    tour.start('menu')
    flushSync()
    await wait(10)
    expect(trapsOn()).toBe(true)
  })
})

describe('тур: области, которых карточка не закрывает (122)', () => {
  const at = (el: HTMLElement, r: { left: number; top: number; width: number; height: number }) => {
    el.getBoundingClientRect = () => ({ ...r, right: r.left + r.width, bottom: r.top + r.height, x: r.left, y: r.top, toJSON: () => ({}) })
  }

  it('селекторы avoid всех шагов принимает querySelector', async () => {
    const { TOUR } = await import('./steps')
    for (const s of TOUR) for (const sel of s.avoid ?? []) expect(() => document.querySelector(sel), `${s.id}: ${sel}`).not.toThrow()
  })

  it('шаг 1: avoid — рамка ленты фото, обрезанная по окну; без кольца и на шаге без avoid — пусто', async () => {
    prefs.load({ level: 'full', showStart: false, first: false, custom: null, tourDue: false })
    store.loaded = true
    render(Tour)
    tour.start('menu')
    flushSync()
    expect(tour.avoid).toEqual([]) // приветствие: кольца нет
    tour.go(0)
    flushSync()
    anchor()
    const lib = document.createElement('div')
    lib.setAttribute('data-guide', 'library')
    at(lib, { left: 0, top: window.innerHeight - 200, width: window.innerWidth + 300, height: 400 })
    document.body.append(lib)
    await wait(100)
    flushSync()
    expect(tour.ring).not.toBeNull()
    expect(tour.avoid).toEqual([{ left: 0, top: window.innerHeight - 200, width: window.innerWidth, height: 200 }])
    lib.remove() // области нет на экране — пропуск
    expect(tour.avoid).toEqual([])
    tour.go(1) // шаг «Лента фото»: avoid не задан
    flushSync()
    expect(tour.avoid).toEqual([])
  })
})
