import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Черта между группами ленты (094 §1): склейка — только Ctrl+щелчок (⌘ на Mac) или Enter/Пробел.
// Окружение — node без DOM: обработчики JoinLine зовём объектом события, разметку сверяем по исходнику.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { setMacForTest } = await import('../../base/core/platform')
const { joinClick, joinKey } = await import('./stripact')
const { joinState, joinTip } = await import('./timeline')

const ok = { ok: true, why: '' }
const click = (ctrlKey = false, metaKey = false) => ({ ctrlKey, metaKey, stopPropagation: vi.fn() })
const key = (k: string) => ({ key: k, preventDefault: vi.fn(), stopPropagation: vi.fn() })
const sp = (id: string, n: number, grid = false) => ({ id, photos: Array.from({ length: n }, (_, i) => id + i), rects: [], grid }) as never

let merge: ReturnType<typeof vi.spyOn>
beforeEach(() => {
  setMacForTest(false)
  merge = vi.spyOn(store, 'merge').mockResolvedValue(undefined)
})
afterEach(() => {
  merge.mockRestore()
  setMacForTest(null)
})

describe('черта склейки (094)', () => {
  it('щелчок без Ctrl не клеит', () => {
    expect(joinClick(click(), 'a', ok, true)).toBe(false)
    expect(merge).not.toHaveBeenCalled()
  })

  it('Ctrl+щелчок клеит', () => {
    expect(joinClick(click(true), 'a', ok, true)).toBe(true)
    expect(merge).toHaveBeenCalledWith('a')
    // Без функции «Разрез и склейка» черта — только цель переноса.
    expect(joinClick(click(true), 'a', ok, false)).toBe(false)
    expect(merge).toHaveBeenCalledTimes(1)
  })

  it('Enter и Пробел клеят без Ctrl', () => {
    expect(joinKey(key('Enter'), 'a', ok, true)).toBe(true)
    expect(joinKey(key(' '), 'a', ok, true)).toBe(true)
    expect(joinKey(key('Tab'), 'a', ok, true)).toBe(false)
    expect(merge).toHaveBeenCalledTimes(2)
  })

  it('подсказка Ctrl / ⌘ по платформе', () => {
    expect(joinTip(ok, false)).toBe('Ctrl+щелчок — склеить')
    expect(joinTip(ok, true)).toBe('⌘+щелчок — склеить')
    setMacForTest(true)
    // На Mac Ctrl+щелчок — правая кнопка: не клеит; ⌘+щелчок — клеит.
    expect(joinClick(click(true), 'a', ok, true)).toBe(false)
    expect(joinClick(click(false, true), 'a', ok, true)).toBe(true)
  })

  it('сетка — прежняя причина', () => {
    const st = joinState([sp('a', 2, true), sp('b', 2)], 0, 64)
    expect(st).toEqual({ ok: false, why: 'Сначала «Вернуть авто»' })
    expect(joinTip(st, false)).toBe('Сначала «Вернуть авто»')
    expect(joinClick(click(true), 'a', st, true)).toBe(false)
    expect(joinKey(key('Enter'), 'a', st, true)).toBe(false)
    expect(merge).not.toHaveBeenCalled()
  })
})

const src = import.meta.glob('/src/panels/timeline/JoinLine.svelte', { query: '?raw', import: 'default', eager: true }) as Record<
  string,
  string
>

describe('подключение черты', () => {
  const join = src['/src/panels/timeline/JoinLine.svelte'] ?? ''
  it('щелчок и Enter идут через joinClick / joinKey, подсказка — joinTip', () => {
    expect(join).toContain('onclick={(e) => joinClick(e, id, state, cuts)}')
    expect(join).toContain('onkeydown={(e) => joinKey(e, id, state, cuts)}')
    expect(join).toContain('joinTip(state, isMac())')
  })
  it('без Ctrl черта не подсвечивается при наведении', () => {
    expect(join).toContain('.join.armed:hover::before')
    expect(join).not.toMatch(/\n {2}\.join:hover::before/)
  })
})
