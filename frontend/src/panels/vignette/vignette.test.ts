import { describe, expect, it } from 'vitest'
import type { VigChoice, VignetteView } from '../../base/core/types'
import { can, type FeatureId } from '../../base/prefs/levels'
import {
  canMakeVignette,
  clampCount,
  formOf,
  groupCount,
  isVignette,
  leftText,
  paramsOf,
  portraitLines,
  portraitText,
  sameAsSheet,
  startForm,
  stepTpl,
  toneOf,
  totalPlaces,
} from './vignette'

const vig = (o: Partial<VignetteView> = {}): VignetteView => ({
  tpl: 'head-t-left',
  teachers: 1,
  pupils: 24,
  ratio: '3:4',
  places: 25,
  max: 64,
  group: false,
  ...o,
})

const choice = (o: Partial<VigChoice> = {}): VigChoice => ({
  id: 'head-t-left',
  name: 'Шапка, учителя слева',
  boxes: [],
  roles: ['teacher', 'pupil'],
  pupilW: 35.2,
  small: false,
  over: false,
  ...o,
})

describe('totalPlaces', () => {
  it('групповое фото — тоже место', () => {
    expect(totalPlaces({ teachers: 1, pupils: 24 }, 1)).toBe(26)
  })
  it('несколько групповых в своём шаблоне — каждое место', () => {
    expect(totalPlaces({ teachers: 1, pupils: 24 }, 2)).toBe(27)
    expect(totalPlaces({ teachers: 1, pupils: 24 }, 0)).toBe(25)
  })
})

describe('clampCount', () => {
  it('пусто, мусор и минус — 0; дробное вниз; сверху предел', () => {
    expect(clampCount('', 18)).toBe(0)
    expect(clampCount('abc', 18)).toBe(0)
    expect(clampCount(-3, 18)).toBe(0)
    expect(clampCount('7.9', 18)).toBe(7)
    expect(clampCount(100, 18)).toBe(18)
    expect(clampCount(undefined, 64)).toBe(0)
  })
})

describe('форма и параметры', () => {
  it('formOf берёт числа листа; нет пропорции или чужая — 3:4', () => {
    expect(formOf(vig({ ratio: undefined })).ratio).toBe('3:4')
    expect(formOf(vig({ ratio: '16:9' })).ratio).toBe('3:4')
    expect(formOf(vig({ ratio: '1:1', ttext: 'Т' }))).toMatchObject({ ratio: '1:1', ttext: 'Т', teachers: 1, pupils: 24 })
  })
  it('paramsOf уходит в Go без шапки (её знает лист)', () => {
    expect(paramsOf(formOf(vig()))).toEqual({ tpl: 'head-t-left', teachers: 1, pupils: 24, ratio: '3:4', ttext: '', head: '' })
  })
  it('sameAsSheet: изменённая форма отличается', () => {
    const f = formOf(vig())
    expect(sameAsSheet(f, vig())).toBe(true)
    expect(sameAsSheet({ ...f, pupils: 20 }, vig())).toBe(false)
    expect(sameAsSheet({ ...f, ratio: '2:3' }, vig())).toBe(false)
  })
})

describe('startForm: «Сделать виньеткой»', () => {
  it('пустой лист — 24 ученика, как у новой виньетки', () => {
    expect(startForm(0)).toMatchObject({ tpl: 'head-t-left', teachers: 1, pupils: 24, ratio: '3:4' })
  })
  it('учеников столько, сколько фото, но с учителем не больше предела', () => {
    expect(startForm(10).pupils).toBe(10)
    expect(startForm(64).pupils).toBe(63)
    expect(startForm(200).pupils).toBe(63)
  })
})

describe('groupCount', () => {
  it('по карточке Go; пока её нет — по виду листа', () => {
    expect(groupCount('g-top', [choice({ id: 'g-top', roles: ['group', 'pupil'] })])).toBe(1)
    expect(groupCount('my:x', [choice({ id: 'my:x', roles: ['group', 'pupil', 'group'] })])).toBe(2)
    expect(groupCount('head-t-left', [choice()])).toBe(0)
    expect(groupCount('g-top', [], vig({ tpl: 'g-top', group: true }))).toBe(1)
    expect(groupCount('other', [], vig({ tpl: 'g-top', group: true }))).toBe(0)
  })
})

describe('подписи', () => {
  it('портрет в мм; мелкий и «не поместится» помечены', () => {
    expect(portraitText(choice())).toBe('портрет 35,2 мм')
    expect(portraitText(choice({ pupilW: 12.34, small: true }))).toBe('портрет 12,3 мм · мелко')
    expect(portraitText(choice({ over: true }))).toBe('не поместится')
    expect(portraitText(choice({ pupilW: 0 }))).toBe('без учеников')
  })
  it('подпись плитки галереи — короткими строками без «портрет» (не налезают на соседние)', () => {
    expect(portraitLines(choice({ pupilW: 34.5 }))).toEqual(['34,5 мм'])
    expect(portraitLines(choice({ pupilW: 12.34, small: true }))).toEqual(['12,3 мм', 'мелко'])
    expect(portraitLines(choice({ over: true }))).toEqual(['не влезет'])
    expect(portraitLines(choice({ pupilW: 0 }))).toEqual(['без учеников'])
  })
  it('тост «Убрано в ленту»', () => {
    expect(leftText(5)).toBe('Убрано в ленту: 5')
  })
  it('тон роли: пустая и чужая — ученик', () => {
    expect(toneOf('teacher')).toBe('teacher')
    expect(toneOf('group')).toBe('group')
    expect(toneOf('')).toBe('pupil')
    expect(toneOf(undefined)).toBe('pupil')
    expect(toneOf('x')).toBe('pupil')
  })
})

describe('stepTpl (Пробел и стрелки на виньетке)', () => {
  const list = [
    { id: 'a', over: false },
    { id: 'b', over: false },
    { id: 'c', over: true },
    { id: 'd', over: false },
  ]
  it('вперёд и назад по порядку, по кругу', () => {
    expect(stepTpl(list, 'a', 1)).toBe('b')
    expect(stepTpl(list, 'd', 1)).toBe('a')
    expect(stepTpl(list, 'a', -1)).toBe('d')
    expect(stepTpl(list, 'b', -1)).toBe('a')
  })
  it('«не поместится» пропускается', () => {
    expect(stepTpl(list, 'b', 1)).toBe('d')
    expect(stepTpl(list, 'd', -1)).toBe('b')
  })
  it('текущего среди карточек нет — с края, откуда идёт шаг', () => {
    expect(stepTpl(list, 'zzz', 1)).toBe('a')
    expect(stepTpl(list, 'zzz', -1)).toBe('d')
  })
  it('менять нечего: пусто или подходит только текущий', () => {
    expect(stepTpl([], 'a', 1)).toBeNull()
    expect(stepTpl([{ id: 'a', over: false }], 'a', 1)).toBeNull()
    expect(
      stepTpl(
        [
          { id: 'a', over: false },
          { id: 'b', over: true },
        ],
        'a',
        1,
      ),
    ).toBeNull()
  })
})

describe('isVignette', () => {
  it('нужны и kind, и параметры', () => {
    expect(isVignette({ kind: 'vignette', vig: vig() })).toBe(true)
    expect(isVignette({ kind: 'vignette' })).toBe(false)
    expect(isVignette({ kind: '', vig: vig() })).toBe(false)
    expect(isVignette(undefined)).toBe(false)
  })
})

describe('canMakeVignette (отзыв 14.10)', () => {
  const at = (l: 'simple' | 'standard' | 'full') => (f: FeatureId) => can(f, l)
  const plain = { id: 'a' }
  it('обычный разворот — со «Стандартного»', () => {
    expect(canMakeVignette(plain, at('simple'))).toBe(false)
    expect(canMakeVignette(plain, at('standard'))).toBe(true)
    expect(canMakeVignette(plain, at('full'))).toBe(true)
  })
  it('обложка, готовая виньетка и пустота — нет', () => {
    expect(canMakeVignette({ id: 'cover' }, at('full'))).toBe(false)
    expect(canMakeVignette({ id: 'a', kind: 'vignette', vig: vig() }, at('full'))).toBe(false)
    expect(canMakeVignette(undefined, at('full'))).toBe(false)
  })
})
