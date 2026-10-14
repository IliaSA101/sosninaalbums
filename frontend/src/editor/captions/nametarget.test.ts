import { describe, expect, it } from 'vitest'
import { captionBottom, hasNameFields, nameTargetAt, namePlace, takesNameTarget, whoOf, type PointCtx, type Who } from './nametarget'
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
    expect(t?.who).toEqual(['last', 'first'])
  })

  it('подпись по шаблону — цель; своя подпись и шаблон без полей имени — нет', () => {
    const s = sheet([cap('a', 10, 100, 60, 12, ['last']), cap('b', 100, 100, 60, 12), cap('c', 200, 100, 60, 12, [])])
    expect(nameTargetAt(s, 40, 105)?.photo).toBe('a')
    expect(nameTargetAt(s, 130, 105)).toBeNull()
    expect(nameTargetAt(s, 230, 105)).toBeNull()
  })

  it('заглушка пустого места при полях имени в листе — цель без фото и без полей (тост)', () => {
    const s = sheet([cap('a', 100, 100, 60, 12, ['last'])], [stub('p2', '', 10, 100, 60, 12)])
    const t = nameTargetAt(s, 40, 105)
    expect(t).not.toBeNull()
    expect(t?.photo).toBe('')
    expect(t?.who).toEqual([])
  })

  it('заглушка пустого места, когда полей имени в листе нет — не цель (остаётся «вписать»)', () => {
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

describe('hasNameFields', () => {
  it('есть who у подписи или заглушки', () => {
    expect(hasNameFields(sheet([cap('a', 0, 0, 1, 1, ['last'])]))).toBe(true)
    expect(hasNameFields(sheet([], [stub('p', 'b', 0, 0, 1, 1, ['first'])]))).toBe(true)
    expect(hasNameFields(sheet([cap('a', 0, 0, 1, 1), cap('b', 0, 0, 1, 1, [])], [stub('p', '', 0, 0, 1, 1)]))).toBe(false)
  })
})

describe('пустая виньетка', () => {
  const vig = (text: string, ttext = ''): SpreadView =>
    ({
      id: 'v',
      kind: 'vignette',
      caps: { text },
      vig: { ttext },
      captions: [],
      stubs: [stub('p2', '', 10, 100, 60, 12)],
    }) as unknown as SpreadView
  it('с шаблоном подписи пустое место — цель (тост), хотя who нигде нет', () => {
    expect(hasNameFields(vig('{Фамилия}'))).toBe(true)
    expect(nameTargetAt(vig('{Фамилия}'), 40, 105)?.photo).toBe('')
  })
  it('поле имени только в шаблоне учителя (ttext) — тоже', () => {
    expect(hasNameFields(vig('', '{имя} {Отчество}'))).toBe(true)
  })
  it('без шаблона подписи или без полей имени — как обычный лист', () => {
    expect(hasNameFields(vig(''))).toBe(false)
    expect(hasNameFields(vig('Класс 11А', '{Заголовок}'))).toBe(false)
    expect(nameTargetAt(vig(''), 40, 105)).toBeNull()
  })
})

describe('шаблон листа с полями имени (не только виньетка)', () => {
  const plain = (text: string): SpreadView =>
    ({ id: 'p', caps: { text }, captions: [], stubs: [stub('p2', '', 10, 100, 60, 12)] }) as unknown as SpreadView
  it('«Подпись к каждому фото» с полями имени — пустое место цель', () => {
    expect(hasNameFields(plain('{Фамилия} {Имя}'))).toBe(true)
    expect(nameTargetAt(plain('{Фамилия}'), 40, 105)?.photo).toBe('')
  })
  it('шаблон без полей имени — нет', () => {
    expect(hasNameFields(plain('Выпуск 2026'))).toBe(false)
    expect(hasNameFields(plain('{Год}'))).toBe(false)
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
