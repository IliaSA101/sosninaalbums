// @vitest-environment happy-dom
import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { createRawSnippet } from 'svelte'
import TplItem from './TplItem.svelte'

// 163: stack — имя и пометка двумя строками, имя целиком (в одну строку с «…» его не режем); без stack — прежняя одна строка.
const children = createRawSnippet(() => ({ render: () => '<div class="card"></div>' }))
const q = (s: string) => document.querySelector<HTMLElement>(s)

afterEach(cleanup)

describe('TplItem stack', () => {
  it('имя и пометка — отдельные строки, полное имя в подсказке', () => {
    render(TplItem, { name: 'Свадьба: синяя акварель', note: 'от 8 фото · 4 разворота', stack: true, children })
    expect(q('.sname')?.textContent).toBe('Свадьба: синяя акварель')
    expect(q('.snote')?.textContent).toBe('от 8 фото · 4 разворота')
    expect(q('.sname')?.title).toBe('Свадьба: синяя акварель · от 8 фото · 4 разворота')
    expect(q('.line1')).toBeNull()
    expect(q('.item')?.classList.contains('named')).toBe(false) // минимальная ширина прежней подписи не нужна
  })
  it('без пометки — только имя; без имени — только пометка с подсказкой', () => {
    const a = render(TplItem, { name: 'Один', stack: true, children })
    expect(q('.sname')?.textContent).toBe('Один')
    expect(q('.snote')).toBeNull()
    a.unmount()
    render(TplItem, { note: 'набор 12 МБ', stack: true, children })
    expect(q('.sname')).toBeNull()
    expect(q('.snote')?.title).toBe('набор 12 МБ')
  })
  it('tone красит пометку', () => {
    render(TplItem, { name: 'А', note: 'мелко', tone: 'small', stack: true, children })
    expect(q('.snote')?.classList.contains('small')).toBe(true)
  })
  it('без stack — прежняя одна строка', () => {
    render(TplItem, { name: 'А', note: 'б', children })
    expect(q('.line1')?.textContent).toContain('А')
    expect(q('.sname')).toBeNull()
    expect(q('.item')?.classList.contains('noted')).toBe(true)
  })
})
