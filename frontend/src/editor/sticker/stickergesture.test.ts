import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { StickerView, View } from '../../base/core/types'

// Стикеры на листе (048 Ф2): жесты, выделение, правки через Go. Go подменён вызовами с проверкой аргументов;
// окно — настоящий EventTarget с таймерами: жест drive слушает pointermove/pointerup на window.
class FakeWindow extends EventTarget {
  setTimeout = setTimeout
  clearTimeout = clearTimeout
}
vi.stubGlobal('window', new FakeWindow())

const api = vi.hoisted(() => ({
  AddSticker: vi.fn(),
  ReplaceSticker: vi.fn(),
  SetStickerBox: vi.fn(),
  FlipSticker: vi.fn(),
  SetStickerFade: vi.fn(),
  SetStickerInks: vi.fn(),
  DeleteSelection: vi.fn(),
  ArrangeLayers: vi.fn(),
  TransformGroup: vi.fn(),
}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => api)
vi.mock('../text/fonts', () => ({ loadFonts: async () => {} }))

const { store } = await import('../../base/core/store.svelte')
const { placesel } = await import('../places/placesel.svelte')
const { edit } = await import('../text/edit.svelte')
const { groupIds, groupMembers } = await import('../places/groupgesture')
const { sticker, stickerFadeLive, currentPage } = await import('./sticker.svelte')

const fire = (type: string, props: Record<string, unknown> = {}) =>
  window.dispatchEvent(Object.assign(new Event(type, { cancelable: true }), props))
const down = (x = 0, y = 0) => ({ clientX: x, clientY: y, button: 0 }) as PointerEvent
// Рамка листа на экране: 600 px на 600 мм — один пиксель равен одному миллиметру.
const root = { getBoundingClientRect: () => ({ left: 0, top: 0, width: 600, height: 300 }) } as HTMLElement

const mk = (id: string, over: Partial<StickerView> = {}): StickerView => ({
  id,
  x: 100,
  y: 100,
  w: 40,
  h: 20,
  rot: 0,
  flipH: false,
  flipV: false,
  fade: 0,
  z: 5,
  photo: '', // Go шлёт всегда, у процедурного — пусто
  stretch: false,
  name: 'Сердце',
  inks: 2,
  url: '/media/gen/art?k=heart&v=1',
  gen: true,
  aspect: 2,
  dpi: 0,
  known: true,
  art: { kind: 'heart', inks: ['#aa0000', '#00aa00'] },
  ...over,
})

let rev = 1
const view = (stickers: StickerView[] = [mk('k1')], grid = false, texts: unknown[] = []): View =>
  ({
    spreads: [
      {
        id: 's1',
        w: 600,
        h: 300,
        mid: 300,
        grid,
        photos: [],
        rects: [],
        frames: [],
        texts,
        captions: [],
        backdrops: [],
        places: grid ? [] : null,
        stickers,
        bg: ['#fff', '#fff'],
      },
    ],
    settings: { pageW: 300, pageH: 300, margin: 12, gap: 3, layflat: true, dpi: 300, bg: '' },
    folders: [],
    rev: rev++,
    safe: [],
    backdropUsed: [],
    minDPI: 200,
  }) as unknown as View

const reply = (v: View = view()) => Promise.resolve(v)
const st1 = () => store.spread!.stickers![0]

// Лист с одним фото p1 (выделить фото рядом со стикером).
const withPhoto = (): View => {
  const v = view()
  const sp = v.spreads[0]
  sp.photos = ['p1']
  sp.rects = [{ x: 0, y: 0, w: 50, h: 50 }]
  sp.frames = [{ z: 1, cellRot: 0 } as never]
  return v
}

beforeEach(() => {
  vi.clearAllMocks()
  store.toast = null
  store.missing = []
  // wire() в тесте не нужен целиком: нужны выделение и обрезка выделения после правки
  store.plug({
    ...store.hooks,
    selectedRefs: () => placesel.refs,
    select: (ids) => placesel.select(ids),
    applied: () => placesel.prune(),
  })
  store.apply(view())
  placesel.clear()
  placesel.snap = false // прилипание проверяется отдельно: числа жестов здесь точные
  edit.clear()
  for (const f of Object.values(api)) f.mockImplementation(() => reply())
})

describe('выделение', () => {
  it('щелчок выделяет один стикер; фото и текст отпускаются', () => {
    store.apply(withPhoto())
    placesel.select(['p1'])
    expect(placesel.selected.length).toBe(1)
    placesel.pickSticker('k1', false)
    expect(placesel.stickers).toEqual(['k1'])
    expect(placesel.refs).toEqual([])
    expect(placesel.soloSticker?.id).toBe('k1')
    expect(placesel.isGroup).toBe(false)
    expect(placesel.count).toBe(1)
  })

  it('Shift/Ctrl добавляет в группу и убирает из неё; остался один — снова одиночный', () => {
    store.apply(view([mk('k1'), mk('k2', { x: 300 })]))
    placesel.pickSticker('k1', false)
    placesel.pickSticker('k2', true)
    expect(placesel.stickers).toEqual(['k1', 'k2'])
    expect(placesel.isGroup).toBe(true)
    expect(placesel.soloSticker).toBeNull()
    placesel.pickSticker('k1', true)
    expect(placesel.stickers).toEqual(['k2'])
    expect(placesel.soloSticker?.id).toBe('k2')
  })

  it('Shift+щелчок по стикеру при выделенном фото — группа из двух, а не замена', () => {
    store.apply(withPhoto())
    placesel.select(['p1'])
    placesel.pickSticker('k1', true)
    expect(placesel.refs).toEqual(['p1'])
    expect(placesel.stickers).toEqual(['k1'])
    expect(placesel.soloSticker).toBeNull()
  })

  it('щелчок по фото или полю (select) снимает выделение стикера', () => {
    placesel.pickSticker('k1', false)
    placesel.select([])
    expect(placesel.stickers).toEqual([])
    placesel.pickSticker('k1', false)
    store.selectedCell = null // щелчок по пустому полю
    expect(placesel.stickers).toEqual([])
  })

  it('выделили текст — стикер отпускается, и наоборот', () => {
    store.apply(view([mk('k1')], false, [{ id: 't1', x: 0, y: 0, w: 10, h: 5, rot: 0, z: 1, style: { size: 12 } }]))
    placesel.pickSticker('k1', false)
    edit.select({ kind: 'text', spread: 's1', id: 't1' })
    expect(placesel.stickers).toEqual([])
    expect(edit.sel?.id).toBe('t1')
    placesel.pickSticker('k1', false)
    expect(edit.sel).toBeNull()
    expect(placesel.stickers).toEqual(['k1'])
  })

  it('Shift+щелчок по стикеру при выделенном тексте — текст уходит в группу', () => {
    store.apply(view([mk('k1')], false, [{ id: 't1', x: 0, y: 0, w: 10, h: 5, rot: 0, z: 1, style: { size: 12 } }]))
    edit.select({ kind: 'text', spread: 's1', id: 't1' })
    placesel.pickSticker('k1', true)
    expect(edit.sel).toBeNull()
    expect(placesel.texts).toEqual(['t1'])
    expect(placesel.stickers).toEqual(['k1'])
    expect(placesel.isGroup).toBe(true)
  })

  it('исчез из вида (Ctrl+Z, удаление) — выпал из выделения', () => {
    store.apply(view([mk('k1'), mk('k2')]))
    placesel.pickSticker('k1', false)
    placesel.pickSticker('k2', true)
    store.apply(view([mk('k2')]))
    placesel.prune()
    expect(placesel.stickers).toEqual(['k2'])
    expect(placesel.soloSticker?.id).toBe('k2')
    store.apply(view([]))
    placesel.prune()
    expect(placesel.stickers).toEqual([])
  })

  it('Ctrl+A берёт и стикеры', () => {
    store.apply(view([mk('k1'), mk('k2')]))
    placesel.selectAll()
    expect(placesel.stickers).toEqual(['k1', 'k2'])
    expect(placesel.isGroup).toBe(true)
  })

  it('рамкой: marquee добавляет и заменяет', () => {
    store.apply(view([mk('k1'), mk('k2', { x: 300 })]))
    placesel.pickHit({ refs: [], texts: [], stickers: ['k1'] }, false)
    expect(placesel.stickers).toEqual(['k1'])
    placesel.pickHit({ refs: [], texts: [], stickers: ['k2'] }, true)
    expect(placesel.stickers).toEqual(['k1', 'k2'])
    placesel.pickHit({ refs: [], texts: [] }, false)
    expect(placesel.stickers).toEqual([])
  })

  it('ключ выделения для угла группы зависит от стикеров', () => {
    placesel.pickSticker('k1', false)
    const a = placesel.key
    placesel.pickSticker('k1', false)
    expect(placesel.key).toBe(a)
    store.apply(view([mk('k1'), mk('k2')]))
    placesel.pickSticker('k2', true)
    expect(placesel.key).not.toBe(a)
  })
})

describe('перенос', () => {
  it('предпросмотр идёт без вызова Go, на отпускании — одна правка рамки', async () => {
    sticker.move(down(100, 100), root, st1())
    fire('pointermove', { clientX: 130, clientY: 110, altKey: true })
    expect(placesel.liveStickers?.get('k1')).toMatchObject({ x: 130, y: 110, w: 40, h: 20 })
    expect(api.SetStickerBox).not.toHaveBeenCalled()
    fire('pointerup')
    await vi.waitFor(() => expect(api.SetStickerBox).toHaveBeenCalledTimes(1))
    expect(api.SetStickerBox).toHaveBeenCalledWith('s1', 'k1', 130, 110, 40, 20, 0, '')
    await vi.waitFor(() => expect(placesel.liveStickers).toBeNull())
  })

  it('щелчок без движения — правки нет, зовётся onClick', async () => {
    const click = vi.fn()
    sticker.move(down(100, 100), root, st1(), click)
    fire('pointerup')
    await Promise.resolve()
    expect(api.SetStickerBox).not.toHaveBeenCalled()
    expect(click).toHaveBeenCalledTimes(1)
  })

  it('Esc отменяет жест и не снимает выделение', async () => {
    placesel.pickSticker('k1', false)
    sticker.move(down(100, 100), root, st1())
    fire('pointermove', { clientX: 150, clientY: 100, altKey: true })
    fire('keydown', { key: 'Escape' })
    expect(placesel.liveStickers).toBeNull()
    fire('pointerup')
    await Promise.resolve()
    expect(api.SetStickerBox).not.toHaveBeenCalled()
    expect(placesel.stickers).toEqual(['k1'])
  })

  it('вернули на место — холостая правка не уходит в Go', async () => {
    sticker.move(down(100, 100), root, st1())
    fire('pointermove', { clientX: 150, clientY: 100, altKey: true })
    fire('pointermove', { clientX: 100, clientY: 100, altKey: true })
    fire('pointerup')
    await Promise.resolve()
    await Promise.resolve()
    expect(api.SetStickerBox).not.toHaveBeenCalled()
  })

  it('Shift держит ось', async () => {
    sticker.move(down(100, 100), root, st1())
    fire('pointermove', { clientX: 160, clientY: 110, altKey: true, shiftKey: true })
    expect(placesel.liveStickers?.get('k1')).toMatchObject({ x: 160, y: 100 })
    fire('pointerup')
    await vi.waitFor(() => expect(api.SetStickerBox).toHaveBeenCalledWith('s1', 'k1', 160, 100, 40, 20, 0, ''))
  })

  it('далеко за край: центр остаётся на листе', async () => {
    sticker.move(down(100, 100), root, st1())
    fire('pointermove', { clientX: 5000, clientY: 100, altKey: true })
    fire('pointerup')
    await vi.waitFor(() => expect(api.SetStickerBox).toHaveBeenCalledWith('s1', 'k1', 580, 100, 40, 20, 0, ''))
  })

  it('с привязкой правый край встаёт на шов, направляющая видна во время жеста', async () => {
    placesel.snap = true
    store.apply(view([mk('k1', { x: 100, w: 50, h: 50 })]))
    sticker.move(down(100, 100), root, st1())
    fire('pointermove', { clientX: 249, clientY: 100 })
    expect(placesel.guides.x).toBe(300)
    expect(placesel.liveStickers?.get('k1')?.x).toBe(250)
    fire('pointerup')
    await vi.waitFor(() => expect(api.SetStickerBox).toHaveBeenCalled())
    expect(placesel.guides.x).toBeNull()
  })
})

describe('размер и угол', () => {
  it('угол: пропорция держится, противоположный угол стоит', async () => {
    sticker.resize(down(140, 120), root, st1(), 'se')
    fire('pointermove', { clientX: 160, clientY: 120 })
    expect(placesel.liveStickers?.get('k1')).toMatchObject({ x: 100, y: 100, w: 60, h: 30 })
    fire('pointerup')
    await vi.waitFor(() => expect(api.SetStickerBox).toHaveBeenCalledWith('s1', 'k1', 100, 100, 60, 30, 0, ''))
  })

  it('растяжимый (плашка): стороны тянутся каждая сама', async () => {
    store.apply(view([mk('k1', { stretch: true })]))
    sticker.resize(down(140, 120), root, st1(), 'se')
    fire('pointermove', { clientX: 160, clientY: 150 })
    fire('pointerup')
    await vi.waitFor(() => expect(api.SetStickerBox).toHaveBeenCalledWith('s1', 'k1', 100, 100, 60, 50, 0, ''))
  })

  it('⟳: угол следит за курсором вокруг центра, Shift — шаг 15°', async () => {
    sticker.rotate(down(120, 60), root, st1()) // над серединой верхней стороны
    fire('pointermove', { clientX: 170, clientY: 110, shiftKey: true })
    expect(placesel.liveStickers?.get('k1')?.rot).toBe(90)
    fire('pointerup')
    await vi.waitFor(() => expect(api.SetStickerBox).toHaveBeenCalledWith('s1', 'k1', 100, 100, 40, 20, 90, ''))
  })

  it('двойной щелчок по ⟳ — угол 0°; у прямого стикера правки нет', async () => {
    store.apply(view([mk('k1', { rot: 30 })]))
    await sticker.zero(st1())
    expect(api.SetStickerBox).toHaveBeenCalledWith('s1', 'k1', 100, 100, 40, 20, 0, '')
    api.SetStickerBox.mockClear()
    store.apply(view([mk('k1')]))
    await sticker.zero(st1())
    expect(api.SetStickerBox).not.toHaveBeenCalled()
  })
})

describe('вызовы Go', () => {
  it('добавить: вид из ответа применяется, новый стикер выделен', async () => {
    api.AddSticker.mockImplementationOnce(async () => ({ view: view([mk('k1'), mk('k2')]), id: 'k2' }))
    expect(await sticker.add('s1', 1, 'sa:heart')).toBe(true)
    expect(api.AddSticker).toHaveBeenCalledWith('s1', 1, 'sa:heart')
    expect(store.spread?.stickers?.map((s) => s.id)).toEqual(['k1', 'k2'])
    expect(placesel.stickers).toEqual(['k2'])
  })

  it('добавить: отказ Go — тост, выделение прежнее', async () => {
    placesel.pickSticker('k1', false)
    api.AddSticker.mockImplementationOnce(async () => {
      throw new Error('стикеров на листе уже 64')
    })
    expect(await sticker.add('s1', 0, 'sa:heart')).toBe(false)
    expect(store.toast?.text).toContain('64')
    expect(placesel.stickers).toEqual(['k1'])
  })

  it('заменить, цвета, зеркало', async () => {
    await sticker.replace('s1', 'k1', 'my:0123456789abcdef')
    expect(api.ReplaceSticker).toHaveBeenCalledWith('s1', 'k1', 'my:0123456789abcdef')
    await sticker.setInks(st1(), ['#112233', '#00aa00'])
    expect(api.SetStickerInks).toHaveBeenCalledWith('s1', 'k1', ['#112233', '#00aa00'], '')
    placesel.pickSticker('k1', false)
    await placesel.flip('v')
    expect(api.FlipSticker).toHaveBeenCalledWith('s1', 'k1', 'v')
  })

  it('зеркало без выделенного стикера стикер не трогает', async () => {
    await sticker.flip('h')
    expect(api.FlipSticker).not.toHaveBeenCalled()
  })

  it('прозрачность: серия идёт с merge-сессией, просмотр снимается после ответа', async () => {
    stickerFadeLive.set('s1', 'k1', 0.4)
    expect(stickerFadeLive.of('s1', 'k1', 0)).toBe(0.4)
    await sticker.setFade(st1(), 0.4, 'pt-abc')
    expect(api.SetStickerFade).toHaveBeenCalledWith('s1', 'k1', 0.4, 'pt-abc')
    expect(stickerFadeLive.of('s1', 'k1', 0)).toBe(0)
  })

  it('target: только при одном выделенном', () => {
    expect(sticker.target()).toBeNull()
    placesel.pickSticker('k1', false)
    expect(sticker.target()).toEqual({ spread: 's1', id: 'k1' })
  })

  it('страница для добавления — под курсором', () => {
    placesel.page = 1
    expect(currentPage()).toBe(1)
    placesel.page = 0
    expect(currentPage()).toBe(0)
  })
})

describe('Delete, слои, стрелки', () => {
  it('Delete: стикеры уходят в texts вместе с текстами, один вызов', async () => {
    store.apply(view([mk('k1'), mk('k2')]))
    placesel.pickSticker('k1', false)
    placesel.pickSticker('k2', true)
    await placesel.deleteSelected()
    expect(api.DeleteSelection).toHaveBeenCalledWith('s1', [], [], ['k1', 'k2'])
  })

  it('слой: ID стикера идёт в ArrangeLayers — и на «Авто», и на «Ручной»', async () => {
    placesel.pickSticker('k1', false)
    expect(placesel.canLayer).toBe(true) // «Авто»: стикер лежит поверх, Go порядок хранит (сц. 14–15)
    await placesel.layer('up')
    expect(api.ArrangeLayers).toHaveBeenCalledWith('s1', 'up', ['k1'])
    store.apply(view([mk('k1')], true))
    placesel.pickSticker('k1', false)
    expect(placesel.canLayer).toBe(true)
    await placesel.layer('front')
    expect(api.ArrangeLayers).toHaveBeenLastCalledWith('s1', 'front', ['k1'])
  })

  it('слой на «Авто»: стикер + текст — можно; стикер + фото места или одно фото — нельзя; ничего — нельзя', async () => {
    expect(placesel.canLayer).toBe(false) // пусто
    store.apply(view([mk('k1')], false, [{ id: 't1' }]))
    placesel.pickSticker('k1', false)
    placesel.texts = ['t1']
    expect(placesel.canLayer).toBe(true)
    await placesel.layer('back')
    expect(api.ArrangeLayers).toHaveBeenLastCalledWith('s1', 'back', ['t1', 'k1'])
    store.apply(withPhoto())
    placesel.select(['p1'])
    expect(placesel.canLayer).toBe(false) // одно фото «Авто»
    placesel.pickSticker('k1', true)
    expect(placesel.refs).toEqual(['p1'])
    expect(placesel.canLayer).toBe(false) // группа с фото места: ячейки Авто по слоям не двигаем
    api.ArrangeLayers.mockClear()
    await placesel.layer('up')
    expect(api.ArrangeLayers).not.toHaveBeenCalled()
  })

  it('стрелки двигают стикер на «Авто» и в одной серии — одна сессия', async () => {
    placesel.pickSticker('k1', false)
    placesel.nudge('ArrowRight', false)
    placesel.nudge('ArrowRight', true)
    await placesel.queue(async () => {})
    expect(api.SetStickerBox).toHaveBeenCalledTimes(2)
    const [a, b] = api.SetStickerBox.mock.calls
    expect(a.slice(0, 7)).toEqual(['s1', 'k1', 101, 100, 40, 20, 0])
    expect(a[7]).toMatch(/^[A-Za-z0-9-]{1,32}$/)
    expect(b[7]).toBe(a[7])
  })

  it('стрелки без выделенного стикера на «Авто» молчат', async () => {
    placesel.nudge('ArrowRight', false)
    await placesel.queue(async () => {})
    expect(api.SetStickerBox).not.toHaveBeenCalled()
  })

  it('группа стикеров: стрелки — один TransformGroup со всеми ID', async () => {
    store.apply(view([mk('k1'), mk('k2')]))
    placesel.pickSticker('k1', false)
    placesel.pickSticker('k2', true)
    placesel.nudge('ArrowDown', true)
    await placesel.queue(async () => {})
    expect(api.TransformGroup).toHaveBeenCalledTimes(1)
    const c = api.TransformGroup.mock.calls[0]
    expect(c.slice(0, 7)).toEqual(['s1', ['k1', 'k2'], 1, 0, 0, 0, 10])
  })
})

describe('группа', () => {
  it('стикеры — члены группы для Go и для предпросмотра', () => {
    store.apply(view([mk('k1'), mk('k2', { x: 300, rot: 30 })]))
    placesel.pickSticker('k1', false)
    placesel.pickSticker('k2', true)
    const s = store.spread!
    expect(groupIds(s)).toEqual(['k1', 'k2'])
    const m = groupMembers(s)
    expect(m.stickerIds).toEqual(['k1', 'k2'])
    expect(m.in.places).toEqual([
      { x: 100, y: 100, w: 40, h: 20, rot: 0 },
      { x: 300, y: 100, w: 40, h: 20, rot: 30 },
    ])
  })

  it('рамка группы охватывает стикеры, живой предпросмотр её двигает', () => {
    store.apply(view([mk('k1'), mk('k2', { x: 300 })]))
    placesel.pickSticker('k1', false)
    placesel.pickSticker('k2', true)
    const f = placesel.frame
    expect(f).not.toBeNull()
    expect(f!.x).toBeCloseTo(100, 6)
    expect(f!.w).toBeCloseTo(240, 6)
    placesel.liveStickers = new Map([['k1', { x: 120, y: 100, w: 40, h: 20, rot: 0 }]])
    expect(placesel.frame!.x).toBeCloseTo(120, 6)
    expect(placesel.boxOf([])?.x).toBeCloseTo(120, 6)
  })
})
