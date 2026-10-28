// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Клавиши и «Лучший» в окне сравнения (138-8): отметка — активному кадру, Ctrl+Z доходит, Enter — один PickBest,
// без функции «Отметки» Enter молчит и кнопки нет, Delete убирает, Tab меняет активный, Ctrl+V, B, F молчат.
const m = vi.hoisted(() => ({ pick: vi.fn(), undo: vi.fn(), redo: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => ({
  ...(await orig<typeof import('../../../wailsjs/go/app/App')>()),
  PickBest: m.pick,
  Undo: m.undo,
  Redo: m.redo,
}))

const { default: PhotoCompare } = await import('./PhotoCompare.svelte')
const { compare } = await import('./compare.svelte')
const { photoui } = await import('../../editor/photo/photoui.svelte')
const { keyCommand, preventsDefault } = await import('../../shell/menu/keys')
const { runKey } = await import('../../shell/menu/commands')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { ownMarks } = await import('../marks/ownmarks.svelte')
const { clip } = await import('../../editor/clip/clip.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')
const { useCatalog, dropCatalog } = await import('../../base/keys/keymap.fixture')

const IDS = ['a', 'b', 'c', 'd', 'e']
const photo = (id: string) => ({ id, name: `${id}.jpg`, w: 3, h: 2 })

// То же, что App.keys(): контекст из живых состояний; слушатель в фазе всплытия — клавиши, погашенные окном, сюда не доходят.
const reached: string[] = []
function app(e: KeyboardEvent) {
  reached.push(e.code)
  const id = keyCommand(e, {
    inTextField: false,
    onCheckbox: false,
    galleryOpen: false,
    hasSelection: store.selectedIds.length > 0,
    pasteable: true,
    copyable: true,
    zone: store.zone,
    dialog: modalStack.any() ? (modalStack.topUndo() ? 'undo' : 'shut') : undefined,
    preview: photoui.preview?.from,
    compare: compare.open,
    can: prefs.can,
  })
  if (!id) return
  if (preventsDefault(id, e)) e.preventDefault()
  runKey(id, e)
}

function key(code: string, init: KeyboardEventInit = {}): KeyboardEvent {
  const e = new KeyboardEvent('keydown', { code, key: init.key ?? code, bubbles: true, cancelable: true, ...init })
  ;(document.querySelector('[role=dialog]') ?? document.body).dispatchEvent(e)
  flushSync()
  return e
}
const ctrl = { ctrlKey: true }

let marks: ReturnType<typeof vi.spyOn>
let history: ReturnType<typeof vi.spyOn>
let paste: ReturnType<typeof vi.spyOn>
beforeEach(() => {
  useCatalog()
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(1200)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(700)
  store.photos = IDS.map(photo) as never
  store.selected.clear()
  for (const id of ['b', 'c', 'd']) store.selected.add(id)
  store.anchor = 'b'
  store.zone = 'library'
  store.toast = null
  reached.length = 0
  m.pick.mockReset()
  marks = vi.spyOn(ownMarks, 'setMarks').mockResolvedValue(true)
  history = vi.spyOn(store, 'runHistory').mockResolvedValue(true)
  paste = vi.spyOn(clip, 'paste').mockImplementation(() => {})
  vi.spyOn(store, 'run').mockResolvedValue(true)
  window.addEventListener('keydown', app)
})
afterEach(() => {
  window.removeEventListener('keydown', app)
  compare.close()
  cleanup()
  dropCatalog()
  vi.restoreAllMocks()
})

const open = () => {
  render(PhotoCompare)
  expect(compare.openByKey()).toBe(true)
  flushSync()
}
const lastHost = () => marks.mock.calls.at(-1)![0] as { selectedIds: string[] }
const tiles = () => [...document.querySelectorAll<HTMLElement>('.cell')]

describe('отметки под сравнением', () => {
  it('P идёт активному кадру, выделение ленты не меняется', () => {
    open()
    key('KeyP')
    expect(marks.mock.calls[0][1]).toEqual({ flag: 1 })
    expect(lastHost().selectedIds).toEqual(['b'])
    expect([...store.selected].sort()).toEqual(['b', 'c', 'd'])
  })

  it('после Tab отметка идёт новому активному', () => {
    open()
    key('Tab', { key: 'Tab' })
    key('Digit4')
    expect(lastHost().selectedIds).toEqual(['c'])
    key('Tab', { key: 'Tab', shiftKey: true })
    expect(compare.activeId).toBe('b')
  })

  it('щелчок по ячейке делает её активной', () => {
    open()
    fireEvent.pointerDown(tiles()[2], { button: 0, pointerId: 1 })
    flushSync()
    expect(compare.activeId).toBe('d')
    expect(tiles()[2].classList.contains('active')).toBe(true)
  })

  it('0 не «Звёзды: убрать»: окну достаётся «вписать»', () => {
    open()
    expect(key('Digit0').defaultPrevented).toBe(true)
    expect(marks).not.toHaveBeenCalled()
    expect(reached).toEqual([])
  })

  it('Ctrl+Z и Ctrl+Y доходят до отмены', async () => {
    open()
    key('KeyZ', ctrl)
    key('KeyY', ctrl)
    await vi.waitFor(() => expect(history).toHaveBeenCalledTimes(2))
    expect(m.undo).toHaveBeenCalledOnce()
    expect(m.redo).toHaveBeenCalledOnce()
  })
})

describe('«Лучший»', () => {
  it('Enter — один PickBest(активный, остальные), окно закрыто, выделен лучший, тост', async () => {
    open()
    key('Tab', { key: 'Tab' })
    key('Enter', { key: 'Enter' })
    await vi.waitFor(() => expect(compare.open).toBe(false))
    expect(m.pick).toHaveBeenCalledExactlyOnceWith('c', ['b', 'd'])
    expect([...store.selected]).toEqual(['c'])
    expect(store.anchor).toBe('c')
    expect(store.toast?.text).toMatch(/^Лучший — c\.jpg; отклонено: 2/)
    expect(modalStack.any()).toBe(false)
  })

  it('кнопка «Лучший» в ячейке — то же, для своего кадра', async () => {
    open()
    const btn = tiles()[2].querySelector<HTMLButtonElement>('.accent')!
    expect(btn.textContent?.trim()).toBe('Лучший')
    expect(btn.tabIndex).toBe(-1)
    await fireEvent.click(btn)
    await vi.waitFor(() => expect(compare.open).toBe(false))
    expect(m.pick).toHaveBeenCalledExactlyOnceWith('d', ['b', 'c'])
  })

  it('ошибка Go: окно остаётся', async () => {
    vi.mocked(store.run).mockResolvedValue(false)
    open()
    key('Enter', { key: 'Enter' })
    await Promise.resolve()
    expect(compare.open).toBe(true)
    expect(store.selectedIds).toEqual(['b', 'c', 'd'])
  })

  it('без функции «Отметки»: кнопки нет, Enter молчит и погашен', () => {
    vi.spyOn(prefs, 'can').mockImplementation((f) => f !== 'marks')
    open()
    expect(tiles()[0].querySelector('.accent')).toBeNull()
    const e = key('Enter', { key: 'Enter' })
    expect(e.defaultPrevented).toBe(true)
    expect(m.pick).not.toHaveBeenCalled()
    expect(compare.open).toBe(true)
    expect(reached).toEqual([])
  })
})

describe('Delete, Esc, C, стрелки', () => {
  it('Delete убирает активный кадр, отметки не трогает', () => {
    open()
    key('Delete', { key: 'Delete' })
    expect(compare.ids).toEqual(['c', 'd'])
    expect(marks).not.toHaveBeenCalled()
    expect(m.pick).not.toHaveBeenCalled()
  })

  it('убрали до одного — окно закрыто, выделен оставшийся', () => {
    store.selected.delete('d')
    open()
    key('Delete', { key: 'Delete' })
    expect(compare.open).toBe(false)
    expect([...store.selected]).toEqual(['c'])
    expect(store.anchor).toBe('c')
  })

  it('✕ ячейки убирает её', async () => {
    open()
    await fireEvent.click(tiles()[1].querySelector<HTMLButtonElement>('.sq')!)
    expect(compare.ids).toEqual(['b', 'd'])
  })

  it('Esc закрывает, не доходя до программы; выделение — оставшиеся кадры', () => {
    open()
    key('Delete', { key: 'Delete' })
    const e = key('Escape', { key: 'Escape' })
    expect(e.defaultPrevented).toBe(true)
    expect(reached).toEqual([])
    expect(compare.open).toBe(false)
    expect([...store.selected].sort()).toEqual(['c', 'd'])
    expect(modalStack.any()).toBe(false)
  })

  it('C закрывает; переназначенная клавиша — вместо C', () => {
    useCatalog({ photoCompare: ['KeyJ'] })
    open()
    expect(key('KeyJ').defaultPrevented).toBe(true)
    expect(compare.open).toBe(false)
    open()
    key('KeyC')
    expect(compare.open).toBe(true) // C больше не её: идёт диспетчеру, под окном молчит
  })

  it('← → меняют кадр в активной ячейке; Shift+стрелка идёт дальше', () => {
    open()
    key('ArrowRight', { key: 'ArrowRight' })
    expect(compare.ids).toEqual(['e', 'c', 'd'])
    key('ArrowLeft', { key: 'ArrowLeft', shiftKey: true })
    expect(reached).toEqual(['ArrowLeft'])
  })
})

describe('остальные клавиши молчат', () => {
  it('Ctrl+V не вставляет, B, F, Пробел ничего не делают', () => {
    open()
    store.zone = 'canvas'
    key('KeyV', ctrl)
    key('KeyB')
    key('KeyF')
    key('Space', { key: ' ' })
    expect(paste).not.toHaveBeenCalled()
    expect(compare.open).toBe(true)
    expect(compare.ids).toHaveLength(3)
  })

  it('после закрытия клавиши программы снова работают', () => {
    open()
    key('Escape', { key: 'Escape' })
    key('KeyP')
    expect(marks).toHaveBeenCalledOnce()
    expect(lastHost()).toBe(store)
  })

  it('F1 под окном молчит: сравнение стоит в списке «других окон» справочника', async () => {
    const { readFileSync } = await import('node:fs')
    const appSrc = readFileSync(`${import.meta.dirname}/../../App.svelte`, 'utf8')
    expect(appSrc).toMatch(/const other =[\s\S]*?compare\.open/)
    expect(appSrc).toContain('compare: compare.open')
  })
})
