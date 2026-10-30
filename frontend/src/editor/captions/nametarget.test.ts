import { describe, expect, it } from 'vitest'
import {
  captionBottom,
  captionBoxes,
  nameTargetAt,
  namePlace,
  takesNameTarget,
  targetKey,
  whoOf,
  whoOfPlace,
  type PointCtx,
  type Who,
} from './nametarget'
import type { CaptionView, SpreadView, StubView } from '../../base/core/types'

const style = { font: 'f', size: 12, color: '#000', align: 'center' as const }
const box = (x: number, y: number, w: number, h: number, z: number, rot = 0) => ({ id: '', x, y, w, h, rot, z, style, lines: [] })

const cap = (photo: string, x: number, y: number, w: number, h: number, who?: Who[], rot = 0): CaptionView =>
  ({ ...box(x, y, w, h, 1, rot), photo, who }) as unknown as CaptionView
const stub = (place: string, photo: string, x: number, y: number, w: number, h: number, who?: Who[], rot = 0): StubView =>
  ({ ...box(x, y, w, h, 1, rot), place, photo, who }) as unknown as StubView

const sheet = (captions: CaptionView[], stubs: StubView[] = []): SpreadView => ({ id: 's', captions, stubs }) as unknown as SpreadView

describe('nameTargetAt', () => {
  it('двойной щелчок по заглушке — ввод имени, не вписать', () => {
    const s = sheet([], [stub('p1', 'a', 10, 100, 60, 12, ['last', 'first'])])
    const t = nameTargetAt(s, 40, 105)
    expect(t).not.toBeNull()
    expect(t?.photo).toBe('a')
    expect(t?.place).toBe('')
    expect(t?.who).toEqual(['last', 'first'])
  })

  it('подпись по шаблону — цель; своя подпись и шаблон без полей имени — нет', () => {
    const s = sheet([cap('a', 10, 100, 60, 12, ['last']), cap('b', 100, 100, 60, 12), cap('c', 200, 100, 60, 12, [])])
    expect(nameTargetAt(s, 40, 105)?.photo).toBe('a')
    expect(nameTargetAt(s, 130, 105)).toBeNull()
    expect(nameTargetAt(s, 230, 105)).toBeNull()
  })

  it('заглушка пустого места с who — цель-место (071): без фото, с ID места и полями шаблона', () => {
    const s = sheet([cap('a', 100, 100, 60, 12, ['last'])], [stub('p2', '', 10, 100, 60, 12, ['last', 'first'])])
    const t = nameTargetAt(s, 40, 105)
    expect(t).not.toBeNull()
    expect(t?.photo).toBe('')
    expect(t?.place).toBe('p2')
    expect(t?.who).toEqual(['last', 'first'])
    expect(t?.box).toEqual({ x: 10, y: 100, w: 60, h: 12 })
  })

  it('заглушка фото на месте с табличкой — цель-фото (place пуст): поля правят табличку через Go', () => {
    const s = sheet([], [stub('p1', 'a', 10, 100, 60, 12, ['last'])])
    const t = nameTargetAt(s, 40, 105)
    expect(t?.photo).toBe('a')
    expect(t?.place).toBe('')
  })

  it('заглушка пустого места без who (шаблона с именем нет) — не цель (остаётся «вписать»)', () => {
    const s = sheet([cap('a', 100, 100, 60, 12)], [stub('p2', '', 10, 100, 60, 12)])
    expect(nameTargetAt(s, 40, 105)).toBeNull()
  })

  it('точка между подписями и пустое поле — null', () => {
    const s = sheet([cap('a', 10, 100, 60, 12, ['last'])], [stub('p2', 'b', 100, 100, 60, 12, ['last'])])
    expect(nameTargetAt(s, 85, 105)).toBeNull()
    expect(nameTargetAt(s, 40, 50)).toBeNull()
  })

  it('повёрнутое место: подпись повёрнута вокруг своего угла', () => {
    // 60×12 вокруг (100, 100) на 90°: занимает x 88..100, y 100..160.
    const s = sheet([], [stub('p1', 'a', 100, 100, 60, 12, ['last'], 90)])
    expect(nameTargetAt(s, 94, 150)?.photo).toBe('a')
    // Внутри неповёрнутой рамки, но вне повёрнутой.
    expect(nameTargetAt(s, 150, 106)).toBeNull()
  })
})

describe('whoOf', () => {
  it('подпись по шаблону и заглушка фото; иначе пусто', () => {
    const s = sheet([cap('a', 0, 0, 10, 10, ['last', 'first'])], [stub('p', 'b', 0, 0, 10, 10, ['last'])])
    expect(whoOf(s, 'a')).toEqual(['last', 'first'])
    expect(whoOf(s, 'b')).toEqual(['last'])
    expect(whoOf(s, 'z')).toEqual([])
    expect(whoOf(undefined, 'a')).toEqual([])
  })
})

describe('captionBottom', () => {
  it('низ подписи и заглушки выделенных; пустое место — по заглушке места', () => {
    const s = sheet([cap('a', 10, 100, 60, 12, ['last'])], [stub('p2', '', 10, 150, 60, 12)])
    expect(captionBottom(s, [{ id: 'p1', photo: 'a' }])).toBe(112)
    expect(captionBottom(s, [{ id: 'p2', photo: '' }])).toBe(162)
    expect(captionBottom(s, [{ id: 'p3', photo: 'z' }])).toBe(0)
  })
})

describe('captionBoxes', () => {
  it('боксы подписей и заглушек выделенных; подписей нет — пусто', () => {
    const s = sheet([cap('a', 10, 100, 60, 12, ['last']), cap('a', 10, 90, 60, 8)], [stub('p2', '', 10, 150, 60, 12)])
    expect(captionBoxes(s, [{ id: 'p1', photo: 'a' }])).toEqual([
      { x: 10, y: 100, w: 60, h: 12 },
      { x: 10, y: 90, w: 60, h: 8 },
    ])
    expect(captionBoxes(s, [{ id: 'p2', photo: '' }])).toEqual([{ x: 10, y: 150, w: 60, h: 12 }])
    expect(captionBoxes(s, [{ id: 'p3', photo: 'z' }])).toEqual([])
    expect(captionBoxes(undefined, [{ id: 'p1', photo: 'a' }])).toEqual([])
  })
})

describe('targetKey / whoOfPlace', () => {
  it('ключ цели: место у пустого места, иначе фото', () => {
    expect(targetKey({ photo: '', place: 'p2' })).toBe('p2')
    expect(targetKey({ photo: 'a', place: '' })).toBe('a')
  })
  it('поля имени пустого места — по заглушке без фото; занятое место и чужое — пусто', () => {
    const s = sheet([], [stub('p2', '', 0, 0, 10, 10, ['last', 'subject']), stub('p1', 'a', 0, 0, 10, 10, ['last'])])
    expect(whoOfPlace(s, 'p2')).toEqual(['last', 'subject'])
    expect(whoOfPlace(s, 'p1')).toEqual([])
    expect(whoOfPlace(s, 'zz')).toEqual([])
    expect(whoOfPlace(s, '')).toEqual([])
    expect(whoOfPlace(undefined, 'p2')).toEqual([])
  })
})

// Go отдаёт who и пустому месту (поля имени шаблона, 071): шаблон листа и учителя фронт не разбирает.
describe('пустая виньетка (071)', () => {
  const vig = (...stubs: StubView[]): SpreadView => ({ id: 'v', kind: 'vignette', captions: [], stubs }) as unknown as SpreadView
  it('учитель: заглушка с «Предметом» — цель с предметом; ученик — без него', () => {
    const s = vig(stub('t', '', 10, 10, 60, 12, ['last', 'first', 'middle', 'subject']), stub('u1', '', 100, 10, 60, 12, ['last', 'first']))
    expect(nameTargetAt(s, 40, 15)?.who).toEqual(['last', 'first', 'middle', 'subject'])
    expect(nameTargetAt(s, 130, 15)?.place).toBe('u1')
    expect(nameTargetAt(s, 130, 15)?.who).toEqual(['last', 'first'])
  })
  it('подпись фото с «Предметом» (049): who сохраняет subject', () => {
    const s = sheet([cap('a', 10, 100, 60, 12, ['last', 'subject'])])
    expect(nameTargetAt(s, 40, 105)?.who).toEqual(['last', 'subject'])
    expect(whoOf(s, 'a')).toEqual(['last', 'subject'])
  })
  it('шаблона с полями имени нет — заглушки без who, целей нет', () => {
    expect(nameTargetAt(vig(stub('u1', '', 100, 10, 60, 12)), 130, 15)).toBeNull()
  })
})

describe('namePlace', () => {
  const b = { x: 100, y: 50, w: 40, h: 10 }
  it('под подписью по её центру', () => {
    expect(namePlace(b, 1, 400, 300, 80)).toEqual({ left: 120, top: 64 })
  })
  it('внизу листа места нет — над подписью', () => {
    const low = { x: 100, y: 270, w: 40, h: 10 }
    expect(namePlace(low, 1, 400, 300, 80)).toEqual({ left: 120, top: 270 - 4 - 80 })
  })
  it('не выходит за боковые края с учётом ширины w', () => {
    expect(namePlace({ x: 0, y: 50, w: 10, h: 10 }, 1, 400, 300, 80, 300).left).toBe(150)
    expect(namePlace({ x: 390, y: 50, w: 10, h: 10 }, 1, 400, 300, 80, 300).left).toBe(250)
  })
  it('нет места ни снизу, ни сверху — прижато к низу листа', () => {
    expect(namePlace({ x: 100, y: 40, w: 40, h: 200 }, 1, 400, 300, 120).top).toBe(180)
  })
})

describe('takesNameTarget', () => {
  const base: PointCtx = { can: true, bgEdit: false, editing: false, empty: false, onNamed: true }
  it('подпись с полями имени или пустое поле — берём', () => {
    expect(takesNameTarget(base)).toBe(true)
    expect(takesNameTarget({ ...base, onNamed: false, empty: true })).toBe(true)
  })
  it('цель выше по z (свободный текст, фото) — не берём', () => {
    expect(takesNameTarget({ ...base, onNamed: false })).toBe(false)
  })
  it('открыт редактор текста, режим фона или выключен уровень — не берём', () => {
    expect(takesNameTarget({ ...base, editing: true })).toBe(false)
    expect(takesNameTarget({ ...base, bgEdit: true })).toBe(false)
    expect(takesNameTarget({ ...base, can: false })).toBe(false)
  })
})
