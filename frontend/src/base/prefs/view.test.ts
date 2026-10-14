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
    m.rememberAuto(30, 64, 21.4, 'per')
    const n = await fresh()
    expect([n.autoSpreads, n.autoMaxPer, n.autoPer, n.autoBy]).toEqual([30, 64, 21.4, 'per'])
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
    m.rememberAuto(5, 64, 500, 'per')
    expect(m.autoPer).toBe(64)
    m.rememberAuto(5, 64, 0.2, 'per')
    expect(m.autoPer).toBe(1)
  })

  it('разнообразие (041): по умолчанию «Обычное», запоминается, мусор — «Обычное»', async () => {
    const m = await fresh()
    expect(m.autoVariety).toBe(1)
    m.rememberAuto(5, 64, 4, 'spreads', 2)
    expect((await fresh()).autoVariety).toBe(2)
    m.rememberAuto(5, 64, 4, 'spreads', 0)
    expect((await fresh()).autoVariety).toBe(0)
    m.rememberAuto(5, 64, 4, 'spreads') // без аргумента — прежнее значение
    expect((await fresh()).autoVariety).toBe(0)
    localStorage.setItem('viewMode', JSON.stringify({ autoVariety: 7 }))
    expect((await fresh()).autoVariety).toBe(1)
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
