import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Зона разреза в ленте (094 §1–§2): обработчики SpreadChips и их подключение. Окружение тестов — node без DOM,
// поэтому щелчок — объект события, а разметка компонента сверяется по исходнику (как imgcancel.test.ts).
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))

// Слушатели окна у drag: тест сам двигает мышь и отпускает.
const ls = vi.hoisted(() => ({ move: null as ((e: unknown) => void) | null, up: null as (() => void) | null }))
vi.stubGlobal('window', {
  addEventListener: (t: string, f: never) => {
    if (t === 'pointermove') ls.move = f
    if (t === 'pointerup') ls.up = f
  },
  removeEventListener: () => {},
  setTimeout: globalThis.setTimeout,
})
vi.stubGlobal('document', {
  body: { classList: { add: () => {}, remove: () => {} } },
  elementsFromPoint: () => [],
  addEventListener: () => {}, // скрытие окна (092 U1)
  removeEventListener: () => {},
})

const { store } = await import('../../base/core/store.svelte')
const { drag } = await import('../../base/drag/drag.svelte')
const { setMacForTest } = await import('../../base/core/platform')
const { cutClick, cutKey } = await import('./stripact')

const click = (ctrlKey = false, metaKey = false) => ({ ctrlKey, metaKey, stopPropagation: vi.fn() })
const key = (k: string) => ({ key: k, preventDefault: vi.fn(), stopPropagation: vi.fn() })
const tick = () => new Promise((r) => setTimeout(r, 0))

let split: ReturnType<typeof vi.spyOn>
beforeEach(() => {
  setMacForTest(false)
  split = vi.spyOn(store, 'split').mockResolvedValue(undefined)
})
afterEach(() => {
  split.mockRestore()
  setMacForTest(null)
})

describe('зона разреза (094)', () => {
  it('щелчок без Ctrl по зоне не режет', () => {
    const e = click()
    expect(cutClick(e, 's1', 2)).toBe(false)
    expect(split).not.toHaveBeenCalled()
    // Событие идёт дальше — к миниатюре (выделение фото).
    expect(e.stopPropagation).not.toHaveBeenCalled()
  })

  it('Ctrl+щелчок по зоне режет перед фото j', () => {
    const e = click(true)
    expect(cutClick(e, 's1', 2)).toBe(true)
    expect(split).toHaveBeenCalledWith('s1', 2)
    // Миниатюра его не получит: иначе Ctrl+щелчок ещё и выделил бы группу.
    expect(e.stopPropagation).toHaveBeenCalled()
  })

  it('на Mac режет ⌘+щелчок, а Ctrl+щелчок (там правая кнопка) — нет', () => {
    setMacForTest(true)
    expect(cutClick(click(true), 's1', 1)).toBe(false)
    expect(cutClick(click(false, true), 's1', 1)).toBe(true)
    expect(split).toHaveBeenCalledTimes(1)
  })

  it('после переноса щелчок в ту же точку не режет', async () => {
    const down = { button: 0, ctrlKey: false, shiftKey: false, metaKey: false, altKey: false, clientX: 10, clientY: 10 } as never
    drag.start(down, () => ({ kind: 'photos', ids: ['p1'], fromSpread: 's1' }))
    ls.move!({ clientX: 40, clientY: 10 })
    expect(drag.active).toBe(true)
    ls.up!()
    // Щелчок того же отпускания гасит drag, даже с Ctrl.
    expect(cutClick(click(true), 's1', 1)).toBe(false)
    await tick()
    // Следующий простой щелчок в ту же точку (зона встала под курсор после ответа Go) — тоже не режет.
    expect(cutClick(click(), 's1', 1)).toBe(false)
    expect(split).not.toHaveBeenCalled()
  })

  it('Enter на зоне режет', () => {
    const e = key('Enter')
    expect(cutKey(e, 's1', 3)).toBe(true)
    expect(split).toHaveBeenCalledWith('s1', 3)
    // Синтетического щелчка кнопки не будет — второго разреза тоже.
    expect(e.preventDefault).toHaveBeenCalled()
    expect(cutKey(key(' '), 's1', 3)).toBe(true)
    expect(cutKey(key('a'), 's1', 3)).toBe(false)
    expect(split).toHaveBeenCalledTimes(2)
  })
})

// Разметка: без Ctrl зона мышь не ловит, с Ctrl (класс .cutting у ленты) — ловит; 6 px без захода на фото.
const src = import.meta.glob(['/src/panels/timeline/SpreadChips.svelte', '/src/panels/timeline/Timeline.svelte'], {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>

describe('подключение зоны разреза', () => {
  const chips = src['/src/panels/timeline/SpreadChips.svelte'] ?? ''
  const rule = (sel: string) => chips.match(new RegExp(`\\n  ${sel.replace(/[.()]/g, '\\$&')} \\{([^}]*)\\}`))?.[1] ?? ''

  it('щелчок и Enter идут через cutClick / cutKey', () => {
    expect(chips).toContain('onclick={(e) => cutClick(e, sp.id, j)}')
    expect(chips).toContain('onkeydown={(e) => cutPress(e, sp.id, j)}')
  })
  it('зона 6 px слева от рамки, мышь — только при .cutting', () => {
    const cut = rule('.cut')
    expect(cut).toContain('left: -6px;')
    expect(cut).toContain('width: 6px;')
    expect(cut).toContain('pointer-events: none;')
    expect(rule(':global(.timeline.cutting) .cut')).toContain('pointer-events: auto;')
  })
  it('лента ставит .cutting по зажатому Ctrl', () => {
    const tl = src['/src/panels/timeline/Timeline.svelte'] ?? ''
    expect(tl).toContain('const cutting = $derived(cuts && cutKey.held)')
    expect(tl).toMatch(/class:cutting\b/)
  })
})
