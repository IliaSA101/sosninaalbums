// @vitest-environment happy-dom
import { readFileSync } from 'node:fs'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Клавиши под окном крупного просмотра из ленты (128-5): окно гасит свои, остальное идёт диспетчеру как в App.svelte.
const m = vi.hoisted(() => ({ api: vi.fn(), undo: vi.fn(), redo: vi.fn(), toggle: vi.fn(), remove: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => ({
  ...(await orig<typeof import('../../../wailsjs/go/app/App')>()),
  SetMarks: m.api,
  Undo: m.undo,
  Redo: m.redo,
}))
vi.mock('../../panels/bins/binact', async (orig) => ({
  ...(await orig<typeof import('../../panels/bins/binact')>()),
  toggleSelected: m.toggle,
  removeSelected: m.remove,
}))

const { default: PhotoPreview } = await import('./PhotoPreview.svelte')
const { photoui } = await import('./photoui.svelte')
const { keyCommand, preventsDefault } = await import('../../shell/menu/keys')
const { runKey } = await import('../../shell/menu/commands')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { modes } = await import('../../base/prefs/view.svelte')
const { gallery } = await import('../../panels/gallery/gallery.svelte')
const { ownMarks } = await import('../../panels/marks/ownmarks.svelte')
const { clip } = await import('../clip/clip.svelte')
const { canvas } = await import('../canvas/canvas.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')
const { useCatalog, dropCatalog } = await import('../../base/keys/keymap.fixture')

const photo = (id: string) => ({ id, name: `${id}.jpg`, w: 3, h: 2 })

// То же, что App.keys(): контекст из живых состояний и вызов команды. Слушатель на window в фазе всплытия,
// как svelte:window onkeydown, поэтому клавиши, погашенные окном (stopPropagation в захвате), сюда не доходят.
const reached: string[] = []
function app(e: KeyboardEvent) {
  reached.push(e.code)
  const id = keyCommand(e, {
    inTextField: false,
    onCheckbox: false,
    galleryOpen: !!gallery.cur,
    hasSelection: store.selectedIds.length > 0,
    pasteable: true,
    copyable: true,
    zone: store.zone,
    big: modes.big,
    dialog: modalStack.any() ? (modalStack.topUndo() ? 'undo' : 'shut') : undefined,
    preview: photoui.preview?.from,
    can: prefs.can,
  })
  if (!id) return
  if (preventsDefault(id, e)) e.preventDefault()
  runKey(id, e)
}

// Клавиша в окно: событие уходит на элемент окна, всплывает через window.
function key(code: string, init: KeyboardEventInit = {}): KeyboardEvent {
  const e = new KeyboardEvent('keydown', { code, key: init.key ?? code, bubbles: true, cancelable: true, ...init })
  ;(document.querySelector('[role=dialog]') ?? document.body).dispatchEvent(e)
  flushSync() // эффекты окна (стек окон) выполняются после события
  return e
}
const ctrl = { ctrlKey: true }

let marks: ReturnType<typeof vi.spyOn>
let history: ReturnType<typeof vi.spyOn>
let paste: ReturnType<typeof vi.spyOn>
beforeEach(() => {
  useCatalog()
  store.photos = ['a', 'b', 'c', 'd'].map(photo) as never
  store.selected.clear()
  for (const id of ['a', 'b', 'c', 'd']) store.selected.add(id)
  store.anchor = 'a'
  store.zone = 'library'
  store.toast = null
  reached.length = 0
  marks = vi.spyOn(ownMarks, 'setMarks').mockResolvedValue(true)
  history = vi.spyOn(store, 'runHistory').mockResolvedValue(true)
  paste = vi.spyOn(clip, 'paste').mockImplementation(() => {})
  m.toggle.mockClear()
  m.remove.mockClear()
  window.addEventListener('keydown', app)
})
afterEach(() => {
  window.removeEventListener('keydown', app)
  cleanup()
  photoui.preview = null
  modes.big = false
  gallery.cur = null
  dropCatalog()
  vi.restoreAllMocks()
})

const open = (id = 'c') => {
  render(PhotoPreview)
  photoui.openLibrary(id)
  flushSync()
}
const lastHost = () => marks.mock.calls.at(-1)![0] as { selectedIds: string[] }

describe('отметки под окном из ленты', () => {
  it('P ставит флажок показанному фото, выделение из четырёх не меняется', () => {
    open('c')
    const e = key('KeyP')
    expect(e.defaultPrevented).toBe(false)
    expect(marks).toHaveBeenCalledOnce()
    expect(marks.mock.calls[0][1]).toEqual({ flag: 1 })
    expect(lastHost().selectedIds).toEqual(['c'])
    expect([...store.selected].sort()).toEqual(['a', 'b', 'c', 'd'])
  })

  it('листание не меняет выделение, отметка идёт новому показанному', () => {
    open('c')
    key('ArrowRight', { key: 'ArrowRight' })
    expect(photoui.previewId).toBe('d')
    key('Digit4')
    expect(marks.mock.calls[0][1]).toEqual({ rating: 4 })
    expect(lastHost().selectedIds).toEqual(['d'])
    expect([...store.selected].sort()).toEqual(['a', 'b', 'c', 'd'])
  })

  it('и когда последнее нажатие мыши было на холсте (зона canvas)', () => {
    open('c')
    store.zone = 'canvas'
    key('KeyP')
    expect(lastHost().selectedIds).toEqual(['c'])
  })

  it('без подмены ownMarks: Go получает SetMarks(["c"], …) через store.run', () => {
    const run = vi.spyOn(store, 'run').mockResolvedValue(true)
    marks.mockRestore()
    m.api.mockResolvedValue(undefined)
    open('c')
    key('Digit3')
    expect(m.api).toHaveBeenCalledExactlyOnceWith(['c'], { rating: 3 })
    expect(run).toHaveBeenCalledOnce()
  })

  it('0 и Mod+0 — не «Звёзды: убрать»: звёзды целы, масштаб холста не меняется', () => {
    const fit = vi.spyOn(canvas, 'fit').mockImplementation(() => {})
    open('c')
    expect(key('Digit0').defaultPrevented).toBe(true)
    expect(key('Digit0', ctrl).defaultPrevented).toBe(true)
    expect(marks).not.toHaveBeenCalled()
    expect(fit).not.toHaveBeenCalled()
    expect(reached).toEqual([])
  })
})

describe('остальные клавиши', () => {
  it('Ctrl+Z и Ctrl+Y доходят до отмены', async () => {
    open('c')
    key('KeyZ', ctrl)
    key('KeyY', ctrl)
    await vi.waitFor(() => expect(history).toHaveBeenCalledTimes(2))
    expect(m.undo).toHaveBeenCalledOnce()
    expect(m.redo).toHaveBeenCalledOnce()
  })

  it('Ctrl+V не вставляет, даже на холсте с непустым буфером', () => {
    open('c')
    store.zone = 'canvas'
    key('KeyV', ctrl)
    expect(paste).not.toHaveBeenCalled()
  })

  it('B, Shift+B, Del, Пробел, F молчат', () => {
    const toggleBig = vi.spyOn(modes, 'toggleBig').mockImplementation(() => {})
    open('c')
    key('KeyB')
    key('KeyB', { shiftKey: true })
    key('Delete')
    key('Space', { key: ' ' })
    key('KeyF')
    expect(m.toggle).not.toHaveBeenCalled()
    expect(m.remove).not.toHaveBeenCalled()
    expect(toggleBig).not.toHaveBeenCalled()
    expect(photoui.previewId).toBe('c')
  })

  it('F1 под окном молчит: окно стоит в списке «другие окна» справочника', () => {
    const app = readFileSync(`${import.meta.dirname}/../../App.svelte`, 'utf8')
    expect(app).toMatch(/const other =[\s\S]*?!!photoui\.preview/)
    expect(app).toContain('preview: photoui.preview?.from')
  })

  it('после закрытия клавиши программы снова работают', () => {
    open('c')
    key('Escape', { key: 'Escape' })
    expect(photoui.preview).toBeNull()
    expect(modalStack.any()).toBe(false)
    key('KeyP')
    expect(marks).toHaveBeenCalledOnce() // обычные отметки ленты
    expect(lastHost()).toBe(store)
  })
})

describe('свои клавиши окна', () => {
  it('Esc закрывает, не доходя до программы; в стеке окон ничего не остаётся', () => {
    open('c')
    expect(modalStack.any()).toBe(true)
    const e = key('Escape', { key: 'Escape' })
    expect(e.defaultPrevented).toBe(true)
    expect(reached).toEqual([])
    expect(photoui.preview).toBeNull()
    expect(modalStack.any()).toBe(false)
  })

  it('← → листают, края без кольца, клавиши погашены', () => {
    open('b')
    const l = key('ArrowLeft', { key: 'ArrowLeft' })
    expect(l.defaultPrevented).toBe(true)
    expect(photoui.previewId).toBe('a')
    key('ArrowLeft', { key: 'ArrowLeft' })
    expect(photoui.previewId).toBe('a')
    key('ArrowRight', { key: 'ArrowRight' })
    key('ArrowRight', { key: 'ArrowRight' })
    key('ArrowRight', { key: 'ArrowRight' })
    key('ArrowRight', { key: 'ArrowRight' })
    expect(photoui.previewId).toBe('d')
    expect(reached).toEqual([])
  })

  it('стрелки с Ctrl/Shift/Alt не листают и идут дальше', () => {
    open('b')
    key('ArrowRight', { key: 'ArrowRight', shiftKey: true })
    expect(photoui.previewId).toBe('b')
    expect(reached).toEqual(['ArrowRight'])
  })

  it('Z, 0, Mod+0, Mod+=, Mod+− гасятся окном, а переназначенная клавиша — вместо Z', () => {
    useCatalog({ photoPreview: ['KeyJ'] })
    open('c')
    expect(key('KeyJ').defaultPrevented).toBe(true)
    expect(reached).toEqual([]) // J — «Крупный просмотр» окна
    key('KeyZ')
    expect(reached).toEqual(['KeyZ']) // Z больше не её: идёт диспетчеру
    reached.length = 0
    for (const code of ['Equal', 'Minus']) expect(key(code, ctrl).defaultPrevented).toBe(true)
    expect(reached).toEqual([])
  })

  it('стандартная Z гасится окном', () => {
    open('c')
    expect(key('KeyZ').defaultPrevented).toBe(true)
    expect(reached).toEqual([])
  })
})

describe('Z вне окна', () => {
  it('в «Во весь экран» из ленты молчит: окна нет, тоста нет', () => {
    modes.big = true
    runViaApp('KeyZ')
    expect(photoui.preview).toBeNull()
    expect(store.toast).toBeNull()
  })

  it('при открытой галерее молчит', () => {
    gallery.cur = { spread: 's1', scope: 'spread', tab: 'layouts' } as never
    runViaApp('KeyZ')
    expect(photoui.preview).toBeNull()
    expect(store.toast).toBeNull()
  })

  it('в ленте с выделением открывает окно: якорь, если он выделен', () => {
    store.anchor = 'c'
    runViaApp('KeyZ')
    expect(photoui.preview).toEqual({ ids: ['a', 'b', 'c', 'd'], at: 2, from: 'library' })
    cleanup()
  })

  function runViaApp(code: string) {
    window.dispatchEvent(new KeyboardEvent('keydown', { code, key: code === 'KeyZ' ? 'z' : code, bubbles: true, cancelable: true }))
  }
})
