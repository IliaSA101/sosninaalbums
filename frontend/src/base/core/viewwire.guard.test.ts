import { describe, expect, it, vi } from 'vitest'
import { store } from './store.svelte'

vi.mock('../../../wailsjs/go/app/App', () => ({}))
vi.mock('../../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn() }))

// Страж 075: вид попадает в store одним путём — apply() с проверкой цепочки seq (дельта, дубль, пересинхрон).
// Прямая запись store.view или своя подписка на событие view обходят цепочку: развороты затрутся или задвоятся.
describe('вид альбома только через store.apply', () => {
  const files = import.meta.glob('/src/**/*.{ts,svelte}', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
  const code = (src: string) => src.split('\n').map((l) => l.replace(/\/\/.*$/, ''))
  const own = (path: string) =>
    /\.test\.ts$/.test(path) || path === '/src/base/core/store.svelte.ts' || path === '/src/base/core/nav.svelte.ts'

  it('view пишет только store/nav, события view слушает только store', () => {
    const bad: string[] = []
    for (const [path, src] of Object.entries(files)) {
      if (own(path)) continue
      code(src).forEach((line, i) => {
        if (/\bstore\.view\s*=[^=]/.test(line) || /\bstore\.view\.(spreads|cover)\s*=[^=]/.test(line))
          bad.push(`${path}:${i + 1}: запись в store.view`)
        if (/EventsOn\(\s*['"]view['"]/.test(line)) bad.push(`${path}:${i + 1}: подписка на view`)
        if (/\bGetView\(/.test(line) && !path.startsWith('/wailsjs')) bad.push(`${path}:${i + 1}: GetView мимо store.resync`)
      })
    }
    expect(bad, 'store.apply(view) / store.run(Api.X())').toEqual([])
  })

  it('store.apply проверяет цепочку, пересинхрон идёт через GetView, saved не принимает вид', () => {
    const src = files['/src/base/core/store.svelte.ts']
    const chain = files['/src/base/core/viewchain.ts']
    expect(chain).toMatch(/admit\(this\.host\.current\(\), this\.seq, raw\)/)
    expect(chain).toMatch(/kind === 'skip'/)
    expect(chain).toMatch(/this\.host\.fetch\(\)/)
    expect(src).toMatch(/chain\.take\(/)
    expect(src).toMatch(/Api\.GetView\(\)/)
    const saved = src.split('\n').find((l) => l.includes("EventsOn('saved'")) ?? ''
    expect(saved).toContain('dirty')
    expect(saved).not.toMatch(/:\s*View\b/)
  })

  // 075-16: view — $state.raw, глубокого прокси нет. Запись во вложенное (store.view.rev = 5, view.spreads[0].name = …,
  // push/splice/sort по массивам вида) не вызовет перерисовку и портит неизменённые развороты, общие между видами.
  // Правка вида — только store.apply(новый вид); в тесте — store.view = { ...store.view, поле }.
  it('вложенное store.view.* на месте не пишется нигде, включая тесты', () => {
    const bad: string[] = []
    const write = /\bstore\.view(\??\.[A-Za-z_]\w*|\[[^\]]+\])+\s*(=[^=>]|\+=|-=|\*=|\+\+|--)/
    const cast = /\(store\.view as [^)]*\)(\??\.[A-Za-z_]\w*|\[[^\]]+\])+\s*(=[^=>]|\+=|-=)/
    const inc = /(\+\+|--)\s*store\.view\./
    const mut = /\bstore\.view(\??\.[A-Za-z_]\w*|\[[^\]]+\])*\.(push|pop|shift|unshift|splice|sort|reverse|fill|copyWithin)\(/
    const assign = /(Object\.assign\(\s*|\bdelete\s+\(?)store\.view\b/
    for (const [path, src] of Object.entries(files)) {
      if (path.endsWith('viewwire.guard.test.ts')) continue
      code(src).forEach((line, i) => {
        if (write.test(line) || cast.test(line) || inc.test(line) || mut.test(line) || assign.test(line))
          bad.push(`${path}:${i + 1}: ${line.trim()}`)
      })
    }
    expect(bad, 'замените вид целиком: store.apply(...) или store.view = { ...store.view, ... }').toEqual([])
  })

  it('view объявлен как $state.raw, used считает UsedIndex', () => {
    const nav = files['/src/base/core/nav.svelte.ts']
    expect(nav).toMatch(/view = \$state\.raw<View>\(/)
    expect(nav).toMatch(/this\.usedIdx\.update\(/)
  })

  it('в dev и тестах вид заморожен: запись на месте падает', () => {
    store.apply({ rev: 1, seq: 0, spreads: [{ id: 'a', photos: ['x'], rects: [] }], settings: {} } as never)
    expect(() => {
      store.view.rev = 2
    }).toThrow()
    expect(() => {
      store.view.spreads[0].photos.push('y')
    }).toThrow()
    expect(() => {
      ;(store.view.spreads[0] as { name?: string }).name = 'Правка'
    }).toThrow()
    store.view = { ...store.view, rev: 3 } // замена целиком разрешена
    expect(store.view.rev).toBe(3)
  })

  it('страж видит файлы', () => {
    expect(Object.keys(files).length).toBeGreaterThan(100)
  })
})
