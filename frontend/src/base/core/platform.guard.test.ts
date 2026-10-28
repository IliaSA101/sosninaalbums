import { describe, expect, it } from 'vitest'

// Страж 073: кнопку мыши проверяет только platform.ts. Иначе на Mac Ctrl+щелчок снова начнёт жест или выделение.
describe('кнопка мыши только через platform.ts', () => {
  const files = import.meta.glob('/src/**/*.{ts,svelte}', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

  it('в обработчиках нет button !== 0 и button === 0', () => {
    const bad: string[] = []
    for (const [path, src] of Object.entries(files)) {
      if (/\.test\.ts$/.test(path) || path === '/src/base/core/platform.ts') continue
      src.split('\n').forEach((line, i) => {
        const code = line.replace(/\/\/.*$/, '').replace(/<!--.*?-->/g, '')
        if (/\bbutton\s*[!=]==?\s*0\b/.test(code) || /\b0\s*[!=]==?\s*\w+\.button\b/.test(code))
          bad.push(`${path}:${i + 1}: ${line.trim()}`)
      })
    }
    expect(bad, 'primaryPress(e) / secondaryPress(e) из base/core/platform').toEqual([])
  })

  it('страж видит файлы', () => {
    expect(files['/src/base/core/platform.ts']).toContain('primaryPress')
    expect(Object.keys(files).length).toBeGreaterThan(100)
  })
})
