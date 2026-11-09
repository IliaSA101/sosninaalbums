import { describe, expect, it } from 'vitest'
import type { FeatureId } from '../../base/prefs/levels'
import { fillKeys } from '../../base/keys/keytext'
import { fillLevels, levelMarks, nestedLevels } from './leveltext'

const on = new Set<string>(['adjust'])
const can = (id: FeatureId) => on.has(id)
const hint = (id: string) => ({ undo: 'Ctrl+Z' })[id] ?? ''

describe('fillLevels', () => {
  it('функция включена — кусок остаётся без метки, выключена — опускается целиком', () => {
    expect(fillLevels('Блок «Фото»{can:adjust| и «Ч/б развороты»}.', can)).toBe('Блок «Фото» и «Ч/б развороты».')
    expect(fillLevels('Блок «Фото»{can:variants| и «Семейства»}.', can)).toBe('Блок «Фото».')
  })
  it('несколько меток в одном тексте', () => {
    const t = 'A{can:adjust| раз}; B{can:variants| два}; C{can:adjust| три}'
    expect(fillLevels(t, can)).toBe('A раз; B; C три')
  })
  it('без метки текст как есть, чужие скобки не трогаются', () => {
    expect(fillLevels('Просто текст {не метка} и {key:undo}', can)).toBe('Просто текст {не метка} и {key:undo}')
  })
  it('{key} внутри куска: метки клавиш разворачиваются раньше', () => {
    const t = 'Шаг{can:adjust| назад{key:undo| (%)}}.'
    expect(fillLevels(fillKeys(t, hint), can)).toBe('Шаг назад (Ctrl+Z).')
    expect(fillLevels(fillKeys(t, hint), () => false)).toBe('Шаг.')
  })
})

describe('levelMarks и страж вложенности', () => {
  it('id всех меток, в том числе у куска с {key} внутри', () => {
    expect(levelMarks('a{can:adjust| x} b{can:variants| y{key:undo| (%)}} c')).toEqual(['adjust', 'variants'])
    expect(levelMarks('нет меток')).toEqual([])
  })
  it('вложенную {can} находит, обычный текст и {key} — нет', () => {
    expect(nestedLevels('a{can:adjust| x{can:variants| y}}')).toHaveLength(1)
    expect(nestedLevels('a{can:adjust| x{key:undo| (%)}} b{can:variants| y}')).toEqual([])
  })
})
