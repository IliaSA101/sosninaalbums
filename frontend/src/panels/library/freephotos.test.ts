import { describe, expect, it } from 'vitest'
import { allInAlbum, allInAlbumText, freePhotos, pickFor, type PickHost } from './freephotos'
import { filterFolders } from '../folders/folders'
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
    const shown = filterFolders(['D:/one', 'D:/two'], inA, new Set(['d:/one']))
    expect(pickFor(host(shown), true, true)).toMatchObject({ ids: ['c', 'd'], filtered: true })
  })

  it('все папки скрыты — пусто и тост про фильтр', () => {
    const toasts: string[] = []
    const shown = filterFolders(['D:/one', 'D:/two'], inA, new Set(['d:/one', 'd:/two']))
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

describe('allInAlbumText (054)', () => {
  it('без скрытых папок — прежний текст, со скрытыми — про показанные', () => {
    expect(allInAlbumText(false)).toBe('Все фото уже в альбоме')
    expect(allInAlbumText(true)).toBe('Все фото показанных папок уже в альбоме')
  })
})
