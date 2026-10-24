import { beforeEach, describe, expect, it, vi } from 'vitest'

// Галерея сторонки обложки (127-17): доли × зона → мм листа, вкладка по умолчанию, открытие/листание/закрытие.
// Go подменён: StepCoverGrid записывается и отвечает следующим видом.
const calls: string[] = []

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  StepCoverGrid: async (scope: string, delta: number) => {
    calls.push(`StepCoverGrid:${scope}:${delta}`)
    return view(7)
  },
  LayoutChoices: async () => ({}),
  Marks: async () => ({}),
}))
vi.mock('../../editor/text/fonts', () => ({ loadFonts: async () => {} }))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { gallery } = await import('./gallery.svelte')
const { openN } = await import('../layouts/coverbar')
const { boxRects, coverPreview, coverSourceTexts } = await import('./gridgallery')

type Side = { scope: 'left' | 'right'; label: string; box: object; places: number; known: boolean; alone: boolean }
const side = (scope: 'left' | 'right', o: Partial<Side> = {}): Side => ({
  scope,
  label: scope === 'left' ? 'Задняя' : 'Лицевая',
  box: { x: scope === 'left' ? 10 : 160, y: 10, w: 130, h: 280 },
  places: 3,
  known: true,
  alone: false,
  ...o,
})
const view = (rev: number, sides: Side[] = [side('left'), side('right')], cover = true) =>
  ({
    rev,
    marksRev: 0,
    folders: [],
    settings: { sheet: false },
    spreads: [{ id: 's1', photos: [], layout: 'k', w: 300, h: 300 }],
    cover: cover ? { sheet: { id: 'cover', photos: [], rects: [], w: 300, h: 300, mid: 150 }, sides, spine: 10 } : null,
  }) as never

beforeEach(() => {
  calls.length = 0
  gallery.close()
  store.toast = null
  store.onCover = false
  store.apply(view(5))
  store.goCover()
  gallery.close() // goCover уже закрыл через хук; держим явно
})

describe('boxRects: доли зоны → мм листа', () => {
  it('сдвиг на угол зоны и масштаб по её размеру', () => {
    const r = boxRects({ boxes: [{ x: 0.5, y: 0.25, w: 0.5, h: 0.5 }] }, { x: 160, y: 10, w: 130, h: 280 })
    expect(r).toEqual([{ x: 225, y: 80, w: 65, h: 140 }])
  })

  it('угол, форма и скругление едут вместе с коробкой, у обычной их нет', () => {
    const r = boxRects(
      {
        boxes: [
          { x: 0, y: 0, w: 1, h: 1 },
          { x: 0, y: 0, w: 0.5, h: 0.5, rot: 15, shape: 'oval', round: 3 },
        ],
      },
      { x: 10, y: 10, w: 100, h: 100 },
    )
    expect(r[0]).toEqual({ x: 10, y: 10, w: 100, h: 100 })
    expect(r[1]).toMatchObject({ rot: 15, shape: 'oval', round: 3 })
  })
})

describe('coverPreview: предпросмотр на всём листе обложки', () => {
  const sheet = {
    rects: [
      { x: 10, y: 10, w: 50, h: 50 }, // задняя
      { x: 170, y: 10, w: 50, h: 50 }, // лицевая
      { x: 230, y: 10, w: 50, h: 50 }, // лицевая
      { x: 170, y: 100, w: 50, h: 50 }, // лицевая
    ],
  }
  const ch = { boxes: [{ x: 0, y: 0, w: 1, h: 0.5 }] }
  const box = { x: 160, y: 10, w: 130, h: 280 }

  it('фото лицевой по порядку встают в новые места, лишние схлопываются, задняя цела', () => {
    const r = coverPreview(ch, box, sheet)
    expect(r[0]).toBe(sheet.rects[0])
    expect(r[1]).toEqual({ x: 160, y: 10, w: 130, h: 140 })
    expect(r[2].w).toBeLessThan(1)
    expect(r[3].w).toBeLessThan(1)
    expect(r).toHaveLength(4)
  })

  it('для задней — наоборот', () => {
    const r = coverPreview(ch, { x: 10, y: 10, w: 130, h: 280 }, sheet)
    expect(r[0]).toEqual({ x: 10, y: 10, w: 130, h: 140 })
    expect(r.slice(1)).toEqual(sheet.rects.slice(1))
  })

  it('фото на корешке или в расставе (центр вне рамки) не трогается, даже если лист делится иначе', () => {
    // Центр 150 — ровно середина листа, но рамки задней (10..140) и лицевой (160..290) его не содержат.
    const spine = { x: 130, y: 10, w: 40, h: 50 }
    const gap = { x: 140, y: 100, w: 20, h: 50 } // центр 150, в расставе
    const s = { rects: [sheet.rects[1], spine, gap, sheet.rects[0]] }
    const front = coverPreview(ch, box, s)
    expect(front[0]).toEqual({ x: 160, y: 10, w: 130, h: 140 })
    expect(front[1]).toBe(spine)
    expect(front[2]).toBe(gap)
    expect(front[3]).toBe(sheet.rects[0])
    const back = coverPreview(ch, { x: 10, y: 10, w: 130, h: 280 }, s)
    expect(back[1]).toBe(spine)
    expect(back[2]).toBe(gap)
    expect(back[0]).toBe(sheet.rects[1])
    expect(back[3]).toEqual({ x: 10, y: 10, w: 130, h: 140 })
  })

  it('место за пределами по вертикали (центр вне рамки) тоже не трогается', () => {
    const low = { x: 170, y: 295, w: 50, h: 20 }
    expect(coverPreview(ch, box, { rects: [low] })[0]).toBe(low)
  })

  it('повёрнутые места другой сторонки сохраняют наклон из frames', () => {
    const s = { ...sheet, frames: [{ cellRot: 12 }, { cellRot: 0 }, { cellRot: 0 }, { cellRot: 0 }] }
    const r = coverPreview(ch, box, s)
    expect(r[0]).toEqual({ ...sheet.rects[0], rot: 12 })
    expect(r[1]).not.toHaveProperty('rot')
  })

  it('угол, уже стоящий в rect, не затирается; нулевой угол поля не добавляет', () => {
    const own = { x: 10, y: 10, w: 50, h: 50, rot: 7 }
    const s = { rects: [own, sheet.rects[0]], frames: [{ cellRot: 99 }, { cellRot: 0 }] }
    const r = coverPreview(ch, box, s)
    expect(r[0]).toBe(own)
    expect(r[1]).toBe(sheet.rects[0])
  })
})

describe('вкладка по умолчанию', () => {
  it('число мест сторонки; 0 — «1»; много — «7+»', () => {
    expect(openN({ places: 3 })).toBe(3)
    expect(openN({ places: 0 })).toBe(1)
    expect(openN({ places: 12 })).toBe(7)
  })
})

describe('серый «По фото»: причина', () => {
  it('нет мест, другая вкладка, больше 16', () => {
    expect(coverSourceTexts(0, 1).off.byPhoto).toBe('На сторонке нет мест: выберите раскладку во «Встроенных»')
    expect(coverSourceTexts(5, 2).off.byPhoto).toBe('На сторонке 5 мест — откройте вкладку «5»')
    expect(coverSourceTexts(1, 3).off.byPhoto).toBe('На сторонке 1 место — откройте вкладку «1»')
    expect(coverSourceTexts(9, 2).off.byPhoto).toBe('На сторонке 9 мест — откройте вкладку «7+»')
    expect(coverSourceTexts(17, 7).off.byPhoto).toBe('Раскладки по фото — до 16 мест на сторонке')
  })

  it('вкладка своя, а ряда нет — общая причина, не ложная «откройте вкладку»', () => {
    expect(coverSourceTexts(3, 3).off.byPhoto).toBe('Для этих фото раскладок по фото нет')
  })
})

describe('gallery.open у обложки', () => {
  it('▦ сторонки открывает «Шаблоны» сторонки, повторный вызов закрывает, другая сторонка переключает', () => {
    gallery.open('right', 'grids')
    expect(gallery.cur).toMatchObject({ spread: 'cover', scope: 'right', tab: 'grids' })
    gallery.open('left', 'grids')
    expect(gallery.cur).toMatchObject({ spread: 'cover', scope: 'left' })
    gallery.open('left', 'grids')
    expect(gallery.cur).toBeNull()
  })

  it('раскладки «Авто», вся обложка и сторонка без зоны не открываются', () => {
    gallery.open('right', 'layouts')
    expect(gallery.cur).toBeNull()
    gallery.open('spread', 'grids')
    expect(gallery.cur).toBeNull()
    store.apply(view(6, [side('right')]))
    gallery.open('left', 'grids')
    expect(gallery.cur).toBeNull()
  })

  it('у обычного разворота ничего не изменилось: ▦ открывает сетки', () => {
    store.go(0)
    gallery.open('spread', 'grids')
    expect(gallery.cur).toMatchObject({ spread: 's1', tab: 'grids' })
  })
})

describe('gallery.sync: закрытие при пропаже', () => {
  it('галерея живёт, пока в виде есть сторонка; обложка пропала — закрывается', () => {
    gallery.open('right', 'grids')
    gallery.sync()
    expect(gallery.cur).not.toBeNull()
    store.apply(view(6))
    gallery.sync()
    expect(gallery.cur).not.toBeNull()
    store.apply(view(7, [side('left')]))
    gallery.sync()
    expect(gallery.cur).toBeNull()

    gallery.open('left', 'grids')
    store.apply(view(8, [], false))
    gallery.sync()
    expect(gallery.cur).toBeNull()
  })
})

describe('gallery.coverStep (‹ ›, Пробел)', () => {
  it('форма узнана — Go листает', async () => {
    await gallery.coverStep('right', 1)
    await gallery.coverStep('right', -1)
    expect(calls).toEqual(['StepCoverGrid:right:1', 'StepCoverGrid:right:-1'])
    expect(gallery.cur).toBeNull()
  })

  it('не узнана — Go не зовётся, открывается ▦ сторонки; повтор её не закрывает', async () => {
    store.apply(view(6, [side('left'), side('right', { known: false })]))
    await gallery.coverStep('right', 1)
    expect(calls).toEqual([])
    expect(gallery.cur).toMatchObject({ scope: 'right', tab: 'grids' })
    await gallery.coverStep('right', 1)
    expect(gallery.cur).toMatchObject({ scope: 'right', tab: 'grids' })
  })

  it('0 мест — ▦ без правки, места сами не появляются', async () => {
    store.apply(view(6, [side('left'), side('right', { places: 0, known: false })]))
    await gallery.coverStep('right', -1)
    expect(calls).toEqual([])
    expect(gallery.cur).toMatchObject({ scope: 'right' })
  })

  it('одна раскладка — тост, правки и ▦ нет', async () => {
    store.apply(view(6, [side('left'), side('right', { places: 1, alone: true })]))
    await gallery.coverStep('right', 1)
    expect(calls).toEqual([])
    expect(gallery.cur).toBeNull()
    expect(store.toast?.text).toBe('Других раскладок для этих фото нет')
  })

  it('сторонки нет — молчит', async () => {
    store.apply(view(6, [side('left')]))
    await gallery.coverStep('right', 1)
    expect(calls).toEqual([])
    expect(gallery.cur).toBeNull()
    expect(store.toast).toBeNull()
  })
})
