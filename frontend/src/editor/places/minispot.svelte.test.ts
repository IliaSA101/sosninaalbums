import { describe, expect, it } from 'vitest'
import { miniSpot } from './minispot.svelte'
import { NO_ROOM } from './minipanel'

// Контракт U1/U2 (063, design §3.4): MiniPanel пишет поля, CellOverlay читает.
describe('miniSpot', () => {
  it('по умолчанию: фото нет, места дуге нет, подъёма нет', () => {
    expect(miniSpot.photo).toBe('')
    expect(miniSpot.where).toBe('below')
    expect(miniSpot.lift).toBe(0)
    expect(miniSpot.arc).toBe(NO_ROOM)
    expect(NO_ROOM.mode).toBe('none')
  })
  it('поля пишутся и читаются', () => {
    miniSpot.photo = 'a.jpg'
    miniSpot.where = 'inside'
    miniSpot.lift = 52
    miniSpot.arc = { mode: 'in', vx: 100, vy: 0, vw: 200 }
    expect(miniSpot.arc.mode).toBe('in')
    expect([miniSpot.photo, miniSpot.where, miniSpot.lift]).toEqual(['a.jpg', 'inside', 52])
  })
})
