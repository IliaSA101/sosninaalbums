import { describe, expect, it } from 'vitest'

// Страж 073 (большой альбом): тысячи узлов не читают размер синхронно при монтировании, а длинные списки
// монтируют только видимое. Иначе открытие альбома из 200 разворотов снова зависнет на секунды.
describe('большой альбом: размеры и окна списков', () => {
  const files = import.meta.glob('/src/**/*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>
  const src = (name: string): string => {
    const path = Object.keys(files).find((p) => p.endsWith(name))
    if (!path) throw new Error(`нет файла ${name}`)
    return files[path]
  }
  const code = (s: string) => s.replace(/<!--[\s\S]*?-->/g, '').replace(/\/\/.*$/gm, '')

  it('в списочных компонентах нет bind:clientWidth/clientHeight/offsetWidth/offsetHeight', () => {
    for (const f of [
      '/photo/Cell.svelte',
      '/spread/Spread.svelte',
      '/timeline/SpreadChips.svelte',
      '/library/Thumb.svelte',
      '/details/SpreadGrid.svelte',
    ]) {
      expect(code(src(f)), f).not.toMatch(/bind:(client|offset)(Width|Height)/)
    }
  })

  it('«Все развороты»: Spread монтируется только под near.has', () => {
    const s = code(src('/details/SpreadGrid.svelte'))
    const uses = s.match(/<Spread\b/g) ?? []
    expect(uses).toHaveLength(1)
    expect(s).toMatch(/\{#if[^}]*near\.has\([^}]*\}\s*<Spread\b/)
  })

  it('лента альбома: SpreadChips монтируется только под near.has', () => {
    const s = code(src('/timeline/Timeline.svelte'))
    expect(s).toMatch(/\{#if near\.has\([^}]*\}\s*<SpreadChips\b/)
  })

  it('лента фото рисует окно рядов, а не весь store.visible', () => {
    const s = code(src('/library/Library.svelte'))
    expect(s).not.toMatch(/\{#each\s+store\.visible/)
    expect(s).toMatch(/\{#each shown as/)
  })

  it('near.watch читает id только через untrack: правка даёт новые объекты разворотов, иначе плитки размонтируются', () => {
    let found = 0
    for (const [path, text] of Object.entries(files)) {
      const s = code(text)
      const all = s.match(/near\.watch\(/g) ?? []
      const safe = s.match(/near\.watch\(\s*\w+,\s*untrack\(/g) ?? []
      expect(safe.length, path).toBe(all.length)
      found += all.length
    }
    expect(found).toBeGreaterThanOrEqual(2)
  })

  it('страж видит файлы', () => {
    expect(Object.keys(files).length).toBeGreaterThan(100)
  })
})
