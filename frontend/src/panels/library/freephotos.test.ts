import { describe, expect, it } from 'vitest'
import {
  allInAlbum,
  allInAlbumText,
  emptyReason,
  fillBlock,
  fillCount,
  fillDigits,
  fillLead,
  fillTitle,
  fillWhere,
  freePhotos,
  pickFor,
  type PickHost,
} from './freephotos'
import { unitOf } from '../../base/geom/sheet'
import type { View } from '../../base/core/types'
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

describe('поиск по имени как фильтр ленты (0.26-124)', () => {
  // store.visible после поиска — найденные; дальше те же правила, что у фильтра папок и отметок.
  const found = ['IMG_0801', 'IMG_0805', 'IMG_0810'].map(ph)
  const host = (visible: Photo[], used = new Set<string>(), notify: (t: string) => void = () => {}): PickHost => ({
    visible,
    selectedIds: [],
    used,
    backdropUsed: new Set(),
    marks: {},
    notify,
  })

  it('автосборка без выделения берёт найденные свободные, filtered: true', () => {
    expect(pickFor(host(found, new Set(['IMG_0805'])), true, true)).toMatchObject({
      ids: ['IMG_0801', 'IMG_0810'],
      from: 'free',
      filtered: true,
    })
  })

  it('отклонённые среди найденных пропускаются', () => {
    const h = { ...host(found), marks: { IMG_0805: { flag: -1 as const } } }
    expect(pickFor(h, true, true)).toMatchObject({ ids: ['IMG_0801', 'IMG_0810'], filtered: true })
  })

  it('нашлись только стоящие в альбоме — пусто и тост про фильтр', () => {
    const toasts: string[] = []
    const r = pickFor(
      host(found, new Set(found.map((p) => p.id)), (t) => toasts.push(t)),
      true,
      true,
    )
    expect(r.ids).toEqual([])
    expect(toasts).toEqual(['Под фильтр не подходит ни одно свободное фото'])
  })
})

describe('132: «Заполнить места» — числа и тексты «отсюда»', () => {
  const spread = unitOf(undefined)
  const sheet = unitOf({ sheet: true } as never)
  const sp = (free: number) => ({ places: Array.from({ length: free }, () => ({})) })
  const v = (o: object) => o as View

  it('fillCount: обложка — coverFill, разворот — fillFrom[i]; поля нет или короче — 0', () => {
    const view = { coverFill: 7, fillFrom: [4, 3, 1] }
    expect(fillCount(view, true, 2)).toBe(7)
    expect(fillCount(view, false, 0)).toBe(4)
    expect(fillCount(view, false, 2)).toBe(1)
    expect(fillCount(view, false, 3)).toBe(0)
    expect(fillCount({}, true, 0)).toBe(0)
    expect(fillCount({ fillFrom: [] }, false, 0)).toBe(0)
  })

  it('fillTitle: с обложки — порядок «лицевая, задняя, затем развороты», с разворота — прежний текст', () => {
    expect(fillTitle(true, 5, spread)).toBe(
      'Заполнить пустые места (5): положить выделенные фото (или все, что ещё не в альбоме) в пустые места, начиная с обложки: лицевая, задняя, затем развороты. Без выделения — первые свободные фото ленты, отклонённые пропускаются',
    )
    expect(fillTitle(false, 2, spread)).toBe(
      'Заполнить пустые места (2): положить выделенные фото (или все, что ещё не в альбоме) в пустые места разворотов-сеток, начиная с этого разворота. Без выделения отклонённые пропускаются',
    )
    expect(fillTitle(false, 2, sheet)).toContain('листов-сеток, начиная с этого листа')
  })

  it('fillWhere: обложка и разворот; несколько — «и», больше трёх — «и ещё N»; листы', () => {
    const base = { coverFill: 3, fillFrom: [1, 1, 1] }
    expect(fillWhere({ ...base, spreads: [sp(0), sp(0), sp(1)] } as never, spread)).toBe('Пустые места — на обложке, в развороте 3')
    expect(fillWhere({ coverFill: 0, fillFrom: [0, 0, 0], spreads: [sp(1), sp(0), sp(1)] } as never, spread)).toBe(
      'Пустые места — в разворотах 1 и 3',
    )
    expect(fillWhere({ coverFill: 0, spreads: [sp(1), sp(1), sp(1)] } as never, spread)).toBe('Пустые места — в разворотах 1, 2 и 3')
    expect(fillWhere({ coverFill: 0, spreads: [sp(1), sp(1), sp(1), sp(1), sp(1)] } as never, spread)).toBe(
      'Пустые места — в разворотах 1, 2, 3 и ещё 2',
    )
    expect(fillWhere({ coverFill: 1, fillFrom: [0], spreads: [sp(0)] } as never, sheet)).toBe('Пустые места — на обложке')
    expect(fillWhere({ coverFill: 0, spreads: [sp(0), sp(2)] } as never, sheet)).toBe('Пустые места — в листе 2')
  })

  it('fillWhere: части через запятую, «и» — только внутри перечня номеров', () => {
    expect(fillWhere({ coverFill: 3, fillFrom: [1], spreads: [sp(1), sp(0), sp(1)] } as never, spread)).toBe(
      'Пустые места — на обложке, в разворотах 1 и 3',
    )
    expect(fillWhere({ coverFill: 3, fillFrom: [1], spreads: [sp(1), sp(1), sp(1), sp(1), sp(1)] } as never, spread)).toBe(
      'Пустые места — на обложке, в разворотах 1, 2, 3 и ещё 2',
    )
  })

  it('fillDigits: разряды по пустым местам альбома, не меньше разрядов числа; нет поля — 1', () => {
    expect(fillDigits(12, 0)).toBe(2)
    expect(fillDigits(9, 3)).toBe(1)
    expect(fillDigits(100, 5)).toBe(3)
    expect(fillDigits(undefined, 0)).toBe(1)
    expect(fillDigits(3, 12)).toBe(2)
  })

  it('fillWhere: назвать нечем (печать: полей нет, мест у разворотов нет) — не молчит', () => {
    expect(fillWhere({ spreads: [] } as never, spread)).toBe('Пустые места — в другом месте альбома')
  })

  it('fillBlock: отсюда есть места — пусто; нет — текст «где»', () => {
    const view = v({ coverFill: 2, fillFrom: [1, 0], spreads: [sp(1), sp(0)], settings: {} })
    expect(fillBlock(view, false, 0)).toBe('')
    expect(fillBlock(view, false, 1)).toBe('Пустые места — на обложке, в развороте 1')
  })

  it('fillLead: разность видов; одно из двух или ничего — пусто', () => {
    const before = v({ coverFill: 7, fillFrom: [4, 2], settings: {} })
    expect(fillLead(before, v({ coverFill: 0, fillFrom: [0, 0], settings: {} }))).toBe('На обложку: 3, в развороты: 4')
    expect(fillLead(before, v({ coverFill: 4, fillFrom: [4, 2], settings: {} }))).toBe('') // только обложка
    expect(fillLead(before, v({ coverFill: 5, fillFrom: [0, 0], settings: {} }))).toBe('') // только развороты
    expect(fillLead(before, before)).toBe('')
    expect(fillLead(v({}), v({}))).toBe('') // полей нет
    expect(fillLead(before, v({ coverFill: 2, fillFrom: [2, 0], settings: { sheet: true } }))).toBe('На обложку: 3, в листы: 2')
  })
})
