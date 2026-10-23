import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// Черта между группами ленты (094 §1): склейка — только Ctrl+щелчок (⌘ на Mac) или Enter/Пробел.
// Окружение — node без DOM: обработчики JoinLine зовём объектом события, разметку сверяем по исходнику.
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))
vi.stubGlobal('window', globalThis)

const { store } = await import('../../base/core/store.svelte')
const { setMacForTest } = await import('../../base/core/platform')
const { joinClick, joinKey } = await import('./stripact')
const { joinState, joinTip, VIG_WHY } = await import('./timeline')
const { gridAct } = await import('../layouts/gridact.svelte')

const ok = { ok: true, why: '' }
const click = (ctrlKey = false, metaKey = false) => ({ ctrlKey, metaKey, stopPropagation: vi.fn() })
const key = (k: string) => ({ key: k, preventDefault: vi.fn(), stopPropagation: vi.fn() })
const sp = (id: string, n: number, grid = false) => ({ id, photos: Array.from({ length: n }, (_, i) => id + i), rects: [], grid }) as never

let merge: ReturnType<typeof vi.spyOn>
let askGlue: ReturnType<typeof vi.spyOn>
beforeEach(() => {
  setMacForTest(false)
  merge = vi.spyOn(store, 'merge').mockResolvedValue(undefined)
  askGlue = vi.spyOn(gridAct, 'askGlue').mockResolvedValue(undefined)
})
afterEach(() => {
  merge.mockRestore()
  askGlue.mockRestore()
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

  it('ручной: Ctrl+щелчок и Enter открывают вопрос, не клеят (103)', async () => {
    store.view = { ...store.view, spreads: [sp('a', 2, true), sp('b', 2)] }
    const st = joinState(store.view.spreads, 0, 64)
    expect(st).toEqual({ ok: true, why: '', ask: 'У разворота 1 ручная раскладка' })
    expect(joinTip(st, false)).toBe('Ctrl+щелчок — склеить. У разворота 1 ручная раскладка: места вернутся к «Авто», сначала спросим')
    expect(joinClick(click(), 'a', st, true)).toBe(false)
    expect(joinClick(click(true), 'a', st, true)).toBe(true)
    expect(joinKey(key('Enter'), 'a', st, true)).toBe(true)
    expect(joinKey(key(' '), 'a', st, true)).toBe(true)
    expect(merge).not.toHaveBeenCalled()
    expect(askGlue).toHaveBeenCalledTimes(3)
    const g = askGlue.mock.calls[0][0]
    expect(g).toMatchObject({
      kind: 'join',
      title: 'Склеить развороты?',
      ids: ['a', 'b'],
      ask: 'У разворота 1 ручная раскладка. Ваши правки мест сбросятся. Склеить развороты 1 и 2?',
    })
    // «Склеить» в окне — склейка через ручной с галочкой кадра
    await g.run(true)
    expect(merge).toHaveBeenCalledExactlyOnceWith('a', true)
  })

  it('виньетка в паре — причина, вопроса и склейки нет', () => {
    const st = joinState([{ id: 'a', photos: ['x', 'y'], rects: [], grid: true, kind: 'vignette' } as never, sp('b', 2)], 0, 64)
    expect(st).toEqual({ ok: false, why: VIG_WHY })
    expect(joinTip(st, false)).toBe(VIG_WHY)
    expect(joinClick(click(true), 'a', st, true)).toBe(false)
    expect(joinKey(key('Enter'), 'a', st, true)).toBe(false)
    expect(merge).not.toHaveBeenCalled()
    expect(askGlue).not.toHaveBeenCalled()
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
