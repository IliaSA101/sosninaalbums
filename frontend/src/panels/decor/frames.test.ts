import { describe, expect, it } from 'vitest'
import type { BorderView, PlaceView } from '../../base/core/types'
import { FRAME_ALL, frameStanding, framePlaceIds, frameTargets, frameWhereDefault, frameWhereIn, frameWhereItems } from './frames'
import { deleteText, emptyText, NOUN, DECOR_TITLE, uploadHint, type DecorItem } from './decor'

const bv = (over: Partial<BorderView> = {}): BorderView => ({
  kind: 'line',
  photo: '',
  lib: '',
  preset: '',
  name: 'Линия',
  inks: [],
  width: 1.5,
  scale: 0,
  ins: [1.5, 1.5, 1.5, 1.5],
  url: '/media/gen/frame?k=line',
  maskUrl: '',
  pic: false,
  open: false,
  known: true,
  ...over,
})
const place = (id: string, photo = '', border?: BorderView): PlaceView =>
  ({ id, photo, fit: '', spine: false, z: 0, x: 0, y: 0, w: 10, h: 10, rot: 0, border }) as PlaceView
const tile = (id: string): DecorItem => ({ id, theme: 'basic', name: id, url: '/x', own: id.startsWith('my:'), aspect: 1, w: 0, h: 0 })

describe('«Куда» окна «Рамочки»', () => {
  it('с выделением — «Выделенные (N)» и «Все места разворота», без — только второй', () => {
    expect(frameWhereItems(3)).toEqual([
      { id: 'sel', label: 'Выделенные (3)' },
      { id: 'all', label: FRAME_ALL },
    ])
    expect(frameWhereItems(0)).toEqual([{ id: 'all', label: 'Все места разворота' }])
  })

  it('по умолчанию — выделенные, если они есть; пропавшее выделение возвращает на весь лист', () => {
    expect(frameWhereDefault(2)).toBe('sel')
    expect(frameWhereDefault(0)).toBe('all')
    expect(frameWhereIn('sel', 0)).toBe('all')
    expect(frameWhereIn('sel', 1)).toBe('sel')
    expect(frameWhereIn('all', 4)).toBe('all')
  })

  it('все места: у сетки — места вместе с пустыми, у «Авто» — фото, нет листа — ничего', () => {
    expect(framePlaceIds({ places: [place('p1', 'a'), place('p2')], photos: ['a'] })).toEqual(['p1', 'p2'])
    expect(framePlaceIds({ places: null, photos: ['a', 'b'] })).toEqual(['a', 'b'])
    expect(framePlaceIds({ places: [], photos: [] })).toEqual([])
    expect(framePlaceIds(undefined)).toEqual([])
  })

  it('цели: выделенные как есть, все — места листа', () => {
    const sp = { places: [place('p1'), place('p2')], photos: [] }
    expect(frameTargets(sp, 'sel', ['p2'])).toEqual(['p2'])
    expect(frameTargets(sp, 'all', ['p2'])).toEqual(['p1', 'p2'])
  })
})

describe('какая плитка стоит сейчас', () => {
  const items = [tile('sa:fr-line'), tile('my:0123456789abcdef')]
  const sp = {
    places: [
      place('p1', 'a', bv({ preset: 'sa:fr-line' })),
      place('p2', 'b', bv({ preset: 'sa:fr-line' })),
      place('p3', 'c', bv({ preset: '' })),
      place('p4', 'd', bv({ lib: 'my:0123456789abcdef', pic: true, kind: '' })),
      place('p5'),
    ],
  }

  it('одна и та же на всех целях — отмечена; ID места или фото', () => {
    expect(frameStanding(sp, ['p1', 'p2'], items)).toBe('sa:fr-line')
    expect(frameStanding(sp, ['a', 'p2'], items)).toBe('sa:fr-line')
  })
  it('своя — по ключу библиотеки', () => {
    expect(frameStanding(sp, ['p4'], items)).toBe('my:0123456789abcdef')
  })
  it('разные, правленные руками, без рамочки, пустой список, «Авто» — не отмечена', () => {
    expect(frameStanding(sp, ['p1', 'p4'], items)).toBe('')
    expect(frameStanding(sp, ['p3'], items)).toBe('')
    expect(frameStanding(sp, ['p1', 'p5'], items)).toBe('')
    expect(frameStanding(sp, [], items)).toBe('')
    expect(frameStanding({ places: null }, ['a'], items)).toBe('')
    expect(frameStanding(undefined, ['a'], items)).toBe('')
  })
  it('плитку скрыли из каталога — не отмечена', () => {
    expect(frameStanding(sp, ['p1'], [tile('sa:fr-other')])).toBe('')
  })
})

describe('слова окна «Рамочки»', () => {
  it('заголовок, имя предмета, пустые «Мои», загрузка, удаление', () => {
    expect(DECOR_TITLE.frame).toBe('Рамочки')
    expect(NOUN.frame).toBe('рамочку')
    expect(emptyText('mine', false, 'frame')).toBe('Загрузите свои кнопкой выше')
    expect(emptyText('mine', true, 'frame')).toBe('Здесь появятся ваши рамочки')
    expect(uploadHint('frame')).toContain('PNG с прозрачностью и SVG')
    expect(deleteText('Резная', 'frame')).toBe('«Резная» будет убрана из библиотеки. Где она уже стоит, не изменится.')
  })
})
