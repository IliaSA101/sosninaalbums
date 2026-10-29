import { describe, expect, it } from 'vitest'
import { COVER_ID } from '../../base/geom/sheet'
import { unitWords } from './unitword'

const album = { sheet: false }
const sheet = { sheet: true }
const all = (w: ReturnType<typeof unitWords>) => Object.values(w).join(' ')

describe('слова подписей по словарю 035', () => {
  it('в альбоме нет «лист»', () => {
    const w = unitWords(album, { id: 'a' })
    expect(all(w)).not.toMatch(/лист/)
    expect(w.acc).toBe('разворот')
    expect(w.gen).toBe('разворота')
    expect(w.on).toBe('на развороте')
    expect(w.onCap).toBe('На развороте')
  })
  it('в проекте-листе — «лист»', () => {
    const w = unitWords(sheet, { id: 'a' })
    expect(w.acc).toBe('лист')
    expect(w.gen).toBe('листа')
    expect(w.genThis).toBe('этого листа')
    expect(w.on).toBe('на листе')
    expect(w.onCap).toBe('На листе')
  })
  it('на обложке — «обложки», без «лист» и «разворот»', () => {
    for (const st of [album, sheet]) {
      const w = unitWords(st, { id: COVER_ID })
      expect(w.gen).toBe('обложки')
      expect(w.acc).toBe('обложку')
      expect(w.genThis).toBe('этой обложки')
      expect(all(w)).not.toMatch(/лист|разворот/)
    }
  })
  it('без разворота и настроек — альбом', () => {
    expect(unitWords(undefined).gen).toBe('разворота')
  })
})
