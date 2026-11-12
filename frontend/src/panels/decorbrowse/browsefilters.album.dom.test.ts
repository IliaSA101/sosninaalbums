// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import BrowseFilters from './BrowseFilters.svelte'

// 163: для макетов (kind album) ряда «Вид» нет, «Стиль» подписан по-своему; без kind — прежний ряд.
const br = {
  packs: [],
  forms: [
    { id: 'sticker', label: 'Стикеры' },
    { id: 'corner', label: 'Уголки' },
  ],
  styles: [
    { id: 'А', label: 'А' },
    { id: 'Б', label: 'Б' },
  ],
  sel: { pack: '', form: '' as const, style: '' },
}

afterEach(cleanup)

describe('BrowseFilters: kind', () => {
  it('без kind — «Вид» и «Стиль стикеров»', () => {
    render(BrowseFilters, { br, onpick: vi.fn() })
    expect(document.body.textContent).toContain('Вид')
    expect(document.querySelector('[aria-label="Стиль стикеров"]')).not.toBeNull()
  })
  it('album — «Вида» нет, стиль макетов', () => {
    render(BrowseFilters, { br, onpick: vi.fn(), kind: 'album' })
    expect(document.body.textContent).not.toContain('Вид')
    expect(document.querySelector('[aria-label="Вид стикеров"]')).toBeNull()
    expect(document.querySelector('[aria-label="Стиль макетов"]')).not.toBeNull()
  })
  it('album с одними видами — ряда нет совсем', () => {
    render(BrowseFilters, { br: { ...br, styles: [] }, onpick: vi.fn(), kind: 'album' })
    expect(document.querySelector('.filters')).toBeNull()
  })
})
