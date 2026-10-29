import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Окно «QR-код» (100 п. 2): ввод, образец от Go с задержкой, ошибки под полем, «Поставить». Go подменён.
const api = vi.hoisted(() => ({
  DecorCatalog: vi.fn(),
  QRPreview: vi.fn(),
  AddQRSticker: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { decor } = await import('./decor.svelte')
const { qrForm } = await import('./qr.svelte')
const { QR_DELAY, qrLines } = await import('./qr')
const { store } = await import('../../base/core/store.svelte')
const { placesel } = await import('../../editor/places/placesel.svelte')

const info = (o: Record<string, unknown> = {}) => ({ text: 'https://site.ru/x', side: 33, path: 'M4 4h7v1h-7z', minMM: 26, ...o })
const cat = (readOnly = false) => ({ kind: 'sticker', themes: [], items: [], recent: [], readOnly })
const sheet = (stickers: unknown[] = []) => ({
  id: 's1',
  photos: [],
  w: 600,
  h: 300,
  mid: 300,
  frames: [],
  texts: [],
  captions: [],
  bg: ['#fff', '#fff'],
  bgSet: ['', ''],
  backdrops: null,
  stickers,
  n: 0,
})
const view = (rev: number, stickers: unknown[] = []) => ({
  rev,
  folders: [],
  settings: { pageW: 300, pageH: 300 },
  spreads: [sheet(stickers)],
})

const openAll = async (readOnly = false) => {
  api.DecorCatalog.mockResolvedValue(cat(readOnly))
  decor.show('sticker', 's1', 'spread')
  await vi.waitFor(() => expect(decor.cat).not.toBeNull())
  qrForm.show()
}
const type = async (v: string) => {
  qrForm.input(v)
  await vi.advanceTimersByTimeAsync(QR_DELAY)
}

beforeEach(() => {
  vi.useFakeTimers()
  for (const f of Object.values(api)) f.mockReset()
  api.QRPreview.mockImplementation(async (l: string) => info({ text: l }))
  decor.close()
  qrForm.close()
  store.toast = null
  store.apply(view(1) as never)
  placesel.stickers = []
})
afterEach(() => vi.useRealTimers())

describe('qrLines: строки под полем', () => {
  it('ссылку дополнили — «Будет в коде» и минимум печати; не дополнили — только минимум', () => {
    expect(qrLines(info(), 'site.ru/x')).toEqual([
      'Будет в коде: https://site.ru/x',
      'Печатать не мельче 26 мм — иначе телефон может не прочитать',
    ])
    expect(qrLines(info(), '  https://site.ru/x ')).toEqual(['Печатать не мельче 26 мм — иначе телефон может не прочитать'])
  })
  it('нет образца — строк нет', () => expect(qrLines(null, 'x')).toEqual([]))
})

describe('окно «QR-код»: образец', () => {
  it('до ввода окно пустое и «Поставить» серая; Go не спрашивали', async () => {
    await openAll()
    expect(qrForm.open).toBe(true)
    expect(qrForm.info).toBeNull()
    expect(qrForm.error).toBe('')
    expect(qrForm.canPut).toBe(false)
    expect(api.QRPreview).not.toHaveBeenCalled()
  })

  it('образец — через 150 мс после ввода, одним запросом на серию букв', async () => {
    await openAll()
    qrForm.input('s')
    qrForm.input('si')
    await vi.advanceTimersByTimeAsync(QR_DELAY - 1)
    expect(api.QRPreview).not.toHaveBeenCalled()
    qrForm.input('site.ru')
    await vi.advanceTimersByTimeAsync(QR_DELAY)
    expect(api.QRPreview).toHaveBeenCalledTimes(1)
    expect(api.QRPreview).toHaveBeenCalledWith('site.ru')
    expect(qrForm.info?.text).toBe('site.ru')
    expect(qrForm.canPut).toBe(true)
  })

  it('ошибка Go — под полем, «Поставить» серая; исправили — ошибка уходит', async () => {
    api.QRPreview.mockRejectedValueOnce('ссылка слишком длинная')
    await openAll()
    await type('x'.repeat(501))
    expect(qrForm.error).toBe('Ссылка слишком длинная')
    expect(qrForm.info).toBeNull()
    expect(qrForm.canPut).toBe(false)
    await type('site.ru')
    expect(qrForm.error).toBe('')
    expect(qrForm.canPut).toBe(true)
  })

  it('исправил ссылку и сразу Enter — старая ошибка не мешает, ставится без ожидания образца', async () => {
    api.QRPreview.mockRejectedValueOnce('ссылка слишком длинная')
    api.AddQRSticker.mockResolvedValue({ view: view(2, [{ id: 'k9' }]), id: 'k9' })
    await openAll()
    await type('x'.repeat(501))
    expect(qrForm.canPut).toBe(false)
    qrForm.input('site.ru') // таймер образца (150 мс) ещё не сработал
    expect(qrForm.error).toBe('')
    expect(await qrForm.commit()).toBe(true)
    expect(api.AddQRSticker).toHaveBeenCalledWith('s1', 0, 'site.ru')
  })

  it('ответ на устаревший ввод отбрасывается', async () => {
    let late!: (v: unknown) => void
    api.QRPreview.mockImplementationOnce(() => new Promise((r) => (late = r)))
    await openAll()
    await type('a')
    await type('ab') // второй запрос обгоняет первый
    late(info({ text: 'a' }))
    await vi.advanceTimersByTimeAsync(0)
    expect(qrForm.info?.text).toBe('ab')
  })

  it('повторное открытие — чистая форма', async () => {
    await openAll()
    await type('site.ru')
    qrForm.close()
    qrForm.show()
    expect(qrForm.link).toBe('')
    expect(qrForm.info).toBeNull()
  })

  it('окно «Стикеры» закрыли — форма закрыта вместе с ним (как «Сохранить разворот…»)', async () => {
    await openAll()
    decor.close()
    qrForm.close()
    expect(qrForm.open).toBe(false)
  })
})

describe('окно «QR-код»: «Поставить»', () => {
  it('страница и ссылка уходят в Go; новый стикер выделен; оба окна закрыты', async () => {
    api.AddQRSticker.mockResolvedValue({ view: view(2, [{ id: 'k9' }]), id: 'k9' })
    await openAll()
    await type('site.ru/x')
    placesel.page = 1
    expect(await qrForm.commit()).toBe(true)
    expect(api.AddQRSticker).toHaveBeenCalledWith('s1', 1, 'site.ru/x')
    expect(store.view.rev).toBe(2)
    expect(placesel.stickers).toEqual(['k9'])
    expect(qrForm.open).toBe(false)
    expect(decor.cur).toBeNull()
  })

  it('Enter в пустом поле — Go не ставит, причина под полем', async () => {
    api.QRPreview.mockRejectedValueOnce('введите ссылку')
    await openAll()
    expect(await qrForm.commit()).toBe(false)
    expect(api.AddQRSticker).not.toHaveBeenCalled()
    expect(qrForm.error).toBe('Введите ссылку')
    expect(qrForm.open).toBe(true)
  })

  it('отказ Go — красный тост, окна остаются, форма снова живая', async () => {
    api.AddQRSticker.mockRejectedValue('на листе уже 64 стикера')
    await openAll()
    await type('site.ru')
    expect(await qrForm.commit()).toBe(false)
    expect(store.toast).toMatchObject({ kind: 'error', text: 'На листе уже 64 стикера' })
    expect(qrForm.open).toBe(true)
    expect(decor.cur).not.toBeNull()
    expect(qrForm.busy).toBe(false)
  })

  it('библиотека только для чтения — «Поставить» серая, Go не зовём', async () => {
    await openAll(true)
    await type('site.ru')
    expect(qrForm.canPut).toBe(false)
    expect(await qrForm.commit()).toBe(false)
    expect(api.AddQRSticker).not.toHaveBeenCalled()
  })

  it('двойной Enter, пока идёт постановка, ставит один раз', async () => {
    let done!: (v: unknown) => void
    api.AddQRSticker.mockImplementationOnce(() => new Promise((r) => (done = r)))
    await openAll()
    await type('site.ru')
    const first = qrForm.commit()
    expect(await qrForm.commit()).toBe(false)
    done({ view: view(2), id: '' })
    expect(await first).toBe(true)
    expect(api.AddQRSticker).toHaveBeenCalledTimes(1)
  })

  it('без окна «Стикеры» (лист пропал) — не ставит', async () => {
    await openAll()
    await type('site.ru')
    decor.close()
    expect(await qrForm.commit()).toBe(false)
    expect(api.AddQRSticker).not.toHaveBeenCalled()
  })
})
