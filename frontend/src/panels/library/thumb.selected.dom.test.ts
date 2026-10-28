// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'
import thumbSrc from './Thumb.svelte?raw'

// Выделение в ленте фото (0.27-m-11): выбранные плитки несут класс selected (щелчок, Ctrl, Shift); вид класса — в стилях Thumb.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ WhereUsed: vi.fn().mockResolvedValue([]), SetMarks: vi.fn() }))

const { default: Thumb } = await import('./Thumb.svelte')
const { store } = await import('../../base/core/store.svelte')
const { wire } = await import('../../shell/boot/wire')
wire()

const photo = (id: string) => ({ id, name: `${id}.jpg`, path: `/p/${id}.jpg`, w: 3, h: 2 }) as never
const IDS = ['a', 'b', 'c', 'd']

beforeEach(() => {
  store.view = { ...store.view, rev: 1, spreads: [], folders: [] } as never
  store.photos = IDS.map(photo) as never
  store.selected.clear()
  store.anchor = null
})
afterEach(() => cleanup())

// Плитки ленты: каждая монтируется в свой узел body; возвращает поиск плитки и список выбранных по DOM.
function lenta() {
  for (const id of IDS) render(Thumb, { p: photo(id) })
  const tile = (id: string) => document.querySelector(`[data-thumb="${id}"]`) as HTMLElement
  const chosen = () => IDS.filter((id) => tile(id).classList.contains('selected'))
  return { tile, chosen }
}

describe('плитки ленты: класс выделения', () => {
  it('без выделения — ни одной выбранной', () => {
    const { chosen } = lenta()
    expect(chosen()).toEqual([])
  })

  it('обычный щелчок — выбрана одна; другой щелчок переносит выбор', async () => {
    const { tile, chosen } = lenta()
    await fireEvent.click(tile('b'))
    flushSync()
    expect(chosen()).toEqual(['b'])
    await fireEvent.click(tile('c'))
    flushSync()
    expect(chosen()).toEqual(['c'])
  })

  it('Ctrl+щелчок добавляет вторую и снимает по повторному', async () => {
    const { tile, chosen } = lenta()
    await fireEvent.click(tile('a'))
    await fireEvent.click(tile('c'), { ctrlKey: true })
    flushSync()
    expect(chosen()).toEqual(['a', 'c'])
    await fireEvent.click(tile('a'), { ctrlKey: true })
    flushSync()
    expect(chosen()).toEqual(['c'])
  })

  it('Shift+щелчок — диапазон', async () => {
    const { tile, chosen } = lenta()
    await fireEvent.click(tile('a'))
    await fireEvent.click(tile('c'), { shiftKey: true })
    flushSync()
    expect(chosen()).toEqual(['a', 'b', 'c'])
  })
})

describe('стили выбранной плитки', () => {
  const css = thumbSrc.slice(thumbSrc.indexOf('<style>'))
  const rule = (sel: string) => css.match(new RegExp(`${sel.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\s*\\{([^}]*)\\}`))?.[1] ?? ''

  it('рамка акцента 4 px и заливка акцентом поверх фото, мышь сквозь неё', () => {
    const r = rule('.thumb.selected .im::before')
    expect(r).toMatch(/border:\s*4px solid var\(--accent\)/)
    expect(r).toMatch(/background:\s*color-mix\(in srgb, var\(--accent\)/)
    expect(r).toMatch(/pointer-events:\s*none/)
  })

  it('слой — ::before: значки (✓, флажок, звёзды, профиль) идут в DOM позже и лежат над заливкой', () => {
    expect(css).not.toContain('.thumb.selected .im::after')
    const body = thumbSrc.replace(/<script[\s\S]*?<\/script>/, '').replace(/<style[\s\S]*$/, '')
    expect(body.indexOf('class="check"')).toBeGreaterThan(body.indexOf('class="im"'))
  })

  it('заглушка «нет файла» и полоса метки подстроены под рамку 4 px', () => {
    expect(rule('.thumb.selected .gone')).toMatch(/border:\s*4px solid var\(--accent\)/)
    expect(rule('.thumb.selected .strip')).toMatch(/bottom:\s*4px/)
  })
})
