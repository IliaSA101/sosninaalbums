// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { createRawSnippet } from 'svelte'
import Seg, { type SegItem } from './Seg.svelte'

// Необязательные свойства общего Seg (часть 2 техдолга): значок, aria пункта, колонки, растяжение, действия, фокус, свой пункт.
type Id = 'a' | 'b' | 'c'
const ITEMS: SegItem<Id>[] = [
  { id: 'a', label: 'Раз', title: 'Первый', icon: 'layer-front', aria: 'Один' },
  { id: 'b', label: 'Два' },
  { id: 'c', label: 'Три', off: true },
]

afterEach(cleanup)

const seg = () => screen.getByRole('group', { name: 'Тест' })

describe('Seg: необязательные свойства', () => {
  it('без новых свойств — прежний вид: aria-pressed, без значка и классов', () => {
    render(Seg, { items: ITEMS.slice(1), value: 'b', onpick: () => {}, label: 'Тест' })
    expect(screen.getByRole('button', { name: 'Два' }).getAttribute('aria-pressed')).toBe('true')
    expect(seg().className).not.toMatch(/grow|big|act|grid/)
    expect(seg().querySelector('svg')).toBeNull()
    expect(screen.getByRole('button', { name: 'Два' }).hasAttribute('aria-label')).toBe(false)
  })

  it('значок рисуется перед подписью, aria пункта становится именем', async () => {
    const onpick = vi.fn()
    render(Seg, { items: ITEMS, value: 'b', onpick, label: 'Тест' })
    const b = screen.getByRole('button', { name: 'Один' })
    expect(b.querySelector('svg')).not.toBeNull()
    expect(b.textContent?.trim()).toBe('Раз')
    expect(b.getAttribute('title')).toBe('Первый')
    await fireEvent.click(b)
    expect(onpick).toHaveBeenCalledWith('a')
  })

  it('iconSize задаёт размер значка', () => {
    render(Seg, { items: ITEMS, value: 'b', onpick: () => {}, label: 'Тест', iconSize: 14 })
    expect(seg().querySelector('svg')?.getAttribute('width')).toBe('14')
  })

  it('action: aria-pressed нет, щелчок по любому зовёт onpick, серый — нет', async () => {
    const onpick = vi.fn()
    render(Seg, { items: ITEMS, onpick, label: 'Тест', action: true })
    for (const b of screen.getAllByRole('button')) expect(b.hasAttribute('aria-pressed')).toBe(false)
    await fireEvent.click(screen.getByRole('button', { name: 'Два' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Три' }))
    expect(onpick.mock.calls).toEqual([['b']])
  })

  it('cols: сетка в N столбцов', () => {
    render(Seg, { items: ITEMS, onpick: () => {}, label: 'Тест', cols: 2 })
    expect(seg().style.gridTemplateColumns).toBe('repeat(2, 1fr)')
    expect(seg().className).toContain('grid')
  })

  it('grow и big добавляют свои классы группе', () => {
    render(Seg, { items: ITEMS, onpick: () => {}, label: 'Тест', grow: true, big: true })
    expect(seg().className).toMatch(/grow/)
    expect(seg().className).toMatch(/big/)
  })

  it('keepFocus: mousedown отменяется; без него — нет', async () => {
    const r = render(Seg, { items: ITEMS, onpick: () => {}, label: 'Тест', keepFocus: true })
    const b = screen.getByRole('button', { name: 'Два' })
    expect(await fireEvent.mouseDown(b)).toBe(false)
    r.unmount()
    render(Seg, { items: ITEMS, onpick: () => {}, label: 'Тест' })
    expect(await fireEvent.mouseDown(screen.getByRole('button', { name: 'Два' }))).toBe(true)
  })

  it('item без aria у пункта: имя кнопки — label', () => {
    const item = createRawSnippet<[SegItem<string | number>]>(() => ({ render: () => '<i class="mark">*</i>' }))
    render(Seg, { items: ITEMS, onpick: () => {}, label: 'Тест', item })
    expect(screen.getByRole('button', { name: 'Два' }).getAttribute('aria-label')).toBe('Два')
  })

  it('grow вместе с look=card: оба класса на группе', () => {
    render(Seg, { items: ITEMS, onpick: () => {}, label: 'Тест', look: 'card', grow: true })
    expect(seg().classList.contains('card')).toBe(true)
    expect(seg().classList.contains('grow')).toBe(true)
  })

  it('item: свой пункт вместо подписи; имя — из aria', async () => {
    const item = createRawSnippet<[SegItem<string | number>]>((it) => ({ render: () => `<i class="mark">${it().id}!</i>` }))
    const onpick = vi.fn()
    render(Seg, { items: ITEMS, onpick, label: 'Тест', item })
    const b = screen.getByRole('button', { name: 'Один' })
    expect(b.querySelector('.mark')?.textContent).toBe('a!')
    expect(b.textContent).not.toContain('Раз')
    await fireEvent.click(b)
    expect(onpick).toHaveBeenCalledWith('a')
  })
})
