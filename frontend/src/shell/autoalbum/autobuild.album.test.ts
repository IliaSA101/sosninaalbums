import { describe, expect, it } from 'vitest'
import type { AutoPlan, PhotoOpts } from '../../base/core/types'
import { albumIds } from '../../base/auto/albumpick'
import { AutoDialog, type PlanFn } from '../autobuild/autobuild.svelte'
import { albumKept, albumLine, baseLabel, freeLabel, NOTHING_TO_REBUILD, rebuildAskLines, untouchedText } from './autoalbum'
import { albumAfter, arrow } from '../autobuild/autobuild'
import { unitOf } from '../../base/geom/sheet'

// 107 U2/U3: тексты режима альбома, состав ids (design §2.3), N по умолчанию и поправка плана.
const A = { spreads: 22, photos: 161, free: 0, manual: 3, vignettes: 1, decor: 2, blank: 0 }
const settle = () => new Promise((r) => setTimeout(r, 0))

describe('строка источника «Весь альбом» (сц. 5, 7)', () => {
  it('свободные в пуле входят в число фото (107-qa-1): 224 + 2 = 226, как в альбоме после сборки с галочкой', () => {
    expect(albumLine({ ...A, spreads: 46, photos: 224, free: 2, manual: 0, vignettes: 0, decor: 0 }, false)).toBe(
      'Весь альбом: 226 фото на 46 разворотах',
    )
  })
  it('число фото, разворотов и всё, что не трогается', () => {
    expect(albumLine(A, true)).toBe(
      'Весь альбом: 161 фото на 22 разворотах; не трогаю: обложку, 3 ручных, 1 виньетку, 2 оформленных (надписи, стикеры, свой фон, название)',
    )
  })

  it('только ненулевые части; без обложки — без обложки; нечего не трогать — без «не трогаю»', () => {
    expect(albumLine({ ...A, spreads: 21, photos: 100, manual: 0, vignettes: 0, decor: 0, blank: 2 }, false)).toBe(
      'Весь альбом: 100 фото на 21 развороте; не трогаю: 2 пустых',
    )
    expect(albumLine({ ...A, manual: 0, vignettes: 0, decor: 0, blank: 0 }, false)).toBe('Весь альбом: 161 фото на 22 разворотах')
    expect(albumLine({ ...A, manual: 1, vignettes: 5, decor: 1, blank: 1, spreads: 2 }, false)).toContain(
      '1 ручной, 5 виньеток, 1 оформленный (надписи, стикеры, свой фон, название), 1 пустой',
    )
  })

  it('листы виньеток: «на листах»', () => {
    expect(albumLine({ ...A, spreads: 3, manual: 0, vignettes: 0, decor: 0 }, false, unitOf({ sheet: { w: 1, h: 1 } } as never))).toContain(
      'на 3 листах',
    )
  })

  it('голых нет — объяснение; плана ещё нет — название режима', () => {
    expect(albumLine({ ...A, spreads: 0, photos: 0 }, true)).toBe(NOTHING_TO_REBUILD)
    expect(NOTHING_TO_REBUILD).toContain('Вернуть авто')
    expect(albumLine(undefined, true)).toBe('Весь альбом заново')
  })

  it('якоря в «станет»: развороты на месте плюс новые; пустой набор якорей — строки нет', () => {
    expect(albumKept(A)).toBe(6)
    expect(albumAfter(albumKept(A), 22)).toBe('В альбоме станет 28 разворотов (56 стр.)')
    expect(albumAfter(0, 22)).toBe('')
  })

  it('untouchedText: ничего — пусто', () => {
    expect(untouchedText({ manual: 0, vignettes: 0, decor: 0, blank: 0 }, false)).toBe('')
  })
})

describe('вопрос «Пересобрать альбом?» (сц. 8)', () => {
  it('числа, не трогаю, похожие, отмена', () => {
    expect(rebuildAskLines(A, 22, 37, true, 'Ctrl+Z')).toEqual([
      '22 разворота соберутся заново: раскладки и фото на них сменятся',
      'Не трогаю: обложку, 3 ручных, 1 виньетку, 2 оформленных (надписи, стикеры, свой фон, название)',
      'Похожих кадров уйдёт в ленту: 37',
      'Отменить — Ctrl+Z',
    ])
  })

  it('один разворот — «соберётся»; N отличается — строка «Станет»; без похожих и якорей — только главное', () => {
    const lines = rebuildAskLines({ ...A, spreads: 1, manual: 0, vignettes: 0, decor: 0 }, 3, 0, false, '')
    expect(lines).toEqual([
      '1 разворот соберётся заново: раскладки и фото на них сменятся',
      'Станет: 3 разворота',
      'Отменить — «Правка ▸ Отменить»',
    ])
  })
})

describe('подписи «Собрать:»', () => {
  const ids = ['a', 'b', 'c']
  it('первый пункт — выбор, к которому вернуться', () => {
    expect(baseLabel({ from: 'free', ids })).toBe('Свободные фото: 3')
    expect(baseLabel({ from: 'selected', ids })).toBe('Выделенные: 3')
    expect(baseLabel({ from: 'free', ids, bin: 'Церковь' })).toBe('Подборка «Церковь»: 3')
    expect(freeLabel(4)).toBe('Добавить свободные фото ленты: 4')
  })

  it('стрелки: шаг 1, PgUp/PgDn — 5', () => {
    expect(arrow('ArrowUp')).toEqual([1, 1])
    expect(arrow('PageDown')).toEqual([-1, 5])
    expect(arrow('a')).toEqual([0, 1])
  })
})

describe('ids режима альбома (design §2.3)', () => {
  const lib = ['a', 'b', 'c', 'd', 'e'].map((id) => ({ id }))
  const used = new Set(['c', 'a'])
  it('фото альбома — в порядке ленты', () => {
    expect(albumIds(lib, used, ['d'], false)).toEqual(['a', 'c'])
  })
  it('свободные — только с галочкой и тоже по ленте', () => {
    expect(albumIds(lib, used, ['e', 'd'], true)).toEqual(['a', 'c', 'd', 'e'])
  })
  it('пустая лента и нет свободных — пусто', () => {
    expect(albumIds([], used, [], true)).toEqual([])
    expect(albumIds(lib, new Set(), [], false)).toEqual([])
  })
})

describe('окно в режиме альбома (AutoDialog)', () => {
  const plan = (p: Partial<AutoPlan> = {}): AutoPlan => ({
    photos: 161,
    spreads: 22,
    minSpreads: 10,
    maxSpreads: 60,
    avg: 7,
    min: 3,
    max: 12,
    reuse: 0,
    from: 3,
    to: 12,
    cuts: 0,
    album: A,
    ...p,
  })
  const ids = Array.from({ length: 161 }, (_, i) => `p${i}`)
  function make(
    saved: { spreads?: number | null; again?: boolean; album?: boolean } = { album: true },
    answer: (n: number) => AutoPlan = (n) => plan({ spreads: n }),
  ) {
    const asked: { n: number; ph: PhotoOpts }[] = []
    const planFn: PlanFn = async (_i, n, _r, ph) => {
      asked.push({ n, ph })
      return answer(n)
    }
    const d = new AutoDialog(ids, planFn, { spreads: null, ...saved })
    return { d, asked }
  }

  it('album: true уходит в каждый запрос; N по умолчанию — сколько голых сейчас (сц. 6)', async () => {
    const { d, asked } = make()
    d.start()
    await settle()
    expect(d.n).toBe(22)
    expect(asked.every((a) => a.ph.album === true)).toBe(true)
    expect(d.plan?.album).toEqual(A)
    expect(d.canBuild).toBe(true)
  })

  it('обычное окно album не шлёт', async () => {
    const { d, asked } = make({}, () => plan({ album: undefined }))
    d.start()
    await settle()
    expect(asked[0].ph.album).toBeUndefined()
  })

  it('N руками остаётся; ползунок зажат пределами плана', async () => {
    const { d } = make()
    d.start()
    await settle()
    d.setSpreads(30)
    await settle()
    expect(d.n).toBe(30)
    d.setSpreads(500)
    await settle()
    expect(d.n).toBe(60)
  })

  it('план поднял N выше запрошенного — окно показывает итог плана', async () => {
    const { d } = make({ album: true }, (n) => plan({ spreads: Math.max(n, 24), minSpreads: 10 }))
    d.start()
    await settle()
    d.setSpreads(15)
    await settle()
    expect(d.n).toBe(24)
    expect(d.plan?.spreads).toBe(24)
  })

  it('голых нет: фото 0 — собрать нельзя', async () => {
    const { d } = make({ album: true }, () =>
      plan({ photos: 0, spreads: 0, minSpreads: 0, maxSpreads: 0, album: { ...A, spreads: 0, photos: 0 } }),
    )
    d.start()
    await settle()
    expect(d.canBuild).toBe(false)
  })

  it('пересборка держит N прошлой сборки, а не число голых', async () => {
    const { d } = make({ album: true, again: true, spreads: 26 }, () => plan({ spreads: 26 }))
    d.start()
    await settle()
    expect(d.n).toBe(26)
    expect(d.ph.opts).toMatchObject({ again: true, album: true })
  })
})
