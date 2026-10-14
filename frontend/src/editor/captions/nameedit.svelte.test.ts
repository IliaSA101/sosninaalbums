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
