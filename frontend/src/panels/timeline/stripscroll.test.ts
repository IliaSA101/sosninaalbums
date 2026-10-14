import { describe, expect, it } from 'vitest'
import { CENTER_SEL, StripFollow, shouldScroll } from './stripscroll'
import { rowScroll } from '../../editor/spread/spreadrow'

const base = { posChanged: false, movingStarted: false, scrollLeft: 500, written: 500 }

describe('shouldScroll', () => {
  it('ручная прокрутка + новый rev: не трогаем', () => {
    expect(shouldScroll({ ...base, scrollLeft: 1200 })).toBe(false)
  })
  it('новая позиция ряда: едем, даже после ручной прокрутки', () => {
    expect(shouldScroll({ ...base, scrollLeft: 1200, posChanged: true })).toBe(true)
  })
  it('начался ход ряда: едем', () => {
    expect(shouldScroll({ ...base, scrollLeft: 1200, movingStarted: true })).toBe(true)
  })
  it('прокрутки не было, новый rev/bw: едем', () => {
    expect(shouldScroll(base)).toBe(true)
    expect(shouldScroll({ ...base, scrollLeft: 500.4 })).toBe(true)
  })
  it('первая запись: едем', () => {
    expect(shouldScroll({ ...base, written: null, scrollLeft: 0 })).toBe(true)
  })
})

describe('StripFollow', () => {
  it('первый лист в свежей ленте: едем', () => {
    const f = new StripFollow()
    expect(f.next(0, false, 0)).toBe(true)
  })

  it('ручная прокрутка без хода ряда: не трогаем', () => {
    const f = new StripFollow()
    f.next(0, false, 0)
    f.wrote(244)
    expect(f.next(0, false, 244)).toBe(true) // новый rev, прокрутка наша
    expect(f.next(0, false, 900)).toBe(false) // пользователь прокрутил сам
    expect(f.next(1, false, 900)).toBe(true) // ряд сдвинулся — снова едем
  })

  // Жалоба владельца 14.10: лист 1 создан после пустой ленты (удалили все развороты, новый альбом) — браузер прижал
  // scrollLeft к 0, позиция ряда та же (0), прежняя запись 244 выглядела ручной прокруткой, лента стояла на 0.
  it('0 → 1 после пустой ленты: прежняя запись забыта, едем', () => {
    const f = new StripFollow()
    f.next(0, false, 0)
    f.wrote(244)
    f.reset() // слотов 0
    expect(f.next(0, false, 0)).toBe(true)
  })

  it('без сброса тот же переход принимается за ручную прокрутку (почему нужен reset)', () => {
    const f = new StripFollow()
    f.next(0, false, 0)
    f.wrote(244)
    expect(f.next(0, false, 0)).toBe(false)
  })

  it('начало хода — один раз', () => {
    const f = new StripFollow()
    expect(f.starting(true)).toBe(true)
    f.next(0, true, 0)
    expect(f.starting(true)).toBe(false)
    f.reset()
    expect(f.starting(true)).toBe(true)
  })
})

// Модель ленты по разметке Timeline: поле .edge (bw/2), отступ 16, плитка обложки (заглушка — не слот ряда),
// черта 17, узкая зона «в начало» 14, группы с чертами склейки 12, «+» с отступом 6, поле .edge.
type Item = { w: number; slot: boolean; name: string }
function strip(bw: number, sheets: number, cover: boolean): { centers: number[]; at: (name: string) => number; max: number } {
  const items: Item[] = [{ w: bw / 2, slot: false, name: 'edge' }]
  if (sheets + (cover ? 1 : 0) > 0) {
    items.push({ w: cover ? 98 : 122, slot: cover, name: cover ? 'cover' : 'stub' })
    items.push({ w: 17, slot: false, name: 'sep' })
  }
  if (sheets > 0) items.push({ w: 14, slot: false, name: 'lead' })
  for (let i = 0; i < sheets; i++) {
    items.push({ w: 63, slot: true, name: `g${i + 1}` })
    if (i < sheets - 1) items.push({ w: 12, slot: false, name: 'join' })
  }
  items.push({ w: 6, slot: false, name: 'gap' }, { w: 133, slot: true, name: 'add' }, { w: bw / 2, slot: false, name: 'edge' })
  let x = 16
  const mid = new Map<string, number>()
  const centers: number[] = []
  for (const it of items) {
    if (it.slot) centers.push(x + it.w / 2)
    mid.set(it.name, x + it.w / 2)
    x += it.w
  }
  return { centers, at: (n) => mid.get(n)!, max: x + 16 - bw }
}

describe('центрирование ленты под уголком', () => {
  const bw = 1920
  it('заглушка «+ Обложка» в выборке центров не участвует', () => {
    expect(CENTER_SEL).toContain('[data-cover-slot]')
    expect(CENTER_SEL).not.toContain('data-drop="cover"')
  })

  for (const cover of [false, true]) {
    const lead = cover ? 1 : 0
    for (const sheets of [1, 2]) {
      it(`${sheets} лист(а), ${cover ? 'с обложкой' : 'заглушка обложки'}: текущий лист ровно под уголком`, () => {
        const s = strip(bw, sheets, cover)
        expect(s.centers).toHaveLength(sheets + lead + 1) // слотов ряда + «+»
        for (let i = 0; i < sheets; i++) {
          const left = rowScroll(s.centers, i + lead, bw)
          expect(left).toBeGreaterThan(0)
          expect(left).toBeLessThanOrEqual(s.max)
          expect(left + bw / 2).toBe(s.at(`g${i + 1}`))
        }
        // Слот «новый» — под уголком «+»
        expect(rowScroll(s.centers, sheets + lead, bw) + bw / 2).toBe(s.at('add'))
      })
    }
  }

  it('только обложка: она под уголком, «+» — следующий слот', () => {
    const s = strip(bw, 0, true)
    expect(rowScroll(s.centers, 0, bw) + bw / 2).toBe(s.at('cover'))
    expect(rowScroll(s.centers, 1, bw) + bw / 2).toBe(s.at('add'))
  })
})
