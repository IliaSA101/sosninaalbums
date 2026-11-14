import { describe, expect, it } from 'vitest'
import { mergeSnapshot, noEvents } from './startstate'
import type { Photo, ScanProgress } from './types'

const ph = (id: string) => ({ id }) as Photo
const scan = (active: boolean): ScanProgress => ({ active, walking: false, found: 0, read: 0 })

const cur = { photos: [ph('a'), ph('b')], scan: scan(false), missing: ['x'] }
const snap = { photos: [], scan: scan(true), missing: [] }

describe('mergeSnapshot', () => {
  it('без событий берёт снимок', () => {
    const r = mergeSnapshot(cur, snap, noEvents())
    expect(r.photos).toEqual([])
    expect(r.scan.active).toBe(true)
    expect(r.missing).toEqual([])
  })

  it('событие сильнее снимка по каждому полю отдельно', () => {
    const r = mergeSnapshot(cur, snap, { photos: true, scan: false, missing: true })
    expect(r.photos).toBe(cur.photos)
    expect(r.scan.active).toBe(true)
    expect(r.missing).toBe(cur.missing)
  })

  it('все события были: снимок игнорируется', () => {
    expect(mergeSnapshot(cur, snap, { photos: true, scan: true, missing: true })).toEqual(cur)
  })

  it('поля, которых нет в снимке, не трогает', () => {
    expect(mergeSnapshot(cur, { photos: [ph('z')] }, noEvents())).toEqual({ ...cur, photos: [ph('z')] })
    expect(mergeSnapshot(cur, undefined, noEvents())).toEqual(cur)
  })
})
