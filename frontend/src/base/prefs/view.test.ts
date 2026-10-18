import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

// Минимальное хранилище вместо localStorage (среда тестов — node).
function useStorage() {
  const m = new Map<string, string>()
  const s = { getItem: (k: string) => m.get(k) ?? null, setItem: (k: string, v: string) => void m.set(k, v) }
  Object.defineProperty(globalThis, 'localStorage', { value: s, configurable: true, writable: true })
}

// ViewMode читает хранилище в конструкторе, поэтому модуль каждый раз грузится заново.
async function fresh() {
  vi.resetModules()
  return (await import('./view.svelte')).modes
}

describe('modes.sections.card', () => {
  // Первый import .svelte-модуля компилирует его и под нагрузкой полного прогона
  // не укладывается в 5 с теста: прогреваем кэш один раз с запасом по времени.
  beforeAll(async () => {
    useStorage()
    await import('./view.svelte')
  }, 30000)
  beforeEach(useStorage)

  it('по умолчанию карточка открыта', async () => {
    expect((await fresh()).sections.card).toBe(true)
  })

  it('сворачивание запоминается', async () => {
    const m = await fresh()
    m.toggleSection('card')
    expect(m.sections.card).toBe(false)
    expect((await fresh()).sections.card).toBe(false)
  })

  it('старая запись без card даёт true, остальное сохраняется', async () => {
    localStorage.setItem('viewMode', JSON.stringify({ details: true, sections: { info: false, settings: true, grid: false } }))
    const m = await fresh()
    expect(m.sections.card).toBe(true)
    expect(m.sections.info).toBe(false)
    expect(m.sections.settings).toBe(true)
    expect(m.details).toBe(true)
  })
})

describe('modes.auto*', () => {
  beforeEach(useStorage)

  it('среднее и якорь запоминаются', async () => {
    const m = await fresh()
    m.rememberAuto(30, 'auto', 21.4, 'per')
    const n = await fresh()
    expect([n.autoSpreads, n.autoRange, n.autoPer, n.autoBy]).toEqual([30, 'auto', 21.4, 'per'])
  })

  it('051: «Свой» диапазон и флажок серий запоминаются', async () => {
    const m = await fresh()
    expect([m.autoRange, m.autoSeries]).toEqual(['auto', true]) // первое открытие — «Авто», серии включены
    m.rememberAuto(11, [8, 10], 9, 'spreads', 1, false)
    const n = await fresh()
    expect([n.autoRange, n.autoSeries]).toEqual([[8, 10], false])
  })

  it('051: миграция прежнего «не больше»: 64 или нет — «Авто», меньшее m — «от min(3, m) до m»', async () => {
    const of = async (v: object) => {
      localStorage.setItem('viewMode', JSON.stringify(v))
      return (await fresh()).autoRange
    }
    expect(await of({ autoMaxPer: 64 })).toBe('auto')
    expect(await of({})).toBe('auto')
    expect(await of({ autoMaxPer: 20 })).toEqual([3, 20])
    expect(await of({ autoMaxPer: 2 })).toEqual([2, 2])
    expect(await of({ autoMaxPer: 1 })).toEqual([1, 1])
    expect(await of({ autoMaxPer: 'много' })).toBe('auto') // мусор
    expect(await of({ autoMaxPer: 200 })).toBe('auto')
    expect(await of({ autoRange: [4, 12], autoMaxPer: 20 })).toEqual([4, 12]) // новое поле главнее
    expect(await of({ autoRange: [12, 4] })).toBe('auto') // «от» > «до» — мусор
    expect(await of({ autoRange: 'x', autoSeries: 'да' })).toBe('auto')
    localStorage.setItem('viewMode', JSON.stringify({ autoSeries: 'да' }))
    expect((await fresh()).autoSeries).toBe(true)
  })

  it('по умолчанию — N, среднего нет', async () => {
    const m = await fresh()
    expect([m.autoPer, m.autoBy]).toEqual([null, 'spreads'])
  })

  it('мусор в хранилище — прежние значения', async () => {
    localStorage.setItem('viewMode', JSON.stringify({ autoPer: 100, autoBy: 'x' }))
    const m = await fresh()
    expect([m.autoPer, m.autoBy]).toEqual([null, 'spreads'])
    localStorage.setItem('viewMode', JSON.stringify({ autoPer: 'много', autoBy: 5 }))
    const k = await fresh()
    expect([k.autoPer, k.autoBy]).toEqual([null, 'spreads'])
    localStorage.setItem('viewMode', JSON.stringify({ autoPer: 0.2 }))
    expect((await fresh()).autoPer).toBeNull()
  })

  it('среднее при запоминании зажимается в 1…64', async () => {
    const m = await fresh()
    m.rememberAuto(5, 'auto', 500, 'per')
    expect(m.autoPer).toBe(64)
    m.rememberAuto(5, 'auto', 0.2, 'per')
    expect(m.autoPer).toBe(1)
  })

  it('разнообразие (041): по умолчанию «Обычное», запоминается, мусор — «Обычное»', async () => {
    const m = await fresh()
    expect(m.autoVariety).toBe(1)
    m.rememberAuto(5, 'auto', 4, 'spreads', 2)
    expect((await fresh()).autoVariety).toBe(2)
    m.rememberAuto(5, 'auto', 4, 'spreads', 0)
    expect((await fresh()).autoVariety).toBe(0)
    m.rememberAuto(5, 'auto', 4, 'spreads') // без аргумента — прежнее значение
    expect((await fresh()).autoVariety).toBe(0)
    localStorage.setItem('viewMode', JSON.stringify({ autoVariety: 7 }))
    expect((await fresh()).autoVariety).toBe(1)
  })

  it('семейства (065): по умолчанию «Все», запоминаются, мусор и «все шесть» — «Все»', async () => {
    const m = await fresh()
    expect(m.autoFamilies).toEqual([])
    m.rememberAuto(5, 'auto', 4, 'spreads', 1, true, ['hero', 'dense'])
    expect((await fresh()).autoFamilies).toEqual(['dense', 'hero']) // порядок сегментов
    m.rememberAuto(5, 'auto', 4, 'spreads') // без аргумента — прежний выбор
    expect((await fresh()).autoFamilies).toEqual(['dense', 'hero'])
    const of = async (x: unknown) => {
      localStorage.setItem('viewMode', JSON.stringify({ autoFamilies: x }))
      return (await fresh()).autoFamilies
    }
    expect(await of('dense')).toEqual([])
    expect(await of(7)).toEqual([])
    expect(await of(['dense', 'мусор', 5, 'dense', 'strip'])).toEqual(['dense', 'strip'])
    expect(await of(['dense', 'hero', 'air', 'bleed', 'strip', 'figure'])).toEqual([])
    m.rememberAuto(5, 'auto', 4, 'spreads', 1, true, [])
    expect((await fresh()).autoFamilies).toEqual([])
  })

  it('«Разнообразить раскладки»: «Меньше» считается «Обычным», «Больше» остаётся', async () => {
    const m = await fresh()
    m.autoVariety = 0
    expect(m.varyLevel).toBe(1)
    m.autoVariety = 1
    expect(m.varyLevel).toBe(1)
    m.autoVariety = 2
    expect(m.varyLevel).toBe(2)
  })
})

describe('thumbSize load', () => {
  beforeEach(useStorage)

  it('по умолчанию 110', async () => {
    expect((await fresh()).thumbSize).toBe(110)
  })

  it('setThumb округляет, зажимает и запоминается', async () => {
    const m = await fresh()
    m.setThumb(133.6)
    expect(m.thumbSize).toBe(134)
    m.setThumb(9999)
    expect(m.thumbSize).toBe(220)
    m.setThumb(1)
    expect(m.thumbSize).toBe(70)
    m.setThumb(150)
    expect((await fresh()).thumbSize).toBe(150)
  })

  it('setThumb: NaN размер не меняет', async () => {
    const m = await fresh()
    m.setThumb(NaN)
    expect(m.thumbSize).toBe(110)
  })

  it('мусор в хранилище — 110', async () => {
    for (const bad of ['abc', 5, 500, 120.5, null, [], {}, '150']) {
      localStorage.setItem('viewMode', JSON.stringify({ thumbSize: bad }))
      expect((await fresh()).thumbSize).toBe(110)
    }
  })
})

describe('modes.presetGroup (058)', () => {
  beforeEach(useStorage)

  it('по умолчанию пусто («Общие»); выбор запоминается', async () => {
    const m = await fresh()
    expect(m.presetGroup).toBe('')
    m.setPresetGroup('Нетпринт')
    expect((await fresh()).presetGroup).toBe('Нетпринт')
  })

  it('мусор в хранилище — «Общие»', async () => {
    localStorage.setItem('viewMode', JSON.stringify({ presetGroup: 5 }))
    expect((await fresh()).presetGroup).toBe('')
  })
})

describe('modes.markup (066)', () => {
  beforeEach(useStorage)

  it('по умолчанию включена', async () => {
    expect((await fresh()).markup).toBe(true)
  })
  it('выключение запоминается и возвращается обратно', async () => {
    const m = await fresh()
    m.toggleMarkup()
    expect(m.markup).toBe(false)
    const n = await fresh()
    expect(n.markup).toBe(false)
    n.toggleMarkup()
    expect((await fresh()).markup).toBe(true)
  })
  it('старая запись без поля даёт true; мусор вместо флажка — тоже', async () => {
    localStorage.setItem('viewMode', JSON.stringify({ safe: true }))
    expect((await fresh()).markup).toBe(true)
    localStorage.setItem('viewMode', JSON.stringify({ markup: 'нет' }))
    expect((await fresh()).markup).toBe(true)
  })
  it('не зависит от безопасной зоны', async () => {
    const m = await fresh()
    m.toggleMarkup()
    m.toggleSafe()
    const n = await fresh()
    expect([n.markup, n.safe]).toEqual([false, true])
  })
})

describe('modes.libSort (075: порядок ленты)', () => {
  beforeEach(useStorage)

  it('по умолчанию «по имени»', async () => {
    expect((await fresh()).libSort).toBe('name')
  })
  it('выбор запоминается; повторный выбор того же ничего не ломает', async () => {
    const m = await fresh()
    m.setLibSort('shot')
    m.setLibSort('shot')
    expect((await fresh()).libSort).toBe('shot')
    const n = await fresh()
    n.setLibSort('name')
    expect((await fresh()).libSort).toBe('name')
  })
  it('мусор в хранилище — «по имени»', async () => {
    localStorage.setItem('viewMode', JSON.stringify({ libSort: 'date' }))
    expect((await fresh()).libSort).toBe('name')
  })
})

describe('modes.keysMode и keysSplit (074: окно «Горячие клавиши»)', () => {
  beforeEach(useStorage)

  it('по умолчанию — «Карта крупно», границы по размеру клавиш', async () => {
    const m = await fresh()
    expect(m.keysMode).toBe('map')
    expect(m.keysSplit).toEqual({ ra: null, rw: 0.27, rh: 0.62 })
  })
  it('вид и доли запоминаются; повтор того же вида без записи', async () => {
    const m = await fresh()
    m.setKeysMode('list')
    m.setKeysSplit({ ra: 0.5, rw: 0.4, rh: 0.3 })
    const n = await fresh()
    expect(n.keysMode).toBe('list')
    expect(n.keysSplit).toEqual({ ra: 0.5, rw: 0.4, rh: 0.3 })
    const set = vi.spyOn(localStorage, 'setItem')
    n.setKeysMode('list')
    expect(set).not.toHaveBeenCalled()
  })
  it('мусор в хранилище — значения по умолчанию, годные доли остаются', async () => {
    localStorage.setItem('viewMode', JSON.stringify({ keysMode: 'x', keysSplit: { ra: 2, rw: 0.4, rh: 'a' } }))
    const m = await fresh()
    expect(m.keysMode).toBe('map')
    expect(m.keysSplit).toEqual({ ra: null, rw: 0.4, rh: 0.62 })
  })
})
