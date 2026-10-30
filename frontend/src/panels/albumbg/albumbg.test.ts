import { describe, expect, it } from 'vitest'
import type { AlbumBg, BackdropView, PatternView, SpreadView } from '../../base/core/types'
import { ownBg } from '../../base/geom/sheet'
import { backdropLost } from '../../base/core/undo'
import { backdropOf, zoneScopes } from '../../editor/backdrop/backdrop'
import { backdropAt, keepMode } from '../../editor/backdrop/bgmode'
import { patternOf } from '../../editor/pattern/pattern'
import { missingIn } from '../library/missing'
import { bgPickItems, bgTargets } from '../decor/bgtargets'
import type { DecorItem } from '../decor/decor'
import { unitOf } from '../../base/geom/sheet'
import { albumCard, albumItem, albumName, albumStanding, albumWhere, ALBUM_NAME_FALLBACK } from './albumbg'

const rect = { x: 0, y: 0, w: 100, h: 100 }
const bd = (o: Partial<BackdropView> = {}) =>
  ({ scope: 'spread', photo: 'p1', area: rect, box: { ...rect, rot: 0 }, img: rect, ...o }) as unknown as BackdropView
const pat = (o: Partial<PatternView> = {}) => ({ scope: 'spread', area: rect, known: true, url: '/g', ...o }) as unknown as PatternView
const sheet = (o: Partial<SpreadView> = {}) => ({ id: 's1', photos: [], backdrops: null, ...o }) as unknown as SpreadView
const ab = (o: Partial<AlbumBg> = {}): AlbumBg => ({ name: 'Лён', fade: 0, pages: false, preset: 'sa:len', ...o })
const items = [
  { id: 'sa:len', name: 'Лён (плитка)' },
  { id: 'my:1', name: 'Мой фон' },
] as DecorItem[]

describe('карточка листа (098 §3 п. 3)', () => {
  it('фон альбома рисуется, своего нет — «Изменить… · Не показывать здесь»', () => {
    expect(albumCard(sheet({ backdrops: [bd({ album: true })] }), ab())).toBe('on')
    expect(albumCard(sheet({ patterns: [pat({ album: true })] }), ab())).toBe('on')
  })
  it('скрытый — «Фон альбома скрыт · Показать»', () => {
    expect(albumCard(sheet({ noAlbumBg: true }), ab())).toBe('off')
  })
  it('своё фото фона или узор на листе — фон альбома не виден, состояние «own» (115 Г2)', () => {
    expect(albumCard(sheet({ backdrops: [bd(), bd({ album: true, scope: 'left' })] }), ab())).toBe('own')
    expect(albumCard(sheet({ patterns: [pat()] }), ab())).toBe('own')
  })
  it('own: на обложке и без фона альбома карточки нет', () => {
    expect(albumCard(sheet({ id: 'cover', patterns: [pat()] }), ab())).toBeNull()
    expect(albumCard(sheet({ patterns: [pat()] }), undefined)).toBeNull()
    expect(albumCard(sheet({ backdrops: [bd()] }), undefined)).toBeNull()
  })
  it('фона альбома нет, обложка, нет листа — карточки нет', () => {
    expect(albumCard(sheet({ backdrops: [bd({ album: true })] }), undefined)).toBeNull()
    expect(albumCard(sheet({ id: 'cover', backdrops: [bd({ album: true })] }), ab())).toBeNull()
    expect(albumCard(undefined, ab())).toBeNull()
  })
  it('скрыт, а потом поставили свой фон — не «off» (ревью 0.23 п. 5: «Показать» ничего бы не меняло), а «own»', () => {
    expect(albumCard(sheet({ noAlbumBg: true, backdrops: [bd()] }), ab())).toBe('own')
  })
  it('скрыт и свой убрали — снова «скрыт» (флаг листа не сбрасывается)', () => {
    expect(albumCard(sheet({ noAlbumBg: true }), ab())).toBe('off')
  })
})

describe('«Куда» для фона альбома', () => {
  it('разворот: весь разворот и каждая страница; лист виньетки — один пункт', () => {
    expect(albumWhere(false, 'Весь разворот').map((i) => [i.id, i.label])).toEqual([
      ['spread', 'Весь разворот'],
      ['pages', 'Каждая страница'],
    ])
    expect(albumWhere(true, 'Весь лист')).toHaveLength(1)
  })
})

describe('плитка, имя и отметка в окне «Фоны»', () => {
  it('узор — по плитке пресета, своё фото — по ключу библиотеки', () => {
    expect(albumItem(ab(), items)?.id).toBe('sa:len')
    expect(albumItem(ab({ preset: undefined, lib: 'my:1' }), items)?.id).toBe('my:1')
    expect(albumStanding(ab({ preset: undefined, lib: 'my:1' }), items)).toBe('my:1')
  })
  it('плитку скрыли из каталога — отметки нет, имя от Go, а без него общее слово', () => {
    const gone = ab({ preset: 'sa:gone', name: 'Старый узор' })
    expect(albumStanding(gone, items)).toBe('')
    expect(albumName(gone, items)).toBe('Старый узор')
    expect(albumName(ab({ preset: undefined, name: '' }), items)).toBe(ALBUM_NAME_FALLBACK)
    expect(albumName(ab(), items)).toBe('Лён (плитка)')
  })
  it('нет фона альбома — отметки нет', () => expect(albumStanding(undefined, items)).toBe(''))
})

describe('слой фона альбома не выделяется и своим фоном не считается', () => {
  const s = sheet({ backdrops: [bd({ album: true })], patterns: [pat({ album: true })] })
  it('ownBg отбрасывает album и терпит null', () => {
    expect(ownBg([bd(), bd({ album: true })])).toHaveLength(1)
    expect(ownBg(null)).toEqual([])
  })
  it('backdropOf, patternOf, zoneScopes', () => {
    expect(backdropOf(s, 'spread')).toBeUndefined()
    expect(patternOf(s, 'spread')).toBeUndefined()
    expect(zoneScopes([bd({ album: true, scope: 'left' })], false)).toEqual(['spread'])
  })
  it('щелчок по слою альбома не попадает в фон (backdropAt) и не держит режим фона (keepMode)', () => {
    expect(backdropAt(s.backdrops, { x: 50, y: 50 })).toBeNull()
    expect(backdropAt([bd()], { x: 50, y: 50 })).toBe('spread')
    expect(keepMode({ spread: 's1', scope: 'spread' }, s)).toBe(false)
  })
  it('склейка и «пропавшие фото» его не считают', () => {
    expect(backdropLost(s, sheet({ backdrops: [bd()] }))).toBe(false)
    expect(backdropLost(sheet({ backdrops: [bd()] }), sheet({ backdrops: [bd()] }))).toBe(true)
    expect(missingIn({ ...s, photos: [] }, new Set(['p1']))).toBe(0)
    expect(missingIn(sheet({ backdrops: [bd()] }), new Set(['p1']))).toBe(1)
  })
})

describe('«Весь альбом» в строке «На» (0.23-q5)', () => {
  const unit = unitOf({ sheet: false })
  it('стоит после «Все развороты», до «Выделенные»', () => {
    expect(bgPickItems(3, 8, unit).map((i) => i.id)).toEqual(['this', 'all', 'album', 'sel', 'nums'])
    expect(bgPickItems(0, 8, unit).map((i) => i.label)).toContain('Весь альбом')
  })
  it('целей-листов нет', () => {
    expect(bgTargets({ pick: 'album', cur: 's1', scope: 'spread', sel: [], nums: [], all: ['s1', 's2'] })).toEqual([])
  })
})
