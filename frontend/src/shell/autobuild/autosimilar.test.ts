import { describe, expect, it } from 'vitest'
import {
  SIMILAR_KEEP,
  SIMILAR_OPTIONS,
  canShowDrops,
  dropTitle,
  keepKey,
  keptOf,
  mergeDrops,
  sigsText,
  similarHint,
  similarStatus,
} from './autosimilar'

const d = (id: string) => ({ id, like: 'x', dist: 1 })
const base = { level: 1, waiting: false, sigs: null, dropped: 0, kept: 0, error: false }

describe('«Убирать похожие кадры»: уровни и подсказки', () => {
  it('Нет · Дубли · Очень похожие · Похожие = 0…3', () => {
    expect(SIMILAR_OPTIONS.map((o) => [o.v, o.label])).toEqual([
      [0, 'Нет'],
      [1, 'Дубли'],
      [2, 'Очень похожие'],
      [3, 'Похожие'],
    ])
  })
  it('подсказка уровня; у «Нет» без правила про «беру» и 4–5★', () => {
    expect(similarHint(0)).toBe('Сборка берёт все фото')
    for (const v of [1, 2, 3]) expect(similarHint(v)).toContain(SIMILAR_KEEP)
    expect(SIMILAR_KEEP).toContain('4–5★')
    expect(similarHint(9)).toBe('Сборка берёт все фото') // мусор — как «Нет»
  })
})

describe('строка статуса', () => {
  it('«Нет» — пусто', () => {
    expect(similarStatus({ ...base, level: 0, dropped: 5 })).toBe('')
  })
  it('идёт подготовка: «Сверяю кадры: done из total…»', () => {
    expect(similarStatus({ ...base, waiting: true, sigs: { done: 830, total: 3200 } })).toBe('Сверяю кадры: 830 из 3200…')
    expect(sigsText(null)).toBe('Сверяю кадры…')
  })
  it('«Не возьму N похожих кадров» со склонением', () => {
    expect(similarStatus({ ...base, dropped: 37 })).toBe('Не возьму 37 похожих кадров')
    expect(similarStatus({ ...base, dropped: 1 })).toBe('Не возьму 1 похожего кадра')
    expect(similarStatus({ ...base, dropped: 2 })).toBe('Не возьму 2 похожих кадра')
    expect(similarStatus({ ...base, dropped: 21 })).toBe('Не возьму 21 похожего кадра')
  })
  it('нечего убирать, всё вернули, ошибка', () => {
    expect(similarStatus(base)).toBe('Похожих кадров нет')
    expect(similarStatus({ ...base, kept: 4 })).toBe('Все похожие кадры возвращены (4)')
    expect(similarStatus({ ...base, error: true })).toBe('Не удалось сверить кадры')
  })
  it('«Показать» есть, пока есть что показывать', () => {
    expect(canShowDrops(0, 0)).toBe(false)
    expect(canShowDrops(3, 0)).toBe(true)
    expect(canShowDrops(0, 2)).toBe(true) // всё вернули — ряд нужен, чтобы убрать снова
  })
})

describe('ключ «Вернуть» и ряд', () => {
  it('ключ не зависит от порядка и различает наборы', () => {
    expect(keepKey(new Set(['a', 'b']))).toBe(keepKey(new Set(['b', 'a'])))
    expect(keepKey(new Set(['a', 'b']))).not.toBe(keepKey(new Set(['a', 'c'])))
    expect(keepKey(new Set(['a']))).not.toBe(keepKey(new Set()))
    expect(keepKey(new Set())).toBe('0:0')
  })
  it('слияние: ответ Go + вернувшиеся из прежнего ряда, порядок ленты, без повторов', () => {
    const order = { a: 0, b: 1, c: 2, d: 3 }
    const prev = [d('a'), d('c'), d('d')]
    expect(mergeDrops(prev, [d('d'), d('b')], new Set(['c']), order).map((x) => x.id)).toEqual(['b', 'c', 'd'])
    expect(mergeDrops(null, [d('c'), d('a')], new Set(), order).map((x) => x.id)).toEqual(['a', 'c'])
    expect(mergeDrops(prev, [], new Set(), order)).toEqual([]) // никто не вернут, Go ничего не отдал
  })
  it('сколько вернули из показанных', () => {
    expect(keptOf([d('a'), d('b'), d('c')], new Set(['b', 'z']))).toBe(1)
  })
  it('подсказка плитки называет имя, состояние и действие', () => {
    expect(dropTitle('a.jpg', 'b.jpg', false)).toContain('не возьму')
    expect(dropTitle('a.jpg', 'b.jpg', false)).toContain('вернуть')
    expect(dropTitle('a.jpg', 'b.jpg', true)).toContain('вернётся')
    expect(dropTitle('a.jpg', 'b.jpg', true)).toContain('убрать снова')
  })
})
