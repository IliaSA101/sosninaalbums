import { describe, expect, it } from 'vitest'

// Страж 075-9 (derived_inert): исчезающая ветка (`out:`) на время перехода INERT, её обработчики читают
// производные неживого владельца. Три правки держим исходником: слот без указателя, снимок данных
// переноса при нажатии, перехват предупреждения со стеком в dev.
describe('derived_inert: правки 075-9', () => {
  const svelte = import.meta.glob('/src/**/*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
  const ts = import.meta.glob(['/src/main.ts', '/src/base/drag/drag.svelte.ts'], {
    query: '?raw',
    import: 'default',
    eager: true,
  }) as Record<string, string>
  const code = (s: string) => s.replace(/<!--[\s\S]*?-->/g, '').replace(/\/\/.*$/gm, '')
  const file = (name: string) => {
    const path = Object.keys(ts).find((p) => p.endsWith(name))
    if (!path) throw new Error(`нет файла ${name}`)
    return code(ts[path])
  }

  it('у каждого out: функция перехода задаёт pointer-events:none', () => {
    let found = 0
    for (const [path, text] of Object.entries(svelte)) {
      const s = code(text)
      for (const m of s.matchAll(/\bout:(\w+)/g)) {
        found++
        const start = s.indexOf(`function ${m[1]}(`)
        expect(start, `${path}: нет функции ${m[1]}`).toBeGreaterThan(-1)
        // тело функции — до следующей верхнеуровневой функции или конца скрипта
        const rest = s.slice(start + 1)
        const end = rest.search(/\n {2}function |<\/script>/)
        const body = end < 0 ? rest : rest.slice(0, end)
        expect(body, `${path}: ${m[1]}`).toContain('pointer-events:none')
      }
    }
    expect(found).toBeGreaterThanOrEqual(1)
  })

  it('drag.start: make() зовётся в теле start до подписки на pointermove', () => {
    const s = file('drag.svelte.ts')
    const start = s.indexOf('start(e: PointerEvent')
    const make = s.indexOf('make()', start)
    const sub = s.indexOf("addEventListener('pointermove'", start)
    expect(start).toBeGreaterThan(-1)
    expect(make).toBeGreaterThan(start)
    expect(make).toBeLessThan(sub)
    // единственный вызов — не в обработчике сдвига
    expect(s.match(/\bmake\(\)/g)).toHaveLength(1)
  })

  it('main.ts: в dev derived_inert уходит в журнал рантайма со стеком', () => {
    const s = file('main.ts')
    expect(s).toContain('import.meta.env.DEV')
    expect(s).toContain('derived_inert')
    expect(s).toContain('LogWarning')
    expect(s).toContain('new Error().stack')
  })
})
