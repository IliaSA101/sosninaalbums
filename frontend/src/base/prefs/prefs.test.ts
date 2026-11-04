import { beforeEach, describe, expect, it, vi } from 'vitest'

// Go и события Wails подменены: проверяется только состояние prefs и порядок вызовов.
// notify ставит таймер через window (тест идёт в окружении node).
vi.stubGlobal('window', globalThis)

const calls: string[] = []
let failCustom = false
let gate: Promise<void> | null = null

vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({
  SetLevel: async (l: string) => {
    calls.push('level:' + l)
    return { level: l, showStart: true, first: false }
  },
  SetCustom: async (c: { base: string; on: string[]; off: string[] }) => {
    calls.push('custom:' + JSON.stringify(c))
    const fail = failCustom
    if (gate) await gate
    if (fail) throw new Error('диск полон')
    return {}
  },
  SetShowStart: async () => ({}),
}))

const { prefs } = await import('./prefs.svelte')
const { store } = await import('../core/store.svelte')
const { modes } = await import('./view.svelte')

const tick = () => new Promise((r) => setTimeout(r, 0))

beforeEach(() => {
  calls.length = 0
  failCustom = false
  gate = null
  prefs.load({ level: 'standard', showStart: false, first: false, custom: null })
  store.toast = null
  modes.big = false
})

describe('prefs: набор «Своего» (спека 011 U2)', () => {
  it('load: уровень и набор из ответа Go; чужая основа — «Простой»', () => {
    prefs.load({ level: 'custom', showStart: true, first: false, custom: { base: 'weird', on: ['rotate'], off: [] } })
    expect(prefs.level).toBe('custom')
    expect(prefs.custom).toEqual({ base: 'simple', on: ['rotate'], off: [] })
    // rotate без опоры manual гаснет (несогласованный набор из файла)
    expect(prefs.can('rotate')).toBe(false)
    expect(prefs.can('base')).toBe(true)
  })

  it('«Свой» в первый раз копирует текущий уровень; набор пишется до уровня', async () => {
    await prefs.set('custom')
    expect(prefs.custom).toEqual({ base: 'standard', on: [], off: [] })
    expect(prefs.can('text')).toBe(true)
    expect(prefs.can('manual')).toBe(false)
    expect(calls).toEqual(['custom:{"base":"standard","on":[],"off":[]}', 'level:custom'])
  })

  it('переход на другой уровень не теряет набор, возврат отдаёт его', async () => {
    prefs.toggle('flip', false)
    await tick()
    const set = prefs.custom
    expect(set?.off).toEqual(['flip'])
    await prefs.set('full')
    expect(prefs.custom).toEqual(set)
    expect(prefs.can('flip')).toBe(true)
    calls.length = 0
    await prefs.set('custom')
    expect(prefs.can('flip')).toBe(false)
    expect(calls).toEqual(['level:custom']) // набор не переписывается
  })

  it('галочка «Поворот мест» включает «Ручную раскладку»; «Вернуть» снимает обе', async () => {
    const e = prefs.toggle('rotate', true)
    await tick()
    expect(e.added).toEqual(['manual', 'rotate'])
    expect(prefs.level).toBe('custom')
    expect(prefs.can('manual') && prefs.can('rotate')).toBe(true)
    await prefs.apply(e.prev)
    expect(prefs.level).toBe('standard')
    expect(prefs.can('manual') || prefs.can('rotate')).toBe(false)
  })

  it('ошибка Go откатывает набор и показывает тост', async () => {
    failCustom = true
    prefs.toggle('flip', false)
    await tick()
    expect(prefs.level).toBe('standard')
    expect(prefs.custom).toBeNull()
    expect(store.toast?.kind).toBe('error')
  })

  it('два apply: сбой первого не затирает успешный второй', async () => {
    let release: () => void = () => {}
    gate = new Promise<void>((r) => (release = r))
    failCustom = true
    const p1 = prefs.apply({ level: 'custom', custom: { base: 'simple', on: ['flip'], off: [] } })
    failCustom = false
    gate = null
    const second = { base: 'simple' as const, on: ['rotate'], off: [] }
    const p2 = prefs.apply({ level: 'custom', custom: second })
    await p2
    release()
    await p1
    expect(prefs.custom).toEqual(second)
    expect(prefs.level).toBe('custom')
    expect(store.toast?.kind).toBe('error')
  })

  it('место «Ручной» в «Своём»: тост называет функцию, «Включить» включает её', async () => {
    await prefs.set('custom')
    expect(prefs.blocked('place')).toBe(true)
    expect(store.toast?.text).toBe('Править — включите «Ручная раскладка»')
    expect(store.toast?.action?.label).toBe('Включить')
    store.toast?.action?.run()
    await tick()
    expect(prefs.can('manual')).toBe(true)
    expect(prefs.blocked('place')).toBe(false)
  })

  it('«Включить» после включения зовёт after (выделение места)', async () => {
    await prefs.set('custom')
    let picked = 0
    prefs.blocked('place', () => picked++)
    store.toast?.action?.run()
    await tick()
    await tick()
    expect(picked).toBe(1)
  })

  it('«Включить» при ошибке Go: набор откатан, after не зовётся', async () => {
    await prefs.set('custom')
    let picked = 0
    prefs.blocked('place', () => picked++)
    failCustom = true
    store.toast?.action?.run()
    await tick()
    await tick()
    expect(picked).toBe(0)
    expect(prefs.can('manual')).toBe(false)
  })

  it('на готовом уровне тост прежний: «Переключить»', () => {
    prefs.load({ level: 'simple', showStart: false, first: false, custom: null })
    expect(prefs.blocked('text')).toBe(true)
    expect(store.toast?.text).toBe('Править — в уровне «Стандартный»')
    expect(store.toast?.action?.label).toBe('Переключить')
  })

  it('подпись без «Подписей»: объект блокирован с названием функции', async () => {
    prefs.toggle('captions', false)
    await tick()
    expect(prefs.blocked('caption')).toBe(true)
    expect(store.toast?.text).toBe('Править — включите «Подписи и цвет фона»')
  })

  it('смена основы начинает набор заново', async () => {
    prefs.toggle('flip', false)
    await tick()
    prefs.setBase('full')
    await tick()
    expect(prefs.custom).toEqual({ base: 'full', on: [], off: [] })
    expect(prefs.can('manual')).toBe(true)
  })
})
