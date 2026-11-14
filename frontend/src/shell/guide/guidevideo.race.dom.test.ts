// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// «▶ Смотреть» раньше ответа GuideVideos (спека 140, CI wq039a): сначала список от Go (кэш), потом решение «играть / качать».
// Отдельный файл: guidevideo.dom.test.ts упёрся в предел строк.
const api = vi.hoisted(() => ({
  GuideVideos: vi.fn(),
  DownloadGuideVideos: vi.fn(),
  CancelGuideVideos: vi.fn(),
  RevealGuideVideo: vi.fn(),
  LibraryPacks: vi.fn(),
  SetLibraryNet: vi.fn(),
  GuideLearned: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), ...api }
})
vi.mock('../guidetext/videos', () => ({
  GUIDE_VIDEOS: { 'start-window': { seconds: 48, hash: 'x' }, open: { seconds: 30, hash: 'y' } },
}))

const { default: Guide } = await import('./Guide.svelte')
const { guide } = await import('./guidestate.svelte')
const { videos } = await import('./videos.svelte')
const { store } = await import('../../base/core/store.svelte')
const { packsUi } = await import('../packs/packs.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')
const { dropCatalog, useCatalog } = await import('../../base/keys/keymap.fixture')

const settle = async () => {
  for (let i = 0; i < 6; i++) await tick()
}
const qa = (sel: string) => [...document.body.querySelectorAll<HTMLElement>(sel)]
const text = (e: Element | null) => (e?.textContent ?? '').replace(/\s+/g, ' ').trim()
const vrow = () => qa('.card .row').find((r) => text(r.querySelector('b')) === 'Ролик')!
const watchBtn = () => [...vrow().querySelectorAll('button')].find((b) => text(b) === '▶ Смотреть')!
const dialog = (name: string) => qa('[role="dialog"]').find((d) => d.getAttribute('aria-label') === name)
const item = (ready: boolean) => ({ id: 'start-window', version: 1, seconds: 48, size: 1, ready })

let allowed = true
// Открывает справочник и жмёт «▶ Смотреть», пока GuideVideos ещё не ответил (если мок отложен)
async function clickEarly() {
  guide.sel = 'start-window'
  guide.open = true
  render(Guide)
  await settle()
  await fireEvent.click(watchBtn())
  await settle()
}

beforeEach(() => {
  useCatalog()
  Element.prototype.scrollIntoView = vi.fn()
  allowed = true
  api.GuideVideos.mockReset().mockResolvedValue([])
  api.DownloadGuideVideos.mockReset().mockResolvedValue([item(true)])
  api.CancelGuideVideos.mockReset().mockResolvedValue(undefined)
  api.LibraryPacks.mockReset().mockImplementation(async () => ({
    packs: [],
    net: { allowed, atStart: false, checked: 0, skipped: 0 },
    busy: false,
    readOnly: false,
  }))
  api.SetLibraryNet.mockReset().mockResolvedValue({ allowed: true, atStart: false, checked: 0, skipped: 0 })
  api.GuideLearned.mockResolvedValue([])
  packsUi.listed = false
  packsUi.net.take(undefined)
  videos.items.clear()
  videos.failed.clear()
  videos.reset()
  videos.busy = []
  vi.spyOn(prefs, 'can').mockReturnValue(true)
  vi.spyOn(store, 'notify').mockImplementation(() => {})
  URL.createObjectURL = vi.fn(() => 'blob:ролик')
  URL.revokeObjectURL = vi.fn()
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => ({
      ok: true,
      headers: { get: () => 'video/webm' },
      blob: async () => new Blob([new Uint8Array(8)]),
    })),
  )
  guide.opened.clear()
  guide.query = ''
  guide.rowFilter = null
  guide.open = false
})
afterEach(() => {
  cleanup()
  guide.open = false
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  dropCatalog()
})

describe('«Смотреть» до ответа GuideVideos', () => {
  const winDialog = () => dialog('Ролик: ' + document.body.querySelector('.card h2')!.textContent)

  it('скачанный ролик открывается из кэша, DownloadGuideVideos не зовётся', async () => {
    let answer: (v: unknown) => void = () => {}
    api.GuideVideos.mockReturnValue(new Promise((r) => (answer = r)))
    allowed = false // сети нет: любой вопрос «Обратиться к GitHub?» был бы лишним
    await clickEarly()
    expect(api.DownloadGuideVideos).not.toHaveBeenCalled() // ждёт список от Go
    answer([item(true)])
    await settle()
    expect(api.GuideVideos).toHaveBeenCalledTimes(1) // второго запроса нет
    expect(api.DownloadGuideVideos).not.toHaveBeenCalled()
    expect(dialog('Обратиться к GitHub?')).toBeUndefined()
    const dlg = winDialog()!
    expect(dlg.querySelector('video')!.getAttribute('src')).toBe('blob:ролик')
    expect((URL.createObjectURL as ReturnType<typeof vi.fn>).mock.calls[0][0]).toBeInstanceOf(Blob)
  })
  it('ролика нет в списке — прежний путь скачивания', async () => {
    let answer: (v: unknown) => void = () => {}
    api.GuideVideos.mockReturnValue(new Promise((r) => (answer = r)))
    await clickEarly()
    expect(api.DownloadGuideVideos).not.toHaveBeenCalled()
    answer([item(false)])
    await settle()
    expect(api.DownloadGuideVideos).toHaveBeenCalledWith(['start-window'])
  })
  it('Go ответил ошибкой — watch не ломается, идёт скачивание', async () => {
    api.GuideVideos.mockRejectedValue('нет связи с Go')
    await clickEarly()
    expect(api.DownloadGuideVideos).toHaveBeenCalledWith(['start-window'])
  })
})
