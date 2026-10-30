import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { countLabel } from './topbar'

describe('сводка в шапке', () => {
  const album = { sheet: false }
  it('склоняет разворот', () => {
    expect(countLabel(1, 2, album)).toBe('1 разворот (2 стр.)')
    expect(countLabel(2, 4, album)).toBe('2 разворота (4 стр.)')
    expect(countLabel(5, 10, album)).toBe('5 разворотов (10 стр.)')
    expect(countLabel(11, 22, album)).toBe('11 разворотов (22 стр.)')
    expect(countLabel(21, 42, album)).toBe('21 разворот (42 стр.)')
  })
  it('у листа — «листа», без страниц', () => {
    expect(countLabel(3, 3, { sheet: true })).toBe('3 листа')
    expect(countLabel(1, 1, { sheet: true })).toBe('1 лист')
  })
})

// happy-dom раскладку не считает, поэтому правила сжатия шапки проверяем по тексту стилей компонента.
const css = readFileSync(new URL('./TopBar.svelte', import.meta.url), 'utf8').split('<style>')[1] ?? ''
const body = (selector: string): string => {
  const esc = selector.replace(/[.*+?^${}()|[\]\\:>]/g, '\\$&')
  return new RegExp(`(?:^|\\})\\s*${esc}\\s*\\{([^}]*)\\}`, 'm').exec(css)?.[1] ?? ''
}

describe('сжатие шапки в окне 1024 (регресс QA 0.25: «Экспорт» обрезан на 6 px)', () => {
  it('левая группа уступает место: может сжиматься, а не распирает шапку', () => {
    expect(body('.group:not(.right)')).toMatch(/flex:\s*0 1 auto/)
    expect(body('.group:not(.right)')).toMatch(/min-width:\s*0/)
  })
  it('сжимается только название (с «…»), остальное в группе держит размер', () => {
    expect(body('.group:not(.right) > :not(.title)')).toMatch(/flex:\s*none/)
    expect(body('.title')).toMatch(/min-width:\s*\d+px/)
    expect(body('.title')).toMatch(/text-overflow:\s*ellipsis/)
  })
  it('правая группа с «Экспортом» не сжимается', () => {
    expect(body('.group')).toMatch(/flex:\s*none/)
  })
})
