// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/svelte'
import { flushSync } from 'svelte'
import { revealLabel } from '../../base/core/platform'

// Меню миниатюры ленты (106): правый щелчок выделяет, пункты от Go, переход, ошибки — тостом, закрытие по rev и Esc.
const m = vi.hoisted(() => ({
  whereUsed: vi.fn(),
  reveal: vi.fn(),
  goUse: vi.fn(),
  can: vi.fn((_f: string) => true),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({ WhereUsed: m.whereUsed, ShowPhotoInExplorer: m.reveal }))
vi.mock('../../editor/photoprops/gouse.svelte', () => ({ goUse: m.goUse }))

const { default: ThumbMenu } = await import('./ThumbMenu.svelte')
const { thumbmenu } = await import('./thumbmenustate.svelte')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { lost } = await import('./lost.svelte')

const photo = (id: string) => ({ id, name: `${id}.jpg`, w: 3, h: 2 })
const use = (i: number, page: string, what: string, kind = 'place') => ({
  sheet: i,
  label: `${page} — ${what}`,
  kind,
  spread: `s${i}`,
  page,
  what,
  pick: `p${i}`,
})
// Один спрос: фото A стоит дважды (число на плитке 2)
const sp = (id: string, ...photos: string[]) => ({ id, photos, grid: false })
const aUses = [use(1, 'Разворот 2', 'место'), use(4, 'Разворот 5', 'фон', 'backdrop')]

const event = (x = 40, y = 30) => new MouseEvent('contextmenu', { clientX: x, clientY: y, cancelable: true })
const items = () => [...document.body.querySelectorAll<HTMLElement>('.drop [data-item]')].map((b) => b.dataset.item)
const reveal = revealLabel()

beforeEach(() => {
  vi.spyOn(prefs, 'can').mockImplementation((f) => m.can(f))
  m.can.mockReturnValue(true)
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(200)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(200)
  window.innerWidth = 1000
  window.innerHeight = 700
  store.view = { ...store.view, rev: 1, spreads: [sp('s1', 'a'), sp('s2', 'a')], folders: [] } as never
  store.photos = [photo('a'), photo('b'), photo('c')] as never
  store.selected.clear()
  thumbmenu.close()
  m.whereUsed.mockResolvedValue([{ id: 'a', uses: aUses, places: 1, move: '' }])
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  vi.clearAllMocks()
  store.toast = null
})

const open = async (id: string, e = event()) => {
  render(ThumbMenu)
  thumbmenu.open(e, id)
  flushSync()
  await vi.waitFor(() => expect(document.body.querySelector('.drop')).not.toBeNull())
}

describe('ThumbMenu', () => {
  it('правый щелчок по невыделенному выделяет только его; события не доходят до браузерного меню', async () => {
    store.selected.add('b')
    const e = event()
    await open('a', e)
    expect(e.defaultPrevented).toBe(true)
    expect([...store.selected]).toEqual(['a'])
    expect(items()).toEqual(['explorer', 'copy', 'uses'])
    expect(screen.getByText(reveal)).toBeTruthy()
  })

  it('щелчок по выделенному оставляет выделение; несколько фото — только «Копировать имена файлов»', async () => {
    store.selected.add('a')
    store.selected.add('b')
    await open('a')
    expect([...store.selected].sort()).toEqual(['a', 'b'])
    expect(items()).toEqual(['copy'])
    expect(screen.getByText('Копировать имена файлов')).toBeTruthy()
  })

  it('«Где стоит ▸» — строки от WhereUsed; щелчок закрывает меню и ведёт к вхождению', async () => {
    await open('a')
    expect(m.whereUsed).toHaveBeenCalledExactlyOnceWith(['a'])
    await fireEvent.pointerEnter(screen.getByRole('menuitem', { name: /Где стоит/ }))
    await vi.waitFor(() => expect(screen.getByText('Разворот 5 — фон')).toBeTruthy())
    expect(screen.getByText('Разворот 2 — место')).toBeTruthy()
    await fireEvent.click(screen.getByText('Разворот 5 — фон'))
    expect(m.goUse).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ kind: 'backdrop', spread: 's4', pick: 'p4' }))
    expect(thumbmenu.menu).toBeNull()
    expect(document.body.querySelector('.drop')).toBeNull()
  })

  it('фото нигде не стоит — «Где стоит» нет, Go за вхождениями не зовём', async () => {
    await open('c')
    expect(items()).toEqual(['explorer', 'copy'])
    expect(m.whereUsed).not.toHaveBeenCalled()
  })

  it('«Показать в …» зовёт Go по ID ленты и закрывает меню; ошибка — тост', async () => {
    m.reveal.mockRejectedValueOnce(new Error('файл не найден'))
    await open('a')
    await fireEvent.click(screen.getByText(reveal))
    expect(m.reveal).toHaveBeenCalledExactlyOnceWith('a')
    expect(thumbmenu.menu).toBeNull()
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
  })

  it('пропавший файл: «Показать в …» и «Копировать» нет', async () => {
    lost.mark('a')
    await open('a')
    expect(items()).toEqual(['uses'])
    lost.clear()
  })

  it('ошибка WhereUsed — меню закрыто, тост', async () => {
    m.whereUsed.mockRejectedValueOnce(new Error('сбой'))
    await open('a')
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
    expect(thumbmenu.menu).toBeNull()
  })

  it('«Простой»: меню нет, плитка не выделяется', async () => {
    m.can.mockImplementation((f) => f !== 'photoMenu')
    render(ThumbMenu)
    thumbmenu.open(event(), 'a')
    flushSync()
    expect(document.body.querySelector('.drop')).toBeNull()
    expect(store.selected.size).toBe(0)
  })

  it('Esc, щелчок мимо и смена rev закрывают', async () => {
    await open('a')
    await fireEvent.keyDown(document.activeElement ?? document.body, { key: 'Escape' })
    expect(thumbmenu.menu).toBeNull()

    thumbmenu.open(event(), 'a')
    flushSync()
    await fireEvent.pointerDown(document.body)
    expect(thumbmenu.menu).toBeNull()

    thumbmenu.open(event(), 'a')
    flushSync()
    expect(thumbmenu.menu).not.toBeNull()
    store.view = { ...store.view, rev: 2 } as never
    flushSync()
    expect(thumbmenu.menu).toBeNull()
  })

  it('↑/↓ ходят по пунктам', async () => {
    await open('a')
    const first = document.activeElement as HTMLElement
    await vi.waitFor(() => expect(first?.dataset.item).toBe('explorer'))
    await fireEvent.keyDown(first, { key: 'ArrowDown' })
    expect((document.activeElement as HTMLElement).dataset.item).toBe('copy')
    await fireEvent.keyDown(document.activeElement as Element, { key: 'ArrowUp' })
    expect((document.activeElement as HTMLElement).dataset.item).toBe('explorer')
  })

  it('поздний ответ закрытого меню отбрасывается', async () => {
    let done: (v: unknown) => void = () => {}
    m.whereUsed.mockReturnValueOnce(new Promise((r) => (done = r)))
    await open('a')
    thumbmenu.close()
    done([{ id: 'a', uses: aUses, places: 1, move: '' }])
    await Promise.resolve()
    expect(thumbmenu.uses).toBeNull()
  })
})
