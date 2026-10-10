import { describe, expect, it } from 'vitest'
import { FEATURES, LEVELS, can, type Base, type Custom, type FeatureId, type RowId } from './levels'
import { blockerOf, dependentsOf, diff, features, fromBase, inBase, locked, needsOf, rows, setBase, toggle } from './levelcustom'

const ROW_IDS = FEATURES.map((f) => f.id)
const baseRows = (b: Base): RowId[] => FEATURES.filter((f) => inBase(f, b)).map((f) => f.id)
const sorted = <T extends string>(s: Iterable<T>): T[] => [...s].sort()

describe('замкнутость по опорам', () => {
  it('у каждой функции трёх уровней её опоры есть в том же уровне', () => {
    for (const b of LEVELS) {
      const set = rows(fromBase(b))
      for (const id of set) for (const n of needsOf(id)) expect(set.has(n), `${b}: ${id} → ${n}`).toBe(true)
    }
  })
  it('опоры транзитивны и считаются без самой строки', () => {
    expect(needsOf('groupXf')).toEqual(['groupSelect', 'manual'])
    expect(needsOf('rotate')).toEqual(['manual'])
    expect(needsOf('manual')).toEqual([])
    expect(dependentsOf('manual')).toEqual(['panels', 'rotate', 'layers', 'align', 'templates', 'groupXf'])
    expect(dependentsOf('groupSelect')).toEqual(['groupXf'])
    expect(dependentsOf('base')).toEqual([])
  })
})

describe('«Свой» и основа', () => {
  it('«Свой» без правок равен основе', () => {
    for (const b of LEVELS) {
      expect(sorted(rows(fromBase(b)))).toEqual(sorted(baseRows(b)))
      expect(sorted(features('custom', fromBase(b)))).toEqual(sorted(features(b, null)))
    }
  })
  it('features готового уровня совпадает с can', () => {
    for (const b of LEVELS) {
      const set = features(b, null)
      for (const f of FEATURES) for (const id of [f.id, ...(f.also ?? [])]) expect(set.has(id), `${b}:${id}`).toBe(can(id, b))
    }
  })
  it('features несёт also строки: цвет фона вместе с подписями', () => {
    const set = features('custom', toggle(fromBase('simple'), 'captions', false).custom)
    expect(set.has('captions')).toBe(false)
    expect(set.has('bgColor')).toBe(false)
    expect(features('custom', fromBase('standard')).has('allSpreads')).toBe(true)
  })
  it('«Свой» без набора — «Простой»', () => {
    expect(sorted(features('custom', null))).toEqual(sorted(features('simple', null)))
  })
  it('в «Своём» заблокированы только base и menu', () => {
    expect(FEATURES.filter(locked).map((f) => f.id)).toEqual(['base', 'menu'])
    for (const id of ['base', 'menu'] as const) {
      const r = toggle(fromBase('full'), id, false)
      expect(r.custom).toEqual(fromBase('full'))
      expect(r.removed).toEqual([])
    }
    expect(toggle(fromBase('simple'), 'turn', false).removed).toEqual(['turn'])
  })
  it('снять всё — остаются base и menu (основа любого уровня)', () => {
    for (const b of LEVELS) {
      let c = fromBase(b)
      for (const id of ROW_IDS) c = toggle(c, id, false).custom
      expect(sorted(rows(c))).toEqual(['base', 'menu'])
      expect(c.on).toEqual([])
      expect(c.off).toHaveLength(baseRows(b).length - 2)
    }
  })
  it('смена основы сбрасывает правки', () => {
    const edited = toggle(fromBase('full'), 'manual', false).custom
    expect(setBase('standard')).toEqual({ base: 'standard', on: [], off: [] })
    expect(sorted(rows(setBase('full')))).toEqual(sorted(baseRows('full')))
    expect(rows(edited).has('manual')).toBe(false)
  })
})

describe('галочки и опоры', () => {
  it('галочка включает опоры: поворот мест → ручная', () => {
    const r = toggle(fromBase('standard'), 'rotate', true)
    expect(r.added).toEqual(['manual', 'rotate'])
    expect(r.removed).toEqual([])
    expect(rows(r.custom).has('manual')).toBe(true)
    expect(sorted(r.custom.on)).toEqual(['manual', 'rotate'])
  })
  it('масштаб группы → ручная и выделение группы', () => {
    const r = toggle(fromBase('simple'), 'groupXf', true)
    expect(sorted(r.added)).toEqual(['groupSelect', 'groupXf', 'manual'])
    expect(features('custom', r.custom).has('groupXf')).toBe(true)
  })
  it('включение уже включённой ничего не меняет', () => {
    const r = toggle(fromBase('full'), 'rotate', true)
    expect(r.added).toEqual([])
    expect(r.custom).toEqual(fromBase('full'))
  })
  it('снятие гасит зависящие: ручная → 6 строк', () => {
    const r = toggle(fromBase('full'), 'manual', false)
    expect(sorted(r.removed)).toEqual(sorted(['manual', 'panels', 'rotate', 'layers', 'align', 'templates', 'groupXf']))
    expect(r.removed).toHaveLength(7) // сама строка и шесть зависящих
    const set = rows(r.custom)
    for (const id of r.removed) expect(set.has(id), id).toBe(false)
    expect(set.has('rulers')).toBe(true) // линейки на ручную не опираются
  })
  it('снятие выделения группы гасит масштаб группы, но не ручную', () => {
    const r = toggle(fromBase('full'), 'groupSelect', false)
    expect(sorted(r.removed)).toEqual(['groupSelect', 'groupXf'])
    expect(rows(r.custom).has('manual')).toBe(true)
  })
  it('снятие строки не затрагивает её опоры', () => {
    const r = toggle(fromBase('full'), 'rotate', false)
    expect(r.removed).toEqual(['rotate'])
    expect(rows(r.custom).has('manual')).toBe(true)
  })
  it('туда и обратно возвращает пустой набор', () => {
    const off = toggle(fromBase('full'), 'manual', false).custom
    expect(toggle(off, 'rotate', true).added).toEqual(['manual', 'rotate'])
    let c = off
    for (const id of ['manual', 'panels', 'rotate', 'layers', 'align', 'templates', 'groupXf'] as RowId[]) c = toggle(c, id, true).custom
    expect(c).toEqual(fromBase('full'))
  })
  it('чужая строка и пустой id — без изменений', () => {
    const c = fromBase('standard')
    expect(toggle(c, 'nope' as RowId, true)).toEqual({ custom: c, added: [], removed: [] })
  })
})

describe('разница с основой', () => {
  it('разница с основой минимальна', () => {
    const c = toggle(toggle(fromBase('standard'), 'rulers', true).custom, 'flip', false).custom
    expect(c).toEqual({ base: 'standard', on: ['rulers'], off: ['flip'] })
    // вернули — набор снова пуст
    const back = toggle(toggle(c, 'rulers', false).custom, 'flip', true).custom
    expect(back).toEqual(fromBase('standard'))
  })
  it('diff не пишет заблокированные в off и порядок — как в таблице', () => {
    const set = new Set<RowId>(['base', 'menu', 'rulers', 'turn'])
    const d = diff('simple', set)
    expect(d.on).toEqual(['rulers'])
    expect(d.off).toEqual(['variants', 'timeline', 'captions', 'marks'])
    expect(diff('simple', new Set<RowId>()).off).not.toContain('base')
  })
  it('новая функция версии следует основе', () => {
    // В файле упомянуты только text (снят) и rulers (добавлен); всё прочее живёт по основе.
    const full: Custom = { base: 'full', on: [], off: ['text'] }
    const set = rows(full)
    for (const f of FEATURES) {
      if (f.id === 'text' || f.id === 'textRotate') expect(set.has(f.id), f.id).toBe(false)
      else expect(set.has(f.id), f.id).toBe(true)
    }
    const simple: Custom = { base: 'simple', on: ['text'], off: [] }
    expect(sorted(rows(simple))).toEqual(sorted([...baseRows('simple'), 'text']))
  })
  it('чужие id сохраняются при правке', () => {
    const c: Custom = { base: 'standard', on: ['future1'], off: ['future2', 'flip'] }
    const r = toggle(c, 'rulers', true).custom
    expect(r.on).toEqual(['rulers', 'future1'])
    expect(r.off).toEqual(['flip', 'future2'])
    expect(toggle(r, 'flip', true).custom.off).toEqual(['future2'])
    // и не влияют на набор
    expect(sorted(rows(c))).toEqual(sorted(baseRows('standard').filter((id) => id !== 'flip')))
  })
  it('несогласованный набор из файла гасит зависящие', () => {
    // «Поворот мест» включён сверх «Простого», а «Ручной» нет — гасим, а не включаем опору.
    const c: Custom = { base: 'simple', on: ['rotate', 'groupXf', 'panels'], off: [] }
    const set = rows(c)
    for (const id of ['rotate', 'groupXf', 'panels'] as const) expect(set.has(id), id).toBe(false)
    // основа «Полный» с выключенной ручной: зависящие тоже гаснут
    const full: Custom = { base: 'full', on: [], off: ['manual'] }
    for (const id of dependentsOf('manual')) expect(rows(full).has(id), id).toBe(false)
    expect(rows(full).has('rulers')).toBe(true)
  })
  it('неизвестные id и основа из файла не ломают набор', () => {
    const c = { base: 'custom', on: ['zzz'], off: ['yyy'] } as unknown as Custom
    expect(sorted(rows(c))).toEqual(sorted(baseRows('simple')))
    const off: Custom = { base: 'full', on: [], off: ['base', 'menu'] }
    expect(rows(off).has('base') && rows(off).has('menu')).toBe(true) // заблокированные не снимаются
  })
  it('нетронутый вход не мутируется', () => {
    const c: Custom = { base: 'full', on: ['x'], off: ['y'] }
    const copy = structuredClone(c)
    toggle(c, 'manual', false)
    expect(c).toEqual(copy)
  })
})

describe('blockerOf', () => {
  it('на готовом уровне называет уровень', () => {
    expect(blockerOf('text', 'simple', null)).toEqual({ level: 'standard' })
    expect(blockerOf('place', 'standard', null)).toEqual({ level: 'full' })
    expect(blockerOf('place', 'full', null)).toBeNull()
    expect(blockerOf('caption', 'simple', null)).toBeNull()
  })
  it('в «Своём» называет функцию', () => {
    const c = toggle(fromBase('full'), 'manual', false).custom
    expect(blockerOf('place', 'custom', c)).toEqual({ row: 'manual' })
    expect(blockerOf('text', 'custom', c)).toBeNull()
    const noText = toggle(fromBase('full'), 'text', false).custom
    expect(blockerOf('text', 'custom', noText)).toEqual({ row: 'text' })
    expect(blockerOf('caption', 'custom', toggle(fromBase('simple'), 'captions', false).custom)).toEqual({ row: 'captions' })
  })
  it('«Свой» без набора — как «Простой»', () => {
    expect(blockerOf('text', 'custom', null)).toEqual({ row: 'text' })
    expect(blockerOf('caption', 'custom', null)).toBeNull()
  })
})

// Все главные id, которые отдаёт features, существуют (страховка от опечаток в also).
describe('features', () => {
  it('отдаёт только известные id', () => {
    const known = new Set<FeatureId>(FEATURES.flatMap((f) => [f.id, ...(f.also ?? [])]))
    for (const id of features('custom', fromBase('full'))) expect(known.has(id)).toBe(true)
    expect(features('custom', fromBase('full')).size).toBe(known.size)
  })
})
