// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'

// Подпись формата в шапке (123 QA): страница из px — 303,95 × 202,95 мм — без float-хвостов, с запятой.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ RecentProjects: vi.fn(async () => []) }))

const { default: TopBar } = await import('./TopBar.svelte')
const { store } = await import('../../base/core/store.svelte')
const { jobs, EXPORT_EMPTY } = await import('../../base/core/jobs.svelte')
const { dropCatalog, useCatalog } = await import('../../base/keys/keymap.fixture')

const settings = { pageW: 300, pageH: 300, dpi: 300, margin: 10, gap: 3, safe: 5, bleed: 3, bg: '' }
const show = (st: Record<string, unknown>) => {
  store.view = { rev: 1, folders: [], spreads: [], settings: { ...settings, ...st } } as never
}
const label = () => document.querySelector('button[title="Настройки альбома в панели деталей"]')!.textContent!.replace(/\s+/g, ' ').trim()

beforeEach(() => useCatalog())
afterEach(() => {
  cleanup()
  dropCatalog()
})

describe('шапка: формат страницы', () => {
  it('дробная страница — «Формат 30,4×20,29 см»', () => {
    show({ pageW: 303.95, pageH: 202.95 })
    render(TopBar)
    expect(label()).toBe('Формат 30,4×20,29 см')
  })
  it('круглая страница и лист виньетки', () => {
    show({ pageW: 300, pageH: 300 })
    render(TopBar)
    expect(label()).toBe('Формат 30×30 см')
    cleanup()
    show({ pageW: 204, pageH: 304, sheet: true })
    render(TopBar)
    expect(label()).toBe('Лист 20,4×30,4 см')
  })
})

describe('шапка: режим «Во весь экран» (128-11)', () => {
  it('подсказка и подпись кнопки F — «Во весь экран» с клавишей из реестра', () => {
    show({})
    render(TopBar)
    const b = document.querySelector<HTMLElement>('[data-guide="topbar-big"]')!
    expect(b.getAttribute('title')).toBe('Во весь экран (F)')
    expect(b.getAttribute('aria-label')).toBe('Во весь экран')
  })
})

describe('шапка: строка сводки', () => {
  it('обрезанная при узком окне строка имеет title с полным текстом (124 Q1)', () => {
    const sp = (id: string) => ({ id, grid: false, photos: [], rects: [], places: [], frames: [], texts: [], captions: [] })
    store.photos = [
      { id: 'a', name: 'a.jpg', w: 1, h: 1 },
      { id: 'b', name: 'b.jpg', w: 1, h: 1 },
    ] as never
    store.view = { rev: 1, folders: [], spreads: [sp('s1'), sp('s2')], settings } as never
    render(TopBar)
    const info = document.querySelector('.info')!
    const text = info.textContent!.replace(/\s+/g, ' ').trim()
    expect(text).toMatch(/^2 разворота \(\d+ стр\.\) · 0 \/ 2 фото в альбоме$/)
    expect(info.getAttribute('title')).toBe(text)
    store.photos = []
  })
})

describe('шапка: «Экспорт» без разворотов (QA 120)', () => {
  const exportBtn = () => document.querySelector<HTMLElement>('[data-guide="topbar-export"]')!
  afterEach(() => {
    jobs.exportOpen = false
    store.toast = null
  })
  it('пустой альбом: кнопка серая с подсказкой, щелчок — тост, окно не открывается', async () => {
    show({})
    render(TopBar)
    const b = exportBtn()
    expect(b.getAttribute('aria-disabled')).toBe('true')
    expect(b.classList.contains('off')).toBe(true)
    expect(b.getAttribute('title')).toBe('Выгружать нечего: в альбоме нет разворотов')
    await fireEvent.click(b)
    await vi.waitFor(() => expect(store.toast?.text).toBe(EXPORT_EMPTY))
    expect(jobs.exportOpen).toBe(false)
  })
  it('есть разворот: кнопка обычная, щелчок открывает окно выгрузки без тоста', async () => {
    const sp = { id: 's1', grid: false, photos: [], rects: [], places: [], frames: [], texts: [], captions: [] }
    store.view = { rev: 1, folders: [], spreads: [sp], settings } as never
    render(TopBar)
    const b = exportBtn()
    expect(b.getAttribute('aria-disabled')).toBeNull()
    expect(b.getAttribute('title')).toBeNull()
    await fireEvent.click(b)
    await vi.waitFor(() => expect(jobs.exportOpen).toBe(true))
    expect(store.toast).toBeNull()
  })
})
