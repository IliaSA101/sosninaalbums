// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { tick } from 'svelte'

// Строка «Ролик» и окно ролика в F1 (спека 140, раздел C): состояния строки, согласие, ход, ошибки, Blob и Esc.
const MB = 1048576
const api = vi.hoisted(() => ({
  GuideVideos: vi.fn(),
  DownloadGuideVideos: vi.fn(),
  CancelGuideVideos: vi.fn(),
  RevealGuideVideo: vi.fn(),
  LibraryPacks: vi.fn(),
  SetLibraryNet: vi.fn(),
  GuideLearned: vi.fn(),
}))
const events = vi.hoisted(() => new Map<string, (p: unknown) => void>())
vi.mock('../../../wailsjs/runtime/runtime', () => ({
  EventsOn: (name: string, f: (p: unknown) => void) => {
    events.set(name, f)
    return () => events.delete(name)
  },
}))
vi.mock('../../../wailsjs/go/app/App', async (orig) => {
  const all = await orig<Record<string, unknown>>()
  return { ...Object.fromEntries(Object.keys(all).map((k) => [k, vi.fn(async () => undefined)])), ...api }
})
// Встроенный список: у двух статей ролик есть и без сети (0.30-140-15)
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
const q = (sel: string) => document.body.querySelector<HTMLElement>(sel)
const qa = (sel: string) => [...document.body.querySelectorAll<HTMLElement>(sel)]
const text = (e: Element | null) => (e?.textContent ?? '').replace(/\s+/g, ' ').trim()
// Строка «Ролик»: ряд карточки, у которого заголовок «Ролик»
const vrow = () => qa('.card .row').find((r) => text(r.querySelector('b')) === 'Ролик') ?? null
const btn = (label: string, root: ParentNode = document.body) =>
  [...root.querySelectorAll<HTMLButtonElement>('button')].find((b) => text(b) === label)
const dialog = (name: string) => qa('[role="dialog"]').find((d) => d.getAttribute('aria-label') === name)
const item = (o: Record<string, unknown> = {}) => ({ id: 'start-window', version: 1, seconds: 48, size: MB / 2, ready: false, ...o })

let notify: ReturnType<typeof vi.spyOn>
let allowed = true
let revoke: ReturnType<typeof vi.fn>

async function open(id = 'start-window') {
  guide.sel = id
  guide.open = true
  render(Guide)
  await settle()
}
// fetch отдаёт ролик: ok, тип и тело
const stubFetch = (o: { ok?: boolean; type?: string | null; fail?: boolean } = {}) =>
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => {
      if (o.fail) throw new TypeError('network')
      return {
        ok: o.ok ?? true,
        headers: { get: () => (o.type === undefined ? 'video/webm' : o.type) },
        blob: async () => new Blob([new Uint8Array(8)]),
      }
    }),
  )

beforeEach(() => {
  useCatalog()
  Element.prototype.scrollIntoView = vi.fn()
  allowed = true
  events.clear()
  api.GuideVideos.mockReset().mockResolvedValue([])
  api.DownloadGuideVideos.mockReset().mockResolvedValue([item({ ready: true })])
  api.CancelGuideVideos.mockReset().mockResolvedValue(undefined)
  api.RevealGuideVideo.mockReset().mockResolvedValue(undefined)
  api.LibraryPacks.mockReset().mockImplementation(async () => ({
    packs: [],
    net: { allowed, atStart: false, checked: 0, skipped: 0 },
    busy: false,
    readOnly: false,
  }))
  api.SetLibraryNet.mockReset().mockImplementation(async (a: boolean, s: boolean) => ({ allowed: a, atStart: s, checked: 0, skipped: 0 }))
  api.GuideLearned.mockResolvedValue([])
  packsUi.listed = false
  packsUi.net.take(undefined)
  videos.items.clear()
  videos.failed.clear()
  videos.reset()
  videos.busy = []
  videos.progress = null
  vi.spyOn(prefs, 'can').mockReturnValue(true)
  notify = vi.spyOn(store, 'notify').mockImplementation(() => {})
  revoke = vi.fn()
  URL.createObjectURL = vi.fn(() => 'blob:ролик')
  URL.revokeObjectURL = revoke as unknown as typeof URL.revokeObjectURL
  stubFetch()
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

describe('строка «Ролик»', () => {
  it('статья без ролика — строки нет', async () => {
    await open('crop')
    expect(vrow()).toBeNull()
  })
  it('ролик из встроенного списка виден до сети: «Скачать · 48 с», каталог читается из кэша Go, скачивания нет', async () => {
    await open()
    expect(text(vrow())).toBe('Ролик Скачать · 48 с')
    expect(api.GuideVideos).toHaveBeenCalledTimes(1)
    expect(api.DownloadGuideVideos).not.toHaveBeenCalled()
  })
  it('каталог дал размер: «Скачать · 48 с, 512 КБ»; ссылка «все ролики (2, 1,0 МБ)» — когда не скачано больше одного', async () => {
    api.GuideVideos.mockResolvedValue([item(), item({ id: 'open' }), item({ id: 'save', ready: true })])
    await open()
    expect(text(vrow())).toBe('Ролик Скачать · 48 с, 512 КБ все ролики (2, 1,0 МБ)')
  })
  it('скачан: «▶ Смотреть · 48 с», ссылок нет', async () => {
    api.GuideVideos.mockResolvedValue([item({ ready: true }), item({ id: 'open' })])
    await open()
    expect(text(vrow())).toBe('Ролик ▶ Смотреть · 48 с')
  })
  it('ролик пересняли (в каталоге ready=false) — снова «Скачать»', async () => {
    api.GuideVideos.mockResolvedValue([item({ version: 2, ready: false })])
    await open()
    expect(btn('Скачать', vrow()!)).toBeTruthy()
    expect(btn('▶ Смотреть', vrow()!)).toBeUndefined()
  })
  it('выбор статьи с роликом и без: строка появляется и уходит', async () => {
    await open()
    expect(vrow()).not.toBeNull()
    guide.sel = 'crop'
    await settle()
    expect(vrow()).toBeNull()
  })
})

describe('«Скачать»', () => {
  it('с согласием: DownloadGuideVideos([id]), ход событием guide:video, затем «Смотреть»', async () => {
    let done: (v: unknown) => void = () => {}
    api.DownloadGuideVideos.mockReturnValue(new Promise((r) => (done = r)))
    await open()
    await fireEvent.click(btn('Скачать', vrow()!)!)
    await settle()
    expect(api.DownloadGuideVideos).toHaveBeenCalledWith(['start-window'])
    expect(text(vrow())).toContain('Скачиваем…')
    expect(btn('Отменить', vrow()!)).toBeTruthy()

    events.get('guide:video')?.({ id: 'start-window', done: Math.round(0.3 * MB), total: MB / 2 })
    await settle()
    expect(text(vrow())).toContain('Скачиваем… 0,3 из 0,5 МБ')
    expect(q('.card progress')?.getAttribute('value')).toBe('60')

    done([item({ ready: true })])
    await settle()
    expect(text(vrow())).toBe('Ролик ▶ Смотреть · 48 с')
    expect(notify).not.toHaveBeenCalled()
  })
  it('ход чужого ролика строку не трогает', async () => {
    api.DownloadGuideVideos.mockReturnValue(new Promise(() => {}))
    await open()
    await fireEvent.click(btn('Скачать', vrow()!)!)
    await settle()
    events.get('guide:video')?.({ id: 'open', done: 1, total: 2 })
    await settle()
    expect(text(vrow())).toBe('Ролик Скачиваем… Отменить')
  })
  it('без согласия: вопрос «Обратиться к GitHub?» тем же текстом; «Отмена» — Go не звали, строка прежняя', async () => {
    allowed = false
    await open()
    await fireEvent.click(btn('Скачать', vrow()!)!)
    await settle()
    const dlg = dialog('Обратиться к GitHub?')!
    expect(dlg).toBeTruthy()
    expect(text(dlg.querySelector('p'))).toContain(
      'Программа скачает список наборов и роликов, картинки-образцы наборов, новые наборы и выбранное вами с github.com',
    )
    expect(text(dlg.querySelector('label'))).toBe('Проверять при запуске (раз в день)')
    expect([...dlg.querySelectorAll('button')].map((b) => text(b)).filter(Boolean)).toEqual(['Отмена', 'Разрешить и скачать']) // порядок: «Отмена» слева
    await fireEvent.click(btn('Отмена', dlg)!)
    await settle()
    expect(dialog('Обратиться к GitHub?')).toBeUndefined()
    expect(api.SetLibraryNet).not.toHaveBeenCalled()
    expect(api.DownloadGuideVideos).not.toHaveBeenCalled()
    expect(btn('Скачать', vrow()!)).toBeTruthy()
  })
  it('«Разрешить и скачать»: SetLibraryNet(true, false), затем скачивание; флажок — SetLibraryNet(true, true)', async () => {
    allowed = false
    await open()
    await fireEvent.click(btn('Скачать', vrow()!)!)
    await settle()
    await fireEvent.click(btn('Разрешить и скачать', dialog('Обратиться к GitHub?')!)!)
    await settle()
    expect(api.SetLibraryNet).toHaveBeenLastCalledWith(true, false)
    expect(api.DownloadGuideVideos).toHaveBeenCalledWith(['start-window'])
    expect(packsUi.net.net.allowed).toBe(true) // согласие общее с «Наборами»

    // второй раз согласие уже есть — вопроса нет
    videos.items.set('start-window', item())
    await settle()
    await fireEvent.click(btn('Скачать', vrow()!)!)
    await settle()
    expect(dialog('Обратиться к GitHub?')).toBeUndefined()
  })
  it('флажок «Проверять при запуске» в вопросе уходит в SetLibraryNet', async () => {
    allowed = false
    await open()
    await fireEvent.click(btn('Скачать', vrow()!)!)
    await settle()
    const dlg = dialog('Обратиться к GitHub?')!
    await fireEvent.click(dlg.querySelector('input[type="checkbox"]')!)
    await fireEvent.click(btn('Разрешить и скачать', dlg)!)
    await settle()
    expect(api.SetLibraryNet).toHaveBeenLastCalledWith(true, true)
  })
  it('Esc в вопросе закрывает только его, справочник остаётся', async () => {
    allowed = false
    await open()
    await fireEvent.click(btn('Скачать', vrow()!)!)
    await settle()
    await fireEvent.keyDown(document.body, { key: 'Escape' })
    await settle()
    expect(dialog('Обратиться к GitHub?')).toBeUndefined()
    expect(dialog('Справочник функций')).toBeTruthy()
    expect(api.DownloadGuideVideos).not.toHaveBeenCalled()
  })
  it('«все ролики»: DownloadGuideVideos со всеми не скачанными', async () => {
    api.GuideVideos.mockResolvedValue([item(), item({ id: 'open' }), item({ id: 'save', ready: true })])
    await open()
    await fireEvent.click(btn('все ролики (2, 1,0 МБ)', vrow()!)!)
    await settle()
    expect(api.DownloadGuideVideos).toHaveBeenCalledWith(['start-window', 'open'])
  })
  it('повторный щелчок во время хода второй загрузки не запускает', async () => {
    api.DownloadGuideVideos.mockReturnValue(new Promise(() => {}))
    await open()
    await fireEvent.click(btn('Скачать', vrow()!)!)
    await settle()
    void videos.get(['start-window'])
    await settle()
    expect(api.DownloadGuideVideos).toHaveBeenCalledTimes(1)
  })
})

describe('«Отменить» и ошибки', () => {
  it('«Отменить» зовёт CancelGuideVideos; отмена без тоста, строка снова «Скачать»', async () => {
    let fail: (e: unknown) => void = () => {}
    api.DownloadGuideVideos.mockReturnValue(new Promise((_, r) => (fail = r)))
    await open()
    await fireEvent.click(btn('Скачать', vrow()!)!)
    await settle()
    await fireEvent.click(btn('Отменить', vrow()!)!)
    await settle()
    expect(api.CancelGuideVideos).toHaveBeenCalledTimes(1)
    expect(text(vrow())).toContain('Отменяем…')
    fail('скачивание ролика отменено')
    await settle()
    expect(notify).not.toHaveBeenCalled()
    expect(btn('Скачать', vrow()!)).toBeTruthy()
  })
  it('ошибка Go — тостом с заглавной, строка снова «Скачать», список перечитан', async () => {
    api.DownloadGuideVideos.mockRejectedValue('нет связи с github.com — проверьте сеть')
    await open()
    await fireEvent.click(btn('Скачать', vrow()!)!)
    await settle()
    expect(notify).toHaveBeenCalledWith('Нет связи с github.com — проверьте сеть', 'error')
    expect(btn('Скачать', vrow()!)).toBeTruthy()
    expect(api.GuideVideos).toHaveBeenCalledTimes(2)
  })
  it('отзыв согласия в «Наборах» во время загрузки ролика — без красного тоста «отменено»', async () => {
    let fail: (e: unknown) => void = () => {}
    api.DownloadGuideVideos.mockReturnValue(new Promise((_, r) => (fail = r)))
    await open()
    await fireEvent.click(btn('Скачать', vrow()!)!)
    await settle()
    await packsUi.net.revoke() // Go обрывает загрузку и отвечает ошибкой «отменено»
    fail('скачивание ролика отменено')
    await settle()
    expect(api.SetLibraryNet).toHaveBeenCalledWith(false, false)
    expect(notify).not.toHaveBeenCalledWith(expect.anything(), 'error')
    expect(notify.mock.calls.map((c: unknown[]) => c[0]).join('|')).not.toContain('отменено')
    expect(btn('Скачать', vrow()!)).toBeTruthy()
  })
  it('ошибка после отзыва согласия в прошлом ходе тост даёт как обычно', async () => {
    await packsUi.net.revoke()
    allowed = false // отзыв был до этого хода: он тост не глушит
    notify.mockClear()
    api.DownloadGuideVideos.mockRejectedValue('нет связи с github.com — проверьте сеть')
    await open()
    await fireEvent.click(btn('Скачать', vrow()!)!)
    await settle()
    await fireEvent.click(btn('Разрешить и скачать', dialog('Обратиться к GitHub?')!)!)
    await settle()
    expect(notify).toHaveBeenCalledWith('Нет связи с github.com — проверьте сеть', 'error')
  })
  it('«Ролика пока нет в библиотеке» — обычный тост, не красный', async () => {
    api.DownloadGuideVideos.mockRejectedValue('ролика пока нет в библиотеке — повторите позже')
    await open()
    await fireEvent.click(btn('Скачать', vrow()!)!)
    await settle()
    expect(notify).toHaveBeenCalledWith('Ролика пока нет в библиотеке — повторите позже', 'info')
  })
  it('отказ в согласии на записи (SetLibraryNet) — тост, скачивания нет', async () => {
    allowed = false
    api.SetLibraryNet.mockRejectedValue('диск недоступен')
    await open()
    await fireEvent.click(btn('Скачать', vrow()!)!)
    await settle()
    await fireEvent.click(btn('Разрешить и скачать', dialog('Обратиться к GitHub?')!)!)
    await settle()
    expect(notify).toHaveBeenCalledWith('Диск недоступен', 'error')
    expect(api.DownloadGuideVideos).not.toHaveBeenCalled()
  })
})

describe('окно ролика', () => {
  const ready = () => api.GuideVideos.mockResolvedValue([item({ ready: true })])

  it('«Смотреть»: fetch целиком → Blob → createObjectURL; <video> играет без звука со своими кнопками', async () => {
    ready()
    await open()
    await fireEvent.click(btn('▶ Смотреть', vrow()!)!)
    await settle()
    const dlg = dialog('Ролик: ' + q('.card h2')!.textContent)!
    expect(dlg).toBeTruthy()
    expect(globalThis.fetch).toHaveBeenCalledWith('/media/guide-video/start-window', expect.anything())
    const blob = (URL.createObjectURL as ReturnType<typeof vi.fn>).mock.calls[0][0] as Blob
    expect(blob.type).toBe('video/webm')
    const v = dlg.querySelector('video')!
    expect(v.getAttribute('src')).toBe('blob:ролик')
    expect(v.hasAttribute('controls') && v.hasAttribute('autoplay') && v.muted).toBe(true)
  })
  it('закрытие окна отзывает Blob-адрес; фокус возвращается на «Смотреть»', async () => {
    ready()
    await open()
    await fireEvent.click(btn('▶ Смотреть', vrow()!)!)
    await settle()
    await fireEvent.click(dialog('Ролик: ' + q('.card h2')!.textContent)!.querySelector('.x')!)
    await settle()
    expect(revoke).toHaveBeenCalledWith('blob:ролик')
    expect(qa('video')).toHaveLength(0)
    expect(document.activeElement).toBe(btn('▶ Смотреть', vrow()!))
  })
  it('Esc закрывает только окно ролика: справочник на месте, Blob отозван, фокус на «Смотреть»', async () => {
    ready()
    await open()
    await fireEvent.click(btn('▶ Смотреть', vrow()!)!)
    await settle()
    await fireEvent.keyDown(document.body, { key: 'Escape' })
    await settle()
    expect(qa('video')).toHaveLength(0)
    expect(dialog('Справочник функций')).toBeTruthy()
    expect(guide.open).toBe(true)
    expect(revoke).toHaveBeenCalledWith('blob:ролик')
    expect(document.activeElement).toBe(btn('▶ Смотреть', vrow()!))
    // второй Esc закрывает уже справочник
    await fireEvent.keyDown(document.body, { key: 'Escape' })
    await settle()
    expect(dialog('Справочник функций')).toBeUndefined()
  })
  it('закрыли до конца загрузки: Blob не создаётся', async () => {
    ready()
    let release: () => void = () => {}
    let signal: AbortSignal | undefined
    vi.stubGlobal(
      'fetch',
      vi.fn(async (_url: string, init?: RequestInit) => ({
        ok: true,
        headers: { get: () => ((signal = init?.signal ?? undefined), 'video/webm') },
        blob: () => new Promise<Blob>((r) => (release = () => r(new Blob([new Uint8Array(8)])))),
      })),
    )
    await open()
    await fireEvent.click(btn('▶ Смотреть', vrow()!)!)
    await settle()
    await fireEvent.keyDown(document.body, { key: 'Escape' })
    await settle()
    expect(signal?.aborted).toBe(true) // загрузка оборвана при закрытии окна
    release()
    await settle()
    expect(URL.createObjectURL).not.toHaveBeenCalled()
    expect(vrow() && text(vrow())).toBe('Ролик ▶ Смотреть · 48 с')
  })
  it('сам файл не открылся (ошибка <video>): окно закрывается, строка «Ролик не открылся…» и «Показать файл»', async () => {
    ready()
    await open()
    await fireEvent.click(btn('▶ Смотреть', vrow()!)!)
    await settle()
    await fireEvent(q('video')!, new Event('error'))
    await settle()
    expect(qa('video')).toHaveLength(0)
    expect(revoke).toHaveBeenCalledWith('blob:ролик')
    expect(text(vrow())).toBe('Ролик Ролик не открылся в окне программы Показать файл Скачать заново')
    await fireEvent.click(btn('Показать файл', vrow()!)!)
    expect(api.RevealGuideVideo).toHaveBeenCalledWith('start-window')
  })
  it('«не открылся» → «Скачать заново»: DownloadGuideVideos([id]), ход строки, затем «Смотреть» (файл заменён)', async () => {
    ready()
    stubFetch({ ok: false })
    let done: (v: unknown) => void = () => {}
    api.DownloadGuideVideos.mockReturnValue(new Promise((r) => (done = r)))
    await open()
    await fireEvent.click(btn('▶ Смотреть', vrow()!)!)
    await settle()
    expect(text(vrow())).toBe('Ролик Ролик не открылся в окне программы Показать файл Скачать заново')
    await fireEvent.click(btn('Скачать заново', vrow()!)!)
    await settle()
    expect(api.DownloadGuideVideos).toHaveBeenCalledWith(['start-window'])
    expect(text(vrow())).toContain('Скачиваем…')
    expect(btn('Скачать заново', vrow()!)).toBeUndefined()
    done([item({ ready: true })])
    await settle()
    expect(text(vrow())).toBe('Ролик ▶ Смотреть · 48 с')
  })
  it('«Скачать заново» без согласия на сеть — тот же вопрос «Обратиться к GitHub?»; «Отмена» — строка осталась «не открылся»', async () => {
    ready()
    stubFetch({ ok: false })
    allowed = false
    await open()
    await fireEvent.click(btn('▶ Смотреть', vrow()!)!)
    await settle()
    await fireEvent.click(btn('Скачать заново', vrow()!)!)
    await settle()
    await fireEvent.click(btn('Отмена', dialog('Обратиться к GitHub?')!)!)
    await settle()
    expect(api.DownloadGuideVideos).not.toHaveBeenCalled()
    expect(text(vrow())).toContain('Ролик не открылся в окне программы')
  })
  it.each([
    ['fetch упал', () => stubFetch({ fail: true })],
    ['ответ не 200', () => stubFetch({ ok: false })],
    ['тип не видео', () => stubFetch({ type: 'text/html' })],
  ])('%s — та же строка «не открылся»', async (_n, make) => {
    ready()
    make()
    await open()
    await fireEvent.click(btn('▶ Смотреть', vrow()!)!)
    await settle()
    expect(text(vrow())).toContain('Ролик не открылся в окне программы')
    expect(URL.createObjectURL).not.toHaveBeenCalled()
  })
  it('«Показать файл» с ошибкой Go — тост; «Смотреть» в строке «не открылся» нет', async () => {
    ready()
    stubFetch({ ok: false })
    api.RevealGuideVideo.mockRejectedValue('файл не найден')
    await open()
    await fireEvent.click(btn('▶ Смотреть', vrow()!)!)
    await settle()
    await fireEvent.click(btn('Показать файл', vrow()!)!)
    await settle()
    expect(notify).toHaveBeenCalledWith('Файл не найден', 'error')
    expect(btn('▶ Смотреть', vrow()!)).toBeUndefined()
    // справочник закрыли и открыли снова: сбой мог быть случайным — «Смотреть» опять есть
    cleanup()
    guide.open = false
    await settle()
    await open()
    expect(btn('▶ Смотреть', vrow()!)).toBeTruthy()
  })
  it('закрытие справочника роняет окно ролика и вопрос о сети', async () => {
    ready()
    await open()
    await fireEvent.click(btn('▶ Смотреть', vrow()!)!)
    await settle()
    guide.open = false
    await settle()
    expect(videos.playing).toBeNull()
    expect(videos.consent).toBeNull()
    expect(revoke).toHaveBeenCalledWith('blob:ролик')
    expect(events.has('guide:video')).toBe(false) // подписка на ход снята
  })
})
