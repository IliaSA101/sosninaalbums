// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render } from '@testing-library/svelte'
import { flushSync } from 'svelte'

// Плитки «Можно скачать» (144) в окне «Фоны» и соседях: щелчок → вопрос → скачивание → постановка. Go подменён,
// постановка (decor.pick) — шпион: её правила проверяют другие тесты окна.
const api = vi.hoisted(() => ({
  DecorCatalog: vi.fn(),
  LibraryPacks: vi.fn(),
  DownloadLibraryPack: vi.fn(),
  PackItem: vi.fn(),
  FetchPackPreviews: vi.fn(),
  CancelLibrary: vi.fn(),
  SetLibraryNet: vi.fn(),
  UpdateLibrary: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))

const { default: DecorDialog } = await import('../decor/DecorDialog.svelte')
const { decor } = await import('../decor/decor.svelte')
const { packsUi } = await import('../../shell/packs/packs.svelte')
const { store } = await import('../../base/core/store.svelte')

const MB = 1048576
const SRC = '0123456789abcdef'
const tile = (o: Record<string, unknown> = {}) => ({
  id: `get:school:${SRC}`,
  pack: 'school',
  src: SRC,
  title: 'Школа — осень',
  url: '/media/lp/school/bg/0',
  w: 240,
  h: 160,
  bytes: 38 * MB,
  with: [],
  bg: 7,
  sticker: 62,
  spread: 0,
  frame: 3,
  ...o,
})
const cat = (kind: string, o: Record<string, unknown> = {}) => ({
  kind,
  themes: [
    { id: 'all', name: 'Все', count: 1 },
    { id: 'get', name: 'Можно скачать', count: 1 },
  ],
  items: [{ id: 'sa:x', theme: 'basic', name: 'Икс', url: '/media/gen/art?k=x&v=1', own: false, aspect: 0, w: 0, h: 0 }],
  recent: [],
  new: [],
  get: [tile()],
  invite: false,
  readOnly: false,
  ...o,
})
const sheet = (id: string) => ({
  id,
  photos: [],
  stickers: [],
  w: 600,
  h: 300,
  mid: 300,
  frames: [],
  texts: [],
  captions: [],
  bg: ['#fff', '#fff'],
  bgSet: ['', ''],
  backdrops: null,
  n: 0,
})
const got = (o: Record<string, unknown> = {}) => ({
  id: 'my:0123456789abcdef',
  theme: 'school',
  name: 'Осень',
  url: '/media/lib/x',
  own: false,
  aspect: 1.5,
  w: 0,
  h: 0,
  ...o,
})
const picked = (o: Record<string, unknown> = {}) => ({ added: 1, updated: 0, skipped: [], bytes: 1, picked: true, ...o })

const open = async (kind: 'bg' | 'sticker' | 'frame' | 'spread', c: Record<string, unknown> = {}, scope: 'spread' | 'left' = 'spread') => {
  api.DecorCatalog.mockResolvedValue(cat(kind, c))
  const r = render(DecorDialog)
  decor.show(kind, 's1', scope)
  await vi.waitFor(() => expect(decor.cat).not.toBeNull())
  flushSync()
  return r
}
const goGet = () => {
  decor.setTheme('get')
  flushSync()
}
const btns = () => Array.from(document.querySelectorAll('button'))
const byText = (text: string) => btns().find((b) => b.textContent?.trim() === text)
const getTile = (id = `get:school:${SRC}`) => document.querySelector<HTMLButtonElement>(`[data-get-id="${id}"]`)
const toast = () => store.toast as { text: string; kind?: string } | null
const dialogText = () => Array.from(document.querySelectorAll('p')).map((p) => p.textContent)
const flush = async () => {
  await Promise.resolve()
  flushSync()
}
let pick: ReturnType<typeof vi.spyOn>

beforeEach(() => {
  for (const f of Object.values(api)) f.mockReset()
  api.LibraryPacks.mockResolvedValue({ packs: [], net: { allowed: true }, busy: false, readOnly: false })
  api.FetchPackPreviews.mockResolvedValue(undefined)
  api.PackItem.mockResolvedValue(got())
  decor.close()
  decor.cat = null
  decor.theme = ''
  packsUi.close()
  packsUi.net.updating = false
  packsUi.net.progress = null
  store.toast = null
  store.view = { rev: 1, folders: [], settings: { pageW: 300, pageH: 300 }, spreads: [sheet('s1'), sheet('s2')] } as never
  pick = vi.spyOn(decor, 'pick').mockResolvedValue(undefined)
})
afterEach(() => {
  pick.mockRestore()
  cleanup()
})

describe('строка и плитка', () => {
  it('«Можно скачать» последней в темах, число; плитка с подписью «⤓ Название · размер» и подсказкой', async () => {
    await open('bg')
    const rows = Array.from(document.querySelectorAll('.vseg button')).map((b) => b.textContent?.replace(/\s+/g, ' ').trim())
    expect(rows.at(-1)).toBe('Можно скачать1')
    goGet()
    const t = getTile()!
    expect(t.title).toBe('Скачать набор «Школа — осень» (38,0 МБ) и поставить этот фон')
    expect(t.parentElement?.querySelector('.line1')?.textContent?.replace(/\s+/g, ' ').trim()).toBe('⤓ Школа — осень · 38,0 МБ')
    expect(t.querySelector('img')?.getAttribute('src')).toBe('/media/lp/school/bg/0')
  })

  it('вне строки «Можно скачать» (во «Все») плиток скачивания нет', async () => {
    await open('bg')
    decor.setTheme('all')
    flushSync()
    expect(getTile()).toBeNull()
  })

  it('название набора с разметкой показано буквами, картинки из названия нет', async () => {
    await open('bg', { get: [tile({ title: '<img src=x onerror=alert(1)>' })] })
    goGet()
    const t = getTile()!
    expect(t.parentElement?.querySelector('.line1')?.textContent).toContain('<img src=x onerror=alert(1)>')
    expect(document.querySelector('img[src="x"]')).toBeNull()
    expect(document.querySelector('[onerror]')).toBeNull()
    fireEvent.click(t)
    flushSync()
    expect(dialogText().join(' ')).toContain('«<img src=x onerror=alert(1)>»')
    expect(document.querySelector('img[src="x"]')).toBeNull()
  })

  it('отбор «Набор» сужает выборку', async () => {
    await open('bg', { get: [tile(), tile({ id: `get:b:${SRC}`, pack: 'b', title: 'Б' })] })
    goGet()
    expect(getTile(`get:b:${SRC}`)).not.toBeNull()
    decor.setFilter('pack', 'b')
    flushSync()
    expect(getTile()).toBeNull()
    expect(getTile(`get:b:${SRC}`)).not.toBeNull()
  })

  it('образцы докачиваются при открытии окна; library:previews каталог перечитывает, докачку не зовёт', async () => {
    await open('bg')
    expect(api.FetchPackPreviews).toHaveBeenCalledTimes(1)
    api.DecorCatalog.mockClear()
    packsUi.previews()
    await vi.waitFor(() => expect(api.DecorCatalog).toHaveBeenCalledTimes(1))
    expect(api.FetchPackPreviews).toHaveBeenCalledTimes(1)
  })
})

describe('щелчок: вопрос, скачивание, постановка', () => {
  it('вопрос → «Скачать и поставить» → DownloadLibraryPack → PackItem → decor.pick; тост «скачан»', async () => {
    api.DownloadLibraryPack.mockResolvedValue(picked())
    await open('bg')
    goGet()
    fireEvent.click(getTile()!)
    flushSync()
    expect(document.body.textContent).toContain('Скачать набор?')
    expect(dialogText()).toContain(
      'В наборе «Школа — осень» 7 фонов, 62 стикера, 3 рамочки — 38,0 МБ. Этот фон встанет сразу после скачивания.',
    )
    expect(byText('Отмена')).toBeTruthy()
    fireEvent.click(byText('Скачать и поставить')!)
    await vi.waitFor(() => expect(pick).toHaveBeenCalledTimes(1))
    expect(api.DownloadLibraryPack).toHaveBeenCalledWith('school')
    expect(api.PackItem).toHaveBeenCalledWith('bg', 'school', SRC)
    expect(pick.mock.calls[0][0]).toMatchObject({ id: 'my:0123456789abcdef' })
    expect(toast()?.text).toBe('Набор «Школа — осень» скачан')
  })

  it('«Отмена» в вопросе: ничего не скачивается', async () => {
    await open('bg')
    goGet()
    fireEvent.click(getTile()!)
    flushSync()
    fireEvent.click(byText('Отмена')!)
    flushSync()
    expect(byText('Скачать и поставить')).toBeUndefined()
    expect(api.DownloadLibraryPack).not.toHaveBeenCalled()
    expect(pick).not.toHaveBeenCalled()
  })

  it('вид окна уходит в PackItem: стикеры и рамочки', async () => {
    api.DownloadLibraryPack.mockResolvedValue(picked())
    for (const kind of ['sticker', 'frame'] as const) {
      api.PackItem.mockClear()
      decor.close()
      cleanup()
      await open(kind)
      goGet()
      fireEvent.click(getTile()!)
      flushSync()
      fireEvent.click(byText('Скачать и поставить')!)
      await vi.waitFor(() => expect(api.PackItem).toHaveBeenCalledWith(kind, 'school', SRC))
    }
  })

  it('вопрос с «Вместе с ним» для набора с дополнениями', async () => {
    await open('bg', { get: [tile({ with: ['Школа — осень: рамки'] })] })
    goGet()
    fireEvent.click(getTile()!)
    flushSync()
    expect(dialogText().join(' ')).toContain('Вместе с ним: «Школа — осень: рамки».')
  })

  it('не встал (версия новее): постановки нет, PackItem не зовётся, тост Go', async () => {
    api.DownloadLibraryPack.mockResolvedValue(picked({ picked: false, why: 'нужна новая версия программы' }))
    await open('bg')
    goGet()
    fireEvent.click(getTile()!)
    flushSync()
    fireEvent.click(byText('Скачать и поставить')!)
    await vi.waitFor(() => expect(toast()?.text).toBe('Набор «Школа — осень» не поставлен: нужна новая версия программы'))
    expect(api.PackItem).not.toHaveBeenCalled()
    expect(pick).not.toHaveBeenCalled()
  })

  it('связь пропала: красный тост, ничего не ставится', async () => {
    api.DownloadLibraryPack.mockRejectedValue('нет связи с github.com')
    await open('bg')
    goGet()
    fireEvent.click(getTile()!)
    flushSync()
    fireEvent.click(byText('Скачать и поставить')!)
    await vi.waitFor(() => expect(toast()).toMatchObject({ text: 'Нет связи с github.com', kind: 'error' }))
    expect(pick).not.toHaveBeenCalled()
  })

  it('предмета в наборе не оказалось: тост «выберите другой», постановки нет', async () => {
    api.DownloadLibraryPack.mockResolvedValue(picked())
    api.PackItem.mockRejectedValue('этого предмета в наборе нет — выберите другой')
    await open('bg')
    goGet()
    fireEvent.click(getTile()!)
    flushSync()
    fireEvent.click(byText('Скачать и поставить')!)
    await vi.waitFor(() => expect(toast()?.text).toBe('Набор «Школа — осень» скачан, но этого фона в нём нет — выберите другой'))
    expect(pick).not.toHaveBeenCalled()
  })

  it('другая ошибка PackItem — красный тост', async () => {
    api.DownloadLibraryPack.mockResolvedValue(picked())
    api.PackItem.mockRejectedValue('библиотека недоступна')
    await open('bg')
    goGet()
    fireEvent.click(getTile()!)
    flushSync()
    fireEvent.click(byText('Скачать и поставить')!)
    await vi.waitFor(() => expect(toast()).toMatchObject({ text: 'Библиотека недоступна', kind: 'error' }))
    expect(pick).not.toHaveBeenCalled()
  })
})

describe('пока идёт ход', () => {
  const start = async (kind: 'bg' | 'spread' | 'frame' = 'bg') => {
    let done: (v: unknown) => void = () => {}
    api.DownloadLibraryPack.mockReturnValue(new Promise((r) => (done = r)))
    await open(kind)
    goGet()
    fireEvent.click(getTile()!)
    flushSync()
    fireEvent.click(byText('Скачать и поставить')!)
    await vi.waitFor(() => expect(packsUi.net.updating).toBe(true))
    flushSync()
    return done
  }

  it('над плитками полоса хода с «Отменить»; плитки скачивания серые, щелчок по ним молчит', async () => {
    const done = await start()
    packsUi.net.progressed({ stage: 'download', title: 'Школа — осень', done: 12 * MB, total: 38 * MB })
    flushSync()
    expect(document.querySelector('[role="status"]')?.textContent).toBe('Скачиваем «Школа — осень» — 12 из 38 МБ')
    expect(byText('Отменить')).toBeTruthy()
    const t = getTile()!
    expect(t.getAttribute('aria-disabled')).toBe('true')
    expect(t.title).toBe('Библиотека обновляется')
    fireEvent.click(t)
    flushSync()
    expect(byText('Скачать и поставить')).toBeUndefined()
    byText('Отменить')!.click()
    expect(api.CancelLibrary).toHaveBeenCalledTimes(1)
    done(picked())
    await vi.waitFor(() => expect(packsUi.net.updating).toBe(false))
  })

  it('окно закрыто во время хода: набор встал, тост, ничего не ставится и PackItem не зовётся', async () => {
    const done = await start()
    decor.close()
    flushSync()
    done(picked())
    await vi.waitFor(() => expect(packsUi.net.updating).toBe(false))
    await flush()
    expect(toast()?.text).toBe('Набор «Школа — осень» скачан')
    expect(api.PackItem).not.toHaveBeenCalled()
    expect(pick).not.toHaveBeenCalled()
  })

  it('окно закрыли и открыли снова — это уже другое открытие, не ставим', async () => {
    const done = await start()
    decor.show('bg', 's1', 'spread')
    flushSync()
    done(picked())
    await vi.waitFor(() => expect(packsUi.net.updating).toBe(false))
    await flush()
    expect(pick).not.toHaveBeenCalled()
  })

  // Ход кончился, метка цели сменилась: набор встал (тост), но ничего не ставится
  const settle = async (done: (v: unknown) => void) => {
    done(picked())
    await vi.waitFor(() => expect(packsUi.net.updating).toBe(false))
    await flush()
    expect(pick).not.toHaveBeenCalled()
    expect(toast()?.text).toBe('Набор «Школа — осень» скачан')
  }

  it('сменили «Куда» (область) — не ставим', async () => {
    const done = await start()
    decor.setScope('left')
    flushSync()
    await settle(done)
  })

  it('сменили «Куда» Фона альбома (на весь разворот ↔ на каждую страницу) — не ставим', async () => {
    const done = await start()
    decor.setAlbumPages(true)
    flushSync()
    await settle(done)
  })

  it('сменили «На» (bgPick) — не ставим', async () => {
    const done = await start()
    decor.bgPick = 'all'
    flushSync()
    await settle(done)
  })

  it('сменили «Куда» рамочек (frameWhere) — не ставим', async () => {
    const done = await start('frame')
    decor.frameWhere = 'sel'
    flushSync()
    await settle(done)
  })

  it('открыли окно на другом листе — не ставим', async () => {
    const done = await start()
    decor.show('bg', 's2', 'spread')
    flushSync()
    await settle(done)
  })

  it('PackItem отвечает уже после закрытия окна: still() после поиска — не ставим', async () => {
    api.DownloadLibraryPack.mockResolvedValue(picked())
    let item: (v: unknown) => void = () => {}
    api.PackItem.mockReturnValue(new Promise((r) => (item = r)))
    await open('bg')
    goGet()
    fireEvent.click(getTile()!)
    flushSync()
    fireEvent.click(byText('Скачать и поставить')!)
    await vi.waitFor(() => expect(api.PackItem).toHaveBeenCalledTimes(1))
    decor.close()
    item(got())
    await flush()
    await flush()
    expect(pick).not.toHaveBeenCalled()
  })

  it('«Отменить» ответом Go: тост «отменено», ничего не ставится, плитки снова живые', async () => {
    let fail: (e: unknown) => void = () => {}
    api.DownloadLibraryPack.mockReturnValue(new Promise((_, rej) => (fail = rej)))
    await open('bg')
    goGet()
    fireEvent.click(getTile()!)
    flushSync()
    fireEvent.click(byText('Скачать и поставить')!)
    await vi.waitFor(() => expect(packsUi.net.updating).toBe(true))
    fail('обновление библиотеки отменено')
    await vi.waitFor(() => expect(packsUi.net.updating).toBe(false))
    flushSync()
    expect(toast()?.text).toBe('Обновление отменено')
    expect(getTile()!.getAttribute('aria-disabled')).toBeNull()
    expect(pick).not.toHaveBeenCalled()
  })
})

describe('развороты: «Куда»', () => {
  it('скачан разворот на две страницы, а «Куда» — левая: тост «для другого «Куда»», не ставится', async () => {
    api.DownloadLibraryPack.mockResolvedValue(picked())
    api.PackItem.mockResolvedValue(got({ pages: 2 }))
    await open(
      'spread',
      { get: [tile({ url: '/media/lp/school/spread/0', w: 240, h: 120, bg: 0, sticker: 0, frame: 0, spread: 4 })] },
      'left',
    )
    goGet()
    expect(getTile()!.title).toBe('Скачать набор «Школа — осень» (38,0 МБ) и поставить этот разворот')
    fireEvent.click(getTile()!)
    flushSync()
    fireEvent.click(byText('Скачать и поставить')!)
    await vi.waitFor(() => expect(toast()?.text).toBe('Набор «Школа — осень» скачан — этот разворот для другого «Куда», он в «Новых»'))
    expect(pick).not.toHaveBeenCalled()
  })

  it('плитка скачивания видна при любом «Куда»; разворот-страница ставится', async () => {
    api.DownloadLibraryPack.mockResolvedValue(picked())
    api.PackItem.mockResolvedValue(got({ pages: 1 }))
    await open('spread', { get: [tile({ url: '/media/lp/school/spread/0', w: 240, h: 120, spread: 4 })] }, 'left')
    goGet()
    expect(getTile()).not.toBeNull()
    fireEvent.click(getTile()!)
    flushSync()
    fireEvent.click(byText('Скачать и поставить')!)
    await vi.waitFor(() => expect(pick).toHaveBeenCalledTimes(1))
  })
})

describe('приглашение и согласие', () => {
  it('«Обновить» → «Разрешить» при открытых «Фонах»: докачка образцов, каталог перечитан, приглашение сменилось строкой', async () => {
    api.LibraryPacks.mockResolvedValue({ packs: [], net: { allowed: false }, busy: false, readOnly: false })
    api.SetLibraryNet.mockResolvedValue({ allowed: true })
    api.UpdateLibrary.mockResolvedValue({ added: 0, updated: 0, skipped: [], bytes: 0, available: 1 })
    await open('bg', { invite: true, get: [], themes: [{ id: 'all', name: 'Все', count: 1 }] })
    await packsUi.show()
    expect(api.FetchPackPreviews).toHaveBeenCalledTimes(1) // при открытии «Фонов»
    api.DecorCatalog.mockClear()
    api.DecorCatalog.mockResolvedValue(cat('bg'))
    packsUi.net.update()
    await packsUi.net.confirmConsent()
    await vi.waitFor(() => expect(api.FetchPackPreviews).toHaveBeenCalledTimes(2))
    await vi.waitFor(() => expect(api.DecorCatalog).toHaveBeenCalled())
    await vi.waitFor(() => expect(decor.cat?.get).toHaveLength(1))
    flushSync()
    expect(Array.from(document.querySelectorAll('.vseg button')).some((b) => b.textContent?.includes('Можно скачать'))).toBe(true)
  })

  it('«Обновить» при закрытых галереях: докачку не зовёт', async () => {
    api.LibraryPacks.mockResolvedValue({ packs: [], net: { allowed: true }, busy: false, readOnly: false })
    api.UpdateLibrary.mockResolvedValue({ added: 0, updated: 0, skipped: [], bytes: 0, available: 0 })
    await packsUi.show()
    packsUi.net.update()
    await vi.waitFor(() => expect(api.UpdateLibrary).toHaveBeenCalled())
    await vi.waitFor(() => expect(packsUi.net.updating).toBe(false))
    expect(api.FetchPackPreviews).not.toHaveBeenCalled()
  })

  it('списка не было (invite): в конце «Все» плитка «⤓ Ещё из интернета», щелчок открывает «Наборы библиотеки»', async () => {
    await open('bg', { invite: true, get: [], themes: [{ id: 'all', name: 'Все', count: 1 }] })
    decor.setTheme('all')
    flushSync()
    const inv = getTile('invite')!
    expect(inv).not.toBeNull()
    expect(inv.parentElement?.querySelector('.line1')?.textContent?.trim()).toBe('⤓ Ещё из интернета')
    expect(inv.title).toBe('Наборы фонов, стикеров и макетов с github.com — окно «Наборы библиотеки»')
    expect(inv.querySelector('img')).toBeNull()
    fireEvent.click(inv)
    await vi.waitFor(() => expect(packsUi.open).toBe(true))
  })

  it('приглашение только во «Все»; без invite его нет (снято согласие при готовом списке — ни плиток, ни приглашения)', async () => {
    await open('bg', {
      invite: true,
      get: [],
      themes: [
        { id: 'all', name: 'Все', count: 1 },
        { id: 'basic', name: 'Базовые', count: 1 },
      ],
    })
    decor.setTheme('basic')
    flushSync()
    expect(getTile('invite')).toBeNull()
    cleanup()
    decor.close()
    await open('bg', { invite: false, get: [], themes: [{ id: 'all', name: 'Все', count: 1 }] })
    decor.setTheme('all')
    flushSync()
    expect(getTile('invite')).toBeNull()
    expect(getTile()).toBeNull()
    expect(Array.from(document.querySelectorAll('.vseg button')).some((b) => b.textContent?.includes('Можно скачать'))).toBe(false)
  })

  it('старый Go без полей get и invite: окно как раньше', async () => {
    api.DecorCatalog.mockResolvedValue({
      kind: 'bg',
      themes: [{ id: 'all', name: 'Все', count: 1 }],
      items: cat('bg').items,
      recent: [],
      readOnly: false,
    })
    render(DecorDialog)
    decor.show('bg', 's1', 'spread')
    await vi.waitFor(() => expect(decor.cat).not.toBeNull())
    decor.setTheme('all')
    flushSync()
    expect(document.querySelectorAll('[data-get-id]')).toHaveLength(0)
  })

  it('библиотека только для чтения: плитки серые с подсказкой, щелчок молчит', async () => {
    await open('bg', { readOnly: true })
    goGet()
    const t = getTile()!
    expect(t.getAttribute('aria-disabled')).toBe('true')
    expect(t.title).toBe('Библиотеку изменила более новая версия программы: свои файлы пока только смотреть')
    fireEvent.click(t)
    flushSync()
    expect(byText('Скачать и поставить')).toBeUndefined()
  })
})
