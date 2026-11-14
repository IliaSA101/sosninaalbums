// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Меню группы ленты (103 §2): «Вставить разворот перед / после». Go подменён — ответ собирается по аргументам.
const newSpread = vi.hoisted(() => vi.fn())
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ NewSpread: newSpread }))

const { default: Timeline } = await import('../timeline/Timeline.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { naming } = await import('../timeline/naming.svelte')
const { gridAct } = await import('../layouts/gridact.svelte')
const { spreadsel } = await import('../timeline/spreadsel.svelte')

const sp = (id: string) => ({ id, name: '', photos: [], rects: [], w: 400, h: 200, kind: '' })
const setView = (ids: string[], rev: number, settings = {}) => {
  store.view = { ...store.view, rev, spreads: ids.map(sp), settings } as never
}
const items = () => Array.from(document.querySelectorAll<HTMLElement>('[role="menuitem"]'))
const item = (text: string) => items().find((b) => b.textContent?.includes(text))
const groups = (c: HTMLElement) => Array.from(c.querySelectorAll<HTMLElement>('.group'))

beforeEach(() => {
  naming.stop()
  spreadsel.clear()
  prefs.level = 'full'
  setView(['a', 'b', 'c'], 1)
  store.currentId = 'a'
  store.onCover = false
  newSpread.mockImplementation(async (at: number) => {
    const ids = store.view.spreads.map((x) => x.id)
    ids.splice(at, 0, 'n')
    setView(ids, store.view.rev + 1)
    return store.view
  })
})
afterEach(() => {
  cleanup()
  newSpread.mockReset()
})

describe('лента: «Вставить разворот перед / после» (103)', () => {
  it('«перед» первой группы — NewSpread(0, []), новый — текущий; меню закрыто', async () => {
    const { container } = render(Timeline)
    await fireEvent.contextMenu(groups(container)[0])
    flushSync()
    await fireEvent.click(item('Вставить разворот перед')!)
    await vi.waitFor(() => expect(newSpread).toHaveBeenCalledExactlyOnceWith(0, []))
    await vi.waitFor(() => expect(store.currentId).toBe('n'))
    expect(store.view.spreads.map((x) => x.id)).toEqual(['n', 'a', 'b', 'c'])
    expect(document.querySelector('[role="menu"]')).toBeNull()
  })

  it('«после» средней группы — NewSpread(i+1, [])', async () => {
    const { container } = render(Timeline)
    await fireEvent.contextMenu(groups(container)[1])
    flushSync()
    await fireEvent.click(item('Вставить разворот после')!)
    await vi.waitFor(() => expect(newSpread).toHaveBeenCalledExactlyOnceWith(2, []))
  })

  it('«после» последней — в конец (at = число групп)', async () => {
    const { container } = render(Timeline)
    await fireEvent.contextMenu(groups(container)[2])
    flushSync()
    await fireEvent.click(item('Вставить разворот после')!)
    await vi.waitFor(() => expect(newSpread).toHaveBeenCalledExactlyOnceWith(3, []))
  })

  it('альбом листов — «Вставить лист …»', async () => {
    setView(['a', 'b'], 2, { sheet: true })
    const { container } = render(Timeline)
    await fireEvent.contextMenu(groups(container)[0])
    flushSync()
    expect(item('Вставить лист перед')).toBeDefined()
    expect(item('Вставить лист после')).toBeDefined()
    expect(item('Вставить разворот')).toBeUndefined()
  })

  it('две выделенные группы — пунктов вставки нет', async () => {
    const { container } = render(Timeline)
    await fireEvent.click(groups(container)[0], { ctrlKey: true })
    await fireEvent.click(groups(container)[1], { ctrlKey: true })
    await fireEvent.contextMenu(groups(container)[1])
    flushSync()
    expect(item('Вставить')).toBeUndefined()
    expect(item('Удалить развороты (2)')).toBeDefined()
  })

  it('Esc закрывает меню, ничего не вставив', async () => {
    const { container } = render(Timeline)
    await fireEvent.contextMenu(groups(container)[0])
    flushSync()
    expect(document.querySelector('[role="menu"]')).not.toBeNull()
    await fireEvent.keyDown(window, { key: 'Escape' })
    flushSync()
    expect(document.querySelector('[role="menu"]')).toBeNull()
    expect(newSpread).not.toHaveBeenCalled()
  })
})

describe('лента: «Склеить со следующим» у ручного разворота (103)', () => {
  const manual = (ids: string[], kinds: Record<string, object>) => {
    store.view = {
      ...store.view,
      rev: 5,
      spreads: ids.map((id) => ({ ...sp(id), photos: ['p1'], ...kinds[id] })),
    } as never
  }

  it('пункт активен, щелчок открывает вопрос и не зовёт Go', async () => {
    manual(['a', 'b', 'c'], { a: { grid: true } })
    const askGlue = vi.spyOn(gridAct, 'askGlue').mockResolvedValue(undefined)
    const { container } = render(Timeline)
    await fireEvent.contextMenu(groups(container)[0])
    flushSync()
    const join = item('Склеить со следующим') as HTMLButtonElement
    expect(join.disabled).toBe(false)
    await fireEvent.click(join)
    expect(askGlue).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ kind: 'join', ids: ['a', 'b'], title: 'Склеить развороты?' }))
    expect(document.querySelector('[role="menu"]')).toBeNull()
    askGlue.mockRestore()
  })

  it('виньетка в паре — пункт серый с причиной', async () => {
    manual(['a', 'b', 'c'], { b: { grid: true, kind: 'vignette' } })
    const { container } = render(Timeline)
    await fireEvent.contextMenu(groups(container)[0])
    flushSync()
    const join = item('Склеить со следующим') as HTMLButtonElement
    expect(join.disabled).toBe(true)
    expect(join.title).toBe('Виньетку не склеить — сначала «Обычный разворот» (правая кнопка)')
  })
})

describe('меню группы на общем MenuList (108)', () => {
  const cur = () => document.querySelector('.drop button.cur')?.textContent ?? ''

  it('открывается с фокусом на первом пункте; ↓/↑ ведут одну подсветку .cur', async () => {
    const { container } = render(Timeline)
    await fireEvent.contextMenu(groups(container)[0])
    flushSync()
    await vi.waitFor(() => expect(document.querySelectorAll('.drop button.cur')).toHaveLength(1))
    const first = cur()
    await fireEvent.keyDown(window, { key: 'ArrowDown' })
    flushSync()
    expect(document.querySelectorAll('.drop button.cur')).toHaveLength(1)
    expect(cur()).not.toBe(first)
    await fireEvent.keyDown(window, { key: 'ArrowUp' })
    flushSync()
    expect(cur()).toBe(first)
  })

  it('у каждого пункта есть ячейка значка (столбец слева; отметок в меню нет)', async () => {
    const { container } = render(Timeline)
    await fireEvent.contextMenu(groups(container)[0])
    flushSync()
    expect(items().length).toBeGreaterThan(0)
    expect(document.querySelectorAll('.drop button .ico svg')).toHaveLength(items().length)
    expect(document.querySelectorAll('.drop button .mark')).toHaveLength(0)
  })

  it('правый щелчок по другой группе заменяет меню, а не плодит второе', async () => {
    const { container } = render(Timeline)
    await fireEvent.contextMenu(groups(container)[0])
    flushSync()
    await fireEvent.contextMenu(groups(container)[1])
    flushSync()
    expect(document.querySelectorAll('[role="menu"]')).toHaveLength(1)
  })
})

describe('название из «Всех разворотов» не открывает поле в ленте (103-3)', () => {
  it('naming.where = grid — в ленте поля нет, номер на месте', () => {
    const { container } = render(Timeline)
    naming.start('b', 'grid')
    flushSync()
    expect(container.querySelector('input[aria-label="Название разворота"]')).toBeNull()
    expect(container.querySelectorAll('.group .num')).toHaveLength(3)
  })

  it('naming.where = strip — поле в ленте', () => {
    const { container } = render(Timeline)
    naming.start('b', 'strip')
    flushSync()
    expect(container.querySelector('input[aria-label="Название разворота"]')).not.toBeNull()
  })
})
