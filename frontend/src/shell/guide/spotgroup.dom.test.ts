// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { createRawSnippet } from 'svelte'

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))

const { default: CardGroup } = await import('../../editor/inspector/CardGroup.svelte')
const { cardGroups, cardContext } = await import('../../base/prefs/cardgroups.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { spot, seek, unfoldFor } = await import('./spotstate.svelte')

// F1 «Показать» и тур к элементу в свёрнутой группе (спека 124 §4.4, 0.26-124-19): группа раскрывается и запоминается,
// а нижний элемент остаётся в видимой области, хотя группа растёт 0,45 с и первая прокрутка упёрлась бы в старый предел.
const later = (ms: number) => new Promise<void>((r) => setTimeout(r, ms))
const VIEW = 300 // высота видимой части панели
const BODY = 600 // высота раскрытого тела группы
const GROW = 450 // --move

const anchor = (name: string) => createRawSnippet(() => ({ render: () => `<div><button data-guide="${name}">x</button></div>` }))
const ctx = (docked: boolean) => ({ context: new Map([cardContext(() => docked)]) })

// Панель с прокруткой, где высота тела растёт по часам, а предел прокрутки считается от текущей высоты — как в браузере.
function mountPanel(docked: boolean, id: 'photo.about' | 'photo.frame' = 'photo.about') {
  const aside = document.body.appendChild(document.createElement('aside'))
  aside.style.overflowY = 'auto'
  render(CardGroup, { target: aside, props: { id, title: 'О снимке', children: anchor('bottom-x') } as never, ...ctx(docked) })
  const body = aside.querySelector<HTMLElement>('.body')!
  const target = aside.querySelector<HTMLElement>('[data-guide="bottom-x"]')!
  let top = 0
  let openedAt = 0
  const grown = () => {
    if (body.hasAttribute('inert')) return (openedAt = 0)
    openedAt ||= performance.now()
    return Math.min(1, (performance.now() - openedAt) / GROW)
  }
  const total = () => 22 + BODY * grown()
  Object.defineProperty(aside, 'scrollHeight', { get: total })
  Object.defineProperty(aside, 'clientHeight', { get: () => VIEW })
  Object.defineProperty(aside, 'scrollTop', {
    get: () => top,
    set: (v: number) => (top = Math.min(Math.max(0, v), Math.max(0, total() - VIEW))),
  })
  const rect = (t: number, h: number) => ({
    left: 0,
    right: 80,
    width: 80,
    top: t,
    bottom: t + h,
    height: h,
    x: 0,
    y: t,
    toJSON: () => ({}),
  })
  aside.getBoundingClientRect = () => rect(0, VIEW) as DOMRect
  target.getBoundingClientRect = () => rect(22 + BODY * grown() - 20 - top, 20) as DOMRect
  return { scroller: aside, target }
}

beforeEach(() => {
  localStorage.clear()
  cardGroups.reset()
  vi.spyOn(prefs, 'can').mockReturnValue(true)
  Object.defineProperty(window, 'innerWidth', { value: 1200, configurable: true })
  Object.defineProperty(window, 'innerHeight', { value: 700, configurable: true })
})
afterEach(() => {
  cleanup()
  spot.cancel()
  document.body.innerHTML = ''
  vi.restoreAllMocks()
})

describe('F1 «Показать» к элементу в свёрнутой группе', () => {
  it('раскрывает группу, запоминает открытой и подводит нижний элемент в видимую область', async () => {
    const m = mountPanel(true)
    cardGroups.toggle('d', 'photo.about')
    expect(cardGroups.isOpen('d', 'photo.about')).toBe(false)

    const found = await seek('ui:bottom-x')
    expect(found?.el).toBe(m.target)
    expect(cardGroups.isOpen('d', 'photo.about')).toBe(true)
    expect(JSON.parse(localStorage.getItem('cardGroups')!)).toEqual({ d: [], f: [] })

    await later(GROW + 400) // рост и подводка по кадрам (700 мс)
    const r = m.target.getBoundingClientRect()
    const area = m.scroller.getBoundingClientRect()
    expect(r.top).toBeGreaterThanOrEqual(area.top)
    expect(r.bottom).toBeLessThanOrEqual(area.bottom)
  })

  it('уже открытая группа: память не пишется, подсветку ставит вызывающий', async () => {
    const m = mountPanel(true)
    localStorage.removeItem('cardGroups')
    const found = await seek('ui:bottom-x')
    expect(found?.el).toBe(m.target)
    expect(localStorage.getItem('cardGroups')).toBeNull()
    expect(spot.cur).toBeNull() // seek только находит, кольцо ставит вызывающий
  })

  it('раскрывает группу своего хоста: плавающая карточка не трогает память панели', async () => {
    mountPanel(false)
    cardGroups.toggle('f', 'photo.about')
    cardGroups.toggle('d', 'photo.about')
    await seek('ui:bottom-x')
    expect(cardGroups.isOpen('f', 'photo.about')).toBe(true)
    expect(cardGroups.isOpen('d', 'photo.about')).toBe(false)
  })

  it('нет такого места — null, ничего не раскрыто', async () => {
    mountPanel(true)
    cardGroups.toggle('d', 'photo.about')
    expect(await seek('ui:no-such-place')).toBeNull()
    expect(cardGroups.isOpen('d', 'photo.about')).toBe(false)
  })
})

describe('unfoldFor', () => {
  it('элемент вне групп и чужой data-group не раскрывают ничего', () => {
    document.body.innerHTML =
      '<div data-guide="a"></div><section data-group="old.group" data-group-host="d"><i data-guide="b"></i></section>'
    cardGroups.toggle('d', 'photo.frame')
    expect(unfoldFor('[data-guide="a"]')).toBe(false)
    expect(unfoldFor('[data-guide="b"]')).toBe(false)
    expect(cardGroups.isOpen('d', 'photo.frame')).toBe(false)
  })

  it('раскрывает только группы с найденными элементами', () => {
    document.body.innerHTML =
      '<section data-group="photo.frame" data-group-host="d"><i data-guide="a"></i></section><section data-group="photo.tone" data-group-host="d"><i data-guide="b"></i></section>'
    cardGroups.toggle('d', 'photo.frame')
    cardGroups.toggle('d', 'photo.tone')
    expect(unfoldFor('[data-guide="a"]')).toBe(true)
    expect(cardGroups.isOpen('d', 'photo.frame')).toBe(true)
    expect(cardGroups.isOpen('d', 'photo.tone')).toBe(false)
    expect(unfoldFor('[data-guide="a"]')).toBe(false) // уже открыта
  })
})
