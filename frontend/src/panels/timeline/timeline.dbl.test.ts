import { beforeEach, describe, expect, it, vi } from 'vitest'

// Двойной щелчок по номеру (094 §3.1), лента из 3 групп. Окружение — node без DOM: щелчок проходит тот же путь,
// что в ленте, — захват на корне (naming.capture), затем, если не погашен, обработчик группы под курсором (pickGroup).
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: () => () => {} }))
vi.mock('../../../wailsjs/go/app/App', () => ({}))
vi.stubGlobal('window', globalThis)

const { naming } = await import('./naming.svelte')
const { prefs } = await import('../../base/prefs/prefs.svelte')

// Узел ленты: номер группы или миниатюра; closest — как у DOM для '.num' и '[data-spread]'.
type Attr = { getAttribute: (name: string) => string | null }
type Node = { closest: (sel: string) => Attr | null }
const node = (spread: string, num: boolean): Node => {
  const group: Attr = { getAttribute: (n) => (n === 'data-spread' ? spread : null) }
  const numEl: Attr = { getAttribute: () => null }
  return { closest: (sel) => (sel === '.num' ? (num ? numEl : null) : sel === '[data-spread]' ? group : null) }
}

const go = vi.fn()
const click = (target: Node | null, x: number, y: number, t: number, mods: Partial<Record<'ctrlKey' | 'shiftKey', boolean>> = {}) => {
  let stopped = false
  const e = {
    target,
    clientX: x,
    clientY: y,
    timeStamp: t,
    ctrlKey: false,
    shiftKey: false,
    metaKey: false,
    altKey: false,
    ...mods,
    stopPropagation: () => (stopped = true),
    preventDefault: () => {},
  }
  naming.capture(e)
  // Не погашен — доходит до группы: переход на неё (как pickGroup без модификаторов).
  if (!stopped && target) go(target.closest('[data-spread]')?.getAttribute('data-spread'))
}

beforeEach(() => {
  go.mockClear()
  naming.stop()
  prefs.level = 'standard'
  click(null, -1000, -1000, -100000) // сбросить запомненный первый щелчок
  go.mockClear()
})

describe('двойной щелчок в ленте (094)', () => {
  it('щелчок по номеру группы 2, второй в той же точке над группой 3 — поле у группы 2, перехода на 3 нет', () => {
    click(node('s2', true), 300, 120, 1000)
    expect(go).toHaveBeenCalledWith('s2') // первый щелчок ведёт на разворот, как раньше
    // Лента уехала: под курсором теперь номер группы 3.
    click(node('s3', true), 300, 120, 1250)
    expect(naming.id).toBe('s2')
    expect(go).toHaveBeenCalledTimes(1)
  })

  it('два щелчка через 600 мс — поля нет', () => {
    click(node('s2', true), 300, 120, 1000)
    click(node('s2', true), 300, 120, 1600)
    expect(naming.id).toBeNull()
    expect(go).toHaveBeenCalledTimes(2)
  })

  it('первый — по миниатюре или с Ctrl: двойного нет', () => {
    click(node('s2', false), 300, 120, 1000)
    click(node('s2', true), 300, 120, 1100)
    expect(naming.id).toBeNull()
    click(node('s1', true), 50, 120, 2000, { ctrlKey: true })
    click(node('s1', true), 50, 120, 2100)
    expect(naming.id).toBeNull()
  })

  it('без функции «Названия» второй щелчок гасится, но поле не открывается', () => {
    prefs.level = 'simple' // «Названия» — со «Стандартного»
    expect(prefs.can('names')).toBe(false)
    click(node('s1', true), 50, 120, 3000)
    click(node('s1', true), 50, 120, 3100)
    expect(naming.id).toBeNull()
    expect(go).toHaveBeenCalledTimes(1)
  })

  it('stop(id) закрывает только своё поле', () => {
    naming.start('s3')
    naming.stop('s1')
    expect(naming.id).toBe('s3')
    naming.stop('s3')
    expect(naming.id).toBeNull()
  })
})

// Подключение: захват на корне ленты; у номера своего ondblclick нет — иначе два пути открытия поля.
const src = import.meta.glob(['/src/panels/timeline/Timeline.svelte', '/src/panels/timeline/TimelineGroup.svelte'], {
  query: '?raw',
  import: 'default',
  eager: true,
}) as Record<string, string>

describe('подключение двойного щелчка', () => {
  it('корень ленты зовёт naming.capture в фазе захвата, номер группы dblclick не ловит', () => {
    expect(src['/src/panels/timeline/Timeline.svelte']).toContain('onclickcapture={(e) => naming.capture(e)}')
    expect(src['/src/panels/timeline/TimelineGroup.svelte']).not.toContain('ondblclick')
  })
})
