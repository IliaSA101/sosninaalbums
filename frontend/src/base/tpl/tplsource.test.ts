import { describe, expect, it } from 'vitest'
import { tplSources } from './tplsource'

const t = {
  empty: { all: 'нет шаблонов', builtin: 'нет встроенных', mine: 'нет своих' },
  off: { builtin: 'причина встроенных', mine: 'причина своих' },
}
const b = ['b1', 'b2', 'b3']
const m = ['m1']

describe('tplSources', () => {
  it('Все — встроенные, затем свои за чертой', () => {
    const v = tplSources(b, m, 'all', t)
    expect(v.parts).toEqual([
      { sep: '', list: b },
      { sep: 'мои', list: m },
    ])
    expect(v.items.map((i) => [i.id, i.count])).toEqual([
      ['all', 4],
      ['builtin', 3],
      ['mine', 1],
    ])
    expect(v.empty).toBe('')
  })

  it('Встроенные и Мои — без черты', () => {
    expect(tplSources(b, m, 'builtin', t).parts).toEqual([{ sep: '', list: b }])
    expect(tplSources(b, m, 'mine', t).parts).toEqual([{ sep: '', list: m }])
  })

  it('«Все» с одной непустой частью — без черты', () => {
    expect(tplSources([], m, 'all', t).parts).toEqual([{ sep: '', list: m }])
    expect(tplSources(b, [], 'all', t).parts).toEqual([{ sep: '', list: b }])
  })

  it('нет своих — «Мои» выключен с подсказкой', () => {
    const v = tplSources(b, [], 'all', t)
    const mine = v.items.find((i) => i.id === 'mine')
    expect(mine).toMatchObject({ off: true, count: 0, title: 'причина своих' })
    expect(v.items.find((i) => i.id === 'all')?.off).toBeFalsy()
    expect(v.items.find((i) => i.id === 'builtin')?.off).toBeFalsy()
    expect(tplSources([], m, 'all', t).items.find((i) => i.id === 'builtin')).toMatchObject({ off: true, title: 'причина встроенных' })
  })

  it('вид отбирает только встроенные', () => {
    // builtin уже под фильтром вида (2 из 3), свои приходят целиком
    const v = tplSources(b.slice(0, 2), ['m1', 'm2'], 'all', t)
    expect(v.items[0].count).toBe(4)
    expect(v.items[1].count).toBe(2)
    expect(v.items[2].count).toBe(2)
  })

  it('пустой источник — текст пустого ряда', () => {
    expect(tplSources([], [], 'all', t)).toMatchObject({ parts: [], empty: 'нет шаблонов' })
    expect(tplSources(b, [], 'mine', t)).toMatchObject({ parts: [], empty: 'нет своих' })
    expect(tplSources([], m, 'builtin', t)).toMatchObject({ parts: [], empty: 'нет встроенных' })
  })
})
