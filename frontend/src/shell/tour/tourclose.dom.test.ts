// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync, tick } from 'svelte'

// Go-часть тура в миниатюре: состояние first и «done не откатывается» (internal/prefs/tours.go), журнал LogUI.
const cfg = vi.hoisted(() => ({ first: '', calls: [] as string[][], log: [] as string[] }))
const setTour = vi.hoisted(() =>
  vi.fn(async (id: string, state: string) => {
    cfg.calls.push([id, state])
    if (id === 'first' && !(cfg.first === 'done' && state === 'pending')) cfg.first = state
    return { level: 'simple', showStart: false, first: false, tourDue: cfg.first !== 'done', tourMore: false }
  }),
)
const logUI = vi.hoisted(() => vi.fn(async (t: string) => void cfg.log.push(t)))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ SetTour: setTour, SetLevel: async () => ({}), LogUI: logUI, OpenLink: async () => {} }))

const { default: Tour } = await import('./Tour.svelte')
const { tour } = await import('./tourstate.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const wait = (ms: number) => new Promise((r) => setTimeout(r, ms))

beforeEach(() => {
  cfg.first = ''
  cfg.calls.length = 0
  cfg.log.length = 0
  tour.started = false
  // Первый запуск: уровень не выбран, стартовое окно открыто, тур ждёт его закрытия.
  prefs.load({ level: 'simple', showStart: true, first: true, custom: null, tourDue: true })
  store.loaded = true
  store.view = { ...store.view, folders: [] }
})
afterEach(() => {
  cleanup()
  tour.finish('done')
  tour.started = false
})

// Закрытие окна программы: Svelte-дерево уходит, ни одной команды тура не приходит.
const closeWindow = async () => {
  cleanup()
  await wait(50)
}

async function startTour() {
  render(Tour)
  prefs.closeStart()
  flushSync()
  await tick()
  expect(tour.open).toBe(true)
}

describe('тур: закрытие окна программы посреди тура', () => {
  it('приветствие: окно закрыто — в config остаётся pending, тур вернётся', async () => {
    await startTour()
    await closeWindow()
    expect(cfg.calls).toEqual([['first', 'pending']])
    expect(cfg.first).toBe('pending')
  })

  it('шаг 1, папка добавлена (карточка «Фото уже в ленте…»), окно закрыто — pending', async () => {
    await startTour()
    tour.next()
    flushSync()
    await wait(50)
    store.view = { ...store.view, folders: [{ path: '/a' } as never] }
    flushSync()
    await wait(300)
    expect(tour.res?.live).toBe(true)
    await closeWindow()
    expect(cfg.calls).toEqual([['first', 'pending']])
    expect(cfg.first).toBe('pending')
  })

  it('шаг 3 без папки, окно закрыто — pending', async () => {
    await startTour()
    tour.go(2)
    flushSync()
    await wait(50)
    await closeWindow()
    expect(cfg.first).toBe('pending')
  })

  it('«Пропустить» — done и строка в журнале: чем и где тур закрыт', async () => {
    await startTour()
    tour.next()
    flushSync()
    await fireEvent.click(screen.getByRole('button', { name: 'Пропустить' }))
    await wait(50)
    expect(cfg.first).toBe('done')
    expect(cfg.log).toEqual(['тур закрыт: skip, click, шаг 0, запуск auto'])
  })

  it('Esc в карточке — тоже skip, в журнале «key»', async () => {
    await startTour()
    await fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    await wait(50)
    expect(cfg.first).toBe('done')
    expect(cfg.log).toEqual(['тур закрыт: skip, key, шаг intro, запуск auto'])
  })
})
