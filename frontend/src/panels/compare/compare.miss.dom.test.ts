// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Заглушка ячейки сравнения (138): подсказка как в ленте и на развороте — «Не читается: …» у битого файла,
// «Файл не найден: …» у отсутствующего (в окне 128 так же, PhotoPreview).
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

const { default: PhotoCompare } = await import('./PhotoCompare.svelte')
const { compare } = await import('./compare.svelte')
const { store } = await import('../../base/core/store.svelte')
const { useCatalog, dropCatalog } = await import('../../base/keys/keymap.fixture')

beforeEach(() => {
  useCatalog()
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(1200)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(700)
  store.view = { ...store.view, rev: 1, spreads: [], folders: [], missing: [] } as never
  store.unread = {}
  store.photos = ['good', 'bad', 'gone'].map((n) => ({ id: n, name: `${n}.jpg`, w: 4000, h: 3000 })) as never
  store.selected.clear()
  for (const id of ['good', 'bad', 'gone']) store.selected.add(id)
})
afterEach(() => {
  compare.close()
  store.unread = {}
  store.missing = []
  cleanup()
  dropCatalog()
  vi.restoreAllMocks()
})

const open = () => {
  render(PhotoCompare)
  compare.openByKey()
  flushSync()
}
const imgOf = (id: string) => document.querySelector<HTMLImageElement>(`.cell img.ph:not(.orig)[src*="${id}"]`)!
const miss = (i: number) => document.querySelectorAll<HTMLElement>('.cell')[i]?.querySelector<HTMLElement>('.miss')

describe('подсказка заглушки ячейки', () => {
  it('битый файл — «Не читается: причина», как в ленте', () => {
    store.unread = { bad: 'повреждённый JPEG' }
    open()
    const m = miss(1)!
    expect(m.title).toBe('Не читается: повреждённый JPEG')
    expect(m.getAttribute('aria-label')).toBe('Не читается: повреждённый JPEG')
  })

  it('отсутствующий файл — «Файл не найден: имя»', () => {
    open()
    fireEvent.error(imgOf('gone'))
    flushSync()
    const m = miss(2)!
    expect(m.title).toBe('Файл не найден: gone.jpg')
    expect(m.getAttribute('aria-label')).toBe('Файл не найден: gone.jpg')
  })

  it('битый файл по store.unread — заглушка сразу, без ошибки загрузки превью (как в ленте)', () => {
    store.unread = { bad: 'повреждённый JPEG' }
    open()
    expect(imgOf('bad')).toBeNull()
    expect(miss(1)?.title).toBe('Не читается: повреждённый JPEG')
    expect(compare.gone('bad')).toBe(true)
  })

  it('пропавший по store.missing — заглушка сразу, целый — нет', () => {
    store.missing = ['gone']
    open()
    expect(miss(2)?.title).toBe('Файл не найден: gone.jpg')
    expect(compare.gone('good')).toBe(false)
  })

  it('целый кадр — без заглушки', () => {
    store.unread = { bad: 'повреждённый JPEG' }
    open()
    expect(miss(0)).toBeNull()
  })
})
