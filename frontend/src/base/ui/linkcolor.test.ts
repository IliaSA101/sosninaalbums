import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

// Контраст WCAG 2.x по токенам из style.css (тёмная тема — единственная).
const root = fileURLToPath(new URL('../../', import.meta.url))
const css = readFileSync(join(root, 'style.css'), 'utf8')

const token = (name: string): string => new RegExp(`--${name}:\\s*(#[0-9a-fA-F]{6})`).exec(css)?.[1] ?? ''

function lum(hex: string): number {
  const c = [1, 3, 5]
    .map((i) => parseInt(hex.slice(i, i + 2), 16) / 255)
    .map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4))
  return 0.2126 * c[0]! + 0.7152 * c[1]! + 0.0722 * c[2]!
}
function contrast(a: string, b: string): number {
  const [hi, lo] = [lum(a), lum(b)].sort((x, y) => y - x) as [number, number]
  return (hi + 0.05) / (lo + 0.05)
}

// Фоны окон и плиток: страница, панель, вдавленная панель, кнопка (--line).
const BACKS = ['bg', 'panel', 'panel2', 'line'].map(token)

// Ссылка внутри предупреждения сознательно наследует его цвет (#e0806f), иначе пропадает сигнал «внимание».
const INHERITS_WARN = new Set(['AlbumInfo.svelte'])

describe('цвет ссылки-кнопки --link', () => {
  it('контраст ≥ 4.5:1 на фонах окон', () => {
    const link = token('link')
    expect(link).not.toBe('')
    for (const back of BACKS) expect(contrast(link, back), back).toBeGreaterThanOrEqual(4.5)
  })

  it('подсчёт верен: белое на чёрном 21:1', () => {
    expect(contrast('#ffffff', '#000000')).toBeCloseTo(21, 5)
  })

  it('подчёркнутые .link красятся --link (или сознательно --accent), не --muted и не наследуют цвет', () => {
    const bad: string[] = []
    let found = 0
    const walk = (dir: string): void => {
      for (const name of readdirSync(dir)) {
        const p = join(dir, name)
        if (statSync(p).isDirectory()) walk(p)
        else if (name.endsWith('.svelte')) {
          const src = readFileSync(p, 'utf8')
          for (const m of src.matchAll(/^ {2}\.link \{([^}]*)\}/gm)) {
            const body = m[1] ?? ''
            found++
            if (INHERITS_WARN.has(name)) continue
            if (/underline/.test(body) && !/(^|[;\s{])color:\s*var\(--(link|accent)\)/.test(body)) bad.push(name)
          }
        }
      }
    }
    walk(root)
    expect(found).toBeGreaterThan(10)
    expect(bad).toEqual([])
  })
})
