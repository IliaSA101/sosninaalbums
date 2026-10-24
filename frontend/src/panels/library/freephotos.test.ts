import { describe, expect, it } from 'vitest'
import { allInAlbum, allInAlbumText, emptyReason, freePhotos, pickFor, type PickHost } from './freephotos'
import { filterTree } from '../folders/tree'
import type { Photo } from '../../base/core/types'

const ph = (id: string): Photo => ({ id, path: id, name: id, w: 100, h: 100 })
const photos = ['a', 'b', 'c', 'd'].map(ph)
const used = new Set(['a'])
const bg = new Set(['b'])

describe('freePhotos', () => {
  it('не в местах и не на фоне', () => {
    expect(freePhotos(photos, used, bg).map((p) => p.id)).toEqual(['c', 'd'])
  })
  it('без фонов — только вычет used', () => {
    expect(freePhotos(photos, used, new Set()).map((p) => p.id)).toEqual(['b', 'c', 'd'])
  })
})

describe('pickFor (034): откуда фото для окна автосборки', () => {
  const host = (selectedIds: string[]): PickHost => ({
    visible: photos,
    selectedIds,
    used,
    backdropUsed: bg,
    marks: {},
    notify: () => {},
  })
  it('выделение: сколько уже в альбоме', () => {
    expect(pickFor(host(['a', 'c']), true, false)).toMatchObject({ ids: ['a', 'c'], from: 'selected', inAlbum: 1, filtered: false })
  })
  it('без выделения: лента и фильтр', () => {
    expect(pickFor(host([]), true, true)).toMatchObject({ ids: ['c', 'd'], from: 'free', inAlbum: 0, filtered: true })
  })
})

describe('folder filter (054)', () => {
  const inA = ['a', 'b', 'c', 'd'].map((id) => ({ ...ph(id), path: `D:/${id < 'c' ? 'one' : 'two'}/${id}.jpg` }))
  const host = (visible: Photo[], notify: (t: string) => void = () => {}): PickHost => ({
    visible,
    selectedIds: [],
    used: new Set(),
    backdropUsed: new Set(),
    marks: {},
    notify,
  })

  it('автосборка берёт только показанные, filtered: true', () => {
    const shown = filterTree(inA, new Set(['d:d:/one']))
    expect(pickFor(host(shown), true, true)).toMatchObject({ ids: ['c', 'd'], filtered: true })
  })

  it('все папки скрыты — пусто и тост про фильтр', () => {
    const toasts: string[] = []
    const shown = filterTree(inA, new Set(['d:d:/one', 'd:d:/two']))
    expect(
      pickFor(
        host(shown, (t) => toasts.push(t)),
        true,
        true,
      ).ids,
    ).toEqual([])
    expect(toasts).toEqual(['Под фильтр не подходит ни одно свободное фото'])
  })
})

describe('allInAlbum (035 U11)', () => {
  it('«Не в альбоме» без свободных фото при непустой папке', () => {
    expect(allInAlbum('unused', 0, 12)).toBe(true)
  })
  it('есть свободные, фильтр «Все» или фото нет вовсе — нет', () => {
    expect(allInAlbum('unused', 3, 12)).toBe(false)
    expect(allInAlbum('all', 0, 12)).toBe(false)
    expect(allInAlbum('unused', 0, 0)).toBe(false)
  })
})

describe('подборка (083)', () => {
  const host = (visible: Photo[], inBin: Photo[] | undefined, notify: (t: string) => void = () => {}): PickHost => ({
    visible,
    inBin,
    selectedIds: [],
    used: new Set(['a', 'b', 'c', 'd']),
    backdropUsed: new Set(),
    marks: {},
    notify,
  })
  it('без выделения: имя подборки в результате', () => {
    const h = { ...host(photos, photos), used: new Set<string>() }
    expect(pickFor(h, true, false, 'Запас')).toMatchObject({ from: 'free', bin: 'Запас' })
  })
  it('выделение главнее: подборка не называется', () => {
    const h = { ...host(photos, photos), selectedIds: ['a'], used: new Set<string>() }
    expect(pickFor(h, true, false, 'Запас').bin).toBeUndefined()
  })
  it('все фото подборки в альбоме — тост с её именем', () => {
    const t: string[] = []
    expect(
      pickFor(
        host(photos, photos, (x) => t.push(x)),
        true,
        false,
        'Запас',
      ).ids,
    ).toEqual([])
    expect(t).toEqual(['Все фото подборки «Запас» уже в альбоме'])
  })
  it('в подборке нет фото — свой тост', () => {
    const t: string[] = []
    pickFor(
      host([], [], (x) => t.push(x)),
      true,
      false,
      'Запас',
    )
    expect(t).toEqual(['В подборке «Запас» нет фото'])
  })
  it('подборка и фильтр отметок: причина — фильтр', () => {
    expect(emptyReason(false, 0, true, 'Запас')).toBe('Под фильтр не подходит ни одно свободное фото')
  })
  it('без подборки причины прежние', () => {
    expect(emptyReason(false, 0, false)).toBe('Все фото уже в альбоме')
    expect(emptyReason(true, 0, false, 'Запас')).toBe('Выделенные фото уже в альбоме')
  })
  it('пустая лента «Не в альбоме»: текст про подборку главнее папок', () => {
    expect(allInAlbumText(true, 'Запас')).toBe('Все фото подборки «Запас» уже в альбоме')
  })
})

describe('allInAlbumText (054)', () => {
  it('без скрытых папок — прежний текст, со скрытыми — про показанные', () => {
    expect(allInAlbumText(false)).toBe('Все фото уже в альбоме')
    expect(allInAlbumText(true)).toBe('Все фото показанных папок уже в альбоме')
  })
})
