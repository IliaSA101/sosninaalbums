// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Входы в сравнение (138-9): C в ленте при 0/1/3/5 выделенных, Shift+C со списком ленты, C и Shift+C из окна 128 —
// окно 128 закрывается без смены выделения и прокрутки, их делает закрытие сравнения; не открылось — 128 остаётся.
const m = vi.hoisted(() => ({ similar: vi.fn() }))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => ({
  ...(await orig<typeof import('../../../wailsjs/go/app/App')>()),
  SimilarTo: m.similar,
}))

const { default: PhotoCompare } = await import('./PhotoCompare.svelte')
const { default: PhotoPreview } = await import('../../editor/photo/PhotoPreview.svelte')
const { compare } = await import('./compare.svelte')
const { photoui } = await import('../../editor/photo/photoui.svelte')
const { keyCommand, preventsDefault } = await import('../../shell/menu/keys')
const { runKey } = await import('../../shell/menu/commands')
const { store } = await import('../../base/core/store.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { modalStack } = await import('../../base/modal/modalstack.svelte')
const { useCatalog, dropCatalog } = await import('../../base/keys/keymap.fixture')

const IDS = ['a', 'b', 'c', 'd', 'e', 'f']
const photo = (id: string) => ({ id, name: `${id}.jpg`, w: 3, h: 2 })

// То же, что App.keys(): контекст из живых состояний.
function app(e: KeyboardEvent) {
  const id = keyCommand(e, {
    inTextField: false,
    onCheckbox: false,
    galleryOpen: false,
    hasSelection: store.selectedIds.length > 0,
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
function key(code: string, init: KeyboardEventInit = {}) {
  const e = new KeyboardEvent('keydown', { code, key: init.key ?? code, bubbles: true, cancelable: true, ...init })
  ;(document.querySelector('[role=dialog]') ?? document.body).dispatchEvent(e)
  flushSync()
  return e
}
const C = { key: 'c' }
const shiftC = { key: 'C', shiftKey: true }
const select = (...ids: string[]) => {
  store.selected.clear()
  for (const id of ids) store.selected.add(id)
  store.anchor = ids[0] ?? null
}

beforeEach(() => {
  useCatalog()
  vi.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(1200)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(700)
  store.photos = IDS.map(photo) as never
  store.zone = 'library'
  store.toast = null
  photoui.reveal = null
  select()
  m.similar.mockReset()
  window.addEventListener('keydown', app)
})
afterEach(() => {
  window.removeEventListener('keydown', app)
  compare.close()
  photoui.preview = null
  cleanup()
  dropCatalog()
  vi.restoreAllMocks()
})

describe('C в ленте', () => {
  it('0 и 5 выделенных — тост, окна нет; 1 — пара с соседним; 3 — они', () => {
    render(PhotoCompare)
    key('KeyC', C)
    expect(store.toast?.text).toBe('Выделите в ленте 2–4 фото, чтобы сравнить')
    expect(compare.open).toBe(false)
    select('a', 'b', 'c', 'd', 'e')
    key('KeyC', C)
    expect(store.toast?.text).toBe('Сравнить можно до 4 фото — выделено 5')
    select('b')
    key('KeyC', C)
    expect(compare.ids).toEqual(['b', 'c'])
    expect(document.querySelectorAll('.cell')).toHaveLength(2)
    compare.close()
    flushSync() // эффект окна снимает его с modalStack
    select('a', 'c', 'e')
    key('KeyC', C)
    expect(compare.ids).toEqual(['a', 'c', 'e'])
    expect(modalStack.any()).toBe(true)
  })

  it('на холсте C молчит', () => {
    render(PhotoCompare)
    select('a', 'b')
    store.zone = 'canvas'
    key('KeyC', C)
    expect(compare.open).toBe(false)
    expect(store.toast).toBeNull()
  })
})

describe('Shift+C', () => {
  it('Go получает кадр и замороженный список ленты; ответ открывает окно', async () => {
    m.similar.mockResolvedValue({ ids: ['c', 'd'], more: 0, why: '' })
    render(PhotoCompare)
    select('c')
    key('KeyC', shiftC)
    await vi.waitFor(() => expect(compare.open).toBe(true))
    expect(m.similar).toHaveBeenCalledExactlyOnceWith('c', IDS)
    expect(compare.ids).toEqual(['c', 'd'])
  })
})

describe('из окна 128', () => {
  const open128 = (id = 'c') => {
    render(PhotoPreview)
    render(PhotoCompare)
    select('a', 'b')
    photoui.openLibrary(id)
    flushSync()
  }

  it('C: 128 закрыто без смены выделения и прокрутки, сравнение — показанный и соседний по списку окна', () => {
    open128('c')
    key('KeyC', C)
    expect(photoui.preview).toBeNull()
    expect(compare.ids).toEqual(['c', 'd'])
    expect([...store.selected]).toEqual(['a', 'b'])
    expect(photoui.reveal).toBeNull()
    expect(modalStack.count()).toBe(1)
    // выделение и прокрутку делает закрытие сравнения
    key('Escape', { key: 'Escape' })
    expect([...store.selected].sort()).toEqual(['c', 'd'])
    expect(photoui.reveal?.id).toBe('c')
  })

  it('C у последнего кадра списка — пара с предыдущим', () => {
    open128('f')
    key('KeyC', C)
    expect(compare.ids).toEqual(['e', 'f'])
    expect(compare.activeId).toBe('f')
  })

  it('Shift+C: 128 остаётся, пока Go ищет; ответ — 128 закрыто, окно открыто, выделение цело', async () => {
    let done: (v: unknown) => void = () => {}
    m.similar.mockReturnValue(new Promise((r) => (done = r)))
    open128('c')
    key('KeyC', shiftC)
    expect(m.similar).toHaveBeenCalledExactlyOnceWith('c', IDS)
    expect(photoui.preview).not.toBeNull()
    done({ ids: ['b', 'c'], more: 0, why: '' })
    await vi.waitFor(() => expect(compare.open).toBe(true))
    expect(photoui.preview).toBeNull()
    expect(compare.activeId).toBe('c')
    expect([...store.selected]).toEqual(['a', 'b'])
  })

  it('похожих нет — тост, 128 остаётся открытым', async () => {
    m.similar.mockResolvedValue({ ids: [], more: 0, why: 'none' })
    open128('c')
    key('KeyC', shiftC)
    await vi.waitFor(() => expect(store.toast?.text).toBe('Похожих кадров рядом в ленте нет'))
    expect(photoui.preview).not.toBeNull()
    expect(compare.open).toBe(false)
  })

  it('окно 128 с холста: C и Shift+C молчат', () => {
    render(PhotoPreview)
    render(PhotoCompare)
    photoui.openCanvas('c')
    flushSync()
    key('KeyC', C)
    key('KeyC', shiftC)
    expect(compare.open).toBe(false)
    expect(photoui.preview).not.toBeNull()
    expect(m.similar).not.toHaveBeenCalled()
  })
})
