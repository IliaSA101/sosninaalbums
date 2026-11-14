import { beforeEach, describe, expect, it } from 'vitest'
import { nameEdit } from './nameedit.svelte'
import type { SpreadView } from '../../base/core/types'

const sheet = (id: string): SpreadView =>
  ({
    id,
    captions: [{ photo: 'a', x: 1, y: 2, w: 10, h: 5, rot: 0, z: 1, who: ['last'] }],
    stubs: [],
  }) as unknown as SpreadView

describe('nameEdit', () => {
  beforeEach(() => nameEdit.close())

  it('openFor открывает окошко подписи с полями имени', () => {
    nameEdit.openFor(sheet('s1'), 'a')
    expect(nameEdit.cur?.spread).toBe('s1')
  })

  it('release закрывает только окошко своего разворота (NameEditor размонтируется)', () => {
    nameEdit.openFor(sheet('s1'), 'a')
    nameEdit.release('s2')
    expect(nameEdit.cur).not.toBeNull()
    nameEdit.release('s1')
    expect(nameEdit.cur).toBeNull()
  })

  it('новый вид с тем же id (store.apply) окошко не закрывает; sync другого разворота — закрывает', () => {
    nameEdit.openFor(sheet('s1'), 'a')
    nameEdit.sync('s1')
    nameEdit.sync('s1')
    expect(nameEdit.cur).not.toBeNull()
    expect(nameEdit.verdict(sheet('s1'), 's1')).toBe('keep')
    nameEdit.sync('s2')
    expect(nameEdit.cur).toBeNull()
  })
})

describe('nameEdit.verdict', () => {
  const stubSheet = (id: string, who?: string[]): SpreadView =>
    ({ id, captions: [], stubs: [{ photo: 'a', place: 'p', x: 0, y: 0, w: 10, h: 5, rot: 0, z: 1, who }] }) as unknown as SpreadView
  const capSheet = (id: string, who?: string[]): SpreadView =>
    ({ id, captions: [{ photo: 'a', x: 0, y: 0, w: 10, h: 5, rot: 0, z: 1, who }], stubs: [] }) as unknown as SpreadView

  beforeEach(() => nameEdit.close())

  it('заглушка → подпись (после SetPerson) — одна цель, окошко живо', () => {
    nameEdit.openFor(stubSheet('s1', ['last', 'first']), 'a')
    expect(nameEdit.verdict(stubSheet('s1', ['last', 'first']), 's1')).toBe('keep')
    expect(nameEdit.verdict(capSheet('s1', ['last', 'first']), 's1')).toBe('keep')
  })

  it('вид без полей имени — закрыть сразу; вернулись поля — keep', () => {
    nameEdit.openFor(stubSheet('s1', ['last']), 'a')
    expect(nameEdit.verdict(capSheet('s1'), 's1')).toBe('close')
    expect(nameEdit.verdict({ id: 's1', captions: [], stubs: [] } as unknown as SpreadView, 's1')).toBe('close')
    expect(nameEdit.verdict(capSheet('s1', ['last']), 's1')).toBe('keep')
  })

  it('другой разворот — закрыть сразу; закрытое окошко — keep', () => {
    nameEdit.openFor(stubSheet('s1', ['last']), 'a')
    expect(nameEdit.verdict(stubSheet('s1', ['last']), 's2')).toBe('close')
    nameEdit.close()
    expect(nameEdit.verdict(stubSheet('s1'), 's1')).toBe('keep')
  })
})

describe('nameEdit: пустое место (071)', () => {
  const style = { font: 'f', size: 12, color: '#000', align: 'center' as const }
  const stub = (place: string, photo: string, who?: string[]) => ({
    place,
    photo,
    x: 10,
    y: 20,
    w: 60,
    h: 12,
    rot: 0,
    z: 1,
    style,
    lines: [],
    who,
  })
  const placeSheet = (id: string, stubs = [stub('p9', '', ['last', 'first'])]): SpreadView =>
    ({ id, w: 200, h: 100, captions: [], stubs }) as unknown as SpreadView
  const click = (x: number, y: number) => ({ clientX: x, clientY: y }) as MouseEvent
  // Разворот 200×100 мм на экране 200×100 px от (0, 0): px = мм.
  const root = { getBoundingClientRect: () => ({ left: 0, top: 0, width: 200, height: 100 }) } as HTMLElement

  beforeEach(() => nameEdit.close())

  it('двойной щелчок по заглушке пустого места открывает окошко места, тоста нет', () => {
    expect(nameEdit.dbl(placeSheet('s1'), click(30, 25), root)).toBe(true)
    expect(nameEdit.cur).toMatchObject({ spread: 's1', photo: '', place: 'p9', who: ['last', 'first'] })
  })

  it('мимо заглушки и заглушка без who — false, окошко не открыто', () => {
    expect(nameEdit.dbl(placeSheet('s1'), click(150, 80), root)).toBe(false)
    expect(nameEdit.dbl(placeSheet('s1', [stub('p9', '')]), click(30, 25), root)).toBe(false)
    expect(nameEdit.cur).toBeNull()
  })

  it('verdict держит окошко места, пока у места есть заглушка с who (в том числе с табличкой)', () => {
    nameEdit.dbl(placeSheet('s1'), click(30, 25), root)
    expect(nameEdit.verdict(placeSheet('s1'), 's1')).toBe('keep')
    expect(nameEdit.verdict(placeSheet('s1', [stub('p9', '', ['last'])]), 's1')).toBe('keep')
  })

  it('verdict закрывает окошко места, когда на нём встало фото или заглушки нет; другой разворот — тоже', () => {
    nameEdit.dbl(placeSheet('s1'), click(30, 25), root)
    expect(nameEdit.verdict(placeSheet('s1', [stub('p9', 'a', ['last'])]), 's1')).toBe('close')
    expect(nameEdit.verdict(placeSheet('s1', []), 's1')).toBe('close')
    expect(nameEdit.verdict(placeSheet('s1'), 's2')).toBe('close')
  })

  it('двойной щелчок по подписи фото — окошко фото (place пуст)', () => {
    const s = {
      id: 's1',
      w: 200,
      h: 100,
      captions: [{ photo: 'a', x: 10, y: 20, w: 60, h: 12, rot: 0, z: 1, who: ['last'] }],
      stubs: [],
    } as unknown as SpreadView
    expect(nameEdit.dbl(s, click(30, 25), root)).toBe(true)
    expect(nameEdit.cur).toMatchObject({ photo: 'a', place: '' })
  })
})
