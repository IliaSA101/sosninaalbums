import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { View } from '../../base/core/types'

// Буфер на холсте (099): Ctrl+C, X, V, D — вызовы Go, выделение вставленного, тосты, отказы (repeat, предел).
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  CopyItems: vi.fn(),
  CutItems: vi.fn(),
  PasteItems: vi.fn(),
  DuplicateItems: vi.fn(),
  ClipState: vi.fn(),
  Undo: vi.fn(),
}))
vi.mock('../../base/drag/copyask.svelte', () => ({ askCopy: vi.fn() }))
vi.mock('../text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const Api = await import('../../../wailsjs/go/app/App')
const { askCopy } = await import('../../base/drag/copyask.svelte')
const { store } = await import('../../base/core/store.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { edit } = await import('../text/edit.svelte')
const { useCatalog } = await import('../../base/keys/keymap.fixture')
const { clip } = await import('./clip.svelte')
const { NO_CLIP, resetStamps, fresh } = await import('./clip')
const { NameTrack } = await import('../spread/nametrack.svelte')

const CopyItems = vi.mocked(Api.CopyItems)
const CutItems = vi.mocked(Api.CutItems)
const PasteItems = vi.mocked(Api.PasteItems)
const DuplicateItems = vi.mocked(Api.DuplicateItems)
const ClipState = vi.mocked(Api.ClipState)
const AskCopy = vi.mocked(askCopy)

const place = (id: string, photo: string, x = 0) => ({
  id,
  photo,
  x,
  y: 0,
  w: 10,
  h: 10,
  fit: '',
  rot: 0,
  shape: '',
  round: 0,
  spine: false,
  z: 0,
})
const sheet = (id: string, o: Record<string, unknown> = {}) => ({
  id,
  w: 600,
  h: 300,
  mid: 300,
  grid: true,
  photos: ['a'],
  rects: [],
  frames: [],
  texts: [{ id: 't1' }],
  captions: [],
  backdrops: [],
  stickers: [{ id: 'k1' }],
  places: [place('p0', 'a'), place('p1', '', 20)],
  ...o,
})
let rev = 1
const view = (spreads: object[] = [sheet('s1')]): View =>
  ({ rev, marksRev: 0, folders: [], spreads, settings: { pageW: 300, pageH: 300 }, backdropUsed: [], safe: [] }) as unknown as View
const info = (o: Record<string, unknown> = {}) => ({ photos: 0, places: 0, texts: 0, stickers: 0, sheet: '', ...o })
const pasted = (o: Record<string, unknown> = {}) => ({
  view: view(),
  refs: [],
  texts: [],
  stickers: [],
  repeat: [],
  froze: false,
  clip: info({ photos: 1 }),
  ...o,
})
const flush = async () => {
  for (let i = 0; i < 8; i++) await Promise.resolve()
  await new Promise((r) => setTimeout(r, 0))
}

beforeEach(() => {
  vi.resetAllMocks() // и очереди mock*Once от прошлых тестов
  store.onCover = false
  useCatalog()
  resetStamps()
  rev = 1
  store.toast = null
  store.apply(view())
  store.go(0)
  placesel.clear()
  edit.clear()
  clip.info = NO_CLIP
  clip.hover = null
})

describe('Ctrl+C', () => {
  it('снимок выделенного: места по ID, надпись и стикер; сводка и тост из ответа Go', async () => {
    placesel.pickHit({ refs: ['a'], texts: ['t1'], stickers: ['k1'] }, false)
    CopyItems.mockResolvedValueOnce(info({ photos: 1, texts: 1, stickers: 1, sheet: 's1' }) as never)
    clip.copy()
    await flush()
    expect(CopyItems).toHaveBeenCalledWith('s1', { refs: ['p0'], texts: ['t1'], stickers: ['k1'] })
    expect(clip.info).toMatchObject({ photos: 1, texts: 1, stickers: 1 })
    expect(store.toast?.text).toBe('Скопировано: фото, надпись, стикер. Вставить — Ctrl+V на любом листе')
    expect(store.view.rev).toBe(1) // правки нет: ни шага, ни ревизии
  })
  it('одиночная надпись (выделена в edit) копируется', async () => {
    edit.select({ kind: 'text', spread: 's1', id: 't1' })
    CopyItems.mockResolvedValueOnce(info({ texts: 1 }) as never)
    clip.copy()
    await flush()
    expect(CopyItems).toHaveBeenCalledWith('s1', { refs: [], texts: ['t1'], stickers: [] })
  })
  it('нет выделения — Go не зовём, буфер и тост прежние', async () => {
    clip.info = info({ stickers: 2 })
    clip.copy()
    await flush()
    expect(CopyItems).not.toHaveBeenCalled()
    expect(clip.info.stickers).toBe(2)
    expect(store.toast).toBeNull()
  })
  it('выделенное исчезло с листа — ничего не копируется', async () => {
    placesel.pickHit({ refs: [], texts: ['gone'], stickers: ['gone'] }, false)
    clip.copy()
    await flush()
    expect(CopyItems).not.toHaveBeenCalled()
  })
  it('идёт ввод надписи — копирование блока не вмешивается', async () => {
    edit.select({ kind: 'text', spread: 's1', id: 't1' })
    edit.target = { kind: 'text', spread: 's1', id: 't1' }
    expect(clip.canCopy).toBe(false)
    clip.copy()
    await flush()
    expect(CopyItems).not.toHaveBeenCalled()
    edit.target = null
  })
  it('клавиша вставки снята — тост без хвоста', async () => {
    useCatalog({ pasteItems: [] })
    placesel.pickHit({ refs: [], texts: ['t1'] }, false)
    CopyItems.mockResolvedValueOnce(info({ texts: 1 }) as never)
    clip.copy()
    await flush()
    expect(store.toast?.text).toBe('Скопировано: надпись.')
  })
  it('ошибка Go — красный тост, буфер прежний', async () => {
    placesel.pickHit({ refs: [], texts: ['t1'] }, false)
    CopyItems.mockRejectedValueOnce(new Error('сбой'))
    clip.copy()
    await flush()
    expect(store.toast?.kind).toBe('error')
    expect(clip.info).toEqual(NO_CLIP)
  })
})

describe('Ctrl+X', () => {
  it('один вызов Go, вид принят, тост «Вырезано»', async () => {
    placesel.pickHit({ refs: ['a'], texts: [] }, false)
    rev = 2
    CutItems.mockResolvedValueOnce({
      view: view([sheet('s1', { places: [place('p1', '', 20)], photos: [] })]),
      clip: info({ photos: 1 }),
    } as never)
    clip.cut()
    await flush()
    expect(CutItems).toHaveBeenCalledWith('s1', { refs: ['p0'], texts: [], stickers: [] })
    expect(store.view.rev).toBe(2)
    expect(clip.info.photos).toBe(1)
    expect(store.toast?.text).toBe('Вырезано: фото. Вставить — Ctrl+V')
  })
  it('нет выделения — ничего', async () => {
    clip.cut()
    await flush()
    expect(CutItems).not.toHaveBeenCalled()
  })
})

describe('Ctrl+V', () => {
  it('буфер пуст — вызова нет', async () => {
    clip.paste()
    await flush()
    expect(PasteItems).not.toHaveBeenCalled()
  })
  it('шлёт лист и курсор над листом, без At; вставленное выделено группой (занятое место — по фото)', async () => {
    clip.info = info({ photos: 1, texts: 1 })
    clip.hover = { x: 120, y: 40 }
    rev = 2
    PasteItems.mockResolvedValueOnce(
      pasted({
        view: view([sheet('s1', { places: [place('p0', 'a'), place('n1', 'b', 30), place('n2', '', 50)], photos: ['a', 'b'] })]),
        refs: ['n1', 'n2'],
        texts: ['t1'],
      }) as never,
    )
    clip.paste()
    await flush()
    expect(PasteItems).toHaveBeenCalledWith({ sheet: 's1', repeat: '', hover: { x: 120, y: 40 } })
    expect(placesel.refs).toEqual(['b', 'n2'])
    expect(placesel.texts).toEqual(['t1']) // группа: места и надпись выделены вместе
  })
  it('точка меню — в At, курсор не шлётся, если его нет', async () => {
    clip.info = info({ stickers: 1 })
    PasteItems.mockResolvedValueOnce(pasted({ stickers: ['k1'] }) as never)
    clip.paste({ x: 50, y: 60 })
    await flush()
    expect(PasteItems).toHaveBeenCalledWith({ sheet: 's1', repeat: '', at: { x: 50, y: 60 } })
    expect(placesel.stickers).toEqual(['k1'])
  })
  describe('фото уже в альбоме (repeat, окно 098)', () => {
    beforeEach(() => {
      clip.info = info({ photos: 1 })
      placesel.pickHit({ refs: ['p1'], texts: [] }, false)
      PasteItems.mockResolvedValueOnce(pasted({ repeat: ['a'] }) as never)
    })
    it('«Отмена»: второго вызова нет, вид и выделение прежние, тоста нет', async () => {
      AskCopy.mockResolvedValueOnce('cancel')
      clip.paste()
      await flush()
      expect(AskCopy).toHaveBeenCalledWith(['a'])
      expect(PasteItems).toHaveBeenCalledTimes(1)
      expect(store.toast).toBeNull()
      expect(store.view.rev).toBe(1)
      expect(placesel.refs).toEqual(['p1'])
    })
    it.each(['copy', 'move'] as const)('«%s»: второй вызов с ответом, вставленное выделяется', async (a) => {
      AskCopy.mockResolvedValueOnce(a)
      PasteItems.mockResolvedValueOnce(pasted({ refs: ['a-2'], view: view([sheet('s1', { places: [place('n1', 'a-2')] })]) }) as never)
      clip.paste()
      await flush()
      expect(PasteItems).toHaveBeenCalledTimes(2)
      expect(PasteItems).toHaveBeenLastCalledWith({ sheet: 's1', repeat: a })
      expect(placesel.refs).toEqual(['a-2'])
    })
    // 098 п. 4: «Перенести сюда» через вставку — тот же тост, что у броска из ленты.
    describe('перенос: тост «Фото перенесено …»', () => {
      const two = (a0: string[], a1: string[]) =>
        view([sheet('s0', { photos: a0, places: [] }), sheet('s1', { photos: a1, places: [place('n1', a1[0] ?? '')] })])
      beforeEach(() => {
        store.apply(two(['a'], []))
        store.go(1)
      })
      it('«Перенести сюда»: «Фото перенесено из разворота 1»', async () => {
        AskCopy.mockResolvedValueOnce('move')
        rev++
        PasteItems.mockResolvedValueOnce(pasted({ refs: ['a'], view: two([], ['a']) }) as never)
        clip.paste()
        await flush()
        expect(store.toast?.text).toBe('Фото перенесено из разворота 1')
      })
      it('«Поставить копию»: переноса нет, тоста нет', async () => {
        AskCopy.mockResolvedValueOnce('copy')
        rev++
        PasteItems.mockResolvedValueOnce(pasted({ refs: ['a-2'], view: two(['a'], ['a-2']) }) as never)
        clip.paste()
        await flush()
        expect(store.toast).toBeNull()
      })
      it('фото не встало на лист: тоста нет', async () => {
        AskCopy.mockResolvedValueOnce('move')
        rev++
        PasteItems.mockResolvedValueOnce(pasted({ view: two(['a'], []) }) as never)
        clip.paste()
        await flush()
        expect(store.toast).toBeNull()
      })
    })
    it('Go снова вернул repeat: тост-причина, без третьего вызова', async () => {
      AskCopy.mockResolvedValueOnce('copy')
      PasteItems.mockResolvedValueOnce(pasted({ repeat: ['a'] }) as never)
      clip.paste()
      await flush()
      expect(PasteItems).toHaveBeenCalledTimes(2)
      expect(store.toast).toMatchObject({ text: 'Не вставлено: это фото уже стоит в альбоме' })
    })
    it('дубль не спрашивает: repeat — тост-причина', async () => {
      clip.info = info()
      DuplicateItems.mockResolvedValueOnce(pasted({ repeat: ['a'] }) as never)
      clip.duplicate()
      await flush()
      expect(AskCopy).not.toHaveBeenCalled()
      expect(store.toast).toMatchObject({ text: 'Не вставлено: это фото уже стоит в альбоме' })
    })
  })
  it('предел листа: ошибка Go тостом, выделение прежнее', async () => {
    clip.info = info({ texts: 1 })
    placesel.pickHit({ refs: ['p1'], texts: [] }, false)
    PasteItems.mockRejectedValueOnce(new Error('на листе не больше 64 надписей'))
    clip.paste()
    await flush()
    expect(store.toast?.kind).toBe('error')
    expect(store.toast?.text).toMatch(/64 надписей/)
    expect(placesel.refs).toEqual(['p1'])
  })
  it('«Авто» стала «Ручной»: тост с «Вернуть», как от «+ Место»', async () => {
    store.apply(view([sheet('s1', { grid: false, places: [], photos: ['a'], rects: [{ x: 0, y: 0, w: 5, h: 5 }] })]))
    clip.info = info({ places: 1 })
    rev = 2
    PasteItems.mockResolvedValueOnce(
      pasted({ view: view([sheet('s1', { places: [place('n1', '', 5)] })]), refs: ['n1'], froze: true }) as never,
    )
    clip.paste()
    await flush()
    expect(store.toast?.text).toMatch(/Раскладка стала ручной/)
    expect(store.toast?.action?.label).toBe('Вернуть')
    expect(placesel.refs).toEqual(['n1'])
  })
  it('лист сменился до ответа — выделение чужого листа не ставится', async () => {
    store.apply(view([sheet('s1'), sheet('s2')]))
    clip.info = info({ places: 1 })
    let done!: (v: unknown) => void
    PasteItems.mockReturnValueOnce(new Promise((r) => (done = r)) as never)
    clip.paste()
    store.go(1)
    done(pasted({ view: view([sheet('s1'), sheet('s2')]), refs: ['p1'] }))
    await flush()
    expect(placesel.refs).toEqual([])
  })
  it('идёт ввод надписи — вставка не вмешивается', async () => {
    clip.info = info({ texts: 1 })
    edit.select({ kind: 'text', spread: 's1', id: 't1' })
    edit.target = { kind: 'text', spread: 's1', id: 't1' }
    expect(clip.canPaste).toBe(false)
    clip.paste()
    await flush()
    expect(PasteItems).not.toHaveBeenCalled()
    edit.target = null
  })
  it('обложка: лист вставки — обложка', async () => {
    store.apply({ ...view(), cover: { sheet: sheet('cover') } } as unknown as View)
    store.goCover()
    clip.info = info({ texts: 1 })
    PasteItems.mockResolvedValueOnce(pasted({ texts: ['t1'] }) as never)
    clip.paste()
    await flush()
    expect(PasteItems.mock.calls[0][0]).toMatchObject({ sheet: 'cover' })
  })
})

describe('Ctrl+D', () => {
  it('дубль выделенного: Go получает выделение, вставленное выделяется; буфер — как ответил Go', async () => {
    placesel.pickHit({ refs: [], texts: [], stickers: ['k1'] }, false)
    clip.info = info({ photos: 3 })
    DuplicateItems.mockResolvedValueOnce(pasted({ stickers: ['k2'], clip: info({ photos: 3 }) }) as never)
    clip.duplicate()
    await flush()
    expect(DuplicateItems).toHaveBeenCalledWith('s1', { refs: [], texts: [], stickers: ['k1'] })
    expect(placesel.stickers).toEqual(['k2'])
    expect(clip.info.photos).toBe(3)
  })
  it('фото в альбоме — копия сразу, без окна и без тоста про «уже в альбоме» (0.23-q2)', async () => {
    placesel.pickHit({ refs: ['a'], texts: [] }, false)
    rev = 2
    DuplicateItems.mockResolvedValueOnce(
      pasted({
        view: view([sheet('s1', { places: [place('p0', 'a'), place('n1', 'a-1', 5)], photos: ['a', 'a-1'] })]),
        refs: ['n1'],
      }) as never,
    )
    clip.duplicate()
    await flush()
    expect(store.toast).toBeNull()
    expect(placesel.refs).toEqual(['a-1'])
    expect(PasteItems).not.toHaveBeenCalled()
  })
  it('нет выделения — ничего', async () => {
    clip.duplicate()
    await flush()
    expect(DuplicateItems).not.toHaveBeenCalled()
  })
})

describe('очередь, сводка и страж', () => {
  it('вызовы идут по порядку: вставка не обгоняет копирование', async () => {
    const order: string[] = []
    let freeCopy!: (v: unknown) => void
    CopyItems.mockImplementationOnce(() => new Promise((r) => (freeCopy = r)) as never)
    PasteItems.mockImplementationOnce(async () => (order.push('paste'), pasted() as never))
    placesel.pickHit({ refs: [], texts: ['t1'] }, false)
    clip.copy()
    clip.info = info({ texts: 1 }) // буфер уже отдал сводку (прежний), вставка встала в очередь
    clip.paste()
    await flush()
    expect(order).toEqual([])
    order.push('copy')
    freeCopy(info({ texts: 1 }))
    await flush()
    expect(order).toEqual(['copy', 'paste'])
  })
  it('sync берёт сводку у Go; сбой — пустой буфер', async () => {
    ClipState.mockResolvedValueOnce(info({ photos: 2, sheet: 's1' }) as never)
    await clip.sync()
    expect(clip.info.photos).toBe(2)
    ClipState.mockRejectedValueOnce(new Error('нет'))
    await clip.sync()
    expect(clip.info).toEqual(NO_CLIP)
  })
  it('sync в очереди: поздний ClipState не откатывает сводку после Ctrl+C', async () => {
    let freeState!: (v: unknown) => void
    ClipState.mockImplementationOnce(() => new Promise((r) => (freeState = r)) as never)
    CopyItems.mockResolvedValueOnce(info({ texts: 1, sheet: 's1' }) as never)
    placesel.pickHit({ refs: [], texts: ['t1'] }, false)
    const synced = clip.sync()
    clip.copy()
    await flush()
    expect(CopyItems).not.toHaveBeenCalled() // копирование ждёт ответа ClipState
    freeState(NO_CLIP)
    await synced
    await flush()
    expect(CopyItems).toHaveBeenCalledTimes(1)
    expect(clip.info.texts).toBe(1) // сводка копирования, не пустая
  })
  it('любая команда ставит отметку: DOM-путь Mac сразу после клавиши не сработает', async () => {
    clip.info = info({ texts: 1 })
    PasteItems.mockResolvedValueOnce(pasted() as never)
    expect(fresh('pasteItems', Date.now())).toBe(true)
    clip.paste()
    expect(fresh('pasteItems', Date.now())).toBe(false)
    await flush()
  })
})

describe('курсор над листом (NameTrack → clip.hover)', () => {
  const mkTrack = () => {
    let nt!: InstanceType<typeof NameTrack>
    const off = $effect.root(() => {
      nt = new NameTrack(() => ({ spread: store.spread!, mini: false, root: undefined, bgEdit: false }))
    })
    return { nt, off }
  }
  // Лист 600×300 мм нарисован в рамке 300×150 px со сдвигом (10, 20).
  const move = (x: number, y: number) =>
    ({
      clientX: x,
      clientY: y,
      target: null,
      currentTarget: { getBoundingClientRect: () => ({ left: 10, top: 20, width: 300, height: 150 }) },
    }) as unknown as PointerEvent
  it('движение переводит px в мм листа; уход курсора снимает точку', () => {
    const { nt, off } = mkTrack()
    nt.track(move(160, 95))
    expect(clip.hover).toEqual({ x: 300, y: 150 })
    nt.leave()
    expect(clip.hover).toBeNull()
    off()
  })
  it('нулевой размер рамки — точки нет, а не NaN', () => {
    const { nt, off } = mkTrack()
    nt.track({ ...move(0, 0), currentTarget: { getBoundingClientRect: () => ({ left: 0, top: 0, width: 0, height: 0 }) } } as never)
    expect(clip.hover).toBeNull()
    off()
  })
})
