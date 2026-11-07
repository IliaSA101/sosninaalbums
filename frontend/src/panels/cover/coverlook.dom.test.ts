// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'

// «Создать обложку» в стиле макета альбома (131 G5, решение 0.28-131-170): тост «Обложка в стиле «…»» и «Без оформления».
const api = vi.hoisted(() => ({
  CreateCoverLook: vi.fn(),
  CreateCover: vi.fn(),
  Undo: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), ...api }
})

const { default: CoverTile } = await import('./CoverTile.svelte')
const { default: Overlay } = await import('../../base/drag/Overlay.svelte')
const { store } = await import('../../base/core/store.svelte')

const sp = (id: string) => ({
  id,
  photos: [] as string[],
  w: 600,
  h: 300,
  mid: 300,
  frames: [],
  texts: [],
  captions: [],
  bg: ['#fff', '#fff'],
  bgSet: ['', ''],
})
const view = (rev: number, cover = false) => ({
  rev,
  folders: [],
  settings: { pageW: 300, pageH: 300, sheet: false },
  spreads: [sp('s1'), sp('s2')],
  cover: cover ? { sheet: sp('cover'), spine: 14 } : null,
})

const text = () => document.body.textContent ?? ''
const btn = (label: string) =>
  Array.from(document.querySelectorAll('button')).find((b) => b.textContent?.trim() === label) as HTMLButtonElement | undefined

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  store.toast = null
  store.onCover = false
  store.apply(view(1) as never)
  store.go(0)
  render(CoverTile)
  render(Overlay)
})
afterEach(() => {
  cleanup()
  store.toast = null
})

describe('«+ Обложка» при макете альбома', () => {
  it('макет с обложкой: CreateCoverLook, тост «Обложка в стиле «Светлая»» с кнопкой «Без оформления»', async () => {
    api.CreateCoverLook.mockResolvedValue({ view: view(2, true), styled: true, look: 'Светлая' })
    await fireEvent.click(btn('+ Обложка') as HTMLElement)
    await vi.waitFor(() => expect(text()).toContain('Обложка в стиле «Светлая»'))
    expect(api.CreateCoverLook).toHaveBeenCalledExactlyOnceWith('', '')
    expect(api.CreateCover).not.toHaveBeenCalled()
    expect(store.onCover).toBe(true)
    expect(btn('Без оформления')).toBeTruthy()
  })

  it("«Без оформления» с фото броска: Undo, затем CreateCover('', фото), обложка открыта", async () => {
    api.CreateCoverLook.mockResolvedValue({ view: view(2, true), styled: true, look: 'Светлая' })
    api.Undo.mockResolvedValue(view(3))
    api.CreateCover.mockResolvedValue(view(4, true))
    expect(await store.createCover('p1')).toBe(true)
    expect(api.CreateCoverLook).toHaveBeenCalledExactlyOnceWith('', 'p1')
    await vi.waitFor(() => expect(btn('Без оформления')).toBeTruthy())
    await fireEvent.click(btn('Без оформления') as HTMLElement)
    await vi.waitFor(() => expect(api.CreateCover).toHaveBeenCalledExactlyOnceWith('', 'p1'))
    expect(api.Undo).toHaveBeenCalledOnce()
    expect(api.Undo.mock.invocationCallOrder[0]).toBeLessThan(api.CreateCover.mock.invocationCallOrder[0])
    expect(store.onCover).toBe(true)
    expect(store.view.rev).toBe(4)
    expect(text()).not.toContain('Обложка в стиле')
  })

  it('«Без оформления» пустой обложки (плитка «+ Обложка»): CreateCover с пустым фото', async () => {
    api.CreateCoverLook.mockResolvedValue({ view: view(2, true), styled: true, look: 'Светлая' })
    api.Undo.mockResolvedValue(view(3))
    api.CreateCover.mockResolvedValue(view(4, true))
    await fireEvent.click(btn('+ Обложка') as HTMLElement)
    await vi.waitFor(() => expect(btn('Без оформления')).toBeTruthy())
    await fireEvent.click(btn('Без оформления') as HTMLElement)
    await vi.waitFor(() => expect(api.CreateCover).toHaveBeenCalledExactlyOnceWith('', ''))
    expect(api.Undo).toHaveBeenCalledOnce()
  })

  it('Undo не прошёл — CreateCover не зовётся, вид прежний, ошибка красным тостом', async () => {
    api.CreateCoverLook.mockResolvedValue({ view: view(2, true), styled: true, look: 'Светлая' })
    api.Undo.mockRejectedValue('нечего отменять')
    await fireEvent.click(btn('+ Обложка') as HTMLElement)
    await vi.waitFor(() => expect(btn('Без оформления')).toBeTruthy())
    await fireEvent.click(btn('Без оформления') as HTMLElement)
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
    expect(api.CreateCover).not.toHaveBeenCalled()
    expect(store.view.rev).toBe(2)
  })

  it('после обложки была другая правка (ревизия сменилась) — тост гаснет, Undo не зовётся', async () => {
    api.CreateCoverLook.mockResolvedValue({ view: view(2, true), styled: true, look: 'Светлая' })
    await fireEvent.click(btn('+ Обложка') as HTMLElement)
    await vi.waitFor(() => expect(btn('Без оформления')).toBeTruthy())
    const run = store.toast?.action?.run as () => Promise<void>
    store.apply(view(3, true) as never)
    await vi.waitFor(() => expect(btn('Без оформления')).toBeUndefined())
    // Действие, взятое до смены ревизии, само не пускает Undo: «Уже были другие изменения».
    await run()
    expect(text()).toContain('Уже были другие изменения')
    expect(api.Undo).not.toHaveBeenCalled()
    expect(api.CreateCover).not.toHaveBeenCalled()
  })

  it('Go отдал styled без имени макета — тоста нет, обложка есть', async () => {
    api.CreateCoverLook.mockResolvedValue({ view: view(2, true), styled: true, look: '' })
    await fireEvent.click(btn('+ Обложка') as HTMLElement)
    await vi.waitFor(() => expect(store.onCover).toBe(true))
    expect(store.toast).toBeNull()
  })

  it('макета нет (styled=false): обычная обложка, тоста нет', async () => {
    api.CreateCoverLook.mockResolvedValue({ view: view(2, true), styled: false, look: '' })
    await fireEvent.click(btn('+ Обложка') as HTMLElement)
    await vi.waitFor(() => expect(store.onCover).toBe(true))
    expect(store.toast).toBeNull()
    expect(text()).not.toContain('Обложка в стиле')
  })

  it('ошибка Go: красный тост, обложки нет, «Без оформления» нет', async () => {
    api.CreateCoverLook.mockRejectedValue('в альбоме нет разворотов')
    await fireEvent.click(btn('+ Обложка') as HTMLElement)
    await vi.waitFor(() => expect(store.toast?.kind).toBe('error'))
    expect(btn('Без оформления')).toBeUndefined()
    expect(store.onCover).toBe(false)
  })
})
