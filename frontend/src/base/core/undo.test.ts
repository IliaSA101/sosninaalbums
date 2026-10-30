import { describe, expect, it } from 'vitest'
import { pickCurrent } from './undo'

describe('pickCurrent', () => {
  it('разворот на месте — идём за ним по id', () => {
    expect(pickCurrent(['x', 'a', 'b'], 'b', 1, '')).toEqual({ index: 2, id: 'b', lost: '', back: -1 })
  })
  it('текущий удалён — тот же индекс, id запомнен', () => {
    expect(pickCurrent(['a', 'c'], 'b', 1, '')).toEqual({ index: 1, id: 'c', lost: 'b', back: -1 })
  })
  it('удалён последний — индекс сжимается', () => {
    expect(pickCurrent(['a'], 'b', 1, '')).toEqual({ index: 0, id: 'a', lost: 'b', back: -1 })
  })
  it('Undo вернул исчезнувший — back на него, ожидание сброшено', () => {
    expect(pickCurrent(['a', 'b', 'c'], 'c', 1, 'b')).toEqual({ index: 2, id: 'c', lost: '', back: 1 })
  })
  it('не вернулся — ожидание живёт', () => {
    expect(pickCurrent(['a', 'c'], 'c', 1, 'b').lost).toBe('b')
  })
  it('пустой список', () => {
    expect(pickCurrent([], 'a', 0, '')).toEqual({ index: 0, id: '', lost: 'a', back: -1 })
  })
})
