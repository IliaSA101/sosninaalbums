import { describe, expect, it } from 'vitest'
import { arrowKey, hasIcons, hasMarks, markText, nextIndex, nodeKey, nodeRole, placeAtPoint, placeBeside, type MenuNode } from './menulist'

const n = (id: string, extra: Partial<MenuNode> = {}): MenuNode => ({ id, label: id, ...extra })

describe('nextIndex', () => {
  const list = [n('a'), n('b', { disabled: true }), n('note', { kind: 'note' }), n('c'), n('d', { disabled: true })]
  it('nextIndex пропускает серые и заметки, по кругу', () => {
    expect(nextIndex(list, 0, 1)).toBe(3)
    expect(nextIndex(list, 3, 1)).toBe(0) // d серый — круг на a
    expect(nextIndex(list, 0, -1)).toBe(3)
    expect(nextIndex(list, 3, -1)).toBe(0)
  })
  it('фокуса нет: вниз — первый, вверх — последний доступный', () => {
    expect(nextIndex(list, -1, 1)).toBe(0)
    expect(nextIndex(list, -1, -1)).toBe(3)
  })
  it('один доступный пункт — он же; ни одного или пусто — -1', () => {
    expect(nextIndex([n('a')], 0, 1)).toBe(0)
    expect(nextIndex([n('a', { disabled: true }), n('b', { kind: 'note' })], -1, 1)).toBe(-1)
    expect(nextIndex([], -1, 1)).toBe(-1)
  })
})

describe('arrowKey', () => {
  const group = n('g', { sub: [n('x')] })
  it('→ на пункте с подменю входит, на обычном и внутри подменю ничего', () => {
    expect(arrowKey('ArrowRight', group, false)).toBe('enter')
    expect(arrowKey('ArrowRight', n('plain'), false)).toBeNull()
    expect(arrowKey('ArrowRight', undefined, false)).toBeNull()
    expect(arrowKey('ArrowRight', group, true)).toBeNull()
  })
  it('← в подменю возвращает к пункту, на основном уровне ничего', () => {
    expect(arrowKey('ArrowLeft', undefined, true)).toBe('back')
    expect(arrowKey('ArrowLeft', group, false)).toBeNull()
  })
})

describe('вид пункта', () => {
  it('✓ и •: роль и текст отметки по виду', () => {
    expect(nodeRole(n('a'))).toBe('menuitem')
    expect(nodeRole(n('a', { mark: 'check' }))).toBe('menuitemcheckbox')
    expect(nodeRole(n('a', { mark: 'radio' }))).toBe('menuitemradio')
    expect(markText(n('a', { mark: 'check', checked: true }))).toBe('✓')
    expect(markText(n('a', { mark: 'radio', checked: true }))).toBe('•')
    expect(markText(n('a', { mark: 'radio', checked: false }))).toBe('')
    expect(markText(n('a'))).toBe('')
  })
  it('ключ пункта подменю различает параметр', () => {
    expect(nodeKey(n('layer', { arg: 'front' }))).not.toBe(nodeKey(n('layer', { arg: 'up' })))
    expect(nodeKey(n('delete'))).toBe('delete')
  })
})

describe('место', () => {
  const view = { width: 1000, height: 600 }
  const size = { width: 200, height: 300 }
  it('у точки: держится в окне', () => {
    expect(placeAtPoint({ x: 100, y: 50 }, size, view)).toEqual({ left: 100, top: 50 })
    expect(placeAtPoint({ x: 950, y: 590 }, size, view)).toEqual({ left: 792, top: 292 })
    expect(placeAtPoint({ x: -5, y: -5 }, size, view)).toEqual({ left: 8, top: 8 })
  })
  it('у кнопки: справа, если места нет — слева, по вертикали вровень, не ниже окна', () => {
    const btn = { left: 100, top: 100, width: 34, height: 22 }
    expect(placeBeside(btn, size, view, 'right')).toEqual({ left: 134, top: 100, flipped: false })
    expect(placeBeside(btn, size, view, 'left')).toEqual({ left: 134, top: 100, flipped: false }) // слева места нет — справа
    const far = { left: 700, top: 500, width: 34, height: 22 }
    expect(placeBeside(far, size, view, 'left')).toEqual({ left: 500, top: 292, flipped: true })
    const edge = { left: 900, top: 100, width: 34, height: 22 }
    expect(placeBeside(edge, size, view, 'right')).toEqual({ left: 700, top: 100, flipped: true }) // как в строке меню
  })
})

describe('hasIcons / hasMarks: столбцы значков и отметок', () => {
  it('нет ни значков, ни отметок — столбец отметок остаётся (меню без значков не сдвигается)', () => {
    const l: MenuNode[] = [n('a')]
    expect(hasIcons(l)).toBe(false)
    expect(hasMarks(l)).toBe(true)
  })
  it('значки без отметок — столбец значков вместо отметок; с отметками — оба', () => {
    expect(hasIcons([n('a'), n('b', { icon: 'text' })])).toBe(true)
    expect(hasMarks([n('b', { icon: 'text' })])).toBe(false)
    expect(hasMarks([n('b', { icon: 'text' }), n('c', { mark: 'check' })])).toBe(true)
  })
})
